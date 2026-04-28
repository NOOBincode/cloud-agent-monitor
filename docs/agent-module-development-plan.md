# Agent 模块开发计划

> 版本: v1.0 | 日期: 2026-04-18 | 状态: 待审核

## 一、项目背景与现状评估

### 1.1 现有 Agent 相关模块盘点

| 模块             | 路径                     | 实际状态                                                                                        |
| -------------- | ---------------------- | ------------------------------------------------------------------------------------------- |
| **advisor**    | `internal/advisor/`    | 骨架占位 — 所有文件仅含 `package xxx` 声明，无业务逻辑。目录结构已规划（domain/infrastructure/interfaces/prompts），但零实现 |
| **agentcoord** | `internal/agentcoord/` | 骨架占位 — 同上，所有文件仅含 package 声明                                                                 |
| **mcp**        | `internal/mcp/`        | 唯一有实质实现的模块 — eino 工具注册/鉴权体系完整，3 个只读工具（topology/alerting/slo）已实现                             |

### 1.2 MCP 模块已有能力（可直接复用）

| 能力      | 实现位置                                                       | 说明                                                                           |
| ------- | ---------------------------------------------------------- | ---------------------------------------------------------------------------- |
| 工具注册体系  | `eino/registry.go`                                         | `ToolRegistry` + `ReadOnlyTool` 接口，支持注册/查询/执行                                |
| 权限鉴权    | `eino/registry.go` + `auth/permission.go`                  | `AuthzToolWrapper` + `PermissionChecker`，RBAC 四角色（viewer/editor/admin/agent） |
| Gin 中间件 | `middleware/eino_auth.go`                                  | Bearer Token 认证 + 工具列表/执行 HTTP 端点                                            |
| 3 个只读工具 | `eino/topology_tool.go`, `alerting_tool.go`, `slo_tool.go` | 完整的 `InvokableTool` 实现，含参数校验和 action 分发                                      |
| Eino 集成 | `eino/node.go`                                             | `compose.ToolsNode` 构建，与 eino Graph 体系对接                                     |

### 1.3 基础设施盘点

| 基础设施           | 状态    | 说明                                                       |
| -------------- | ----- | -------------------------------------------------------- |
| asynq 任务队列     | ✅ 生产级 | `pkg/infra/queue.go`，已在 topology 模块验证                    |
| freecache 本地缓存 | ✅ 生产级 | `pkg/infra/cache.go`，L1 热缓存                              |
| Redis 分布式缓存    | ✅ 生产级 | `go-redis/v9`，topology 模块已用                              |
| PostgreSQL + GORM | ✅ 生产级 | 完整仓储层，含 migration 体系，已切换至 PGSQL                          |
| pgvector       | ✅ 已集成 | PostgreSQL 向量扩展，替代 Qdrant，HNSW 索引支持高效语义检索               |
| Wire DI        | ✅ 生产级 | 编译时注入，wire\_gen.go 已验证                                   |
| eino 框架        | ✅ 已引入 | `github.com/cloudwego/eino v0.8.5`，含 compose/tool/schema |

### 1.4 关键结论

1. **advisor 和 agentcoord 是空壳** — 目录结构暗示了设计意图（fork/记忆/引擎/规则），但无任何实现代码
2. **MCP 工具层是唯一可用的基础** — 但它只解决了"工具调用"这一个维度
3. **5 大需求全部需要从零构建** — 技能系统、RAG、上下文工程、任务处理、架构扩展均无现有代码可复用

***

## 二、模块重命名方案：MCP → Agent

### 2.1 重命名理由

当前 `internal/mcp/` 模块的实际职责已超出 MCP（Model Context Protocol）协议范畴：

- 包含了工具注册、权限鉴权、HTTP 中间件等 Agent 通用能力
- 新增的技能系统、上下文管理、任务处理等均属于 Agent 能力层
- 统一命名为 `agent` 更准确反映模块定位，避免概念混淆

### 2.2 重命名映射

| 原路径                                       | 新路径                                         | 说明             |
| ----------------------------------------- | ------------------------------------------- | -------------- |
| `internal/mcp/`                           | `internal/agent/`                           | 模块根目录          |
| `internal/mcp/domain/`                    | `internal/agent/domain/`                    | 领域层            |
| `internal/mcp/application/`               | `internal/agent/application/`               | 应用层            |
| `internal/mcp/infrastructure/eino/`       | `internal/agent/infrastructure/eino/`       | Eino 工具集成      |
| `internal/mcp/infrastructure/auth/`       | `internal/agent/infrastructure/auth/`       | 权限鉴权           |
| `internal/mcp/infrastructure/middleware/` | `internal/agent/infrastructure/middleware/` | HTTP 中间件       |
| `internal/mcp/infrastructure/server/`     | `internal/agent/infrastructure/server/`     | MCP Server（保留） |
| `internal/mcp/interfaces/http/`           | `internal/agent/interfaces/http/`           | HTTP 接口        |
| `internal/advisor/`                       | 废弃，能力合并入 `internal/agent/`                  | 避免命名混乱         |
| `internal/agentcoord/`                    | 废弃，能力合并入 `internal/agent/`                  | 避免命名混乱         |

### 2.3 重命名影响范围

- 所有 import 路径从 `cloud-agent-monitor/internal/mcp/` 变更为 `cloud-agent-monitor/internal/agent/`
- `cmd/platform-api/wire.go` 中的相关 Provider 需更新 import
- `internal/advisor/` 和 `internal/agentcoord/` 的空壳目录删除
- 数据库 migration 无影响（现有表名不变）

***

## 三、技术选型

| 领域           | 选型                                          | 理由                                           |
| ------------ | ------------------------------------------- | -------------------------------------------- |
| LLM 编排框架     | `cloudwego/eino`（已有 v0.8.5）                 | 项目已引入，compose.Graph 支持 DAG 编排，与现有工具层天然集成     |
| Agent 工具协议   | **MCP（Model Context Protocol）**             | 领域 Agent 作为 MCP Host 消费 MCP Server 工具；eino-ext 已支持 MCP 工具注册；跨域唤起通过 `invoke_domain_agent` 实现 |
| 向量存储        | **pgvector**（PostgreSQL 扩展）                   | 统一存储引擎，减少独立中间件运维；HNSW 索引支持高效向量检索，与 PGSQL 共享连接池 |
| Embedding 模型 | OpenAI `text-embedding-3-small` / 本地 BGE-M3 | 通过 eino 的 `embedder` 接口抽象，支持多后端切换            |
| 文档解析         | `eino-contrib` 的 `loader` 组件                | 与 eino 生态一致，支持 PDF/Markdown/HTML             |
| 任务调度         | `asynq`（已有）                                 | 复用 `pkg/infra/queue.go`，任务链模式已在 topology 验证  |
| 缓存           | freecache(L1) + Redis(L2)（已有）               | 复用现有双层缓存架构，Agent 工具结果自动缓存                    |
| 持久化          | PostgreSQL + GORM（已有）                        | 复用现有仓储层和 migration 体系，已切换至 PGSQL             |

### 备选方案

| 领域        | 备选                      | 适用场景                         |
| --------- | ----------------------- | ---------------------------- |
| 向量存储     | Qdrant                  | 超 100 万向量规模时考虑独立部署，pgvector 在 50 万以内性能充足 |
| 向量存储     | Milvus                  | 云原生架构，支持混合检索，但运维复杂度高 |
| Embedding | Cohere Embed            | 多语言支持好，但需外部 API 调用           |
| MCP Server 部署 | 进程内注册（默认） | 开发和小规模部署，MCP Server 与 Agent Host 同进程 |
| MCP Server 部署 | Sidecar / 独立进程   | GPU 监控工具集就近部署到 GPU 节点 |
| MCP Server 部署 | 远程 SSE/Streamable HTTP | 集群级 MCP Server（K8S 拓扑），多 Agent 共享 |

***

## 四、模块划分与目录结构

```
internal/agent/                              ← 统一 Agent 模块（原 mcp 重命名）
├── domain/                                  ← 领域层
│   ├── tool_pool.go                         ← P0-0: 工具池领域模型（ToolPool, PoolRouter, ToolBudget）→ 将重构为 MCPServerSpec
│   ├── domain_agent.go                      ← P0: 领域 Agent 模型（DomainAgentConfig, InvokeDomainAgentSpec）
│   ├── workflow.go                          ← P0-A: 工作流领域模型（WorkflowDef, WorkflowInstance）
│   ├── skill.go                             ← P0: 技能接口规范与领域模型
│   ├── context.go                           ← P0: 上下文管理领域模型
│   ├── task.go                              ← P0: 任务领域模型（拆解/验证/重试）
│   ├── session.go                           ← P1: 会话领域模型
│   ├── memory.go                            ← P1: 记忆领域模型（两层：Working + Semantic）
│   ├── trace.go                             ← P0: 调用链与思考过程领域模型（OTel 对齐）
│   ├── tool.go                              ← 已有: 工具领域模型（保留）
│   ├── errors.go                            ← 已有: 错误定义（保留）
│   ├── repository.go                        ← 已有: 仓储接口（保留）
│   └── service.go                           ← 已有: 服务接口（保留）
├── application/                             ← 应用层
│   ├── domain_agent_service.go              ← P0: 领域 Agent 生命周期管理（注册/唤起/编排）
│   ├── skill_service.go                     ← P0: 技能注册/管理/调用
│   ├── context_service.go                   ← P0: 上下文 CRUD/裁剪/压缩/清理
│   ├── task_service.go                      ← P0: 任务拆解/执行/验证/重试
│   ├── observability_service.go             ← P0: 可观测性/可视化服务（基于 OTel）
│   ├── session_service.go                   ← P1: 会话生命周期管理
│   ├── rag_service.go                       ← P1: RAG 检索增强
│   └── service.go                           ← 已有: MCP 服务（保留）
├── infrastructure/                          ← 基础设施层
│   ├── mcp/                                 ← P0-0: MCP Server 工具集（替代原 FatTool 内嵌注册）
│   │   ├── server.go                        ← MCP Server 基础设施（进程内 stdio 注册）
│   │   ├── alerting_server.go               ← P0-0: 告警领域 MCP Server（6 个瘦工具）
│   │   ├── topology_server.go               ← P0-0: 拓扑领域 MCP Server（10 个瘦工具）
│   │   ├── slo_server.go                    ← P0-0: SLO 领域 MCP Server（5 个瘦工具）
│   │   ├── gpu_server.go                    ← P1: GPU 监控 MCP Server（预留）
│   │   ├── inference_server.go              ← P1: 推理服务 MCP Server（预留）
│   │   └── cost_server.go                   ← P1: 成本分析 MCP Server（预留）
│   ├── agent_host/                          ← P0: 领域 Agent Host（MCP Host 端）
│   │   ├── host.go                          ← DomainAgentHost 基础（MCP 工具发现 + eino Graph 构建）
│   │   ├── router.go                        ← 意图 → 领域 Agent 路由（替代原 IntentRouter）
│   │   ├── builder.go                       ← DomainAgent 构建（连接 MCP Server + 注入 system prompt + 配置 budget）
│   │   ├── invoke_tool.go                   ← invoke_domain_agent 工具实现（跨域唤起）
│   │   └── builtin/                         ← 内置领域 Agent 配置
│   │       ├── gpu_agent.go                 ← GPU 领域 Agent 配置（prompt + MCP Server 列表 + budget）
│   │       ├── k8s_agent.go                 ← K8S/拓扑领域 Agent 配置
│   │       ├── alerting_agent.go            ← 告警领域 Agent 配置
│   │       ├── slo_agent.go                 ← SLO 领域 Agent 配置
│   │       ├── diagnosis_agent.go           ← 诊断领域 Agent（跨域：告警+拓扑+K8S）
│   │       └── coordinator.go               ← P2: 协调者 Agent（全局意图路由 + 跨域编排）
│   ├── eino/                                ← 已有: Eino 工具集成（保留核心，简化）
│   │   ├── registry.go                      ← ToolRegistry + AuthzToolWrapper（保留，MCP 工具也注册到这里）
│   │   ├── pool.go                          ← PoolRegistry + IntentRouter → 逐步迁移到 agent_host/router.go
│   │   ├── pools_builtin.go                 ← 内置池配置 → 逐步迁移到 agent_host/builtin/
│   │   ├── node.go                          ← ToolsNode 构建（改为从 MCP Server 获取工具）
│   │   ├── llm.go                           ← LLM ChatModel 创建
│   │   ├── topology_tool.go                 ← 拓扑胖工具（保留兼容，逐步废弃）
│   │   ├── alerting_tool.go                 ← 告警胖工具（保留兼容，逐步废弃）
│   │   └── slo_tool.go                      ← SLO 胖工具（保留兼容，逐步废弃）
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
│   ├── context/                             ← P0: 上下文管理
│   │   ├── manager.go                       ← 上下文创建/维护/裁剪/压缩
│   │   ├── pgvector_store.go                ← P0: pgvector 向量存储（语义检索）
│   │   ├── postgres_store.go                ← P0: PostgreSQL 消息持久化（原 mysql_store 重命名）
│   │   ├── keyword_compressor.go            ← P0: 关键词提取压缩（优先级高于摘要）
│   │   ├── summary_compressor.go            ← P0: LLM 摘要压缩（仅自然语言为主的上下文触发）
│   │   └── cleaner.go                       ← 过期上下文清理（asynq 定时任务）
│   ├── task/                                ← P0: 任务处理
│   │   ├── decomposer.go                    ← 任务拆解（LLM 驱动）
│   │   ├── executor.go                      ← 任务执行器
│   │   ├── verifier.go                      ← 交付验证器
│   │   └── retry.go                         ← 失败重试策略
│   ├── rag/                                 ← P1: RAG 子系统
│   │   ├── pgvector_store.go                ← P1: pgvector 文档向量存储
│   │   ├── embedder.go                      ← P1: BGE-M3 Embedding 生成
│   │   ├── retriever.go                     ← P1: 混合检索器（向量+关键词）
│   │   ├── loader.go                        ← P1: 文档加载器
│   │   └── chunker.go                       ← P1: 文档分块器
│   ├── memory/                              ← P1: 两层记忆（Working + Semantic）
│   │   ├── working.go                       ← 工作记忆（当前对话上下文）
│   │   └── semantic.go                      ← 语义记忆（知识库/RAG，与 P1-A 对齐）
│   ├── otel/                                ← P0: OTel GenAI 可观测性
│   │   ├── tracer.go                        ← AgentTracer（Workflow/Agent/Task span 工厂）
│   │   ├── eino_interceptor.go              ← eino Graph 节点 span 注入
│   │   ├── span_processor.go                ← 自定义 SpanProcessor（PostgreSQL 双写）
│   │   └── metrics.go                       ← OTel Meter 指标注册
│   └── persistence/                         ← 持久化
│       ├── context_repository.go            ← 上下文 PostgreSQL 仓储
│       ├── task_repository.go               ← 任务 PostgreSQL 仓储
│       ├── pool_repository.go               ← 工具池 PostgreSQL 仓储
│       ├── trace_repository.go              ← 调用链 PostgreSQL 仓储
│       └── thought_repository.go            ← 思考过程 PostgreSQL 仓储
└── interfaces/                              ← 接口层
    └── http/
        ├── handler.go                       ← Agent API Handler（扩展现有）
        └── routes.go                        ← 路由注册（扩展现有）
```

***

## 五、核心接口设计

### 5.0 P0-0 — MCP 工具集架构（Tool Pool → MCP Server + 领域 Agent）

> 优先级最高，作为 P0-A 技能系统和领域 Agent 的前置依赖。解决工具数量增长后 LLM 调用错误率上升的问题，并通过领域 Agent 实现跨域诊断。

#### 5.0.1 问题分析

当前 `ToolRegistry` 是扁平注册表，所有工具平铺在一个 map 里，`IntentRouter.Route()` 胜者通吃只返回单个池：

| 问题           | 说明                                                                  |
| ------------ | ------------------------------------------------------------------- |
| Token 浪费     | 每个工具的 `ToolInfo`（含参数描述）都注入 system prompt，3 个工具尚可，30+ 工具消耗数千 Token |
| 选择干扰         | LLM 在过多工具中容易选错，尤其是功能相近的工具                                           |
| 无场景感知        | 诊断场景不需要 SLO 工具，告警场景不需要拓扑工具，但当前全部暴露                                  |
| 跨域调用受限       | IntentRouter 只返回单个池（胜者通吃），GPU Agent 发现 K8S 问题时无法调用 topology/alerting 池的工具 |
| 胖工具 JSON 转发  | `FatToolDelegator` + `mergeAction` 增加中间层，瘦工具本质只是注入 action 字段再转发 |

#### 5.0.2 架构设计 — MCP Server + 领域 Agent

**核心思路**：每个领域的工具集设计为 MCP Server，领域 Agent 作为 MCP Host 消费 MCP Server 的工具。跨域诊断通过 `invoke_domain_agent` 工具实现领域 Agent 之间的唤起。

```
┌──────────────────────────────────────────────────────────┐
│                  Coordinator Agent (协调者)                  │
│              意图路由 → 领域 Agent 唤起 → 结果整合              │
│         invoke_domain_agent 作为工具注册到 eino Graph        │
└───────────────────────┬──────────────────────────────────┘
                        │ invoke_domain_agent(domain="gpu", question="MFU下降原因")
                        ▼
┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│   GPU Domain Agent   │  │  K8S Domain Agent    │  │ Alerting Domain Agent│
│   (MCP Host)         │  │  (MCP Host)          │  │  (MCP Host)          │
│                      │  │                      │  │                      │
│  ┌────────────────┐  │  │  ┌────────────────┐  │  │  ┌────────────────┐  │
│  │ MCP Server:    │  │  │  │ MCP Server:    │  │  │  │ MCP Server:    │  │
│  │ gpu_tools      │  │  │  │ k8s_topology   │  │  │  │ alerting_tools │  │
│  │ (6 tools)      │  │  │  │ (10 tools)     │  │  │  │ (6 tools)      │  │
│  └────────────────┘  │  │  └────────────────┘  │  │  └────────────────┘  │
│                      │  │                      │  │                      │
│  invoke_domain_agent │  │  invoke_domain_agent │  │  invoke_domain_agent │
│  → 可唤起 K8S Agent  │  │  → 可唤起 Alert Agent│  │  → 可唤起 SLO Agent  │
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘

每个领域 Agent:
  - 独立 system prompt（领域专家角色）
  - 独立 MCP Server 工具集（只看到本领域 + 可调用的其他 Agent）
  - 独立 token budget
  - 独立 eino Graph 实例
  - 跨域唤起 = invoke_domain_agent tool call → 新的 LLM 对话（非 JSON 参数转发）
```

#### 5.0.3 领域模型

```go
// domain/domain_agent.go

// DomainAgentConfig 定义领域 Agent 的配置
type DomainAgentConfig struct {
    ID              string          // 领域 Agent 标识（如 "gpu", "k8s", "alerting", "slo"）
    Name            string          // 显示名称（如 "GPU 监控专家"）
    Description     string          // 用于 invoke_domain_agent 工具描述，LLM据此决定是否唤起
    SystemPrompt    string          // 领域专家 system prompt
    MCPServerIDs    []string        // 可消费的 MCP Server ID 列表
    CanInvoke       []string        // 可唤起的领域 Agent ID 列表（跨域唤起）
    MaxTokens       int             // 独立 token budget
    RequiredPermission string       // 调用此 Agent 所需权限
    Keywords        []string        // 意图路由关键词（替代原 Pool keywords）
    Priority        int             // 路由优先级
}

// DomainAgent 是运行时的领域 Agent 实例
type DomainAgent struct {
    config  DomainAgentConfig
    graph   *compose.Graph           // 独立的 eino Graph
    tools   []tool.InvokableTool     // MCP Server 工具 + invoke_domain_agent
    mcpConn []*mcp.Client            // MCP Server 连接
}

// InvokeDomainAgentSpec 定义 invoke_domain_agent 工具的参数 schema
type InvokeDomainAgentSpec struct {
    Domain    string `json:"domain"`     // 目标领域 Agent ID
    Question  string `json:"question"`   // 传递给目标 Agent 的问题描述
    Context   string `json:"context,omitempty"` // 附加上下文（如"MFU下降到45%，可能是Pod调度问题"）
}
```

```go
// domain/tool_pool.go — 保留但重构

// MCPServerSpec 定义一个 MCP Server 的规范
type MCPServerSpec struct {
    ID          string        // MCP Server 标识（如 "gpu_tools", "k8s_topology"）
    Name        string        // 显示名称
    Description string        // Server 描述
    Category    ToolCategory  // 所属领域
    Transport   MCPTransport  // 传输方式（进程内 stdio / SSE / Streamable HTTP）
    Endpoint    string        // 远程 MCP Server 地址（仅 SSE/HTTP 传输）
}

type MCPTransport string

const (
    TransportStdio           MCPTransport = "stdio"            // 进程内注册（默认）
    TransportSSE             MCPTransport = "sse"              // 远程 SSE
    TransportStreamableHTTP  MCPTransport = "streamable_http"  // 远程 HTTP（MCP 2025-03 规范）
)

// ToolPool 保留用于 MCP Server 级别的分组（每个 MCP Server = 一个工具池）
type ToolPool struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Categories  []ToolCategory `json:"categories"`
    ToolNames   []string      `json:"tool_names"`
    Keywords    []string      `json:"keywords"`
    Priority    int           `json:"priority"`
    MaxTools    int           `json:"max_tools"`
    IsBuiltin   bool          `json:"is_builtin"`
    // 🆕 MCP Server 关联
    MCPServerID string        `json:"mcp_server_id,omitempty"`  // 关联的 MCP Server ID
}

// ToolBudget 保留
type ToolBudget struct {
    MaxToolsPerRequest int `json:"max_tools_per_request"`
    MaxTokensForTools  int `json:"max_tokens_for_tools"`
}
```

#### 5.0.4 核心接口

```go
// DomainAgentHost 管理领域 Agent 的生命周期
type DomainAgentHost interface {
    RegisterAgent(config DomainAgentConfig) error
    UnregisterAgent(agentID string) error
    RouteAgent(intent string) (*DomainAgent, error)        // 意图 → 领域 Agent 路由
    GetAgent(agentID string) (*DomainAgent, bool)
    ListAgents() []*DomainAgentConfig
    InvokeAgent(ctx context.Context, agentID string, question string, contextHint string) (string, error)  // 跨域唤起
}

// MCPServerRegistry 管理 MCP Server 的注册与连接
type MCPServerRegistry interface {
    RegisterServer(spec MCPServerSpec) error
    ConnectServer(ctx context.Context, serverID string) (*mcp.Client, error)  // 建立 MCP 连接
    ListServerTools(ctx context.Context, serverID string) ([]tool.InvokableTool, error)  // 发现工具
    DisconnectServer(serverID string) error
}

// DomainAgentBuilder 构建领域 Agent 的 eino Graph
type DomainAgentBuilder interface {
    Build(ctx context.Context, config DomainAgentConfig, host DomainAgentHost) (*DomainAgent, error)
}
```

#### 5.0.5 MCP Server 工具集方案

**当前**：3 个"胖工具"，每个内部用 `switch action` 分发，通过 `FatToolDelegator` 注入 action

**演进后**：每个领域的工具集作为 MCP Server，LLM 通过 MCP 协议发现和调用工具

| 领域 | MCP Server ID | 工具数 | 瘦工具列表 |
|------|-------------|-------|---------|
| 告警 | `alerting_tools` | 6 | `alerting_list_active`, `alerting_list_history`, `alerting_stats`, `alerting_noisy`, `alerting_high_risk`, `alerting_feedback` |
| 拓扑 | `k8s_topology` | 10 | `topology_get_service_topology`, `topology_get_network_topology`, `topology_get_node`, `topology_get_upstream`, `topology_get_downstream`, `topology_analyze_impact`, `topology_find_path`, `topology_find_shortest_path`, `topology_find_anomalies`, `topology_get_stats` |
| SLO | `slo_tools` | 5 | `slo_list`, `slo_get`, `slo_get_error_budget`, `slo_get_burn_rate_alerts`, `slo_get_summary` |
| GPU（P1） | `gpu_tools` | 6 | 预留 |
| 推理（P1） | `inference_tools` | 6 | 预留 |
| 成本（P1） | `cost_tools` | 5 | 预留 |

> 保留原胖工具文件作为兼容层，MCP Server 稳定后逐步废弃胖工具和 FatToolDelegator。

#### 5.0.6 领域 Agent 配置

| Agent ID | 名称 | 消费的 MCP Server | 可唤起的 Agent | 关键词 | 优先级 |
|----------|------|----------------|-------------|--------|--------|
| `gpu` | GPU 监控专家 | `gpu_tools` | `k8s`, `alerting` | GPU, MFU, 显存, CUDA, vLLM, 推理延迟 | 10 |
| `alerting` | 告警分析专家 | `alerting_tools` | `k8s`, `slo` | 告警, 报警, alert, alarm, firing, severity | 9 |
| `k8s` | K8S/拓扑专家 | `k8s_topology` | `alerting`, `slo` | 拓扑, 依赖, Pod, Node, K8S, 调用链, down, unhealthy | 8 |
| `slo` | SLO 检查专家 | `slo_tools` | `alerting`, `k8s` | SLO, SLI, 错误预算, 燃烧率, objective | 7 |
| `diagnosis` | 诊断协调专家 | `alerting_tools`, `k8s_topology`, `slo_tools` | 所有 Agent | 诊断, 故障, 异常, diagnose, 根因, 排查 | 10 |
| `general` | 通用查询 | `alerting_tools`, `k8s_topology`, `slo_tools`（各取 2-3 个代表工具） | 所有 Agent | 概览, 状态, overview, health | 1 |

#### 5.0.7 跨域唤起机制

**核心设计**：`invoke_domain_agent` 作为工具注册到领域 Agent 的 eino Graph 中。

```go
// infrastructure/agent_host/invoke_tool.go

type InvokeDomainAgentTool struct {
    host    DomainAgentHost
    agentID string  // 被唤起的领域 Agent ID
    info    *schema.ToolInfo
}

func NewInvokeDomainAgentTool(host DomainAgentHost, targetAgent DomainAgentConfig) *InvokeDomainAgentTool {
    return &InvokeDomainAgentTool{
        host:    host,
        agentID: targetAgent.ID,
        info: &schema.ToolInfo{
            Name: fmt.Sprintf("invoke_%s_agent", targetAgent.ID),
            Desc: fmt.Sprintf("唤起%s专家来回答关于%s的问题。适用场景：%s",
                targetAgent.Name, targetAgent.Description, targetAgent.Description),
        },
    }
}

func (t *InvokeDomainAgentTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
    var spec InvokeDomainAgentSpec
    if err := json.Unmarshal([]byte(argsJSON), &spec); err != nil {
        return "", fmt.Errorf("invalid invoke_domain_agent args: %w", err)
    }

    // 跨域唤起 = 启动新的领域 Agent LLM 对话
    result, err := t.host.InvokeAgent(ctx, spec.Domain, spec.Question, spec.Context)
    if err != nil {
        return "", fmt.Errorf("domain agent %s invocation failed: %w", spec.Domain, err)
    }
    return result, nil
}
```

**交互流程示例**：

```
用户: "GPU MFU 下降到 45%，帮我诊断原因"
    ↓
[Coordinator Agent] → 路由到 GPU Domain Agent
    ↓
[GPU Agent] → 调用 gpu_tools: 查询 MFU/利用率/显存
    → GPU Agent 判断: MFU 下降可能与 K8S Pod 调度有关
    → 调用 invoke_k8s_agent(question="Pod 调度是否导致 GPU 利用率不足？", context="MFU=45%")
    ↓
[K8S Agent] → 调用 k8s_topology: 查询 Pod 分布/节点负载/调度策略
    → K8S Agent 判断: Pod 资源限制配置不当 + 节点调度不均衡
    → 调用 invoke_alerting_agent(question="GPU 节点是否有告警？", context="MFU=45%, Pod调度不均衡")
    ↓
[Alerting Agent] → 调用 alerting_tools: 查询 GPU 节点告警
    → 返回: 3 条 GPU 温度告警 + 1 条资源压力告警
    ↓
结果回传: Alerting Agent → K8S Agent → GPU Agent
    ↓
[GPU Agent 整合三份报告] → 最终回答用户
```

#### 5.0.8 MCP Server 实现方式

eino-ext 已提供 MCP 工具注册能力，领域 Agent 通过 MCP 协议消费工具集：

```go
// 进程内 MCP Server（P0 默认模式）
func NewInProcessMCPServer(spec MCPServerSpec, tools []ReadOnlyTool) *mcp.Server {
    server := mcp.NewServer(spec.ID, spec.Name, spec.Description)
    for _, t := range tools {
        server.AddTool(t)  // MCP Server 注册瘦工具
    }
    return server
}

// Domain Agent 连接 MCP Server
func (b *DomainAgentBuilder) Build(ctx context.Context, config DomainAgentConfig, host DomainAgentHost) (*DomainAgent, error) {
    // 1. 连接所有 MCP Server，发现工具
    var agentTools []tool.InvokableTool
    for _, serverID := range config.MCPServerIDs {
        mcpClient, err := b.mcpRegistry.ConnectServer(ctx, serverID)
        if err != nil {
            return nil, fmt.Errorf("failed to connect MCP server %s: %w", serverID, err)
        }
        tools, err := b.mcpRegistry.ListServerTools(ctx, serverID)
        if err != nil {
            return nil, fmt.Errorf("failed to list tools from MCP server %s: %w", serverID, err)
        }
        agentTools = append(agentTools, tools...)
    }

    // 2. 注册 invoke_domain_agent 工具（基于 CanInvoke 配置）
    for _, targetID := range config.CanInvoke {
        targetConfig, ok := host.GetAgent(targetID)
        if ok {
            invokeTool := NewInvokeDomainAgentTool(host, targetConfig)
            agentTools = append(agentTools, invokeTool)
        }
    }

    // 3. 构建 eino Graph（ChatModel + ToolsNode）
    graph := compose.NewGraph[*schema.Message, *schema.Message]()
    graph.AddChatModelNode("chat", b.chatModel)
    graph.AddToolsNode("tools", compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: agentTools}))
    // ... 添加边和条件分支

    return &DomainAgent{
        config:  config,
        graph:   graph,
        tools:   agentTools,
    }, nil
}
```

#### 5.0.9 与现有架构的兼容性与迁移

| 现有组件               | 变化                                           |
| ------------------ | -------------------------------------------- |
| `ToolRegistry`     | 保留为底层注册表，MCP 进程内 Server 的工具也注册到这里，权限鉴权不变      |
| `AuthzToolWrapper` | 不变，权限鉴权仍在工具执行层                               |
| `ReadOnlyTool` 接口  | 不变，MCP Server 内的瘦工具仍实现此接口                   |
| `IntentRouter`     | 逐步迁移到 `DomainAgentRouter`（意图 → 领域 Agent 路由） |
| `PoolRegistry`     | 逐步迁移到 `DomainAgentHost`（池 → 领域 Agent）      |
| `FatToolDelegator` | MCP Server 稳定后废弃，工具独立定义                    |
| 原胖工具               | 保留文件，标记为 deprecated，MCP Server 稳定后移除       |
| `NewToolsNodeFromPool` | 改为 `NewToolsNodeFromMCP`（从 MCP Server 获取工具） |
| `internal/trace/` 空壳 | 删除（不变）                                      |

**迁移策略**：P0-0 先实现进程内 MCP Server + 领域 Agent，与现有 ToolRegistry/PoolRegistry 共存。P1 阶段完成迁移后废弃 PoolRegistry 和 FatToolDelegator。

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

### 5.2 P0 — 上下文管理（pgvector + PostgreSQL 统一存储）

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
    Embedding  []float64     `json:"embedding,omitempty"`  // pgvector 向量
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

**pgvector 向量存储接口**：

```go
// infrastructure/context/pgvector_store.go

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
写入路径: Message → PostgreSQL(完整内容) → Embedding → pgvector(向量+payload) → freecache(L1) → Redis(L2)
读取路径: freecache(L1) → Redis(L2) → PostgreSQL(精确检索) → pgvector(语义检索)
清理路径: asynq 定时任务 → PostgreSQL 批量删除（行数据 + pgvector 索引）
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

### 5.4 P1 — 两层记忆

> **架构决策：采用两层记忆（Working + Semantic），去除 Episodic 层。**
>
> 在监控场景中，每次诊断是独立事件，不依赖上次对话的原始上下文。有价值的信息是提炼后的模式（semantic），而非对话历史本身（episodic）。
> Episodic 和 Semantic 的边界模糊 — 如果 episodic 已做摘要提炼，本质就是 semantic。
> Semantic 层直接与 RAG 子系统对齐，从历史诊断中提取模式知识，支持自学习。

```go
// domain/memory.go

type MemoryLayer string

const (
    MemoryWorking   MemoryLayer = "working"    // 当前对话上下文
    MemorySemantic  MemoryLayer = "semantic"   // 知识库/RAG（与 P1-A 对齐）
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

**两层记忆的职责划分**：

| 层 | 存储 | 写入时机 | 读取时机 | TTL |
|---|---|---|---|---|
| Working | freecache(L1) + Redis(L2) | 每轮对话消息追加 | 领域 Agent 运行时读取当前会话上下文 | 会话结束时清理 |
| Semantic | pgvector (HNSW 索引) | 诊断完成后提取模式知识（由 RAG P1-A 驱动） | 领域 Agent 检测到类似场景时检索历史模式 | 长期保留，定期归档 |

**Working → Semantic 的转化路径**：

```
领域 Agent 完成一次诊断
    → 诊断结果 + 关键决策点 → Embedding → pgvector 存入 semantic memory
    → 未来类似问题 → pgvector 语义检索 → 返回历史诊断模式 → Agent 参考经验决策
```

> 如后续发现确实需要"上次对话的原始摘要"场景（如用户延续上次未完成的诊断），可补充 episodic 层。但目前两层先行。

### 5.5 P2 — 领域 Agent 架构（MCP Host）

> **架构演进：SubAgent（Tool-like）→ Domain Agent（MCP Host）**
>
> 原 SubAgent 设计（§5.5-旧版）将 Agent 封装为 `ReadOnlyTool` 注册到父 Agent 工具池。
> 但在 AI infra + K8S 环境下，领域 Agent 需要消费独立的 MCP Server 工具集，具备独立的推理能力，
> 而非简单的 JSON 参数转发。MCP Host 模式让领域 Agent 成为独立的 MCP 协议消费者，
> 天然支持跨域唤起（通过 `invoke_domain_agent` 工具）、独立部署（Sidecar/远程 MCP Server）和关注点分离。

#### 5.5.1 核心设计：领域 Agent = MCP Host

每个领域 Agent 是一个 **MCP Host** — 它消费 MCP Server 的工具集，拥有独立的 eino Graph、system prompt 和 token budget：

```go
// domain/domain_agent.go（§5.0.3 已定义 DomainAgentConfig）

// DomainAgent 是运行时的领域 Agent 实例
type DomainAgent struct {
    config  DomainAgentConfig
    graph   compose.Runnable[*schema.Message, *schema.Message]  // 编译后的 eino Graph
    mcpConn []*mcp.Client           // MCP Server 连接列表
}

// Run 执行领域 Agent 的推理循环
func (da *DomainAgent) Run(ctx context.Context, userMessage *schema.Message) (*schema.Message, error) {
    ctx, span := otel.Tracer("cloud-agent-monitor/agent").Start(ctx,
        fmt.Sprintf("invoke_agent %s", da.config.ID),
        trace.WithAttributes(
            attribute.String("gen_ai.operation.name", "invoke_agent"),
            attribute.String("gen_ai.agent.name", da.config.ID),
        ),
    )
    defer span.End()

    return da.graph.Run(ctx, userMessage)
}
```

#### 5.5.2 预定义领域 Agent 实例

| 领域 Agent | 消费的 MCP Server | System Prompt 要点 | 可唤起 Agent | 适用场景 |
|-----------|-----------------|-------------------|-------------|---------|
| `gpu_agent` | `gpu_tools` | "你是 GPU 监控专家，分析 MFU/利用率/显存/温度，关注推理性能" | `k8s`, `alerting` | GPU MFU 下降诊断、推理延迟分析 |
| `k8s_agent` | `k8s_topology` | "你是 K8S/微服务拓扑专家，理解 Pod 调度、服务依赖、调用链" | `alerting`, `slo` | 拓扑查询、Pod 调度问题、依赖分析 |
| `alerting_agent` | `alerting_tools` | "你是告警分析专家，擅长降噪、关联分析和风险评估" | `k8s`, `slo` | 告警聚合、降噪、高风险识别 |
| `slo_agent` | `slo_tools` | "你是 SLO/容量规划专家，关注错误预算、燃烧率和容量趋势" | `alerting`, `k8s` | SLO 检查、容量评估 |
| `diagnosis_agent` | `alerting_tools` + `k8s_topology` + `slo_tools` | "你是故障诊断协调专家，必须逐步排查，不可跳过步骤" | 所有 Agent | 综合故障诊断、根因分析 |

#### 5.5.3 交互流程

```
用户提问: "GPU MFU 下降到 45%，影响了哪些推理服务？Pod 调度有问题吗？"
    ↓
[DomainAgentRouter] → 路由到 gpu Agent（关键词: GPU, MFU）
    ↓
[gpu_agent 内部执行]
    → invoke gpu_tools: 查询 MFU/利用率 → 发现 MFU=45%, GPU温度偏高
    → LLM 判断: 可能是 Pod 调度导致的资源碎片化
    → invoke_k8s_agent(question="Pod调度是否导致GPU资源碎片化？", context="MFU=45%")
    ↓
[k8s_agent 内部执行]
    → invoke k8s_topology: 查询 Pod 分布 → 发现 GPU 节点调度不均衡
    → invoke_alerting_agent(question="GPU节点是否有资源压力告警？", context="Pod调度不均衡")
    ↓
[alerting_agent 内部执行]
    → invoke alerting_tools: 查询活跃告警 → 发现 3 条 GPU 温度告警
    → 返回结构化告警报告
    ↓
结果回传: alerting_agent → k8s_agent → gpu_agent
    ↓
[gpu_agent 整合三份报告] → 最终回答用户
```

#### 5.5.4 与 SubAgent（旧版）的关键区别

| 维度 | SubAgent（旧版） | Domain Agent（MCP Host） |
|------|-------------|---------------|
| 工具来源 | 从 PoolRegistry 选工具（内存 map） | 通过 MCP 协议从 MCP Server 发现工具 |
| 跨域唤起 | JSON 参数转发（FatToolDelegator 模式） | `invoke_domain_agent` 工具 → 新的 LLM 对话 |
| 自主性 | 共享父 Agent 的部分工具 | 独立 MCP Server 工具集 + 独立 prompt |
| 部署 | 单进程 | 可分进程/Sidecar/远程 MCP Server |
| 可观测性 | TracedToolDecorator（单层 span） | `invoke_agent` span + 内部 `execute_tool` span（完整链路） |
| 记忆隔离 | 共享记忆 | 独立 working memory |
| 递归深度 | 无限制 | 配置化限制（默认 max_depth=2） |

#### 5.5.5 递归深度与安全限制

```go
// 递归深度限制，防止 Agent 无限唤起
type RecursionGuard struct {
    maxDepth int                    // 最大递归深度（默认 2）
    current  map[string]int         // 当前会话中每个 Agent 的调用次数
}

func (g *RecursionGuard) AllowInvoke(agentID string) bool {
    g.current[agentID]++
    return g.current[agentID] <= g.maxDepth
}

// 唤起链示例（depth=2）:
// gpu_agent(depth=0) → k8s_agent(depth=1) → alerting_agent(depth=2) ✅
// gpu_agent(depth=0) → k8s_agent(depth=1) → alerting_agent(depth=2) → slo_agent(depth=3) ❌ 超限
```

#### 5.5.6 PGSQL + pgvector 环境适配

- 每个 MCP Server 的工具结果自动进入 freecache(L1) + Redis(L2) 双层缓存
- 领域 Agent 共享 PGSQL 连接池（通过 `*gorm.DB` 依赖注入），不增加连接数
- pgvector 的 HNSW 索引在进程内共享，多个领域 Agent 并行查询不增加内存
- MCP Server 可就近部署：GPU 工具集 Sidecar 到 GPU 节点，K8S 工具集部署在集群管理面

### 5.5A — 跨域唤起机制（替代原多池联合选择 RouteMulti）

> **设计目标**：解决原 `IntentRouter.Route()` 胜者通吃的限制，通过领域 Agent 之间的 `invoke_domain_agent` 工具实现跨域诊断，而非工具平铺。

#### 5.5A.1 为什么不用 RouteMulti

原设计 `RouteMulti` + `SelectToolsMulti` 是请求级别的多池工具合并，但存在两个问题：

1. **Token 爆炸**：多池合并后工具数量叠加（6 + 10 + 5 = 21），回到最初的平铺问题
2. **无法表达跨域推理**：把 GPU 工具 + K8S 工具平铺给一个 LLM，LLM 不具备确定性跨域推理能力

领域 Agent 的 `invoke_domain_agent` 是**推理级别**的跨域协作 — 每个 Agent 在自己的专业域内推理，检测到跨域问题时唤起对应的专家 Agent，对方用独立 prompt + 独立工具进行专业推理。

#### 5.5A.2 invoke_domain_agent 实现

已在 §5.0.7 详细定义 `InvokeDomainAgentTool`。核心逻辑：

```
领域 Agent A 在推理过程中检测到跨域问题
    → LLM 选择 invoke_<domain>_agent 工具
    → 传入 question + context 参数
    → DomainAgentHost.InvokeAgent() 启动领域 Agent B
    → Agent B 用自己的 MCP Server 工具集推理
    → Agent B 返回结构化报告
    → Agent A 继续推理，整合 Agent B 的报告
```

#### 5.5A.3 与场景化工作流的配合

| 场景 | 使用方式 | 说明 |
|------|---------|------|
| 简单查询（单领域） | 单个领域 Agent | "查看告警" → alerting_agent → alerting MCP Server |
| 跨域诊断（有推理链） | 领域 Agent + invoke_domain_agent | "GPU MFU 下降 + K8S 问题" → gpu_agent → invoke_k8s_agent → invoke_alerting_agent |
| 复杂任务（有流程控制） | Coordinator Agent 编排多个领域 Agent | "综合故障诊断" → coordinator → 按 DAG 串行/并行唤起多个领域 Agent |
| 自由对话 | general Agent（ReAct 兜底） | 通用问题 → general Agent → 自由选择 MCP Server 工具 |

### 5.5B — TaskPlanner 模块（领域 Agent 编排）

> **设计目标**：将复杂用户问题分解为多步骤执行计划，每个步骤可以是领域 Agent 或单个工具调用，支持步骤间参数传递和结果整合。

#### 5.5B.1 定位

TaskPlanner 是 P0-C（任务处理）阶段的核心组件，也是领域 Agent 编排的关键。它负责：

1. **任务拆解**：将复杂用户问题分解为有序的子任务 DAG
2. **领域 Agent 映射**：每个子任务映射到特定领域 Agent（而非单个工具），Agent 内部自主选择工具
3. **参数映射**：定义子任务间的参数传递规则（如 Step1 的输出 → Step2 Agent 的 context 参数）
4. **执行调度**：驱动领域 Agent 的串行/并行执行
5. **结果聚合**：收集所有领域 Agent 结果并整合为最终回答

#### 5.5B.2 领域模型

```go
// domain/task.go

type TaskPlan struct {
    ID          string
    SessionID   string
    UserIntent  string
    Steps       []TaskStep
    Status      TaskPlanStatus
}

type TaskStep struct {
    ID            string
    DomainAgentID string            // 目标领域 Agent ID（替代原 PoolID）
    Question      string            // 传递给领域 Agent 的问题描述
    InputMapping  map[string]string // 参数映射：key=Agent context 参数名, value=引用表达式（如 "$step1.result.service_id"）
    DependsOn     []string          // 依赖的步骤 ID
    RetryPolicy   *RetryPolicy
    Timeout       time.Duration
}

type TaskPlanStatus string

const (
    TaskPlanPending   TaskPlanStatus = "pending"
    TaskPlanRunning   TaskPlanStatus = "running"
    TaskPlanCompleted TaskPlanStatus = "completed"
    TaskPlanFailed    TaskPlanStatus = "failed"
)
```

#### 5.5B.3 实现路径

TaskPlanner 的实现分为两个阶段：

**Phase 1（P0-C）**：LLM 驱动的任务拆解

- 用户问题 → LLM(Planning Prompt) → 生成 JSON 格式的执行计划
- 每个步骤指定 `DomainAgentID`（如 gpu, k8s, alerting）和 `Question`
- `TaskExecutor` 按计划顺序调用 `DomainAgentHost.InvokeAgent()`
- 支持 `$step1.result.xxx` 参数映射，前一步 Agent 的输出注入下一步的 context

**Phase 2（P2）**：Coordinator Agent 自动编排

- Coordinator Agent 作为 MCP Host，消费所有领域 Agent 的 invoke 工具
- Coordinator Agent 自主决定唤起顺序和参数传递
- TaskPlanner 变为 Coordinator Agent 的内置能力，而非独立模块

### 5.5C — Agent 缓存与异步任务架构

> **设计目标**：利用项目已有的 freecache(L1) + Redis(L2) 双层缓存和 asynq 任务队列，为 Agent 工具调用结果提供缓存加速和异步处理能力。

#### 5.5C.1 缓存分层策略

Agent 的每次工具调用都经过三层缓存检查，避免重复查询后端服务：

```
工具调用请求
    ↓
[L1 freecache] → 命中（~100μs）→ 直接返回
    ↓ 未命中
[L2 Redis]     → 命中（~1ms）  → 写入 L1 → 返回
    ↓ 未命中
[后端服务/PGSQL] → 查询结果 → 写入 L1 + L2 → 返回
```

**缓存 Key 设计**：

| 数据类型 | Key 格式 | TTL | 说明 |
|---------|---------|-----|------|
| 活跃告警列表 | `agent:alert:active:{namespace}` | 30s | 告警变化频繁，短 TTL |
| 告警统计 | `agent:alert:stats:{from}:{to}` | 5min | 统计数据相对稳定 |
| 服务拓扑 | `agent:topo:service:{namespace}` | 60s | 拓扑变更频率中等 |
| 服务节点 | `agent:topo:node:{service_id}` | 120s | 单节点信息稳定 |
| 影响分析 | `agent:topo:impact:{service_id}` | 60s | 依赖拓扑数据 |
| SLO 列表 | `agent:slo:list` | 120s | SLO 配置变化少 |
| SLO 详情 | `agent:slo:get:{slo_id}` | 60s | SLO 状态实时性要求高 |
| SubAgent 报告 | `agent:subagent:{agent_id}:{task_hash}` | 300s | SubAgent 整合结果缓存 |

**缓存失效策略**：

- 写操作（告警确认、SLO 修改等）自动清除相关缓存 Key
- asynq 定时任务每分钟刷新高频数据的 L1 缓存（预加载）
- L1 容量上限 100MB，仅缓存热数据；L2 容量不受限

#### 5.5C.2 asynq 异步任务应用

asynq 在 Agent 模块中承担以下异步处理职责：

| 任务类型 | Queue | 频率 | 说明 |
|---------|-------|------|------|
| 缓存预热 | `critical` | 每分钟 | 刷新 L1 热数据缓存（活跃告警、拓扑概览） |
| Embedding 生成 | `default` | 按需 | 上下文消息写入后异步生成 Embedding，写入 pgvector |
| 过期上下文清理 | `low` | 每小时 | 清理过期会话上下文、pgvector 向量 |
| 任务重试 | `default` | 按需 | TaskExecutor 失败的子任务重试 |
| SubAgent 超时监控 | `critical` | 按需 | 监控 SubAgent 执行超时，触发取消 |

**asynq 与 Agent 工具调用的配合**：

```
用户请求 → Agent 处理
    ├── 同步路径：L1 → L2 → 后端服务（实时返回）
    └── 异步路径：
        ├── Embedding 生成 → asynq enqueue → 后台 worker 写入 pgvector
        ├── 缓存预热 → asynq scheduler → 定期刷新 L1
        └── 长任务 → asynq enqueue → TaskExecutor 异步执行 → 结果写入缓存
```

#### 5.5C.3 关键配置

```yaml
# configs/config.yaml
agent:
  cache:
    l1_max_bytes: 104857600    # freecache 100MB
    default_ttl: 60s
    key_prefix: "agent:"
  async:
    embedding_queue: "default"
    cleanup_queue: "low"
    cache_warmup_interval: "60s"
    subagent_timeout: "30s"
    max_retries: 3
```

### 5.6 P0 — 可观测性与可视化（OTel GenAI 原生）

> **设计原则：不自建 trace/metrics 采集系统，直接基于 OpenTelemetry GenAI 语义约定构建可观测性。**
> 项目已有 `AISession`/`ToolCall` 模型与 otel GenAI 字段高度对齐，但采集链路完全断裂。
> 本节将自建 `TraceCollector`/`ThoughtRecorder`/`AgentMetrics` 替换为 OTel 原生方案。

#### 5.6.1 现状评估

| 层级       | 现有能力                                                  | 覆盖度    | 缺口                           |
| -------- | ----------------------------------------------------- | ------ | ---------------------------- |
| 数据模型     | `AISession` + `ToolCall` 已有完整 otel GenAI 字段           | 🟢 70% | 缺 workflow/task/embedding 层级 |
| 基础设施     | OTel SDK 已引入（go.mod: `otel v1.41.0`）                  | 🟡 30% | 仅 indirect，无 Provider 初始化    |
| Agent 模块 | `TraceCollector`/`ThoughtRecorder`/`AgentMetrics` 已设计 | 🔴 0%  | 自建方案，与 OTel 生态脱节             |
| trace 模块 | `internal/trace/` 全部空壳                                | 🔴 0%  | 9 个文件全部 `type X struct{}`    |

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

| 项目概念         | otel GenAI Span                     | 关键 Attributes                                                                    |
| ------------ | ----------------------------------- | -------------------------------------------------------------------------------- |
| 工作流执行        | `gen_ai.workflow {name}`            | `gen_ai.workflow.name`, `gen_ai.workflow.type`, `gen_ai.operation.name=workflow` |
| 技能/Agent 调用  | `invoke_agent {name}`               | `gen_ai.agent.name`, `gen_ai.operation.name=invoke_agent`                        |
| 子任务          | `gen_ai.task {name}`                | `gen_ai.task.name`, `gen_ai.task.type`, `gen_ai.task.status`                     |
| LLM 调用       | `chat {model}`                      | `gen_ai.request.model`, `gen_ai.usage.input_tokens/output_tokens`                |
| 工具调用         | `execute_tool {tool_name}`          | `gen_ai.tool.name`, `gen_ai.tool.type`, `gen_ai.tool.call.id`                    |
| Embedding 调用 | `embeddings {model}`                | `gen_ai.request.model`, `gen_ai.usage.input_tokens`                              |
| 人工审批         | Span Event `gen_ai.interrupt`       | `approval.reason`, `approval.tool`                                               |
| 上下文压缩        | Span Event `gen_ai.context.compact` | `compression.level`, `compression.ratio`                                         |

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

| 指标名                                | 类型        | 单位    | 关键 Attributes                                                        |
| ---------------------------------- | --------- | ----- | -------------------------------------------------------------------- |
| `gen_ai.client.operation.duration` | Histogram | s     | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.agent.name` |
| `gen_ai.client.token.usage`        | Histogram | token | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.token.type` |
| `gen_ai.workflow.duration`         | Histogram | s     | `gen_ai.workflow.name`, `gen_ai.workflow.type`                       |
| `gen_ai.task.duration`             | Histogram | s     | `gen_ai.task.name`, `gen_ai.task.type`                               |

**项目自定义指标（补充标准指标未覆盖的维度）：**

| 指标名                                   | 类型        | 说明             |
| ------------------------------------- | --------- | -------------- |
| `agent_tool_call_duration_seconds`    | Histogram | 工具调用耗时（按工具名分桶） |
| `agent_tool_call_errors_total`        | Counter   | 工具调用错误数        |
| `agent_context_compression_ratio`     | Gauge     | 上下文压缩率         |
| `agent_pool_selection_total`          | Counter   | 工具池选择次数（按池名）   |
| `agent_intent_route_duration_seconds` | Histogram | 意图路由耗时         |

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
    → SpanProcessor (自定义) → PostgreSQL (持久化，复用 ai_sessions/tool_calls 表)

  OTel MeterProvider
    → OTLP gRPC Exporter → Prometheus (指标采集)
```

#### 5.6.7 安全风险防范

| 风险                 | 防范措施                                                                                                 |
| ------------------ | ---------------------------------------------------------------------------------------------------- |
| Span 中泄露敏感数据       | 配置 `OTEL_GEN_AI_CONTENT_CAPTURE=metadata`（仅捕获元数据，不捕获内容）；生产环境禁止 `gen_ai.content.prompt/completion` 事件 |
| Token 指标暴露业务规模     | 指标按 role 聚合，不暴露 user\_id；Grafana 仪表盘按 RBAC 控制访问                                                      |
| Trace ID 跨模块传播泄露   | 遵循最小权限原则，trace 查询需 `agent:traces:read` 权限                                                            |
| ThoughtStep 记录推理过程 | `Reasoning` 字段脱敏处理，生产环境可配置 `record_reasoning: false`                                                 |
| OTel Exporter 端点暴露 | Exporter 仅监听内部网络，NetworkPolicy 限制访问                                                                  |

#### 5.6.8 与现有架构的兼容性

| 现有功能                          | 兼容性    | 处理策略                                                                        |
| ----------------------------- | ------ | --------------------------------------------------------------------------- |
| `AISession`/`ToolCall` 模型     | ✅ 完全兼容 | OTel Span → PostgreSQL 双写，复用现有字段（trace\_id/span\_id/parent\_span\_id/gen\_ai.\*） |
| `internal/trace/` 空壳          | ✅ 将删除  | 9 个空壳文件无代码引用，功能整合到 `agent/infrastructure/otel/`（详见 §5.8）                    |
| `go.opentelemetry.io/otel` 依赖 | ✅ 已有   | 从 indirect 升级为 direct，添加 `sdk/trace`、`sdk/metric`、`otlp/grpc`               |
| `otelhttp` 中间件                | ✅ 已有   | `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` 已在 go.mod   |
| Prometheus 指标                 | ✅ 完全兼容 | OTel Meter → OTLP → Prometheus，自定义指标保持 `agent_` 前缀                          |
| eino Graph 执行                 | ✅ 无侵入  | span 仅作为 side-effect，不改变 Graph 编译/运行结果                                      |
| freecache                     | ✅ 无冲突  | trace 数据使用独立缓存 key 前缀，不超总容量 10%                                             |

### 5.7 P0-A — 场景化工作流（Workflow）

> 基于 eino `compose.Graph` 构建场景化预定义工作流，与 Tool Pool 天然配合。每个场景池对应一个工作流，池中的工具就是该工作流可用的工具集。

#### 5.6.1 设计理念

采用**方案 B：场景化预定义工作流**，而非简单 ReAct 或动态编排：

| 方案               | 说明                        | 选择            |
| ---------------- | ------------------------- | ------------- |
| A. 简单 ReAct      | LLM 自由选工具，无流程控制           | ❌ 复杂任务容易跑偏    |
| **B. 场景化预定义工作流** | **意图路由 → 预定义 DAG → 受控执行** | ✅ **采用**      |
| C. 动态编排          | LLM 生成工作流描述 → 解析为 Graph   | ❌ 质量不稳定，安全风险高 |

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

**2. 告警分析工作流 (alert\_analyze)**

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

**3. 容量规划工作流 (capacity\_check)**

```
用户询问容量/SLO
    → [ChatModel: 提取关注的 SLO/服务]
    → [ToolsNode: 查询 SLO 列表 (slo_list)]
    → [ToolsNode: 错误预算 (slo_get_error_budget)]
    → [ToolsNode: 燃烧率 (slo_get_burn_rate_alerts)]
    → [ChatModel: 生成容量评估报告]
```

**4. 自由对话工作流 (free\_chat)** — ReAct 兜底

```
用户提问
    → [ChatModel + ToolsNode: 标准 ReAct 循环]
        LLM 自由选择 general 池中的工具
```

#### 5.6.6 与 Tool Pool 的集成

| Tool Pool  | 对应工作流 | 工具来源                                        |
| ---------- | ----- | ------------------------------------------- |
| `diagnose` | 故障诊断  | `PoolRegistry.SelectTools(ctx, "diagnose")` |
| `alert`    | 告警分析  | `PoolRegistry.SelectTools(ctx, "alert")`    |
| `capacity` | 容量规划  | `PoolRegistry.SelectTools(ctx, "capacity")` |
| `general`  | 自由对话  | `PoolRegistry.SelectTools(ctx, "general")`  |

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

| 阶段                 | 工作流能力            | 说明                                                                  |
| ------------------ | ---------------- | ------------------------------------------------------------------- |
| **P0-0** Tool Pool | 无工作流             | 保持 ReAct，但池分组为工作流奠定基础                                               |
| **P0-A** 技能系统      | 技能 = 工作流         | `Skill.Execute()` 内部调用 `WorkflowBuilder.Build().Run()`，技能即预定义工作流的封装 |
| **P0-C** 任务处理      | 子任务→工作流节点        | `TaskDecomposer` 拆解的子任务映射到工作流步骤，`TaskExecutor` 驱动 Graph 执行          |
| **P1-C** 会话管理      | 多轮对话=多次 Graph 执行 | 会话绑定工作流实例，通过 `CheckPointStore` 实现断点续跑                               |
| **P2** SubAgent    | SubAgent=Tool-like Agent | 将 SubAgent 封装为 ReadOnlyTool，独立 Graph + 独立 prompt + 独立工具子集             |

### 5.8 Trace 整合架构方案

> **核心决策：删除** **`internal/trace/`** **空壳模块，将 trace 功能整合为** **`internal/agent/infrastructure/otel/`** **子模块。**
> 当前 `internal/trace/` 的 9 个文件全部是 `type X struct{}` 空壳，从未有过实现，不存在数据迁移或业务连续性风险。

#### 5.8.1 整合理由

| 维度   | 独立 trace 模块                 | 整合为 agent 子模块                                   |
| ---- | --------------------------- | ----------------------------------------------- |
| 数据主体 | trace 数据的主体是 Agent 行为       | ✅ 与 Agent 紧耦合，放在一起减少跨模块依赖                       |
| 采集时机 | Agent 运行时才产生 span           | ✅ Agent 代码直接调用 AgentTracer，无需跨模块调用              |
| 查询场景 | 人类查看 Agent 行为               | ✅ 通过 Agent API 统一暴露，权限统一管理                      |
| 代码复用 | AgentTrace 与 AISession 字段重叠 | ✅ SpanProcessor 直接复用 ai\_sessions/tool\_calls 表 |
| 维护成本 | 两个模块各自维护 trace 逻辑           | ✅ 一套 OTel 基础设施，维护成本减半                           |

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
│   │   │   ├── span_processor.go     OTel Span → PostgreSQL 双写
│   │   │   └── metrics.go            OTel Meter 指标注册
│   │   │
│   │   ├── auth/                 ✅ 已有（保留 + 扩展权限）
│   │   │   ├── permission.go         ✅ + 🆕 agent:traces:read 等权限
│   │   │   └── provider.go           ✅ 保留
│   │   │
│   │   ├── middleware/           ✅ 已有（保留 + 扩展）
│   │   │   └── eino_auth.go          ✅ + otelhttp 注入
│   │   │
│   │   ├── persistence/         🆕 PostgreSQL 持久化
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
                                → 自定义 SpanProcessor → PostgreSQL agent_traces/agent_thoughts
```

**读取路径（人类查询可观测数据）：**

```
1. GET /api/v1/agent/traces/:session_id → authMiddleware + casbin 权限检查
2. ObservabilityService.GetSessionTraces() → TraceRepository → PostgreSQL
3. GET /api/v1/agent/traces/:trace_id/waterfall → 按 parent_span_id 构建瀑布图
4. GET /api/v1/agent/thoughts/:session_id → PostgreSQL agent_thoughts
5. GET /api/v1/agent/metrics → Prometheus HTTP API → gen_ai.* 指标
```

#### 5.8.4 功能完整性对照

| 原 trace 空壳文件                      | 原设计意图      | 整合后实现位置                                      | 覆盖                        |
| --------------------------------- | ---------- | -------------------------------------------- | ------------------------- |
| `domain/trace.go`                 | Trace 数据模型 | `agent/domain/trace.go`                      | ✅ AgentTrace（含 OTel 字段）   |
| `domain/span.go`                  | Span 数据模型  | `agent/domain/trace.go`                      | ✅ AgentTrace 本身就是 span 模型 |
| `domain/service_dependency.go`    | 服务依赖关系     | `topology/domain/`                           | ✅ 已在 topology 模块完整实现      |
| `application/service.go`          | Trace 查询服务 | `agent/application/observability_service.go` | ✅                         |
| `infrastructure/otlp_receiver.go` | OTLP 数据接收  | `pkg/infra/otel.go` + OTel SDK               | ✅ OTel Provider 自动接收      |
| `infrastructure/jaeger_client.go` | Jaeger 查询  | OTel SDK → Tempo/Jaeger                      | ✅ OTLP Exporter 替代        |
| `infrastructure/tempo_client.go`  | Tempo 查询   | OTel SDK → Tempo                             | ✅ OTLP Exporter 替代        |
| `interfaces/http/handler.go`      | HTTP API   | `agent/interfaces/http/handler.go`           | ✅ 统一到 Agent API           |
| `interfaces/http/routes.go`       | 路由注册       | `agent/interfaces/http/routes.go`            | ✅ 统一到 Agent 路由            |

#### 5.8.5 人类操作权限设计

**新增权限常量（扩展** **`infrastructure/auth/permission.go`）：**

| 权限                    | 说明            | viewer | editor | admin | agent |
| --------------------- | ------------- | ------ | ------ | ----- | ----- |
| `agent:read`          | Agent 对话/任务查询 | ✅      | ✅      | ✅     | ✅     |
| `agent:write`         | Agent 对话/任务操作 | ❌      | ✅      | ✅     | ✅     |
| `agent:traces:read`   | 调用链/思考过程查询    | ✅      | ✅      | ✅     | ❌     |
| `agent:traces:export` | 调用链导出         | ❌      | ❌      | ✅     | ❌     |
| `agent:metrics:read`  | 指标查询          | ✅      | ✅      | ✅     | ❌     |

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

| 数据             | 存储位置                   | 写入方           | 读取方                  | 访问权限                |
| -------------- | ---------------------- | ------------- | -------------------- | ------------------- |
| OTel Span（实时）  | Tempo/Jaeger           | OTLP Exporter | Grafana              | Tempo 数据源权限         |
| OTel Span（持久化） | PostgreSQL `agent_traces`   | SpanProcessor | ObservabilityService | `agent:traces:read` |
| 思考过程           | PostgreSQL `agent_thoughts` | SpanProcessor | ObservabilityService | `agent:traces:read` |
| LLM 调用详情       | PostgreSQL `ai_sessions`    | aiinfra 模块    | aiinfra API          | `aiinfra:read`      |
| 工具调用详情         | PostgreSQL `tool_calls`     | aiinfra 模块    | aiinfra API          | `aiinfra:read`      |
| OTel 指标        | Prometheus             | OTel Meter    | Grafana              | Prometheus 数据源权限    |

**跨表关联键：**

| 关联                              | 共享键        | 关系                                    |
| ------------------------------- | ---------- | ------------------------------------- |
| agent\_traces ↔ ai\_sessions    | `trace_id` | 一个 workflow 包含多个 LLM 调用               |
| agent\_traces ↔ tool\_calls     | `span_id`  | 一个 execute\_tool span 对应一条 tool\_call |
| agent\_thoughts ↔ agent\_traces | `span_id`  | 思考过程附加到具体 span                        |

**隔离原则：**

| 原则        | 实现                                                                  |
| --------- | ------------------------------------------------------------------- |
| 写入隔离      | Agent 运行时写入 OTel Span → SpanProcessor → PostgreSQL；人类只读                  |
| 连接池隔离     | SpanProcessor 使用独立 PostgreSQL 连接池（max 5 conn）                            |
| 缓存隔离      | trace 查询使用独立 freecache key 前缀 `agent:trace:*`，不超总容量 10%             |
| Schema 隔离 | agent\_traces/agent\_thoughts 是独立表，不修改 ai\_sessions/tool\_calls 表结构 |

#### 5.8.7 性能影响评估

| 影响点          | 评估                                  | 缓解措施                                   |
| ------------ | ----------------------------------- | -------------------------------------- |
| OTel span 创建 | 每次 `tracer.Start()` 约 1-5μs         | BatchSpanProcessor 异步导出                |
| PostgreSQL 双写     | 每次 span.End() 触发 INSERT             | SpanProcessor 批量写入（每 100 条或每 5s flush） |
| HTTP 延迟      | otelhttp 中间件增加约 0.1ms               | 可忽略                                    |
| 内存占用         | BatchSpanProcessor 缓存未导出 span       | 限制 batch size 512，超限强制 flush           |
| PostgreSQL 存储     | 每次对话约 5-20 条 trace + 3-10 条 thought | 30 天保留 + 定期归档                          |

**压测验证目标：**

| 指标     | 基线（无 OTel） | 目标（有 OTel） | 可接受范围 |
| ------ | ---------- | ---------- | ----- |
| P50 延迟 | 200ms      | 210ms      | +5%   |
| P99 延迟 | 500ms      | 525ms      | +5%   |
| 吞吐量    | 1000 req/s | 950 req/s  | -5%   |

#### 5.8.8 迁移计划

由于 `internal/trace/` 全部是空壳，**不存在数据迁移**。迁移仅涉及代码清理：

| 步骤 | 操作                                                           | 风险               |
| -- | ------------------------------------------------------------ | ---------------- |
| 1  | 删除 `internal/trace/` 全部 9 个文件                                | 零风险（无代码引用这些文件）   |
| 2  | 在 `internal/agent/infrastructure/otel/` 创建 OTel 实现           | 零风险（新增代码）        |
| 3  | 在 `internal/agent/domain/trace.go` 定义 AgentTrace/ThoughtStep | 零风险（新增代码）        |
| 4  | 在 `cmd/platform-api/wire.go` 添加 OTel Provider + Agent 路由注册   | 低风险（需验证 Wire 编译） |
| 5  | 在 `pkg/config/config.go` 添加 OTelConfig                       | 零风险（新增配置）        |
| 6  | 在 `infrastructure/auth/permission.go` 添加 trace 相关权限          | 零风险（新增常量）        |

#### 5.8.9 维护与升级机制

| 维护维度 | 隔离策略                                                           |
| ---- | -------------------------------------------------------------- |
| 代码隔离 | `infrastructure/otel/` 独立包，仅通过 `AgentTracer` 接口暴露              |
| 配置隔离 | `config.OTelConfig` 独立配置段                                      |
| 依赖隔离 | OTel SDK 依赖仅在 `infrastructure/otel/` 和 `pkg/infra/otel.go` 中引用 |
| 测试隔离 | `infrastructure/otel/*_test.go` 使用 `tracetest` 独立测试            |

**OTel GenAI 语义约定升级策略：**

| 场景              | 处理方式                                     |
| --------------- | ---------------------------------------- |
| v1.28.0 稳定版字段变更 | 同步更新 `AgentTracer` 的 attribute key       |
| #2912 提案字段稳定化   | optional → required，添加新字段到 `AgentTracer` |
| 新增 operation 类型 | 在 `AgentTracer` 中添加新 `StartXxxSpan` 方法   |
| 废弃 operation 类型 | 标记 deprecated，保留兼容性至少一个大版本               |

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

| 阶段          | 优先级 | 内容                          | 依赖                      | 交付物                                                                                                                         |
| ----------- | --- | --------------------------- | ----------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| **P0-0**    | 最高  | **MCP Tool Pool**           | 无                       | `ToolPool`/`PoolRegistry`/`IntentRouter`、21 个瘦工具、5 个内置池、池 CRUD API + PostgreSQL 持久化                                              |
| **P0-D-前置** | 最高  | **OTel 基础设施**（原 trace 模块整合） | 无                       | 删除 `internal/trace/` 空壳、OTel Provider 初始化、OTelConfig、otelhttp 中间件、go.mod 依赖升级、权限常量扩展                                        |
| **P0-A**    | 最高  | 技能系统 + 场景化工作流               | P0-0                    | `Skill` 接口、`SkillRegistry`、参数验证器、`ToolSkillAdapter`、3 个内置技能、4 个预定义工作流、`WorkflowBuilder`/`WorkflowRouter`/`WorkflowRegistry` |
| **P0-B**    | 最高  | 上下文管理（pgvector + PostgreSQL）       | pgvector（PGSQL 扩展）、BGE-M3、asynq     | `ContextManager`、pgvector 向量存储、PostgreSQL 消息持久化、关键词提取压缩、compact API、用户绑定与数据隔离                                                      |
| **P0-C**    | 最高  | 任务处理                        | eino（已有）、asynq（已有）      | `TaskDecomposer`、`TaskExecutor`、`TaskVerifier`、`RetryPolicy`、任务状态跟踪                                                         |
| **P0-D**    | 最高  | 可观测性与可视化（OTel GenAI 原生）     | P0-D-前置                 | `AgentTracer`、eino span 注入、SpanProcessor PostgreSQL 双写、调用链/思考过程可视化 API、OTel GenAI 标准指标                                           |
| **P1-A**    | 高   | RAG 子系统                     | pgvector（P0-B 已集成）、BGE-M3 | pgvector 文档向量存储、Embedding 生成、文档加载/分块、混合检索                                                                                     |
| **P1-B**    | 高   | 三层记忆                        | P1-A、Redis（已有）          | 工作记忆、情景记忆、语义记忆                                                                                                              |
| **P1-C**    | 高   | 会话管理                        | P0-A/B/C/D              | 会话生命周期、多轮对话编排、工作流实例绑定、CheckPointStore 断点续跑                                                                                  |
| **P2**      | 中   | SubAgent 架构（替代原 Fork）      | P1 全部                   | Tool-like SubAgent、预定义专家 Agent、独立 Graph + prompt + 工具子集、TaskPlanner 编排          |

### 6.2 P0 阶段详细任务拆解

#### P0-D-前置：OTel 基础设施（原 trace 模块整合）

| # | 任务                      | 文件                                  | 说明                                                                                                               |
| - | ----------------------- | ----------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| 1 | 删除 `internal/trace/` 空壳 | `internal/trace/`                   | 删除全部 9 个空壳文件（无代码引用，零风险）                                                                                          |
| 2 | OTel Provider 初始化       | `pkg/infra/otel.go`                 | TracerProvider + MeterProvider + OTLP gRPC Exporter + 优雅关闭                                                       |
| 3 | OTel 配置                 | `pkg/config/config.go`              | Config 新增 OTelConfig（Endpoint、采样率、内容捕获策略）                                                                        |
| 4 | HTTP 入口注入               | `middleware/eino_auth.go`           | 添加 otelhttp 中间件，请求入口自动注入 trace context                                                                           |
| 5 | go.mod 依赖升级             | `go.mod`                            | otel 从 indirect → direct，添加 `sdk/trace`、`sdk/metric`、`otlp/grpc`                                                 |
| 6 | 权限常量扩展                  | `infrastructure/auth/permission.go` | 新增 `agent:read`、`agent:write`、`agent:traces:read`、`agent:traces:export`、`agent:metrics:read`                     |
| 7 | Wire 注入变更               | `cmd/platform-api/wire.go`          | 新增 ProvideOTelShutdown、ProvideAgentTracer、ProvideTraceRepository、ProvideObservabilityService、ProvideAgentHandler |

#### P0-0：MCP Tool Pool（工具池）

| #  | 任务              | 文件                                     | 说明                                               |
| -- | --------------- | -------------------------------------- | ------------------------------------------------ |
| 1  | 定义工具池领域模型       | `domain/tool_pool.go`                  | ToolPool、IntentRouter、ToolBudget、PoolRegistry 接口 |
| 2  | 实现 PoolRegistry | `infrastructure/eino/pool.go`          | 池注册/注销/选择/列表，复用 ToolRegistry                     |
| 3  | 实现 IntentRouter | `infrastructure/eino/pool.go`          | 关键词分词 → 池匹配 → 优先级排序 → 预算裁剪；新增 `RouteMulti` 支持多池联合选择（§5.5A） |
| 4  | 拆分 topology 胖工具 | `infrastructure/eino/topology_*.go`    | 10 个独立瘦工具，每个实现 ReadOnlyTool                      |
| 5  | 拆分 alerting 胖工具 | `infrastructure/eino/alerting_*.go`    | 6 个独立瘦工具                                         |
| 6  | 拆分 slo 胖工具      | `infrastructure/eino/slo_*.go`         | 5 个独立瘦工具                                         |
| 7  | 注册内置池           | `infrastructure/eino/pools_builtin.go` | diagnose/alert/topology/capacity/general 5 个池    |
| 8  | 更新 node 构建      | `infrastructure/eino/node.go`          | NewToolsNodeWithAuth 改为从池选择工具                    |
| 9  | 池管理 HTTP API    | `interfaces/http/handler.go`           | 池列表/详情/选择/注册/注销端点                                |
| 10 | 单元测试            | `infrastructure/eino/pool_test.go`     | PoolRegistry、IntentRouter、ToolBudget 测试          |

#### P0-A：技能系统

| # | 任务                  | 文件                                               | 说明                                               |
| - | ------------------- | ------------------------------------------------ | ------------------------------------------------ |
| 1 | 定义 Skill 领域模型       | `domain/skill.go`                                | Skill 接口、SkillInfo、ParamSpec、SkillResult         |
| 2 | 实现 SkillRegistry    | `infrastructure/skills/registry.go`              | 注册/查询/执行/列表，复用 auth 权限体系                         |
| 3 | 实现参数验证器             | `infrastructure/skills/validator.go`             | 类型检查、必填校验、枚举值校验、默认值填充                            |
| 4 | 实现 ToolSkillAdapter | `infrastructure/skills/adapter.go`               | 将瘦工具（ReadOnlyTool）包装为 Skill，通过 PoolRegistry 获取工具 |
| 5 | 实现 K8s 诊断技能         | `infrastructure/skills/builtin/k8s_diagnose.go`  | 组合 topology + alerting 工具的高级技能                   |
| 6 | 实现告警分析技能            | `infrastructure/skills/builtin/alert_analyze.go` | 告警聚合/关联/降噪分析                                     |
| 7 | 实现 SLO 检查技能         | `infrastructure/skills/builtin/slo_check.go`     | SLO 状态/错误预算/燃烧率综合检查                              |
| 8 | 技能 HTTP API         | `interfaces/http/handler.go`                     | 注册/列表/执行/详情端点                                    |
| 9 | 单元测试                | `infrastructure/skills/*_test.go`                | Registry、Validator、Adapter 测试                    |

#### P0-B：上下文管理（pgvector + PostgreSQL）

| #  | 任务                | 文件                                                 | 说明                                                              |
| -- | ----------------- | -------------------------------------------------- | --------------------------------------------------------------- |
| 1  | 定义上下文领域模型         | `domain/context.go`                                | ConversationContext、Message、CompressionConfig、CompressionResult |
| 2  | pgvector 基础设施       | `pkg/infra/pgvector.go`                              | pgvector 扩展启用、HNSW 索引创建、向量 Upsert/Search/Delete      |
| 3  | Embedding 基础设施    | `pkg/infra/embedding.go`                           | BGE-M3 Embedding 封装，Embed/EmbedBatch/Dimension                  |
| 4  | pgvector 配置         | `pkg/config/config.go`                             | Config 新增 PgvectorConfig、EmbeddingConfig                          |
| 5  | 实现 pgvector 向量存储    | `infrastructure/context/pgvector_store.go`          | VectorStore 接口实现，HNSW 索引 + 向量存储与语义检索                                    |
| 6  | 实现 PostgreSQL 消息持久化    | `infrastructure/context/mysql_store.go`            | 消息 CRUD，用户 ID 绑定与数据隔离                                           |
| 7  | 实现 ContextManager | `infrastructure/context/manager.go`                | 双层存储编排：PostgreSQL → Embedding → pgvector → freecache(L1) → Redis(L2)                   |
| 8  | 实现关键词提取压缩         | `infrastructure/context/keyword_compressor.go`     | TF-IDF + TextRank 关键词提取，三级压缩配置                                  |
| 9  | 实现摘要压缩            | `infrastructure/context/summary_compressor.go`     | LLM 驱动摘要，关键词压缩后仍超预算时触发                                          |
| 10 | 实现 compact API    | `interfaces/http/handler.go`                       | POST /contexts/:id/compact，支持 level + strategy 配置               |
| 11 | 实现过期清理            | `infrastructure/context/cleaner.go`                | asynq 定时任务，PostgreSQL 批量删除                                  |
| 12 | 上下文持久化            | `infrastructure/persistence/context_repository.go` | GORM 仓储实现                                                       |
| 13 | 单元测试              | `infrastructure/context/*_test.go`                 | Manager、Compressor、PgvectorStore 测试                               |

#### P0-C：任务处理

| # | 任务          | 文件                                              | 说明                                                      |
| - | ----------- | ----------------------------------------------- | ------------------------------------------------------- |
| 1 | 定义任务领域模型    | `domain/task.go`                                | Task、TaskPlan、TaskStep、TaskPlanStatus、TaskDecomposer、TaskVerifier、RetryPolicy |
| 2 | 实现任务拆解      | `infrastructure/task/decomposer.go`             | LLM 驱动的任务分解，生成 TaskPlan（含子任务 DAG 和参数映射）                          |
| 3 | 实现任务执行器     | `infrastructure/task/executor.go`               | 子任务调度、技能调用、结果收集                                         |
| 4 | 实现交付验证器     | `infrastructure/task/verifier.go`               | LLM 驱动的结果质量评估                                           |
| 5 | 实现重试策略      | `infrastructure/task/retry.go`                  | 指数退避 + 可配置重试条件                                          |
| 6 | 任务 asynq 集成 | `infrastructure/task/queue_handlers.go`         | 任务执行/重试的 asynq handler                                  |
| 7 | 任务持久化       | `infrastructure/persistence/task_repository.go` | GORM 仓储实现                                               |
| 8 | 单元测试        | `infrastructure/task/*_test.go`                 | Decomposer、Executor、Verifier 测试                         |

#### P0-D：可观测性与可视化（OTel GenAI 原生）

> 前置条件：P0-D-前置 已完成（OTel Provider、OTelConfig、otelhttp、权限常量、Wire 注入）

| #  | 任务               | 文件                                                 | 说明                                                                                                                    |
| -- | ---------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| 1  | 定义调用链领域模型        | `domain/trace.go`                                  | AgentTrace（含 TraceID/SpanID/Operation）、ThoughtStep（含 SpanID 关联）                                                       |
| 2  | 实现 AgentTracer   | `infrastructure/otel/tracer.go`                    | StartWorkflowSpan/StartAgentSpan/StartTaskSpan/RecordThought/RecordInterrupt/RecordCompact                            |
| 3  | 实现 eino 拦截器      | `infrastructure/otel/eino_interceptor.go`          | TracedLambda 包装器，eino Graph 节点自动注入 span                                                                               |
| 4  | 实现 SpanProcessor | `infrastructure/otel/span_processor.go`            | 自定义 SpanProcessor，OTel Span → PostgreSQL 双写（复用 ai\_sessions/tool\_calls 表）                                                 |
| 5  | 实现 OTel 指标       | `infrastructure/otel/metrics.go`                   | gen\_ai.client.token.usage、gen\_ai.client.operation.duration、gen\_ai.workflow\.duration、gen\_ai.task.duration + 自定义指标 |
| 6  | 调用链持久化           | `infrastructure/persistence/trace_repository.go`   | GORM 仓储实现                                                                                                             |
| 7  | 思考过程持久化          | `infrastructure/persistence/thought_repository.go` | GORM 仓储实现                                                                                                             |
| 8  | 可视化 HTTP API     | `interfaces/http/handler.go`                       | 调用链/思考过程/指标摘要端点 + 瀑布图端点                                                                                               |
| 9  | Agent 路由注册       | `interfaces/http/routes.go`                        | /agent/traces、/agent/thoughts、/agent/metrics 路由 + RequirePermission 中间件                                               |
| 10 | 单元测试             | `infrastructure/otel/*_test.go`                    | AgentTracer、SpanProcessor、eino\_interceptor 测试                                                                        |

***

## 七、与现有架构的兼容性评估

| 现有功能                  | 兼容性                   | 处理策略                                                                                                             |
| --------------------- | --------------------- | ---------------------------------------------------------------------------------------------------------------- |
| MCP ToolRegistry      | ✅ 完全兼容                | `PoolRegistry` 在 `ToolRegistry` 之上构建，不修改任何现有方法；`Skill` 接口通过 `ToolSkillAdapter` 包装 `ReadOnlyTool`，现有 3 个工具自动注册为技能 |
| MCP 权限体系              | ✅ 完全兼容                | `Skill.RequiredPermission()` 与现有 RBAC 对齐，复用 `PermissionChecker`；瘦工具权限与原胖工具一致                                     |
| MCP 胖工具               | ✅ 向后兼容                | 保留原胖工具文件作为兼容层，瘦工具独立文件实现 `ReadOnlyTool`，后续可逐步废弃胖工具                                                                |
| asynq 任务队列            | ✅ 完全兼容                | 任务重试/清理直接复用 `infra.Queue`，handler 链式调度模式已验证                                                                      |
| freecache + Redis     | ✅ 完全兼容                | 上下文缓存复用双层缓存架构                                                                                                    |
| pgvector 扩展            | ✅ PGSQL 内置               | PostgreSQL 原生扩展 `CREATE EXTENSION vector`，无需独立部署，P0-B 阶段启用，P1-A 复用                                                         |
| BGE-M3 Embedding      | 🆕 新增依赖               | ONNX Runtime 本地推理，1024 维，dense+sparse+colbert 三重表示，中文优化                                                          |
| OTel SDK              | ✅ 已有（indirect→direct） | `go.opentelemetry.io/otel v1.41.0`，需添加 `sdk/trace`、`sdk/metric`、`otlp/grpc` 为 direct 依赖                          |
| OTel GenAI 语义约定       | 🆕 新规范                | 遵循 v1.28.0+ 稳定版 + #2912 提案（Workflow/Task span），不自建 trace 体系                                                      |
| Wire DI               | ✅ 完全兼容                | 新增 Provider 注入 wire.go                                                                                           |
| advisor/agentcoord 空壳 | ✅ 已完成                 | 已删除空壳目录，统一到 `internal/agent/` 模块                                                                                 |
| `internal/trace/` 空壳  | ✅ 将删除                 | 9 个空壳文件无代码引用，功能整合到 `agent/infrastructure/otel/`（详见 §5.8）                                                         |
| 数据库 migration         | ✅ 无影响                 | 现有表名不变，新增 agent 相关表通过新 migration 文件添加                                                                            |

***

## 八、关键风险与缓解措施

| 风险                       | 影响                         | 缓解措施                                                                          |
| ------------------------ | -------------------------- | ----------------------------------------------------------------------------- |
| 意图路由关键词匹配精度不足            | 选错工具池，LLM 看到不相关工具          | P0-0 先用关键词匹配，P1 阶段升级为 Embedding 语义匹配；兜底 `general` 池确保始终有工具可用                  |
| 瘦工具数量激增                  | 注册表膨胀，管理复杂度上升              | 通过池分组控制暴露数量；`PoolRegistry.SelectTools()` 预算裁剪确保单次请求工具数 ≤ 8                    |
| 胖工具与瘦工具并存期               | 调用方可能混淆使用                  | 胖工具标记 deprecated，池注册只使用瘦工具；过渡期后移除胖工具                                          |
| pgvector 索引性能调优              | HNSW 参数选择影响查询精度           | 开发环境用默认参数（m=16, ef_construction=64），生产环境根据数据规模调优                      |
| BGE-M3 模型加载耗时            | 首次启动慢                      | ONNX Runtime 懒加载 + 模型文件预下载到镜像；Dimension=1024 固定，无需动态配置                        |
| 关键词提取压缩质量                | 压缩后丢失关键上下文                 | 三级压缩配置（low/medium/high），默认 low 保留 80%；压缩后仍超预算再触发 LLM 摘要                       |
| OTel SDK 性能开销            | Span 采集影响请求延迟              | 使用 BatchSpanProcessor 异步导出；压测验证 P99 延迟增加 < 5%；采样率可配置                          |
| OTel GenAI 语义约定变更        | #2912 提案未稳定，字段可能调整         | 遵循 v1.28.0 稳定版核心字段，#2912 扩展字段作为 optional attributes，后续可平滑升级                   |
| Span 内容泄露敏感数据            | 监控数据含用户隐私/密钥               | 配置 `OTEL_GEN_AI_CONTENT_CAPTURE=metadata`，生产环境禁止 content 事件；Reasoning 字段可配置脱敏 |
| LLM 任务拆解质量不稳定            | 任务执行可能偏离预期                 | 引入 `TaskVerifier` 交付验证 + 人工审批流（`ToolCall.IsApproved` 已有模型字段）                  |
| 上下文窗口溢出                  | 对话质量下降                     | 关键词提取 + LLM 摘要双策略，Token 计数实时监控，compact API 支持手动触发                             |
| 技能安全边界                   | 未授权操作                      | 所有技能必须声明 `IsReadOnly()` + `RequiredPermission()`，写操作需二次审批                     |
| trace 整合后 Agent 角色权限循环   | Agent 读取自身 trace 可能影响决策    | Agent 角色默认拒绝 `agent:traces:read`，避免循环依赖；如需自我诊断需显式授权                           |
| SpanProcessor PostgreSQL 写入延迟 | 批量写入期间 PostgreSQL 不可用导致 span 丢失 | 批量写入 + 重试机制；PostgreSQL 不可用时 span 仍通过 OTLP 导出到 Tempo，不丢失实时数据                        |

***

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

-- 上下文消息表（PostgreSQL 持久化，pgvector 向量索引与行数据同库）
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

***

## 十、API 端点规划

### P0 阶段

| 方法     | 路径                                         | 说明                                  |
| ------ | ------------------------------------------ | ----------------------------------- |
| GET    | `/api/v1/agent/pools`                      | 列出所有工具池                             |
| GET    | `/api/v1/agent/pools/:id`                  | 获取池详情（含工具列表）                        |
| POST   | `/api/v1/agent/pools/select`               | 根据意图选择工具                            |
| POST   | `/api/v1/agent/pools`                      | 注册自定义池                              |
| DELETE | `/api/v1/agent/pools/:id`                  | 注销池                                 |
| POST   | `/api/v1/agent/skills/register`            | 注册自定义技能                             |
| GET    | `/api/v1/agent/skills`                     | 列出所有可用技能                            |
| GET    | `/api/v1/agent/skills/:name`               | 获取技能详情                              |
| POST   | `/api/v1/agent/skills/:name/execute`       | 执行技能                                |
| DELETE | `/api/v1/agent/skills/:name`               | 注销技能                                |
| POST   | `/api/v1/agent/contexts`                   | 创建上下文                               |
| GET    | `/api/v1/agent/contexts/:id`               | 获取上下文                               |
| POST   | `/api/v1/agent/contexts/:id/messages`      | 追加消息                                |
| POST   | `/api/v1/agent/contexts/:id/compact`       | 压缩上下文（支持 level + strategy 配置）       |
| GET    | `/api/v1/agent/contexts/:id/relevant`      | 语义检索相关消息（pgvector 向量检索）               |
| POST   | `/api/v1/agent/tasks`                      | 创建任务                                |
| GET    | `/api/v1/agent/tasks/:id`                  | 获取任务状态                              |
| POST   | `/api/v1/agent/tasks/:id/retry`            | 重试失败任务                              |
| GET    | `/api/v1/agent/traces/:session_id`         | 获取会话调用链（OTel Span 列表）               |
| GET    | `/api/v1/agent/traces/:session_id/tree`    | 获取调用链树形结构（基于 parent\_span\_id 构建）   |
| GET    | `/api/v1/agent/traces/:trace_id/waterfall` | 获取瀑布图数据（按时间排序的 span 列表）             |
| GET    | `/api/v1/agent/thoughts/:session_id`       | 获取思考过程（Span Event 列表）               |
| GET    | `/api/v1/agent/metrics`                    | 获取可观测性指标摘要（OTel GenAI 标准指标 + 自定义指标） |
| PUT    | `/api/v1/agent/pools/:id`                  | 更新池配置                               |
| POST   | `/api/v1/agent/pools/:id/tools`            | 向池添加工具                              |
| DELETE | `/api/v1/agent/pools/:id/tools/:tool_name` | 从池移除工具                              |

### P1 阶段

| 方法     | 路径                                   | 说明         |
| ------ | ------------------------------------ | ---------- |
| POST   | `/api/v1/agent/sessions`             | 创建会话       |
| GET    | `/api/v1/agent/sessions/:id`         | 获取会话       |
| POST   | `/api/v1/agent/sessions/:id/chat`    | 发送消息（多轮对话） |
| DELETE | `/api/v1/agent/sessions/:id`         | 结束会话       |
| POST   | `/api/v1/agent/rag/documents`        | 上传文档       |
| POST   | `/api/v1/agent/rag/query`            | RAG 检索     |
| GET    | `/api/v1/agent/memories/:session_id` | 获取记忆       |

***

## 十一、待确认事项

- [x] ~~向量数据库选型确认：Milvus vs Redis Search vs Qdrant vs pgvector~~ → ✅ 确认 pgvector（PGSQL 扩展，替代独立向量数据库）
- [x] ~~advisor/agentcoord 空壳目录是否直接删除~~ ✅ 已删除
- [x] ~~Embedding 模型选型确认：OpenAI API vs 本地模型~~ → ✅ 确认 BGE-M3 本地 ONNX Runtime
- [ ] 技能写操作是否需要人工审批流
- [ ] 上下文 Token 上限配置策略
- [ ] 任务最大拆解深度限制
- [ ] Tool Pool 意图路由策略确认：P0-0 先用关键词匹配，P1 是否升级为 Embedding 语义匹配
- [ ] 胖工具废弃时间线：瘦工具稳定后何时移除原胖工具文件
- [x] ~~pgvector / Qdrant 部署方案~~ → ✅ 确认 pgvector（PGSQL 扩展，无需独立部署）
- [ ] BGE-M3 模型文件分发策略：镜像内置 vs 运行时下载
- [ ] 上下文压缩默认级别确认：low(80%) / medium(50%) / high(20%)
- [ ] 调用链数据保留策略：保留天数、归档方案
- [ ] OTel GenAI 内容捕获策略确认：`metadata`（仅元数据）vs `content`（含 prompt/completion），生产环境建议 metadata
- [ ] OTel 采样率确认：开发环境 100% vs 生产环境按比例采样（建议 10%-50%）
- [ ] OTel Exporter 部署方案：Tempo vs Jaeger vs 已有 Grafana Tempo 实例
- [ ] ThoughtStep Reasoning 字段脱敏规则确认：正则替换密钥/Token vs 完全不记录
- [x] ~~trace 模块是否独立开发~~ → ✅ 确认整合为 agent 子模块（§5.8），删除 `internal/trace/` 空壳
- [x] ~~Agent Fork vs SubAgent 架构选型~~ → ✅ 确认放弃 Fork，直接实现 SubAgent（§5.5），采用 Tool-like SubAgent 模式
- [x] ~~多池联合选择机制~~ → ✅ 确认在 IntentRouter 中新增 `RouteMulti`，支持跨池工具合并（§5.5A）
- [x] ~~TaskPlanner 模块定位~~ → ✅ 确认纳入 P0-C 阶段实现，作为 SubAgent 编排的前置依赖（§5.5B）
- [ ] SubAgent 预定义实例确认：diagnosis_agent / capacity_agent / topology_agent 的工具子集划分
- [ ] SubAgent 独立 Token 预算配置策略：固定值 vs 按任务复杂度动态分配
- [ ] SubAgent 递归深度限制：是否允许 SubAgent 内部再调用 SubAgent
- [ ] 多池联合选择的默认策略：何时用 SelectTools（单池）vs SelectToolsMulti（多池）
- [ ] Agent 角色是否需要 `agent:traces:read` 权限的例外场景（如 Agent 自我诊断）
- [ ] pgvector HNSW 索引参数确认：m（建议 16）、ef_construction（建议 64）、ef_search（建议 40）
- [ ] OTel 压测基线确认：当前无 OTel 的 P50/P99/吞吐量基线数据
- [x] ~~自定义池是否需要持久化到数据库~~ → ✅ 已确认持久化（agent\_tool\_pools + agent\_pool\_tools 表）

