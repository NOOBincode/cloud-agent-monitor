// Package application 实现拓扑模块的应用层业务逻辑。
//
// 该包包含拓扑服务的核心业务逻辑，负责协调领域对象完成
// 拓扑发现、影响分析、路径查找等核心功能。应用层通过依赖注入
// 使用基础设施层提供的服务，保持领域模型的纯粹性。
//
// 核心组件:
//   - TopologyService: 拓扑服务主入口，协调发现、缓存和持久化
//   - GraphAnalyzer: 图算法分析器，提供影响分析和路径查找
//   - ImpactCacheService: 影响分析结果预计算服务
//   - InMemoryGraph: 内存图结构，支持高效的拓扑遍历
package application

import (
	"container/heap"
	"context"
	"math"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
)

// 异常检测阈值常量。
// 这些阈值用于判断服务是否存在异常，可根据实际业务需求调整。
const (
	// HighErrorRateThreshold 高错误率阈值，超过此值判定为异常
	HighErrorRateThreshold = 0.05
	// HighLatencyThresholdMs 高延迟阈值（毫秒），P99 超过此值判定为异常
	HighLatencyThresholdMs = 1000.0
	// DefaultAnomalyMaxDepth 异常检测的默认遍历深度
	DefaultAnomalyMaxDepth = 5
	// DefaultPathMaxHops 路径查找的默认最大跳数
	DefaultPathMaxHops = 10
)

// GraphAnalyzer 提供基于内存图的拓扑分析功能。
//
// GraphAnalyzer 封装了服务调用图的各种分析算法，包括：
//   - 影响分析：BFS 遍历上下游服务，计算影响范围
//   - 路径查找：DFS 查找所有路径，Dijkstra 查找最短路径
//   - 异常检测：检测不健康服务、高错误率、高延迟等异常
//   - 中心性分析：计算节点重要性指标
//
// 所有分析方法都是只读的，不会修改图结构。
type GraphAnalyzer struct {
	graph *InMemoryGraph
}

// NewGraphAnalyzer 创建一个新的图分析器。
//
// 参数:
//   - graph: 内存图实例，必须已初始化
func NewGraphAnalyzer(graph *InMemoryGraph) *GraphAnalyzer {
	return &GraphAnalyzer{
		graph: graph,
	}
}

// AnalyzeImpact 分析指定服务故障的影响范围。
//
// 该方法使用 BFS 算法遍历服务的上下游依赖，计算故障可能影响的
// 所有服务。影响分数基于服务状态和调用深度计算：
//   - 深度越近，影响越大（1/depth 权重）
//   - 不健康服务权重 2.0，警告状态权重 1.5
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 待分析的服务 ID
//   - maxDepth: 最大遍历深度，防止无限遍历
//
// 返回:
//   - *ImpactResult: 影响分析结果，包含上下游服务列表
//   - error: 服务不存在或图未就绪时返回错误
//
// 时间复杂度: O(V + E)，其中 V 为节点数，E 为边数
func (a *GraphAnalyzer) AnalyzeImpact(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.ImpactResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(serviceID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	result := &domain.ImpactResult{
		RootService:     node,
		UpstreamDepth:   0,
		DownstreamDepth: 0,
		Upstream:        make([]*domain.ImpactNode, 0),
		Downstream:      make([]*domain.ImpactNode, 0),
		CriticalPath:    make([]domain.PathHop, 0),
		AnalyzedAt:      time.Now(),
	}

	upstreamVisited := make(map[uuid.UUID]int)
	a.bfsUpstream(serviceID, maxDepth, upstreamVisited)

	for id, depth := range upstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n != nil {
			result.Upstream = append(result.Upstream, &domain.ImpactNode{
				Node:       n,
				Depth:      depth,
				Impact:     a.calculateImpactScore(n, depth),
				IsCritical: depth <= 2,
			})
			if depth > result.UpstreamDepth {
				result.UpstreamDepth = depth
			}
		}
	}

	downstreamVisited := make(map[uuid.UUID]int)
	a.bfsDownstream(serviceID, maxDepth, downstreamVisited)

	for id, depth := range downstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n != nil {
			result.Downstream = append(result.Downstream, &domain.ImpactNode{
				Node:       n,
				Depth:      depth,
				Impact:     a.calculateImpactScore(n, depth),
				IsCritical: depth <= 2,
			})
			if depth > result.DownstreamDepth {
				result.DownstreamDepth = depth
			}
		}
	}

	result.TotalAffected = len(result.Upstream) + len(result.Downstream)

	return result, nil
}

// bfsUpstream 使用 BFS 算法向上游遍历（查找依赖当前服务的服务）。
func (a *GraphAnalyzer) bfsUpstream(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int) {
	a.bfs(start, maxDepth, visited, a.graph.GetUpstreamIDs)
}

// bfsDownstream 使用 BFS 算法向下游遍历（查找当前服务依赖的服务）。
func (a *GraphAnalyzer) bfsDownstream(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int) {
	a.bfs(start, maxDepth, visited, a.graph.GetDownstreamIDs)
}

// bfs 是通用的广度优先搜索实现。
//
// 该方法实现了标准的 BFS 算法，通过 getNeighbors 函数获取相邻节点，
// 支持向上游或下游遍历。遍历结果存储在 visited map 中，key 为节点 ID，
// value 为该节点距离起点的深度。
//
// 参数:
//   - start: 起始节点 ID
//   - maxDepth: 最大遍历深度，超过此深度的节点将被忽略
//   - visited: 已访问节点记录，方法会修改此 map
//   - getNeighbors: 获取相邻节点的函数，决定遍历方向
//
// 算法说明:
//   - 使用队列实现层次遍历
//   - 已访问节点不会重复处理
//   - 时间复杂度 O(V + E)，空间复杂度 O(V)
func (a *GraphAnalyzer) bfs(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int, getNeighbors func(uuid.UUID) []uuid.UUID) {
	queue := []struct {
		id    uuid.UUID
		depth int
	}{{id: start, depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > maxDepth {
			continue
		}

		if _, seen := visited[current.id]; seen {
			continue
		}
		visited[current.id] = current.depth

		neighborIDs := getNeighbors(current.id)
		for _, neighborID := range neighborIDs {
			if _, seen := visited[neighborID]; !seen {
				queue = append(queue, struct {
					id    uuid.UUID
					depth int
				}{id: neighborID, depth: current.depth + 1})
			}
		}
	}
}

// calculateImpactScore 计算服务的影响分数。
//
// 影响分数综合考虑三个因素：
//  1. 调用深度：深度越近影响越大，基础分数为 1/depth
//  2. 服务状态：不健康服务权重 2.0，警告状态权重 1.5
//  3. 流量大小：高 QPS 服务权重更高
//
// 分数范围：0 < score <= 3.0
func (a *GraphAnalyzer) calculateImpactScore(node *domain.ServiceNode, depth int) float64 {
	baseScore := 1.0 / float64(depth)

	statusMultiplier := 1.0
	switch node.Status {
	case domain.ServiceStatusUnhealthy:
		statusMultiplier = 2.0
	case domain.ServiceStatusWarning:
		statusMultiplier = 1.5
	case domain.ServiceStatusHealthy:
		statusMultiplier = 1.0
	default:
		statusMultiplier = 1.0
	}

	trafficMultiplier := 1.0
	if node.RequestRate > 1000 {
		trafficMultiplier = 1.5
	} else if node.RequestRate > 100 {
		trafficMultiplier = 1.2
	}

	return baseScore * statusMultiplier * trafficMultiplier
}

// FindPath 查找两个服务之间的所有调用路径。
//
// 该方法使用 DFS 算法查找从源服务到目标服务的所有可能路径，
// 路径数量受 maxHops 限制。适用于需要了解完整调用链的场景。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - sourceID: 源服务 ID（调用发起方）
//   - targetID: 目标服务 ID（调用接收方）
//   - maxHops: 路径最大跳数限制
//
// 返回:
//   - *PathResult: 路径查找结果，包含所有找到的路径
//   - error: 服务不存在或路径不存在时返回错误
func (a *GraphAnalyzer) FindPath(ctx context.Context, sourceID, targetID uuid.UUID, maxHops int) (*domain.PathResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil {
		return nil, domain.ErrNodeNotFound
	}
	if a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	paths := a.findAllPaths(sourceID, targetID, maxHops)
	if len(paths) == 0 {
		return nil, domain.ErrPathNotFound
	}

	result := &domain.PathResult{
		SourceID: sourceID,
		TargetID: targetID,
		Paths:    paths,
		FoundAt:  time.Now(),
	}

	if len(paths) > 0 {
		result.ShortestPath = paths[0]
	}

	return result, nil
}

// findAllPaths 使用 DFS 算法查找所有可能的路径。
//
// 该方法递归遍历图结构，收集从源节点到目标节点的所有路径。
// 使用回溯算法避免重复访问节点，同时支持路径长度限制。
//
// 参数:
//   - sourceID: 源节点 ID
//   - targetID: 目标节点 ID
//   - maxHops: 路径最大跳数
//
// 返回: 所有找到的路径列表
func (a *GraphAnalyzer) findAllPaths(sourceID, targetID uuid.UUID, maxHops int) [][]domain.PathHop {
	var paths [][]domain.PathHop
	visited := make(map[uuid.UUID]bool)
	currentPath := make([]domain.PathHop, 0)

	a.dfsPaths(sourceID, targetID, maxHops, visited, &currentPath, &paths)

	return paths
}

// dfsPaths 是 findAllPaths 的递归 DFS 实现。
//
// 该方法使用回溯算法遍历图，当到达目标节点时记录路径。
// 注意：该方法会修改 visited 和 currentPath 参数。
func (a *GraphAnalyzer) dfsPaths(
	current, target uuid.UUID,
	maxHops int,
	visited map[uuid.UUID]bool,
	currentPath *[]domain.PathHop,
	paths *[][]domain.PathHop,
) {
	if len(*currentPath) > maxHops {
		return
	}

	if visited[current] {
		return
	}

	node := a.graph.GetNode(current)
	if node == nil {
		return
	}

	visited[current] = true
	*currentPath = append(*currentPath, domain.PathHop{
		NodeID:    current,
		NodeName:  node.Name,
		Namespace: node.Namespace,
	})

	if current == target {
		pathCopy := make([]domain.PathHop, len(*currentPath))
		copy(pathCopy, *currentPath)
		*paths = append(*paths, pathCopy)
	} else {
		downstreamIDs := a.graph.GetDownstreamIDs(current)
		for _, nextID := range downstreamIDs {
			a.dfsPaths(nextID, target, maxHops, visited, currentPath, paths)
		}
	}

	*currentPath = (*currentPath)[:len(*currentPath)-1]
	delete(visited, current)
}

// FindShortestPath 使用 BFS 算法查找两个服务之间的最短路径。
//
// 最短路径定义为跳数最少的路径。该方法使用 BFS 保证找到的第一条路径
// 就是最短路径。适用于需要快速定位调用链的场景。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - sourceID: 源服务 ID
//   - targetID: 目标服务 ID
//
// 返回:
//   - []PathHop: 最短路径的节点序列
//   - error: 服务不存在或路径不存在时返回错误
//
// 时间复杂度: O(V + E)
func (a *GraphAnalyzer) FindShortestPath(ctx context.Context, sourceID, targetID uuid.UUID) ([]domain.PathHop, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil {
		return nil, domain.ErrNodeNotFound
	}
	if a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	parent := make(map[uuid.UUID]uuid.UUID)
	visited := make(map[uuid.UUID]bool)
	queue := []uuid.UUID{sourceID}
	visited[sourceID] = true

	found := false
	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]

		if current == targetID {
			found = true
			break
		}

		downstreamIDs := a.graph.GetDownstreamIDs(current)
		for _, nextID := range downstreamIDs {
			if !visited[nextID] {
				visited[nextID] = true
				parent[nextID] = current
				queue = append(queue, nextID)
			}
		}
	}

	if !found {
		return nil, domain.ErrPathNotFound
	}

	var path []domain.PathHop
	current := targetID
	for current != sourceID {
		node := a.graph.GetNode(current)
		if node != nil {
			path = append([]domain.PathHop{{
				NodeID:    current,
				NodeName:  node.Name,
				Namespace: node.Namespace,
			}}, path...)
		}
		current = parent[current]
	}

	sourceNode := a.graph.GetNode(sourceID)
	if sourceNode != nil {
		path = append([]domain.PathHop{{
			NodeID:    sourceID,
			NodeName:  sourceNode.Name,
			Namespace: sourceNode.Namespace,
		}}, path...)
	}

	return path, nil
}

// FindAnomalies 检测拓扑中的异常服务。
//
// 该方法遍历所有服务节点，检测以下类型的异常：
//   - unhealthy_service: 服务状态为不健康
//   - high_error_rate: 错误率超过 HighErrorRateThreshold (默认 5%)
//   - high_latency: P99 延迟超过 HighLatencyThresholdMs (默认 1000ms)
//   - pod_degradation: Pod 数量下降（部分 Pod 不健康）
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*TopologyAnomaly: 检测到的异常列表
//   - error: 图未就绪时返回错误
func (a *GraphAnalyzer) FindAnomalies(ctx context.Context) ([]*domain.TopologyAnomaly, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	var anomalies []*domain.TopologyAnomaly

	nodes := a.graph.GetAllNodes()
	for _, node := range nodes {
		if node.Status == domain.ServiceStatusUnhealthy {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "unhealthy_service",
				Severity:    "high",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service is in unhealthy state",
				DetectedAt:  time.Now(),
			})
		}

		if node.ErrorRate > HighErrorRateThreshold {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "high_error_rate",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has high error rate",
				Metrics: map[string]float64{
					"error_rate": node.ErrorRate,
				},
				DetectedAt: time.Now(),
			})
		}

		if node.LatencyP99 > HighLatencyThresholdMs {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "high_latency",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has high P99 latency",
				Metrics: map[string]float64{
					"latency_p99": node.LatencyP99,
				},
				DetectedAt: time.Now(),
			})
		}

		if node.PodCount > 0 && node.ReadyPods < node.PodCount {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "pod_degradation",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Not all pods are ready",
				Metrics: map[string]float64{
					"pod_count":  float64(node.PodCount),
					"ready_pods": float64(node.ReadyPods),
				},
				DetectedAt: time.Now(),
			})
		}
	}

	cycles := a.detectCycles()
	for _, cycle := range cycles {
		if len(cycle) > 0 {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "circular_dependency",
				Severity:    "high",
				NodeID:      cycle[0],
				NodeName:    "",
				Namespace:   "",
				Description: "Circular dependency detected in service topology",
				DetectedAt:  time.Now(),
			})
		}
	}

	orphanNodes := a.findOrphanNodes()
	for _, nodeID := range orphanNodes {
		node := a.graph.GetNode(nodeID)
		if node != nil {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "orphan_service",
				Severity:    "low",
				NodeID:      nodeID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has no incoming or outgoing connections",
				DetectedAt:  time.Now(),
			})
		}
	}

	return anomalies, nil
}

func (a *GraphAnalyzer) detectCycles() [][]uuid.UUID {
	var cycles [][]uuid.UUID
	visited := make(map[uuid.UUID]bool)
	recStack := make(map[uuid.UUID]bool)
	var path []uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		if !visited[node.ID] {
			a.dfsCycle(node.ID, visited, recStack, &path, &cycles)
		}
	}

	return cycles
}

func (a *GraphAnalyzer) dfsCycle(
	current uuid.UUID,
	visited, recStack map[uuid.UUID]bool,
	path *[]uuid.UUID,
	cycles *[][]uuid.UUID,
) {
	visited[current] = true
	recStack[current] = true
	*path = append(*path, current)

	downstreamIDs := a.graph.GetDownstreamIDs(current)
	for _, nextID := range downstreamIDs {
		if !visited[nextID] {
			a.dfsCycle(nextID, visited, recStack, path, cycles)
		} else if recStack[nextID] {
			cycleStart := -1
			for i, id := range *path {
				if id == nextID {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]uuid.UUID, len(*path)-cycleStart)
				copy(cycle, (*path)[cycleStart:])
				*cycles = append(*cycles, cycle)
			}
		}
	}

	*path = (*path)[:len(*path)-1]
	recStack[current] = false
}

func (a *GraphAnalyzer) findOrphanNodes() []uuid.UUID {
	var orphans []uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		upstream := a.graph.GetUpstreamIDs(node.ID)
		downstream := a.graph.GetDownstreamIDs(node.ID)

		if len(upstream) == 0 && len(downstream) == 0 {
			orphans = append(orphans, node.ID)
		}
	}

	return orphans
}

func (a *GraphAnalyzer) CalculateCentrality(ctx context.Context) (map[uuid.UUID]float64, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	centrality := make(map[uuid.UUID]float64)
	nodes := a.graph.GetAllNodes()

	for _, node := range nodes {
		centrality[node.ID] = a.calculateBetweennessCentrality(node.ID)
	}

	return centrality, nil
}

func (a *GraphAnalyzer) calculateBetweennessCentrality(nodeID uuid.UUID) float64 {
	totalPaths := 0
	pathsThroughNode := 0

	nodes := a.graph.GetAllNodes()
	for i, source := range nodes {
		for j, target := range nodes {
			if i >= j {
				continue
			}

			paths := a.findAllPaths(source.ID, target.ID, 5)
			for _, path := range paths {
				totalPaths++
				for _, hop := range path {
					if hop.NodeID == nodeID {
						pathsThroughNode++
						break
					}
				}
			}
		}
	}

	if totalPaths == 0 {
		return 0
	}

	return float64(pathsThroughNode) / float64(totalPaths)
}

func (a *GraphAnalyzer) FindClusters(ctx context.Context) ([][]uuid.UUID, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	visited := make(map[uuid.UUID]bool)
	var clusters [][]uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		if !visited[node.ID] {
			cluster := a.findConnectedComponent(node.ID, visited)
			if len(cluster) > 1 {
				clusters = append(clusters, cluster)
			}
		}
	}

	return clusters, nil
}

func (a *GraphAnalyzer) findConnectedComponent(start uuid.UUID, visited map[uuid.UUID]bool) []uuid.UUID {
	var component []uuid.UUID
	queue := []uuid.UUID{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}

		visited[current] = true
		component = append(component, current)

		for _, neighbor := range a.graph.GetDownstreamIDs(current) {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}

		for _, neighbor := range a.graph.GetUpstreamIDs(current) {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return component
}

type Path struct {
	Nodes []uuid.UUID
	Cost  float64
}

type PriorityQueue []*Path

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Cost < pq[j].Cost
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*Path))
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (a *GraphAnalyzer) FindShortestPathWeighted(ctx context.Context, sourceID, targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) ([]uuid.UUID, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil || a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	if weightFunc == nil {
		weightFunc = func(edge *domain.CallEdge) float64 {
			return 1.0
		}
	}

	dist := make(map[uuid.UUID]float64)
	prev := make(map[uuid.UUID]uuid.UUID)

	for _, node := range a.graph.GetAllNodes() {
		dist[node.ID] = math.Inf(1)
	}
	dist[sourceID] = 0

	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	heap.Push(&pq, &Path{Nodes: []uuid.UUID{sourceID}, Cost: 0})

	visited := make(map[uuid.UUID]bool)

	for pq.Len() > 0 {
		current := heap.Pop(&pq).(*Path)
		currentNode := current.Nodes[len(current.Nodes)-1]

		if visited[currentNode] {
			continue
		}
		visited[currentNode] = true

		if currentNode == targetID {
			return current.Nodes, nil
		}

		edges := a.graph.GetAllEdges()
		for _, edge := range edges {
			if edge.SourceID == currentNode {
				weight := weightFunc(edge)
				newDist := dist[currentNode] + weight

				if newDist < dist[edge.TargetID] {
					dist[edge.TargetID] = newDist
					prev[edge.TargetID] = currentNode

					newPath := make([]uuid.UUID, len(current.Nodes)+1)
					copy(newPath, current.Nodes)
					newPath[len(newPath)-1] = edge.TargetID

					heap.Push(&pq, &Path{Nodes: newPath, Cost: newDist})
				}
			}
		}
	}

	return nil, domain.ErrPathNotFound
}

// ============ 加权图算法扩展 ============

// AnalyzeImpactWeighted 分析加权影响范围
//
// TODO: 实现加权影响分析
// 与普通影响分析的区别：
// - 考虑服务重要性权重
// - 计算加权影响分数
// - 识别关键受影响服务
//
// 加权分数计算：
// weighted_score = sum(impact * node_weight / depth) for all affected nodes
func (a *GraphAnalyzer) AnalyzeImpactWeighted(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.WeightedImpactResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(serviceID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	// TODO: 实现加权影响分析
	// 骨架代码：
	// // 1. 使用 BFS 遍历上下游
	// upstreamVisited := make(map[uuid.UUID]int)
	// downstreamVisited := make(map[uuid.UUID]int)
	//
	// a.bfsUpstream(serviceID, maxDepth, upstreamVisited)
	// a.bfsDownstream(serviceID, maxDepth, downstreamVisited)
	//
	// // 2. 构建影响结果
	// result := &domain.WeightedImpactResult{
	//     ImpactResult: &domain.ImpactResult{
	//         RootService: node,
	//         Upstream:    make([]*domain.ImpactNode, 0),
	//         Downstream:  make([]*domain.ImpactNode, 0),
	//         AnalyzedAt:  time.Now(),
	//     },
	//     CriticalServices:    make([]*domain.ImpactNode, 0),
	//     ImpactByImportance:  make(map[domain.ServiceImportance]int),
	// }
	//
	// // 3. 计算加权分数
	// var weightedScore float64
	//
	// for id, depth := range upstreamVisited {
	//     if id == serviceID {
	//         continue
	//     }
	//     n := a.graph.GetNode(id)
	//     if n == nil {
	//         continue
	//     }
	//
	//     nodeWeight := n.GetEffectiveWeight()
	//     impact := nodeWeight / float64(depth)
	//     weightedScore += impact
	//
	//     impactNode := &domain.ImpactNode{
	//         Node:       n,
	//         Depth:      depth,
	//         Impact:     impact,
	//         IsCritical: n.Importance == domain.ImportanceCritical || n.Importance == domain.ImportanceImportant,
	//     }
	//
	//     result.Upstream = append(result.Upstream, impactNode)
	//     result.ImpactByImportance[n.Importance]++
	//
	//     if impactNode.IsCritical {
	//         result.CriticalServices = append(result.CriticalServices, impactNode)
	//     }
	// }
	//
	// // 4. 同样处理下游
	// // ...
	//
	// // 5. 归一化分数到 0-100
	// result.WeightedScore = math.Min(weightedScore*10, 100)
	// result.TotalAffected = len(result.Upstream) + len(result.Downstream)
	//
	// return result, nil

	return nil, domain.ErrGraphNotReady
}

// FindKShortestPaths 查找 K 条最短路径（Yen's 算法）
//
// TODO: 实现 Yen's K 最短路径算法
// 算法步骤：
// 1. 使用 Dijkstra 找到最短路径 P1
// 2. 对于每条已找到的路径 Pi，生成候选路径：
//   - 依次移除 Pi 中的每条边
//   - 使用 Dijkstra 找到新的最短路径
//   - 将候选路径加入优先队列
//
// 3. 从候选队列中选择最短的作为下一条路径
// 4. 重复直到找到 K 条路径或队列为空
//
// 学习要点：
// - Yen's 算法：在 Dijkstra 基础上扩展，找多条路径
// - 候选路径生成：通过"偏离"已有路径生成新路径
// - 应用场景：需要备选路径的容灾场景
func (a *GraphAnalyzer) FindKShortestPaths(ctx context.Context, sourceID, targetID uuid.UUID, k int, weightFunc func(edge *domain.CallEdge) float64) ([]*domain.WeightedPathResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	// TODO: 实现 Yen's K 最短路径算法
	// 骨架代码：
	// var results []*domain.WeightedPathResult
	//
	// // 1. 找到最短路径
	// nodes, err := a.FindShortestPathWeighted(ctx, sourceID, targetID, weightFunc)
	// if err != nil {
	//     return nil, err
	// }
	//
	// // 2. 转换为 WeightedPathResult
	// // ...
	//
	// // 3. 候选路径队列
	// // ...
	//
	// return results, nil

	return nil, domain.ErrPathNotFound
}

// CalculateCentralityDetailed 计算节点详细中心性指标
//
// TODO: 实现详细中心性计算
// 中心性指标：
// - 度中心性：直接连接数
// - 接近中心性：到其他节点的平均距离
// - 介数中心性：经过该节点的最短路径数量
// - 加权中心性：考虑边权重的中心性
func (a *GraphAnalyzer) CalculateCentralityDetailed(ctx context.Context, nodeID uuid.UUID) (*CentralityResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(nodeID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	// TODO: 实现详细中心性计算
	// 骨架代码：
	// result := &CentralityResult{
	//     NodeID: nodeID,
	// }
	//
	// // 度中心性
	// upstream := a.graph.GetUpstreamIDs(nodeID)
	// downstream := a.graph.GetDownstreamIDs(nodeID)
	// result.DegreeCentrality = float64(len(upstream) + len(downstream))
	//
	// // 接近中心性（需要计算到所有节点的最短路径）
	// // ...
	//
	// // 介数中心性（使用已有的 calculateBetweennessCentrality）
	// result.BetweennessCentrality = a.calculateBetweennessCentrality(nodeID)
	//
	// // 加权中心性
	// result.WeightedCentrality = result.DegreeCentrality * node.GetEffectiveWeight()
	//
	// return result, nil

	return nil, domain.ErrGraphNotReady
}

// CentralityResult 中心性计算结果
type CentralityResult struct {
	NodeID                uuid.UUID `json:"node_id"`
	DegreeCentrality      float64   `json:"degree_centrality"`
	ClosenessCentrality   float64   `json:"closeness_centrality"`
	BetweennessCentrality float64   `json:"betweenness_centrality"`
	WeightedCentrality    float64   `json:"weighted_centrality"`
}

// FindCriticalPath 查找关键路径
//
// TODO: 实现关键路径查找
// 关键路径：影响分数最高的路径
// 用于识别故障传播的主要路径
func (a *GraphAnalyzer) FindCriticalPath(ctx context.Context, serviceID uuid.UUID, maxDepth int) ([]domain.PathHop, float64, error) {
	// TODO: 实现关键路径查找
	// 提示：
	// 1. 分析影响范围
	// 2. 找到影响分数最高的路径
	// 3. 考虑服务重要性权重

	return nil, 0, domain.ErrGraphNotReady
}

// DetectBottlenecks 检测瓶颈节点
//
// TODO: 实现瓶颈检测
// 瓶颈节点特征：
// - 高介数中心性：很多最短路径经过
// - 高流量：RequestRate 高
// - 高延迟：LatencyP99 高
// - 高错误率：ErrorRate 高
func (a *GraphAnalyzer) DetectBottlenecks(ctx context.Context) ([]*BottleneckNode, error) {
	// TODO: 实现瓶颈检测
	// 骨架代码：
	// var bottlenecks []*BottleneckNode
	//
	// nodes := a.graph.GetAllNodes()
	// for _, node := range nodes {
	//     centrality, _ := a.CalculateCentralityDetailed(ctx, node.ID)
	//
	//     // 计算瓶颈分数
	//     score := centrality.WeightedCentrality * (1 + node.ErrorRate) * (1 + node.LatencyP99/1000)
	//
	//     if score > 1.0 { // 阈值
	//         bottlenecks = append(bottlenecks, &BottleneckNode{
	//             Node:    node,
	//             Score:   score,
	//             Reasons: a.identifyBottleneckReasons(node, centrality),
	//         })
	//     }
	// }
	//
	// return bottlenecks, nil

	return nil, domain.ErrGraphNotReady
}

// BottleneckNode 瓶颈节点
type BottleneckNode struct {
	Node    *domain.ServiceNode `json:"node"`
	Score   float64             `json:"score"`
	Reasons []string            `json:"reasons"`
}

// identifyBottleneckReasons 识别瓶颈原因
//
// TODO: 实现瓶颈原因识别
func (a *GraphAnalyzer) identifyBottleneckReasons(node *domain.ServiceNode, centrality *CentralityResult) []string {
	// TODO: 实现瓶颈原因识别
	// 骨架代码：
	// var reasons []string
	//
	// if centrality.BetweennessCentrality > 10 {
	//     reasons = append(reasons, "high_betweenness")
	// }
	// if node.RequestRate > 1000 {
	//     reasons = append(reasons, "high_traffic")
	// }
	// if node.LatencyP99 > 500 {
	//     reasons = append(reasons, "high_latency")
	// }
	// if node.ErrorRate > 0.01 {
	//     reasons = append(reasons, "high_error_rate")
	// }
	//
	// return reasons

	return nil
}

// ============ A* 算法扩展 ============

// FindShortestPathAStar 使用 A* 算法查找最短路径
//
// TODO: 实现 A* 算法
// A* 算法使用启发式函数加速搜索
// f(n) = g(n) + h(n)
// - g(n): 从源节点到 n 的实际代价
// - h(n): 从 n 到目标节点的估计代价（启发式函数）
//
// 启发式函数选择：
// - 欧几里得距离（如果有坐标）
// - 最小边权重 * 跳数估计
// - 基于历史数据的估计
func (a *GraphAnalyzer) FindShortestPathAStar(ctx context.Context, sourceID, targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64, heuristic func(uuid.UUID) float64) ([]uuid.UUID, error) {
	// TODO: 实现 A* 算法
	// 骨架代码：
	// if a.graph == nil {
	//     return nil, domain.ErrGraphNotReady
	// }
	//
	// if a.graph.GetNode(sourceID) == nil || a.graph.GetNode(targetID) == nil {
	//     return nil, domain.ErrNodeNotFound
	// }
	//
	// // 初始化
	// gScore := make(map[uuid.UUID]float64)
	// fScore := make(map[uuid.UUID]float64)
	// parent := make(map[uuid.UUID]uuid.UUID)
	//
	// for _, node := range a.graph.GetAllNodes() {
	//     gScore[node.ID] = math.Inf(1)
	//     fScore[node.ID] = math.Inf(1)
	// }
	//
	// gScore[sourceID] = 0
	// fScore[sourceID] = heuristic(sourceID)
	//
	// // 开放列表
	// openSet := &PriorityQueue{}
	// heap.Init(openSet)
	// heap.Push(openSet, &Path{Nodes: []uuid.UUID{sourceID}, Cost: fScore[sourceID]})
	//
	// // A* 主循环
	// for openSet.Len() > 0 {
	//     current := heap.Pop(openSet).(*Path)
	//     currentNode := current.Nodes[len(current.Nodes)-1]
	//
	//     if currentNode == targetID {
	//         return current.Nodes, nil
	//     }
	//
	//     // 遍历邻居
	//     // ...
	// }
	//
	// return nil, domain.ErrPathNotFound

	return nil, domain.ErrPathNotFound
}

// DefaultHeuristic 默认启发式函数
//
// TODO: 实现默认启发式函数
// 使用最小边权重 * 估计跳数
func (a *GraphAnalyzer) DefaultHeuristic(targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) func(uuid.UUID) float64 {
	return func(nodeID uuid.UUID) float64 {
		// TODO: 实现启发式函数
		// 骨架代码：
		// // 使用最小边权重作为估计
		// minWeight := 1.0
		// edges := a.graph.GetAllEdges()
		// for _, edge := range edges {
		//     w := weightFunc(edge)
		//     if w < minWeight {
		//         minWeight = w
		//     }
		// }
		// // 假设最少需要 1 跳
		// return minWeight

		return 1.0
	}
}

// ============ 时间相关扩展 ============

// TimeDependentWeight 时间相关权重
// 不同时间段权重可能不同（如高峰期权重更高）
type TimeDependentWeight struct {
	BaseWeight  float64
	TimeFactors map[time.Weekday]map[int]float64 // 星期 -> 小时 -> 因子
}

// GetWeight 获取指定时间的权重
//
// TODO: 实现时间相关权重计算
func (tdw *TimeDependentWeight) GetWeight(t time.Time) float64 {
	// TODO: 实现时间相关权重
	// 骨架代码：
	// weekday := t.Weekday()
	// hour := t.Hour()
	//
	// if dayFactors, ok := tdw.TimeFactors[weekday]; ok {
	//     if factor, ok := dayFactors[hour]; ok {
	//         return tdw.BaseWeight * factor
	//     }
	// }
	//
	// return tdw.BaseWeight

	return tdw.BaseWeight
}
