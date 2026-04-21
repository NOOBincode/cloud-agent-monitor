package application

import (
	"context"
	"log"
	"sync"
	"time"

	"cloud-agent-monitor/internal/topology/domain"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
)

// ImpactCacheService 管理影响分析结果的预计算和缓存。
//
// 该服务定期预计算所有服务的影响分析结果，缓存到 TopologyCache 中，
// 提升后续查询的响应速度。预计算采用批量并发处理，通过 batchSize
// 控制并发数量，避免资源过度消耗。
//
// 使用示例:
//
//	cacheService := NewImpactCacheService(topologyService, cache, analyzer, &ImpactCacheConfig{
//	    PrecomputeInterval: 5 * time.Minute,
//	    MaxDepth:           5,
//	    BatchSize:          50,
//	})
//	if err := cacheService.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer cacheService.Stop()
type ImpactCacheService struct {
	service  *TopologyService
	cache    domain.TopologyCache
	analyzer *GraphAnalyzer
	queue    infra.QueueInterface

	precomputeInterval time.Duration
	maxDepth           int
	batchSize          int

	running bool
	mu      sync.RWMutex

	precomputedCount int64
	lastPrecompute   time.Time
}

// ImpactCacheConfig 定义 ImpactCacheService 的配置参数。
type ImpactCacheConfig struct {
	// PrecomputeInterval 预计算执行间隔
	PrecomputeInterval time.Duration
	// MaxDepth 影响分析的最大遍历深度
	MaxDepth int
	// BatchSize 批量处理的并发数量
	BatchSize int
}

// NewImpactCacheService 创建一个新的影响缓存服务实例。
//
// 参数:
//   - service: 拓扑服务，用于获取服务列表
//   - cache: 拓扑缓存，用于存储预计算结果
//   - analyzer: 图分析器，用于执行影响分析
//   - cfg: 服务配置，为 nil 时使用默认值
//
// 默认配置:
//   - PrecomputeInterval: 5 分钟
//   - MaxDepth: 5
//   - BatchSize: 50
func NewImpactCacheService(
	service *TopologyService,
	cache domain.TopologyCache,
	analyzer *GraphAnalyzer,
	queue infra.QueueInterface,
	cfg *ImpactCacheConfig,
) *ImpactCacheService {
	if cfg == nil {
		cfg = &ImpactCacheConfig{
			PrecomputeInterval: 5 * time.Minute,
			MaxDepth:           5,
			BatchSize:          50,
		}
	}

	return &ImpactCacheService{
		service:            service,
		cache:              cache,
		analyzer:           analyzer,
		queue:              queue,
		precomputeInterval: cfg.PrecomputeInterval,
		maxDepth:           cfg.MaxDepth,
		batchSize:          cfg.BatchSize,
	}
}

// Start 启动影响缓存服务，开始定期预计算。
//
// 该方法会先执行一次全量预计算，然后启动后台 goroutine
// 定期执行预计算任务。如果服务已在运行，则直接返回 nil。
//
// 参数:
//   - ctx: 上下文，用于初始化时的预计算
//
// 返回: 启动失败时返回错误
func (s *ImpactCacheService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	if err := s.PrecomputeAll(ctx); err != nil {
		log.Printf("[ImpactCacheService] Initial precompute failed: %v", err)
	}

	s.registerQueueHandlers()

	if s.queue != nil {
		if _, err := s.queue.Enqueue(ctx, "topology:precompute-impact", nil); err != nil {
			log.Printf("[ImpactCacheService] Failed to enqueue initial precompute-impact: %v", err)
		}
	}

	return nil
}

func (s *ImpactCacheService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

func (s *ImpactCacheService) registerQueueHandlers() {
	if s.queue == nil {
		return
	}

	s.queue.RegisterHandler("topology:precompute-impact", func(ctx context.Context, payload []byte) error {
		if err := s.PrecomputeAll(ctx); err != nil {
			log.Printf("[ImpactCacheService] Async precompute failed: %v", err)
			return err
		}
		if _, err := s.queue.EnqueueWithDelay(ctx, "topology:precompute-impact", nil, s.precomputeInterval); err != nil {
			log.Printf("[ImpactCacheService] Failed to schedule next precompute-impact: %v", err)
		}
		return nil
	})
}

// PrecomputeAll 对所有服务执行影响分析预计算。
//
// 该方法获取当前内存图中的所有服务节点，批量执行影响分析
// 并缓存结果。如果图为空，则直接返回 nil。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回: 预计算失败时返回错误
func (s *ImpactCacheService) PrecomputeAll(ctx context.Context) error {
	s.mu.RLock()
	graph := s.service.serviceGraph
	s.mu.RUnlock()

	if graph == nil || graph.IsEmpty() {
		return nil
	}

	nodes := graph.GetAllNodes()
	nodeIDs := make([]uuid.UUID, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}

	return s.PrecomputeBatch(ctx, nodeIDs)
}

// PrecomputeBatch 批量执行影响分析预计算。
//
// 该方法并发处理指定的服务列表，通过 semaphore 限制并发数。
// 每个服务的影响分析结果会被缓存到 TopologyCache 中。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceIDs: 待预计算的服务 ID 列表
//
// 返回: 上下文取消时返回错误
//
// 并发控制:
//   - 使用 semaphore 模式限制并发数（默认 batchSize）
//   - 每个 goroutine 独立执行分析，失败不影响其他服务
func (s *ImpactCacheService) PrecomputeBatch(ctx context.Context, serviceIDs []uuid.UUID) error {
	s.mu.RLock()
	analyzer := s.analyzer
	cache := s.cache
	s.mu.RUnlock()

	if analyzer == nil || cache == nil {
		return nil
	}

	var processed int64
	var failed int64

	maxConcurrency := s.batchSize
	if maxConcurrency <= 0 {
		maxConcurrency = 50
	}

	for i := 0; i < len(serviceIDs); i += s.batchSize {
		end := i + s.batchSize
		if end > len(serviceIDs) {
			end = len(serviceIDs)
		}

		batch := serviceIDs[i:end]

		var wg sync.WaitGroup
		var batchMu sync.Mutex
		sem := make(chan struct{}, maxConcurrency)

		for _, serviceID := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			sem <- struct{}{}
			wg.Add(1)
			go func(id uuid.UUID) {
				defer wg.Done()
				defer func() { <-sem }()

				result, err := analyzer.AnalyzeImpact(ctx, id, s.maxDepth)
				if err != nil {
					batchMu.Lock()
					failed++
					batchMu.Unlock()
					return
				}

				if err := cache.SetImpactCache(ctx, id, result, s.precomputeInterval*2); err != nil {
					log.Printf("[ImpactCacheService] Failed to cache impact for %s: %v", id, err)
					return
				}

				batchMu.Lock()
				processed++
				batchMu.Unlock()
			}(serviceID)
		}

		wg.Wait()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	s.mu.Lock()
	s.precomputedCount += processed
	s.lastPrecompute = time.Now()
	s.mu.Unlock()

	log.Printf("[ImpactCacheService] Precomputed %d services, failed %d", processed, failed)

	return nil
}

// PrecomputeService 对单个服务执行影响分析预计算。
//
// 该方法立即对指定服务执行影响分析并缓存结果，
// 适用于服务状态变更时的即时更新场景。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 待预计算的服务 ID
//
// 返回: 分析或缓存失败时返回错误
func (s *ImpactCacheService) PrecomputeService(ctx context.Context, serviceID uuid.UUID) error {
	s.mu.RLock()
	analyzer := s.analyzer
	cache := s.cache
	s.mu.RUnlock()

	if analyzer == nil || cache == nil {
		return nil
	}

	result, err := analyzer.AnalyzeImpact(ctx, serviceID, s.maxDepth)
	if err != nil {
		return err
	}

	return cache.SetImpactCache(ctx, serviceID, result, s.precomputeInterval*2)
}

// GetImpact 从缓存获取服务的影响分析结果。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 服务 ID
//
// 返回:
//   - *ImpactResult: 缓存的影响分析结果
//   - error: 缓存未命中或缓存不可用时返回错误
func (s *ImpactCacheService) GetImpact(ctx context.Context, serviceID uuid.UUID) (*domain.ImpactResult, error) {
	s.mu.RLock()
	cache := s.cache
	s.mu.RUnlock()

	if cache == nil {
		return nil, domain.ErrCacheMiss
	}

	result, err := cache.GetImpactCache(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetImpactBatch 批量从缓存获取服务的影响分析结果。
//
// 该方法并发获取多个服务的影响分析结果，失败的服务会被跳过。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceIDs: 服务 ID 列表
//
// 返回:
//   - map[uuid.UUID]*ImpactResult: 服务 ID 到结果的映射
//   - error: 当前实现总是返回 nil
func (s *ImpactCacheService) GetImpactBatch(ctx context.Context, serviceIDs []uuid.UUID) (map[uuid.UUID]*domain.ImpactResult, error) {
	results := make(map[uuid.UUID]*domain.ImpactResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, id := range serviceIDs {
		wg.Add(1)
		go func(serviceID uuid.UUID) {
			defer wg.Done()
			result, err := s.GetImpact(ctx, serviceID)
			if err == nil && result != nil {
				mu.Lock()
				results[serviceID] = result
				mu.Unlock()
			}
		}(id)
	}

	wg.Wait()
	return results, nil
}

// InvalidateService 使指定服务的影响分析缓存失效。
//
// 该方法删除指定服务的缓存结果，下次查询时会触发重新计算。
// 适用于服务状态变更后需要立即更新的场景。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - serviceID: 服务 ID
//
// 返回: 删除失败时返回错误
func (s *ImpactCacheService) InvalidateService(ctx context.Context, serviceID uuid.UUID) error {
	s.mu.RLock()
	cache := s.cache
	s.mu.RUnlock()

	if cache == nil {
		return nil
	}

	return cache.DeleteImpactCache(ctx, serviceID)
}

func (s *ImpactCacheService) InvalidateBatch(ctx context.Context, serviceIDs []uuid.UUID) error {
	s.mu.RLock()
	cache := s.cache
	s.mu.RUnlock()

	if cache == nil {
		return nil
	}

	var wg sync.WaitGroup
	for _, id := range serviceIDs {
		wg.Add(1)
		go func(serviceID uuid.UUID) {
			defer wg.Done()
			_ = cache.DeleteImpactCache(ctx, serviceID)
		}(id)
	}
	wg.Wait()

	return nil
}

func (s *ImpactCacheService) GetStats() *ImpactCacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &ImpactCacheStats{
		PrecomputedCount: s.precomputedCount,
		LastPrecompute:   s.lastPrecompute,
		IsRunning:        s.running,
		MaxDepth:         s.maxDepth,
		BatchSize:        s.batchSize,
		Interval:         s.precomputeInterval,
	}
}

type ImpactCacheStats struct {
	PrecomputedCount int64
	LastPrecompute   time.Time
	IsRunning        bool
	MaxDepth         int
	BatchSize        int
	Interval         time.Duration
}

type ImpactPrecomputeJob struct {
	ServiceIDs []uuid.UUID
	Priority   int
	CreatedAt  time.Time
}

type ImpactPrecomputeQueue struct {
	jobs []ImpactPrecomputeJob
	mu   sync.Mutex
}

func NewImpactPrecomputeQueue() *ImpactPrecomputeQueue {
	return &ImpactPrecomputeQueue{
		jobs: make([]ImpactPrecomputeJob, 0),
	}
}

func (q *ImpactPrecomputeQueue) Push(job ImpactPrecomputeJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, existing := range q.jobs {
		if job.Priority > existing.Priority {
			q.jobs = append(q.jobs[:i], append([]ImpactPrecomputeJob{job}, q.jobs[i:]...)...)
			return
		}
	}

	q.jobs = append(q.jobs, job)
}

func (q *ImpactPrecomputeQueue) Pop() *ImpactPrecomputeJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}

	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return &job
}

func (q *ImpactPrecomputeQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

type ImpactDiff struct {
	ServiceID       uuid.UUID
	PreviousImpact  *domain.ImpactResult
	CurrentImpact   *domain.ImpactResult
	ChangeType      string
	ChangeMagnitude float64
	DetectedAt      time.Time
}

func (s *ImpactCacheService) DetectImpactChanges(ctx context.Context, serviceIDs []uuid.UUID) ([]ImpactDiff, error) {
	var diffs []ImpactDiff

	for _, serviceID := range serviceIDs {
		cached, err := s.GetImpact(ctx, serviceID)
		if err != nil {
			continue
		}

		current, err := s.analyzer.AnalyzeImpact(ctx, serviceID, s.maxDepth)
		if err != nil {
			continue
		}

		diff := s.compareImpactResults(serviceID, cached, current)
		if diff != nil {
			diffs = append(diffs, *diff)
		}
	}

	return diffs, nil
}

func (s *ImpactCacheService) compareImpactResults(serviceID uuid.UUID, previous, current *domain.ImpactResult) *ImpactDiff {
	if previous == nil || current == nil {
		return nil
	}

	upstreamDiff := len(current.Upstream) - len(previous.Upstream)
	downstreamDiff := len(current.Downstream) - len(previous.Downstream)

	if upstreamDiff == 0 && downstreamDiff == 0 {
		return nil
	}

	changeType := "unknown"
	magnitude := float64(0)

	if upstreamDiff > 0 || downstreamDiff > 0 {
		changeType = "expansion"
		magnitude = float64(upstreamDiff + downstreamDiff)
	} else {
		changeType = "contraction"
		magnitude = float64(-upstreamDiff - downstreamDiff)
	}

	return &ImpactDiff{
		ServiceID:       serviceID,
		PreviousImpact:  previous,
		CurrentImpact:   current,
		ChangeType:      changeType,
		ChangeMagnitude: magnitude,
		DetectedAt:      time.Now(),
	}
}

func (s *ImpactCacheService) Warmup(ctx context.Context, criticalServices []uuid.UUID) error {
	for _, serviceID := range criticalServices {
		if err := s.PrecomputeService(ctx, serviceID); err != nil {
			log.Printf("[ImpactCacheService] Warmup failed for %s: %v", serviceID, err)
			continue
		}
	}
	return nil
}

func (s *ImpactCacheService) RefreshStale(ctx context.Context, maxAge time.Duration) (int, error) {
	s.mu.RLock()
	lastPrecompute := s.lastPrecompute
	s.mu.RUnlock()

	if time.Since(lastPrecompute) < maxAge {
		return 0, nil
	}

	err := s.PrecomputeAll(ctx)
	if err != nil {
		return 0, err
	}

	return int(s.precomputedCount), nil
}
