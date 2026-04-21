package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/domain"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	mu           sync.RWMutex
	serviceNodes map[uuid.UUID]*domain.ServiceNode
	callEdges    map[uuid.UUID]*domain.CallEdge
	networkNodes map[uuid.UUID]*domain.NetworkNode
	networkEdges map[uuid.UUID]*domain.NetworkEdge
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		serviceNodes: make(map[uuid.UUID]*domain.ServiceNode),
		callEdges:    make(map[uuid.UUID]*domain.CallEdge),
		networkNodes: make(map[uuid.UUID]*domain.NetworkNode),
		networkEdges: make(map[uuid.UUID]*domain.NetworkEdge),
	}
}

func (m *mockRepository) SaveServiceNode(ctx context.Context, node *domain.ServiceNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceNodes[node.ID] = node
	return nil
}

func (m *mockRepository) BatchSaveServiceNodes(ctx context.Context, nodes []*domain.ServiceNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, node := range nodes {
		m.serviceNodes[node.ID] = node
	}
	return nil
}

func (m *mockRepository) GetServiceNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if node, ok := m.serviceNodes[id]; ok {
		return node, nil
	}
	return nil, domain.ErrNodeNotFound
}

func (m *mockRepository) GetServiceNodeByName(ctx context.Context, namespace, name string) (*domain.ServiceNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, node := range m.serviceNodes {
		if node.Namespace == namespace && node.Name == name {
			return node, nil
		}
	}
	return nil, domain.ErrNodeNotFound
}

func (m *mockRepository) ListServiceNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.ServiceNode, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]*domain.ServiceNode, 0, len(m.serviceNodes))
	for _, node := range m.serviceNodes {
		nodes = append(nodes, node)
	}
	return nodes, int64(len(nodes)), nil
}

func (m *mockRepository) DeleteServiceNode(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.serviceNodes, id)
	return nil
}

func (m *mockRepository) SaveCallEdge(ctx context.Context, edge *domain.CallEdge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callEdges[edge.ID] = edge
	return nil
}

func (m *mockRepository) BatchSaveCallEdges(ctx context.Context, edges []*domain.CallEdge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, edge := range edges {
		m.callEdges[edge.ID] = edge
	}
	return nil
}

func (m *mockRepository) GetCallEdge(ctx context.Context, id uuid.UUID) (*domain.CallEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if edge, ok := m.callEdges[id]; ok {
		return edge, nil
	}
	return nil, domain.ErrEdgeNotFound
}

func (m *mockRepository) GetCallEdgeByEndpoints(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.CallEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, edge := range m.callEdges {
		if edge.SourceID == sourceID && edge.TargetID == targetID {
			return edge, nil
		}
	}
	return nil, domain.ErrEdgeNotFound
}

func (m *mockRepository) ListCallEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.CallEdge, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	edges := make([]*domain.CallEdge, 0, len(m.callEdges))
	for _, edge := range m.callEdges {
		edges = append(edges, edge)
	}
	return edges, int64(len(edges)), nil
}

func (m *mockRepository) ListCallEdgesBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.CallEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	edges := make([]*domain.CallEdge, 0)
	for _, edge := range m.callEdges {
		if edge.SourceID == sourceID {
			edges = append(edges, edge)
		}
	}
	return edges, nil
}

func (m *mockRepository) ListCallEdgesByTarget(ctx context.Context, targetID uuid.UUID) ([]*domain.CallEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	edges := make([]*domain.CallEdge, 0)
	for _, edge := range m.callEdges {
		if edge.TargetID == targetID {
			edges = append(edges, edge)
		}
	}
	return edges, nil
}

func (m *mockRepository) DeleteCallEdge(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.callEdges, id)
	return nil
}

func (m *mockRepository) SaveNetworkNode(ctx context.Context, node *domain.NetworkNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.networkNodes[node.ID] = node
	return nil
}

func (m *mockRepository) BatchSaveNetworkNodes(ctx context.Context, nodes []*domain.NetworkNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, node := range nodes {
		m.networkNodes[node.ID] = node
	}
	return nil
}

func (m *mockRepository) GetNetworkNode(ctx context.Context, id uuid.UUID) (*domain.NetworkNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if node, ok := m.networkNodes[id]; ok {
		return node, nil
	}
	return nil, domain.ErrNodeNotFound
}

func (m *mockRepository) GetNetworkNodeByIP(ctx context.Context, ip string) (*domain.NetworkNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, node := range m.networkNodes {
		if node.IPAddress == ip {
			return node, nil
		}
	}
	return nil, domain.ErrNodeNotFound
}

func (m *mockRepository) ListNetworkNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkNode, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	nodes := make([]*domain.NetworkNode, 0, len(m.networkNodes))
	for _, node := range m.networkNodes {
		nodes = append(nodes, node)
	}
	return nodes, int64(len(nodes)), nil
}

func (m *mockRepository) DeleteNetworkNode(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networkNodes, id)
	return nil
}

func (m *mockRepository) SaveNetworkEdge(ctx context.Context, edge *domain.NetworkEdge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.networkEdges[edge.ID] = edge
	return nil
}

func (m *mockRepository) BatchSaveNetworkEdges(ctx context.Context, edges []*domain.NetworkEdge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, edge := range edges {
		m.networkEdges[edge.ID] = edge
	}
	return nil
}

func (m *mockRepository) GetNetworkEdge(ctx context.Context, id uuid.UUID) (*domain.NetworkEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if edge, ok := m.networkEdges[id]; ok {
		return edge, nil
	}
	return nil, domain.ErrEdgeNotFound
}

func (m *mockRepository) ListNetworkEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkEdge, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	edges := make([]*domain.NetworkEdge, 0, len(m.networkEdges))
	for _, edge := range m.networkEdges {
		edges = append(edges, edge)
	}
	return edges, int64(len(edges)), nil
}

func (m *mockRepository) DeleteNetworkEdge(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networkEdges, id)
	return nil
}

func (m *mockRepository) SaveTopologyChange(ctx context.Context, change *domain.TopologyChange) error {
	return nil
}

func (m *mockRepository) ListTopologyChanges(ctx context.Context, from, to time.Time, entityType string) ([]*domain.TopologyChange, error) {
	return []*domain.TopologyChange{}, nil
}

func (m *mockRepository) SaveTopologySnapshot(ctx context.Context, graphType string, snapshot *domain.ServiceTopology) error {
	return nil
}

func (m *mockRepository) GetTopologySnapshot(ctx context.Context, graphType string, timestamp time.Time) (*domain.ServiceTopology, error) {
	return nil, domain.ErrNodeNotFound
}

func (m *mockRepository) CleanupOldData(ctx context.Context, retention time.Duration) error {
	return nil
}

type mockCache struct {
	mu              sync.RWMutex
	serviceTopology *domain.ServiceTopology
	networkTopology *domain.NetworkTopology
	nodes           map[uuid.UUID]*domain.ServiceNode
	edges           map[uuid.UUID]*domain.CallEdge
	adjList         map[uuid.UUID][]uuid.UUID
	impactCache     map[uuid.UUID]*domain.ImpactResult
}

func newMockCache() *mockCache {
	return &mockCache{
		nodes:       make(map[uuid.UUID]*domain.ServiceNode),
		edges:       make(map[uuid.UUID]*domain.CallEdge),
		adjList:     make(map[uuid.UUID][]uuid.UUID),
		impactCache: make(map[uuid.UUID]*domain.ImpactResult),
	}
}

func (m *mockCache) GetServiceTopology(ctx context.Context) (*domain.ServiceTopology, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.serviceTopology == nil {
		return nil, domain.ErrCacheMiss
	}
	return m.serviceTopology, nil
}

func (m *mockCache) SetServiceTopology(ctx context.Context, topology *domain.ServiceTopology, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceTopology = topology
	return nil
}

func (m *mockCache) GetNetworkTopology(ctx context.Context) (*domain.NetworkTopology, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.networkTopology == nil {
		return nil, domain.ErrCacheMiss
	}
	return m.networkTopology, nil
}

func (m *mockCache) SetNetworkTopology(ctx context.Context, topology *domain.NetworkTopology, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.networkTopology = topology
	return nil
}

func (m *mockCache) GetNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if node, ok := m.nodes[id]; ok {
		return node, nil
	}
	return nil, domain.ErrCacheMiss
}

func (m *mockCache) SetNode(ctx context.Context, node *domain.ServiceNode, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
	return nil
}

func (m *mockCache) DeleteNode(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, id)
	return nil
}

func (m *mockCache) GetEdge(ctx context.Context, id uuid.UUID) (*domain.CallEdge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if edge, ok := m.edges[id]; ok {
		return edge, nil
	}
	return nil, domain.ErrCacheMiss
}

func (m *mockCache) SetEdge(ctx context.Context, edge *domain.CallEdge, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edges[edge.ID] = edge
	return nil
}

func (m *mockCache) GetAdjacencyList(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.adjList == nil {
		return nil, domain.ErrCacheMiss
	}
	return m.adjList, nil
}

func (m *mockCache) SetAdjacencyList(ctx context.Context, adjList map[uuid.UUID][]uuid.UUID, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adjList = adjList
	return nil
}

func (m *mockCache) GetImpact(ctx context.Context, serviceID uuid.UUID) (*domain.ImpactResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if result, ok := m.impactCache[serviceID]; ok {
		return result, nil
	}
	return nil, domain.ErrCacheMiss
}

func (m *mockCache) SetImpact(ctx context.Context, serviceID uuid.UUID, result *domain.ImpactResult, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.impactCache[serviceID] = result
	return nil
}

func (m *mockCache) DeleteImpact(ctx context.Context, serviceID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.impactCache, serviceID)
	return nil
}

func (m *mockCache) GetImpactCache(ctx context.Context, serviceID uuid.UUID) (*domain.ImpactResult, error) {
	return m.GetImpact(ctx, serviceID)
}

func (m *mockCache) SetImpactCache(ctx context.Context, serviceID uuid.UUID, result *domain.ImpactResult, ttl time.Duration) error {
	return m.SetImpact(ctx, serviceID, result, ttl)
}

func (m *mockCache) DeleteImpactCache(ctx context.Context, serviceID uuid.UUID) error {
	return m.DeleteImpact(ctx, serviceID)
}

func (m *mockCache) InvalidateAll(ctx context.Context) error {
	return m.Clear(ctx)
}

func (m *mockCache) InvalidateService(ctx context.Context, serviceID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.nodes, serviceID)
	delete(m.impactCache, serviceID)
	return nil
}

func (m *mockCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceTopology = nil
	m.networkTopology = nil
	m.nodes = make(map[uuid.UUID]*domain.ServiceNode)
	m.edges = make(map[uuid.UUID]*domain.CallEdge)
	m.adjList = make(map[uuid.UUID][]uuid.UUID)
	m.impactCache = make(map[uuid.UUID]*domain.ImpactResult)
	return nil
}

type mockDiscoveryBackend struct {
	nodes        []*domain.ServiceNode
	edges        []*domain.CallEdge
	networkNodes []*domain.NetworkNode
	networkEdges []*domain.NetworkEdge
	healthErr    error
	discoverErr  error
}

func (m *mockDiscoveryBackend) DiscoverNodes(ctx context.Context) ([]*domain.ServiceNode, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.nodes, nil
}

func (m *mockDiscoveryBackend) DiscoverEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.edges, nil
}

func (m *mockDiscoveryBackend) DiscoverNetworkNodes(ctx context.Context) ([]*domain.NetworkNode, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.networkNodes, nil
}

func (m *mockDiscoveryBackend) DiscoverNetworkEdges(ctx context.Context) ([]*domain.NetworkEdge, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.networkEdges, nil
}

func (m *mockDiscoveryBackend) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

type mockQueue struct {
	handlers map[string]infra.TaskHandler
	enqueued []string
	mu       sync.Mutex
}

func newMockQueue() *mockQueue {
	return &mockQueue{
		handlers: make(map[string]infra.TaskHandler),
	}
}

func (m *mockQueue) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, taskType)
	return nil, nil
}

func (m *mockQueue) EnqueueWithDelay(ctx context.Context, taskType string, payload any, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, taskType)
	return nil, nil
}

func (m *mockQueue) RegisterHandler(taskType string, handler infra.TaskHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[taskType] = handler
}

func (m *mockQueue) Start() error { return nil }

func (m *mockQueue) Stop() {}

func (m *mockQueue) Close() error { return nil }

func (m *mockQueue) GetInspector() *asynq.Inspector { return nil }

func newTestTopologyService(repo domain.TopologyRepository, cache domain.TopologyCache, discoverers []domain.DiscoveryBackend, cfg *Config) *TopologyService {
	return NewTopologyService(repo, cache, discoverers, nil, nil, cfg)
}

func TestNewTopologyService(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	t.Run("creates service with default config", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, nil, nil)
		assert.NotNil(t, service)
		assert.Equal(t, 30*time.Second, service.refreshInterval)
		assert.Equal(t, 5*time.Minute, service.cacheTTL)
		assert.Equal(t, 10, service.maxDepth)
	})

	t.Run("creates service with custom config", func(t *testing.T) {
		cfg := &Config{
			RefreshInterval: 10 * time.Second,
			CacheTTL:        1 * time.Minute,
			MaxDepth:        5,
		}
		service := newTestTopologyService(repo, cache, nil, cfg)
		assert.NotNil(t, service)
		assert.Equal(t, 10*time.Second, service.refreshInterval)
		assert.Equal(t, 1*time.Minute, service.cacheTTL)
		assert.Equal(t, 5, service.maxDepth)
	})
}

func TestTopologyService_StartStop(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	node1 := &domain.ServiceNode{ID: uuid.New(), Name: "service-1", Namespace: "default", Status: domain.ServiceStatusHealthy}
	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{node1},
		edges: []*domain.CallEdge{},
	}

	t.Run("starts and stops successfully", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, &Config{
			RefreshInterval: 1 * time.Second,
		})

		err := service.Start(context.Background())
		require.NoError(t, err)

		assert.True(t, service.running)

		service.Stop()
		assert.False(t, service.running)
	})

	t.Run("start is idempotent", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.Start(context.Background())
		require.NoError(t, err)

		err = service.Start(context.Background())
		require.NoError(t, err)

		service.Stop()
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		service.Stop()
		service.Stop()
	})
}

func TestTopologyService_GetServiceTopology(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	node1 := &domain.ServiceNode{ID: uuid.New(), Name: "service-1", Namespace: "default", Status: domain.ServiceStatusHealthy}
	node2 := &domain.ServiceNode{ID: uuid.New(), Name: "service-2", Namespace: "production", Status: domain.ServiceStatusHealthy}
	edge := &domain.CallEdge{
		ID:       uuid.New(),
		SourceID: node1.ID,
		TargetID: node2.ID,
		EdgeType: domain.EdgeTypeHTTP,
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{node1, node2},
		edges: []*domain.CallEdge{edge},
	}

	t.Run("returns topology after refresh", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		topology, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
		require.NoError(t, err)
		assert.Len(t, topology.Nodes, 2)
		assert.Len(t, topology.Edges, 1)
	})

	t.Run("filters by namespace", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		topology, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{
			Namespace: "default",
		})
		require.NoError(t, err)
		assert.Len(t, topology.Nodes, 1)
		assert.Equal(t, "default", topology.Nodes[0].Namespace)
	})
}

func TestTopologyService_GetServiceNode(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	node := &domain.ServiceNode{
		ID:        uuid.New(),
		Name:      "test-service",
		Namespace: "default",
		Status:    domain.ServiceStatusHealthy,
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{node},
		edges: []*domain.CallEdge{},
	}

	t.Run("returns node by id", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		result, err := service.GetServiceNode(context.Background(), node.ID)
		require.NoError(t, err)
		assert.Equal(t, node.ID, result.ID)
		assert.Equal(t, "test-service", result.Name)
	})

	t.Run("returns error for non-existent node", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		_, err := service.GetServiceNode(context.Background(), uuid.New())
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})
}

func TestTopologyService_AnalyzeImpact(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	rootNode := &domain.ServiceNode{ID: uuid.New(), Name: "root", Namespace: "default", Status: domain.ServiceStatusHealthy}
	upstreamNode := &domain.ServiceNode{ID: uuid.New(), Name: "upstream", Namespace: "default", Status: domain.ServiceStatusHealthy}
	downstreamNode := &domain.ServiceNode{ID: uuid.New(), Name: "downstream", Namespace: "default", Status: domain.ServiceStatusHealthy}

	edges := []*domain.CallEdge{
		{ID: uuid.New(), SourceID: upstreamNode.ID, TargetID: rootNode.ID},
		{ID: uuid.New(), SourceID: rootNode.ID, TargetID: downstreamNode.ID},
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{rootNode, upstreamNode, downstreamNode},
		edges: edges,
	}

	t.Run("analyzes impact successfully", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		result, err := service.AnalyzeImpact(context.Background(), rootNode.ID, 5)
		require.NoError(t, err)
		assert.Equal(t, rootNode.ID, result.RootService.ID)
		assert.Equal(t, 2, result.TotalAffected)
	})

	t.Run("returns error for non-existent node", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		_, err := service.AnalyzeImpact(context.Background(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})
}

func TestTopologyService_FindPath(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default", Status: domain.ServiceStatusHealthy}
	nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default", Status: domain.ServiceStatusHealthy}
	nodeC := &domain.ServiceNode{ID: uuid.New(), Name: "c", Namespace: "default", Status: domain.ServiceStatusHealthy}

	edges := []*domain.CallEdge{
		{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeB.ID},
		{ID: uuid.New(), SourceID: nodeB.ID, TargetID: nodeC.ID},
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{nodeA, nodeB, nodeC},
		edges: edges,
	}

	t.Run("finds path between nodes", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		result, err := service.FindPath(context.Background(), nodeA.ID, nodeC.ID, 5)
		require.NoError(t, err)
		assert.Equal(t, nodeA.ID, result.SourceID)
		assert.Equal(t, nodeC.ID, result.TargetID)
		assert.GreaterOrEqual(t, len(result.Paths), 1)
	})

	t.Run("returns error when no path exists", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		_, err = service.FindPath(context.Background(), nodeC.ID, nodeA.ID, 5)
		assert.ErrorIs(t, err, domain.ErrPathNotFound)
	})
}

func TestTopologyService_GetUpstreamServices(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	targetNode := &domain.ServiceNode{ID: uuid.New(), Name: "target", Namespace: "default", Status: domain.ServiceStatusHealthy}
	upstreamNode := &domain.ServiceNode{ID: uuid.New(), Name: "upstream", Namespace: "default", Status: domain.ServiceStatusHealthy}

	edges := []*domain.CallEdge{
		{ID: uuid.New(), SourceID: upstreamNode.ID, TargetID: targetNode.ID},
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{targetNode, upstreamNode},
		edges: edges,
	}

	t.Run("returns upstream services", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		upstream, err := service.GetUpstreamServices(context.Background(), targetNode.ID, 5)
		require.NoError(t, err)
		assert.Len(t, upstream, 1)
		assert.Equal(t, upstreamNode.ID, upstream[0].ID)
	})
}

func TestTopologyService_GetDownstreamServices(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	sourceNode := &domain.ServiceNode{ID: uuid.New(), Name: "source", Namespace: "default", Status: domain.ServiceStatusHealthy}
	downstreamNode := &domain.ServiceNode{ID: uuid.New(), Name: "downstream", Namespace: "default", Status: domain.ServiceStatusHealthy}

	edges := []*domain.CallEdge{
		{ID: uuid.New(), SourceID: sourceNode.ID, TargetID: downstreamNode.ID},
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{sourceNode, downstreamNode},
		edges: edges,
	}

	t.Run("returns downstream services", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		downstream, err := service.GetDownstreamServices(context.Background(), sourceNode.ID, 5)
		require.NoError(t, err)
		assert.Len(t, downstream, 1)
		assert.Equal(t, downstreamNode.ID, downstream[0].ID)
	})
}

func TestTopologyService_FindAnomalies(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	healthyNode := &domain.ServiceNode{
		ID:        uuid.New(),
		Name:      "healthy",
		Namespace: "default",
		Status:    domain.ServiceStatusHealthy,
	}
	unhealthyNode := &domain.ServiceNode{
		ID:        uuid.New(),
		Name:      "unhealthy",
		Namespace: "default",
		Status:    domain.ServiceStatusUnhealthy,
	}
	highErrorNode := &domain.ServiceNode{
		ID:        uuid.New(),
		Name:      "high-error",
		Namespace: "default",
		Status:    domain.ServiceStatusHealthy,
		ErrorRate: 0.10,
	}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{healthyNode, unhealthyNode, highErrorNode},
		edges: []*domain.CallEdge{},
	}

	t.Run("detects anomalies", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		anomalies, err := service.FindAnomalies(context.Background())
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(anomalies), 2)
	})
}

func TestTopologyService_GetTopologyStats(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	node1 := &domain.ServiceNode{ID: uuid.New(), Name: "service-1", Namespace: "ns1", Status: domain.ServiceStatusHealthy}
	node2 := &domain.ServiceNode{ID: uuid.New(), Name: "service-2", Namespace: "ns2", Status: domain.ServiceStatusHealthy}
	edge := &domain.CallEdge{ID: uuid.New(), SourceID: node1.ID, TargetID: node2.ID}

	backend := &mockDiscoveryBackend{
		nodes: []*domain.ServiceNode{node1, node2},
		edges: []*domain.CallEdge{edge},
		networkNodes: []*domain.NetworkNode{
			{ID: uuid.New(), Name: "network-1"},
		},
	}

	t.Run("returns topology stats", func(t *testing.T) {
		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		err = service.RefreshNetworkTopology(context.Background())
		require.NoError(t, err)

		stats, err := service.GetTopologyStats(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 2, stats.ServiceNodeCount)
		assert.Equal(t, 1, stats.ServiceEdgeCount)
		assert.Equal(t, 1, stats.NetworkNodeCount)
	})
}

func TestTopologyService_RefreshServiceTopology(t *testing.T) {
	repo := newMockRepository()
	cache := newMockCache()

	t.Run("refreshes from multiple backends", func(t *testing.T) {
		node1 := &domain.ServiceNode{ID: uuid.New(), Name: "service-1", Namespace: "default"}
		node2 := &domain.ServiceNode{ID: uuid.New(), Name: "service-2", Namespace: "default"}

		backend1 := &mockDiscoveryBackend{
			nodes: []*domain.ServiceNode{node1},
			edges: []*domain.CallEdge{},
		}
		backend2 := &mockDiscoveryBackend{
			nodes: []*domain.ServiceNode{node2},
			edges: []*domain.CallEdge{},
		}

		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend1, backend2}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		topology, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
		require.NoError(t, err)
		assert.Len(t, topology.Nodes, 2)
	})

	t.Run("handles discovery errors gracefully", func(t *testing.T) {
		node := &domain.ServiceNode{ID: uuid.New(), Name: "service-1", Namespace: "default"}

		backend1 := &mockDiscoveryBackend{
			nodes:       []*domain.ServiceNode{},
			edges:       []*domain.CallEdge{},
			discoverErr: errors.New("discovery failed"),
		}
		backend2 := &mockDiscoveryBackend{
			nodes: []*domain.ServiceNode{node},
			edges: []*domain.CallEdge{},
		}

		service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend1, backend2}, nil)

		err := service.RefreshServiceTopology(context.Background())
		require.NoError(t, err)

		topology, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
		require.NoError(t, err)
		assert.Len(t, topology.Nodes, 1)
	})
}

func TestInMemoryGraph(t *testing.T) {
	t.Run("rebuilds graph correctly", func(t *testing.T) {
		graph := NewInMemoryGraph()

		node1 := &domain.ServiceNode{ID: uuid.New(), Name: "node-1"}
		node2 := &domain.ServiceNode{ID: uuid.New(), Name: "node-2"}
		edge := &domain.CallEdge{
			ID:       uuid.New(),
			SourceID: node1.ID,
			TargetID: node2.ID,
		}

		graph.Rebuild([]*domain.ServiceNode{node1, node2}, []*domain.CallEdge{edge})

		assert.Equal(t, node1, graph.GetNode(node1.ID))
		assert.Equal(t, node2, graph.GetNode(node2.ID))
		assert.Equal(t, []uuid.UUID{node2.ID}, graph.GetDownstreamIDs(node1.ID))
		assert.Equal(t, []uuid.UUID{node1.ID}, graph.GetUpstreamIDs(node2.ID))
	})

	t.Run("returns nil for non-existent node", func(t *testing.T) {
		graph := NewInMemoryGraph()
		assert.Nil(t, graph.GetNode(uuid.New()))
	})

	t.Run("returns all nodes", func(t *testing.T) {
		graph := NewInMemoryGraph()

		node1 := &domain.ServiceNode{ID: uuid.New(), Name: "node-1"}
		node2 := &domain.ServiceNode{ID: uuid.New(), Name: "node-2"}

		graph.Rebuild([]*domain.ServiceNode{node1, node2}, []*domain.CallEdge{})

		nodes := graph.GetAllNodes()
		assert.Len(t, nodes, 2)
	})

	t.Run("returns all edges", func(t *testing.T) {
		graph := NewInMemoryGraph()

		node := &domain.ServiceNode{ID: uuid.New(), Name: "node"}
		edge := &domain.CallEdge{ID: uuid.New(), SourceID: node.ID, TargetID: uuid.New()}

		graph.Rebuild([]*domain.ServiceNode{node}, []*domain.CallEdge{edge})

		edges := graph.GetAllEdges()
		assert.Len(t, edges, 1)
	})
}
