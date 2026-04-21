# Agent 模块开发计划

> 版本: v1.0 | 日期: 2026-04-18 | 状态: 待审核

## 一、项目背景与现状评估

### 1.1 现有 Agent 相关模块盘点

| 模块 | 路径 | 实际状态 |
|------|------|---------|
| **advisor** | `internal/advisor/` | 骨架占位 — 所有文件仅含 `package xxx` 声明，无业务逻辑。目录结构已规划（domain/infrastructure/interfaces/prompts），但零实现 |
| **agentcoord** | `internal/agentcoord/` | 骨架占位 — 同上，所有文件仅含 package 声明 |
| **mcp** | `internal/mcp/` | 唯一有实质实现的模块 — eino 工具注册/鉴权体系完整，3 个只读工具（topology/alerting/slo）已实现 |

### 1.2 MCP 模块已有能力（可直接复用）

| 能力 | 实现位置 | 说明 |
|------|---------|------|
| 工具注册体系 | `eino/registry.go` | `ToolRegistry` + `ReadOnlyTool` 接口，支持注册/查询/执行 |
| 权限鉴权 | `eino/registry.go` + `auth/permission.go` | `AuthzToolWrapper` + `PermissionChecker`，RBAC 四角色（viewer/editor/admin/agent） |
| Gin 中间件 | `middleware/eino_auth.go` | Bearer Token 认证 + 工具列表/执行 HTTP 端点 |
| 3 个只读工具 | `eino/topology_tool.go`, `alerting_tool.go`, `slo_tool.go` | 完整的 `InvokableTool` 实现，含参数校验和 action 分发 |
| Eino 集成 | `eino/node.go` | `compose.ToolsNode` 构建，与 eino Graph 体系对接 |

### 1.3 基础设施盘点

| 基础设施 | 状态 | 说明 |
|---------|------|------|
| asynq 任务队列 | ✅ 生产级 | `pkg/infra/queue.go`，已在 topology 模块验证 |
| freecache 本地缓存 | ✅ 生产级 | `pkg/infra/cache.go`，L1 热缓存 |
| Redis 分布式缓存 | ✅ 生产级 | `go-redis/v9`，topology 模块已用 |
| MySQL + GORM | ✅ 生产级 | 完整仓储层，含 migration 体系 |
| Wire DI | ✅ 生产级 | 编译时注入，wire_gen.go 已验证 |
| eino 框架 | ✅ 已引入 | `github.com/cloudwego/eino v0.8.5`，含 compose/tool/schema |
| 向量数据库 | ❌ 未集成 | go.mod 中无任何向量数据库依赖 |

### 1.4 关键结论

1. **advisor 和 agentcoord 是空壳** — 目录结构暗示了设计意图（fork/记忆/引擎/规则），但无任何实现代码
2. **MCP 工具层是唯一可用的基础** — 但它只解决了"工具调用"这一个维度
3. **5 大需求全部需要从零构建** — 技能系统、RAG、上下文工程、任务处理、架构扩展均无现有代码可复用

---

## 二、模块重命名方案：MCP → Agent

### 2.1 重命名理由

当前 `internal/mcp/` 模块的实际职责已超出 MCP（Model Context Protocol）协议范畴：
- 包含了工具注册、权限鉴权、HTTP 中间件等 Agent 通用能力
- 新增的技能系统、上下文管理、任务处理等均属于 Agent 能力层
- 统一命名为 `agent` 更准确反映模块定位，避免概念混淆

### 2.2 重命名映射

| 原路径 | 新路径 | 说明 |
|--------|--------|------|
| `internal/mcp/` | `internal/agent/` | 模块根目录 |
| `internal/mcp/domain/` | `internal/agent/domain/` | 领域层 |
| `internal/mcp/application/` | `internal/agent/application/` | 应用层 |
| `internal/mcp/infrastructure/eino/` | `internal/agent/infrastructure/eino/` | Eino 工具集成 |
| `internal/mcp/infrastructure/auth/` | `internal/agent/infrastructure/auth/` | 权限鉴权 |
| `internal/mcp/infrastructure/middleware/` | `internal/agent/infrastructure/middleware/` | HTTP 中间件 |
| `internal/mcp/infrastructure/server/` | `internal/agent/infrastructure/server/` | MCP Server（保留） |
| `internal/mcp/interfaces/http/` | `internal/agent/interfaces/http/` | HTTP 接口 |
| `internal/advisor/` | 废弃，能力合并入 `internal/agent/` | 避免命名混乱 |
| `internal/agentcoord/` | 废弃，能力合并入 `internal/agent/` | 避免命名混乱 |

### 2.3 重命名影响范围

- 所有 import 路径从 `cloud-agent-monitor/internal/mcp/` 变更为 `cloud-agent-monitor/internal/agent/`
- `cmd/platform-api/wire.go` 中的相关 Provider 需更新 import
- `internal/advisor/` 和 `internal/agentcoord/` 的空壳目录删除
- 数据库 migration 无影响（现有表名不变）

---

## 三、技术选型

| 领域 | 选型 | 理由 |
|------|------|------|
| LLM 编排框架 | `cloudwego/eino`（已有 v0.8.5） | 项目已引入，compose.Graph 支持 DAG 编排，与现有工具层天然集成 |
| 向量数据库 | **Milvus**（`milvus-io/milvus-sdk-go/v2`） | 云原生架构，Go SDK 成熟，支持混合检索（向量+标量过滤），与 K8s 部署体系一致 |
| Embedding 模型 | OpenAI `text-embedding-3-small` / 本地 BGE-M3 | 通过 eino 的 `embedder` 接口抽象，支持多后端切换 |
| 文档解析 | `eino-contrib` 的 `loader` 组件 | 与 eino 生态一致，支持 PDF/Markdown/HTML |
| 任务调度 | `asynq`（已有） | 复用 `pkg/infra/queue.go`，任务链模式已在 topology 验证 |
| 缓存 | freecache(L1) + Redis(L2)（已有） | 复用现有双层缓存架构 |
| 持久化 | MySQL + GORM（已有） | 复用现有仓储层和 migration 体系 |

### 备选方案

| 领域 | 备选 | 适用场景 |
|------|------|---------|
| 向量数据库 | Redis Search（`FT.*` 命令） | P1 阶段轻量方案，无需额外部署，但检索能力有限 |
| 向量数据库 | Qdrant | 单机部署简单，但 Go SDK 不如 Milvus 成熟 |
| Embedding | Cohere Embed | 多语言支持好，但需外部 API 调用 |

---

## 四、模块划分与目录结构

```
internal/agent/                              ← 统一 Agent 模块（原 mcp 重命名）
├── domain/                                  ← 领域层
│   ├── tool_pool.go                         ← P0-0: 工具池领域模型（ToolPool, PoolRouter, ToolBudget）
│   ├── workflow.go                          ← P0-A: 工作流领域模型（WorkflowDef, WorkflowInstance）
│   ├── skill.go                             ← P0: 技能接口规范与领域模型
│   ├── context.go                           ← P0: 上下文管理领域模型
│   ├── task.go                              ← P0: 任务领域模型（拆解/验证/重试）
│   ├── session.go                           ← P1: 会话领域模型
│   ├── memory.go                            ← P1: 记忆领域模型（三层）
│   ├── fork.go                              ← P2: Fork 领域模型
│   ├── trace.go                             ← P0: 调用链与思考过程领域模型（OTel 对齐）
│   ├── tool.go                              ← 已有: 工具领域模型（保留）
│   ├── errors.go                            ← 已有: 错误定义（保留）
│   ├── repository.go                        ← 已有: 仓储接口（保留）
│   └── service.go                           ← 已有: 服务接口（保留）
├── application/                             ← 应用层
│   ├── skill_service.go                     ← P0: 技能注册/管理/调用
│   ├── context_service.go                   ← P0: 上下文 CRUD/压缩/清理
│   ├── task_service.go                      ← P0: 任务拆解/执行/验证/重试
│   ├── observability_service.go             ← P0: 可观测性/可视化服务（基于 OTel）
│   ├── session_service.go                   ← P1: 会话生命周期管理
│   ├── rag_service.go                       ← P1: RAG 检索增强
│   ├── agent_service.go                     ← P2: Agent 编排入口
│   └── service.go                           ← 已有: MCP 服务（保留）
├── infrastructure/                          ← 基础设施层
│   ├── eino/                                ← 已有: Eino 工具集成（保留+扩展）
│   │   ├── registry.go                      ← ToolRegistry + AuthzToolWrapper
│   │   ├── pool.go                          ← P0-0: ToolPool + PoolRegistry + IntentRouter
│   │   ├── pools_builtin.go                 ← P0-0: 内置池配置（诊断/告警/拓扑/容量/通用）
│   │   ├── node.go                          ← ToolsNode 构建（改为从池选择）
│   │   ├── topology_tool.go                 ← 拓扑查询工具（保留，兼容旧接口）
│   │   ├── topology_*.go                    ← P0-0: 拆分后的瘦工具（10 个独立工具）
│   │   ├── alerting_tool.go                 ← 告警查询工具（保留，兼容旧接口）
│   │   ├── alerting_*.go                    ← P0-0: 拆分后的瘦工具（6 个独立工具）
│   │   ├── slo_tool.go                      ← SLO 查询工具（保留，兼容旧接口）
│   │   └── slo_*.go                         ← P0-0: 拆分后的瘦工具（5 个独立工具）
│   ├── workflow/                            ← P0-A: 场景化工作流
│   │   ├── builder.go                       ← 工作流构建器（eino Graph 工厂）
│   │   ├── router.go                        ← 意图→工作流路由
│   │   ├── registry.go                      ← 工作流注册中心
│   │   └── builtin/                         ← 内置工作流定义
│   │       ├── diagnose.go                  ← 故障诊断工作流
│   │       ├── alert_analyze.go             ← 告警分析工作流
│   │       ├── capacity_check.go            ← 容量规划工作流
│   │       └── free_chat.go                 ← 自由对话工作流（ReAct 兜底）
│   ├── auth/                                ← 已有: 权限鉴权（保留）
│   │   ├── permission.go                    ← RBAC 权限体系
│   │   └── provider.go                      ← 权限提供者
│   ├── middleware/                           ← 已有: HTTP 中间件（保留）
│   │   └── eino_auth.go                     ← Bearer Token 认证
│   ├── server/                              ← 已有: MCP Server（保留）
│   │   └── server.go
│   ├── skills/                              ← P0: 技能实现
│   │   ├── registry.go                      ← 技能注册中心
│   │   ├── validator.go                     ← 参数验证器
│   │   ├── adapter.go                       ← Skill ↔ ReadOnlyTool 适配器
│   │   └── builtin/                         ← 内置技能
│   │       ├── k8s_diagnose.go              ← K8s 诊断技能
│   │       ├── alert_analyze.go             ← 告警分析技能
│   │       └── slo_check.go                 ← SLO 检查技能
│   ├── context/                             ← P0: 上下文管理
│   │   ├── manager.go                       ← 上下文创建/维护/更新
│   │   ├── qdrant_store.go                  ← P0: Qdrant 向量存储（语义检索）
│   │   ├── mysql_store.go                   ← P0: MySQL 消息持久化
│   │   ├── keyword_compressor.go            ← P0: 关键词提取压缩（优先级高于摘要）
│   │   ├── summary_compressor.go            ← P0: LLM 摘要压缩（备选策略）
│   │   └── cleaner.go                       ← 过期上下文清理（asynq 定时任务）
│   ├── task/                                ← P0: 任务处理
│   │   ├── decomposer.go                    ← 任务拆解（LLM 驱动）
│   │   ├── executor.go                      ← 任务执行器
│   │   ├── verifier.go                      ← 交付验证器
│   │   └── retry.go                         ← 失败重试策略
│   ├── rag/                                 ← P1: RAG 子系统
│   │   ├── qdrant_store.go                  ← P1: Qdrant 文档向量存储
│   │   ├── embedder.go                      ← P1: BGE-M3 Embedding 生成
│   │   ├── retriever.go                     ← P1: 混合检索器（向量+关键词）
│   │   ├── loader.go                        ← P1: 文档加载器
│   │   └── chunker.go                       ← P1: 文档分块器
│   ├── memory/                              ← P1: 三层记忆
│   │   ├── working.go                       ← 工作记忆（当前对话上下文）
│   │   ├── episodic.go                      ← 情景记忆（历史对话摘要）
│   │   └── semantic.go                      ← 语义记忆（知识库/RAG）
│   ├── fork/                                ← P2: Agent Fork
│   │   ├── builder.go                       ← Fork 构建器
│   │   └── executor.go                      ← Fork 执行器
│   ├── otel/                                 ← P0: OTel GenAI 可观测性
│   │   ├── tracer.go                        ← AgentTracer（Workflow/Agent/Task span 工厂）
│   │   ├── eino_interceptor.go              ← eino Graph 节点 span 注入
│   │   ├── span_processor.go                ← 自定义 SpanProcessor（MySQL 双写）
│   │   └── metrics.go                       ← OTel Meter 指标注册
│   └── persistence/                         ← 持久化
│       ├── context_repository.go            ← 上下文 MySQL 仓储
│       ├── task_repository.go               ← 任务 MySQL 仓储
│       ├── pool_repository.go               ← 工具池 MySQL 仓储
│       ├── trace_repository.go              ← 调用链 MySQL 仓储
│       └── thought_repository.go            ← 思考过程 MySQL 仓储
└── interfaces/                              ← 接口层
    └── http/
        ├── handler.go                       ← Agent API Handler（扩展现有）
        └── routes.go                        ← 路由注册（扩展现有）
```

---

## 五、核心接口设计

### 5.0 P0-0 — MCP Tool Pool（工具池）

> 优先级最高，作为 P0-A 技能系统的前置依赖。解决工具数量增长后 LLM 调用错误率上升的问题。

#### 5.0.1 问题分析

当前 `ToolRegistry` 是扁平注册表，所有工具平铺在一个 map 里，LLM 每次调用都能看到全部工具：

| 问题 | 说明 |
|------|------|
| Token 浪费 | 每个工具的 `ToolInfo`（含参数描述）都注入 system prompt，3 个工具尚可，30 个工具可能消耗数千 Token |
| 选择干扰 | LLM 在过多工具中容易选错，尤其是功能相近的工具 |
| 无场景感知 | 诊断场景不需要 SLO 工具，告警场景不需要拓扑工具，但当前全部暴露 |
| 单层 action 分发 | 当前每个工具内部用 `switch action` 分发，本质是把"多工具"塞进"单工具"，增加了 LLM 参数构造错误率 |

#### 5.0.2 架构设计

```
┌─────────────────────────────────────────────────┐
│                  LLM (eino Graph)                │
│         只看到当前场景相关的工具子集               │
└──────────────────────┬──────────────────────────┘
                       │ PoolRegistry.SelectTools(ctx, intent)
                       ▼
┌─────────────────────────────────────────────────┐
│              PoolRegistry (工具池注册中心)        │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐           │
│  │ Pool A  │ │ Pool B  │ │ Pool C  │  ...      │
│  │诊断场景 │ │告警场景 │ │容量场景 │           │
│  └─────────┘ └─────────┘ └─────────┘           │
│  + IntentRouter: intent → pool mapping          │
│  + ToolBudget: max tools per request            │
└──────────────────────┬──────────────────────────┘
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌──────────┐ ┌──────────┐ ┌──────────┐
    │Topology  │ │Alerting  │ │  SLO     │  ...
    │瘦工具x10 │ │瘦工具x6  │ │瘦工具x5  │
    └──────────┘ └──────────┘ └──────────┘
```

#### 5.0.3 领域模型

```go
// domain/tool_pool.go

type ToolPool struct {
    ID          string
    Name        string
    Description string
    ToolNames   []string
    Keywords    []string
    Priority    int
    MaxTools    int
}

type IntentRouter struct {
    pools        map[string]*ToolPool
    keywordIndex map[string][]string
}

type ToolBudget struct {
    MaxToolsPerRequest int
    MaxTokensForTools  int
}

type PoolRegistry struct {
    registry *ToolRegistry
    pools    map[string]*ToolPool
    router   *IntentRouter
    budget   *ToolBudget
}
```

#### 5.0.4 核心接口

```go
// PoolRegistry 核心方法

func (pr *PoolRegistry) RegisterPool(pool *ToolPool) error
func (pr *PoolRegistry) UnregisterPool(poolID string) error
func (pr *PoolRegistry) SelectTools(ctx context.Context, intent string) ([]tool.InvokableTool, error)
func (pr *PoolRegistry) ListPools(ctx context.Context) ([]*ToolPool, error)
func (pr *PoolRegistry) GetPool(poolID string) (*ToolPool, error)
func (pr *PoolRegistry) GetToolsForPool(ctx context.Context, poolID string) ([]tool.InvokableTool, error)
```

#### 5.0.5 工具拆分方案

**当前**：3 个"胖工具"，每个内部用 `switch action` 分发

**拆分后**：21 个独立"瘦工具"，每个参数精确、描述聚焦

| 原胖工具 | action 数 | 拆分后瘦工具 |
|---------|----------|-------------|
| `topology_query` | 10 | `topology_get_service_topology`, `topology_get_network_topology`, `topology_get_node`, `topology_get_upstream`, `topology_get_downstream`, `topology_analyze_impact`, `topology_find_path`, `topology_find_shortest_path`, `topology_find_anomalies`, `topology_get_stats` |
| `alerting_query` | 6 | `alerting_list_active`, `alerting_list_history`, `alerting_stats`, `alerting_noisy`, `alerting_high_risk`, `alerting_get_feedback` |
| `slo_query` | 5 | `slo_list`, `slo_get`, `slo_get_error_budget`, `slo_get_burn_rate_alerts`, `slo_get_summary` |

> 保留原胖工具文件作为兼容层，瘦工具独立文件实现 `ReadOnlyTool` 接口。

#### 5.0.6 内置池配置

| 池 ID | 名称 | 包含工具 | 关键词 | 优先级 |
|-------|------|---------|--------|--------|
| `diagnose` | 故障诊断 | topology_get_node, topology_analyze_impact, topology_find_anomalies, alerting_list_active, alerting_get_feedback, slo_get_error_budget | 诊断, 故障, 异常, down, error, unhealthy, diagnose | 10 |
| `alert` | 告警分析 | alerting_list_active, alerting_list_history, alerting_stats, alerting_noisy, alerting_high_risk, alerting_get_feedback | 告警, 报警, alert, alarm, noise, 降噪 | 10 |
| `topology` | 拓扑查询 | topology_get_service_topology, topology_get_node, topology_get_upstream, topology_get_downstream, topology_find_path, topology_find_shortest_path | 拓扑, 依赖, 调用链, topology, dependency | 8 |
| `capacity` | 容量规划 | slo_list, slo_get, slo_get_error_budget, slo_get_burn_rate_alerts, topology_get_stats, alerting_stats | 容量, SLO, 错误预算, 燃烧率, capacity, budget | 8 |
| `general` | 通用查询 | 所有工具 | （兜底池，无关键词匹配时使用） | 1 |

#### 5.0.7 与现有架构的兼容性

| 现有组件 | 变化 |
|---------|------|
| `ToolRegistry` | 保留为底层注册表，`PoolRegistry` 在其上构建，不修改任何现有方法 |
| `AuthzToolWrapper` | 不变，权限鉴权仍在工具执行层 |
| `ReadOnlyTool` 接口 | 不变，所有瘦工具仍实现此接口 |
| eino `ToolsNode` | `NewToolsNodeWithAuth` 新增 `intent` 参数，从池选择工具 |
| 原胖工具 | 保留文件，标记为 deprecated，瘦工具独立文件 |

### 5.1 P0 — 技能系统

```go
// domain/skill.go

type Skill interface {
    Info(ctx context.Context) (*SkillInfo, error)
    Validate(params map[string]any) error
    Execute(ctx context.Context, params map[string]any) (*SkillResult, error)
    IsReadOnly() bool
    RequiredPermission() string
}

type SkillInfo struct {
    Name        string      `json:"name"`
    Desc        string      `json:"desc"`
    Version     string      `json:"version"`
    Parameters  []ParamSpec `json:"parameters"`
    Category    string      `json:"category"`
    Tags        []string    `json:"tags,omitempty"`
}

type ParamSpec struct {
    Name     string   `json:"name"`
    Type     string   `json:"type"`     // string, integer, boolean, array, object
    Required bool     `json:"required"`
    Desc     string   `json:"desc"`
    Enum     []string `json:"enum,omitempty"`
    Default  any      `json:"default,omitempty"`
}

type SkillResult struct {
    Success bool   `json:"success"`
    Data    any    `json:"data,omitempty"`
    Error   string `json:"error,omitempty"`
}
```

**与现有 MCP 工具的适配器**：

```go
// infrastructure/skills/adapter.go

type ToolSkillAdapter struct {
    tool ReadOnlyTool
}

func NewToolSkillAdapter(tool ReadOnlyTool) *ToolSkillAdapter {
    return &ToolSkillAdapter{tool: tool}
}

func (a *ToolSkillAdapter) Info(ctx context.Context) (*SkillInfo, error) {
    toolInfo, err := a.tool.Info(ctx)
    if err != nil {
        return nil, err
    }
    return &SkillInfo{
        Name: toolInfo.Name,
        Desc: toolInfo.Desc,
    }, nil
}

func (a *ToolSkillAdapter) Execute(ctx context.Context, params map[string]any) (*SkillResult, error) {
    jsonBytes, err := json.Marshal(params)
    if err != nil {
        return nil, err
    }
    result, err := a.tool.InvokableRun(ctx, string(jsonBytes))
    if err != nil {
        return &SkillResult{Success: false, Error: err.Error()}, nil
    }
    return &SkillResult{Success: true, Data: result}, nil
}

func (a *ToolSkillAdapter) IsReadOnly() bool          { return a.tool.IsReadOnly() }
func (a *ToolSkillAdapter) RequiredPermission() string { return a.tool.RequiredPermission() }
```

### 5.2 P0 — 上下文管理（Qdrant + MySQL 双层存储）

```go
// domain/context.go

type ConversationContext struct {
    ID               string
    SessionID        string
    UserID           string
    Messages         []Message
    Metadata         map[string]any
    TokenCount       int
    CompressionLevel CompressionLevel
    CreatedAt        time.Time
    UpdatedAt        time.Time
    ExpiresAt        time.Time
}

type CompressionLevel string

const (
    CompressionNone   CompressionLevel = "none"
    CompressionLow    CompressionLevel = "low"     // 保留 80%
    CompressionMedium CompressionLevel = "medium"  // 保留 50%
    CompressionHigh   CompressionLevel = "high"    // 保留 20%
)

type CompressionStrategy string

const (
    StrategyKeyword CompressionStrategy = "keyword"  // 关键词提取（优先）
    StrategySummary CompressionStrategy = "summary"  // LLM 摘要（备选）
)

type CompressionConfig struct {
    Level     CompressionLevel
    Strategy  CompressionStrategy
    MaxTokens int
}

type Message struct {
    Role       string        `json:"role"`
    Content    string        `json:"content"`
    ToolCalls  []ToolCallRef `json:"tool_calls,omitempty"`
    ToolID     string        `json:"tool_id,omitempty"`
    Timestamp  time.Time     `json:"timestamp"`
    TokenCount int           `json:"token_count"`
    Embedding  []float64     `json:"embedding,omitempty"`  // Qdrant 向量
}

type ToolCallRef struct {
    ID        string         `json:"id"`
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

type ContextManager interface {
    Create(ctx context.Context, sessionID, userID string) (*ConversationContext, error)
    AppendMessage(ctx context.Context, contextID string, msg Message) error
    GetRelevant(ctx context.Context, contextID string, query string, maxTokens int) (*ConversationContext, error)
    Compact(ctx context.Context, contextID string, config CompressionConfig) (*CompressionResult, error)
    CleanExpired(ctx context.Context) error
}

type CompressionResult struct {
    OriginalTokens int     `json:"original_tokens"`
    CompressedTokens int   `json:"compressed_tokens"`
    CompressionRatio float64 `json:"compression_ratio"`
    Strategy       CompressionStrategy `json:"strategy"`
    KeywordsKept   []string `json:"keywords_kept,omitempty"`
}
```

**Qdrant 向量存储接口**：

```go
// infrastructure/context/qdrant_store.go

type VectorStore interface {
    Store(ctx context.Context, collection string, pointID string, vector []float64, payload map[string]any) error
    Search(ctx context.Context, collection string, queryVector []float64, filter map[string]any, limit int) ([]SearchResult, error)
    Delete(ctx context.Context, collection string, pointIDs []string) error
    CreateCollection(ctx context.Context, name string, dimension int) error
}

type SearchResult struct {
    ID      string
    Score   float64
    Payload map[string]any
}
```

**Embedding 接口**：

```go
// pkg/infra/embedding.go

type EmbeddingService interface {
    Embed(ctx context.Context, text string) ([]float64, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
    Dimension() int
}
```

**双层存储架构**：

```
写入路径: Message → MySQL(完整内容) → Embedding → Qdrant(向量+payload) → freecache(热缓存)
读取路径: freecache → MySQL(精确检索) → Qdrant(语义检索)
清理路径: asynq 定时任务 → MySQL DELETE + Qdrant Delete
```

### 5.3 P0 — 任务处理

```go
// domain/task.go

type Task struct {
    ID          string
    ParentID    *string
    SessionID   string
    Description string
    Status      TaskStatus
    SubTasks    []*Task
    Result      *TaskResult
    RetryCount  int
    MaxRetries  int
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type TaskStatus string

const (
    TaskPending   TaskStatus = "pending"
    TaskRunning   TaskStatus = "running"
    TaskCompleted TaskStatus = "completed"
    TaskFailed    TaskStatus = "failed"
    TaskRetrying  TaskStatus = "retrying"
)

type TaskResult struct {
    Success  bool   `json:"success"`
    Output   any    `json:"output,omitempty"`
    Error    string `json:"error,omitempty"`
    Duration int64  `json:"duration_ms"`
}

type TaskDecomposer interface {
    Decompose(ctx context.Context, taskDescription string) ([]*Task, error)
}

type TaskVerifier interface {
    Verify(ctx context.Context, task *Task) (*VerificationResult, error)
}

type VerificationResult struct {
    Passed  bool   `json:"passed"`
    Score   float64 `json:"score"`
    Reason  string  `json:"reason,omitempty"`
}

type RetryPolicy interface {
    ShouldRetry(task *Task, err error) bool
    NextDelay(attempt int) time.Duration
}
```

### 5.4 P1 — 三层记忆

```go
// domain/memory.go

type MemoryLayer string

const (
    MemoryWorking   MemoryLayer = "working"    // 当前对话上下文
    MemoryEpisodic  MemoryLayer = "episodic"   // 历史对话摘要
    MemorySemantic  MemoryLayer = "semantic"    // 知识库/RAG
)

type MemoryEntry struct {
    ID        string
    Layer     MemoryLayer
    SessionID string
    Content   string
    Embedding []float64
    Metadata  map[string]any
    CreatedAt time.Time
    ExpiresAt *time.Time
}

type MemoryStore interface {
    Store(ctx context.Context, entry *MemoryEntry) error
    Retrieve(ctx context.Context, query string, layer MemoryLayer, limit int) ([]*MemoryEntry, error)
    Forget(ctx context.Context, id string) error
}
```

### 5.5 P2 — Agent Fork

```go
// domain/fork.go

type Fork struct {
    ID          string
    SessionID   string
    ParentAgent string
    Branches    []ForkBranch
    MergePolicy MergePolicy
    Status      ForkStatus
}

type ForkBranch struct {
    ID       string
    AgentID  string
    Task     string
    Result   *TaskResult
    Status   TaskStatus
}

type MergePolicy string

const (
    MergeFirst    MergePolicy = "first"     // 第一个完成即返回
    MergeAll      MergePolicy = "all"       // 等待所有完成
    MergeBest     MergePolicy = "best"      // 选择最优结果
    MergeConsensus MergePolicy = "consensus" // 共识投票
)

type ForkStatus string

const (
    ForkRunning  ForkStatus = "running"
    ForkMerged   ForkStatus = "merged"
    ForkFailed   ForkStatus = "failed"
    ForkTimeout  ForkStatus = "timeout"
)
```

### 5.6 P0 — 可观测性与可视化（OTel GenAI 原生）

> **设计原则：不自建 trace/metrics 采集系统，直接基于 OpenTelemetry GenAI 语义约定构建可观测性。**
> 项目已有 `AISession`/`ToolCall` 模型与 otel GenAI 字段高度对齐，但采集链路完全断裂。
> 本节将自建 `TraceCollector`/`ThoughtRecorder`/`AgentMetrics` 替换为 OTel 原生方案。

#### 5.6.1 现状评估

| 层级 | 现有能力 | 覆盖度 | 缺口 |
|------|---------|--------|------|
| 数据模型 | `AISession` + `ToolCall` 已有完整 otel GenAI 字段 | 🟢 70% | 缺 workflow/task/embedding 层级 |
| 基础设施 | OTel SDK 已引入（go.mod: `otel v1.41.0`） | 🟡 30% | 仅 indirect，无 Provider 初始化 |
| Agent 模块 | `TraceCollector`/`ThoughtRecorder`/`AgentMetrics` 已设计 | 🔴 0% | 自建方案，与 OTel 生态脱节 |
| trace 模块 | `internal/trace/` 全部空壳 | 🔴 0% | 9 个文件全部 `type X struct{}` |

**核心问题：自建 TraceCollector 与 OTel GenAI 语义约定不对齐，无法与 Tempo/Jaeger/Grafana 集成。**

#### 5.6.2 OTel GenAI Span 层级设计

遵循 [OTel GenAI Semantic Conventions v1.28.0+](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/) 及社区提案 [#2912](https://github.com/open-telemetry/semantic-conventions/issues/2912)：

```
gen_ai.workflow diagnose_workflow              ← 工作流编排
├── gen_ai.agent invoke_agent diagnose_agent   ← Agent/技能调用
│   ├── gen_ai.task find_anomalies_task        ← 子任务
│   │   └── chat gpt-4                         ← LLM 调用
│   └── execute_tool topology_find_anomalies   ← 工具调用
├── gen_ai.agent invoke_agent report_agent
│   └── chat gpt-4
└── [Event] gen_ai.interrupt                   ← 人工审批
```

**Span 层级映射表：**

| 项目概念 | otel GenAI Span | 关键 Attributes |
|---------|----------------|-----------------|
| 工作流执行 | `gen_ai.workflow {name}` | `gen_ai.workflow.name`, `gen_ai.workflow.type`, `gen_ai.operation.name=workflow` |
| 技能/Agent 调用 | `invoke_agent {name}` | `gen_ai.agent.name`, `gen_ai.operation.name=invoke_agent` |
| 子任务 | `gen_ai.task {name}` | `gen_ai.task.name`, `gen_ai.task.type`, `gen_ai.task.status` |
| LLM 调用 | `chat {model}` | `gen_ai.request.model`, `gen_ai.usage.input_tokens/output_tokens` |
| 工具调用 | `execute_tool {tool_name}` | `gen_ai.tool.name`, `gen_ai.tool.type`, `gen_ai.tool.call.id` |
| Embedding 调用 | `embeddings {model}` | `gen_ai.request.model`, `gen_ai.usage.input_tokens` |
| 人工审批 | Span Event `gen_ai.interrupt` | `approval.reason`, `approval.tool` |
| 上下文压缩 | Span Event `gen_ai.context.compact` | `compression.level`, `compression.ratio` |

#### 5.6.3 领域模型（OTel 对齐版）

```go
// domain/trace.go

type AgentTrace struct {
    ID           string
    SessionID    string
    TraceID      string              // OTel Trace ID
    SpanID       string              // OTel Span ID
    ParentSpanID string              // OTel Parent Span ID
    WorkflowID   string
    NodeID       string
    NodeType     string              // workflow / agent / task / chat_model / tools / lambda / interrupt
    Operation    string              // gen_ai.operation.name: workflow / invoke_agent / task / chat / execute_tool / embeddings
    Input        json.RawMessage
    Output       json.RawMessage
    DurationMs   int
    Status       TraceStatus
    Attributes   map[string]any      // OTel Span Attributes（gen_ai.* 语义约定）
    CreatedAt    time.Time
}

type TraceStatus string

const (
    TraceRunning   TraceStatus = "running"
    TraceCompleted TraceStatus = "completed"
    TraceFailed    TraceStatus = "failed"
)

type ThoughtStep struct {
    ID         string
    SessionID  string
    TraceID    string
    SpanID     string              // 关联到具体 OTel Span
    StepType   string              // reasoning / tool_selection / tool_result / summary
    Content    string
    Reasoning  string              // LLM Chain of Thought（可配置脱敏）
    DurationMs int
    CreatedAt  time.Time
}
```

#### 5.6.4 OTel 原生接口设计

**替代原自建 TraceCollector/ThoughtRecorder/AgentMetrics：**

```go
// infrastructure/otel/tracer.go

type AgentTracer struct {
    tracer trace.Tracer
    meter  metric.Meter
}

func NewAgentTracer() *AgentTracer {
    return &AgentTracer{
        tracer: otel.Tracer("cloud-agent-monitor/agent"),
        meter:  otel.Meter("cloud-agent-monitor/agent"),
    }
}

// StartWorkflowSpan 创建工作流 span
func (t *AgentTracer) StartWorkflowSpan(ctx context.Context, name string, workflowType string) (context.Context, trace.Span) {
    ctx, span := t.tracer.Start(ctx, fmt.Sprintf("gen_ai.workflow %s", name),
        trace.WithAttributes(
            attribute.String("gen_ai.operation.name", "workflow"),
            attribute.String("gen_ai.workflow.name", name),
            attribute.String("gen_ai.workflow.type", workflowType),
        ),
    )
    return ctx, span
}

// StartAgentSpan 创建 Agent 调用 span
func (t *AgentTracer) StartAgentSpan(ctx context.Context, agentName string) (context.Context, trace.Span) {
    ctx, span := t.tracer.Start(ctx, fmt.Sprintf("invoke_agent %s", agentName),
        trace.WithAttributes(
            attribute.String("gen_ai.operation.name", "invoke_agent"),
            attribute.String("gen_ai.agent.name", agentName),
        ),
    )
    return ctx, span
}

// StartTaskSpan 创建子任务 span
func (t *AgentTracer) StartTaskSpan(ctx context.Context, taskName, taskType string) (context.Context, trace.Span) {
    ctx, span := t.tracer.Start(ctx, fmt.Sprintf("gen_ai.task %s", taskName),
        trace.WithAttributes(
            attribute.String("gen_ai.operation.name", "task"),
            attribute.String("gen_ai.task.name", taskName),
            attribute.String("gen_ai.task.type", taskType),
        ),
    )
    return ctx, span
}

// RecordThought 将思考过程作为 Span Event 记录
func (t *AgentTracer) RecordThought(ctx context.Context, stepType, content, reasoning string) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent("gen_ai.thought", trace.WithAttributes(
        attribute.String("gen_ai.thought.type", stepType),
        attribute.String("gen_ai.thought.content", content),
        attribute.String("gen_ai.thought.reasoning", reasoning),
    ))
}

// RecordInterrupt 记录人工审批中断
func (t *AgentTracer) RecordInterrupt(ctx context.Context, reason, toolName string) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent("gen_ai.interrupt", trace.WithAttributes(
        attribute.String("approval.reason", reason),
        attribute.String("approval.tool", toolName),
    ))
}

// RecordCompact 记录上下文压缩
func (t *AgentTracer) RecordCompact(ctx context.Context, level string, ratio float64) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent("gen_ai.context.compact", trace.WithAttributes(
        attribute.String("compression.level", level),
        attribute.Float64("compression.ratio", ratio),
    ))
}
```

**OTel Provider 初始化：**

```go
// pkg/infra/otel.go

func InitOTelProvider(ctx context.Context, cfg *config.OTelConfig) (func(context.Context) error, error) {
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceNameKey.String("cloud-agent-monitor"),
            semconv.ServiceVersionKey.String("1.0.0"),
        ),
    )
    if err != nil {
        return nil, err
    }

    tracerProvider := sdktrace.NewTracerProvider(
        sdktrace.WithResource(res),
        sdktrace.WithBatcher(otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
            otlptracegrpc.WithInsecure(),
        )),
    )
    otel.SetTracerProvider(tracerProvider)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    meterProvider := sdkmetric.NewMeterProvider(
        sdkmetric.WithResource(res),
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
            otlpmetricgrpc.NewClient(
                otlpmetricgrpc.WithEndpoint(cfg.OTLPMetricEndpoint),
                otlpmetricgrpc.WithInsecure(),
            ),
        )),
    )
    otel.SetMeterProvider(meterProvider)

    return func(ctx context.Context) error {
        return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
    }, nil
}
```

**eino Graph 节点注入 span：**

```go
// infrastructure/otel/eino_interceptor.go

func TracedLambda(name string, fn func(ctx context.Context, input *schema.Message) (*schema.Message, error)) compose.AnyLambda {
    tracer := otel.Tracer("cloud-agent-monitor/agent")
    return compose.AnyLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
        ctx, span := tracer.Start(ctx, name,
            trace.WithAttributes(
                attribute.String("gen_ai.operation.name", "task"),
                attribute.String("gen_ai.task.name", name),
            ),
        )
        defer span.End()

        result, err := fn(ctx, input)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        return result, err
    })
}
```

#### 5.6.5 OTel GenAI 标准指标

**必须实现的 otel GenAI 标准指标：**

| 指标名 | 类型 | 单位 | 关键 Attributes |
|--------|------|------|-----------------|
| `gen_ai.client.operation.duration` | Histogram | s | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.agent.name` |
| `gen_ai.client.token.usage` | Histogram | token | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.token.type` |
| `gen_ai.workflow.duration` | Histogram | s | `gen_ai.workflow.name`, `gen_ai.workflow.type` |
| `gen_ai.task.duration` | Histogram | s | `gen_ai.task.name`, `gen_ai.task.type` |

**项目自定义指标（补充标准指标未覆盖的维度）：**

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `agent_tool_call_duration_seconds` | Histogram | 工具调用耗时（按工具名分桶） |
| `agent_tool_call_errors_total` | Counter | 工具调用错误数 |
| `agent_context_compression_ratio` | Gauge | 上下文压缩率 |
| `agent_pool_selection_total` | Counter | 工具池选择次数（按池名） |
| `agent_intent_route_duration_seconds` | Histogram | 意图路由耗时 |

#### 5.6.6 数据流架构

```
写入路径:
  用户请求 → [Gin + otelhttp 中间件] → 注入 trace context
    → [AgentTracer.StartWorkflowSpan] → gen_ai.workflow span
      → [AgentTracer.StartAgentSpan] → invoke_agent span
        → [AgentTracer.StartTaskSpan] → gen_ai.task span
          → [chat/execute_tool span] → 底层 LLM/Tool span
        → [AgentTracer.RecordThought] → Span Event
      → [AgentTracer.RecordInterrupt] → Span Event (审批)
    → span.End()

采集路径:
  OTel TracerProvider
    → OTLP gRPC Exporter → Tempo/Jaeger (实时可视化)
    → SpanProcessor (自定义) → MySQL (持久化，复用 ai_sessions/tool_calls 表)

  OTel MeterProvider
    → OTLP gRPC Exporter → Prometheus (指标采集)
```

#### 5.6.7 安全风险防范

| 风险 | 防范措施 |
|------|---------|
| Span 中泄露敏感数据 | 配置 `OTEL_GEN_AI_CONTENT_CAPTURE=metadata`（仅捕获元数据，不捕获内容）；生产环境禁止 `gen_ai.content.prompt/completion` 事件 |
| Token 指标暴露业务规模 | 指标按 role 聚合，不暴露 user_id；Grafana 仪表盘按 RBAC 控制访问 |
| Trace ID 跨模块传播泄露 | 遵循最小权限原则，trace 查询需 `agent:traces:read` 权限 |
| ThoughtStep 记录推理过程 | `Reasoning` 字段脱敏处理，生产环境可配置 `record_reasoning: false` |
| OTel Exporter 端点暴露 | Exporter 仅监听内部网络，NetworkPolicy 限制访问 |

#### 5.6.8 与现有架构的兼容性

| 现有功能 | 兼容性 | 处理策略 |
|---------|--------|---------|
| `AISession`/`ToolCall` 模型 | ✅ 完全兼容 | OTel Span → MySQL 双写，复用现有字段（trace_id/span_id/parent_span_id/gen_ai.*） |
| `internal/trace/` 空壳 | ✅ 将删除 | 9 个空壳文件无代码引用，功能整合到 `agent/infrastructure/otel/`（详见 §5.8） |
| `go.opentelemetry.io/otel` 依赖 | ✅ 已有 | 从 indirect 升级为 direct，添加 `sdk/trace`、`sdk/metric`、`otlp/grpc` |
| `otelhttp` 中间件 | ✅ 已有 | `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 已在 go.mod |
| Prometheus 指标 | ✅ 完全兼容 | OTel Meter → OTLP → Prometheus，自定义指标保持 `agent_` 前缀 |
| eino Graph 执行 | ✅ 无侵入 | span 仅作为 side-effect，不改变 Graph 编译/运行结果 |
| freecache | ✅ 无冲突 | trace 数据使用独立缓存 key 前缀，不超总容量 10% |

### 5.7 P0-A — 场景化工作流（Workflow）

> 基于 eino `compose.Graph` 构建场景化预定义工作流，与 Tool Pool 天然配合。每个场景池对应一个工作流，池中的工具就是该工作流可用的工具集。

#### 5.6.1 设计理念

采用**方案 B：场景化预定义工作流**，而非简单 ReAct 或动态编排：

| 方案 | 说明 | 选择 |
|------|------|------|
| A. 简单 ReAct | LLM 自由选工具，无流程控制 | ❌ 复杂任务容易跑偏 |
| **B. 场景化预定义工作流** | **意图路由 → 预定义 DAG → 受控执行** | ✅ **采用** |
| C. 动态编排 | LLM 生成工作流描述 → 解析为 Graph | ❌ 质量不稳定，安全风险高 |

核心优势：
- 流程可控，关键步骤不遗漏
- 可插入人工审批（eino `Interrupt`）
- 可复用子图（eino `AddGraphNode`）
- 与 Tool Pool 池分组天然对齐

#### 5.6.2 整体架构

```
用户提问
    ↓
[意图识别] ← WorkflowRouter（复用 PoolRegistry 的 IntentRouter）
    ↓
    ├─ "故障/异常/down"     → 诊断工作流 (diagnose)
    ├─ "告警/报警/alert"    → 告警分析工作流 (alert_analyze)
    ├─ "容量/SLO/预算"      → 容量规划工作流 (capacity_check)
    └─ 其他                 → 自由对话工作流 (free_chat, ReAct 兜底)
    ↓
[工作流执行] ← eino Graph.Compile().Run()
    ↓
LLM 总结回答
```

#### 5.6.3 领域模型

```go
// domain/workflow.go

type WorkflowDef struct {
    ID          string
    Name        string
    Description string
    PoolID      string            // 关联的 Tool Pool ID
    Keywords    []string          // 意图匹配关键词
    Priority    int               // 路由优先级
    Steps       []WorkflowStep    // 工作流步骤定义
}

type WorkflowStep struct {
    ID          string
    Type        StepType           // chat_model / tools / lambda / branch / interrupt
    Name        string
    Config      map[string]any     // 步骤配置（模型名、工具列表、条件等）
    Next        string             // 下一步骤 ID（空表示结束）
    Branches    []WorkflowBranch   // 条件分支（仅 branch 类型）
}

type WorkflowBranch struct {
    Condition string   // 分支条件描述
    TargetID  string   // 目标步骤 ID
}

type StepType string

const (
    StepChatModel StepType = "chat_model"   // LLM 推理节点
    StepTools     StepType = "tools"        // 工具调用节点
    StepLambda    StepType = "lambda"       // 自定义逻辑节点
    StepBranch    StepType = "branch"       // 条件分支节点
    StepInterrupt StepType = "interrupt"    // 人工审批中断点
)

type WorkflowInstance struct {
    ID           string
    WorkflowID   string
    SessionID    string
    Status       WorkflowStatus
    CurrentStep  string
    Checkpoint   []byte              // eino CheckPointStore 序列化状态
    Result       any
    Error        string
    StartedAt    time.Time
    UpdatedAt    time.Time
}

type WorkflowStatus string

const (
    WorkflowRunning    WorkflowStatus = "running"
    WorkflowCompleted  WorkflowStatus = "completed"
    WorkflowFailed     WorkflowStatus = "failed"
    WorkflowInterrupted WorkflowStatus = "interrupted"  // 等待人工审批
    WorkflowCancelled  WorkflowStatus = "cancelled"
)
```

#### 5.6.4 核心接口

```go
// WorkflowBuilder: 将 WorkflowDef 编译为 eino Graph
type WorkflowBuilder interface {
    Build(ctx context.Context, def *WorkflowDef, poolRegistry *PoolRegistry) (compose.Runnable[*schema.Message, *schema.Message], error)
}

// WorkflowRouter: 意图 → 工作流路由
type WorkflowRouter interface {
    Route(ctx context.Context, intent string) (*WorkflowDef, error)
}

// WorkflowRegistry: 工作流注册中心
type WorkflowRegistry interface {
    Register(def *WorkflowDef) error
    Get(workflowID string) (*WorkflowDef, bool)
    List() []*WorkflowDef
    Route(ctx context.Context, intent string) (*WorkflowDef, error)
}
```

#### 5.6.5 内置工作流定义

**1. 故障诊断工作流 (diagnose)**

```
用户描述故障
    → [ChatModel: 提取关键服务信息]
    → [ToolsNode: 查询异常服务 (topology_find_anomalies)]
    → [Lambda: 判断是否需要深入分析]
        ├─ 是 → [ToolsNode: 影响分析 (topology_analyze_impact)]
        │       → [ToolsNode: 关联告警 (alerting_list_active)]
        │       → [ChatModel: 生成诊断报告]
        └─ 否 → [ChatModel: 简要回答]
```

对应 eino Graph 构建：
```go
graph := compose.NewGraph[*schema.Message, *schema.Message]()
graph.AddChatModelNode("extract", chatModel)
graph.AddToolsNode("find_anomalies", toolsNode)     // 从 diagnose 池选择
graph.AddLambdaNode("judge_depth", lambdaNode)
graph.AddToolsNode("analyze_impact", toolsNode)
graph.AddToolsNode("related_alerts", toolsNode)
graph.AddChatModelNode("report", chatModel)
graph.AddChatModelNode("brief_answer", chatModel)

graph.AddEdge("extract", "find_anomalies")
graph.AddEdge("find_anomalies", "judge_depth")
graph.AddBranch("judge_depth", branch)  // 条件分支
graph.AddEdge("analyze_impact", "related_alerts")
graph.AddEdge("related_alerts", "report")
```

**2. 告警分析工作流 (alert_analyze)**

```
用户描述告警问题
    → [ChatModel: 提取告警关键词]
    → [ToolsNode: 查询活跃告警 (alerting_list_active)]
    → [Lambda: 判断告警数量]
        ├─ 多 → [ToolsNode: 降噪分析 (alerting_noisy)]
        │       → [ToolsNode: 高风险告警 (alerting_high_risk)]
        │       → [ChatModel: 生成告警分析报告]
        └─ 少 → [ToolsNode: 告警详情 (alerting_get_feedback)]
                → [ChatModel: 逐条分析]
```

**3. 容量规划工作流 (capacity_check)**

```
用户询问容量/SLO
    → [ChatModel: 提取关注的 SLO/服务]
    → [ToolsNode: 查询 SLO 列表 (slo_list)]
    → [ToolsNode: 错误预算 (slo_get_error_budget)]
    → [ToolsNode: 燃烧率 (slo_get_burn_rate_alerts)]
    → [ChatModel: 生成容量评估报告]
```

**4. 自由对话工作流 (free_chat)** — ReAct 兜底

```
用户提问
    → [ChatModel + ToolsNode: 标准 ReAct 循环]
        LLM 自由选择 general 池中的工具
```

#### 5.6.6 与 Tool Pool 的集成

| Tool Pool | 对应工作流 | 工具来源 |
|-----------|-----------|---------|
| `diagnose` | 故障诊断 | `PoolRegistry.SelectTools(ctx, "diagnose")` |
| `alert` | 告警分析 | `PoolRegistry.SelectTools(ctx, "alert")` |
| `capacity` | 容量规划 | `PoolRegistry.SelectTools(ctx, "capacity")` |
| `general` | 自由对话 | `PoolRegistry.SelectTools(ctx, "general")` |

工作流构建时，`WorkflowBuilder` 从 `PoolRegistry` 获取对应池的工具列表，注入 `ToolsNode`。

#### 5.6.7 人工审批机制

利用 eino `Interrupt` 实现关键节点的审批中断：

```go
// 在需要审批的步骤前插入 Interrupt
graph.AddLambdaNode("approval_gate", compose.AnyLambda(
    func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
        return nil, compose.Interrupt(ctx, map[string]any{
            "reason":  "写操作需要人工审批",
            "tool":    "xxx",
            "params":  input.Content,
        })
    },
))
```

执行到该节点时暂停，等待 `compose.Resume(ctx)` 恢复。

#### 5.6.8 与各开发阶段的集成

| 阶段 | 工作流能力 | 说明 |
|------|-----------|------|
| **P0-0** Tool Pool | 无工作流 | 保持 ReAct，但池分组为工作流奠定基础 |
| **P0-A** 技能系统 | 技能 = 工作流 | `Skill.Execute()` 内部调用 `WorkflowBuilder.Build().Run()`，技能即预定义工作流的封装 |
| **P0-C** 任务处理 | 子任务→工作流节点 | `TaskDecomposer` 拆解的子任务映射到工作流步骤，`TaskExecutor` 驱动 Graph 执行 |
| **P1-C** 会话管理 | 多轮对话=多次 Graph 执行 | 会话绑定工作流实例，通过 `CheckPointStore` 实现断点续跑 |
| **P2** Agent Fork | Fork=并行 Graph 实例 | 同一 WorkflowDef 的多个并行执行，结果按 MergePolicy 聚合 |

### 5.8 Trace 整合架构方案

> **核心决策：删除 `internal/trace/` 空壳模块，将 trace 功能整合为 `internal/agent/infrastructure/otel/` 子模块。**
> 当前 `internal/trace/` 的 9 个文件全部是 `type X struct{}` 空壳，从未有过实现，不存在数据迁移或业务连续性风险。

#### 5.8.1 整合理由

| 维度 | 独立 trace 模块 | 整合为 agent 子模块 |
|------|---------------|-------------------|
| 数据主体 | trace 数据的主体是 Agent 行为 | ✅ 与 Agent 紧耦合，放在一起减少跨模块依赖 |
| 采集时机 | Agent 运行时才产生 span | ✅ Agent 代码直接调用 AgentTracer，无需跨模块调用 |
| 查询场景 | 人类查看 Agent 行为 | ✅ 通过 Agent API 统一暴露，权限统一管理 |
| 代码复用 | AgentTrace 与 AISession 字段重叠 | ✅ SpanProcessor 直接复用 ai_sessions/tool_calls 表 |
| 维护成本 | 两个模块各自维护 trace 逻辑 | ✅ 一套 OTel 基础设施，维护成本减半 |

#### 5.8.2 整合后目录结构

```
internal/
├── trace/                    ← 删除全部 9 个空壳文件
│   (此目录将被删除)
│
├── agent/                    ← 整合后的完整 Agent 模块
│   ├── domain/
│   │   ├── tool.go               ✅ 已有（保留）
│   │   ├── errors.go             ✅ 已有（保留）
│   │   ├── repository.go         ✅ 已有（保留）
│   │   ├── service.go            ✅ 已有（保留）
│   │   ├── trace.go              🆕 AgentTrace + ThoughtStep（OTel 对齐，§5.6.3 已定义）
│   │   ├── context.go            🆕 上下文管理领域模型
│   │   ├── task.go               🆕 任务领域模型
│   │   └── pool.go               🆕 工具池领域模型
│   │
│   ├── application/
│   │   ├── service.go            🆕 AgentService（核心编排）
│   │   ├── observability_service.go  🆕 可观测性查询服务
│   │   ├── context_service.go    🆕 上下文管理服务
│   │   └── task_service.go       🆕 任务管理服务
│   │
│   ├── infrastructure/
│   │   ├── eino/                 ✅ 已有（保留 + 扩展）
│   │   │   ├── registry.go           ✅ ToolRegistry
│   │   │   ├── node.go               ✅ NewToolsNodeWithAuth
│   │   │   ├── topology_tool.go      ✅ TopologyTool
│   │   │   ├── alerting_tool.go      ✅ AlertingTool
│   │   │   ├── slo_tool.go           ✅ SLOTool
│   │   │   └── workflow_builder.go   🆕 eino Graph 工作流构建
│   │   │
│   │   ├── otel/                 🆕 OTel GenAI 可观测性（原 trace 功能）
│   │   │   ├── tracer.go             AgentTracer（§5.6.4 已定义）
│   │   │   ├── eino_interceptor.go   eino Graph 节点 span 注入（§5.6.4 已定义）
│   │   │   ├── span_processor.go     OTel Span → MySQL 双写
│   │   │   └── metrics.go            OTel Meter 指标注册
│   │   │
│   │   ├── auth/                 ✅ 已有（保留 + 扩展权限）
│   │   │   ├── permission.go         ✅ + 🆕 agent:traces:read 等权限
│   │   │   └── provider.go           ✅ 保留
│   │   │
│   │   ├── middleware/           ✅ 已有（保留 + 扩展）
│   │   │   └── eino_auth.go          ✅ + otelhttp 注入
│   │   │
│   │   ├── persistence/         🆕 MySQL 持久化
│   │   │   ├── trace_repository.go   agent_traces + agent_thoughts GORM 仓储
│   │   │   ├── context_repository.go 上下文 GORM 仓储
│   │   │   └── pool_repository.go    工具池 GORM 仓储
│   │   │
│   │   └── server/               🆕 gRPC/HTTP Agent 服务
│   │
│   └── interfaces/http/
│       ├── handler.go            🆕 Agent HTTP Handler
│       └── routes.go             🆕 Agent 路由注册
│
├── aiinfra/                  ← 保持独立（补全实现）
│   ├── domain/                   ✅ 已有完整 GenAI 语义约定
│   ├── application/              🆕 补全 service.go
│   ├── infrastructure/           🆕 补全 collector/metrics/repository
│   └── interfaces/http/          🆕 补全 handler
```

#### 5.8.3 数据流转路径

**写入路径（Agent 运行时产生可观测数据）：**

```
1. HTTP 请求 → otelhttp 中间件注入 trace context → EinoAuthMiddleware 注入 user_id
2. AgentService → AgentTracer.StartWorkflowSpan() → gen_ai.workflow span
3. 工作流节点 → AgentTracer.StartAgentSpan() → invoke_agent span
4. 子任务 → AgentTracer.StartTaskSpan() → gen_ai.task span
5. LLM 调用 → eino ChatModel → gen_ai.client.chat span + Token 用量
6. 工具调用 → ToolRegistry.Execute() → execute_tool span
7. 思考过程 → AgentTracer.RecordThought() → span.AddEvent("gen_ai.thought")
8. span.End() → BatchSpanProcessor → OTLP gRPC → Tempo/Jaeger
                                → 自定义 SpanProcessor → MySQL agent_traces/agent_thoughts
```

**读取路径（人类查询可观测数据）：**

```
1. GET /api/v1/agent/traces/:session_id → authMiddleware + casbin 权限检查
2. ObservabilityService.GetSessionTraces() → TraceRepository → MySQL
3. GET /api/v1/agent/traces/:trace_id/waterfall → 按 parent_span_id 构建瀑布图
4. GET /api/v1/agent/thoughts/:session_id → MySQL agent_thoughts
5. GET /api/v1/agent/metrics → Prometheus HTTP API → gen_ai.* 指标
```

#### 5.8.4 功能完整性对照

| 原 trace 空壳文件 | 原设计意图 | 整合后实现位置 | 覆盖 |
|------------------|----------|-------------|------|
| `domain/trace.go` | Trace 数据模型 | `agent/domain/trace.go` | ✅ AgentTrace（含 OTel 字段） |
| `domain/span.go` | Span 数据模型 | `agent/domain/trace.go` | ✅ AgentTrace 本身就是 span 模型 |
| `domain/service_dependency.go` | 服务依赖关系 | `topology/domain/` | ✅ 已在 topology 模块完整实现 |
| `application/service.go` | Trace 查询服务 | `agent/application/observability_service.go` | ✅ |
| `infrastructure/otlp_receiver.go` | OTLP 数据接收 | `pkg/infra/otel.go` + OTel SDK | ✅ OTel Provider 自动接收 |
| `infrastructure/jaeger_client.go` | Jaeger 查询 | OTel SDK → Tempo/Jaeger | ✅ OTLP Exporter 替代 |
| `infrastructure/tempo_client.go` | Tempo 查询 | OTel SDK → Tempo | ✅ OTLP Exporter 替代 |
| `interfaces/http/handler.go` | HTTP API | `agent/interfaces/http/handler.go` | ✅ 统一到 Agent API |
| `interfaces/http/routes.go` | 路由注册 | `agent/interfaces/http/routes.go` | ✅ 统一到 Agent 路由 |

#### 5.8.5 人类操作权限设计

**新增权限常量（扩展 `infrastructure/auth/permission.go`）：**

| 权限 | 说明 | viewer | editor | admin | agent |
|------|------|--------|--------|-------|-------|
| `agent:read` | Agent 对话/任务查询 | ✅ | ✅ | ✅ | ✅ |
| `agent:write` | Agent 对话/任务操作 | ❌ | ✅ | ✅ | ✅ |
| `agent:traces:read` | 调用链/思考过程查询 | ✅ | ✅ | ✅ | ❌ |
| `agent:traces:export` | 调用链导出 | ❌ | ❌ | ✅ | ❌ |
| `agent:metrics:read` | 指标查询 | ✅ | ✅ | ✅ | ❌ |

**关键设计：agent 角色不能读取 traces。** Agent 自身产生 trace 数据，不应有权限回读自己的决策过程，避免循环依赖。

**API 级权限控制：**

```go
// interfaces/http/routes.go

func RegisterRoutes(r *gin.RouterGroup, handler *Handler, authMW *auth.AuthMiddleware) {
    agent := r.Group("/agent")
    agent.Use(authMW.RequireAPIKey())

    agent.GET("/sessions/:id", RequirePermission("agent:read"), handler.GetSession)
    agent.POST("/sessions", RequirePermission("agent:write"), handler.CreateSession)
    agent.POST("/tasks", RequirePermission("agent:write"), handler.CreateTask)

    traces := agent.Group("/traces")
    traces.Use(RequirePermission("agent:traces:read"))
    {
        traces.GET("/:session_id", handler.GetSessionTraces)
        traces.GET("/:session_id/tree", handler.GetTraceTree)
        traces.GET("/:trace_id/waterfall", handler.GetTraceWaterfall)
    }

    thoughts := agent.Group("/thoughts")
    thoughts.Use(RequirePermission("agent:traces:read"))
    {
        thoughts.GET("/:session_id", handler.GetSessionThoughts)
    }

    metrics := agent.Group("/metrics")
    metrics.Use(RequirePermission("agent:metrics:read"))
    {
        metrics.GET("", handler.GetMetrics)
    }

    agent.POST("/traces/:session_id/export", RequirePermission("agent:traces:export"), handler.ExportTraces)
}
```

#### 5.8.6 数据隔离与共享策略

| 数据 | 存储位置 | 写入方 | 读取方 | 访问权限 |
|------|---------|--------|--------|---------|
| OTel Span（实时） | Tempo/Jaeger | OTLP Exporter | Grafana | Tempo 数据源权限 |
| OTel Span（持久化） | MySQL `agent_traces` | SpanProcessor | ObservabilityService | `agent:traces:read` |
| 思考过程 | MySQL `agent_thoughts` | SpanProcessor | ObservabilityService | `agent:traces:read` |
| LLM 调用详情 | MySQL `ai_sessions` | aiinfra 模块 | aiinfra API | `aiinfra:read` |
| 工具调用详情 | MySQL `tool_calls` | aiinfra 模块 | aiinfra API | `aiinfra:read` |
| OTel 指标 | Prometheus | OTel Meter | Grafana | Prometheus 数据源权限 |

**跨表关联键：**

| 关联 | 共享键 | 关系 |
|------|--------|------|
| agent_traces ↔ ai_sessions | `trace_id` | 一个 workflow 包含多个 LLM 调用 |
| agent_traces ↔ tool_calls | `span_id` | 一个 execute_tool span 对应一条 tool_call |
| agent_thoughts ↔ agent_traces | `span_id` | 思考过程附加到具体 span |

**隔离原则：**

| 原则 | 实现 |
|------|------|
| 写入隔离 | Agent 运行时写入 OTel Span → SpanProcessor → MySQL；人类只读 |
| 连接池隔离 | SpanProcessor 使用独立 MySQL 连接池（max 5 conn） |
| 缓存隔离 | trace 查询使用独立 freecache key 前缀 `agent:trace:*`，不超总容量 10% |
| Schema 隔离 | agent_traces/agent_thoughts 是独立表，不修改 ai_sessions/tool_calls 表结构 |

#### 5.8.7 性能影响评估

| 影响点 | 评估 | 缓解措施 |
|--------|------|---------|
| OTel span 创建 | 每次 `tracer.Start()` 约 1-5μs | BatchSpanProcessor 异步导出 |
| MySQL 双写 | 每次 span.End() 触发 INSERT | SpanProcessor 批量写入（每 100 条或每 5s flush） |
| HTTP 延迟 | otelhttp 中间件增加约 0.1ms | 可忽略 |
| 内存占用 | BatchSpanProcessor 缓存未导出 span | 限制 batch size 512，超限强制 flush |
| MySQL 存储 | 每次对话约 5-20 条 trace + 3-10 条 thought | 30 天保留 + 定期归档 |

**压测验证目标：**

| 指标 | 基线（无 OTel） | 目标（有 OTel） | 可接受范围 |
|------|----------------|----------------|-----------|
| P50 延迟 | 200ms | 210ms | +5% |
| P99 延迟 | 500ms | 525ms | +5% |
| 吞吐量 | 1000 req/s | 950 req/s | -5% |

#### 5.8.8 迁移计划

由于 `internal/trace/` 全部是空壳，**不存在数据迁移**。迁移仅涉及代码清理：

| 步骤 | 操作 | 风险 |
|------|------|------|
| 1 | 删除 `internal/trace/` 全部 9 个文件 | 零风险（无代码引用这些文件） |
| 2 | 在 `internal/agent/infrastructure/otel/` 创建 OTel 实现 | 零风险（新增代码） |
| 3 | 在 `internal/agent/domain/trace.go` 定义 AgentTrace/ThoughtStep | 零风险（新增代码） |
| 4 | 在 `cmd/platform-api/wire.go` 添加 OTel Provider + Agent 路由注册 | 低风险（需验证 Wire 编译） |
| 5 | 在 `pkg/config/config.go` 添加 OTelConfig | 零风险（新增配置） |
| 6 | 在 `infrastructure/auth/permission.go` 添加 trace 相关权限 | 零风险（新增常量） |

#### 5.8.9 维护与升级机制

| 维护维度 | 隔离策略 |
|---------|---------|
| 代码隔离 | `infrastructure/otel/` 独立包，仅通过 `AgentTracer` 接口暴露 |
| 配置隔离 | `config.OTelConfig` 独立配置段 |
| 依赖隔离 | OTel SDK 依赖仅在 `infrastructure/otel/` 和 `pkg/infra/otel.go` 中引用 |
| 测试隔离 | `infrastructure/otel/*_test.go` 使用 `tracetest` 独立测试 |

**OTel GenAI 语义约定升级策略：**

| 场景 | 处理方式 |
|------|---------|
| v1.28.0 稳定版字段变更 | 同步更新 `AgentTracer` 的 attribute key |
| #2912 提案字段稳定化 | optional → required，添加新字段到 `AgentTracer` |
| 新增 operation 类型 | 在 `AgentTracer` 中添加新 `StartXxxSpan` 方法 |
| 废弃 operation 类型 | 标记 deprecated，保留兼容性至少一个大版本 |

#### 5.8.10 Wire 依赖注入变更

```go
// cmd/platform-api/wire.go 新增 Provider

func ProvideOTelShutdown(cfg *config.Config) (func(context.Context) error, error) {
    return otel.InitOTelProvider(context.Background(), &cfg.OTel)
}

func ProvideAgentTracer() *agentotel.AgentTracer {
    return agentotel.NewAgentTracer()
}

func ProvideTraceRepository(db *gorm.DB) agentpersistence.TraceRepository {
    return agentpersistence.NewTraceRepository(db)
}

func ProvideObservabilityService(
    traceRepo agentpersistence.TraceRepository,
    promClient *promclient.Client,
) *agentapp.ObservabilityService {
    return agentapp.NewObservabilityService(traceRepo, promClient)
}

func ProvideAgentHandler(
    obsSvc *agentapp.ObservabilityService,
) *agenthttp.Handler {
    return agenthttp.NewHandler(obsSvc)
}
```

**ProvideHTTPServer 变更：**

```go
func ProvideHTTPServer(
    // ... 现有参数 ...
    agentHandler *agenthttp.Handler,       // 🆕
    otelShutdown func(context.Context) error, // 🆕
) *gin.Engine {
    // ... 现有代码 ...

    // 🆕 Agent 路由注册
    agenthttp.RegisterRoutes(protected, agentHandler, authMiddleware)

    // 🆕 otelhttp 中间件（在现有中间件之前）
    r.Use(otelhttp.NewMiddleware("cloud-agent-monitor"))

    return r
}
```

### 6.1 阶段规划

| 阶段 | 优先级 | 内容 | 依赖 | 交付物 |
|------|--------|------|------|--------|
| **P0-0** | 最高 | **MCP Tool Pool** | 无 | `ToolPool`/`PoolRegistry`/`IntentRouter`、21 个瘦工具、5 个内置池、池 CRUD API + MySQL 持久化 |
| **P0-D-前置** | 最高 | **OTel 基础设施**（原 trace 模块整合） | 无 | 删除 `internal/trace/` 空壳、OTel Provider 初始化、OTelConfig、otelhttp 中间件、go.mod 依赖升级、权限常量扩展 |
| **P0-A** | 最高 | 技能系统 + 场景化工作流 | P0-0 | `Skill` 接口、`SkillRegistry`、参数验证器、`ToolSkillAdapter`、3 个内置技能、4 个预定义工作流、`WorkflowBuilder`/`WorkflowRouter`/`WorkflowRegistry` |
| **P0-B** | 最高 | 上下文管理（Qdrant + MySQL） | Qdrant、BGE-M3、asynq | `ContextManager`、Qdrant 向量存储、MySQL 消息持久化、关键词提取压缩、compact API、用户绑定与数据隔离 |
| **P0-C** | 最高 | 任务处理 | eino（已有）、asynq（已有） | `TaskDecomposer`、`TaskExecutor`、`TaskVerifier`、`RetryPolicy`、任务状态跟踪 |
| **P0-D** | 最高 | 可观测性与可视化（OTel GenAI 原生） | P0-D-前置 | `AgentTracer`、eino span 注入、SpanProcessor MySQL 双写、调用链/思考过程可视化 API、OTel GenAI 标准指标 |
| **P1-A** | 高 | RAG 子系统 | Qdrant（P0-B 已集成）、BGE-M3 | Qdrant 文档向量存储、Embedding 生成、文档加载/分块、混合检索 |
| **P1-B** | 高 | 三层记忆 | P1-A、Redis（已有） | 工作记忆、情景记忆、语义记忆 |
| **P1-C** | 高 | 会话管理 | P0-A/B/C/D | 会话生命周期、多轮对话编排、工作流实例绑定、CheckPointStore 断点续跑 |
| **P2** | 中 | Agent Fork | P1 全部 | 并行分支执行、结果聚合、资源隔离 |

### 6.2 P0 阶段详细任务拆解

#### P0-D-前置：OTel 基础设施（原 trace 模块整合）

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 删除 `internal/trace/` 空壳 | `internal/trace/` | 删除全部 9 个空壳文件（无代码引用，零风险） |
| 2 | OTel Provider 初始化 | `pkg/infra/otel.go` | TracerProvider + MeterProvider + OTLP gRPC Exporter + 优雅关闭 |
| 3 | OTel 配置 | `pkg/config/config.go` | Config 新增 OTelConfig（Endpoint、采样率、内容捕获策略） |
| 4 | HTTP 入口注入 | `middleware/eino_auth.go` | 添加 otelhttp 中间件，请求入口自动注入 trace context |
| 5 | go.mod 依赖升级 | `go.mod` | otel 从 indirect → direct，添加 `sdk/trace`、`sdk/metric`、`otlp/grpc` |
| 6 | 权限常量扩展 | `infrastructure/auth/permission.go` | 新增 `agent:read`、`agent:write`、`agent:traces:read`、`agent:traces:export`、`agent:metrics:read` |
| 7 | Wire 注入变更 | `cmd/platform-api/wire.go` | 新增 ProvideOTelShutdown、ProvideAgentTracer、ProvideTraceRepository、ProvideObservabilityService、ProvideAgentHandler |

#### P0-0：MCP Tool Pool（工具池）

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 定义工具池领域模型 | `domain/tool_pool.go` | ToolPool、IntentRouter、ToolBudget、PoolRegistry 接口 |
| 2 | 实现 PoolRegistry | `infrastructure/eino/pool.go` | 池注册/注销/选择/列表，复用 ToolRegistry |
| 3 | 实现 IntentRouter | `infrastructure/eino/pool.go` | 关键词分词 → 池匹配 → 优先级排序 → 预算裁剪 |
| 4 | 拆分 topology 胖工具 | `infrastructure/eino/topology_*.go` | 10 个独立瘦工具，每个实现 ReadOnlyTool |
| 5 | 拆分 alerting 胖工具 | `infrastructure/eino/alerting_*.go` | 6 个独立瘦工具 |
| 6 | 拆分 slo 胖工具 | `infrastructure/eino/slo_*.go` | 5 个独立瘦工具 |
| 7 | 注册内置池 | `infrastructure/eino/pools_builtin.go` | diagnose/alert/topology/capacity/general 5 个池 |
| 8 | 更新 node 构建 | `infrastructure/eino/node.go` | NewToolsNodeWithAuth 改为从池选择工具 |
| 9 | 池管理 HTTP API | `interfaces/http/handler.go` | 池列表/详情/选择/注册/注销端点 |
| 10 | 单元测试 | `infrastructure/eino/pool_test.go` | PoolRegistry、IntentRouter、ToolBudget 测试 |

#### P0-A：技能系统

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 定义 Skill 领域模型 | `domain/skill.go` | Skill 接口、SkillInfo、ParamSpec、SkillResult |
| 2 | 实现 SkillRegistry | `infrastructure/skills/registry.go` | 注册/查询/执行/列表，复用 auth 权限体系 |
| 3 | 实现参数验证器 | `infrastructure/skills/validator.go` | 类型检查、必填校验、枚举值校验、默认值填充 |
| 4 | 实现 ToolSkillAdapter | `infrastructure/skills/adapter.go` | 将瘦工具（ReadOnlyTool）包装为 Skill，通过 PoolRegistry 获取工具 |
| 5 | 实现 K8s 诊断技能 | `infrastructure/skills/builtin/k8s_diagnose.go` | 组合 topology + alerting 工具的高级技能 |
| 6 | 实现告警分析技能 | `infrastructure/skills/builtin/alert_analyze.go` | 告警聚合/关联/降噪分析 |
| 7 | 实现 SLO 检查技能 | `infrastructure/skills/builtin/slo_check.go` | SLO 状态/错误预算/燃烧率综合检查 |
| 8 | 技能 HTTP API | `interfaces/http/handler.go` | 注册/列表/执行/详情端点 |
| 9 | 单元测试 | `infrastructure/skills/*_test.go` | Registry、Validator、Adapter 测试 |

#### P0-B：上下文管理（Qdrant + MySQL）

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 定义上下文领域模型 | `domain/context.go` | ConversationContext、Message、CompressionConfig、CompressionResult |
| 2 | Qdrant 基础设施 | `pkg/infra/qdrant.go` | QdrantClient 封装，Collection CRUD、Point Upsert/Search/Delete |
| 3 | Embedding 基础设施 | `pkg/infra/embedding.go` | BGE-M3 Embedding 封装，Embed/EmbedBatch/Dimension |
| 4 | Qdrant 配置 | `pkg/config/config.go` | Config 新增 QdrantConfig、EmbeddingConfig |
| 5 | 实现 Qdrant 向量存储 | `infrastructure/context/qdrant_store.go` | VectorStore 接口实现，消息向量存储与语义检索 |
| 6 | 实现 MySQL 消息持久化 | `infrastructure/context/mysql_store.go` | 消息 CRUD，用户 ID 绑定与数据隔离 |
| 7 | 实现 ContextManager | `infrastructure/context/manager.go` | 双层存储编排：MySQL → Embedding → Qdrant → freecache |
| 8 | 实现关键词提取压缩 | `infrastructure/context/keyword_compressor.go` | TF-IDF + TextRank 关键词提取，三级压缩配置 |
| 9 | 实现摘要压缩 | `infrastructure/context/summary_compressor.go` | LLM 驱动摘要，关键词压缩后仍超预算时触发 |
| 10 | 实现 compact API | `interfaces/http/handler.go` | POST /contexts/:id/compact，支持 level + strategy 配置 |
| 11 | 实现过期清理 | `infrastructure/context/cleaner.go` | asynq 定时任务，MySQL + Qdrant 批量删除 |
| 12 | 上下文持久化 | `infrastructure/persistence/context_repository.go` | GORM 仓储实现 |
| 13 | 单元测试 | `infrastructure/context/*_test.go` | Manager、Compressor、QdrantStore 测试 |

#### P0-C：任务处理

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 定义任务领域模型 | `domain/task.go` | Task、TaskStatus、TaskDecomposer、TaskVerifier、RetryPolicy |
| 2 | 实现任务拆解 | `infrastructure/task/decomposer.go` | LLM 驱动的任务分解，生成子任务 DAG |
| 3 | 实现任务执行器 | `infrastructure/task/executor.go` | 子任务调度、技能调用、结果收集 |
| 4 | 实现交付验证器 | `infrastructure/task/verifier.go` | LLM 驱动的结果质量评估 |
| 5 | 实现重试策略 | `infrastructure/task/retry.go` | 指数退避 + 可配置重试条件 |
| 6 | 任务 asynq 集成 | `infrastructure/task/queue_handlers.go` | 任务执行/重试的 asynq handler |
| 7 | 任务持久化 | `infrastructure/persistence/task_repository.go` | GORM 仓储实现 |
| 8 | 单元测试 | `infrastructure/task/*_test.go` | Decomposer、Executor、Verifier 测试 |

#### P0-D：可观测性与可视化（OTel GenAI 原生）

> 前置条件：P0-D-前置 已完成（OTel Provider、OTelConfig、otelhttp、权限常量、Wire 注入）

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1 | 定义调用链领域模型 | `domain/trace.go` | AgentTrace（含 TraceID/SpanID/Operation）、ThoughtStep（含 SpanID 关联） |
| 2 | 实现 AgentTracer | `infrastructure/otel/tracer.go` | StartWorkflowSpan/StartAgentSpan/StartTaskSpan/RecordThought/RecordInterrupt/RecordCompact |
| 3 | 实现 eino 拦截器 | `infrastructure/otel/eino_interceptor.go` | TracedLambda 包装器，eino Graph 节点自动注入 span |
| 4 | 实现 SpanProcessor | `infrastructure/otel/span_processor.go` | 自定义 SpanProcessor，OTel Span → MySQL 双写（复用 ai_sessions/tool_calls 表） |
| 5 | 实现 OTel 指标 | `infrastructure/otel/metrics.go` | gen_ai.client.token.usage、gen_ai.client.operation.duration、gen_ai.workflow.duration、gen_ai.task.duration + 自定义指标 |
| 6 | 调用链持久化 | `infrastructure/persistence/trace_repository.go` | GORM 仓储实现 |
| 7 | 思考过程持久化 | `infrastructure/persistence/thought_repository.go` | GORM 仓储实现 |
| 8 | 可视化 HTTP API | `interfaces/http/handler.go` | 调用链/思考过程/指标摘要端点 + 瀑布图端点 |
| 9 | Agent 路由注册 | `interfaces/http/routes.go` | /agent/traces、/agent/thoughts、/agent/metrics 路由 + RequirePermission 中间件 |
| 10 | 单元测试 | `infrastructure/otel/*_test.go` | AgentTracer、SpanProcessor、eino_interceptor 测试 |

---

## 七、与现有架构的兼容性评估

| 现有功能 | 兼容性 | 处理策略 |
|---------|--------|---------|
| MCP ToolRegistry | ✅ 完全兼容 | `PoolRegistry` 在 `ToolRegistry` 之上构建，不修改任何现有方法；`Skill` 接口通过 `ToolSkillAdapter` 包装 `ReadOnlyTool`，现有 3 个工具自动注册为技能 |
| MCP 权限体系 | ✅ 完全兼容 | `Skill.RequiredPermission()` 与现有 RBAC 对齐，复用 `PermissionChecker`；瘦工具权限与原胖工具一致 |
| MCP 胖工具 | ✅ 向后兼容 | 保留原胖工具文件作为兼容层，瘦工具独立文件实现 `ReadOnlyTool`，后续可逐步废弃胖工具 |
| asynq 任务队列 | ✅ 完全兼容 | 任务重试/清理直接复用 `infra.Queue`，handler 链式调度模式已验证 |
| freecache + Redis | ✅ 完全兼容 | 上下文缓存复用双层缓存架构 |
| Qdrant 向量数据库 | 🆕 新增依赖 | `github.com/qdrant/go-client` gRPC 客户端，P0-B 阶段引入，P1-A 复用 |
| BGE-M3 Embedding | 🆕 新增依赖 | ONNX Runtime 本地推理，1024 维，dense+sparse+colbert 三重表示，中文优化 |
| OTel SDK | ✅ 已有（indirect→direct） | `go.opentelemetry.io/otel v1.41.0`，需添加 `sdk/trace`、`sdk/metric`、`otlp/grpc` 为 direct 依赖 |
| OTel GenAI 语义约定 | 🆕 新规范 | 遵循 v1.28.0+ 稳定版 + #2912 提案（Workflow/Task span），不自建 trace 体系 |
| Wire DI | ✅ 完全兼容 | 新增 Provider 注入 wire.go |
| advisor/agentcoord 空壳 | ✅ 已完成 | 已删除空壳目录，统一到 `internal/agent/` 模块 |
| `internal/trace/` 空壳 | ✅ 将删除 | 9 个空壳文件无代码引用，功能整合到 `agent/infrastructure/otel/`（详见 §5.8） |
| 数据库 migration | ✅ 无影响 | 现有表名不变，新增 agent 相关表通过新 migration 文件添加 |

---

## 八、关键风险与缓解措施

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 意图路由关键词匹配精度不足 | 选错工具池，LLM 看到不相关工具 | P0-0 先用关键词匹配，P1 阶段升级为 Embedding 语义匹配；兜底 `general` 池确保始终有工具可用 |
| 瘦工具数量激增 | 注册表膨胀，管理复杂度上升 | 通过池分组控制暴露数量；`PoolRegistry.SelectTools()` 预算裁剪确保单次请求工具数 ≤ 8 |
| 胖工具与瘦工具并存期 | 调用方可能混淆使用 | 胖工具标记 deprecated，池注册只使用瘦工具；过渡期后移除胖工具 |
| Qdrant 集群运维 | P0-B 阶段可能延迟 | 开发环境用 Qdrant Docker 单节点，生产环境用 Qdrant Cloud 或 K8s Helm 部署 |
| BGE-M3 模型加载耗时 | 首次启动慢 | ONNX Runtime 懒加载 + 模型文件预下载到镜像；Dimension=1024 固定，无需动态配置 |
| 关键词提取压缩质量 | 压缩后丢失关键上下文 | 三级压缩配置（low/medium/high），默认 low 保留 80%；压缩后仍超预算再触发 LLM 摘要 |
| OTel SDK 性能开销 | Span 采集影响请求延迟 | 使用 BatchSpanProcessor 异步导出；压测验证 P99 延迟增加 < 5%；采样率可配置 |
| OTel GenAI 语义约定变更 | #2912 提案未稳定，字段可能调整 | 遵循 v1.28.0 稳定版核心字段，#2912 扩展字段作为 optional attributes，后续可平滑升级 |
| Span 内容泄露敏感数据 | 监控数据含用户隐私/密钥 | 配置 `OTEL_GEN_AI_CONTENT_CAPTURE=metadata`，生产环境禁止 content 事件；Reasoning 字段可配置脱敏 |
| LLM 任务拆解质量不稳定 | 任务执行可能偏离预期 | 引入 `TaskVerifier` 交付验证 + 人工审批流（`ToolCall.IsApproved` 已有模型字段） |
| 上下文窗口溢出 | 对话质量下降 | 关键词提取 + LLM 摘要双策略，Token 计数实时监控，compact API 支持手动触发 |
| 技能安全边界 | 未授权操作 | 所有技能必须声明 `IsReadOnly()` + `RequiredPermission()`，写操作需二次审批 |
| trace 整合后 Agent 角色权限循环 | Agent 读取自身 trace 可能影响决策 | Agent 角色默认拒绝 `agent:traces:read`，避免循环依赖；如需自我诊断需显式授权 |
| SpanProcessor MySQL 写入延迟 | 批量写入期间 MySQL 不可用导致 span 丢失 | 批量写入 + 重试机制；MySQL 不可用时 span 仍通过 OTLP 导出到 Tempo，不丢失实时数据 |

---

## 九、数据库新增表规划

### P0 阶段

```sql
-- 技能注册表
CREATE TABLE agent_skills (
    id          CHAR(36) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    version     VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    description TEXT,
    category    VARCHAR(100),
    parameters  JSON,
    is_readonly BOOLEAN NOT NULL DEFAULT TRUE,
    permission  VARCHAR(255),
    tags        JSON,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 上下文表
CREATE TABLE agent_contexts (
    id                CHAR(36) PRIMARY KEY,
    session_id        CHAR(36) NOT NULL,
    user_id           VARCHAR(255) NOT NULL,
    messages          JSON NOT NULL,
    metadata          JSON,
    token_count       INT NOT NULL DEFAULT 0,
    compression_level VARCHAR(20) NOT NULL DEFAULT 'none',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at        DATETIME,
    INDEX idx_session_id (session_id),
    INDEX idx_user_id (user_id),
    INDEX idx_expires_at (expires_at)
);

-- 上下文消息表（MySQL 持久化，与 Qdrant 向量索引互补）
CREATE TABLE agent_context_messages (
    id          CHAR(36) PRIMARY KEY,
    context_id  CHAR(36) NOT NULL,
    role        VARCHAR(20) NOT NULL,
    content     TEXT NOT NULL,
    tool_calls  JSON,
    tool_id     VARCHAR(255),
    token_count INT NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_context_id (context_id),
    FOREIGN KEY (context_id) REFERENCES agent_contexts(id) ON DELETE CASCADE
);

-- 任务表
CREATE TABLE agent_tasks (
    id          CHAR(36) PRIMARY KEY,
    parent_id   CHAR(36),
    session_id  CHAR(36) NOT NULL,
    description TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'pending',
    result      JSON,
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_session_id (session_id),
    INDEX idx_parent_id (parent_id),
    INDEX idx_status (status)
);

-- 工具池表（自定义池持久化）
CREATE TABLE agent_tool_pools (
    id          CHAR(36) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    keywords    JSON NOT NULL,
    max_tools   INT NOT NULL DEFAULT 8,
    is_builtin  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 工具池-工具关联表
CREATE TABLE agent_pool_tools (
    id       CHAR(36) PRIMARY KEY,
    pool_id  CHAR(36) NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    UNIQUE KEY uk_pool_tool (pool_id, tool_name),
    FOREIGN KEY (pool_id) REFERENCES agent_tool_pools(id) ON DELETE CASCADE
);

-- 调用链表（OTel Span 双写）
CREATE TABLE agent_traces (
    id              CHAR(36) PRIMARY KEY,
    session_id      CHAR(36) NOT NULL,
    trace_id        CHAR(32) NOT NULL,
    span_id         CHAR(16) NOT NULL,
    parent_span_id  CHAR(16),
    workflow_id     VARCHAR(255),
    node_id         VARCHAR(255) NOT NULL,
    node_type       VARCHAR(50) NOT NULL,
    operation       VARCHAR(50) NOT NULL,
    input           JSON,
    output          JSON,
    duration_ms     INT NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'running',
    attributes      JSON,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_session_id (session_id),
    INDEX idx_trace_id (trace_id),
    INDEX idx_span_id (span_id),
    INDEX idx_workflow_id (workflow_id),
    INDEX idx_operation (operation)
);

-- 思考过程表（Span Event 双写）
CREATE TABLE agent_thoughts (
    id          CHAR(36) PRIMARY KEY,
    session_id  CHAR(36) NOT NULL,
    trace_id    CHAR(32) NOT NULL,
    span_id     CHAR(16) NOT NULL,
    step_type   VARCHAR(50) NOT NULL,
    content     TEXT NOT NULL,
    reasoning   TEXT,
    duration_ms INT NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_session_id (session_id),
    INDEX idx_trace_id (trace_id),
    INDEX idx_span_id (span_id)
);
```

### P1 阶段

```sql
-- 记忆表
CREATE TABLE agent_memories (
    id          CHAR(36) PRIMARY KEY,
    layer       VARCHAR(20) NOT NULL,
    session_id  CHAR(36),
    content     TEXT NOT NULL,
    embedding   BLOB,
    metadata    JSON,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at  DATETIME,
    INDEX idx_layer (layer),
    INDEX idx_session_id (session_id),
    INDEX idx_expires_at (expires_at)
);

-- 会话表
CREATE TABLE agent_sessions (
    id          CHAR(36) PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    context_id  CHAR(36),
    metadata    JSON,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    ended_at    DATETIME,
    INDEX idx_user_id (user_id),
    INDEX idx_status (status)
);
```

---

## 十、API 端点规划

### P0 阶段

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/agent/pools` | 列出所有工具池 |
| GET | `/api/v1/agent/pools/:id` | 获取池详情（含工具列表） |
| POST | `/api/v1/agent/pools/select` | 根据意图选择工具 |
| POST | `/api/v1/agent/pools` | 注册自定义池 |
| DELETE | `/api/v1/agent/pools/:id` | 注销池 |
| POST | `/api/v1/agent/skills/register` | 注册自定义技能 |
| GET | `/api/v1/agent/skills` | 列出所有可用技能 |
| GET | `/api/v1/agent/skills/:name` | 获取技能详情 |
| POST | `/api/v1/agent/skills/:name/execute` | 执行技能 |
| DELETE | `/api/v1/agent/skills/:name` | 注销技能 |
| POST | `/api/v1/agent/contexts` | 创建上下文 |
| GET | `/api/v1/agent/contexts/:id` | 获取上下文 |
| POST | `/api/v1/agent/contexts/:id/messages` | 追加消息 |
| POST | `/api/v1/agent/contexts/:id/compact` | 压缩上下文（支持 level + strategy 配置） |
| GET | `/api/v1/agent/contexts/:id/relevant` | 语义检索相关消息（Qdrant 向量检索） |
| POST | `/api/v1/agent/tasks` | 创建任务 |
| GET | `/api/v1/agent/tasks/:id` | 获取任务状态 |
| POST | `/api/v1/agent/tasks/:id/retry` | 重试失败任务 |
| GET | `/api/v1/agent/traces/:session_id` | 获取会话调用链（OTel Span 列表） |
| GET | `/api/v1/agent/traces/:session_id/tree` | 获取调用链树形结构（基于 parent_span_id 构建） |
| GET | `/api/v1/agent/traces/:trace_id/waterfall` | 获取瀑布图数据（按时间排序的 span 列表） |
| GET | `/api/v1/agent/thoughts/:session_id` | 获取思考过程（Span Event 列表） |
| GET | `/api/v1/agent/metrics` | 获取可观测性指标摘要（OTel GenAI 标准指标 + 自定义指标） |
| PUT | `/api/v1/agent/pools/:id` | 更新池配置 |
| POST | `/api/v1/agent/pools/:id/tools` | 向池添加工具 |
| DELETE | `/api/v1/agent/pools/:id/tools/:tool_name` | 从池移除工具 |

### P1 阶段

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agent/sessions` | 创建会话 |
| GET | `/api/v1/agent/sessions/:id` | 获取会话 |
| POST | `/api/v1/agent/sessions/:id/chat` | 发送消息（多轮对话） |
| DELETE | `/api/v1/agent/sessions/:id` | 结束会话 |
| POST | `/api/v1/agent/rag/documents` | 上传文档 |
| POST | `/api/v1/agent/rag/query` | RAG 检索 |
| GET | `/api/v1/agent/memories/:session_id` | 获取记忆 |

---

## 十一、待确认事项

- [x] ~~向量数据库选型确认：Milvus vs Redis Search vs Qdrant~~ → ✅ 确认 Qdrant
- [x] ~~advisor/agentcoord 空壳目录是否直接删除~~ ✅ 已删除
- [x] ~~Embedding 模型选型确认：OpenAI API vs 本地模型~~ → ✅ 确认 BGE-M3 本地 ONNX Runtime
- [ ] 技能写操作是否需要人工审批流
- [ ] 上下文 Token 上限配置策略
- [ ] 任务最大拆解深度限制
- [ ] Tool Pool 意图路由策略确认：P0-0 先用关键词匹配，P1 是否升级为 Embedding 语义匹配
- [ ] 胖工具废弃时间线：瘦工具稳定后何时移除原胖工具文件
- [ ] Qdrant 部署方案确认：Docker 单节点 vs Qdrant Cloud vs K8s Helm
- [ ] BGE-M3 模型文件分发策略：镜像内置 vs 运行时下载
- [ ] 上下文压缩默认级别确认：low(80%) / medium(50%) / high(20%)
- [ ] 调用链数据保留策略：保留天数、归档方案
- [ ] OTel GenAI 内容捕获策略确认：`metadata`（仅元数据）vs `content`（含 prompt/completion），生产环境建议 metadata
- [ ] OTel 采样率确认：开发环境 100% vs 生产环境按比例采样（建议 10%-50%）
- [ ] OTel Exporter 部署方案：Tempo vs Jaeger vs 已有 Grafana Tempo 实例
- [ ] ThoughtStep Reasoning 字段脱敏规则确认：正则替换密钥/Token vs 完全不记录
- [x] ~~trace 模块是否独立开发~~ → ✅ 确认整合为 agent 子模块（§5.8），删除 `internal/trace/` 空壳
- [ ] Agent 角色是否需要 `agent:traces:read` 权限的例外场景（如 Agent 自我诊断）
- [ ] SpanProcessor MySQL 批量写入参数确认：batch size（建议 100）vs flush interval（建议 5s）
- [ ] OTel 压测基线确认：当前无 OTel 的 P50/P99/吞吐量基线数据
- [x] ~~自定义池是否需要持久化到数据库~~ → ✅ 已确认持久化（agent_tool_pools + agent_pool_tools 表）
