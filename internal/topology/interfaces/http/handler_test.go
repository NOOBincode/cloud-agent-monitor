package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/application"
	"cloud-agent-monitor/internal/topology/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

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
	return m.GetImpact(context.Background(), serviceID)
}

func (m *mockCache) SetImpactCache(ctx context.Context, serviceID uuid.UUID, result *domain.ImpactResult, ttl time.Duration) error {
	return m.SetImpact(context.Background(), serviceID, result, ttl)
}

func (m *mockCache) DeleteImpactCache(ctx context.Context, serviceID uuid.UUID) error {
	return m.DeleteImpact(context.Background(), serviceID)
}

func (m *mockCache) InvalidateAll(ctx context.Context) error {
	return m.Clear(context.Background())
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

func setupTestHandler() (*Handler, *application.TopologyService) {
	repo := newMockRepository()
	cache := newMockCache()

	node1 := &domain.ServiceNode{
		ID:          uuid.New(),
		Name:        "user-service",
		Namespace:   "default",
		Status:      domain.ServiceStatusHealthy,
		RequestRate: 1000,
		ErrorRate:   0.5,
		LatencyP99:  100,
	}
	node2 := &domain.ServiceNode{
		ID:          uuid.New(),
		Name:        "order-service",
		Namespace:   "default",
		Status:      domain.ServiceStatusHealthy,
		RequestRate: 500,
		ErrorRate:   0.1,
		LatencyP99:  50,
	}
	node3 := &domain.ServiceNode{
		ID:          uuid.New(),
		Name:        "payment-service",
		Namespace:   "production",
		Status:      domain.ServiceStatusUnhealthy,
		RequestRate: 200,
		ErrorRate:   10.0,
		LatencyP99:  2000,
	}

	edge := &domain.CallEdge{
		ID:          uuid.New(),
		SourceID:    node1.ID,
		TargetID:    node2.ID,
		EdgeType:    domain.EdgeTypeHTTP,
		IsDirect:    true,
		Confidence:  0.9,
		RequestRate: 500,
	}

	networkNode := &domain.NetworkNode{
		ID:        uuid.New(),
		Name:      "pod-network-1",
		Type:      "pod",
		Layer:     domain.NetworkLayerPod,
		Namespace: "default",
	}

	backend := &mockDiscoveryBackend{
		nodes:        []*domain.ServiceNode{node1, node2, node3},
		edges:        []*domain.CallEdge{edge},
		networkNodes: []*domain.NetworkNode{networkNode},
	}

	service := application.NewTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, nil, nil, nil, nil)
	_ = service.RefreshServiceTopology(context.Background())
	_ = service.RefreshNetworkTopology(context.Background())

	return NewHandler(service), service
}

func TestHandler_GetServiceTopology(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("returns service topology", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services", nil)

		handler.GetServiceTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["nodes"])
		assert.NotNil(t, data["edges"])
	})

	t.Run("filters by namespace", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services?namespace=default", nil)

		handler.GetServiceTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandler_GetServiceNode(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.Greater(t, len(topology.Nodes), 0)
	nodeID := topology.Nodes[0].ID

	t.Run("returns service node by id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String(), nil)

		handler.GetServiceNode(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.Equal(t, nodeID.String(), data["id"])
	})

	t.Run("returns error for invalid id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid-uuid"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/invalid-uuid", nil)

		handler.GetServiceNode(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for non-existent node", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+uuid.New().String(), nil)

		handler.GetServiceNode(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_GetServiceNodeByName(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.Greater(t, len(topology.Nodes), 0)
	firstNode := topology.Nodes[0]

	t.Run("returns service node by name or not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{
			{Key: "namespace", Value: firstNode.Namespace},
			{Key: "name", Value: firstNode.Name},
		}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+firstNode.Namespace+"/"+firstNode.Name, nil)

		handler.GetServiceNodeByName(c)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound, "Expected 200 or 404, got %d", w.Code)
	})

	t.Run("returns error for non-existent service", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{
			{Key: "namespace", Value: "default"},
			{Key: "name", Value: "non-existent"},
		}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/default/non-existent", nil)

		handler.GetServiceNodeByName(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandler_GetUpstreamServices(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.Greater(t, len(topology.Nodes), 0)
	nodeID := topology.Nodes[0].ID

	t.Run("returns upstream services", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/upstream?depth=3", nil)

		handler.GetUpstreamServices(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns error for invalid depth", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/upstream?depth=abc", nil)

		handler.GetUpstreamServices(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for depth out of range", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/upstream?depth=100", nil)

		handler.GetUpstreamServices(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_GetDownstreamServices(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.Greater(t, len(topology.Nodes), 0)
	nodeID := topology.Nodes[0].ID

	t.Run("returns downstream services", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/downstream?depth=3", nil)

		handler.GetDownstreamServices(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandler_AnalyzeImpact(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.Greater(t, len(topology.Nodes), 0)
	nodeID := topology.Nodes[0].ID

	t.Run("analyzes impact", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/impact?max_depth=5", nil)

		handler.AnalyzeImpact(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["root_service"])
	})

	t.Run("returns error for invalid id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid-uuid"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/invalid-uuid/impact", nil)

		handler.AnalyzeImpact(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for max_depth out of range", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/services/"+nodeID.String()+"/impact?max_depth=100", nil)

		handler.AnalyzeImpact(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_GetNetworkTopology(t *testing.T) {
	handler, service := setupTestHandler()

	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)
	err = service.RefreshNetworkTopology(context.Background())
	require.NoError(t, err)

	t.Run("returns network topology", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/network", nil)

		handler.GetNetworkTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandler_GetNetworkNode(t *testing.T) {
	handler, service := setupTestHandler()

	networkTopology, _ := service.GetNetworkTopology(context.Background(), domain.TopologyQuery{})

	t.Run("returns network node by id", func(t *testing.T) {
		if len(networkTopology.Nodes) == 0 {
			t.Skip("No network nodes available")
		}
		nodeID := networkTopology.Nodes[0].ID

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: nodeID.String()}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/network/"+nodeID.String(), nil)

		handler.GetNetworkNode(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns error for invalid id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid-uuid"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/network/invalid-uuid", nil)

		handler.GetNetworkNode(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_FindPath(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.GreaterOrEqual(t, len(topology.Nodes), 2)
	sourceID := topology.Nodes[0].ID
	targetID := topology.Nodes[1].ID

	t.Run("finds path between services or returns not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?source_id="+sourceID.String()+"&target_id="+targetID.String()+"&max_hops=10", nil)

		handler.FindPath(c)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})

	t.Run("returns error for missing source_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?target_id="+targetID.String(), nil)

		handler.FindPath(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid source_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/path?source_id=invalid&target_id="+targetID.String(), nil)

		handler.FindPath(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_FindShortestPath(t *testing.T) {
	handler, service := setupTestHandler()

	topology, _ := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
	require.GreaterOrEqual(t, len(topology.Nodes), 2)
	sourceID := topology.Nodes[0].ID
	targetID := topology.Nodes[1].ID

	t.Run("finds shortest path or returns not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/path/shortest?source_id="+sourceID.String()+"&target_id="+targetID.String(), nil)

		handler.FindShortestPath(c)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
	})

	t.Run("returns error for missing parameters", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/path/shortest", nil)

		handler.FindShortestPath(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandler_FindAnomalies(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("finds anomalies", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/anomalies", nil)

		handler.FindAnomalies(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["anomalies"])
	})
}

func TestHandler_GetTopologyAtTime(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("returns error for invalid timestamp", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?timestamp=invalid", nil)

		handler.GetTopologyAtTime(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for missing timestamp", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/history", nil)

		handler.GetTopologyAtTime(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for valid timestamp when no snapshot exists", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		timestamp := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/history?timestamp="+timestamp, nil)

		handler.GetTopologyAtTime(c)

		assert.NotEqual(t, http.StatusOK, w.Code)
	})
}

func TestHandler_GetTopologyChanges(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("returns error for missing parameters", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/changes", nil)

		handler.GetTopologyChanges(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns error for invalid from timestamp", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/changes?from=invalid&to=2024-01-01T00:00:00Z", nil)

		handler.GetTopologyChanges(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid request returns empty changes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		from := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
		to := time.Now().UTC().Format(time.RFC3339)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/changes?from="+from+"&to="+to, nil)

		handler.GetTopologyChanges(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["changes"])
	})
}

func TestHandler_GetTopologyStats(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("returns topology stats", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/topology/stats", nil)

		handler.GetTopologyStats(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		data := response["data"].(map[string]interface{})
		assert.NotNil(t, data["service_node_count"])
		assert.NotNil(t, data["service_edge_count"])
	})
}

func TestHandler_RefreshTopology(t *testing.T) {
	handler, _ := setupTestHandler()

	t.Run("refreshes all topology", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topology/refresh?type=all", bytes.NewReader([]byte{}))

		handler.RefreshTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("refreshes service topology", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topology/refresh?type=service", bytes.NewReader([]byte{}))

		handler.RefreshTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("refreshes network topology", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topology/refresh?type=network", bytes.NewReader([]byte{}))

		handler.RefreshTopology(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns error for invalid type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/topology/refresh?type=invalid", bytes.NewReader([]byte{}))

		handler.RefreshTopology(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
