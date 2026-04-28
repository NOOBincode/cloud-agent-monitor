package application

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGraphAnalyzer(t *testing.T) {
	graph := NewInMemoryGraph()
	analyzer := NewGraphAnalyzer(graph)

	assert.NotNil(t, analyzer)
	assert.Equal(t, graph, analyzer.graph)
}

func TestGraphAnalyzer_AnalyzeImpact(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.AnalyzeImpact(context.Background(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("returns error when node not found", func(t *testing.T) {
		graph := NewInMemoryGraph()
		analyzer := NewGraphAnalyzer(graph)

		_, err := analyzer.AnalyzeImpact(context.Background(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})

	t.Run("analyzes single node with no connections", func(t *testing.T) {
		graph := NewInMemoryGraph()
		node := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "isolated-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
		}
		graph.Rebuild([]*domain.ServiceNode{node}, []*domain.CallEdge{})

		analyzer := NewGraphAnalyzer(graph)
		result, err := analyzer.AnalyzeImpact(context.Background(), node.ID, 5)

		require.NoError(t, err)
		assert.Equal(t, node.ID, result.RootService.ID)
		assert.Equal(t, 0, result.TotalAffected)
		assert.Empty(t, result.Upstream)
		assert.Empty(t, result.Downstream)
	})

	t.Run("analyzes node with upstream and downstream", func(t *testing.T) {
		graph := NewInMemoryGraph()

		rootNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "root-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
		}
		upstreamNode := &domain.ServiceNode{
			ID:          uuid.New(),
			Name:        "upstream-service",
			Namespace:   "default",
			Status:      domain.ServiceStatusHealthy,
			RequestRate: 500,
		}
		downstreamNode := &domain.ServiceNode{
			ID:          uuid.New(),
			Name:        "downstream-service",
			Namespace:   "default",
			Status:      domain.ServiceStatusUnhealthy,
			RequestRate: 200,
		}

		nodes := []*domain.ServiceNode{rootNode, upstreamNode, downstreamNode}

		edges := []*domain.CallEdge{
			{
				ID:       uuid.New(),
				SourceID: upstreamNode.ID,
				TargetID: rootNode.ID,
				EdgeType: domain.EdgeTypeHTTP,
			},
			{
				ID:       uuid.New(),
				SourceID: rootNode.ID,
				TargetID: downstreamNode.ID,
				EdgeType: domain.EdgeTypeHTTP,
			},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		result, err := analyzer.AnalyzeImpact(context.Background(), rootNode.ID, 5)

		require.NoError(t, err)
		assert.Equal(t, rootNode.ID, result.RootService.ID)
		assert.Equal(t, 2, result.TotalAffected)
		assert.Len(t, result.Upstream, 1)
		assert.Len(t, result.Downstream, 1)
		assert.Equal(t, upstreamNode.ID, result.Upstream[0].Node.ID)
		assert.Equal(t, downstreamNode.ID, result.Downstream[0].Node.ID)
	})

	t.Run("respects max depth", func(t *testing.T) {
		graph := NewInMemoryGraph()

		node1 := &domain.ServiceNode{ID: uuid.New(), Name: "node-1", Namespace: "default", Status: domain.ServiceStatusHealthy}
		node2 := &domain.ServiceNode{ID: uuid.New(), Name: "node-2", Namespace: "default", Status: domain.ServiceStatusHealthy}
		node3 := &domain.ServiceNode{ID: uuid.New(), Name: "node-3", Namespace: "default", Status: domain.ServiceStatusHealthy}
		node4 := &domain.ServiceNode{ID: uuid.New(), Name: "node-4", Namespace: "default", Status: domain.ServiceStatusHealthy}

		nodes := []*domain.ServiceNode{node1, node2, node3, node4}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: node1.ID, TargetID: node2.ID},
			{ID: uuid.New(), SourceID: node2.ID, TargetID: node3.ID},
			{ID: uuid.New(), SourceID: node3.ID, TargetID: node4.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		result, err := analyzer.AnalyzeImpact(context.Background(), node1.ID, 2)

		require.NoError(t, err)
		assert.LessOrEqual(t, result.DownstreamDepth, 2)
	})
}

func TestGraphAnalyzer_CalculateImpactScore(t *testing.T) {
	graph := NewInMemoryGraph()
	analyzer := NewGraphAnalyzer(graph)

	tests := []struct {
		name     string
		node     *domain.ServiceNode
		depth    int
		minScore float64
		maxScore float64
	}{
		{
			name: "healthy service at depth 1",
			node: &domain.ServiceNode{
				Status:      domain.ServiceStatusHealthy,
				RequestRate: 50,
			},
			depth:    1,
			minScore: 0.9,
			maxScore: 1.5,
		},
		{
			name: "unhealthy service at depth 2",
			node: &domain.ServiceNode{
				Status:      domain.ServiceStatusUnhealthy,
				RequestRate: 50,
			},
			depth:    2,
			minScore: 0.9,
			maxScore: 1.5,
		},
		{
			name: "high traffic service",
			node: &domain.ServiceNode{
				Status:      domain.ServiceStatusHealthy,
				RequestRate: 2000,
			},
			depth:    1,
			minScore: 1.0,
			maxScore: 3.0,
		},
		{
			name: "warning status service",
			node: &domain.ServiceNode{
				Status:      domain.ServiceStatusWarning,
				RequestRate: 100,
			},
			depth:    1,
			minScore: 1.0,
			maxScore: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.calculateImpactScore(tt.node, tt.depth)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}

func TestGraphAnalyzer_FindPath(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.FindPath(context.Background(), uuid.New(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("returns error when source not found", func(t *testing.T) {
		graph := NewInMemoryGraph()
		analyzer := NewGraphAnalyzer(graph)

		_, err := analyzer.FindPath(context.Background(), uuid.New(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})

	t.Run("returns error when target not found", func(t *testing.T) {
		graph := NewInMemoryGraph()
		source := &domain.ServiceNode{ID: uuid.New(), Name: "source", Namespace: "default"}
		graph.Rebuild([]*domain.ServiceNode{source}, []*domain.CallEdge{})

		analyzer := NewGraphAnalyzer(graph)
		_, err := analyzer.FindPath(context.Background(), source.ID, uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})

	t.Run("returns error when no path exists", func(t *testing.T) {
		graph := NewInMemoryGraph()

		source := &domain.ServiceNode{ID: uuid.New(), Name: "source", Namespace: "default"}
		target := &domain.ServiceNode{ID: uuid.New(), Name: "target", Namespace: "default"}

		graph.Rebuild([]*domain.ServiceNode{source, target}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		_, err := analyzer.FindPath(context.Background(), source.ID, target.ID, 5)
		assert.ErrorIs(t, err, domain.ErrPathNotFound)
	})

	t.Run("finds direct path", func(t *testing.T) {
		graph := NewInMemoryGraph()

		source := &domain.ServiceNode{ID: uuid.New(), Name: "source", Namespace: "default"}
		target := &domain.ServiceNode{ID: uuid.New(), Name: "target", Namespace: "default"}

		nodes := []*domain.ServiceNode{source, target}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: source.ID, TargetID: target.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		result, err := analyzer.FindPath(context.Background(), source.ID, target.ID, 5)

		require.NoError(t, err)
		assert.Equal(t, source.ID, result.SourceID)
		assert.Equal(t, target.ID, result.TargetID)
		assert.Len(t, result.Paths, 1)
		assert.Len(t, result.Paths[0], 2)
	})

	t.Run("finds multiple paths", func(t *testing.T) {
		graph := NewInMemoryGraph()

		nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default"}
		nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default"}
		nodeC := &domain.ServiceNode{ID: uuid.New(), Name: "c", Namespace: "default"}
		nodeD := &domain.ServiceNode{ID: uuid.New(), Name: "d", Namespace: "default"}

		nodes := []*domain.ServiceNode{nodeA, nodeB, nodeC, nodeD}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeB.ID},
			{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeC.ID},
			{ID: uuid.New(), SourceID: nodeB.ID, TargetID: nodeD.ID},
			{ID: uuid.New(), SourceID: nodeC.ID, TargetID: nodeD.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		result, err := analyzer.FindPath(context.Background(), nodeA.ID, nodeD.ID, 5)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Paths), 1)
	})
}

func TestGraphAnalyzer_FindShortestPath(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.FindShortestPath(context.Background(), uuid.New(), uuid.New())
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("returns error when source not found", func(t *testing.T) {
		graph := NewInMemoryGraph()
		analyzer := NewGraphAnalyzer(graph)

		_, err := analyzer.FindShortestPath(context.Background(), uuid.New(), uuid.New())
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})

	t.Run("finds shortest path", func(t *testing.T) {
		graph := NewInMemoryGraph()

		nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default"}
		nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default"}
		nodeC := &domain.ServiceNode{ID: uuid.New(), Name: "c", Namespace: "default"}

		nodes := []*domain.ServiceNode{nodeA, nodeB, nodeC}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeB.ID},
			{ID: uuid.New(), SourceID: nodeB.ID, TargetID: nodeC.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		path, err := analyzer.FindShortestPath(context.Background(), nodeA.ID, nodeC.ID)

		require.NoError(t, err)
		assert.Len(t, path, 3)
		assert.Equal(t, nodeA.ID, path[0].NodeID)
		assert.Equal(t, nodeC.ID, path[2].NodeID)
	})

	t.Run("returns error when no path exists", func(t *testing.T) {
		graph := NewInMemoryGraph()

		nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default"}
		nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default"}

		graph.Rebuild([]*domain.ServiceNode{nodeA, nodeB}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		_, err := analyzer.FindShortestPath(context.Background(), nodeA.ID, nodeB.ID)
		assert.ErrorIs(t, err, domain.ErrPathNotFound)
	})
}

func TestGraphAnalyzer_FindAnomalies(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.FindAnomalies(context.Background())
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("detects unhealthy service", func(t *testing.T) {
		graph := NewInMemoryGraph()

		unhealthyNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "unhealthy-service",
			Namespace: "default",
			Status:    domain.ServiceStatusUnhealthy,
		}
		healthyNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "healthy-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
		}

		graph.Rebuild([]*domain.ServiceNode{unhealthyNode, healthyNode}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundUnhealthy bool
		for _, a := range anomalies {
			if a.Type == "unhealthy_service" && a.NodeID == unhealthyNode.ID {
				foundUnhealthy = true
				assert.Equal(t, "high", a.Severity)
			}
		}
		assert.True(t, foundUnhealthy, "should detect unhealthy service")
	})

	t.Run("detects high error rate", func(t *testing.T) {
		graph := NewInMemoryGraph()

		highErrorNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "high-error-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
			ErrorRate: 0.10,
		}

		graph.Rebuild([]*domain.ServiceNode{highErrorNode}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundHighError bool
		for _, a := range anomalies {
			if a.Type == "high_error_rate" && a.NodeID == highErrorNode.ID {
				foundHighError = true
				assert.Equal(t, "medium", a.Severity)
				assert.Equal(t, 0.10, a.Metrics["error_rate"])
			}
		}
		assert.True(t, foundHighError, "should detect high error rate")
	})

	t.Run("detects high latency", func(t *testing.T) {
		graph := NewInMemoryGraph()

		highLatencyNode := &domain.ServiceNode{
			ID:         uuid.New(),
			Name:       "high-latency-service",
			Namespace:  "default",
			Status:     domain.ServiceStatusHealthy,
			LatencyP99: 2000.0,
		}

		graph.Rebuild([]*domain.ServiceNode{highLatencyNode}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundHighLatency bool
		for _, a := range anomalies {
			if a.Type == "high_latency" && a.NodeID == highLatencyNode.ID {
				foundHighLatency = true
				assert.Equal(t, "medium", a.Severity)
			}
		}
		assert.True(t, foundHighLatency, "should detect high latency")
	})

	t.Run("detects pod degradation", func(t *testing.T) {
		graph := NewInMemoryGraph()

		degradedNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "degraded-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
			PodCount:  3,
			ReadyPods: 1,
		}

		graph.Rebuild([]*domain.ServiceNode{degradedNode}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundPodDegradation bool
		for _, a := range anomalies {
			if a.Type == "pod_degradation" && a.NodeID == degradedNode.ID {
				foundPodDegradation = true
				assert.Equal(t, "medium", a.Severity)
			}
		}
		assert.True(t, foundPodDegradation, "should detect pod degradation")
	})

	t.Run("detects orphan services", func(t *testing.T) {
		graph := NewInMemoryGraph()

		orphanNode := &domain.ServiceNode{
			ID:        uuid.New(),
			Name:      "orphan-service",
			Namespace: "default",
			Status:    domain.ServiceStatusHealthy,
		}

		graph.Rebuild([]*domain.ServiceNode{orphanNode}, []*domain.CallEdge{})
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundOrphan bool
		for _, a := range anomalies {
			if a.Type == "orphan_service" && a.NodeID == orphanNode.ID {
				foundOrphan = true
				assert.Equal(t, "low", a.Severity)
			}
		}
		assert.True(t, foundOrphan, "should detect orphan service")
	})

	t.Run("detects circular dependency", func(t *testing.T) {
		graph := NewInMemoryGraph()

		nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default", Status: domain.ServiceStatusHealthy}
		nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default", Status: domain.ServiceStatusHealthy}

		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeB.ID},
			{ID: uuid.New(), SourceID: nodeB.ID, TargetID: nodeA.ID},
		}

		graph.Rebuild([]*domain.ServiceNode{nodeA, nodeB}, edges)
		analyzer := NewGraphAnalyzer(graph)

		anomalies, err := analyzer.FindAnomalies(context.Background())

		require.NoError(t, err)
		var foundCycle bool
		for _, a := range anomalies {
			if a.Type == "circular_dependency" {
				foundCycle = true
				assert.Equal(t, "high", a.Severity)
			}
		}
		assert.True(t, foundCycle, "should detect circular dependency")
	})
}

func TestGraphAnalyzer_CalculateCentrality(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.CalculateCentrality(context.Background())
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("calculates centrality for nodes", func(t *testing.T) {
		graph := NewInMemoryGraph()

		nodeA := &domain.ServiceNode{ID: uuid.New(), Name: "a", Namespace: "default"}
		nodeB := &domain.ServiceNode{ID: uuid.New(), Name: "b", Namespace: "default"}
		nodeC := &domain.ServiceNode{ID: uuid.New(), Name: "c", Namespace: "default"}

		nodes := []*domain.ServiceNode{nodeA, nodeB, nodeC}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: nodeA.ID, TargetID: nodeB.ID},
			{ID: uuid.New(), SourceID: nodeB.ID, TargetID: nodeC.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		centrality, err := analyzer.CalculateCentrality(context.Background())

		require.NoError(t, err)
		assert.Len(t, centrality, 3)
		for _, node := range nodes {
			_, exists := centrality[node.ID]
			assert.True(t, exists, "centrality should exist for node %s", node.Name)
		}
	})
}

func TestGraphAnalyzer_FindClusters(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.FindClusters(context.Background())
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("finds connected components", func(t *testing.T) {
		graph := NewInMemoryGraph()

		cluster1A := &domain.ServiceNode{ID: uuid.New(), Name: "cluster1-a", Namespace: "default"}
		cluster1B := &domain.ServiceNode{ID: uuid.New(), Name: "cluster1-b", Namespace: "default"}
		cluster2A := &domain.ServiceNode{ID: uuid.New(), Name: "cluster2-a", Namespace: "default"}
		cluster2B := &domain.ServiceNode{ID: uuid.New(), Name: "cluster2-b", Namespace: "default"}

		nodes := []*domain.ServiceNode{cluster1A, cluster1B, cluster2A, cluster2B}
		edges := []*domain.CallEdge{
			{ID: uuid.New(), SourceID: cluster1A.ID, TargetID: cluster1B.ID},
			{ID: uuid.New(), SourceID: cluster2A.ID, TargetID: cluster2B.ID},
		}

		graph.Rebuild(nodes, edges)
		analyzer := NewGraphAnalyzer(graph)

		clusters, err := analyzer.FindClusters(context.Background())

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(clusters), 2)
	})
}

func TestPriorityQueue(t *testing.T) {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	path1 := &Path{Nodes: []uuid.UUID{uuid.New()}, Cost: 10.0}
	path2 := &Path{Nodes: []uuid.UUID{uuid.New()}, Cost: 5.0}
	path3 := &Path{Nodes: []uuid.UUID{uuid.New()}, Cost: 15.0}

	heap.Push(&pq, path1)
	heap.Push(&pq, path2)
	heap.Push(&pq, path3)

	assert.Equal(t, 3, pq.Len())

	popped := heap.Pop(&pq).(*Path)
	assert.Equal(t, 5.0, popped.Cost)

	popped = heap.Pop(&pq).(*Path)
	assert.Equal(t, 10.0, popped.Cost)

	popped = heap.Pop(&pq).(*Path)
	assert.Equal(t, 15.0, popped.Cost)
}

func TestGraphAnalyzer_AnalyzeImpactWeighted(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.AnalyzeImpactWeighted(context.Background(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("returns error when node not found", func(t *testing.T) {
		graph := NewInMemoryGraph()
		analyzer := NewGraphAnalyzer(graph)
		_, err := analyzer.AnalyzeImpactWeighted(context.Background(), uuid.New(), 5)
		assert.ErrorIs(t, err, domain.ErrNodeNotFound)
	})

	t.Run("calculates weighted impact with importance", func(t *testing.T) {
		graph := NewInMemoryGraph()
		rootID := uuid.New()
		upID := uuid.New()
		downID := uuid.New()

		graph.Rebuild(
			[]*domain.ServiceNode{
				{ID: rootID, Name: "root", Namespace: "default", Importance: domain.ImportanceCritical},
				{ID: upID, Name: "upstream", Namespace: "default", Importance: domain.ImportanceImportant},
				{ID: downID, Name: "downstream", Namespace: "default", Importance: domain.ImportanceNormal},
			},
			[]*domain.CallEdge{
				{ID: uuid.New(), SourceID: upID, TargetID: rootID},
				{ID: uuid.New(), SourceID: rootID, TargetID: downID},
			},
		)

		analyzer := NewGraphAnalyzer(graph)
		result, err := analyzer.AnalyzeImpactWeighted(context.Background(), rootID, 5)
		require.NoError(t, err)

		assert.True(t, result.WeightedScore > 0)
		assert.Equal(t, 2, result.TotalAffected)
		assert.Contains(t, result.ImpactByImportance, domain.ImportanceImportant)
		assert.Contains(t, result.ImpactByImportance, domain.ImportanceNormal)
		assert.True(t, len(result.CriticalServices) >= 1)
	})
}

func TestGraphAnalyzer_DetectBottlenecks(t *testing.T) {
	t.Run("returns error when graph is nil", func(t *testing.T) {
		analyzer := NewGraphAnalyzer(nil)
		_, err := analyzer.DetectBottlenecks(context.Background())
		assert.ErrorIs(t, err, domain.ErrGraphNotReady)
	})

	t.Run("detects bottleneck nodes", func(t *testing.T) {
		graph := NewInMemoryGraph()
		bottleneckID := uuid.New()
		normalID := uuid.New()

		graph.Rebuild(
			[]*domain.ServiceNode{
				{ID: bottleneckID, Name: "bottleneck", Namespace: "default", Importance: domain.ImportanceCritical, ErrorRate: 0.5, LatencyP99: 2000},
				{ID: normalID, Name: "normal", Namespace: "default", Importance: domain.ImportanceEdge, ErrorRate: 0, LatencyP99: 10},
			},
			[]*domain.CallEdge{
				{ID: uuid.New(), SourceID: normalID, TargetID: bottleneckID},
			},
		)

		analyzer := NewGraphAnalyzer(graph)
		bottlenecks, err := analyzer.DetectBottlenecks(context.Background())
		require.NoError(t, err)

		assert.True(t, len(bottlenecks) > 0, "should detect bottleneck nodes")
	})
}

func TestGraphAnalyzer_TimeDependentWeight(t *testing.T) {
	tdw := &TimeDependentWeight{
		BaseWeight:  1.0,
		TimeFactors: map[time.Weekday]map[int]float64{
			time.Monday: {9: 1.5, 18: 0.8},
		},
	}

	t.Run("returns base weight when no factor configured", func(t *testing.T) {
		w := tdw.GetWeight(time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC))
		assert.Equal(t, 1.0, w)
	})

	t.Run("returns adjusted weight with factor", func(t *testing.T) {
		w := tdw.GetWeight(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))
		assert.Equal(t, 1.5, w)
	})
}
