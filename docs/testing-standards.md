# Cloud Agent Monitor 测试标准文档

> 本文档定义了 cloud-agent-monitor 项目的完整测试标准，涵盖单元测试、并发测试、海量数据模拟测试、稳定性测试以及 AI Infra 专项测试。

---

## 目录

1. [测试目标与原则](#1-测试目标与原则)
2. [单元测试规范](#2-单元测试规范)
3. [并发测试方案](#3-并发测试方案)
4. [海量数据模拟测试](#4-海量数据模拟测试)
5. [稳定性测试](#5-稳定性测试)
6. [AI Infra 专项测试](#6-ai-infra-专项测试)
7. [测试覆盖率目标](#7-测试覆盖率目标)
8. [测试工具与 CI 集成](#8-测试工具与-ci-集成)
9. [测试最佳实践清单](#9-测试最佳实践清单)

---

## 1. 测试目标与原则

### 1.1 测试目标

| 目标 | 说明 | 衡量指标 |
|------|------|----------|
| **功能正确性** | 确保所有功能按预期工作 | 单元测试通过率 100% |
| **并发安全性** | 验证系统在并发场景下的正确性 | 无数据竞争、无死锁 |
| **性能可预测性** | 确保系统在海量数据下性能可接受 | P99 延迟 < 阈值 |
| **稳定性** | 验证系统长时间运行的稳定性 | 无内存泄漏、无 goroutine 泄漏 |
| **AI Infra 可靠性** | 确保 AI 观测能力的准确性 | 指标精度 > 99.9% |

### 1.2 测试金字塔

```
                    ┌─────────────┐
                    │   E2E 测试   │  5%
                    └─────────────┘
               ┌──────────────────────┐
               │    集成测试           │ 15%
               └──────────────────────┘
          ┌─────────────────────────────────┐
          │         单元测试                 │ 80%
          └─────────────────────────────────┘
```

### 1.3 测试原则

1. **快速反馈**：单元测试必须 < 1ms，集成测试 < 1s
2. **独立性**：每个测试必须独立运行，不依赖执行顺序
3. **可重复性**：相同输入必须产生相同结果
4. **自验证**：测试结果必须自动判定，无需人工干预
5. **完整性**：覆盖正常路径、边界条件、错误路径

---

## 2. 单元测试规范

### 2.1 测试文件组织

```
internal/aiinfra/
├── domain/
│   ├── model.go
│   └── model_test.go          # 同包测试（白盒）
├── application/
│   ├── service.go
│   └── service_test.go
└── infrastructure/
    ├── repository.go
    └── repository_test.go     # 异包测试（黑盒）
```

### 2.2 表驱动测试（Table-Driven Tests）

**强制要求**：所有测试必须使用表驱动模式。

```go
func TestAISession_CalculateCost(t *testing.T) {
    tests := []struct {
        name              string
        session           *domain.AISession
        model             *domain.AIModel
        expectedCost      float64
        expectError       bool
        errorMessage      string
    }{
        {
            name: "standard chat session",
            session: &domain.AISession{
                GenAIUsageInputTokens:  1000,
                GenAIUsageOutputTokens: 500,
            },
            model: &domain.AIModel{
                CostPerInputToken:  0.00003,
                CostPerOutputToken: 0.00006,
            },
            expectedCost: 0.06,
            expectError:  false,
        },
        {
            name: "zero tokens",
            session: &domain.AISession{
                GenAIUsageInputTokens:  0,
                GenAIUsageOutputTokens: 0,
            },
            model: &domain.AIModel{
                CostPerInputToken:  0.00003,
                CostPerOutputToken: 0.00006,
            },
            expectedCost: 0.0,
            expectError:  false,
        },
        {
            name: "nil model returns error",
            session: &domain.AISession{
                GenAIUsageInputTokens:  100,
                GenAIUsageOutputTokens: 50,
            },
            model:        nil,
            expectError:  true,
            errorMessage: "model cannot be nil",
        },
        {
            name: "large token count",
            session: &domain.AISession{
                GenAIUsageInputTokens:  1_000_000,
                GenAIUsageOutputTokens: 500_000,
            },
            model: &domain.AIModel{
                CostPerInputToken:  0.00003,
                CostPerOutputToken: 0.00006,
            },
            expectedCost: 60.0,
            expectError:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cost, err := tt.session.CalculateCost(tt.model)

            if tt.expectError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMessage)
                return
            }

            require.NoError(t, err)
            assert.InDelta(t, tt.expectedCost, cost, 0.0001)
        })
    }
}
```

### 2.3 子测试命名规范

| 测试场景 | 命名模式 | 示例 |
|----------|----------|------|
| 正常路径 | `standard <operation>` | `standard chat session` |
| 边界条件 | `boundary <condition>` | `boundary zero tokens` |
| 错误路径 | `<condition> returns error` | `nil model returns error` |
| 大数据量 | `large <metric>` | `large token count` |
| 并发场景 | `concurrent <operation>` | `concurrent session creation` |

### 2.4 Mock 规范

**原则**：Mock 接口，不 Mock 具体类型。

```go
type MockAISessionRepository struct {
    mock.Mock
}

func (m *MockAISessionRepository) Create(ctx context.Context, session *domain.AISession) error {
    args := m.Called(ctx, session)
    return args.Error(0)
}

func (m *MockAISessionRepository) GetByID(ctx context.Context, id string) (*domain.AISession, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.AISession), args.Error(1)
}

func TestSessionService_Create(t *testing.T) {
    mockRepo := new(MockAISessionRepository)
    service := application.NewSessionService(mockRepo)

    session := &domain.AISession{
        ID:      "test-session-1",
        ModelID: "gpt-4o",
    }

    mockRepo.On("Create", mock.Anything, session).Return(nil)

    err := service.CreateSession(context.Background(), session)

    require.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### 2.5 测试隔离

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}

func TestConcurrentAccess(t *testing.T) {
    t.Parallel()
    
    // 使用独立的测试数据
    testID := uuid.New().String()
    
    // 清理资源
    t.Cleanup(func() {
        cleanupTestData(testID)
    })
}
```

---

## 3. 并发测试方案

### 3.1 数据竞争检测

**强制要求**：所有测试必须在 CI 中启用 `-race` 标志。

```bash
go test -race ./...
```

### 3.2 并发测试模式

#### 3.2.1 并发写入测试

```go
func TestAISessionRepository_ConcurrentCreate(t *testing.T) {
    repo := setupTestRepository(t)
    
    const goroutines = 100
    const sessionsPerGoroutine = 10
    
    var wg sync.WaitGroup
    errCh := make(chan error, goroutines*sessionsPerGoroutine)
    
    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < sessionsPerGoroutine; j++ {
                session := &domain.AISession{
                    ID:     fmt.Sprintf("session-%d-%d", workerID, j),
                    ModelID: "gpt-4o",
                }
                
                if err := repo.Create(context.Background(), session); err != nil {
                    errCh <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errCh)
    
    var errors []error
    for err := range errCh {
        errors = append(errors, err)
    }
    
    assert.Empty(t, errors, "concurrent writes should not fail")
    
    sessions, err := repo.List(context.Background(), nil)
    require.NoError(t, err)
    assert.Equal(t, goroutines*sessionsPerGoroutine, len(sessions))
}
```

#### 3.2.2 并发读写测试

```go
func TestAISessionRepository_ConcurrentReadWrite(t *testing.T) {
    repo := setupTestRepository(t)
    
    const writers = 50
    const readers = 50
    const duration = 5 * time.Second
    
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()
    
    var wg sync.WaitGroup
    
    for i := 0; i < writers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for {
                select {
                case <-ctx.Done():
                    return
                default:
                    session := &domain.AISession{
                        ID:      uuid.New().String(),
                        ModelID: "gpt-4o",
                    }
                    repo.Create(ctx, session)
                }
            }
        }(i)
    }
    
    for i := 0; i < readers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            for {
                select {
                case <-ctx.Done():
                    return
                default:
                    repo.List(ctx, &domain.SessionFilter{Limit: 100})
                }
            }
        }()
    }
    
    wg.Wait()
}
```

### 3.3 Goroutine 泄漏检测

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m, goleak.IgnoreCurrent())
}

func TestCollector_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    
    collector := infrastructure.NewMetricsCollector()
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go collector.Start(ctx)
    
    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(100 * time.Millisecond)
}
```

### 3.4 Channel 死锁检测

```go
func TestToolCallExecutor_NoDeadlock(t *testing.T) {
    executor := NewToolCallExecutor(10)
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    const numCalls = 1000
    
    for i := 0; i < numCalls; i++ {
        select {
        case executor.Submit(&domain.ToolCall{ID: uuid.New().String()}):
        case <-ctx.Done():
            t.Fatal("deadlock detected: submit blocked")
        }
    }
    
    require.Eventually(t, func() bool {
        return executor.ProcessedCount() == numCalls
    }, 5*time.Second, 10*time.Millisecond, "all calls should be processed")
}
```

---

## 4. 海量数据模拟测试

### 4.1 数据生成策略

#### 4.1.1 分层数据量定义

| 测试类型 | 数据量 | 执行时机 |
|----------|--------|----------|
| 单元测试 | 10-100 条 | 每次提交 |
| 集成测试 | 1,000-10,000 条 | 每次合并 |
| 压力测试 | 100,000-1,000,000 条 | 每日构建 |
| 极限测试 | 10,000,000+ 条 | 每周构建 |

#### 4.1.2 数据生成器

```go
type SessionGenerator struct {
    rng *rand.Rand
}

func (g *SessionGenerator) Generate(count int) []*domain.AISession {
    sessions := make([]*domain.AISession, count)
    
    models := []string{"gpt-4o", "gpt-4-turbo", "claude-3-opus", "claude-3-sonnet"}
    statuses := []domain.SessionStatus{
        domain.SessionStatusSuccess,
        domain.SessionStatusError,
        domain.SessionStatusThrottled,
    }
    
    for i := 0; i < count; i++ {
        sessions[i] = &domain.AISession{
            ID:                     uuid.New().String(),
            TraceID:                uuid.New().String(),
            ModelID:                models[g.rng.Intn(len(models))],
            GenAIUsageInputTokens:  g.rng.Intn(10000),
            GenAIUsageOutputTokens: g.rng.Intn(5000),
            Status:                 statuses[g.rng.Intn(len(statuses))],
            DurationMs:             g.rng.Intn(10000),
            CreatedAt:              time.Now().Add(-time.Duration(g.rng.Intn(86400)) * time.Second),
        }
        sessions[i].GenAIUsageTotalTokens = sessions[i].GenAIUsageInputTokens + sessions[i].GenAIUsageOutputTokens
    }
    
    return sessions
}

func (g *SessionGenerator) GenerateWithPattern(count int, pattern string) []*domain.AISession {
    switch pattern {
    case "burst":
        return g.generateBurstPattern(count)
    case "gradual":
        return g.generateGradualPattern(count)
    case "periodic":
        return g.generatePeriodicPattern(count)
    default:
        return g.Generate(count)
    }
}
```

### 4.2 批量插入性能测试

```go
func TestAISessionRepository_BatchInsert_Performance(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance test in short mode")
    }
    
    repo := setupTestRepository(t)
    generator := &SessionGenerator{rng: rand.New(rand.NewSource(42))}
    
    benchmarks := []struct {
        name  string
        count int
    }{
        {"1K sessions", 1_000},
        {"10K sessions", 10_000},
        {"100K sessions", 100_000},
        {"1M sessions", 1_000_000},
    }
    
    for _, bm := range benchmarks {
        t.Run(bm.name, func(t *testing.T) {
            sessions := generator.Generate(bm.count)
            
            start := time.Now()
            err := repo.BatchCreate(context.Background(), sessions)
            duration := time.Since(start)
            
            require.NoError(t, err)
            
            throughput := float64(bm.count) / duration.Seconds()
            t.Logf("Throughput: %.2f sessions/sec", throughput)
            
            assert.Less(t, duration, 30*time.Second, "batch insert should complete within 30s")
        })
    }
}
```

### 4.3 查询性能测试

```go
func TestAISessionRepository_Query_Performance(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance test in short mode")
    }
    
    repo := setupTestRepository(t)
    
    seedTestData(t, repo, 100_000)
    
    tests := []struct {
        name     string
        filter   *domain.SessionFilter
        maxTime  time.Duration
    }{
        {
            name:    "filter by model",
            filter:  &domain.SessionFilter{ModelID: "gpt-4o", Limit: 100},
            maxTime: 100 * time.Millisecond,
        },
        {
            name:    "filter by time range",
            filter:  &domain.SessionFilter{StartTime: time.Now().Add(-24 * time.Hour), Limit: 100},
            maxTime: 200 * time.Millisecond,
        },
        {
            name:    "filter by status",
            filter:  &domain.SessionFilter{Status: domain.SessionStatusError, Limit: 100},
            maxTime: 100 * time.Millisecond,
        },
        {
            name:    "complex filter",
            filter:  &domain.SessionFilter{ModelID: "gpt-4o", Status: domain.SessionStatusSuccess, Limit: 100},
            maxTime: 150 * time.Millisecond,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            start := time.Now()
            sessions, err := repo.List(context.Background(), tt.filter)
            duration := time.Since(start)
            
            require.NoError(t, err)
            assert.Less(t, duration, tt.maxTime)
            t.Logf("Query time: %v, Results: %d", duration, len(sessions))
        })
    }
}
```

### 4.4 内存压力测试

```go
func TestAISessionRepository_MemoryPressure(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping memory pressure test in short mode")
    }
    
    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)
    
    repo := setupTestRepository(t)
    generator := &SessionGenerator{rng: rand.New(rand.NewSource(42))}
    
    const totalSessions = 1_000_000
    const batchSize = 10_000
    
    for i := 0; i < totalSessions; i += batchSize {
        sessions := generator.Generate(batchSize)
        require.NoError(t, repo.BatchCreate(context.Background(), sessions))
        
        if i%100_000 == 0 {
            runtime.GC()
        }
    }
    
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    memIncrease := float64(m2.Alloc-m1.Alloc) / 1024 / 1024
    t.Logf("Memory increase: %.2f MB", memIncrease)
    
    assert.Less(t, memIncrease, 500.0, "memory increase should be < 500MB")
}
```

---

## 5. 稳定性测试

### 5.1 长时间运行测试

```go
func TestCollector_LongRunning(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping long-running test in short mode")
    }
    
    collector := infrastructure.NewMetricsCollector()
    
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
    defer cancel()
    
    var wg sync.WaitGroup
    errorCount := int64(0)
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        collector.Start(ctx)
    }()
    
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                if err := collector.HealthCheck(); err != nil {
                    atomic.AddInt64(&errorCount, 1)
                }
            }
        }
    }()
    
    wg.Wait()
    
    assert.Equal(t, int64(0), errorCount, "no errors should occur during long run")
}
```

### 5.2 错误注入测试

```go
func TestAISessionRepository_ErrorInjection(t *testing.T) {
    repo := setupTestRepository(t)
    
    tests := []struct {
        name        string
        injectError func()
        operation   func() error
        expectError bool
    }{
        {
            name: "database connection lost",
            injectError: func() {
                repo.SimulateConnectionLoss()
            },
            operation: func() error {
                return repo.Create(context.Background(), &domain.AISession{ID: "test"})
            },
            expectError: true,
        },
        {
            name: "timeout scenario",
            injectError: func() {
                repo.SimulateSlowQuery(5 * time.Second)
            },
            operation: func() error {
                ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
                defer cancel()
                _, err := repo.List(ctx, nil)
                return err
            },
            expectError: true,
        },
        {
            name: "disk full simulation",
            injectError: func() {
                repo.SimulateDiskFull()
            },
            operation: func() error {
                return repo.Create(context.Background(), &domain.AISession{ID: "test"})
            },
            expectError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            defer repo.ResetSimulation()
            
            tt.injectError()
            err := tt.operation()
            
            if tt.expectError {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### 5.3 资源泄漏检测

```go
func TestNoResourceLeak(t *testing.T) {
    var beforeFiles, afterFiles []string
    
    beforeFiles = getOpenFiles()
    
    for i := 0; i < 1000; i++ {
        repo, err := NewAISessionRepository(config)
        require.NoError(t, err)
        
        session := &domain.AISession{ID: uuid.New().String()}
        err = repo.Create(context.Background(), session)
        require.NoError(t, err)
        
        repo.Close()
    }
    
    runtime.GC()
    time.Sleep(100 * time.Millisecond)
    
    afterFiles = getOpenFiles()
    
    leakedFiles := diffFiles(beforeFiles, afterFiles)
    assert.Empty(t, leakedFiles, "no file descriptors should leak")
}

func TestNoConnectionLeak(t *testing.T) {
    db := setupTestDB(t)
    
    beforeStats := db.Stats()
    
    for i := 0; i < 100; i++ {
        repo := NewAISessionRepository(db)
        repo.Close()
    }
    
    runtime.GC()
    time.Sleep(100 * time.Millisecond)
    
    afterStats := db.Stats()
    
    assert.Equal(t, beforeStats.OpenConnections, afterStats.OpenConnections,
        "no connections should leak")
}
```

### 5.4 优雅关闭测试

```go
func TestGracefulShutdown(t *testing.T) {
    service := application.NewSessionService(repo)
    
    ctx, cancel := context.WithCancel(context.Background())
    
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        service.Run(ctx)
    }()
    
    time.Sleep(100 * time.Millisecond)
    
    cancel()
    
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
    case <-time.After(10 * time.Second):
        t.Fatal("shutdown took too long")
    }
    
    assert.True(t, service.IsCleanShutdown())
}
```

---

## 6. AI Infra 专项测试

### 6.1 GPU 指标采集测试

#### 6.1.1 Mock DCGM Exporter

```go
type MockDCGMExporter struct {
    server *httptest.Server
    metrics map[string]float64
}

func NewMockDCGMExporter() *MockDCGMExporter {
    m := &MockDCGMExporter{
        metrics: make(map[string]float64),
    }
    m.server = httptest.NewServer(http.HandlerFunc(m.handleMetrics))
    return m
}

func (m *MockDCGMExporter) SetMetric(name string, value float64) {
    m.metrics[name] = value
}

func (m *MockDCGMExporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
    var lines []string
    for name, value := range m.metrics {
        lines = append(lines, fmt.Sprintf("%s{gpu=\"0\"} %.2f", name, value))
    }
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte(strings.Join(lines, "\n")))
}

func TestGPUMetricsCollector(t *testing.T) {
    mockDCGM := NewMockDCGMExporter()
    defer mockDCGM.Close()
    
    mockDCGM.SetMetric("DCGM_FI_DEV_GPU_UTIL", 85.5)
    mockDCGM.SetMetric("DCGM_FI_DEV_FB_FREE", 40960.0)
    mockDCGM.SetMetric("DCGM_FI_DEV_GPU_TEMP", 72.0)
    mockDCGM.SetMetric("DCGM_FI_DEV_POWER_USAGE", 350.0)
    
    collector := infrastructure.NewGPUMetricsCollector(mockDCGM.URL())
    
    metrics, err := collector.Collect(context.Background())
    require.NoError(t, err)
    
    assert.Equal(t, 85.5, metrics.GPUUtilization)
    assert.Equal(t, 40960.0, metrics.FreeMemory)
    assert.Equal(t, 72.0, metrics.Temperature)
    assert.Equal(t, 350.0, metrics.PowerUsage)
}
```

#### 6.1.2 GPU 指标精度测试

```go
func TestGPUMetrics_Precision(t *testing.T) {
    tests := []struct {
        name       string
        rawValue   float64
        expected   float64
        precision  float64
    }{
        {"utilization percentage", 85.567, 85.57, 0.01},
        {"memory in MB", 40960.123, 40960.12, 0.01},
        {"temperature", 72.5, 72.5, 0.1},
        {"power in watts", 350.789, 350.79, 0.01},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := roundToPrecision(tt.rawValue, tt.precision)
            assert.InDelta(t, tt.expected, result, tt.precision/10)
        })
    }
}
```

### 6.2 推理服务指标测试

#### 6.2.1 TTFT/TPOT 测试

```go
func TestInferenceMetrics_TTFT_TPOT(t *testing.T) {
    mockVLLM := NewMockVLLMServer()
    defer mockVLLM.Close()
    
    collector := infrastructure.NewInferenceMetricsCollector(mockVLLM.URL())
    
    tests := []struct {
        name           string
        mockTTFT       time.Duration
        mockTPOT       time.Duration
        expectedStatus string
    }{
        {
            name:           "fast response",
            mockTTFT:       200 * time.Millisecond,
            mockTPOT:       30 * time.Millisecond,
            expectedStatus: "healthy",
        },
        {
            name:           "slow TTFT",
            mockTTFT:       3 * time.Second,
            mockTPOT:       40 * time.Millisecond,
            expectedStatus: "degraded",
        },
        {
            name:           "slow TPOT",
            mockTTFT:       400 * time.Millisecond,
            mockTPOT:       150 * time.Millisecond,
            expectedStatus: "degraded",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockVLLM.SetTTFT(tt.mockTTFT)
            mockVLLM.SetTPOT(tt.mockTPOT)
            
            metrics, err := collector.Collect(context.Background())
            require.NoError(t, err)
            
            assert.Equal(t, tt.mockTTFT, metrics.TTFT)
            assert.Equal(t, tt.mockTPOT, metrics.TPOT)
            assert.Equal(t, tt.expectedStatus, metrics.Status)
        })
    }
}
```

#### 6.2.2 KV Cache 监控测试

```go
func TestKVCache_Monitoring(t *testing.T) {
    mockVLLM := NewMockVLLMServer()
    defer mockVLLM.Close()
    
    collector := infrastructure.NewInferenceMetricsCollector(mockVLLM.URL())
    
    tests := []struct {
        name          string
        cacheUsage    float64
        expectWarning bool
    }{
        {"normal usage", 60.0, false},
        {"high usage", 85.0, false},
        {"critical usage", 95.0, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockVLLM.SetKVCacheUsage(tt.cacheUsage)
            
            metrics, err := collector.Collect(context.Background())
            require.NoError(t, err)
            
            assert.Equal(t, tt.cacheUsage, metrics.KVCacheUsage)
            assert.Equal(t, tt.expectWarning, metrics.KVCacheWarning)
        })
    }
}
```

### 6.3 Token 计费测试

#### 6.3.1 成本计算测试

```go
func TestTokenCost_Calculation(t *testing.T) {
    tests := []struct {
        name              string
        inputTokens       int
        outputTokens      int
        inputCostPerToken float64
        outputCostPerToken float64
        expectedCost      float64
    }{
        {
            name:              "GPT-4o standard",
            inputTokens:       1000,
            outputTokens:      500,
            inputCostPerToken: 0.0000025,
            outputCostPerToken: 0.00001,
            expectedCost:      0.0075,
        },
        {
            name:              "Claude-3 Opus",
            inputTokens:       1000,
            outputTokens:      500,
            inputCostPerToken: 0.000015,
            outputCostPerToken: 0.000075,
            expectedCost:      0.0525,
        },
        {
            name:              "zero tokens",
            inputTokens:       0,
            outputTokens:      0,
            inputCostPerToken: 0.0000025,
            outputCostPerToken: 0.00001,
            expectedCost:      0.0,
        },
        {
            name:              "large batch",
            inputTokens:       1_000_000,
            outputTokens:      500_000,
            inputCostPerToken: 0.0000025,
            outputCostPerToken: 0.00001,
            expectedCost:      7.5,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cost := domain.CalculateTokenCost(
                tt.inputTokens,
                tt.outputTokens,
                tt.inputCostPerToken,
                tt.outputCostPerToken,
            )
            assert.InDelta(t, tt.expectedCost, cost, 0.0001)
        })
    }
}
```

#### 6.3.2 预算控制测试

```go
func TestBudgetController_Enforcement(t *testing.T) {
    controller := infrastructure.NewBudgetController(repo)
    
    tests := []struct {
        name           string
        budget         *domain.Budget
        currentUsage   float64
        requestCost    float64
        expectAllowed  bool
        expectThrottle bool
    }{
        {
            name: "under budget",
            budget: &domain.Budget{
                DailyLimit: 100.0,
                ThrottleThreshold: 0.8,
            },
            currentUsage:   50.0,
            requestCost:    10.0,
            expectAllowed:  true,
            expectThrottle: false,
        },
        {
            name: "at throttle threshold",
            budget: &domain.Budget{
                DailyLimit: 100.0,
                ThrottleThreshold: 0.8,
            },
            currentUsage:   80.0,
            requestCost:    5.0,
            expectAllowed:  true,
            expectThrottle: true,
        },
        {
            name: "over budget",
            budget: &domain.Budget{
                DailyLimit: 100.0,
                ThrottleThreshold: 0.8,
            },
            currentUsage:   100.0,
            requestCost:    1.0,
            expectAllowed:  false,
            expectThrottle: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            decision := controller.Evaluate(context.Background(), tt.budget, tt.currentUsage, tt.requestCost)
            
            assert.Equal(t, tt.expectAllowed, decision.Allowed)
            assert.Equal(t, tt.expectThrottle, decision.Throttle)
        })
    }
}
```

### 6.4 OpenTelemetry 语义约定测试

```go
func TestSemanticConventions_Compliance(t *testing.T) {
    session := &domain.AISession{
        GenAISystem:        "openai",
        GenAIOperationName: "chat",
        GenAIRequestModel:  "gpt-4o",
        GenAIUsageInputTokens:  1000,
        GenAIUsageOutputTokens: 500,
    }
    
    attrs := session.ToOTelAttributes()
    
    requiredAttrs := []string{
        "gen_ai.system",
        "gen_ai.operation.name",
        "gen_ai.request.model",
        "gen_ai.usage.input_tokens",
        "gen_ai.usage.output_tokens",
    }
    
    for _, attr := range requiredAttrs {
        assert.Contains(t, attrs, attr, "missing required OTel attribute: %s", attr)
    }
}

func TestSemanticConventions_AttributeTypes(t *testing.T) {
    session := &domain.AISession{
        GenAISystem:            "openai",
        GenAIRequestMaxTokens:  4096,
        GenAIRequestTemperature: 0.7,
    }
    
    attrs := session.ToOTelAttributes()
    
    assert.IsType(t, attribute.StringValue(""), attrs["gen_ai.system"])
    assert.IsType(t, attribute.Int64Value(0), attrs["gen_ai.request.max_tokens"])
    assert.IsType(t, attribute.Float64Value(0), attrs["gen_ai.request.temperature"])
}
```

### 6.5 工具调用审计测试

```go
func TestToolCall_AuditTrail(t *testing.T) {
    repo := setupTestRepository(t)
    auditor := application.NewToolCallAuditor(repo)
    
    toolCall := &domain.ToolCall{
        ID:            uuid.New().String(),
        SessionID:     "session-1",
        GenAIToolName: "execute_query",
        GenAIToolType: "function",
        Arguments: map[string]interface{}{
            "query": "SELECT * FROM users",
        },
        Status:   domain.ToolCallStatusSuccess,
        DurationMs: 150,
    }
    
    err := auditor.Record(context.Background(), toolCall)
    require.NoError(t, err)
    
    recorded, err := repo.GetByID(context.Background(), toolCall.ID)
    require.NoError(t, err)
    
    assert.Equal(t, toolCall.GenAIToolName, recorded.GenAIToolName)
    assert.Equal(t, toolCall.Arguments, recorded.Arguments)
    assert.Equal(t, toolCall.Status, recorded.Status)
}

func TestToolCall_SensitiveDataMasking(t *testing.T) {
    auditor := application.NewToolCallAuditor(repo)
    
    toolCall := &domain.ToolCall{
        ID:            uuid.New().String(),
        GenAIToolName: "execute_query",
        Arguments: map[string]interface{}{
            "query": "SELECT * FROM users WHERE password = 'secret123'",
        },
    }
    
    err := auditor.Record(context.Background(), toolCall)
    require.NoError(t, err)
    
    recorded, _ := repo.GetByID(context.Background(), toolCall.ID)
    
    assert.NotContains(t, fmt.Sprintf("%v", recorded.Arguments), "secret123")
    assert.Contains(t, fmt.Sprintf("%v", recorded.Arguments), "***MASKED***")
}
```

---

## 7. 测试覆盖率目标

### 7.1 覆盖率分级目标

| 模块 | 单元测试覆盖率 | 集成测试覆盖率 | 总覆盖率目标 |
|------|----------------|----------------|--------------|
| **domain** | ≥ 90% | N/A | ≥ 90% |
| **application** | ≥ 85% | ≥ 70% | ≥ 80% |
| **infrastructure** | ≥ 70% | ≥ 80% | ≥ 75% |
| **interfaces** | ≥ 60% | ≥ 85% | ≥ 75% |
| **整体项目** | ≥ 75% | ≥ 70% | ≥ 80% |

### 7.2 关键路径覆盖率

**强制要求**：以下关键路径必须达到 100% 覆盖：

1. **Token 计费逻辑**
2. **预算控制决策**
3. **GPU 指标采集**
4. **推理性能指标计算**
5. **工具调用审计记录**
6. **错误处理路径**

### 7.3 覆盖率报告生成

```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...

# 按函数查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html

# 查看总覆盖率
go tool cover -func=coverage.out | grep total
```

### 7.4 覆盖率 CI 门禁

```yaml
# .github/workflows/test.yml
- name: Run tests with coverage
  run: go test -coverprofile=coverage.out -covermode=atomic ./...

- name: Check coverage threshold
  run: |
    coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    if [ $(echo "$coverage < 80" | bc) -eq 1 ]; then
      echo "Coverage $coverage% is below threshold 80%"
      exit 1
    fi
```

---

## 8. 测试工具与 CI 集成

### 8.1 测试工具清单

| 工具 | 用途 | 安装命令 |
|------|------|----------|
| `testify` | 断言和 Mock | `go get github.com/stretchr/testify` |
| `goleak` | Goroutine 泄漏检测 | `go get go.uber.org/goleak` |
| `mockgen` | Mock 生成 | `go install github.com/golang/mock/mockgen` |
| `gotests` | 测试生成 | `go install github.com/cweill/gotests/...` |
| `benchstat` | 基准测试分析 | `go install golang.org/x/perf/cmd/benchstat` |

### 8.2 Makefile 测试命令

```makefile
.PHONY: test test-unit test-integration test-race test-coverage test-benchmark

test:
	go test ./...

test-unit:
	go test -short ./...

test-integration:
	go test -tags=integration ./...

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

test-benchmark:
	go test -bench=. -benchmem ./...

test-long:
	go test -v -count=1 -timeout=2h ./...
```

### 8.3 GitHub Actions CI 配置

```yaml
# .github/workflows/test.yml
name: Test

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run unit tests
        run: go test -short -race -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out

  integration-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_USER: obs
          POSTGRES_PASSWORD: test
          POSTGRES_DB: test_db
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run integration tests
        run: go test -tags=integration -race ./...
        env:
          DATABASE_URL: postgres://obs:test@localhost:5432/test_db?sslmode=disable

  stress-test:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Run stress tests
        run: go test -run TestStress -v -timeout=30m ./...

  coverage-check:
    runs-on: ubuntu-latest
    needs: [unit-test, integration-test]
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Generate coverage report
        run: |
          go test -coverprofile=coverage.out -covermode=atomic ./...
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: $coverage%"
          
          if [ $(echo "$coverage < 80" | bc) -eq 1 ]; then
            echo "::error::Coverage $coverage% is below threshold 80%"
            exit 1
          fi
```

### 8.4 Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running pre-commit checks..."

echo "1. Running unit tests..."
go test -short ./...
if [ $? -ne 0 ]; then
    echo "Unit tests failed!"
    exit 1
fi

echo "2. Running race detector..."
go test -race -short ./...
if [ $? -ne 0 ]; then
    echo "Race detected!"
    exit 1
fi

echo "3. Checking coverage..."
go test -coverprofile=coverage.out -short ./...
coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
if [ $(echo "$coverage < 75" | bc) -eq 1 ]; then
    echo "Warning: Coverage $coverage% is below 75%"
fi

echo "All checks passed!"
```

---

## 9. 测试最佳实践清单

### 9.1 单元测试清单

- [ ] 使用表驱动测试
- [ ] 每个测试用例有明确的名称
- [ ] 覆盖正常路径、边界条件、错误路径
- [ ] 使用 `t.Parallel()` 加速独立测试
- [ ] Mock 接口而非具体类型
- [ ] 测试名称遵循 `Test<Function>_<Scenario>` 格式
- [ ] 使用 `require` 处理前置条件，`assert` 处理断言
- [ ] 清理测试资源（使用 `t.Cleanup`）

### 9.2 并发测试清单

- [ ] 启用 `-race` 标志
- [ ] 使用 `goleak` 检测 goroutine 泄漏
- [ ] 测试并发写入、并发读写场景
- [ ] 验证无死锁
- [ ] 测试 channel 缓冲区溢出场景
- [ ] 验证 context 取消的正确处理

### 9.3 性能测试清单

- [ ] 使用 `testing.Short()` 跳过长时间测试
- [ ] 设置合理的性能阈值
- [ ] 测试不同数据量级（1K、10K、100K、1M）
- [ ] 监控内存分配
- [ ] 使用 `b.ReportAllocs()` 报告内存分配
- [ ] 使用 `benchstat` 比较性能变化

### 9.4 稳定性测试清单

- [ ] 测试长时间运行（≥ 1小时）
- [ ] 注入各种错误场景
- [ ] 验证资源泄漏（文件描述符、连接、内存）
- [ ] 测试优雅关闭
- [ ] 验证错误恢复机制

### 9.5 AI Infra 测试清单

- [ ] Mock GPU 指标源（DCGM Exporter）
- [ ] Mock 推理服务（vLLM/Triton）
- [ ] 验证 TTFT/TPOT 计算准确性
- [ ] 测试 Token 计费精度
- [ ] 验证预算控制逻辑
- [ ] 测试 OpenTelemetry 语义约定合规性
- [ ] 验证敏感数据脱敏
- [ ] 测试审计日志完整性

---

## 附录 A：测试命令速查表

```bash
# 运行所有测试
go test ./...

# 运行特定测试
go test -run TestAISession ./...

# 运行子测试
go test -run TestAISession/standard_chat_session ./...

# 启用竞态检测
go test -race ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行基准测试
go test -bench=. -benchmem ./...

# 运行模糊测试
go test -fuzz=FuzzTokenCost ./...

# 运行集成测试
go test -tags=integration ./...

# 详细输出
go test -v ./...

# 限制测试时间
go test -timeout 30s ./...

# 运行短测试
go test -short ./...
```

---

## 附录 B：测试文件模板

### B.1 单元测试模板

```go
package domain_test

import (
    "context"
    "testing"
    
    "cloud-agent-monitor/internal/aiinfra/domain"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAISession_<Function>_<Scenario>(t *testing.T) {
    tests := []struct {
        name     string
        input    <InputType>
        expected <ExpectedType>
        wantErr  bool
    }{
        {
            name:     "standard case",
            input:    <input>,
            expected: <expected>,
            wantErr:  false,
        },
        {
            name:     "edge case",
            input:    <input>,
            expected: <expected>,
            wantErr:  false,
        },
        {
            name:     "error case",
            input:    <input>,
            wantErr:  true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := <Function>(tt.input)
            
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### B.2 并发测试模板

```go
func TestConcurrent_<Operation>(t *testing.T) {
    const goroutines = 100
    const operations = 10
    
    var wg sync.WaitGroup
    errCh := make(chan error, goroutines*operations)
    
    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < operations; j++ {
                if err := <operation>(); err != nil {
                    errCh <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errCh)
    
    var errors []error
    for err := range errCh {
        errors = append(errors, err)
    }
    
    assert.Empty(t, errors)
}
```

### B.3 基准测试模板

```go
func Benchmark<Function>(b *testing.B) {
    benchmarks := []struct {
        name string
        size int
    }{
        {"small", 100},
        {"medium", 10000},
        {"large", 1000000},
    }
    
    for _, bm := range benchmarks {
        b.Run(bm.name, func(b *testing.B) {
            b.ReportAllocs()
            data := generateTestData(bm.size)
            
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                <Function>(data)
            }
        })
    }
}
```

---

**文档版本**: v1.0  
**最后更新**: 2026-04-03  
**维护者**: Cloud Agent Monitor Team
