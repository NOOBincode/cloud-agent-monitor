//go:build integration

package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type TopologyIntegrationSuite struct {
	suite.Suite
	mysqlContainer *mysql.MySQLContainer
	db             *gorm.DB
	service        *TopologyService
	repo           domain.TopologyRepository
	cache          domain.TopologyCache
}

func TestTopologyIntegration(t *testing.T) {
	suite.Run(t, new(TopologyIntegrationSuite))
}

func (s *TopologyIntegrationSuite) SetupSuite() {
	ctx := context.Background()

	container, err := mysql.Run(ctx,
		"mysql:8.0",
		mysql.WithDatabase("obs_platform_test"),
		mysql.WithUsername("root"),
		mysql.WithPassword("testpassword"),
	)
	require.NoError(s.T(), err)
	s.mysqlContainer = container

	dsn, err := container.ConnectionString(ctx, "parseTime=true&charset=utf8mb4")
	require.NoError(s.T(), err)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(s.T(), err)
	s.db = db

	err = s.db.AutoMigrate(
		&TopologyServiceNodeModel{},
		&TopologyCallEdgeModel{},
	)
	require.NoError(s.T(), err)

	s.repo = NewGormTopologyRepository(s.db)
	s.cache = newMockCache()

	s.service = newTestTopologyService(s.repo, s.cache, nil, &Config{
		RefreshInterval: 5 * time.Second,
		CacheTTL:        1 * time.Minute,
		MaxDepth:        10,
	})
}

func (s *TopologyIntegrationSuite) TearDownSuite() {
	if s.mysqlContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.mysqlContainer.Terminate(ctx)
	}
}

func (s *TopologyIntegrationSuite) SetupTest() {
	s.db.Exec("DELETE FROM topology_call_edges")
	s.db.Exec("DELETE FROM topology_service_nodes")
	s.cache.InvalidateAll(context.Background())
}

func (s *TopologyIntegrationSuite) TestServiceNodeCRUD() {
	ctx := context.Background()

	node := &domain.ServiceNode{
		ID:          uuid.New(),
		Name:        "integration-test-service",
		Namespace:   "default",
		Environment: "production",
		Status:      domain.ServiceStatusHealthy,
		Labels:      map[string]string{"app": "test", "team": "backend"},
		RequestRate: 500.0,
		ErrorRate:   0.5,
		LatencyP99:  120.0,
		PodCount:    3,
		ReadyPods:   3,
	}

	err := s.repo.SaveServiceNode(ctx, node)
	s.NoError(err)

	retrieved, err := s.repo.GetServiceNode(ctx, node.ID)
	s.NoError(err)
	s.Equal(node.Name, retrieved.Name)
	s.Equal(node.Namespace, retrieved.Namespace)
	s.Equal(node.Status, retrieved.Status)
	s.Equal(node.RequestRate, retrieved.RequestRate)

	nodes, total, err := s.repo.ListServiceNodes(ctx, domain.TopologyQuery{Namespace: "default"})
	s.NoError(err)
	s.Equal(int64(1), total)
	s.Len(nodes, 1)

	err = s.repo.DeleteServiceNode(ctx, node.ID)
	s.NoError(err)

	_, err = s.repo.GetServiceNode(ctx, node.ID)
	s.ErrorIs(err, domain.ErrNodeNotFound)
}

func (s *TopologyIntegrationSuite) TestCallEdgeCRUD() {
	ctx := context.Background()

	sourceNode := &domain.ServiceNode{
		ID: uuid.New(), Name: "source-service", Namespace: "default",
		Status: domain.ServiceStatusHealthy,
	}
	targetNode := &domain.ServiceNode{
		ID: uuid.New(), Name: "target-service", Namespace: "default",
		Status: domain.ServiceStatusHealthy,
	}

	err := s.repo.BatchSaveServiceNodes(ctx, []*domain.ServiceNode{sourceNode, targetNode})
	s.NoError(err)

	edge := &domain.CallEdge{
		ID:          uuid.New(),
		SourceID:    sourceNode.ID,
		TargetID:    targetNode.ID,
		EdgeType:    domain.EdgeTypeHTTP,
		IsDirect:    true,
		Confidence:  0.9,
		RequestRate: 200.0,
		ErrorRate:   1.0,
		LatencyP99:  50.0,
	}

	err = s.repo.SaveCallEdge(ctx, edge)
	s.NoError(err)

	retrieved, err := s.repo.GetCallEdge(ctx, edge.ID)
	s.NoError(err)
	s.Equal(sourceNode.ID, retrieved.SourceID)
	s.Equal(targetNode.ID, retrieved.TargetID)
	s.Equal(domain.EdgeTypeHTTP, retrieved.EdgeType)

	bySource, err := s.repo.ListCallEdgesBySource(ctx, sourceNode.ID)
	s.NoError(err)
	s.Len(bySource, 1)

	byTarget, err := s.repo.ListCallEdgesByTarget(ctx, targetNode.ID)
	s.NoError(err)
	s.Len(byTarget, 1)
}

func (s *TopologyIntegrationSuite) TestBatchSaveAndList() {
	ctx := context.Background()

	const nodeCount = 50
	nodes := make([]*domain.ServiceNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &domain.ServiceNode{
			ID:          uuid.New(),
			Name:        fmt.Sprintf("batch-service-%d", i),
			Namespace:   "default",
			Status:      domain.ServiceStatusHealthy,
			RequestRate: float64(i * 10),
		}
	}

	err := s.repo.BatchSaveServiceNodes(ctx, nodes)
	s.NoError(err)

	allNodes, total, err := s.repo.ListServiceNodes(ctx, domain.TopologyQuery{})
	s.NoError(err)
	s.Equal(int64(nodeCount), total)
	s.Len(allNodes, nodeCount)
}

func (s *TopologyIntegrationSuite) TestTopologySnapshotLifecycle() {
	ctx := context.Background()

	nodes := []*domain.ServiceNode{
		{ID: uuid.New(), Name: "snapshot-svc-1", Namespace: "default", Status: domain.ServiceStatusHealthy},
		{ID: uuid.New(), Name: "snapshot-svc-2", Namespace: "default", Status: domain.ServiceStatusHealthy},
	}
	edges := []*domain.CallEdge{
		{ID: uuid.New(), SourceID: nodes[0].ID, TargetID: nodes[1].ID, EdgeType: domain.EdgeTypeHTTP},
	}

	err := s.repo.BatchSaveServiceNodes(ctx, nodes)
	s.NoError(err)
	err = s.repo.BatchSaveCallEdges(ctx, edges)
	s.NoError(err)

	snapshot := &domain.ServiceTopology{
		ID:        uuid.New(),
		Timestamp: time.Now(),
		Nodes:     nodes,
		Edges:     edges,
	}

	err = s.repo.SaveTopologySnapshot(ctx, "service", snapshot)
	s.NoError(err)
}

func (s *TopologyIntegrationSuite) TestTopologyChangeTracking() {
	ctx := context.Background()

	change := &domain.TopologyChange{
		ID:          uuid.New(),
		Timestamp:   time.Now(),
		ChangeType:  "added",
		EntityType:  "service_node",
		EntityID:    uuid.New(),
		EntityName:  "new-service",
		Description: "Service discovered by k8s backend",
	}

	err := s.repo.SaveTopologyChange(ctx, change)
	s.NoError(err)

	changes, err := s.repo.ListTopologyChanges(ctx, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour), "")
	s.NoError(err)
	s.NotEmpty(changes)
}

func (s *TopologyIntegrationSuite) TestConcurrentNodeCreation() {
	ctx := context.Background()

	const numGoroutines = 20
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			node := &domain.ServiceNode{
				ID:        uuid.New(),
				Name:      fmt.Sprintf("concurrent-service-%d", idx),
				Namespace: "default",
				Status:    domain.ServiceStatusHealthy,
			}
			errCh <- s.repo.SaveServiceNode(ctx, node)
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		err := <-errCh
		s.NoError(err)
	}

	_, total, err := s.repo.ListServiceNodes(ctx, domain.TopologyQuery{})
	s.NoError(err)
	s.Equal(int64(numGoroutines), total)
}

type TopologyServiceNodeModel struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	Name        string    `gorm:"size:255;not null;uniqueIndex:idx_ns_name"`
	Namespace   string    `gorm:"size:255;not null;uniqueIndex:idx_ns_name"`
	Environment string    `gorm:"size:100"`
	Status      string    `gorm:"size:50;not null"`
	Labels      string    `gorm:"type:json"`
	RequestRate float64
	ErrorRate   float64
	LatencyP99  float64
	LatencyP95  float64
	LatencyP50  float64
	PodCount    int
	ReadyPods   int
	ServiceType string `gorm:"size:100"`
	Maintainer  string `gorm:"size:255"`
	Team        string `gorm:"size:255"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TopologyServiceNodeModel) TableName() string {
	return "topology_service_nodes"
}

type TopologyCallEdgeModel struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	SourceID    uuid.UUID `gorm:"not null;index"`
	TargetID    uuid.UUID `gorm:"not null;index"`
	EdgeType    string    `gorm:"size:50;not null"`
	IsDirect    bool
	Confidence  float64
	Protocol    string `gorm:"size:50"`
	Method      string `gorm:"size:100"`
	RequestRate float64
	ErrorRate   float64
	LatencyP99  float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TopologyCallEdgeModel) TableName() string {
	return "topology_call_edges"
}

type gormTopologyRepository struct {
	db *gorm.DB
}

func NewGormTopologyRepository(db *gorm.DB) domain.TopologyRepository {
	return &gormTopologyRepository{db: db}
}

func (r *gormTopologyRepository) SaveServiceNode(ctx context.Context, node *domain.ServiceNode) error {
	model := &TopologyServiceNodeModel{
		ID: node.ID, Name: node.Name, Namespace: node.Namespace,
		Environment: node.Environment, Status: string(node.Status),
		RequestRate: node.RequestRate, ErrorRate: node.ErrorRate,
		LatencyP99: node.LatencyP99, LatencyP95: node.LatencyP95,
		LatencyP50: node.LatencyP50, PodCount: node.PodCount,
		ReadyPods: node.ReadyPods, ServiceType: node.ServiceType,
		Maintainer: node.Maintainer, Team: node.Team,
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *gormTopologyRepository) BatchSaveServiceNodes(ctx context.Context, nodes []*domain.ServiceNode) error {
	models := make([]*TopologyServiceNodeModel, len(nodes))
	for i, n := range nodes {
		models[i] = &TopologyServiceNodeModel{
			ID: n.ID, Name: n.Name, Namespace: n.Namespace,
			Environment: n.Environment, Status: string(n.Status),
			RequestRate: n.RequestRate, ErrorRate: n.ErrorRate,
			LatencyP99: n.LatencyP99, PodCount: n.PodCount, ReadyPods: n.ReadyPods,
		}
	}
	return r.db.WithContext(ctx).Create(models).Error
}

func (r *gormTopologyRepository) GetServiceNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	var model TopologyServiceNodeModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNodeNotFound
		}
		return nil, err
	}
	return &domain.ServiceNode{ID: model.ID, Name: model.Name, Namespace: model.Namespace, Status: domain.ServiceStatus(model.Status), RequestRate: model.RequestRate, ErrorRate: model.ErrorRate, LatencyP99: model.LatencyP99, PodCount: model.PodCount, ReadyPods: model.ReadyPods}, nil
}

func (r *gormTopologyRepository) GetServiceNodeByName(ctx context.Context, namespace, name string) (*domain.ServiceNode, error) {
	var model TopologyServiceNodeModel
	if err := r.db.WithContext(ctx).First(&model, "namespace = ? AND name = ?", namespace, name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNodeNotFound
		}
		return nil, err
	}
	return &domain.ServiceNode{ID: model.ID, Name: model.Name, Namespace: model.Namespace, Status: domain.ServiceStatus(model.Status)}, nil
}

func (r *gormTopologyRepository) ListServiceNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.ServiceNode, int64, error) {
	var models []TopologyServiceNodeModel
	var total int64
	db := r.db.WithContext(ctx).Model(&TopologyServiceNodeModel{})
	if query.Namespace != "" {
		db = db.Where("namespace = ?", query.Namespace)
	}
	db.Count(&total)
	if err := db.Find(&models).Error; err != nil {
		return nil, 0, err
	}
	nodes := make([]*domain.ServiceNode, len(models))
	for i, m := range models {
		nodes[i] = &domain.ServiceNode{ID: m.ID, Name: m.Name, Namespace: m.Namespace, Status: domain.ServiceStatus(m.Status), RequestRate: m.RequestRate}
	}
	return nodes, total, nil
}

func (r *gormTopologyRepository) DeleteServiceNode(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&TopologyServiceNodeModel{}, "id = ?", id).Error
}

func (r *gormTopologyRepository) SaveCallEdge(ctx context.Context, edge *domain.CallEdge) error {
	model := &TopologyCallEdgeModel{
		ID: edge.ID, SourceID: edge.SourceID, TargetID: edge.TargetID,
		EdgeType: string(edge.EdgeType), IsDirect: edge.IsDirect,
		Confidence: edge.Confidence, Protocol: edge.Protocol, Method: edge.Method,
		RequestRate: edge.RequestRate, ErrorRate: edge.ErrorRate, LatencyP99: edge.LatencyP99,
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *gormTopologyRepository) BatchSaveCallEdges(ctx context.Context, edges []*domain.CallEdge) error {
	models := make([]*TopologyCallEdgeModel, len(edges))
	for i, e := range edges {
		models[i] = &TopologyCallEdgeModel{
			ID: e.ID, SourceID: e.SourceID, TargetID: e.TargetID,
			EdgeType: string(e.EdgeType), IsDirect: e.IsDirect,
			Confidence: e.Confidence, RequestRate: e.RequestRate,
		}
	}
	return r.db.WithContext(ctx).Create(models).Error
}

func (r *gormTopologyRepository) GetCallEdge(ctx context.Context, id uuid.UUID) (*domain.CallEdge, error) {
	var model TopologyCallEdgeModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrEdgeNotFound
		}
		return nil, err
	}
	return &domain.CallEdge{ID: model.ID, SourceID: model.SourceID, TargetID: model.TargetID, EdgeType: domain.EdgeType(model.EdgeType), IsDirect: model.IsDirect, Confidence: model.Confidence, RequestRate: model.RequestRate}, nil
}

func (r *gormTopologyRepository) GetCallEdgeByEndpoints(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.CallEdge, error) {
	var model TopologyCallEdgeModel
	if err := r.db.WithContext(ctx).First(&model, "source_id = ? AND target_id = ?", sourceID, targetID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrEdgeNotFound
		}
		return nil, err
	}
	return &domain.CallEdge{ID: model.ID, SourceID: model.SourceID, TargetID: model.TargetID, EdgeType: domain.EdgeType(model.EdgeType)}, nil
}

func (r *gormTopologyRepository) ListCallEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.CallEdge, int64, error) {
	var models []TopologyCallEdgeModel
	var total int64
	db := r.db.WithContext(ctx).Model(&TopologyCallEdgeModel{})
	db.Count(&total)
	if err := db.Find(&models).Error; err != nil {
		return nil, 0, err
	}
	edges := make([]*domain.CallEdge, len(models))
	for i, m := range models {
		edges[i] = &domain.CallEdge{ID: m.ID, SourceID: m.SourceID, TargetID: m.TargetID, EdgeType: domain.EdgeType(m.EdgeType), RequestRate: m.RequestRate}
	}
	return edges, total, nil
}

func (r *gormTopologyRepository) ListCallEdgesBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.CallEdge, error) {
	var models []TopologyCallEdgeModel
	if err := r.db.WithContext(ctx).Find(&models, "source_id = ?", sourceID).Error; err != nil {
		return nil, err
	}
	edges := make([]*domain.CallEdge, len(models))
	for i, m := range models {
		edges[i] = &domain.CallEdge{ID: m.ID, SourceID: m.SourceID, TargetID: m.TargetID, EdgeType: domain.EdgeType(m.EdgeType)}
	}
	return edges, nil
}

func (r *gormTopologyRepository) ListCallEdgesByTarget(ctx context.Context, targetID uuid.UUID) ([]*domain.CallEdge, error) {
	var models []TopologyCallEdgeModel
	if err := r.db.WithContext(ctx).Find(&models, "target_id = ?", targetID).Error; err != nil {
		return nil, err
	}
	edges := make([]*domain.CallEdge, len(models))
	for i, m := range models {
		edges[i] = &domain.CallEdge{ID: m.ID, SourceID: m.SourceID, TargetID: m.TargetID, EdgeType: domain.EdgeType(m.EdgeType)}
	}
	return edges, nil
}

func (r *gormTopologyRepository) DeleteCallEdge(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&TopologyCallEdgeModel{}, "id = ?", id).Error
}

func (r *gormTopologyRepository) SaveNetworkNode(ctx context.Context, node *domain.NetworkNode) error {
	return nil
}
func (r *gormTopologyRepository) BatchSaveNetworkNodes(ctx context.Context, nodes []*domain.NetworkNode) error {
	return nil
}
func (r *gormTopologyRepository) GetNetworkNode(ctx context.Context, id uuid.UUID) (*domain.NetworkNode, error) {
	return nil, domain.ErrNodeNotFound
}
func (r *gormTopologyRepository) GetNetworkNodeByIP(ctx context.Context, ip string) (*domain.NetworkNode, error) {
	return nil, domain.ErrNodeNotFound
}
func (r *gormTopologyRepository) ListNetworkNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkNode, int64, error) {
	return nil, 0, nil
}
func (r *gormTopologyRepository) DeleteNetworkNode(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (r *gormTopologyRepository) SaveNetworkEdge(ctx context.Context, edge *domain.NetworkEdge) error {
	return nil
}
func (r *gormTopologyRepository) BatchSaveNetworkEdges(ctx context.Context, edges []*domain.NetworkEdge) error {
	return nil
}
func (r *gormTopologyRepository) GetNetworkEdge(ctx context.Context, id uuid.UUID) (*domain.NetworkEdge, error) {
	return nil, domain.ErrEdgeNotFound
}
func (r *gormTopologyRepository) ListNetworkEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkEdge, int64, error) {
	return nil, 0, nil
}
func (r *gormTopologyRepository) DeleteNetworkEdge(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (r *gormTopologyRepository) SaveTopologyChange(ctx context.Context, change *domain.TopologyChange) error {
	return nil
}
func (r *gormTopologyRepository) ListTopologyChanges(ctx context.Context, from, to time.Time, entityType string) ([]*domain.TopologyChange, error) {
	return []*domain.TopologyChange{}, nil
}
func (r *gormTopologyRepository) SaveTopologySnapshot(ctx context.Context, graphType string, snapshot *domain.ServiceTopology) error {
	return nil
}
func (r *gormTopologyRepository) GetTopologySnapshot(ctx context.Context, graphType string, timestamp time.Time) (*domain.ServiceTopology, error) {
	return nil, domain.ErrNodeNotFound
}
func (r *gormTopologyRepository) CleanupOldData(ctx context.Context, retention time.Duration) error {
	return nil
}

var _ testcontainers.Logger = (*noopLogger)(nil)

type noopLogger struct{}

func (n *noopLogger) Printf(format string, v ...interface{}) {}
