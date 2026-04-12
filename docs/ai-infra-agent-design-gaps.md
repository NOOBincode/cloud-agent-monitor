# AI Infra 观测与 Agent 方向设计缺陷分析

> 状态：设计评审
> 创建时间：2026-04-10
> 目的：识别 AI Infra 观测和 Agent 方向的设计缺陷，指导后续完善

---

## 1. 总览

本文档从两个核心方向分析当前设计缺陷：

| 方向 | 当前状态 | 核心问题 | 优先级 |
|------|----------|----------|--------|
| **AI Infra 观测** | Schema 已定义，代码空实现 | MFU 缺失、训练观测缺失、质量监控缺失 | ⭐⭐⭐ |
| **Agent 智能诊断** | 架构已设计，代码空实现 | 行为审计缺失、协作观测缺失、性能基准缺失 | ⭐⭐⭐ |

---

## 2. AI Infra 观测方向设计缺陷

### 2.1 MFU 指标观测缺失 ⚠️ **已记录**

详见 [ai-infra-module-analysis.md](./ai-infra-module-analysis.md) 4.2.4 节。

### 2.2 缺少模型性能基准测试框架

**问题描述**：
- 没有 benchmark 结果存储和对比机制
- 无法评估模型版本升级的性能变化
- 缺少不同配置（batch size、量化）的性能对比

**影响**：
- 无法量化模型优化效果
- 版本升级风险无法评估
- 配置调优缺乏数据支撑

**改进建议**：

```sql
-- 新增模型性能基准表
CREATE TABLE model_benchmarks (
    id CHAR(36) PRIMARY KEY,
    model_version_id CHAR(36) NOT NULL,
    
    -- 测试配置
    test_config JSON NOT NULL COMMENT 'batch_size, max_tokens, concurrency 等',
    test_dataset VARCHAR(255) COMMENT '测试数据集标识',
    
    -- 性能指标
    avg_ttft_ms INT COMMENT '平均 TTFT',
    p99_ttft_ms INT COMMENT 'P99 TTFT',
    avg_tpot_ms INT COMMENT '平均 TPOT',
    throughput_tokens_per_sec DECIMAL(10,2) COMMENT '吞吐量',
    
    -- 资源指标
    avg_gpu_util DECIMAL(5,2) COMMENT '平均 GPU 利用率',
    avg_memory_used_mb INT COMMENT '平均显存使用',
    mfu_percent DECIMAL(5,2) COMMENT 'MFU 百分比',
    
    -- 质量指标
    avg_perplexity DECIMAL(10,4) COMMENT '平均困惑度',
    
    -- 元数据
    benchmarked_at DATETIME NOT NULL,
    duration_seconds INT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (model_version_id) REFERENCES model_versions(id)
);

CREATE INDEX idx_benchmarks_version ON model_benchmarks(model_version_id);
CREATE INDEX idx_benchmarks_time ON model_benchmarks(benchmarked_at DESC);
```

```go
// domain/benchmark.go
type BenchmarkConfig struct {
    BatchSize      int     `json:"batch_size"`
    MaxTokens      int     `json:"max_tokens"`
    Concurrency    int     `json:"concurrency"`
    Temperature    float64 `json:"temperature"`
    TestDataset    string  `json:"test_dataset"`
    DurationSec    int     `json:"duration_seconds"`
}

type BenchmarkResult struct {
    ID              string
    ModelVersionID  string
    Config          BenchmarkConfig
    
    // 性能指标
    AvgTTFTMs       int
    P99TTFTMs       int
    AvgTPOTMs       int
    Throughput      float64
    
    // 资源指标
    AvgGPUUtil      float64
    AvgMemoryUsedMB int
    MFUPercent      float64
    
    // 质量指标
    AvgPerplexity   float64
    
    BenchmarkedAt   time.Time
}

type BenchmarkComparator struct {
    baseline *BenchmarkResult
    current  *BenchmarkResult
}

func (c *BenchmarkComparator) Compare() *BenchmarkDiff {
    return &BenchmarkDiff{
        TTFTChangePct:     float64(c.current.AvgTTFTMs-c.baseline.AvgTTFTMs) / float64(c.baseline.AvgTTFTMs) * 100,
        ThroughputChangePct: (c.current.Throughput - c.baseline.Throughput) / c.baseline.Throughput * 100,
        MFUChangePct:      c.current.MFUPercent - c.baseline.MFUPercent,
    }
}
```

### 2.3 缺少多模态 Token 计费

**问题描述**：
- 当前只支持文本 Token 计费
- 图像、音频、视频 Token 的计费模型缺失
- 多模态模型的成本无法准确核算

**影响**：
- GPT-4V、Gemini 等多模态模型成本核算不准
- 图像理解成本被低估

**改进建议**：

```go
// domain/multimodal.go
type TokenType string

const (
    TokenTypeText  TokenType = "text"
    TokenTypeImage TokenType = "image"
    TokenTypeAudio TokenType = "audio"
    TokenTypeVideo TokenType = "video"
)

type MultimodalTokenUsage struct {
    SessionID string
    
    TextTokens       int     // 文本 Token 数
    ImageTokens      int     // 图像 Token 数（按分辨率折算）
    AudioTokens      int     // 音频 Token 数（按时长折算）
    VideoTokens      int     // 视频 Token 数
    
    // 折算为标准 Token
    EquivalentTokens int     // 等效文本 Token 数
    
    // 成本
    TextCostUSD      float64
    ImageCostUSD     float64
    AudioCostUSD     float64
    TotalCostUSD     float64
}

type MultimodalPricing struct {
    ModelID string
    
    TextPerToken      float64  // 文本 Token 单价
    ImagePerTile      float64  // 图像 tile 单价（如 512x512）
    AudioPerSecond    float64  // 音频每秒单价
    VideoPerSecond    float64  // 视频每秒单价
    
    // 折算规则
    ImageTileToTokens int      // 1 tile 折算多少 Token
    AudioSecondToTokens int    // 1 秒音频折算多少 Token
}

func (p *MultimodalPricing) CalculateCost(usage *MultimodalTokenUsage) float64 {
    textCost := float64(usage.TextTokens) * p.TextPerToken
    imageCost := float64(usage.ImageTokens) * p.ImagePerTile / float64(p.ImageTileToTokens)
    audioCost := float64(usage.AudioTokens) * p.AudioPerSecond / float64(p.AudioSecondToTokens)
    return textCost + imageCost + audioCost
}
```

### 2.4 缺少实时推理质量监控

**问题描述**：
- 只有延迟、吞吐量等性能指标
- 缺少输出质量指标（perplexity、hallucination rate）
- 无法检测模型退化

**影响**：
- 模型输出质量下降无法及时发现
- 幻觉问题无法量化监控

**改进建议**：

```go
// domain/quality.go
type InferenceQualityMetrics struct {
    SessionID string
    
    // 输出质量
    Perplexity        float64  // 困惑度
    OutputLength      int      // 输出长度
    RepetitionScore   float64  // 重复度评分
    
    // 幻觉检测
    HallucinationScore    float64  // 幻觉评分 (0-1)
    FactualConsistency    float64  // 事实一致性
    GroundednessScore     float64  // 基于上下文的程度
    
    // 安全检测
    ToxicityScore     float64  // 毒性评分
    BiasScore         float64  // 偏见评分
    
    // 用户反馈
    UserRating        *int     // 用户评分 (1-5)
    UserFeedback      string   // 用户反馈文本
    
    CreatedAt time.Time
}

type QualityAlertRule struct {
    ID           string
    ModelID      string
    
    MetricName   string    // perplexity, hallucination_score, toxicity_score
    Threshold    float64
    Comparison   string    // gt, lt, eq
    WindowSize   int       // 时间窗口（分钟）
    
    AlertLevel   string    // warning, critical
    NotifyChannel string
}
```

### 2.5 缺少训练任务观测

**问题描述**：
- 当前设计只覆盖推理观测
- 训练任务的 loss、gradient、checkpoint 等指标缺失
- 无法监控训练进度和健康状态

**影响**：
- 训练任务失败无法及时发现
- 训练效率无法评估
- Checkpoint 管理缺乏观测支撑

**改进建议**：

```sql
-- 训练任务表
CREATE TABLE training_jobs (
    id CHAR(36) PRIMARY KEY,
    job_name VARCHAR(255) NOT NULL,
    
    -- 模型信息
    model_name VARCHAR(255) NOT NULL,
    model_version VARCHAR(50),
    framework VARCHAR(50) COMMENT 'pytorch, tensorflow, jax',
    
    -- 训练配置
    config JSON COMMENT 'learning_rate, batch_size, epochs 等',
    dataset VARCHAR(255),
    
    -- 资源配置
    gpu_count INT,
    node_count INT,
    
    -- 进度
    current_epoch INT DEFAULT 0,
    current_step INT DEFAULT 0,
    total_steps INT,
    progress_pct DECIMAL(5,2) DEFAULT 0,
    
    -- 状态
    status VARCHAR(20) NOT NULL COMMENT 'pending, running, completed, failed',
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT,
    
    -- 成本
    estimated_cost_usd DECIMAL(20,6),
    actual_cost_usd DECIMAL(20,6),
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 训练指标快照
CREATE TABLE training_metrics (
    id CHAR(36) PRIMARY KEY,
    training_job_id CHAR(36) NOT NULL,
    
    -- 步骤信息
    epoch INT NOT NULL,
    step INT NOT NULL,
    
    -- Loss 指标
    loss DECIMAL(20,10),
    eval_loss DECIMAL(20,10),
    
    -- 学习率
    learning_rate DECIMAL(20,10),
    
    -- 梯度指标
    grad_norm DECIMAL(20,10),
    
    -- 性能指标
    samples_per_second DECIMAL(10,2),
    tokens_per_second DECIMAL(10,2),
    
    -- GPU 指标
    avg_gpu_util DECIMAL(5,2),
    avg_memory_util DECIMAL(5,2),
    mfu_percent DECIMAL(5,2),
    
    -- 时间戳
    recorded_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (training_job_id) REFERENCES training_jobs(id) ON DELETE CASCADE
);

-- Checkpoint 管理
CREATE TABLE training_checkpoints (
    id CHAR(36) PRIMARY KEY,
    training_job_id CHAR(36) NOT NULL,
    
    checkpoint_path VARCHAR(500) NOT NULL,
    step INT NOT NULL,
    epoch INT NOT NULL,
    
    -- 指标快照
    loss DECIMAL(20,10),
    eval_loss DECIMAL(20,10),
    
    -- 存储
    size_mb INT,
    storage_path VARCHAR(500),
    
    -- 元数据
    is_best BOOLEAN DEFAULT false,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (training_job_id) REFERENCES training_jobs(id) ON DELETE CASCADE
);
```

### 2.6 缺少 GPU 共享/虚拟化观测

**问题描述**：
- MIG (Multi-Instance GPU) 场景下的资源隔离观测缺失
- 多租户 GPU 使用归因不清晰
- GPU 时间片调度监控缺失

**影响**：
- GPU 共享场景下无法准确归因资源使用
- 租户间资源争抢无法检测

**改进建议**：

```go
// domain/gpu_virtualization.go
type MIGInstance struct {
    ID           string
    GPUNodeID    string
    
    Profile      string    // MIG profile: 1g.10gb, 2g.20gb, etc.
    MemoryMB     int       // 分配的显存
    
    // 归属
    TenantID     string
    ServiceID    string
    
    // 状态
    Status       string    // active, idle, error
    
    CreatedAt    time.Time
}

type GPUSharingMode string

const (
    GPUSharingNone      GPUSharingMode = "none"
    GPUSharingMIG       GPUSharingMode = "mig"
    GPUSharingTimeSlice GPUSharingMode = "time_slice"
    GPUSharingMPS       GPUSharingMode = "mps"
)

type GPUSharingMetrics struct {
    GPUNodeID      string
    SharingMode    GPUSharingMode
    
    // 时间片模式
    TimeSliceQuota map[string]int  // tenant_id -> quota_ms
    
    // MIG 模式
    MIGInstances   []MIGInstance
    
    // 争抢指标
    WaitQueueDepth int
    AvgWaitTimeMs  int
    PreemptionCount int
}
```

### 2.7 缺少模型版本变更观测

**问题描述**：
- 模型版本切换没有完整的 trace 记录
- 无法追溯版本变更对性能的影响
- 金丝雀发布缺乏观测支撑

**影响**：
- 版本回滚决策缺乏数据
- 发布风险无法量化

**改进建议**：

```go
// domain/model_lifecycle.go
type ModelDeploymentEvent struct {
    ID              string
    InferenceServiceID string
    
    EventType       string    // deploy, promote, rollback, canary_start, canary_complete
    FromVersion     string
    ToVersion       string
    
    // 发布策略
    Strategy        string    // rolling, canary, blue_green
    CanaryWeight    int       // 金丝雀流量权重
    
    // 触发信息
    TriggeredBy     string
    TriggerReason   string
    
    // 性能对比
    BeforeMetrics   *InferenceMetrics
    AfterMetrics    *InferenceMetrics
    
    // 状态
    Status          string    // in_progress, completed, rolled_back
    CreatedAt       time.Time
    CompletedAt     time.Time
}

type CanaryProgress struct {
    DeploymentID    string
    CurrentWeight   int
    TargetWeight    int
    
    // 性能指标
    ErrorRate       float64
    P99LatencyMs    int
    Throughput      float64
    
    // 决策
    AutoPromote     bool
    AutoRollback    bool
    RollbackReason  string
}
```

---

## 3. Agent 智能诊断方向设计缺陷

### 3.1 缺少 Agent 行为审计与可解释性

**问题描述**：
- Agent 决策过程不可追溯
- 缺少 Agent 行为评分机制
- 诊断结论缺乏推理链路记录

**影响**：
- Agent 误判无法复盘
- 用户对 AI 诊断缺乏信任
- 合规审计困难

**改进建议**：

```sql
-- Agent 决策审计表
CREATE TABLE agent_decisions (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36),
    agent_id VARCHAR(255) NOT NULL,
    
    -- 决策上下文
    decision_type VARCHAR(50) NOT NULL COMMENT 'diagnosis, recommendation, action',
    input_context JSON COMMENT '输入上下文（告警、指标等）',
    
    -- 推理过程
    reasoning_steps JSON COMMENT '推理步骤链',
    tools_called JSON COMMENT '调用的工具列表',
    evidence_collected JSON COMMENT '收集的证据',
    
    -- 决策结果
    conclusion TEXT NOT NULL,
    confidence_score DECIMAL(5,2) COMMENT '置信度 0-100',
    alternative_conclusions JSON COMMENT '备选结论',
    
    -- 评分
    user_rating INT COMMENT '用户评分 1-5',
    expert_rating INT COMMENT '专家评分 1-5',
    is_correct BOOLEAN COMMENT '是否正确（事后验证）',
    
    -- 时间
    decision_at DATETIME NOT NULL,
    duration_ms INT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id)
);

CREATE INDEX idx_agent_decisions_agent ON agent_decisions(agent_id);
CREATE INDEX idx_agent_decisions_type ON agent_decisions(decision_type);
CREATE INDEX idx_agent_decisions_time ON agent_decisions(decision_at DESC);
```

```go
// domain/agent_audit.go
type ReasoningStep struct {
    StepNumber  int                    `json:"step_number"`
    Description string                 `json:"description"`
    Input       map[string]interface{} `json:"input"`
    Output      map[string]interface{} `json:"output"`
    ToolUsed    string                 `json:"tool_used,omitempty"`
    DurationMs  int                    `json:"duration_ms"`
}

type AgentDecision struct {
    ID            string
    SessionID     string
    AgentID       string
    
    DecisionType  string
    InputContext  map[string]interface{}
    
    // 推理链
    ReasoningSteps []ReasoningStep
    ToolsCalled    []ToolCallRecord
    Evidence       []EvidenceItem
    
    // 结论
    Conclusion           string
    ConfidenceScore      float64
    AlternativeConclusions []string
    
    // 评分
    UserRating    *int
    ExpertRating  *int
    IsCorrect     *bool
    
    DecisionAt    time.Time
    DurationMs    int
}

type AgentPerformanceMetrics struct {
    AgentID string
    
    // 准确率
    DiagnosisAccuracy    float64  // 诊断准确率
    RecommendationAcceptRate float64  // 建议采纳率
    
    // 效率
    AvgDecisionTimeMs    int
    AvgToolsPerDecision  float64
    
    // 用户满意度
    AvgUserRating        float64
    PositiveFeedbackRate float64
}
```

### 3.2 缺少 Agent 工具调用链路追踪

**问题描述**：
- 工具调用没有完整的 trace
- 无法分析工具调用效率
- 工具依赖关系不清晰

**影响**：
- 工具性能瓶颈难以定位
- 工具调用失败无法追溯

**改进建议**：

```go
// domain/tool_trace.go
type ToolCallTrace struct {
    TraceID       string
    ParentSpanID  string
    SpanID        string
    
    // 工具信息
    ToolName      string
    ToolType      string
    AgentID       string
    
    // 调用链
    CallDepth     int       // 调用深度
    CallSequence  int       // 调用序号
    
    // 参数与结果
    Arguments     map[string]interface{}
    Result        map[string]interface{}
    
    // 性能
    StartTime     time.Time
    EndTime       time.Time
    DurationMs    int
    
    // 状态
    Status        string    // success, error, timeout
    ErrorType     string
    ErrorMessage  string
    
    // 依赖
    Dependencies  []string  // 依赖的其他工具调用
}

type ToolCallGraph struct {
    RootCalls     []*ToolCallTrace
    AllCalls      map[string]*ToolCallTrace
    TotalDuration int
    CriticalPath  []string  // 关键路径上的工具调用
}
```

### 3.3 缺少 Agent 知识库管理

**问题描述**：
- 没有 RAG 知识库版本管理
- 知识库更新缺乏审计
- 知识检索效果无法评估

**影响**：
- 知识库变更影响无法追溯
- 检索质量无法量化

**改进建议**：

```sql
-- 知识库管理
CREATE TABLE knowledge_bases (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- 版本
    current_version INT DEFAULT 1,
    
    -- 配置
    embedding_model VARCHAR(255),
    chunk_size INT,
    chunk_overlap INT,
    
    -- 统计
    document_count INT DEFAULT 0,
    chunk_count INT DEFAULT 0,
    
    status VARCHAR(20) DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 知识库版本
CREATE TABLE knowledge_base_versions (
    id CHAR(36) PRIMARY KEY,
    knowledge_base_id CHAR(36) NOT NULL,
    version INT NOT NULL,
    
    -- 变更信息
    change_type VARCHAR(20) NOT NULL COMMENT 'create, update, delete',
    documents_added INT DEFAULT 0,
    documents_removed INT DEFAULT 0,
    documents_updated INT DEFAULT 0,
    
    -- 触发者
    changed_by VARCHAR(255),
    change_reason TEXT,
    
    -- 存储
    vector_store_snapshot VARCHAR(500),
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id),
    UNIQUE KEY uk_kb_version (knowledge_base_id, version)
);

-- 知识检索审计
CREATE TABLE knowledge_retrieval_logs (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36),
    knowledge_base_id CHAR(36),
    
    -- 查询
    query_text TEXT NOT NULL,
    query_embedding VECTOR(1536),  -- 假设使用 OpenAI embedding
    
    -- 检索结果
    retrieved_chunks JSON COMMENT '检索到的 chunk 列表',
    retrieval_scores JSON COMMENT '相似度分数',
    
    -- 效果评估
    relevance_rating INT COMMENT '相关性评分 1-5',
    used_in_response BOOLEAN COMMENT '是否用于生成回答',
    
    -- 性能
    retrieval_time_ms INT,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id)
);
```

### 3.4 缺少 Agent 协作观测

**问题描述**：
- 多 Agent 协作场景缺失
- Agent 间通信没有 trace
- 协作效率无法评估

**影响**：
- 复杂诊断场景无法支持
- Agent 协作瓶颈难以发现

**改进建议**：

```go
// domain/agent_collaboration.go
type AgentRole string

const (
    AgentRoleCoordinator AgentRole = "coordinator"
    AgentRoleDiagnoser   AgentRole = "diagnoser"
    AgentRoleInvestigator AgentRole = "investigator"
    AgentRoleRecommender AgentRole = "recommender"
)

type AgentTeam struct {
    ID          string
    Name        string
    
    // 成员
    Agents      []AgentMember
    
    // 协作模式
    Mode        string    // sequential, parallel, hierarchical
    
    CreatedAt   time.Time
}

type AgentMember struct {
    AgentID     string
    Role        AgentRole
    Capabilities []string
    
    // 优先级
    Priority    int
}

type AgentCommunication struct {
    ID           string
    SessionID    string
    
    FromAgentID  string
    ToAgentID    string
    
    // 消息
    MessageType  string    // request, response, broadcast
    Content      map[string]interface{}
    
    // 上下文
    ContextID    string    // 共享上下文 ID
    
    // 性能
    SentAt       time.Time
    ReceivedAt   time.Time
    LatencyMs    int
    
    CreatedAt    time.Time
}

type CollaborationMetrics struct {
    TeamID       string
    
    // 效率
    AvgResolutionTime int
    AvgMessagesPerSession int
    
    // 质量
    SuccessRate  float64
    AvgUserRating float64
    
    // 协作
    AvgAgentsInvolved float64
    AvgHandoffsPerSession float64
}
```

### 3.5 缺少 Agent 性能基准与评测

**问题描述**：
- 没有诊断准确率评估机制
- 缺少响应时间 SLA
- 无法对比不同 Agent 版本的性能

**影响**：
- Agent 质量无法量化
- 优化方向不明确

**改进建议**：

```sql
-- Agent 评测任务
CREATE TABLE agent_evaluations (
    id CHAR(36) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    agent_version VARCHAR(50),
    
    -- 评测配置
    eval_dataset VARCHAR(255) NOT NULL COMMENT '评测数据集',
    eval_config JSON COMMENT '评测配置',
    
    -- 评测指标
    accuracy DECIMAL(5,2) COMMENT '准确率',
    precision DECIMAL(5,2) COMMENT '精确率',
    recall DECIMAL(5,2) COMMENT '召回率',
    f1_score DECIMAL(5,2) COMMENT 'F1 分数',
    
    -- 性能指标
    avg_response_time_ms INT,
    p99_response_time_ms INT,
    avg_tokens_per_request INT,
    
    -- 成本
    total_cost_usd DECIMAL(10,4),
    
    -- 元数据
    evaluated_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 评测数据集
CREATE TABLE evaluation_datasets (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- 数据
    cases JSON NOT NULL COMMENT '评测用例',
    ground_truth JSON COMMENT '标准答案',
    
    -- 统计
    case_count INT,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

```go
// domain/agent_evaluation.go
type EvaluationCase struct {
    ID           string
    DatasetID    string
    
    // 输入
    InputContext map[string]interface{}
    AlertContext *AlertContext
    
    // 期望输出
    ExpectedDiagnosis string
    ExpectedActions   []string
    
    // 实际输出
    ActualDiagnosis   string
    ActualActions     []string
    
    // 评分
    IsCorrect         bool
    PartialScore      float64
    Feedback          string
}

type AgentBenchmark struct {
    AgentID       string
    Version       string
    
    // 准确性
    DiagnosisAccuracy float64
    ActionPrecision   float64
    ActionRecall      float64
    
    // 效率
    AvgResponseTimeMs int
    AvgToolCalls      float64
    
    // 成本
    AvgCostPerRequest float64
    
    // 对比
    BaselineComparison *BenchmarkComparison
}
```

### 3.6 缺少 Agent 安全沙箱观测

**问题描述**：
- 工具执行没有资源限制监控
- 缺少异常行为检测
- 敏感操作缺乏审计

**影响**：
- Agent 可能执行危险操作
- 资源滥用无法检测

**改进建议**：

```go
// domain/agent_sandbox.go
type SandboxPolicy struct {
    ID          string
    AgentID     string
    
    // 资源限制
    MaxExecutionTimeMs  int
    MaxMemoryMB         int
    MaxNetworkCalls     int
    MaxDataSizeMB       int
    
    // 工具限制
    AllowedTools        []string
    BlockedTools        []string
    ToolRateLimits      map[string]int  // tool -> calls per minute
    
    // 数据限制
    AllowedDataSources  []string
    BlockedDataPatterns []string  // 敏感数据模式
    
    // 操作限制
    RequireApproval     []string  // 需要审批的操作
    AutoBlock           []string  // 自动阻止的操作
}

type SandboxViolation struct {
    ID          string
    AgentID     string
    SessionID   string
    
    ViolationType string    // timeout, memory, tool, data, operation
    Severity      string    // warning, critical
    
    Details       map[string]interface{}
    
    Action        string    // blocked, logged, escalated
    ActionTaken   string
    
    DetectedAt    time.Time
}

type SandboxMetrics struct {
    AgentID string
    
    // 资源使用
    AvgExecutionTimeMs int
    MaxExecutionTimeMs int
    AvgMemoryUsageMB   int
    
    // 违规统计
    ViolationCount     int
    BlockedActionCount int
    
    // 审批统计
    ApprovalRequestCount int
    ApprovalRate        float64
}
```

### 3.7 缺少 Agent 会话上下文管理

**问题描述**：
- 长会话上下文丢失
- 缺少会话恢复机制
- 上下文压缩策略缺失

**影响**：
- 复杂诊断场景上下文断裂
- 会话中断后无法恢复

**改进建议**：

```go
// domain/session_context.go
type SessionContext struct {
    ID          string
    SessionID   string
    
    // 上下文层级
    Level       int       // 上下文层级
    
    // 内容
    Messages    []ContextMessage
    State       map[string]interface{}
    
    // 压缩
    IsCompressed bool
    Summary      string    // 压缩后的摘要
    
    // 持久化
    CheckpointedAt time.Time
    RestoredFrom   string    // 恢复来源
    
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type ContextMessage struct {
    Role        string    // user, assistant, system, tool
    Content     string
    TokenCount  int
    
    // 元数据
    ToolCalls   []ToolCall
    References  []string  // 引用的证据/文档
    
    CreatedAt   time.Time
}

type ContextCompressionStrategy struct {
    Name        string
    
    // 触发条件
    MaxTokens   int
    MaxMessages int
    
    // 压缩策略
    Strategy    string    // summary, sliding_window, hierarchical
    
    // 保留规则
    KeepRecentN int       // 保留最近 N 条
    KeepImportant bool    // 保留重要消息
}

func (s *ContextCompressionStrategy) Compress(ctx *SessionContext) (*SessionContext, error) {
    switch s.Strategy {
    case "summary":
        return s.compressBySummary(ctx)
    case "sliding_window":
        return s.compressBySlidingWindow(ctx)
    case "hierarchical":
        return s.compressByHierarchy(ctx)
    }
    return nil, fmt.Errorf("unknown strategy: %s", s.Strategy)
}
```

---

## 4. 遗漏的设计缺陷补充

### 4.1 AI Infra 方向遗漏缺陷

#### 4.1.1 缺少推理阶段分解观测

**问题描述**：
- 当前 `InferenceRequest` 只有 `ttft_ms`、`tpot_ms` 等聚合指标
- 缺少 **Prefill**（预填充）和 **Decode**（解码）阶段的细粒度观测
- 无法区分计算密集型（长 prompt）和生成密集型（长输出）的性能瓶颈

**影响**：
- 无法针对性优化 prefill 或 decode 阶段
- 批处理策略调优缺乏数据支撑

**改进建议**：

```go
// domain/inference_stages.go
type InferenceStageMetrics struct {
    RequestID string
    
    // Prefill 阶段
    PrefillDurationMs    int     // Prefill 耗时
    PrefillTokens        int     // Prefill 处理的 Token 数
    PrefillTokensPerSec  float64 // Prefill 吞吐量
    
    // Decode 阶段
    DecodeDurationMs     int     // Decode 总耗时
    DecodeTokens         int     // Decode 生成的 Token 数
    DecodeTokensPerSec   float64 // Decode 吞吐量
    
    // KV Cache
    KVCacheBlocksUsed    int     // KV Cache 块使用量
    KVCacheBlockHits     int     // KV Cache 命中次数
    KVCacheBlockMisses   int     // KV Cache 未命中次数
    
    // 批处理
    BatchSize            int     // 实际批大小
    BatchEfficiency      float64 // 批处理效率
}

type InferenceStageBreakdown struct {
    RequestID string
    
    // 时间分解
    QueueWaitMs       int  // 队列等待时间
    TokenizationMs    int  // Tokenization 耗时
    PrefillMs         int  // Prefill 耗时
    DecodeMs          int  // Decode 耗时
    DetokenizationMs  int  // Detokenization 耗时
    NetworkMs         int  // 网络传输时间
    
    // 总计
    TotalMs           int  // 总耗时
}
```

#### 4.1.2 缺少 GPU 与服务关联拓扑

**问题描述**：
- `gpu_nodes` 表有 `k8s_pod_name` 字段，但缺少动态关联机制
- GPU 与推理服务的映射关系不清晰
- 多实例 GPU (MIG) 场景下的资源归因缺失

**影响**：
- 无法快速定位哪个服务占用了 GPU 资源
- GPU 资源利用率无法归因到具体业务

**改进建议**：

```sql
-- GPU 与服务关联表
CREATE TABLE gpu_service_bindings (
    id CHAR(36) PRIMARY KEY,
    gpu_node_id CHAR(36) NOT NULL,
    service_id CHAR(36) NOT NULL,
    
    -- 绑定类型
    binding_type VARCHAR(20) NOT NULL COMMENT 'exclusive, shared, mig_slice',
    
    -- MIG 切片信息
    mig_slice_index INT,
    mig_profile VARCHAR(50),
    
    -- 资源配额
    gpu_memory_quota_mb INT,
    compute_quota_pct DECIMAL(5,2),
    
    -- 时间范围
    bound_at DATETIME NOT NULL,
    released_at DATETIME,
    
    -- 状态
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    
    FOREIGN KEY (gpu_node_id) REFERENCES gpu_nodes(id),
    FOREIGN KEY (service_id) REFERENCES services(id),
    INDEX idx_gpu_service_binding (gpu_node_id, service_id, status)
);
```

#### 4.1.3 缺少模型权重加载观测

**问题描述**：
- 模型加载时间（冷启动）没有观测
- 权重加载过程中的内存/显存变化缺失
- 多副本场景下的权重共享观测缺失

**影响**：
- 冷启动延迟无法优化
- 模型部署策略缺乏数据支撑

**改进建议**：

```go
// domain/model_loading.go
type ModelLoadingMetrics struct {
    ID              string
    InferenceServiceID string
    ModelVersionID  string
    
    // 加载阶段
    Stage           string    // downloading, loading_weights, warming_up, ready
    
    // 时间指标
    DownloadMs      int       // 模型下载时间
    LoadWeightsMs   int       // 权重加载时间
    WarmupMs        int       // 预热时间
    TotalMs         int       // 总加载时间
    
    // 资源指标
    PeakMemoryMB    int       // 峰值内存使用
    PeakGPUMemoryMB int       // 峰值显存使用
    ModelSizeMB     int       // 模型大小
    
    // 缓存
    CacheHit        bool      // 是否命中缓存
    CacheSource     string    // 缓存来源
    
    // 状态
    Status          string    // success, failed, timeout
    ErrorMessage    string
    
    StartedAt       time.Time
    CompletedAt     time.Time
}
```

#### 4.1.4 缺少 Token 拒绝与截断观测

**问题描述**：
- Token 超限被拒绝的请求没有统计
- 输出被截断的情况没有记录
- 无法评估 Token 限制对业务的影响

**影响**：
- Token 配额设置缺乏依据
- 用户请求失败原因不清晰

**改进建议**：

```go
// domain/token_limits.go
type TokenLimitEvent struct {
    ID          string
    SessionID   string
    ServiceID   string
    
    // 限制类型
    LimitType   string    // input_exceeded, output_exceeded, budget_exceeded, rate_limited
    
    // Token 信息
    RequestedTokens int
    AllowedTokens   int
    ExceededTokens  int
    
    // 处理方式
    Action      string    // rejected, truncated, queued
    
    // 上下文
    UserID      string
    ModelName   string
    
    CreatedAt   time.Time
}

type TokenTruncationMetrics struct {
    ServiceID       string
    
    // 截断统计
    TotalTruncated  int       // 被截断的请求数
    AvgTruncatedTokens int    // 平均截断 Token 数
    
    // 拒绝统计
    TotalRejected   int       // 被拒绝的请求数
    RejectionReasons map[string]int // 拒绝原因分布
    
    // 时间窗口
    WindowStart     time.Time
    WindowEnd       time.Time
}
```

### 4.2 Agent 方向遗漏缺陷

#### 4.2.1 缺少 Agent 提示词版本管理

**问题描述**：
- `prompt_templates` 表存在，但与 Agent 行为的关联缺失
- 提示词变更对 Agent 行为的影响无法追溯
- 缺少 A/B 测试能力

**影响**：
- 提示词优化效果无法量化
- 提示词回滚缺乏依据

**改进建议**：

```sql
-- Agent 提示词版本关联
CREATE TABLE agent_prompt_versions (
    id CHAR(36) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    prompt_template_id CHAR(36) NOT NULL,
    prompt_version INT NOT NULL,
    
    -- 变更信息
    changed_by VARCHAR(255),
    change_reason TEXT,
    
    -- 效果指标（事后更新）
    avg_response_time_ms INT,
    avg_user_rating DECIMAL(3,2),
    success_rate DECIMAL(5,2),
    
    -- 状态
    is_active BOOLEAN DEFAULT true,
    activated_at DATETIME,
    deactivated_at DATETIME,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (prompt_template_id) REFERENCES prompt_templates(id),
    UNIQUE KEY uk_agent_prompt_version (agent_id, prompt_version)
);

-- A/B 测试配置
CREATE TABLE agent_ab_tests (
    id CHAR(36) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    
    -- 测试配置
    control_prompt_version INT NOT NULL,
    treatment_prompt_version INT NOT NULL,
    traffic_split_pct INT NOT NULL DEFAULT 50,
    
    -- 指标
    primary_metric VARCHAR(50) NOT NULL COMMENT 'accuracy, latency, user_rating',
    
    -- 状态
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    
    -- 结果
    winner VARCHAR(20) COMMENT 'control, treatment, inconclusive',
    confidence_level DECIMAL(5,2),
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 4.2.2 缺少 Agent 错误恢复机制

**问题描述**：
- Agent 执行失败后没有重试/恢复机制
- 部分成功的诊断结果无法保存
- 长时间运行的诊断任务缺乏检查点

**影响**：
- 网络抖动导致诊断失败
- 复杂诊断任务无法断点续作

**改进建议**：

```go
// domain/agent_recovery.go
type AgentCheckpoint struct {
    ID          string
    SessionID   string
    AgentID     string
    
    // 检查点状态
    StepIndex   int       // 当前步骤索引
    StepName    string    // 当前步骤名称
    StepStatus  string    // pending, running, completed, failed
    
    // 中间结果
    CollectedEvidence map[string]interface{}
    IntermediateResults map[string]interface{}
    
    // 恢复信息
    CanResume   bool
    ResumeFrom  string    // 恢复起始点
    
    // 时间
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type AgentRecoveryPolicy struct {
    AgentID     string
    
    // 重试配置
    MaxRetries      int
    RetryDelayMs    int
    RetryBackoff    float64  // 指数退避系数
    
    // 超时配置
    StepTimeoutMs   int
    TotalTimeoutMs  int
    
    // 恢复策略
    RecoveryMode    string    // restart, resume, fallback
    FallbackAgent   string    // 降级 Agent
}

type AgentExecutionLog struct {
    ID          string
    SessionID   string
    AgentID     string
    
    // 执行信息
    StepName    string
    ActionType  string
    
    // 状态
    Status      string    // started, completed, failed, retried
    Attempt     int       // 尝试次数
    
    // 错误信息
    ErrorType   string
    ErrorMessage string
    
    // 时间
    StartedAt   time.Time
    CompletedAt time.Time
    DurationMs  int
}
```

#### 4.2.3 缺少 Agent 资源消耗观测

**问题描述**：
- Agent 执行过程中的资源消耗没有记录
- LLM 调用成本与诊断结果的关联缺失
- 无法评估 Agent 的成本效益

**影响**：
- Agent 成本无法控制
- 高成本诊断无法识别

**改进建议**：

```go
// domain/agent_resources.go
type AgentResourceUsage struct {
    ID          string
    SessionID   string
    AgentID     string
    
    // LLM 资源
    TotalPromptTokens    int
    TotalCompletionTokens int
    TotalTokens          int
    LLMCostUSD           float64
    
    // 调用统计
    LLMCallCount         int
    ToolCallCount        int
    RetryCount           int
    
    // 时间资源
    TotalDurationMs      int
    LLMWaitMs            int
    ToolWaitMs           int
    
    // 计算资源（如果可用）
    PeakMemoryMB         int
    AvgCPUUtilization    float64
    
    CreatedAt            time.Time
}

type AgentCostEfficiency struct {
    AgentID     string
    
    // 成本指标
    AvgCostPerDiagnosis  float64
    AvgCostPerCorrectDiagnosis float64
    
    // 效率指标
    DiagnosisSuccessRate float64
    AvgTimeToDiagnosis   int
    
    // ROI
    CostSavingsUSD       float64  // 相比人工诊断节省的成本
    ROI                  float64  // 投资回报率
}
```

#### 4.2.4 缺少 Agent 学习反馈闭环

**问题描述**：
- 用户反馈没有系统化收集
- 专家修正结果没有用于 Agent 改进
- 缺少持续学习机制

**影响**：
- Agent 无法从错误中学习
- 诊断准确率提升缓慢

**改进建议**：

```sql
-- Agent 反馈收集
CREATE TABLE agent_feedback (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    
    -- 反馈类型
    feedback_type VARCHAR(50) NOT NULL COMMENT 'rating, correction, rejection',
    
    -- 评分反馈
    rating INT COMMENT '1-5 评分',
    rating_dimensions JSON COMMENT 'accuracy, helpfulness, timeliness 等',
    
    -- 修正反馈
    correct_diagnosis TEXT COMMENT '正确诊断（专家填写）',
    correct_actions JSON COMMENT '正确操作（专家填写）',
    correction_reason TEXT,
    
    -- 拒绝反馈
    rejection_reason TEXT,
    
    -- 元数据
    feedback_source VARCHAR(50) COMMENT 'user, expert, automated',
    feedback_by VARCHAR(255),
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id)
);

-- Agent 学习记录
CREATE TABLE agent_learning_events (
    id CHAR(36) PRIMARY KEY,
    agent_id VARCHAR(255) NOT NULL,
    
    -- 学习来源
    source_type VARCHAR(50) NOT NULL COMMENT 'feedback, correction, new_pattern',
    source_id CHAR(36) NOT NULL,
    
    -- 学习内容
    learned_pattern JSON COMMENT '学习到的模式',
    updated_rules JSON COMMENT '更新的规则',
    
    -- 效果验证
    validation_accuracy DECIMAL(5,2),
    validation_samples INT,
    
    -- 状态
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    applied_at DATETIME,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### 4.2.5 缺少 Agent 多模态诊断能力

**问题描述**：
- 当前设计只支持文本诊断
- 图表、拓扑图等视觉信息无法处理
- 缺少多模态证据收集能力

**影响**：
- 复杂故障场景（如拓扑异常）诊断能力受限
- 无法利用 Grafana 截图等视觉证据

**改进建议**：

```go
// domain/multimodal_evidence.go
type EvidenceType string

const (
    EvidenceTypeText       EvidenceType = "text"
    EvidenceTypeImage      EvidenceType = "image"
    EvidenceTypeTimeSeries EvidenceType = "time_series"
    EvidenceTypeTopology   EvidenceType = "topology"
    EvidenceTypeLog        EvidenceType = "log"
    EvidenceTypeTrace      EvidenceType = "trace"
)

type MultimodalEvidence struct {
    ID          string
    SessionID   string
    
    // 证据类型
    Type        EvidenceType
    
    // 内容
    TextContent string              // 文本内容
    ImageURL    string              // 图片 URL
    ImageBase64 string              // 图片 Base64
    StructuredData map[string]interface{} // 结构化数据
    
    // 元数据
    Source      string    // 来源（grafana, prometheus, loki）
    CapturedAt  time.Time
    
    // 相关性
    RelevanceScore float64  // 与诊断的相关性评分
    IsKeyEvidence  bool     // 是否关键证据
}

type MultimodalDiagnosisInput struct {
    SessionID   string
    
    // 多模态输入
    TextInputs  []string
    Images      []MultimodalEvidence
    TimeSeries  []MultimodalEvidence
    Topology    *MultimodalEvidence
    
    // 上下文
    AlertContext map[string]interface{}
    ServiceContext map[string]interface{}
}
```

---

## 5. 优先级排序

### 5.1 AI Infra 观测方向

| 缺陷 | 影响 | 实现成本 | 优先级 |
|------|------|----------|--------|
| MFU 指标观测 | 无法评估真实计算效率 | 中 | **P0** |
| 训练任务观测 | 训练场景完全缺失 | 高 | **P0** |
| 推理阶段分解观测 | 性能瓶颈定位不准 | 中 | **P0** |
| GPU 与服务关联拓扑 | 资源归因不清晰 | 中 | **P1** |
| 推理质量监控 | 输出质量无法监控 | 中 | **P1** |
| 模型基准测试 | 优化效果无法量化 | 中 | **P1** |
| 模型权重加载观测 | 冷启动优化缺失 | 低 | **P1** |
| Token 拒绝与截断观测 | 配额设置缺乏依据 | 低 | **P2** |
| GPU 共享观测 | 多租户场景缺失 | 中 | **P2** |
| 模型版本变更观测 | 发布风险无法评估 | 低 | **P2** |
| 多模态 Token 计费 | 成本核算不准 | 低 | **P3** |

### 5.2 Agent 智能诊断方向

| 缺陷 | 影响 | 实现成本 | 优先级 |
|------|------|----------|--------|
| Agent 行为审计 | 决策不可追溯 | 中 | **P0** |
| Agent 性能基准 | 质量无法量化 | 中 | **P0** |
| Agent 资源消耗观测 | 成本无法控制 | 低 | **P0** |
| Agent 错误恢复机制 | 诊断失败无法恢复 | 中 | **P1** |
| 工具调用链路追踪 | 性能瓶颈难定位 | 低 | **P1** |
| Agent 安全沙箱 | 安全风险 | 中 | **P1** |
| 会话上下文管理 | 长会话断裂 | 中 | **P1** |
| Agent 提示词版本管理 | 优化效果无法量化 | 低 | **P1** |
| Agent 学习反馈闭环 | 无法持续改进 | 中 | **P2** |
| Agent 协作观测 | 复杂场景缺失 | 高 | **P2** |
| 知识库管理 | RAG 效果难评估 | 高 | **P2** |
| Agent 多模态诊断 | 复杂故障诊断受限 | 高 | **P3** |

---

## 6. 实施建议

### 6.1 短期（M4）

**AI Infra**：
1. 实现 MFU 计算逻辑
2. 补充训练任务观测 Schema
3. 实现推理阶段分解观测（Prefill/Decode）
4. 实现 GPU 与服务关联拓扑
5. 实现推理质量基础指标

**Agent**：
1. 实现 Agent 决策审计
2. 建立性能评测框架
3. 实现工具调用 trace
4. 实现 Agent 资源消耗观测
5. 实现基础错误恢复机制

### 6.2 中期（M5）

**AI Infra**：
1. 完善模型基准测试
2. 实现 GPU 共享观测
3. 模型版本变更追踪
4. 模型权重加载观测
5. Token 拒绝与截断观测

**Agent**：
1. 安全沙箱机制
2. 会话上下文管理
3. Agent 协作基础能力
4. 提示词版本管理与 A/B 测试
5. 学习反馈闭环基础能力

### 6.3 长期（M5+）

**AI Infra**：
1. 多模态 Token 计费
2. 高级质量监控（幻觉检测）
3. 训练任务高级观测（梯度、Checkpoint）

**Agent**：
1. 知识库版本管理
2. 多 Agent 协作优化
3. 多模态诊断能力

---

## 7. 关键设计决策建议

### 7.1 AI Infra 方向

| 决策点 | 建议 | 理由 |
|--------|------|------|
| MFU 计算方式 | 服务端计算 + 客户端上报 | 服务端有完整的 FLOPS 信息，客户端上报实际计算量 |
| 推理阶段分解 | 依赖 vLLM/Triton 原生指标 | 避免自行实现，利用推理引擎已有能力 |
| GPU 服务关联 | 动态绑定 + 定期同步 | K8s 环境下 Pod 调度频繁，需要动态追踪 |
| 训练观测 | 独立模块，与推理观测解耦 | 训练和推理的生命周期、指标体系差异大 |

### 7.2 Agent 方向

| 决策点 | 建议 | 理由 |
|--------|------|------|
| 行为审计粒度 | 步骤级 + 决策级 | 既要支持细粒度复盘，也要支持快速定位 |
| 错误恢复 | 检查点 + 幂等重试 | 避免重复执行副作用操作 |
| 资源消耗 | 与 AI Session 复用 | Token 统计已在 Session 中，扩展即可 |
| 多模态诊断 | 渐进式支持 | 先支持文本，再扩展到图表、拓扑 |

---

## 8. 修订记录

| 日期 | 变更 |
|------|------|
| 2026-04-10 | 补充遗漏缺陷：推理阶段分解、GPU服务关联、模型加载、Token限制、Agent提示词版本、错误恢复、资源消耗、学习反馈、多模态诊断 |
| 2026-04-10 | 初版：AI Infra 观测与 Agent 方向设计缺陷全面分析 |
