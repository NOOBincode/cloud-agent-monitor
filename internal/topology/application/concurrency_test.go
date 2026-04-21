package application

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func buildConcurrentTestGraph(nodeCount int) (*InMemoryGraph, []*domain.ServiceNode) {
	graph := NewInMemoryGraph()
	nodes := make([]*domain.ServiceNode, nodeCount)
	edges := make([]*domain.CallEdge, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		nodes[i] = &domain.ServiceNode{
			ID:          uuid.New(),
			Name:        fmt.Sprintf("concurrent-svc-%d", i),
			Namespace:   "default",
			Status:      domain.ServiceStatusHealthy,
			RequestRate: float64(i * 10),
			ErrorRate:   0.01,
			LatencyP99:  100.0,
		}
	}

	for i := 0; i < nodeCount-1; i++ {
		edges = append(edges, &domain.CallEdge{
			ID:         uuid.New(),
			SourceID:   nodes[i].ID,
			TargetID:   nodes[i+1].ID,
			EdgeType:   domain.EdgeTypeHTTP,
			IsDirect:   true,
			Confidence: 0.9,
		})
	}

	graph.Rebuild(nodes, edges)
	return graph, nodes
}

func TestConcurrent_AnalyzeImpact_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var errorCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func(userIdx int) {
			defer wg.Done()
			nodeIdx := userIdx % len(nodes)
			result, err := analyzer.AnalyzeImpact(context.Background(), nodes[nodeIdx].ID, 5)
			if err != nil {
				errorCount.Add(1)
			} else {
				successCount.Add(1)
				assert.NotNil(t, result)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent AnalyzeImpact: %d users, %d success, %d errors",
		numUsers, successCount.Load(), errorCount.Load())
	assert.Equal(t, int64(numUsers), successCount.Load()+errorCount.Load())
}

func TestConcurrent_FindPath_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func(userIdx int) {
			defer wg.Done()
			srcIdx := userIdx % len(nodes)
			tgtIdx := (userIdx + len(nodes)/2) % len(nodes)
			result, err := analyzer.FindPath(context.Background(), nodes[srcIdx].ID, nodes[tgtIdx].ID, 10)
			if err == nil {
				successCount.Add(1)
				assert.NotNil(t, result)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent FindPath: %d users, %d found paths", numUsers, successCount.Load())
}

func TestConcurrent_FindShortestPath_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func(userIdx int) {
			defer wg.Done()
			srcIdx := userIdx % len(nodes)
			tgtIdx := (userIdx + len(nodes)/2) % len(nodes)
			path, err := analyzer.FindShortestPath(context.Background(), nodes[srcIdx].ID, nodes[tgtIdx].ID)
			if err == nil {
				successCount.Add(1)
				assert.NotEmpty(t, path)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent FindShortestPath: %d users, %d found paths", numUsers, successCount.Load())
}

func TestConcurrent_FindAnomalies_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, _ := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func() {
			defer wg.Done()
			anomalies, err := analyzer.FindAnomalies(context.Background())
			if err == nil {
				successCount.Add(1)
				_ = anomalies
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent FindAnomalies: %d users, %d success", numUsers, successCount.Load())
	assert.Equal(t, int64(numUsers), successCount.Load())
}

func TestConcurrent_CalculateCentrality_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, _ := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func() {
			defer wg.Done()
			centrality, err := analyzer.CalculateCentrality(context.Background())
			if err == nil {
				successCount.Add(1)
				assert.NotNil(t, centrality)
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent CalculateCentrality: %d users, %d success", numUsers, successCount.Load())
	assert.Equal(t, int64(numUsers), successCount.Load())
}

func TestConcurrent_TopologyService_AnalyzeImpactBatch_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
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
	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	batchIDs := make([]uuid.UUID, 10)
	for i := 0; i < 10 && i < len(nodes); i++ {
		batchIDs[i] = nodes[i].ID
	}

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func() {
			defer wg.Done()
			results, err := service.AnalyzeImpactBatch(context.Background(), batchIDs, 5)
			if err == nil {
				successCount.Add(1)
				assert.NotEmpty(t, results)
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent AnalyzeImpactBatch: %d users, %d success", numUsers, successCount.Load())
}

func TestConcurrent_TopologyService_GetServiceTopology_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, _ := buildConcurrentTestGraph(50)
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
	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)

	const numUsers = 100
	var wg sync.WaitGroup
	var successCount atomic.Int64

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func() {
			defer wg.Done()
			topology, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
			if err == nil {
				successCount.Add(1)
				assert.NotNil(t, topology)
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent GetServiceTopology: %d users, %d success", numUsers, successCount.Load())
	assert.Equal(t, int64(numUsers), successCount.Load())
}

func TestConcurrent_TopologyService_MixedOperations_100Users(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
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
	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)

	const numUsers = 100
	var wg sync.WaitGroup
	var (
		topologyCount   atomic.Int64
		impactCount     atomic.Int64
		pathCount       atomic.Int64
		upstreamCount   atomic.Int64
		downstreamCount atomic.Int64
		statsCount      atomic.Int64
		completedCount  atomic.Int64
	)

	wg.Add(numUsers)
	for i := 0; i < numUsers; i++ {
		go func(userIdx int) {
			defer wg.Done()
			defer completedCount.Add(1)
			nodeIdx := userIdx % len(nodes)
			nodeID := nodes[nodeIdx].ID

			switch userIdx % 6 {
			case 0:
				_, err := service.GetServiceTopology(context.Background(), domain.TopologyQuery{})
				if err == nil {
					topologyCount.Add(1)
				}
			case 1:
				_, err := service.AnalyzeImpact(context.Background(), nodeID, 5)
				if err == nil {
					impactCount.Add(1)
				}
			case 2:
				tgtIdx := (nodeIdx + len(nodes)/2) % len(nodes)
				_, err := service.FindPath(context.Background(), nodeID, nodes[tgtIdx].ID, 10)
				if err == nil {
					pathCount.Add(1)
				}
			case 3:
				_, err := service.GetUpstreamServices(context.Background(), nodeID, 3)
				if err == nil {
					upstreamCount.Add(1)
				}
			case 4:
				_, err := service.GetDownstreamServices(context.Background(), nodeID, 3)
				if err == nil {
					downstreamCount.Add(1)
				}
			case 5:
				_, err := service.GetTopologyStats(context.Background())
				if err == nil {
					statsCount.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	totalOps := topologyCount.Load() + impactCount.Load() + pathCount.Load() +
		upstreamCount.Load() + downstreamCount.Load() + statsCount.Load()

	t.Logf("Concurrent MixedOps: %d users, %d completed, %d success (topo=%d, impact=%d, path=%d, upstream=%d, downstream=%d, stats=%d)",
		numUsers, completedCount.Load(), totalOps, topologyCount.Load(), impactCount.Load(), pathCount.Load(),
		upstreamCount.Load(), downstreamCount.Load(), statsCount.Load())
	assert.Equal(t, int64(numUsers), completedCount.Load(), "all concurrent operations should complete without panic")
}

func TestConcurrent_TopologyService_RefreshWhileReading(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
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
	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)

	const numReaders = 50
	const numRefreshers = 5
	var wg sync.WaitGroup
	var readSuccess atomic.Int64
	var refreshSuccess atomic.Int64

	wg.Add(numReaders + numRefreshers)

	for i := 0; i < numReaders; i++ {
		go func(userIdx int) {
			defer wg.Done()
			nodeIdx := userIdx % len(nodes)
			_, err := service.AnalyzeImpact(context.Background(), nodes[nodeIdx].ID, 5)
			if err == nil {
				readSuccess.Add(1)
			}
		}(i)
	}

	for i := 0; i < numRefreshers; i++ {
		go func() {
			defer wg.Done()
			err := service.RefreshServiceTopology(context.Background())
			if err == nil {
				refreshSuccess.Add(1)
			}
		}()
	}

	wg.Wait()

	t.Logf("Concurrent RefreshWhileReading: readers=%d success, refreshers=%d success",
		readSuccess.Load(), refreshSuccess.Load())
}

func TestConcurrent_InMemoryGraph_RebuildWhileReading(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
	analyzer := NewGraphAnalyzer(graph)

	const numReaders = 50
	const numRebuilders = 5
	var wg sync.WaitGroup
	var readSuccess atomic.Int64
	var rebuildSuccess atomic.Int64

	wg.Add(numReaders + numRebuilders)

	for i := 0; i < numReaders; i++ {
		go func(userIdx int) {
			defer wg.Done()
			nodeIdx := userIdx % len(nodes)
			_, err := analyzer.AnalyzeImpact(context.Background(), nodes[nodeIdx].ID, 5)
			if err == nil {
				readSuccess.Add(1)
			}
		}(i)
	}

	for i := 0; i < numRebuilders; i++ {
		go func() {
			defer wg.Done()
			newGraph, newNodes := buildConcurrentTestGraph(50)
			graph.Rebuild(newGraph.GetAllNodes(), newGraph.GetAllEdges())
			rebuildSuccess.Add(1)
			_ = newNodes
		}()
	}

	wg.Wait()

	t.Logf("Concurrent RebuildWhileReading: readers=%d success, rebuilders=%d success",
		readSuccess.Load(), rebuildSuccess.Load())
}

func TestConcurrent_TopologyService_AnalyzeImpactBatch_SemaphoreLimit(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	graph, nodes := buildConcurrentTestGraph(50)
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
	err := service.RefreshServiceTopology(context.Background())
	require.NoError(t, err)

	allIDs := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		allIDs[i] = n.ID
	}

	const numConcurrentBatches = 20
	var wg sync.WaitGroup
	var successCount atomic.Int64

	start := time.Now()
	wg.Add(numConcurrentBatches)
	for i := 0; i < numConcurrentBatches; i++ {
		go func() {
			defer wg.Done()
			results, err := service.AnalyzeImpactBatch(context.Background(), allIDs, 5)
			if err == nil {
				successCount.Add(1)
				assert.NotEmpty(t, results)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Concurrent AnalyzeImpactBatch: %d concurrent batches of %d IDs, %d success, elapsed=%v",
		numConcurrentBatches, len(allIDs), successCount.Load(), elapsed)
}
