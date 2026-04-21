package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestServiceStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ServiceStatus
		want   string
	}{
		{
			name:   "healthy status",
			status: ServiceStatusHealthy,
			want:   "healthy",
		},
		{
			name:   "unhealthy status",
			status: ServiceStatusUnhealthy,
			want:   "unhealthy",
		},
		{
			name:   "warning status",
			status: ServiceStatusWarning,
			want:   "warning",
		},
		{
			name:   "unknown status",
			status: ServiceStatusUnknown,
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.status))
		})
	}
}

func TestEdgeType(t *testing.T) {
	tests := []struct {
		name     string
		edgeType EdgeType
		want     string
	}{
		{
			name:     "http edge type",
			edgeType: EdgeTypeHTTP,
			want:     "http",
		},
		{
			name:     "grpc edge type",
			edgeType: EdgeTypeGRPC,
			want:     "grpc",
		},
		{
			name:     "database edge type",
			edgeType: EdgeTypeDatabase,
			want:     "database",
		},
		{
			name:     "cache edge type",
			edgeType: EdgeTypeCache,
			want:     "cache",
		},
		{
			name:     "message queue edge type",
			edgeType: EdgeTypeMessageQueue,
			want:     "mq",
		},
		{
			name:     "indirect edge type",
			edgeType: EdgeTypeIndirect,
			want:     "indirect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.edgeType))
		})
	}
}

func TestNetworkLayer(t *testing.T) {
	tests := []struct {
		name  string
		layer NetworkLayer
		want  string
	}{
		{
			name:  "pod layer",
			layer: NetworkLayerPod,
			want:  "pod",
		},
		{
			name:  "node layer",
			layer: NetworkLayerNode,
			want:  "node",
		},
		{
			name:  "cluster layer",
			layer: NetworkLayerCluster,
			want:  "cluster",
		},
		{
			name:  "ingress layer",
			layer: NetworkLayerIngress,
			want:  "ingress",
		},
		{
			name:  "external layer",
			layer: NetworkLayerExternal,
			want:  "external",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.layer))
		})
	}
}

func TestImpactLevel(t *testing.T) {
	tests := []struct {
		name  string
		level ImpactLevel
		want  string
	}{
		{
			name:  "critical level",
			level: ImpactLevelCritical,
			want:  "critical",
		},
		{
			name:  "high level",
			level: ImpactLevelHigh,
			want:  "high",
		},
		{
			name:  "medium level",
			level: ImpactLevelMedium,
			want:  "medium",
		},
		{
			name:  "low level",
			level: ImpactLevelLow,
			want:  "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.level))
		})
	}
}

func TestServiceNode(t *testing.T) {
	now := time.Now()
	node := &ServiceNode{
		ID:          uuid.New(),
		Name:        "user-service",
		Namespace:   "production",
		Environment: "prod",
		Status:      ServiceStatusHealthy,
		Labels: map[string]string{
			"app":     "user-service",
			"version": "v1.0.0",
		},
		RequestRate: 1000.5,
		ErrorRate:   0.5,
		LatencyP99:  150.0,
		LatencyP95:  100.0,
		LatencyP50:  50.0,
		PodCount:    3,
		ReadyPods:   3,
		ServiceType: "ClusterIP",
		Maintainer:  "team-backend",
		Team:        "backend",
		UpdatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, node.ID)
	assert.Equal(t, "user-service", node.Name)
	assert.Equal(t, "production", node.Namespace)
	assert.Equal(t, ServiceStatusHealthy, node.Status)
	assert.Equal(t, 1000.5, node.RequestRate)
	assert.Equal(t, 0.5, node.ErrorRate)
	assert.Equal(t, 3, node.PodCount)
	assert.Equal(t, 3, node.ReadyPods)
}

func TestCallEdge(t *testing.T) {
	now := time.Now()
	sourceID := uuid.New()
	targetID := uuid.New()

	edge := &CallEdge{
		ID:       uuid.New(),
		SourceID: sourceID,
		TargetID: targetID,
		EdgeType: EdgeTypeHTTP,
		IsDirect: true,
		Confidence: 0.9,
		Protocol:   "HTTP/1.1",
		Method:     "GET",
		RequestRate: 500.0,
		ErrorRate:   1.0,
		LatencyP99:  200.0,
		TargetInstances: []string{"user-service-1", "user-service-2"},
		UpdatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, edge.ID)
	assert.Equal(t, sourceID, edge.SourceID)
	assert.Equal(t, targetID, edge.TargetID)
	assert.Equal(t, EdgeTypeHTTP, edge.EdgeType)
	assert.True(t, edge.IsDirect)
	assert.Equal(t, 0.9, edge.Confidence)
	assert.Equal(t, 500.0, edge.RequestRate)
	assert.Len(t, edge.TargetInstances, 2)
}

func TestNetworkNode(t *testing.T) {
	now := time.Now()
	node := &NetworkNode{
		ID:        uuid.New(),
		Name:      "user-pod-12345",
		Type:      "pod",
		Layer:     NetworkLayerPod,
		IPAddress: "10.0.0.100",
		CIDR:      "10.0.0.0/24",
		Ports:     []int{8080, 9090},
		Namespace: "production",
		PodName:   "user-pod-12345",
		NodeName:  "node-1",
		Zone:      "zone-a",
		DataCenter: "dc-1",
		Connections: 100,
		BytesIn:    1024000,
		BytesOut:   2048000,
		PacketsIn:  10000,
		PacketsOut: 20000,
		PacketLoss: 0.1,
		Latency:    5.0,
		UpdatedAt:  now,
	}

	assert.NotEqual(t, uuid.Nil, node.ID)
	assert.Equal(t, "pod", node.Type)
	assert.Equal(t, NetworkLayerPod, node.Layer)
	assert.Equal(t, "10.0.0.100", node.IPAddress)
	assert.Len(t, node.Ports, 2)
	assert.Equal(t, int64(100), node.Connections)
}

func TestNetworkEdge(t *testing.T) {
	now := time.Now()
	sourceID := uuid.New()
	targetID := uuid.New()

	edge := &NetworkEdge{
		ID:       uuid.New(),
		SourceID: sourceID,
		TargetID: targetID,
		SourceIP:   "10.0.0.100",
		TargetIP:   "10.0.0.200",
		SourcePort: 54321,
		TargetPort: 8080,
		Protocol:   "TCP",
		BytesSent:     1024000,
		BytesReceived: 2048000,
		PacketsSent:   10000,
		PacketsLost:   5,
		ConnectionCount: 50,
		Established:    45,
		TimeWait:       3,
		CloseWait:      2,
		UpdatedAt:      now,
	}

	assert.NotEqual(t, uuid.Nil, edge.ID)
	assert.Equal(t, sourceID, edge.SourceID)
	assert.Equal(t, targetID, edge.TargetID)
	assert.Equal(t, "TCP", edge.Protocol)
	assert.Equal(t, int64(1024000), edge.BytesSent)
	assert.Equal(t, 50, edge.ConnectionCount)
}

func TestServiceTopology(t *testing.T) {
	now := time.Now()
	topology := &ServiceTopology{
		ID:        uuid.New(),
		Timestamp: now,
		Nodes: []*ServiceNode{
			{ID: uuid.New(), Name: "service-a", Namespace: "default"},
			{ID: uuid.New(), Name: "service-b", Namespace: "default"},
		},
		Edges: []*CallEdge{
			{ID: uuid.New(), SourceID: uuid.New(), TargetID: uuid.New()},
		},
		Hash: "abc123",
	}

	assert.NotEqual(t, uuid.Nil, topology.ID)
	assert.Len(t, topology.Nodes, 2)
	assert.Len(t, topology.Edges, 1)
	assert.Equal(t, "abc123", topology.Hash)
}

func TestNetworkTopology(t *testing.T) {
	now := time.Now()
	topology := &NetworkTopology{
		ID:        uuid.New(),
		Timestamp: now,
		Nodes: []*NetworkNode{
			{ID: uuid.New(), Name: "node-1", Type: "pod"},
		},
		Edges: []*NetworkEdge{
			{ID: uuid.New(), SourceID: uuid.New(), TargetID: uuid.New()},
		},
		Hash: "def456",
	}

	assert.NotEqual(t, uuid.Nil, topology.ID)
	assert.Len(t, topology.Nodes, 1)
	assert.Len(t, topology.Edges, 1)
}

func TestImpactResult(t *testing.T) {
	now := time.Now()
	rootService := &ServiceNode{
		ID:   uuid.New(),
		Name: "root-service",
	}

	result := &ImpactResult{
		RootService:     rootService,
		RootServiceID:   rootService.ID,
		RootServiceName: "root-service",
		TotalAffected:   5,
		UpstreamDepth:   2,
		DownstreamDepth: 3,
		Upstream: []*ImpactNode{
			{Node: &ServiceNode{Name: "upstream-1"}, Depth: 1, Impact: 0.8, IsCritical: true},
		},
		Downstream: []*ImpactNode{
			{Node: &ServiceNode{Name: "downstream-1"}, Depth: 1, Impact: 0.6, IsCritical: false},
		},
		CriticalPath: []PathHop{
			{NodeID: uuid.New(), NodeName: "hop-1"},
		},
		AnalyzedAt: now,
		AffectedServices: []AffectedService{
			{ServiceID: uuid.New(), ServiceName: "affected-1", ImpactLevel: ImpactLevelHigh},
		},
	}

	assert.Equal(t, rootService.ID, result.RootServiceID)
	assert.Equal(t, 5, result.TotalAffected)
	assert.Equal(t, 2, result.UpstreamDepth)
	assert.Equal(t, 3, result.DownstreamDepth)
	assert.Len(t, result.Upstream, 1)
	assert.Len(t, result.Downstream, 1)
	assert.Len(t, result.AffectedServices, 1)
}

func TestPathResult(t *testing.T) {
	now := time.Now()
	sourceID := uuid.New()
	targetID := uuid.New()

	result := &PathResult{
		SourceID:     sourceID,
		TargetID:     targetID,
		SourceName:   "source-service",
		TargetName:   "target-service",
		Paths: [][]PathHop{
			{{NodeID: sourceID}, {NodeID: targetID}},
		},
		ShortestPath: []PathHop{
			{NodeID: sourceID, NodeName: "source"},
			{NodeID: targetID, NodeName: "target"},
		},
		ShortestHops: 2,
		FoundAt:      now,
	}

	assert.Equal(t, sourceID, result.SourceID)
	assert.Equal(t, targetID, result.TargetID)
	assert.Len(t, result.Paths, 1)
	assert.Len(t, result.ShortestPath, 2)
	assert.Equal(t, 2, result.ShortestHops)
}

func TestTopologyAnomaly(t *testing.T) {
	now := time.Now()
	nodeID := uuid.New()

	anomaly := &TopologyAnomaly{
		ID:          uuid.New(),
		Type:        "high_error_rate",
		NodeID:      nodeID,
		NodeName:    "user-service",
		Namespace:   "production",
		Description: "Error rate exceeds 5% threshold",
		Severity:    "high",
		Metrics: map[string]float64{
			"error_rate": 7.5,
			"threshold":  5.0,
		},
		DetectedAt: now,
		RelatedIDs: []uuid.UUID{uuid.New()},
	}

	assert.NotEqual(t, uuid.Nil, anomaly.ID)
	assert.Equal(t, "high_error_rate", anomaly.Type)
	assert.Equal(t, nodeID, anomaly.NodeID)
	assert.Equal(t, "high", anomaly.Severity)
	assert.Equal(t, 7.5, anomaly.Metrics["error_rate"])
}

func TestTopologyChange(t *testing.T) {
	now := time.Now()
	entityID := uuid.New()

	change := &TopologyChange{
		ID:          uuid.New(),
		Timestamp:   now,
		ChangeType:  "added",
		EntityType:  "service_node",
		EntityID:    entityID,
		EntityName:  "new-service",
		Description: "New service discovered",
		BeforeState: "",
		AfterState:  "healthy",
	}

	assert.NotEqual(t, uuid.Nil, change.ID)
	assert.Equal(t, "added", change.ChangeType)
	assert.Equal(t, "service_node", change.EntityType)
	assert.Equal(t, entityID, change.EntityID)
}

func TestTopologyQuery_HasNamespace(t *testing.T) {
	tests := []struct {
		name  string
		query TopologyQuery
		want  bool
	}{
		{
			name:  "single namespace",
			query: TopologyQuery{Namespace: "production"},
			want:  true,
		},
		{
			name:  "multiple namespaces",
			query: TopologyQuery{Namespaces: []string{"production", "staging"}},
			want:  true,
		},
		{
			name:  "both namespace fields",
			query: TopologyQuery{Namespace: "production", Namespaces: []string{"staging"}},
			want:  true,
		},
		{
			name:  "no namespace",
			query: TopologyQuery{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.query.HasNamespace())
		})
	}
}

func TestTopologyQuery_HasLabels(t *testing.T) {
	tests := []struct {
		name  string
		query TopologyQuery
		want  bool
	}{
		{
			name: "with labels",
			query: TopologyQuery{Labels: map[string]string{
				"app": "user-service",
			}},
			want: true,
		},
		{
			name:  "empty labels",
			query: TopologyQuery{Labels: map[string]string{}},
			want:  false,
		},
		{
			name:  "nil labels",
			query: TopologyQuery{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.query.HasLabels())
		})
	}
}

func TestAffectedService(t *testing.T) {
	service := AffectedService{
		ServiceID:   uuid.New(),
		ServiceName: "affected-service",
		ImpactLevel: ImpactLevelCritical,
		ImpactPath:  []string{"root-service", "intermediate-service", "affected-service"},
		HopCount:    2,
	}

	assert.NotEqual(t, uuid.Nil, service.ServiceID)
	assert.Equal(t, ImpactLevelCritical, service.ImpactLevel)
	assert.Len(t, service.ImpactPath, 3)
	assert.Equal(t, 2, service.HopCount)
}

func TestImpactNode(t *testing.T) {
	node := &ImpactNode{
		Node:       &ServiceNode{Name: "test-service"},
		Depth:      2,
		Impact:     0.75,
		IsCritical: true,
	}

	assert.Equal(t, "test-service", node.Node.Name)
	assert.Equal(t, 2, node.Depth)
	assert.Equal(t, 0.75, node.Impact)
	assert.True(t, node.IsCritical)
}

func TestPathHop(t *testing.T) {
	nodeID := uuid.New()
	edgeID := uuid.New()

	hop := PathHop{
		NodeID:    nodeID,
		NodeName:  "hop-service",
		Namespace: "production",
		EdgeID:    edgeID,
		Latency:   50.5,
	}

	assert.Equal(t, nodeID, hop.NodeID)
	assert.Equal(t, "hop-service", hop.NodeName)
	assert.Equal(t, "production", hop.Namespace)
	assert.Equal(t, 50.5, hop.Latency)
}
