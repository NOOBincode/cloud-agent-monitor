package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloud-agent-monitor/internal/topology/domain"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
)

// TopologyService 是拓扑模块的核心服务，协调拓扑发现、缓存和分析。
//
// TopologyService 负责管理服务拓扑和网络拓扑的完整生命周期：
//   - 定期从多个数据源发现拓扑数据（Kubernetes、Prometheus 等）
//   - 维护内存中的图结构，支持高效的拓扑查询
//   - 缓存分析结果，提升查询性能
//   - 持久化拓扑快照，支持历史查询
//
// 使用示例:
//
//	service := NewTopologyService(repo, cache, []DiscoveryBackend{k8sBackend, promBackend}, &Config{
//	    RefreshInterval: 30 * time.Second,
//	    CacheTTL:        5 * time.Minute,
//	    MaxDepth:        10,
//	})
//	if err := service.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer service.Stop()
type TopologyService struct {
	repo        domain.TopologyRepository
	cache       domain.TopologyCache
	discoverers []domain.DiscoveryBackend
	analyzer    *GraphAnalyzer
	localCache  *infra.Cache
	queue       infra.QueueInterface

	serviceGraph *InMemoryGraph
	networkGraph *InMemoryGraph

	refreshInterval time.Duration
	cacheTTL        time.Duration
	maxDepth        int

	running bool
	mu      sync.RWMutex
}

// Config 定义 TopologyService 的配置参数。
type Config struct {
	// RefreshInterval 拓扑数据刷新间隔
	RefreshInterval time.Duration
	// CacheTTL 缓存结果的有效期
	CacheTTL time.Duration
	// MaxDepth 影响分析的最大遍历深度
	MaxDepth int
}

// NewTopologyService 创建一个新的拓扑服务实例。
//
// 参数:
//   - repo: 拓扑数据持久化仓储
//   - cache: 拓扑数据缓存
//   - discoverers: 拓扑发现后端列表（支持多个数据源）
//   - cfg: 服务配置，为 nil 时使用默认值
//
// 默认配置:
//   - RefreshInterval: 30 秒
//   - CacheTTL: 5 分钟
//   - MaxDepth: 10
func NewTopologyService(
	repo domain.TopologyRepository,
	cache domain.TopologyCache,
	discoverers []domain.DiscoveryBackend,
	localCache *infra.Cache,
	queue infra.QueueInterface,
	cfg *Config,
) *TopologyService {
	if cfg == nil {
		cfg = &Config{
			RefreshInterval: 30 * time.Second,
			CacheTTL:        5 * time.Minute,
			MaxDepth:        10,
		}
	}

	serviceGraph := NewInMemoryGraph()
	networkGraph := NewInMemoryGraph()

	return &TopologyService{
		repo:            repo,
		cache:           cache,
		discoverers:     discoverers,
		analyzer:        NewGraphAnalyzer(serviceGraph),
		localCache:      localCache,
		queue:           queue,
		serviceGraph:    serviceGraph,
		networkGraph:    networkGraph,
		refreshInterval: cfg.RefreshInterval,
		cacheTTL:        cfg.CacheTTL,
		maxDepth:        cfg.MaxDepth,
	}
}

// Start 启动拓扑服务，开始定期刷新拓扑数据。
//
// 该方法会启动一个后台 goroutine 定期执行拓扑发现和刷新。
// 如果服务已在运行，则直接返回 nil。
//
// 参数:
//   - ctx: 上下文，用于初始化时的拓扑发现
//
// 返回: 启动失败时返回错误
func (s *TopologyService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	if err := s.RefreshServiceTopology(ctx); err != nil {
		log.Printf("[TopologyService] Initial service topology refresh failed: %v", err)
	}

	if err := s.RefreshNetworkTopology(ctx); err != nil {
		log.Printf("[TopologyService] Initial network topology refresh failed: %v", err)
	}

	s.registerQueueHandlers()

	if s.queue != nil {
		if _, err := s.queue.Enqueue(ctx, "topology:refresh-service", nil); err != nil {
			log.Printf("[TopologyService] Failed to enqueue initial refresh-service: %v", err)
		}
		if _, err := s.queue.Enqueue(ctx, "topology:refresh-network", nil); err != nil {
			log.Printf("[TopologyService] Failed to enqueue initial refresh-network: %v", err)
		}
	}

	return nil
}

func (s *TopologyService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

func (s *TopologyService) GetAnalyzer() *GraphAnalyzer {
	return s.analyzer
}

func (s *TopologyService) registerQueueHandlers() {
	if s.queue == nil {
		return
	}

	s.queue.RegisterHandler("topology:refresh-service", func(ctx context.Context, payload []byte) error {
		if err := s.RefreshServiceTopology(ctx); err != nil {
			log.Printf("[TopologyService] Async service topology refresh failed: %v", err)
			return err
		}
		if _, err := s.queue.EnqueueWithDelay(ctx, "topology:refresh-service", nil, s.refreshInterval); err != nil {
			log.Printf("[TopologyService] Failed to schedule next refresh-service: %v", err)
		}
		return nil
	})

	s.queue.RegisterHandler("topology:refresh-network", func(ctx context.Context, payload []byte) error {
		if err := s.RefreshNetworkTopology(ctx); err != nil {
			log.Printf("[TopologyService] Async network topology refresh failed: %v", err)
			return err
		}
		if _, err := s.queue.EnqueueWithDelay(ctx, "topology:refresh-network", nil, s.refreshInterval); err != nil {
			log.Printf("[TopologyService] Failed to schedule next refresh-network: %v", err)
		}
		return nil
	})
}

// GetServiceTopology 获取服务拓扑数据。
//
// 该方法返回当前内存中的服务拓扑，支持按命名空间和服务名称过滤。
// 如果内存图为空，会先触发一次拓扑刷新。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - query: 查询参数，支持 Namespace 和 ServiceName 过滤
//
// 返回:
//   - *ServiceTopology: 服务拓扑数据
//   - error: 拓扑刷新失败时返回错误
func (s *TopologyService) GetServiceTopology(ctx context.Context, query domain.TopologyQuery) (*domain.ServiceTopology, error) {
	if s.localCache != nil && !query.HasNamespace() && query.ServiceName == "" {
		if data, err := s.localCache.Get(ctx, "topology:service"); err == nil {
			var topology domain.ServiceTopology
			if err := json.Unmarshal(data, &topology); err == nil {
				return &topology, nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.serviceGraph.IsEmpty() {
		if err := s.RefreshServiceTopology(ctx); err != nil {
			return nil, err
		}
	}

	nodes := s.serviceGraph.GetAllNodes()
	edges := s.serviceGraph.GetAllEdges()

	if query.HasNamespace() {
		nodes = s.filterNodesByNamespace(nodes, query.Namespace)
		edges = s.filterEdgesByNamespace(edges, query.Namespace)
	}

	if query.ServiceName != "" {
		nodes = s.filterNodesByName(nodes, query.ServiceName)
	}

	topology := &domain.ServiceTopology{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Nodes:     nodes,
		Edges:     edges,
		Hash:      s.computeTopologyHash(nodes, edges),
	}

	return topology, nil
}

func (s *TopologyService) GetServiceNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	node := s.serviceGraph.GetNode(id)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}
	return node, nil
}

func (s *TopologyService) GetServiceNodeByName(ctx context.Context, namespace, name string) (*domain.ServiceNode, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	id := uuid.NewSHA1(uuid.Nil, []byte(key))
	return s.GetServiceNode(ctx, id)
}

func (s *TopologyService) GetNetworkTopology(ctx context.Context, query domain.TopologyQuery) (*domain.NetworkTopology, error) {
	if s.localCache != nil && !query.HasNamespace() {
		if data, err := s.localCache.Get(ctx, "topology:network"); err == nil {
			var topology domain.NetworkTopology
			if err := json.Unmarshal(data, &topology); err == nil {
				return &topology, nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.networkGraph.IsEmpty() {
		if err := s.RefreshNetworkTopology(ctx); err != nil {
			return nil, err
		}
	}

	nodes := s.networkGraph.GetAllNetworkNodes()
	edges := s.networkGraph.GetAllNetworkEdges()

	if query.HasNamespace() {
		nodes = s.filterNetworkNodesByNamespace(nodes, query.Namespace)
	}

	topology := &domain.NetworkTopology{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Nodes:     nodes,
		Edges:     edges,
	}

	return topology, nil
}

func (s *TopologyService) GetNetworkNode(ctx context.Context, id uuid.UUID) (*domain.NetworkNode, error) {
	node := s.networkGraph.GetNetworkNode(id)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}
	return node, nil
}

// AnalyzeImpact 分析指定服务故障的影响范围。
//
// 该方法首先尝试从缓存获取结果，缓存未命中时调用 GraphAnalyzer
// 进行实时分析。分析结果会被缓存以提升后续查询性能。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 待分析的服务 ID
//   - maxDepth: 最大遍历深度，<= 0 时使用服务默认配置
//
// 返回:
//   - *ImpactResult: 影响分析结果
//   - error: 服务不存在或分析失败时返回错误
func (s *TopologyService) AnalyzeImpact(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.ImpactResult, error) {
	if maxDepth <= 0 {
		maxDepth = s.maxDepth
	}

	cached, err := s.cache.GetImpactCache(ctx, serviceID)
	if err == nil && cached != nil {
		return cached, nil
	}

	result, err := s.analyzer.AnalyzeImpact(ctx, serviceID, maxDepth)
	if err != nil {
		return nil, err
	}

	_ = s.cache.SetImpactCache(ctx, serviceID, result, s.cacheTTL)

	return result, nil
}

// AnalyzeImpactBatch 批量分析多个服务的影响范围。
//
// 该方法并发执行多个服务的影响分析，通过 semaphore 限制并发数（默认最大 10）。
// 支持通过 context 取消操作。失败的服务会被跳过，不影响其他服务的分析。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceIDs: 待分析的服务 ID 列表
//   - maxDepth: 最大遍历深度，<= 0 时使用服务默认配置
//
// 返回:
//   - map[uuid.UUID]*ImpactResult: 服务 ID 到分析结果的映射
//   - error: 当前实现总是返回 nil，失败的服务不会出现在结果中
func (s *TopologyService) AnalyzeImpactBatch(ctx context.Context, serviceIDs []uuid.UUID, maxDepth int) (map[uuid.UUID]*domain.ImpactResult, error) {
	if maxDepth <= 0 {
		maxDepth = s.maxDepth
	}

	results := make(map[uuid.UUID]*domain.ImpactResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	maxConcurrency := 10
	if len(serviceIDs) < maxConcurrency {
		maxConcurrency = len(serviceIDs)
	}
	sem := make(chan struct{}, maxConcurrency)

	for _, id := range serviceIDs {
		select {
		case <-ctx.Done():
			break
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(serviceID uuid.UUID) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := s.AnalyzeImpact(ctx, serviceID, maxDepth)
			if err == nil {
				mu.Lock()
				results[serviceID] = result
				mu.Unlock()
			}
		}(id)
	}

	wg.Wait()
	return results, nil
}

// FindPath 查找两个服务之间的调用路径。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - sourceID: 源服务 ID
//   - targetID: 目标服务 ID
//   - maxHops: 路径最大跳数，<= 0 时使用默认值
//
// 返回:
//   - *PathResult: 路径查找结果
//   - error: 服务不存在或路径不存在时返回错误
func (s *TopologyService) FindPath(ctx context.Context, sourceID, targetID uuid.UUID, maxHops int) (*domain.PathResult, error) {
	if maxHops <= 0 {
		maxHops = s.maxDepth
	}
	return s.analyzer.FindPath(ctx, sourceID, targetID, maxHops)
}

// FindShortestPath 查找两个服务之间的最短路径。
//
// 该方法委托给 GraphAnalyzer 执行，使用 BFS 算法保证找到跳数最少的路径。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - sourceID: 源服务 ID
//   - targetID: 目标服务 ID
//
// 返回:
//   - []PathHop: 最短路径的节点序列
//   - error: 服务不存在或路径不存在时返回错误
func (s *TopologyService) FindShortestPath(ctx context.Context, sourceID, targetID uuid.UUID) ([]domain.PathHop, error) {
	return s.analyzer.FindShortestPath(ctx, sourceID, targetID)
}

// GetUpstreamServices 获取指定服务的所有上游服务（依赖当前服务的服务）。
//
// 该方法使用 DFS 遍历上游依赖链，返回指定深度内的所有上游服务。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 目标服务 ID
//   - depth: 遍历深度，<= 0 时使用服务默认配置
//
// 返回:
//   - []*ServiceNode: 上游服务列表
//   - error: 服务不存在时返回错误
func (s *TopologyService) GetUpstreamServices(ctx context.Context, serviceID uuid.UUID, depth int) ([]*domain.ServiceNode, error) {
	if depth <= 0 {
		depth = s.maxDepth
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	visited := make(map[uuid.UUID]bool)
	var nodes []*domain.ServiceNode

	s.traverseUpstream(serviceID, depth, visited, &nodes)
	return nodes, nil
}

// GetDownstreamServices 获取指定服务的所有下游服务（当前服务依赖的服务）。
//
// 该方法使用 DFS 遍历下游依赖链，返回指定深度内的所有下游服务。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 目标服务 ID
//   - depth: 遍历深度，<= 0 时使用服务默认配置
//
// 返回:
//   - []*ServiceNode: 下游服务列表
//   - error: 服务不存在时返回错误
func (s *TopologyService) GetDownstreamServices(ctx context.Context, serviceID uuid.UUID, depth int) ([]*domain.ServiceNode, error) {
	if depth <= 0 {
		depth = s.maxDepth
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	visited := make(map[uuid.UUID]bool)
	var nodes []*domain.ServiceNode

	s.traverseDownstream(serviceID, depth, visited, &nodes)
	return nodes, nil
}

// RefreshServiceTopology 刷新服务拓扑数据。
//
// 该方法从所有配置的 DiscoveryBackend 获取最新的服务节点和调用边数据，
// 合并多个数据源的结果后更新内存图。合并策略：
//   - 相同 namespace/name 的节点会被合并
//   - 指标数据（RequestRate、ErrorRate、LatencyP99）取非零值
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回: 发现失败时返回错误
func (s *TopologyService) RefreshServiceTopology(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allNodes []*domain.ServiceNode
	var allEdges []*domain.CallEdge

	for _, discoverer := range s.discoverers {
		nodes, err := discoverer.DiscoverNodes(ctx)
		if err != nil {
			log.Printf("[TopologyService] DiscoverNodes failed: %v", err)
			continue
		}
		allNodes = append(allNodes, nodes...)

		edges, err := discoverer.DiscoverEdges(ctx)
		if err != nil {
			log.Printf("[TopologyService] DiscoverEdges failed: %v", err)
			continue
		}
		allEdges = append(allEdges, edges...)
	}

	nodeMap := make(map[string]*domain.ServiceNode)
	for _, node := range allNodes {
		key := fmt.Sprintf("%s/%s", node.Namespace, node.Name)
		if existing, exists := nodeMap[key]; exists {
			if node.RequestRate > 0 && existing.RequestRate == 0 {
				existing.RequestRate = node.RequestRate
			}
			if node.ErrorRate > 0 && existing.ErrorRate == 0 {
				existing.ErrorRate = node.ErrorRate
			}
			if node.LatencyP99 > 0 && existing.LatencyP99 == 0 {
				existing.LatencyP99 = node.LatencyP99
			}
		} else {
			nodeMap[key] = node
		}
	}

	mergedNodes := make([]*domain.ServiceNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		mergedNodes = append(mergedNodes, node)
	}

	edgeMap := make(map[string]*domain.CallEdge)
	for _, edge := range allEdges {
		edgeKey := fmt.Sprintf("%s->%s", edge.SourceID, edge.TargetID)
		if existing, exists := edgeMap[edgeKey]; exists {
			if edge.IsDirect && !existing.IsDirect {
				existing.IsDirect = true
				existing.Confidence = edge.Confidence
			}
			if edge.RequestRate > existing.RequestRate {
				existing.RequestRate = edge.RequestRate
			}
		} else {
			edgeMap[edgeKey] = edge
		}
	}

	mergedEdges := make([]*domain.CallEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		mergedEdges = append(mergedEdges, edge)
	}

	s.serviceGraph.Rebuild(mergedNodes, mergedEdges)

	if s.repo != nil {
		if err := s.repo.BatchSaveServiceNodes(ctx, mergedNodes); err != nil {
			log.Printf("[TopologyService] Failed to save nodes: %v", err)
		}
		if err := s.repo.BatchSaveCallEdges(ctx, mergedEdges); err != nil {
			log.Printf("[TopologyService] Failed to save edges: %v", err)
		}
	}

	if s.cache != nil {
		topology := &domain.ServiceTopology{
			ID:        uuid.New(),
			Timestamp: time.Now(),
			Nodes:     mergedNodes,
			Edges:     mergedEdges,
		}
		_ = s.cache.SetServiceTopology(ctx, topology, s.cacheTTL)
		_ = s.cache.SetAdjacencyList(ctx, s.serviceGraph.GetAdjacencyList(), s.cacheTTL)
	}

	if s.localCache != nil {
		topology := &domain.ServiceTopology{
			ID:        uuid.New(),
			Timestamp: time.Now(),
			Nodes:     mergedNodes,
			Edges:     mergedEdges,
		}
		if data, err := json.Marshal(topology); err == nil {
			_ = s.localCache.Set(ctx, "topology:service", data, s.cacheTTL)
		}
		_ = s.localCache.Delete(ctx, "topology:stats")
	}

	return nil
}

// RefreshNetworkTopology 刷新网络拓扑数据。
//
// 该方法从所有配置的 DiscoveryBackend 获取最新的网络节点和边数据，
// 更新内存图并持久化到仓储。网络拓扑包括 Pod、Node、Service、Ingress
// 等网络实体及其连接关系。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回: 发现失败时返回错误
func (s *TopologyService) RefreshNetworkTopology(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allNodes []*domain.NetworkNode
	var allEdges []*domain.NetworkEdge

	for _, discoverer := range s.discoverers {
		nodes, err := discoverer.DiscoverNetworkNodes(ctx)
		if err != nil {
			log.Printf("[TopologyService] DiscoverNetworkNodes failed: %v", err)
			continue
		}
		allNodes = append(allNodes, nodes...)

		edges, err := discoverer.DiscoverNetworkEdges(ctx)
		if err != nil {
			log.Printf("[TopologyService] DiscoverNetworkEdges failed: %v", err)
			continue
		}
		allEdges = append(allEdges, edges...)
	}

	s.networkGraph.RebuildNetwork(allNodes, allEdges)

	if s.repo != nil {
		if err := s.repo.BatchSaveNetworkNodes(ctx, allNodes); err != nil {
			log.Printf("[TopologyService] Failed to save network nodes: %v", err)
		}
		if err := s.repo.BatchSaveNetworkEdges(ctx, allEdges); err != nil {
			log.Printf("[TopologyService] Failed to save network edges: %v", err)
		}
	}

	if s.cache != nil {
		topology := &domain.NetworkTopology{
			ID:        uuid.New(),
			Timestamp: time.Now(),
			Nodes:     allNodes,
			Edges:     allEdges,
		}
		_ = s.cache.SetNetworkTopology(ctx, topology, s.cacheTTL)
	}

	if s.localCache != nil {
		topology := &domain.NetworkTopology{
			ID:        uuid.New(),
			Timestamp: time.Now(),
			Nodes:     allNodes,
			Edges:     allEdges,
		}
		if data, err := json.Marshal(topology); err == nil {
			_ = s.localCache.Set(ctx, "topology:network", data, s.cacheTTL)
		}
		_ = s.localCache.Delete(ctx, "topology:stats")
	}

	return nil
}

// GetTopologyAtTime 获取指定时间点的历史拓扑快照。
//
// 该方法从持久化仓储查询历史拓扑数据，用于回溯分析。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - timestamp: 查询的时间点
//
// 返回:
//   - *ServiceTopology: 历史拓扑数据
//   - error: 仓储未配置或查询失败时返回错误
func (s *TopologyService) GetTopologyAtTime(ctx context.Context, timestamp time.Time) (*domain.ServiceTopology, error) {
	if s.repo == nil {
		return nil, domain.ErrGraphNotReady
	}
	return s.repo.GetTopologySnapshot(ctx, "service", timestamp)
}

// GetTopologyChanges 获取指定时间范围内的拓扑变更记录。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - from: 起始时间
//   - to: 结束时间
//
// 返回:
//   - []*TopologyChange: 变更记录列表
//   - error: 仓储未配置或查询失败时返回错误
func (s *TopologyService) GetTopologyChanges(ctx context.Context, from, to time.Time) ([]*domain.TopologyChange, error) {
	if s.repo == nil {
		return nil, domain.ErrGraphNotReady
	}
	return s.repo.ListTopologyChanges(ctx, from, to, "")
}

// FindAnomalies 检测当前拓扑中的异常服务。
//
// 该方法委托给 GraphAnalyzer 执行异常检测，检测不健康服务、
// 高错误率、高延迟等异常情况。
func (s *TopologyService) FindAnomalies(ctx context.Context) ([]*domain.TopologyAnomaly, error) {
	return s.analyzer.FindAnomalies(ctx)
}

// GetTopologyStats 获取拓扑统计信息。
//
// 返回当前内存图中的节点数、边数、最后发现时间等统计信息。
func (s *TopologyService) GetTopologyStats(ctx context.Context) (*domain.TopologyStats, error) {
	if s.localCache != nil {
		if data, err := s.localCache.Get(ctx, "topology:stats"); err == nil {
			var stats domain.TopologyStats
			if err := json.Unmarshal(data, &stats); err == nil {
				return &stats, nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &domain.TopologyStats{
		ServiceNodeCount:  s.serviceGraph.NodeCount(),
		ServiceEdgeCount:  s.serviceGraph.EdgeCount(),
		NetworkNodeCount:  s.networkGraph.NetworkNodeCount(),
		NetworkEdgeCount:  s.networkGraph.NetworkEdgeCount(),
		LastDiscoveryTime: s.serviceGraph.LastUpdated(),
		DataFreshness:     time.Since(s.serviceGraph.LastUpdated()),
	}

	if s.localCache != nil {
		if data, err := json.Marshal(stats); err == nil {
			_ = s.localCache.Set(ctx, "topology:stats", data, 30*time.Second)
		}
	}

	return stats, nil
}

// traverseUpstream 递归遍历上游服务。
//
// 该方法使用 DFS 算法遍历上游依赖链，收集所有上游服务节点。
func (s *TopologyService) traverseUpstream(serviceID uuid.UUID, depth int, visited map[uuid.UUID]bool, nodes *[]*domain.ServiceNode) {
	if depth <= 0 || visited[serviceID] {
		return
	}

	visited[serviceID] = true

	upstreamIDs := s.serviceGraph.GetUpstreamIDs(serviceID)
	for _, upstreamID := range upstreamIDs {
		node := s.serviceGraph.GetNode(upstreamID)
		if node != nil {
			*nodes = append(*nodes, node)
		}
		s.traverseUpstream(upstreamID, depth-1, visited, nodes)
	}
}

// traverseDownstream 递归遍历下游服务。
//
// 该方法使用 DFS 算法遍历下游依赖链，收集所有下游服务节点。
func (s *TopologyService) traverseDownstream(serviceID uuid.UUID, depth int, visited map[uuid.UUID]bool, nodes *[]*domain.ServiceNode) {
	if depth <= 0 || visited[serviceID] {
		return
	}

	visited[serviceID] = true

	downstreamIDs := s.serviceGraph.GetDownstreamIDs(serviceID)
	for _, downstreamID := range downstreamIDs {
		node := s.serviceGraph.GetNode(downstreamID)
		if node != nil {
			*nodes = append(*nodes, node)
		}
		s.traverseDownstream(downstreamID, depth-1, visited, nodes)
	}
}

func (s *TopologyService) filterNodesByNamespace(nodes []*domain.ServiceNode, namespace string) []*domain.ServiceNode {
	var filtered []*domain.ServiceNode
	for _, node := range nodes {
		if node.Namespace == namespace {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func (s *TopologyService) filterNodesByName(nodes []*domain.ServiceNode, name string) []*domain.ServiceNode {
	var filtered []*domain.ServiceNode
	for _, node := range nodes {
		if containsIgnoreCase(node.Name, name) {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func (s *TopologyService) filterEdgesByNamespace(edges []*domain.CallEdge, namespace string) []*domain.CallEdge {
	var filtered []*domain.CallEdge
	for _, edge := range edges {
		sourceNode := s.serviceGraph.GetNode(edge.SourceID)
		if sourceNode != nil && sourceNode.Namespace == namespace {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func (s *TopologyService) filterNetworkNodesByNamespace(nodes []*domain.NetworkNode, namespace string) []*domain.NetworkNode {
	var filtered []*domain.NetworkNode
	for _, node := range nodes {
		if node.Namespace == namespace {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func (s *TopologyService) computeTopologyHash(nodes []*domain.ServiceNode, edges []*domain.CallEdge) string {
	data, _ := json.Marshal(struct {
		Nodes []*domain.ServiceNode `json:"nodes"`
		Edges []*domain.CallEdge    `json:"edges"`
	}{Nodes: nodes, Edges: edges})

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)[:16]
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsLower(lower(s), lower(substr))))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

type InMemoryGraph struct {
	nodes      map[uuid.UUID]*domain.ServiceNode
	edges      map[uuid.UUID]*domain.CallEdge
	adjList    map[uuid.UUID][]uuid.UUID
	reverseAdj map[uuid.UUID][]uuid.UUID

	networkNodes map[uuid.UUID]*domain.NetworkNode
	networkEdges map[uuid.UUID]*domain.NetworkEdge

	version     int64
	lastUpdated time.Time
	mu          sync.RWMutex
}

func NewInMemoryGraph() *InMemoryGraph {
	return &InMemoryGraph{
		nodes:        make(map[uuid.UUID]*domain.ServiceNode),
		edges:        make(map[uuid.UUID]*domain.CallEdge),
		adjList:      make(map[uuid.UUID][]uuid.UUID),
		reverseAdj:   make(map[uuid.UUID][]uuid.UUID),
		networkNodes: make(map[uuid.UUID]*domain.NetworkNode),
		networkEdges: make(map[uuid.UUID]*domain.NetworkEdge),
	}
}

func (g *InMemoryGraph) Rebuild(nodes []*domain.ServiceNode, edges []*domain.CallEdge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[uuid.UUID]*domain.ServiceNode)
	g.edges = make(map[uuid.UUID]*domain.CallEdge)
	g.adjList = make(map[uuid.UUID][]uuid.UUID)
	g.reverseAdj = make(map[uuid.UUID][]uuid.UUID)

	for _, node := range nodes {
		g.nodes[node.ID] = node
	}

	for _, edge := range edges {
		g.edges[edge.ID] = edge
		g.adjList[edge.SourceID] = append(g.adjList[edge.SourceID], edge.TargetID)
		g.reverseAdj[edge.TargetID] = append(g.reverseAdj[edge.TargetID], edge.SourceID)
	}

	g.version++
	g.lastUpdated = time.Now()
}

func (g *InMemoryGraph) RebuildNetwork(nodes []*domain.NetworkNode, edges []*domain.NetworkEdge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.networkNodes = make(map[uuid.UUID]*domain.NetworkNode)
	g.networkEdges = make(map[uuid.UUID]*domain.NetworkEdge)

	for _, node := range nodes {
		g.networkNodes[node.ID] = node
	}

	for _, edge := range edges {
		g.networkEdges[edge.ID] = edge
	}

	g.version++
	g.lastUpdated = time.Now()
}

func (g *InMemoryGraph) GetNode(id uuid.UUID) *domain.ServiceNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

func (g *InMemoryGraph) GetAllNodes() []*domain.ServiceNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*domain.ServiceNode, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *InMemoryGraph) GetAllEdges() []*domain.CallEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*domain.CallEdge, 0, len(g.edges))
	for _, edge := range g.edges {
		edges = append(edges, edge)
	}
	return edges
}

func (g *InMemoryGraph) GetUpstreamIDs(id uuid.UUID) []uuid.UUID {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.reverseAdj[id]
	if ids == nil {
		return nil
	}
	result := make([]uuid.UUID, len(ids))
	copy(result, ids)
	return result
}

func (g *InMemoryGraph) GetDownstreamIDs(id uuid.UUID) []uuid.UUID {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ids := g.adjList[id]
	if ids == nil {
		return nil
	}
	result := make([]uuid.UUID, len(ids))
	copy(result, ids)
	return result
}

func (g *InMemoryGraph) GetAdjacencyList() map[uuid.UUID][]uuid.UUID {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[uuid.UUID][]uuid.UUID)
	for k, v := range g.adjList {
		result[k] = append([]uuid.UUID{}, v...)
	}
	return result
}

func (g *InMemoryGraph) GetNetworkNode(id uuid.UUID) *domain.NetworkNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.networkNodes[id]
}

func (g *InMemoryGraph) GetAllNetworkNodes() []*domain.NetworkNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*domain.NetworkNode, 0, len(g.networkNodes))
	for _, node := range g.networkNodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (g *InMemoryGraph) GetAllNetworkEdges() []*domain.NetworkEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*domain.NetworkEdge, 0, len(g.networkEdges))
	for _, edge := range g.networkEdges {
		edges = append(edges, edge)
	}
	return edges
}

func (g *InMemoryGraph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

func (g *InMemoryGraph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}

func (g *InMemoryGraph) NetworkNodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.networkNodes)
}

func (g *InMemoryGraph) NetworkEdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.networkEdges)
}

func (g *InMemoryGraph) IsEmpty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes) == 0
}

func (g *InMemoryGraph) LastUpdated() time.Time {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastUpdated
}

// GetEdge 获取两个节点之间的边
//
// TODO: 实现边查询
// 提示：遍历 edges，找到 SourceID == sourceID && TargetID == targetID 的边
func (g *InMemoryGraph) GetEdge(sourceID, targetID uuid.UUID) *domain.CallEdge {
	// TODO: 实现边查询
	// 骨架代码：
	// g.mu.RLock()
	// defer g.mu.RUnlock()
	// for _, edge := range g.edges {
	//     if edge.SourceID == sourceID && edge.TargetID == targetID {
	//         return edge
	//     }
	// }
	// return nil
	return nil
}

// IsNetworkEmpty 检查网络图是否为空
//
// TODO: 实现网络图空检查
// 注意：当前 IsEmpty() 检查的是服务节点，这里应该检查网络节点
func (g *InMemoryGraph) IsNetworkEmpty() bool {
	// TODO: 实现网络图空检查
	// 骨架代码：
	// g.mu.RLock()
	// defer g.mu.RUnlock()
	// return len(g.networkNodes) == 0
	return true
}
