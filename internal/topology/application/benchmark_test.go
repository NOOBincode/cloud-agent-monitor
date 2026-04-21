package application

import (
	"fmt"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
)

func buildBenchmarkGraph(nodeCount int) (*InMemoryGraph, uuid.UUID) {
	graph := NewInMemoryGraph()
	nodes := make([]*domain.ServiceNode, nodeCount)
	edges := make([]*domain.CallEdge, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		nodes[i] = &domain.ServiceNode{
			ID:          uuid.New(),
			Name:        fmt.Sprintf("service-%d", i),
			Namespace:   "default",
			Status:      domain.ServiceStatusHealthy,
			RequestRate: float64(i * 10),
			ErrorRate:   0.01,
			LatencyP99:  100.0,
		}
	}

	for i := 0; i < nodeCount-1; i++ {
		edges = append(edges, &domain.CallEdge{
			ID:          uuid.New(),
			SourceID:    nodes[i].ID,
			TargetID:    nodes[i+1].ID,
			EdgeType:    domain.EdgeTypeHTTP,
			IsDirect:    true,
			Confidence:  0.9,
			RequestRate: float64(i * 5),
		})
	}

	for i := 0; i < nodeCount/5; i++ {
		src := i * 5
		tgt := (i*5 + 3) % nodeCount
		edges = append(edges, &domain.CallEdge{
			ID:         uuid.New(),
			SourceID:   nodes[src].ID,
			TargetID:   nodes[tgt].ID,
			EdgeType:   domain.EdgeTypeGRPC,
			IsDirect:   true,
			Confidence: 0.85,
		})
	}

	graph.Rebuild(nodes, edges)
	return graph, nodes[0].ID
}

func buildStarGraph(fanout int) (*InMemoryGraph, uuid.UUID) {
	graph := NewInMemoryGraph()
	center := &domain.ServiceNode{
		ID: uuid.New(), Name: "center", Namespace: "default",
		Status: domain.ServiceStatusHealthy, RequestRate: 1000,
	}
	nodes := []*domain.ServiceNode{center}
	edges := make([]*domain.CallEdge, 0, fanout*2)

	for i := 0; i < fanout; i++ {
		upstream := &domain.ServiceNode{
			ID: uuid.New(), Name: fmt.Sprintf("upstream-%d", i), Namespace: "default",
			Status: domain.ServiceStatusHealthy, RequestRate: float64(100 + i*10),
		}
		nodes = append(nodes, upstream)
		edges = append(edges, &domain.CallEdge{
			ID: uuid.New(), SourceID: upstream.ID, TargetID: center.ID,
			EdgeType: domain.EdgeTypeHTTP, IsDirect: true, Confidence: 0.9,
		})

		downstream := &domain.ServiceNode{
			ID: uuid.New(), Name: fmt.Sprintf("downstream-%d", i), Namespace: "default",
			Status: domain.ServiceStatusHealthy, RequestRate: float64(50 + i*5),
		}
		nodes = append(nodes, downstream)
		edges = append(edges, &domain.CallEdge{
			ID: uuid.New(), SourceID: center.ID, TargetID: downstream.ID,
			EdgeType: domain.EdgeTypeGRPC, IsDirect: true, Confidence: 0.85,
		})
	}

	graph.Rebuild(nodes, edges)
	return graph, center.ID
}

func BenchmarkGraphAnalyzer_AnalyzeImpact(b *testing.B) {
	sizes := []int{10, 100, 500, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("linear_nodes=%d", size), func(b *testing.B) {
			graph, rootID := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = analyzer.AnalyzeImpact(b.Context(), rootID, 10)
			}
		})
	}

	fanouts := []int{5, 20, 50, 100}
	for _, fanout := range fanouts {
		b.Run(fmt.Sprintf("star_fanout=%d", fanout), func(b *testing.B) {
			graph, centerID := buildStarGraph(fanout)
			analyzer := NewGraphAnalyzer(graph)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = analyzer.AnalyzeImpact(b.Context(), centerID, 10)
			}
		})
	}
}

func BenchmarkGraphAnalyzer_FindPath(b *testing.B) {
	sizes := []int{10, 100, 500}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph, rootID := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			var targetID uuid.UUID
			allNodes := graph.GetAllNodes()
			if len(allNodes) > 1 {
				targetID = allNodes[len(allNodes)-1].ID
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = analyzer.FindPath(b.Context(), rootID, targetID, 10)
			}
		})
	}
}

func BenchmarkGraphAnalyzer_FindShortestPath(b *testing.B) {
	sizes := []int{10, 100, 500}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph, rootID := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			var targetID uuid.UUID
			allNodes := graph.GetAllNodes()
			if len(allNodes) > 1 {
				targetID = allNodes[len(allNodes)-1].ID
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = analyzer.FindShortestPath(b.Context(), rootID, targetID)
			}
		})
	}
}

func BenchmarkGraphAnalyzer_FindAnomalies(b *testing.B) {
	sizes := []int{10, 100, 500, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph, _ := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = analyzer.FindAnomalies(b.Context())
			}
		})
	}
}

func BenchmarkGraphAnalyzer_CalculateCentrality(b *testing.B) {
	sizes := []int{10, 50, 100}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph, _ := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = analyzer.CalculateCentrality(b.Context())
			}
		})
	}
}

func BenchmarkGraphAnalyzer_FindClusters(b *testing.B) {
	sizes := []int{10, 100, 500}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph, _ := buildBenchmarkGraph(size)
			analyzer := NewGraphAnalyzer(graph)
			b.ReportAllocs()
			for b.Loop() {
				_, _ = analyzer.FindClusters(b.Context())
			}
		})
	}
}

func BenchmarkInMemoryGraph_Rebuild(b *testing.B) {
	sizes := []int{100, 500, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			graph := NewInMemoryGraph()
			nodes := make([]*domain.ServiceNode, size)
			edges := make([]*domain.CallEdge, size-1)
			for i := 0; i < size; i++ {
				nodes[i] = &domain.ServiceNode{
					ID: uuid.New(), Name: fmt.Sprintf("svc-%d", i),
					Namespace: "default", Status: domain.ServiceStatusHealthy,
				}
			}
			for i := 0; i < size-1; i++ {
				edges[i] = &domain.CallEdge{
					ID: uuid.New(), SourceID: nodes[i].ID, TargetID: nodes[i+1].ID,
					EdgeType: domain.EdgeTypeHTTP,
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				graph.Rebuild(nodes, edges)
			}
		})
	}
}

func BenchmarkTopologyService_AnalyzeImpactBatch(b *testing.B) {
	graph, _ := buildBenchmarkGraph(100)
	repo := newMockRepository()
	cache := newMockCache()

	backend := &mockDiscoveryBackend{
		nodes: graph.GetAllNodes(),
		edges: graph.GetAllEdges(),
	}

	service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, &Config{
		RefreshInterval: 1 * time.Second,
		CacheTTL:        5 * time.Minute,
		MaxDepth:        10,
	})
	_ = service.RefreshServiceTopology(b.Context())

	allNodes := graph.GetAllNodes()
	ids := make([]uuid.UUID, len(allNodes))
	for i, n := range allNodes {
		ids[i] = n.ID
	}

	concurrencies := []int{5, 10, 20}
	for _, c := range concurrencies {
		b.Run(fmt.Sprintf("batch_%d_nodes_concurrency_%d", len(ids), c), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = service.AnalyzeImpactBatch(b.Context(), ids[:c], 5)
			}
		})
	}
}

func BenchmarkTopologyService_GetServiceTopology(b *testing.B) {
	graph, _ := buildBenchmarkGraph(100)
	repo := newMockRepository()
	cache := newMockCache()

	backend := &mockDiscoveryBackend{
		nodes: graph.GetAllNodes(),
		edges: graph.GetAllEdges(),
	}

	service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, &Config{
		RefreshInterval: 1 * time.Hour,
		CacheTTL:        5 * time.Minute,
		MaxDepth:        10,
	})
	_ = service.RefreshServiceTopology(b.Context())

	b.ReportAllocs()
	for b.Loop() {
		_, _ = service.GetServiceTopology(b.Context(), domain.TopologyQuery{})
	}
}

func BenchmarkTopologyService_RefreshServiceTopology(b *testing.B) {
	graph, _ := buildBenchmarkGraph(100)
	repo := newMockRepository()
	cache := newMockCache()

	backend := &mockDiscoveryBackend{
		nodes: graph.GetAllNodes(),
		edges: graph.GetAllEdges(),
	}

	service := newTestTopologyService(repo, cache, []domain.DiscoveryBackend{backend}, &Config{
		RefreshInterval: 1 * time.Hour,
		CacheTTL:        5 * time.Minute,
		MaxDepth:        10,
	})

	b.ReportAllocs()
	for b.Loop() {
		_ = service.RefreshServiceTopology(b.Context())
	}
}
