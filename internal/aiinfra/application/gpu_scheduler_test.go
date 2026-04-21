package application

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud-agent-monitor/internal/aiinfra/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockGPUNodeRepository struct {
	mu    sync.RWMutex
	nodes map[string]*domain.GPUNode
}

func newMockGPUNodeRepository() *mockGPUNodeRepository {
	return &mockGPUNodeRepository{nodes: make(map[string]*domain.GPUNode)}
}

func (m *mockGPUNodeRepository) Create(ctx context.Context, node *domain.GPUNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
	return nil
}

func (m *mockGPUNodeRepository) GetByID(ctx context.Context, id string) (*domain.GPUNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n, ok := m.nodes[id]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("gpu node not found: %s", id)
}

func (m *mockGPUNodeRepository) GetByUUID(ctx context.Context, uuid string) (*domain.GPUNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, n := range m.nodes {
		if n.GPUUUID == uuid {
			return n, nil
		}
	}
	return nil, fmt.Errorf("gpu node not found by uuid: %s", uuid)
}

func (m *mockGPUNodeRepository) List(ctx context.Context, status *domain.GPUNodeStatus) ([]*domain.GPUNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.GPUNode, 0)
	for _, n := range m.nodes {
		if status == nil || n.Status == *status {
			result = append(result, n)
		}
	}
	return result, nil
}

func (m *mockGPUNodeRepository) Update(ctx context.Context, node *domain.GPUNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[node.ID] = node
	return nil
}

type mockGPUMetricRepository struct {
	mu      sync.RWMutex
	metrics map[string][]*domain.GPUMetric
	latest  map[string]*domain.GPUMetric
}

func newMockGPUMetricRepository() *mockGPUMetricRepository {
	return &mockGPUMetricRepository{
		metrics: make(map[string][]*domain.GPUMetric),
		latest:  make(map[string]*domain.GPUMetric),
	}
}

func (m *mockGPUMetricRepository) Create(ctx context.Context, metric *domain.GPUMetric) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics[metric.GPUNodeID] = append(m.metrics[metric.GPUNodeID], metric)
	m.latest[metric.GPUNodeID] = metric
	return nil
}

func (m *mockGPUMetricRepository) ListByNode(ctx context.Context, nodeID string, start, end time.Time, limit int) ([]*domain.GPUMetric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics[nodeID], nil
}

func (m *mockGPUMetricRepository) GetLatest(ctx context.Context, nodeID string) (*domain.GPUMetric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m, ok := m.latest[nodeID]; ok {
		return m, nil
	}
	return nil, fmt.Errorf("no metrics for node: %s", nodeID)
}

type mockGPUAlertRepository struct {
	mu     sync.RWMutex
	alerts map[string]*domain.GPUAlert
}

func newMockGPUAlertRepository() *mockGPUAlertRepository {
	return &mockGPUAlertRepository{alerts: make(map[string]*domain.GPUAlert)}
}

func (m *mockGPUAlertRepository) Create(ctx context.Context, alert *domain.GPUAlert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts[alert.ID] = alert
	return nil
}

func (m *mockGPUAlertRepository) GetByID(ctx context.Context, id string) (*domain.GPUAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a, ok := m.alerts[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("alert not found: %s", id)
}

func (m *mockGPUAlertRepository) ListActive(ctx context.Context, nodeID *string) ([]*domain.GPUAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.GPUAlert, 0)
	for _, a := range m.alerts {
		if a.Status != "resolved" {
			if nodeID == nil || a.GPUNodeID == *nodeID {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

func (m *mockGPUAlertRepository) Resolve(ctx context.Context, id string, resolvedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.alerts[id]; ok {
		a.Status = "resolved"
		a.ResolvedAt = resolvedAt
	}
	return nil
}

type mockResourceAllocationRepository struct {
	mu          sync.RWMutex
	allocations map[string]*domain.ResourceAllocation
	byNode      map[string][]*domain.ResourceAllocation
}

func newMockResourceAllocationRepository() *mockResourceAllocationRepository {
	return &mockResourceAllocationRepository{
		allocations: make(map[string]*domain.ResourceAllocation),
		byNode:      make(map[string][]*domain.ResourceAllocation),
	}
}

func (m *mockResourceAllocationRepository) Create(ctx context.Context, alloc *domain.ResourceAllocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocations[alloc.ID] = alloc
	m.byNode[alloc.NodeName] = append(m.byNode[alloc.NodeName], alloc)
	return nil
}

func (m *mockResourceAllocationRepository) ListByJob(ctx context.Context, jobID string) ([]*domain.ResourceAllocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.ResourceAllocation, 0)
	for _, a := range m.allocations {
		if a.JobID == jobID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockResourceAllocationRepository) ListByNode(ctx context.Context, nodeName string, activeOnly bool) ([]*domain.ResourceAllocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	allocs := m.byNode[nodeName]
	if !activeOnly {
		return allocs, nil
	}
	result := make([]*domain.ResourceAllocation, 0)
	for _, a := range allocs {
		if a.ReleasedAt.IsZero() {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockResourceAllocationRepository) Update(ctx context.Context, alloc *domain.ResourceAllocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocations[alloc.ID] = alloc
	return nil
}

type mockQueueJobRepository struct {
	mu   sync.RWMutex
	jobs map[string]*domain.QueueJob
}

func newMockQueueJobRepository() *mockQueueJobRepository {
	return &mockQueueJobRepository{jobs: make(map[string]*domain.QueueJob)}
}

func (m *mockQueueJobRepository) Create(ctx context.Context, job *domain.QueueJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockQueueJobRepository) GetByID(ctx context.Context, id string) (*domain.QueueJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[id]; ok {
		return j, nil
	}
	return nil, fmt.Errorf("job not found: %s", id)
}

func (m *mockQueueJobRepository) GetByK8sUID(ctx context.Context, k8sUID string) (*domain.QueueJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, j := range m.jobs {
		if j.K8sUID == k8sUID {
			return j, nil
		}
	}
	return nil, fmt.Errorf("job not found by k8s uid: %s", k8sUID)
}

func (m *mockQueueJobRepository) List(ctx context.Context, filter *domain.QueueJobFilter) ([]*domain.QueueJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.QueueJob, 0)
	for _, j := range m.jobs {
		result = append(result, j)
	}
	return result, nil
}

func (m *mockQueueJobRepository) Update(ctx context.Context, job *domain.QueueJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func createTestGPUNodes(count int) []*domain.GPUNode {
	nodes := make([]*domain.GPUNode, count)
	models := []string{"A100", "V100", "H100"}
	for i := 0; i < count; i++ {
		nodes[i] = &domain.GPUNode{
			ID:               fmt.Sprintf("gpu-node-%d", i),
			NodeName:         fmt.Sprintf("worker-%d", i),
			GPUIndex:         i % 4,
			GPUUUID:          fmt.Sprintf("GPU-%d-UUID", i),
			GPUModel:         models[i%len(models)],
			GPUMemoryTotalMB: 81920,
			MIGEnabled:       i%3 == 0,
			MIGProfile:       "1g.10gb",
			K8sNodeName:      fmt.Sprintf("k8s-worker-%d", i),
			Namespace:        "ai-infra",
			Status:           domain.GPUNodeStatusActive,
			HealthStatus:     domain.GPUHealthHealthy,
			LastHealthCheck:  time.Now(),
			Labels: map[string]string{
				"nvidia.com/gpu.product":      models[i%len(models)],
				"nvidia.com/gpu.count":        "4",
				"topology.kubernetes.io/zone": fmt.Sprintf("zone-%c", 'a'+rune(i%3)),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
	return nodes
}

func createTestGPUMetrics(nodeID string, utilization float64, memUsedMB int) *domain.GPUMetric {
	return &domain.GPUMetric{
		ID:                 fmt.Sprintf("metric-%s-%d", nodeID, time.Now().UnixNano()),
		GPUNodeID:          nodeID,
		GPUUtilization:     utilization,
		MemoryUtilization:  float64(memUsedMB) / 81920.0 * 100,
		MemoryUsedMB:       memUsedMB,
		MemoryFreeMB:       81920 - memUsedMB,
		PowerUsageW:        250.0 + utilization*1.5,
		PowerDrawW:         300.0,
		TemperatureC:       35 + int(utilization*0.4),
		SMClockMHz:         1410,
		MemoryClockMHz:     1215,
		PCIeRxThroughputMB: 1024.0,
		PCIeTxThroughputMB: 2048.0,
		CollectedAt:        time.Now(),
		CreatedAt:          time.Now(),
	}
}

func TestGPUNode_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := newMockGPUNodeRepository()

	t.Run("create and retrieve GPU node", func(t *testing.T) {
		node := createTestGPUNodes(1)[0]
		err := repo.Create(ctx, node)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, node.GPUModel, retrieved.GPUModel)
		assert.Equal(t, node.Status, retrieved.Status)
		assert.Equal(t, node.HealthStatus, retrieved.HealthStatus)
	})

	t.Run("list GPU nodes by status", func(t *testing.T) {
		nodes := createTestGPUNodes(5)
		nodes[2].Status = domain.GPUNodeStatusMaintenance
		nodes[4].Status = domain.GPUNodeStatusFailed
		for _, n := range nodes {
			_ = repo.Create(ctx, n)
		}

		activeStatus := domain.GPUNodeStatusActive
		active, err := repo.List(ctx, &activeStatus)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(active), 3)

		all, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 5)
	})

	t.Run("update GPU node status", func(t *testing.T) {
		node := createTestGPUNodes(1)[0]
		node.ID = "gpu-update-test"
		_ = repo.Create(ctx, node)

		node.Status = domain.GPUNodeStatusMaintenance
		node.HealthStatus = domain.GPUHealthDegraded
		err := repo.Update(ctx, node)
		require.NoError(t, err)

		updated, err := repo.GetByID(ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.GPUNodeStatusMaintenance, updated.Status)
		assert.Equal(t, domain.GPUHealthDegraded, updated.HealthStatus)
	})
}

func TestGPUMetric_CollectionAndQuery(t *testing.T) {
	ctx := context.Background()
	repo := newMockGPUMetricRepository()
	nodeID := "gpu-metric-test-node"

	t.Run("create and get latest metric", func(t *testing.T) {
		m1 := createTestGPUMetrics(nodeID, 45.0, 40960)
		err := repo.Create(ctx, m1)
		require.NoError(t, err)

		latest, err := repo.GetLatest(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, 45.0, latest.GPUUtilization)
		assert.Equal(t, 40960, latest.MemoryUsedMB)
	})

	t.Run("latest metric updates on new write", func(t *testing.T) {
		m2 := createTestGPUMetrics(nodeID, 85.0, 65536)
		err := repo.Create(ctx, m2)
		require.NoError(t, err)

		latest, err := repo.GetLatest(ctx, nodeID)
		require.NoError(t, err)
		assert.Equal(t, 85.0, latest.GPUUtilization)
		assert.Equal(t, 65536, latest.MemoryUsedMB)
	})

	t.Run("list metrics by node with time range", func(t *testing.T) {
		metrics, err := repo.ListByNode(ctx, nodeID, time.Now().Add(-1*time.Hour), time.Now(), 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(metrics), 2)
	})
}

func TestGPUAlert_Lifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newMockGPUAlertRepository()

	t.Run("create and resolve alert", func(t *testing.T) {
		alert := &domain.GPUAlert{
			ID:          "alert-1",
			GPUNodeID:   "gpu-node-0",
			AlertType:   domain.AlertTypeTemperature,
			Severity:    domain.AlertSeverityCritical,
			AlertName:   "GPUOverTemperature",
			Message:     "GPU temperature exceeded 90C threshold",
			MetricValue: 92.0,
			Threshold:   90.0,
			Status:      "firing",
			FiredAt:     time.Now(),
			CreatedAt:   time.Now(),
		}

		err := repo.Create(ctx, alert)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, alert.ID)
		require.NoError(t, err)
		assert.Equal(t, "firing", retrieved.Status)

		err = repo.Resolve(ctx, alert.ID, time.Now())
		require.NoError(t, err)

		resolved, err := repo.GetByID(ctx, alert.ID)
		require.NoError(t, err)
		assert.Equal(t, "resolved", resolved.Status)
	})

	t.Run("list active alerts by node", func(t *testing.T) {
		nodeID := "gpu-node-alert-test"
		alerts := []*domain.GPUAlert{
			{ID: "active-1", GPUNodeID: nodeID, AlertType: domain.AlertTypeXIDError, Severity: domain.AlertSeverityWarning, Status: "firing"},
			{ID: "active-2", GPUNodeID: nodeID, AlertType: domain.AlertTypeMemory, Severity: domain.AlertSeverityCritical, Status: "firing"},
			{ID: "resolved-1", GPUNodeID: nodeID, AlertType: domain.AlertTypePower, Severity: domain.AlertSeverityInfo, Status: "resolved"},
		}
		for _, a := range alerts {
			_ = repo.Create(ctx, a)
		}

		active, err := repo.ListActive(ctx, &nodeID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(active), 2)
	})
}

func TestGPUResource_AllocationAndRelease(t *testing.T) {
	ctx := context.Background()
	allocRepo := newMockResourceAllocationRepository()
	jobRepo := newMockQueueJobRepository()

	t.Run("allocate GPU resources for job", func(t *testing.T) {
		job := &domain.QueueJob{
			ID:           "job-1",
			JobName:      "training-llama-70b",
			JobType:      domain.JobTypeTraining,
			QueueName:    "gpu-queue",
			K8sNamespace: "ai-infra",
			GPUCount:     4,
			CPUCores:     16.0,
			MemoryMB:     65536,
			Priority:     10,
			Status:       domain.JobStatusPending,
			SubmittedAt:  time.Now(),
			CreatedAt:    time.Now(),
		}
		err := jobRepo.Create(ctx, job)
		require.NoError(t, err)

		for i := 0; i < job.GPUCount; i++ {
			alloc := &domain.ResourceAllocation{
				ID:           fmt.Sprintf("alloc-%s-gpu-%d", job.ID, i),
				JobID:        job.ID,
				ResourceType: domain.ResourceTypeGPU,
				ResourceName: fmt.Sprintf("nvidia.com/gpu-%d", i),
				Requested:    1.0,
				Allocated:    1.0,
				Used:         0.0,
				Unit:         "gpu",
				NodeName:     fmt.Sprintf("k8s-worker-%d", i%3),
				GPUIndex:     i,
				AllocatedAt:  time.Now(),
				CreatedAt:    time.Now(),
			}
			err = allocRepo.Create(ctx, alloc)
			require.NoError(t, err)
		}

		allocations, err := allocRepo.ListByJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Len(t, allocations, 4)
	})

	t.Run("release GPU resources after job completion", func(t *testing.T) {
		job := &domain.QueueJob{
			ID:       "job-2",
			JobName:  "inference-mistral-7b",
			JobType:  domain.JobTypeInference,
			GPUCount: 1,
			Status:   domain.JobStatusRunning,
		}
		_ = jobRepo.Create(ctx, job)

		alloc := &domain.ResourceAllocation{
			ID:           "alloc-job-2-gpu-0",
			JobID:        job.ID,
			ResourceType: domain.ResourceTypeGPU,
			ResourceName: "nvidia.com/gpu-0",
			Requested:    1.0,
			Allocated:    1.0,
			Used:         0.85,
			Unit:         "gpu",
			NodeName:     "k8s-worker-0",
			GPUIndex:     0,
			AllocatedAt:  time.Now().Add(-1 * time.Hour),
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		}
		_ = allocRepo.Create(ctx, alloc)

		activeOnNode, err := allocRepo.ListByNode(ctx, "k8s-worker-0", true)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(activeOnNode), 1)

		alloc.Used = 0.0
		alloc.ReleasedAt = time.Now()
		err = allocRepo.Update(ctx, alloc)
		require.NoError(t, err)

		activeOnNode, err = allocRepo.ListByNode(ctx, "k8s-worker-0", true)
		require.NoError(t, err)
		for _, a := range activeOnNode {
			assert.NotEqual(t, "alloc-job-2-gpu-0", a.ID)
		}
	})
}

func TestGPUResource_ConcurrentAllocation(t *testing.T) {
	ctx := context.Background()
	allocRepo := newMockResourceAllocationRepository()
	jobRepo := newMockQueueJobRepository()

	const numJobs = 20
	const totalGPUs = 8

	var wg sync.WaitGroup
	var successCount atomic.Int64

	gpuAllocations := make([]atomic.Int64, totalGPUs)

	wg.Add(numJobs)
	for i := 0; i < numJobs; i++ {
		go func(jobIdx int) {
			defer wg.Done()

			job := &domain.QueueJob{
				ID:          fmt.Sprintf("concurrent-job-%d", jobIdx),
				JobName:     fmt.Sprintf("training-job-%d", jobIdx),
				JobType:     domain.JobTypeTraining,
				GPUCount:    1,
				Priority:    jobIdx % 10,
				Status:      domain.JobStatusPending,
				SubmittedAt: time.Now(),
				CreatedAt:   time.Now(),
			}
			_ = jobRepo.Create(ctx, job)

			allocated := false
			for gpuIdx := 0; gpuIdx < totalGPUs; gpuIdx++ {
				current := gpuAllocations[gpuIdx].Load()
				if current < 1 && gpuAllocations[gpuIdx].CompareAndSwap(current, current+1) {
					alloc := &domain.ResourceAllocation{
						ID:           fmt.Sprintf("alloc-%s", job.ID),
						JobID:        job.ID,
						ResourceType: domain.ResourceTypeGPU,
						ResourceName: fmt.Sprintf("nvidia.com/gpu-%d", gpuIdx),
						Requested:    1.0,
						Allocated:    1.0,
						Used:         0.0,
						Unit:         "gpu",
						NodeName:     fmt.Sprintf("k8s-worker-%d", gpuIdx),
						GPUIndex:     gpuIdx,
						AllocatedAt:  time.Now(),
						CreatedAt:    time.Now(),
					}
					_ = allocRepo.Create(ctx, alloc)
					successCount.Add(1)
					allocated = true
					break
				}
			}

			if !allocated {
				job.Status = domain.JobStatusPending
				_ = jobRepo.Update(ctx, job)
			} else {
				job.Status = domain.JobStatusRunning
				_ = jobRepo.Update(ctx, job)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent GPU allocation: %d jobs competing for %d GPUs, %d allocated, %d queued",
		numJobs, totalGPUs, successCount.Load(), int64(numJobs)-successCount.Load())
	assert.LessOrEqual(t, successCount.Load(), int64(totalGPUs), "should not allocate more GPUs than available")
	assert.Greater(t, successCount.Load(), int64(0), "at least some jobs should be allocated")
}

func TestGPULoad_PerformanceUnderLoad(t *testing.T) {
	ctx := context.Background()
	metricRepo := newMockGPUMetricRepository()
	nodeRepo := newMockGPUNodeRepository()

	nodes := createTestGPUNodes(8)
	for _, n := range nodes {
		_ = nodeRepo.Create(ctx, n)
	}

	loadProfiles := []struct {
		name        string
		utilization float64
		memUsedMB   int
	}{
		{"idle", 5.0, 4096},
		{"light", 25.0, 20480},
		{"medium", 55.0, 45056},
		{"heavy", 85.0, 69632},
		{"saturated", 98.0, 77824},
	}

	for _, profile := range loadProfiles {
		t.Run(fmt.Sprintf("load_%s", profile.name), func(t *testing.T) {
			start := time.Now()
			for _, node := range nodes {
				metric := createTestGPUMetrics(node.ID, profile.utilization, profile.memUsedMB)
				err := metricRepo.Create(ctx, metric)
				require.NoError(t, err)

				latest, err := metricRepo.GetLatest(ctx, node.ID)
				require.NoError(t, err)
				assert.Equal(t, profile.utilization, latest.GPUUtilization)
			}
			elapsed := time.Since(start)
			t.Logf("Load profile %s: 8 nodes, elapsed=%v, util=%.0f%%, mem=%dMB",
				profile.name, elapsed, profile.utilization, profile.memUsedMB)
		})
	}
}

func TestGPUMultiNode_Scheduling(t *testing.T) {
	ctx := context.Background()
	allocRepo := newMockResourceAllocationRepository()
	jobRepo := newMockQueueJobRepository()

	t.Run("multi-GPU job allocation across nodes", func(t *testing.T) {
		job := &domain.QueueJob{
			ID:          "multi-gpu-job-1",
			JobName:     "distributed-training-llama",
			JobType:     domain.JobTypeTraining,
			GPUCount:    8,
			CPUCores:    64.0,
			MemoryMB:    262144,
			Priority:    10,
			Status:      domain.JobStatusPending,
			SubmittedAt: time.Now(),
			CreatedAt:   time.Now(),
		}
		err := jobRepo.Create(ctx, job)
		require.NoError(t, err)

		nodeNames := []string{"k8s-worker-0", "k8s-worker-1", "k8s-worker-2"}
		for i := 0; i < job.GPUCount; i++ {
			alloc := &domain.ResourceAllocation{
				ID:           fmt.Sprintf("alloc-multi-%d", i),
				JobID:        job.ID,
				ResourceType: domain.ResourceTypeGPU,
				ResourceName: fmt.Sprintf("nvidia.com/gpu-%d", i),
				Requested:    1.0,
				Allocated:    1.0,
				Used:         0.0,
				Unit:         "gpu",
				NodeName:     nodeNames[i%len(nodeNames)],
				GPUIndex:     i % 4,
				AllocatedAt:  time.Now(),
				CreatedAt:    time.Now(),
			}
			err = allocRepo.Create(ctx, alloc)
			require.NoError(t, err)
		}

		allocations, err := allocRepo.ListByJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Len(t, allocations, 8)

		nodeUsage := make(map[string]int)
		for _, a := range allocations {
			nodeUsage[a.NodeName]++
		}
		for _, count := range nodeUsage {
			assert.LessOrEqual(t, count, 4, "no node should have more than 4 GPU allocations")
		}
		t.Logf("Multi-GPU scheduling: %d GPUs across %d nodes: %v",
			len(allocations), len(nodeUsage), nodeUsage)
	})
}

func TestGPUHealth_DegradationDetection(t *testing.T) {
	ctx := context.Background()
	nodeRepo := newMockGPUNodeRepository()
	alertRepo := newMockGPUAlertRepository()

	nodes := createTestGPUNodes(4)
	for _, n := range nodes {
		_ = nodeRepo.Create(ctx, n)
	}

	t.Run("detect XID error and create alert", func(t *testing.T) {
		node := nodes[0]
		alert := &domain.GPUAlert{
			ID:          "xid-alert-1",
			GPUNodeID:   node.ID,
			AlertType:   domain.AlertTypeXIDError,
			Severity:    domain.AlertSeverityCritical,
			AlertName:   "GPUXIDError",
			Message:     "XID error 79 detected on GPU",
			XIDCode:     79,
			MetricValue: 1.0,
			Threshold:   0.0,
			Status:      "firing",
			FiredAt:     time.Now(),
			CreatedAt:   time.Now(),
		}
		err := alertRepo.Create(ctx, alert)
		require.NoError(t, err)

		node.HealthStatus = domain.GPUHealthCritical
		err = nodeRepo.Update(ctx, node)
		require.NoError(t, err)

		updated, err := nodeRepo.GetByID(ctx, node.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.GPUHealthCritical, updated.HealthStatus)

		active, err := alertRepo.ListActive(ctx, &node.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(active), 1)
	})

	t.Run("detect memory ECC error", func(t *testing.T) {
		node := nodes[1]
		alert := &domain.GPUAlert{
			ID:          "ecc-alert-1",
			GPUNodeID:   node.ID,
			AlertType:   domain.AlertTypeECC,
			Severity:    domain.AlertSeverityWarning,
			AlertName:   "GPUECCError",
			Message:     "ECC memory error detected",
			MetricValue: 5.0,
			Threshold:   1.0,
			Status:      "firing",
			FiredAt:     time.Now(),
			CreatedAt:   time.Now(),
		}
		err := alertRepo.Create(ctx, alert)
		require.NoError(t, err)

		node.HealthStatus = domain.GPUHealthDegraded
		err = nodeRepo.Update(ctx, node)
		require.NoError(t, err)
	})
}

func TestGPUResource_MIGProfileAllocation(t *testing.T) {
	ctx := context.Background()
	allocRepo := newMockResourceAllocationRepository()

	t.Run("allocate MIG profiles on A100", func(t *testing.T) {
		migProfiles := []struct {
			profile    string
			memoryMB   int
			sliceCount int
		}{
			{"1g.10gb", 10240, 7},
			{"2g.20gb", 20480, 3},
			{"3g.40gb", 40960, 2},
			{"4g.40gb", 40960, 1},
		}

		for _, mp := range migProfiles {
			for i := 0; i < mp.sliceCount; i++ {
				alloc := &domain.ResourceAllocation{
					ID:           fmt.Sprintf("mig-alloc-%s-%d", mp.profile, i),
					JobID:        fmt.Sprintf("mig-job-%s-%d", mp.profile, i),
					ResourceType: domain.ResourceTypeGPU,
					ResourceName: fmt.Sprintf("nvidia.com/mig-%s", mp.profile),
					Requested:    1.0,
					Allocated:    1.0,
					Used:         0.0,
					Unit:         "mig",
					NodeName:     "k8s-worker-a100",
					GPUIndex:     0,
					AllocatedAt:  time.Now(),
					CreatedAt:    time.Now(),
				}
				err := allocRepo.Create(ctx, alloc)
				require.NoError(t, err)
			}
		}

		activeOnNode, err := allocRepo.ListByNode(ctx, "k8s-worker-a100", true)
		require.NoError(t, err)
		totalMIGAllocs := len(activeOnNode)
		t.Logf("MIG profile allocations on A100: %d total slices", totalMIGAllocs)
		assert.Greater(t, totalMIGAllocs, 0)
	})
}
