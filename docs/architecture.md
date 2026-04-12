# 架构说明（Architecture）

本文描述**逻辑架构、数据流、部署视图、安全边界**与**推荐完成顺序**。产品愿景与里程碑总表见 [`观测性平台计划.md`](./观测性平台计划.md)；本地 Docker 命令与端口见 [`开发计划.md`](./开发计划.md)；**人日与里程碑排班**见 [`工程量与工期.md`](./工程量与工期.md)。

---

## 1. 文档关系

| 文档 | 用途 |
|------|------|
| `观测性平台计划.md` | 定位、技术选型、Monorepo 结构、里程碑 M0～M5 |
| **architecture.md（本文）** | 架构图、数据流、与代码/中间件的对应关系、**实施顺序** |
| `开发计划.md` | Compose 一键环境、故障注入、阶段任务表 |
| `observability-design.md`（待按服务补充） | 每服务：信号 → 告警 → runbook |
| `runbooks/*.md` | 与告警一一对应的排障步骤 |
| `adr/` | 重大决策（含「不做什么」） |

---

## 2. 系统上下文（谁依赖谁）

```mermaid
flowchart LR
  subgraph users [人 / 自动化]
    SRE[值班 / 开发]
    Agent[AI Agent / IDE]
  end

  subgraph platform [自研观测平台]
    API[platform-api]
    MCP[obs-mcp]
    ADV[advisor-worker]
  end

  subgraph datastack [成熟数据面]
    MySQL[(MySQL)]
    Prom[Prometheus]
    Loki[Loki]
    Graf[Grafana]
  end

  SRE --> Graf
  SRE --> API
  Agent --> MCP
  MCP --> API
  ADV --> API
  API --> PG
  API -.->|只读聚合| Prom
  API -.->|只读聚合| Loki
  Prom --> Graf
  Loki --> Graf
```

**当前本地 MVP（Compose）**：已具备 **Prometheus / Loki / Grafana / Promtail / PostgreSQL / checkout-sim**；**platform-api、obs-mcp、advisor-worker** 为后续增量，通过同一 `obs` 网络接入即可（见第 5 节）。

---

## 3. 逻辑分层架构

```mermaid
flowchart TB
  subgraph clients [调用方]
    Human[Grafana / 浏览器]
    AIAgent[AI Agent]
  end

  subgraph control [控制面 Go]
    API[platform-api Gin]
    MCP[obs-mcp]
    ADV[advisor-worker Eino]
  end

  subgraph planes [平台域能力]
    Catalog[服务目录 / 预存查询模板]
    Policy[白名单 / 配额 / Schema]
    Audit[审计 - 只追加]
    Auth[鉴权]
  end

  subgraph collect [采集 - 演进期]
    Demo[checkout-sim /metrics]
    Alloy[Alloy 或 OTel - 后期]
  end

  subgraph store [存储与展示]
    Prom[Prometheus]
    Loki[Loki]
    Graf[Grafana]
    MySQL[(MySQL)]
  end

  Human --> Graf
  Human --> API
  AIAgent --> MCP
  MCP --> Policy
  MCP --> Auth
  MCP --> Audit
  MCP --> API
  API --> Catalog
  API --> Auth
  API --> Audit
  API --> PG
  ADV --> API
  ADV --> Catalog
  Demo -->|scrape| Prom
  Alloy --> Prom
  Alloy --> Loki
  Prom --> Graf
  Loki --> Graf
```

| 层级 | 职责 | 仓库落点（约定） |
|------|------|------------------|
| 数据面 | 指标/日志的采集、存储、查询、告警计算 | `deploy/compose`、`configs/prometheus-rules`；采集配置模板 → `deploy/alloy` |
| 控制面 | 租户、注册、配置版本、服务目录、对外 HTTP API | `cmd/platform-api`、`internal/platform`、`internal/agentcoord`、`internal/storage` |
| MCP 工具面 | 对 Agent 暴露**有限**只读能力 | `cmd/obs-mcp`、`internal/mcp`、`internal/policy` |
| 智能层 | 规则 + Eino 编排 + 可选 LLM | `cmd/advisor-worker`、`internal/advisor` |
| 审计面 | API/MCP 行为记录与高危规则 | `internal/audit` |
| Demo | 可注入故障的示例业务 | `demo-apps/checkout-sim`、`deploy/compose/fault-injector` |

**元数据存储**：控制面与审计索引以 **PostgreSQL** 为准（与当前 `docker-compose` 一致）。若你在 ADR 中改为 MySQL，仅影响 `internal/storage` 与迁移工具，不改变分层图。

---

## 4. 数据流（三条主线）

### 4.1 指标路径（Metrics）

1. 应用暴露 **`/metrics`**（Prometheus 文本格式）。  
2. **Prometheus** 按 `job` 拉取（间隔、标签、`honor_labels` 在 `scrape_config` / `relabel` 中统一）。  
3. **recording / alerting** 规则从 `configs/prometheus-rules` 挂载（Git 版本化）。  
4. **Grafana** 以 Prometheus 为数据源查询；告警可在 Prometheus UI 或 Grafana 中观察 firing。  

**演进**：生产多集群时，Prometheus **remote_write** 至 Mimir / VictoriaMetrics；架构上把「查询入口」从单 Prometheus 换为兼容端点即可，**服务目录**中保存「环境 → 查询基址 + 模板 ID」。

### 4.2 日志路径（Logs）

1. 应用 **stdout 结构化 JSON**（含 `level`、`msg`，可选 `trace_id`）。  
2. **Promtail**（或后期 Alloy）通过 **Docker SD** 采集容器日志，推 **Loki**。  
3. **Grafana Explore** 按 `{compose_service="..."}` 等标签检索；与告警联动时在 runbook 中写死推荐 LogQL 片段。  

### 4.3 控制面与智能路径（Control + Advisor）

1. **platform-api** 读写 **MySQL**（目录、配置版本、API Key 等）。  
2. **obs-mcp** 经 **policy** 校验后，调用 **platform-api** 或只读封装 **integration**（Prom/Loki/Grafana API），**禁止**默认透传任意 PromQL。  
3. **advisor-worker** 通过 **integration** + **catalog** 拉「事实」，**Eino** 编排；输出必须能映射到 **预存查询 ID / 告警名 / runbook 路径**。  
4. **audit** 在写路径与 MCP 工具边界落库（异步队列可选 NATS/Redis Stream，见总计划）。  

---

## 5. 部署视图：本地（Docker Compose）

| 容器 / 服务 | 角色 | 端口（宿主机） |
|-------------|------|----------------|
| checkout-sim | Demo 业务 + `/metrics` | 18080 |
| prometheus | 拉取 + 规则 | 9090 |
| loki | 日志存储 | 3100 |
| promtail | Docker 日志 → Loki | 9080（内部） |
| grafana | 可视化 | 3000 |
| postgres | 控制面 DB | 5432 |
| fault-injector | 一次性故障剧本（profile `inject`） | 无常驻端口 |

**后续接入自研二进制**：将 `platform-api`（及可选 `obs-mcp`）作为新 service 加入同一 **network: obs**，环境变量示例：`DATABASE_URL=postgres://obs:obs@postgres:5432/obs_platform?sslmode=disable`。

---

## 6. 安全与信任边界

| 边界 | 策略 |
|------|------|
| 人 → Grafana | 账号密码（本地弱口令仅开发）；生产对接 SSO/OIDC。 |
| 人 / Agent → platform-api | JWT / API Key / mTLS（分阶段）。 |
| Agent → obs-mcp | 工具 **白名单**、**速率限制**、**单次结果行数/点数上限**；敏感环境 mTLS。 |
| MCP → 数据面 | 仅 **目录允许的预存查询** 或 **平台侧代理查询**；审计记录 tool 名与参数摘要哈希。 |
| chaos → checkout-sim | **X-Chaos-Token**；仅内网或 compose 网络可达。 |

---

## 7. 平台自身的可观测性（简）

- **Go 服务**：`/healthz`、**RED** 指标、`runtime` 内存与 GC（可选 pprof 仅内网）。  
- **Postgres**：连接数、慢查询（后期）。  
- **Prometheus/Loki**：组件自有指标；磁盘与保留期写入 `开发计划` / 运维笔记。  

---

## 8. 完成顺序（实施路线图）

下列顺序按 **依赖关系** 排列：前一阶段未「验收通过」，不建议大规模展开后一阶段。详细任务见 [`开发计划.md`](./开发计划.md)。

| 顺序 | 阶段 | 目标 | 主要产出 / 验收 | 依赖 | 状态 |
|------|------|------|-----------------|------|------|
| **1** | 阶段1 | 基础环境 | Monorepo、Compose、demo 指标/日志/告警、runbook | 无 | ✅ |
| **2** | 阶段2 | 控制面核心 | platform-api、用户模块、鉴权、服务目录 API | 阶段1 | 🔄 |
| **3** | 阶段3 | 核心能力补齐 | SLO 管理、告警路由、服务拓扑 | 阶段2 | 待开始 |
| **4** | 阶段4 | MCP + Eino | obs-mcp、advisor-worker、policy、评测脚本 | 阶段2、阶段3 | 待开始 |
| **5** | 阶段5 | 可观测性增强 | 分布式追踪、采集器管理、长期存储 | 阶段3 | 待开始 |
| **6** | 阶段6 | AI Infra | 指标字典、dcgm、推理指标、面板 | 阶段2 | 待开始 |
| **7** | 阶段7 | 运营与优化 | 成本管理、备份容灾、多租户、On-call | 阶段5 | 待开始 |

**并行建议**：
- 阶段3（告警路由）可与阶段2（服务目录）并行
- 阶段4（MCP）可与阶段3（SLO/拓扑）部分并行
- 阶段5（长期存储）可与阶段6（AI Infra）并行
- **eBPF / 服务网格** 不进入上表关键路径，单独 ADR + spike

**当前你所在位置**：完成 **阶段1** 即达成「本地架构闭环」；**阶段2** 起进入自研控制面与数据库强绑定开发。

---

## 9. 新增能力模块架构

本节描述补齐成熟可观测平台缺失能力的架构设计，详见 [`开发计划.md`](./开发计划.md) 第 5～6 节。

### 9.1 模块依赖关系

```mermaid
flowchart TB
  subgraph existing [现有模块]
    catalog[服务目录]
    auth[鉴权]
    audit[审计]
    mcp[MCP 工具]
    advisor[Advisor]
  end

  subgraph new [新增模块]
    slo[SLO/SLA 管理]
    alerting[告警路由]
    topology[服务拓扑]
    trace[分布式追踪]
    cost[成本管理]
  end

  subgraph data [数据面]
    prom[Prometheus]
    loki[Loki]
    tempo[Tempo]
    vm[VictoriaMetrics]
  end

  slo --> prom
  slo --> catalog
  alerting --> prom
  alerting --> auth
  topology --> trace
  topology --> prom
  topology --> catalog
  trace --> tempo
  trace --> loki
  cost --> prom
  cost --> loki
  cost --> vm
  mcp --> slo
  mcp --> topology
  advisor --> slo
  advisor --> topology
```

### 9.2 SLO/SLA 管理架构

```mermaid
flowchart LR
  subgraph input [数据输入]
    prom[Prometheus]
    custom[自定义 SLI]
  end

  subgraph slo [SLO 引擎]
    definition[SLO 定义]
    calculator[错误预算计算]
    burnrate[燃烧率告警]
  end

  subgraph output [输出]
    api[REST API]
    dashboard[Grafana 面板]
    alert[告警]
  end

  prom --> |SLI 数据| calculator
  custom --> |业务指标| calculator
  definition --> calculator
  calculator --> burnrate
  burnrate --> alert
  calculator --> api
  calculator --> dashboard
```

**核心数据模型**：

| 实体 | 字段 | 说明 |
|------|------|------|
| SLO | name, target, window | 目标（如 99.9%）、时间窗口（如 30d） |
| SLI | type, query | 指标类型（可用性/延迟）、PromQL 查询 |
| ErrorBudget | remaining, consumed | 剩余/已消耗的错误预算 |
| BurnRate | current, threshold | 当前燃烧率、告警阈值 |

### 9.3 告警路由架构

```mermaid
flowchart TB
  subgraph source [告警源]
    prom[Prometheus]
    loki[Loki]
    custom[自定义规则]
  end

  subgraph routing [告警路由]
    am[Alertmanager]
    router[路由规则]
    silencer[静默规则]
    escalator[升级策略]
  end

  subgraph channel [通知通道]
    dingtalk[钉钉]
    slack[Slack]
    email[邮件]
    pagerduty[PagerDuty]
  end

  prom --> am
  loki --> am
  custom --> am
  am --> router
  router --> silencer
  silencer --> escalator
  escalator --> dingtalk
  escalator --> slack
  escalator --> email
  escalator --> pagerduty
```

**路由规则示例**：

```yaml
routes:
  - match:
      severity: critical
      service: payment
    receiver: pagerduty-oncall
    continue: false
  - match:
      severity: warning
    receiver: slack-alerts
    group_wait: 30s
    group_interval: 5m
```

### 9.4 服务拓扑架构

```mermaid
flowchart TB
  subgraph sources [数据源]
    traces[分布式追踪]
    metrics[服务指标]
    logs[调用日志]
  end

  subgraph analyzer [分析器]
    trace_analyzer[Trace 分析器]
    metric_analyzer[指标分析器]
    log_analyzer[日志分析器]
  end

  subgraph graph [服务图]
    nodes[服务节点]
    edges[调用边]
    impact[影响分析]
  end

  subgraph output [输出]
    api[拓扑 API]
    viz[可视化]
    alert[影响告警]
  end

  traces --> trace_analyzer
  metrics --> metric_analyzer
  logs --> log_analyzer
  trace_analyzer --> edges
  metric_analyzer --> edges
  log_analyzer --> edges
  nodes --> graph
  edges --> graph
  graph --> impact
  impact --> alert
  graph --> api
  graph --> viz
```

### 9.5 分布式追踪架构

```mermaid
flowchart LR
  subgraph apps [应用]
    svc1[服务 A]
    svc2[服务 B]
    svc3[服务 C]
  end

  subgraph collector [采集层]
    otlp[OTLP Collector]
  end

  subgraph storage [存储层]
    tempo[Tempo]
  end

  subgraph integration [集成]
    prom[Prometheus]
    loki[Loki]
    grafana[Grafana]
  end

  svc1 --> |OTLP| otlp
  svc2 --> |OTLP| otlp
  svc3 --> |OTLP| otlp
  otlp --> tempo
  tempo --> grafana
  tempo -.-> |trace_id| prom
  tempo -.-> |trace_id| loki
```

**Trace ID 关联**：

| 支柱 | 关联字段 | 用途 |
|------|----------|------|
| Metrics | `trace_id` label | 从指标跳转到 Trace |
| Logs | `trace_id` field | 从日志跳转到 Trace |
| Traces | `trace_id` | 链路唯一标识 |

### 9.6 成本管理架构

```mermaid
flowchart TB
  subgraph sources [数据源]
    prom[Prometheus 指标]
    loki[Loki 日志]
    vm[VictoriaMetrics]
  end

  subgraph analysis [分析引擎]
    cardinality[基数分析]
    storage[存储分析]
    allocation[成本分摊]
  end

  subgraph output [输出]
    report[成本报告]
    suggest[优化建议]
    alert[成本告警]
  end

  prom --> |指标基数| cardinality
  loki --> |日志量| storage
  vm --> |存储占用| storage
  cardinality --> allocation
  storage --> allocation
  allocation --> report
  allocation --> suggest
  allocation --> alert
```

### 9.7 新增模块代码结构

```
internal/
├── slo/                          # SLO/SLA 管理
│   ├── domain/
│   │   ├── objective.go
│   │   ├── error_budget.go
│   │   └── burn_rate.go
│   ├── application/
│   └── interfaces/http/
│
├── alerting/                     # 告警路由
│   ├── domain/
│   │   ├── routing.go
│   │   ├── silencing.go
│   │   └── escalation.go
│   ├── infrastructure/
│   │   ├── alertmanager.go
│   │   ├── dingtalk.go
│   │   └── slack.go
│   └── interfaces/http/
│
├── topology/                     # 服务拓扑
│   ├── domain/
│   │   ├── service_graph.go
│   │   └── edge.go
│   ├── infrastructure/
│   │   └── trace_analyzer.go
│   └── interfaces/http/
│
├── trace/                        # 分布式追踪
│   ├── domain/
│   │   ├── span.go
│   │   └── trace.go
│   ├── infrastructure/
│   │   ├── otlp_receiver.go
│   │   └── tempo_client.go
│   └── interfaces/http/
│
└── cost/                         # 成本管理
    ├── domain/
    │   ├── allocation.go
    │   └── cardinality.go
    ├── infrastructure/
    └── interfaces/http/
```

---

## 10. 创新方向与差异化能力

本节描述平台在 MVP 之上可逐步构建的**差异化创新点**，用于形成技术壁垒和竞争护城河。这些创新基于现有架构（MCP + Eino + 服务目录）自然延伸，非从零造轮子。

### 9.1 智能诊断编排创新（基于 advisor-worker + Eino）

**现状**：advisor-worker 仅做基础建议生成  
**创新方向**：

| 创新点 | 描述 | 技术实现 | 预期价值 |
|--------|------|----------|----------|
| **故障剧本自动化 (Runbook Automation)** | 定义 SLO violation → 根因分析 → 修复建议的 DAG 工作流 | Eino Chain-of-Thought + 状态机编排 | 值班响应时间从 30min 降至 5min |
| **多模态诊断** | 将日志、指标、Trace 统一编码输入 Eino 做跨信号关联分析 | 向量嵌入 + 跨模态注意力机制 | 复杂故障定位准确率提升 40% |
| **可解释根因定位** | 不仅给出结论，还展示完整推导路径 | Eino 的 Streamable Reasoning | 增强用户信任，便于审计 |

**与现有架构结合点**：复用 `internal/advisor` + `internal/integration`（Prom/Loki API 封装）

### 9.2 MCP 工具生态创新（基于 obs-mcp）

**现状**：obs-mcp 仅暴露基础只读工具  
**创新方向**：

| 创新点 | 描述 | 技术实现 | 预期价值 |
|--------|------|----------|----------|
| **动态工具发现** | Agent 注册时自动暴露其专属工具 | 服务目录 + 反射注册机制 | 工具生态自扩展，无需平台发版 |
| **工具链编排** | 支持多个工具组合调用形成工作流 | MCP Batch Requests + Eino 编排 | 复杂诊断场景一键自动化 |
| **安全沙箱** | 工具执行资源限制 + 结果脱敏 | cgroups + eBPF 监控 + 正则脱敏 | 防止工具滥用和数据泄露 |

**与现有架构结合点**：复用 `internal/mcp` + `internal/policy`（白名单/配额）+ `internal/audit`

### 9.3 观测数据自治创新

**现状**：依赖静态配置的 Prometheus/Loki  
**创新方向**：

| 创新点 | 描述 | 技术实现 | 预期价值 |
|--------|------|----------|----------|
| **智能采样** | 基于 Eino 动态调整采样率 | 异常检测 + 自适应采样策略 | 存储成本降低 50% 同时保证关键数据 |
| **指标预聚合优化** | 用强化学习自动调整 Recording Rules | RL Agent + PromQL 执行成本模型 | 查询延迟降低 30% |
| **日志结构化推断** | 对非结构化日志自动提取字段 | LLM + 模式学习 | 无需人工配置解析规则 |

**与现有架构结合点**：复用 `internal/integration` + `internal/advisor`

### 9.4 平台即产品（PaaS）创新

**现状**：单租户本地部署  
**创新方向**：

| 创新点 | 描述 | 技术实现 | 预期价值 |
|--------|------|----------|----------|
| **多租户观测托管** | 基于现有 tenant 模型提供 SaaS 服务 | namespace 隔离 + 资源配额 + 计费计量 | 商业模式扩展 |
| **观测数据市场** | 团队间发布/订阅指标、面板、告警模板 | 服务目录扩展 + 版本管理 | 知识复用，降低配置成本 |
| **FinOps 集成** | 观测数据与云成本关联分析 | 云厂商 API + 成本归因模型 | 自动识别资源浪费，优化云支出 |

**与现有架构结合点**：复用 `internal/platform`（租户/注册）+ `internal/catalog`

### 9.5 创新实施路线图

```mermaid
flowchart LR
    subgraph phase1 [阶段1: 核心能力]
        mcp[MCP工具生态]
        eino[Eino智能诊断]
    end
    
    subgraph phase2 [阶段2: 数据自治]
        sampling[智能采样]
        preagg[预聚合优化]
    end
    
    subgraph phase3 [阶段3: 平台产品化]
        saas[多租户SaaS]
        finops[FinOps集成]
    end
    
    phase1 --> phase2 --> phase3
```

| 阶段 | 时间估算 | 里程碑 | 依赖 |
|------|----------|--------|------|
| 阶段1 | M2～M3 | MCP工具链 + Eino故障剧本可演示 | 平台基座完成 |
| 阶段2 | M4 | 存储成本降低30% + 查询性能提升 | 数据积累3个月+ |
| 阶段3 | M5+ | SaaS化试点 + FinOps ROI可量化 | 商业化决策 |

### 9.6 与竞品差异化对比

| 能力 | 传统 APM | Datadog | **本平台（创新后）** |
|------|----------|---------|---------------------|
| 根因分析 | 规则引擎 | AI 异常检测 | **Eino 可解释推理 + 故障剧本自动化** |
| Agent 扩展 | 闭源探针 | 闭源探针 | **MCP 开放协议 + 动态工具发现** |
| 数据治理 | 手动配置 | 自动发现 | **智能采样 + RL 预聚合优化** |
| 成本优化 | 无 | 基础计费 | **FinOps 集成 + 资源浪费自动识别** |

**核心护城河**：MCP 开放生态 + Eino 可解释智能 + 数据自治能力，形成「开放-智能-自治」的技术壁垒。

---

## 11. 修订记录

| 日期 | 变更 |
|------|------|
| （首次） | 初版：分层图、数据流、Compose 视图、完成顺序表 |
| 2026-04-06 | 新增第 9 节「新增能力模块架构」，补齐成熟可观测平台缺失能力 |
| 2026-04-06 | 重置第 8 节「完成顺序」，按依赖关系重新排序为 7 个阶段 |

重大架构变更请在本表追加一行，并在 `adr/` 留一条决策说明。
