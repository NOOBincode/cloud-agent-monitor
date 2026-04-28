# Cloud Agent Monitor 技术概览

> 本文档是项目核心技术栈、模块架构与关键设计决策的系统性总结，为团队成员提供快速查阅的技术参考。

---

## 1. 项目定位

Cloud Agent Monitor 是一个面向云原生环境的全栈监控解决方案，核心定位：

| 阶段 | 目标 | 核心能力 |
|------|------|----------|
| **阶段①** | 可运维观测基座 | 指标/日志/告警/多环境支持 |
| **阶段②** | 控制面 + MCP + Agent | 服务目录、审计、可评测诊断 |
| **阶段③** | AI Infra 观测 | GPU/推理/训练指标与成本治理 |

**差异化优势**：在标准 Prometheus/Loki/Grafana 数据面上，叠加服务目录为单一真相源、带边界的 MCP、可追加审计与故障剧本回归。

---

## 2. 技术栈总览

### 2.1 核心技术选型

| 类别 | 技术 | 版本 | 用途 |
|------|------|------|------|
| **语言** | Go | 1.25 | 控制面、Worker、MCP 统一栈 |
| **Web框架** | Gin | 1.12.0 | HTTP API 服务 |
| **ORM** | GORM | 1.31.1 | 数据库访问 |
| **依赖注入** | Google Wire | 0.7.0 | 编译期 DI |
| **配置管理** | Viper | 1.21.0 | 十二因子配置 |
| **任务队列** | Asynq | 0.26.0 | 异步任务处理 |
| **权限控制** | Casbin | 2.135.0 | RBAC/ABAC |
| **AI框架** | CloudWeGo Eino | 0.8.5 | Agent 编排 |
| **缓存** | Redis + FreeCache | 9.14.1 / 1.2.7 | 分布式/本地缓存 |

### 2.2 观测数据面

| 组件 | 用途 | 说明 |
|------|------|------|
| Prometheus | 指标采集与告警 | 支持 remote_write 到长期存储 |
| Loki | 日志聚合 | 与指标标签模型对齐 |
| Grafana | 可视化与告警展示 | 预置数据源与仪表盘 |
| Alertmanager | 告警路由 | 多通道通知（钉钉/Slack/邮件） |
| Tempo/Jaeger | 分布式追踪 | Trace ID 贯穿三大支柱 |

### 2.3 存储与中间件

| 组件 | 用途 | 说明 |
|------|------|------|
| PostgreSQL | 元数据存储 | 服务目录、用户、审计索引 |
| Redis | 缓存/限流 | 支持降级路径 |

---

## 3. 核心模块架构

### 3.1 分层架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        接口层 (Interfaces)                       │
│                     HTTP Handler / MCP Tool                      │
├─────────────────────────────────────────────────────────────────┤
│                        应用层 (Application)                       │
│    TopologyService │ AlertService │ SLOService │ AdvisorService │
├─────────────────────────────────────────────────────────────────┤
│                        领域层 (Domain)                            │
│    ServiceNode │ CallEdge │ Alert │ SLO │ AISession │ Agent     │
├─────────────────────────────────────────────────────────────────┤
│                    基础设施层 (Infrastructure)                    │
│  K8s Backend │ Prometheus Backend │ Redis Cache │ PostgreSQL Repo│
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 模块清单

| 模块 | 目录 | 状态 | 核心功能 |
|------|------|------|----------|
| **topology** | internal/topology | ✅ 完整 | 服务拓扑发现、影响分析、路径查找 |
| **alerting** | internal/alerting | ✅ 完整 | 告警管理、静默、噪音分析、多通道通知 |
| **slo** | internal/slo | ✅ 完整 | SLO 定义、错误预算、燃烧率告警 |
| **advisor** | internal/advisor | ✅ 完整 | Eino 编排、多 Agent 协作诊断 |
| **mcp** | internal/mcp | ✅ 完整 | MCP 工具实现、权限控制 |
| **aiinfra** | internal/aiinfra | ⚠️ 骨架 | GPU/推理/训练观测（Schema 已定义） |
| **trace** | internal/trace | ⚠️ 骨架 | 分布式追踪集成 |
| **cost** | internal/cost | ⚠️ 骨架 | 成本分析与优化建议 |
| **agentcoord** | internal/agentcoord | ✅ 完整 | Agent 注册、心跳、命令下发 |
| **auth** | internal/auth | ✅ 完整 | API Key + Casbin RBAC |
| **audit** | internal/audit | ✅ 完整 | 审计日志记录与查询 |
| **policy** | internal/policy | ✅ 完整 | 策略管理、配额控制 |

### 3.3 模块依赖关系

```
                    ┌─────────────────────┐
                    │   HTTP Interface    │
                    └──────────┬──────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
         ▼                     ▼                     ▼
  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
  │  Topology   │◀────▶│  Alerting   │      │    SLO      │
  │  Service    │      │  Service    │      │  Service    │
  └──────┬──────┘      └──────┬──────┘      └──────┬──────┘
         │                    │                    │
         │    ┌───────────────┼────────────────────┤
         │    │               │                    │
         ▼    ▼               ▼                    ▼
  ┌─────────────────────────────────────────────────────────────┐
  │                      共享基础设施层                           │
  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
  │  │  Redis  │  │PostgreSQL│  │   K8s   │  │Prometheus│        │
  │  │  Cache  │  │   DB    │  │  API    │  │ Client  │        │
  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │
  └─────────────────────────────────────────────────────────────┘
```

---

## 4. 关键设计决策

### 4.1 DDD 分层架构

**决策**：采用领域驱动设计（DDD）分层架构。

**理由**：
- 领域层保持纯粹，不依赖基础设施细节
- 应用层编排业务流程，易于测试
- 基础设施层可替换（如切换数据库）

**示例**：拓扑模块结构
```
internal/topology/
├── domain/           # 领域模型（ServiceNode, CallEdge, ImpactResult）
├── application/      # 业务服务（TopologyService, GraphAnalyzer）
├── infrastructure/   # 技术实现（K8sBackend, RedisCache, PostgreSQLRepo）
└── interfaces/http/  # HTTP 处理器
```

### 4.2 服务目录为单一真相源

**决策**：服务目录作为平台元数据的唯一来源。

**理由**：
- 避免元数据分散在多个系统
- 统一标签规范（env, service, tenant, cluster）
- 支持 AI Agent 通过目录发现服务

**实现**：
- 服务注册支持手动录入和自动发现
- 预存查询模板绑定到服务
- 告警元数据关联服务目录

### 4.3 MCP 工具边界控制

**决策**：MCP 工具采用白名单 + 结果截断策略。

**理由**：
- 防止 AI Agent 执行任意 PromQL
- 控制数据泄露风险
- 支持审计追踪

**实现**：
- 工具白名单配置（policy 模块）
- JSON Schema 入参校验
- 结果条数/体量限制
- 审计记录工具调用

### 4.4 Eino Agent 编排

**决策**：使用 CloudWeGo Eino 进行 Agent 编排。

**理由**：
- Go 原生支持，与项目栈一致
- 支持图引擎驱动的诊断流程
- 可评测的故障剧本回归

**实现**：
- 多 Agent 协作（探索、诊断、规划、执行）
- 规则引擎优先，LLM 可选
- 输出引用目录中的查询/告警 ID

### 4.5 Casbin 权限控制

**决策**：使用 Casbin 替代自定义 RBAC。

**理由**：
- 成熟稳定（17k+ GitHub Stars）
- 支持 RBAC/ABAC/多租户
- 内置缓存机制

**模型**：RBAC with Domains
```
用户 → 角色 → 租户 → 权限
```

---

## 5. 数据模型设计

### 5.1 核心实体

| 实体 | 模块 | 关键字段 |
|------|------|----------|
| **ServiceNode** | topology | ID, Name, Namespace, Status, Labels |
| **CallEdge** | topology | SourceID, TargetID, EdgeType, Confidence |
| **Alert** | alerting | Fingerprint, Status, Labels, Annotations |
| **SLO** | slo | Name, Target, Window, SLIQuery |
| **AISession** | aiinfra | ModelID, Tokens, Cost, Duration |
| **AgentDecision** | advisor | AgentID, Conclusion, Confidence, ReasoningPath |

### 5.2 数据库迁移

迁移文件位于 `internal/storage/migrations/postgresql/`：

| 编号 | 文件 | 内容 |
|------|------|------|
| 001 | init.up.sql | 基础表结构 |
| 002 | ai_observability.up.sql | AI 观测表（ai_models, ai_sessions, tool_calls） |
| 008 | alerting.up.sql | 告警管理表 |
| 010 | slo.up.sql | SLO 管理表 |
| 011 | topology.up.sql | 拓扑表（service_nodes, call_edges） |

---

## 6. API 设计规范

### 6.1 RESTful API 端点

| 模块 | 基础路径 | 核心端点 |
|------|----------|----------|
| **Alerting** | /api/v1/alerts | GET/POST 告警、静默管理 |
| **SLO** | /api/v1/slos | CRUD SLO、错误预算查询 |
| **Topology** | /api/v1/topology | 服务拓扑、影响分析、路径查找 |
| **AI Infra** | /api/v1/ai-infra | GPU 指标、推理服务、成本分析 |
| **Advisor** | /api/v1/advisor | 诊断请求、建议查询 |

### 6.2 MCP 工具清单

| 工具名 | 功能 | 权限要求 |
|--------|------|----------|
| **list_services** | 列出服务目录 | reader |
| **get_alert_context** | 获取告警上下文 | reader |
| **execute_saved_query** | 执行预存查询 | reader |
| **get_runbook_metadata** | 获取 Runbook 元数据 | reader |
| **analyze_impact** | 分析故障影响 | editor |

---

## 7. 测试策略

### 7.1 测试金字塔

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

### 7.2 测试覆盖率目标

| 类型 | 目标 | 工具 |
|------|------|------|
| 单元测试 | ≥ 80% | go test, testify |
| 并发测试 | 无数据竞争 | go test -race |
| 稳定性测试 | 无资源泄漏 | goleak |
| 集成测试 | 关键路径覆盖 | testcontainers |

详见 [testing-standards.md](./testing-standards.md)。

---

## 8. 部署架构

### 8.1 本地开发环境

使用 Docker Compose 一键启动：

```bash
docker compose -f deploy/compose/docker-compose.yml up -d --build
```

| 服务 | 端口 | 说明 |
|------|------|------|
| Grafana | 3000 | 可视化 |
| Prometheus | 9090 | 指标采集 |
| Loki | 3100 | 日志聚合 |
| PostgreSQL | 5432 | 元数据存储 |
| checkout-sim | 18080 | Demo 应用 |

### 8.2 生产部署建议

| 组件 | 高可用方案 |
|------|------------|
| platform-api | 无状态水平扩展 |
| PostgreSQL | 主从复制 + 备份演练 |
| Prometheus | 双副本 + remote_write |
| Redis | Sentinel/Cluster |

---

## 9. 开发路线图

| 阶段 | 目标 | 状态 |
|------|------|------|
| **阶段1** | 基础环境（Compose + Demo + 告警） | ✅ 完成 |
| **阶段2** | 控制面核心（API + 用户 + 鉴权 + 目录） | 🔄 进行中 |
| **阶段3** | 核心能力（SLO + 告警路由 + 拓扑） | 待开始 |
| **阶段4** | MCP + Eino（工具 + 诊断 + 评测） | 待开始 |
| **阶段5** | 可观测增强（追踪 + 采集器 + 存储） | 待开始 |
| **阶段6** | AI Infra（GPU + 推理 + 成本） | 待开始 |
| **阶段7** | 运营优化（成本 + 备份 + 多租户） | 待开始 |

详见 [开发计划.md](./开发计划.md) 和 [工程量与工期.md](./工程量与工期.md)。

---

## 10. 文档体系索引

| 文档 | 用途 |
|------|------|
| **technical-overview.md（本文）** | 技术栈、模块架构、关键决策 |
| **architecture.md** | 分层架构、数据流、部署视图 |
| **testing-standards.md** | 测试规范与覆盖率目标 |
| **observability-maturity.md** | 成熟度评估与演进路线 |
| **开发计划.md** | Compose 命令、阶段任务 |
| **工程量与工期.md** | 人日预估、排班 |
| **casbin-auth.md** | Casbin 权限控制集成 |
| **runbooks/*.md** | 告警排障步骤 |

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-04-17 | 初版：整合技术栈、模块架构、关键设计决策 |