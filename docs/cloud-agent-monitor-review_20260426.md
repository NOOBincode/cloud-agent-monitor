# Cloud Agent Monitor 系统架构审查报告

## 一、整体判断

项目处于**"设计完整、实现残缺"**状态。域模型和数据库 schema 覆盖了大厂能力画像的 80%，但运行时代码只完成约 20%。核心问题不是"缺什么功能"，而是**已有功能的数据流断裂**——采集层大面积空壳导致上层分析无米之炊。

以下从架构设计缺陷、代码实现问题、安全与运维三个维度展开。

---

## 二、架构设计缺陷

### 2.1 数据流断裂：采集层是最大瓶颈

**问题**：整个平台的数据价值链是 `采集 → 存储 → 分析 → 展示/告警`，但采集层大面积空壳，导致后续所有环节无数据可消费。

| 模块 | 域模型 | DB Schema | Service/Repo | Collector | 状态 |
|---|---|---|---|---|---|
| GPU 监控 | ✅ | ✅ | ❌ 空 stub | ❌ 空 stub | 无数据 |
| 推理服务 | ✅ | ✅ | ❌ 空 stub | ❌ 空 stub | 无数据 |
| Cost/Budget | ✅ | ✅ | ❌ 空 stub | ❌ 空 stub | 无数据 |
| Topology | ✅ | ✅ | ❌ 空 struct | — | 工具返回空数据 |

**影响**：
- AI Infra 模块是项目核心差异化定位，但 GPU/推理指标全无数据
- Advisor 智能诊断依赖 Topology 工具，但拓扑工具 10 个方法返回空数据
- Cost 模块定义了 throttle/downgrade/block 动作但无执行逻辑

**建议**：
- **P0**：实现 GPU collector（DCGM Exporter → Prometheus → 仓储）
- **P0**：实现推理服务 collector（vLLM metrics endpoint → 仓储）
- **P1**：填充 Topology domain/service 层空 struct + service 逻辑

### 2.2 Agent 写操作强制锁死，平台变成纯只读仪表盘

**问题**：`AuthzToolWrapper` 硬编码拒绝所有 `IsReadOnly() == false` 的工具调用，`Compensator` 只打日志不执行，`BudgetAction` 定义了动作但无实现。所有 Agent 工具描述明确声明 "All operations are READ-ONLY"。

**影响**：
- 平台定位是"智能监控平台"，但 Agent 只能看不能做，价值大打折扣
- 告警静默、自动伸缩、成本控制等核心闭环动作全部不可用
- 与竞品（Datadog 的自动修复、PagerDuty 的自动响应）差距明显

**建议**：
- 短期：引入"人工审批模式"，Agent 提出操作建议 → 人工确认 → 执行
- 中期：基于 Casbin 的细粒度写权限控制，按角色/租户授权
- 长期：可信操作自动执行（基于置信度阈值 + 审计追溯）

### 2.3 配置文件硬编码敏感信息

**问题**：`configs/config.yaml` 中包含：
- JWT Secret: `your-secret-key-change-in-production`（硬编码弱密钥）
- DeepSeek API Key: `sk-9bc6ad60d20d484ea13bf6bd4c2514fb`（明文 API Key）
- 数据库密码: `root`（明文密码）

**影响**：
- 代码仓库泄露即全部失守
- JWT 弱密钥可被暴力破解伪造任意用户 token
- API Key 泄露导致 LLM 调用被滥用

**建议**：
- 敏感配置全部走环境变量或 Secret Manager（Vault/K8s Secret）
- `config.yaml` 只保留非敏感默认值
- Config Validate 中增加对硬编码默认值的检测（如 `secret_key` 包含 "change-in-production" 则拒绝启动）

### 2.4 数据库选型混乱：文档与代码不一致

**问题**：
- README 说 MySQL 8.0+，架构文档提到 "以 PostgreSQL 为准"
- `config.yaml` 中 `driver: "postgres"`，但 DSN() 方法同时支持 MySQL 和 PostgreSQL
- 迁移文件在 `migrations/postgresql/` 下，但代码中还有 `mysql.go`、`gorm.io/driver/mysql`、`gorm.io/driver/sqlite` 依赖
- `obs_platform.session.sql` 文件名暗示 MySQL session，但实际用 PostgreSQL

**影响**：
- 新成员困惑，不知道该用哪个数据库
- 多数据库驱动增加维护成本和测试复杂度
- 迁移脚本只维护了 PostgreSQL 版本

**建议**：
- 明确定义：生产用 PostgreSQL，开发/测试可用 SQLite
- 移除 MySQL 驱动依赖（除非有明确的多数据库需求）
- 更新 README 和文档，统一说辞

### 2.5 Wire 依赖注入过度集中

**问题**：`wire.go` 中有 40+ 个 Provider 函数，所有依赖在同一个文件中手动管理。`ProvideHTTPServer` 函数接收 12 个参数。

**影响**：
- 新增模块需要修改 wire.go，每次都是大改动
- 编译期 DI 的优势被手动维护的成本抵消
- `ProvideHTTPServer` 参数过多，违反函数参数数量最佳实践

**建议**：
- 按模块拆分 ProviderSet（alerting.ProviderSet, slo.ProviderSet 等）
- 使用 wire.NewSet 组织模块级 Provider
- HTTP Server 构建逻辑拆分到独立的 Router 模块

### 2.6 多租户实现不完整

**问题**：
- `APIKeyInfo` 有 `TenantID` 字段，但 `permissionsToScopes` 转换时 TenantID 为空
- 中间件中 `tenant_id` 为空时硬编码为 `"default"`
- Casbin RBAC with Domains 声称支持多租户，但策略文件 `policy.csv` 未按 domain 组织
- 数据库查询完全没有租户隔离（无 `WHERE tenant_id = ?` 过滤）

**影响**：
- 任何用户都能访问所有租户的数据
- 多租户场景下存在严重数据泄露风险

**建议**：
- 如果短期不需要多租户，先移除 TenantID 相关代码，避免半成品
- 如果需要，在 Repository 层强制加租户过滤，中间件注入租户上下文

---

## 三、代码实现问题

### 3.1 空壳模块严重——247 个 Go 文件，34 个测试文件

**问题**：测试覆盖率约 14%（34/247），远低于文档声称的 80% 目标。更关键的是：
- `internal/agent/domain/` 下 8 个核心域模型文件全是 `package domain`（空 struct）
- `internal/agent/infrastructure/workflow/builtin/` 下 4 个工作流文件全是 `package workflow`（空实现）
- `internal/agent/infrastructure/skill/builtin/` 下 3 个技能文件全是 `package skills`（空实现）
- `internal/cost/application/service.go` 只有一行 `type Service struct{}`
- `internal/aiinfra/infrastructure/collector.go` 和 `application/service.go` 都是空 package

**影响**：
- Agent 的核心能力（诊断工作流、技能执行）完全不可用
- 代码审查无法发现逻辑错误，因为没有逻辑
- CI/CD 的测试实际上只覆盖了基础 CRUD

**建议**：
- 对每个空壳模块创建 GitHub Issue，标记为 P0/P1
- 优先填充有下游消费者的模块（Topology service → Agent 拓扑工具）

### 3.2 E2E 和集成测试完全空缺

**问题**：
- `test/e2e/.gitkeep` 和 `test/integration/.gitkeep` 只有占位文件
- 没有任何端到端测试验证完整数据流
- Postman collection 存在但无法验证采集→存储→展示闭环

**影响**：
- 无法验证系统整体功能
- 部署后才发现集成问题
- 关键路径（告警发送→Alertmanager→通知）未经验证

**建议**：
- 至少覆盖 3 条核心 E2E 路径：告警发送→静默→通知、SLO 创建→错误预算查询、拓扑发现→影响分析
- 使用 testcontainers（已引入依赖）做集成测试

### 3.3 错误处理不一致

**问题**：
- 部分 Handler 直接 `c.JSON(500, ...)` 没有错误日志
- 部分使用 `pkg/response` 统一封装，部分裸返回 `gin.H`
- Repository 层错误直接上抛，没有区分业务错误和基础设施错误

**建议**：
- 统一使用 `pkg/response` 封装所有 HTTP 响应
- 定义业务错误码体系（`pkg/model/errors.go` 已有基础）
- Repository 层包装底层错误为领域错误

### 3.4 Prometheus 客户端无连接池和超时控制

**问题**：`promclient/client.go` 的具体实现需要查看，但 Wire 中 `ProvidePrometheusClient` 只传入配置，没有连接池、重试、熔断等保护。

**建议**：
- 添加 HTTP 连接池配置
- 实现指数退避重试
- 考虑引入 circuit breaker（如 `sony/gobreaker`）

### 3.5 缺少平台自身的可观测性

**问题**：
- 已引入 `go.opentelemetry.io/otel` 依赖，但只在 Agent 模块使用
- platform-api 自身的 RED 指标（Request rate, Error rate, Duration）未暴露
- 没有 `/metrics` 端点供 Prometheus 抓取
- 缺少请求链路追踪传播

**建议**：
- 为 Gin 添加 otelhttp 中间件
- 暴露 `/metrics` 端点
- 请求日志注入 trace_id

---

## 四、安全与运维问题

### 4.1 JWT 实现安全隐患

**问题**：
- `Validate()` 只检查 SecretKey 长度 ≥ 32，不检查强度
- 没有实现 Token 黑名单/撤销机制
- Refresh Token 没有轮换策略（每次刷新应颁发新的 Refresh Token）
- Access Token 有效期 60 分钟偏长

**建议**：
- Access Token 缩短到 15-30 分钟
- 实现 Refresh Token 轮换
- 添加 Token 黑名单（Redis 存储）

### 4.2 Casbin 策略文件与代码脱节

**问题**：
- `configs/casbin/rbac_model.conf` 和 `policy.csv` 需要检查是否与代码中的 Casbin 使用一致
- 策略变更没有版本管理和审批流程
- 没有策略热加载机制

**建议**：
- 策略文件 Git 版本化管理
- 实现 Casbin Adapter 对接数据库（而非 CSV 文件）
- 添加策略变更审计

### 4.3 缺少 API 限流

**问题**：
- `pkg/infra/ratelimit.go` 存在但未在路由层使用
- Agent 工具调用无速率限制
- 公开路由（`/api/v1/alerts/public/*`）无保护

**建议**：
- 在 Gin 中间件层启用限流
- Agent 工具调用按 session 限流
- 公开路由加 IP 限流

### 4.4 迁移管理不规范

**问题**：
- 12 个迁移文件手动编号，无迁移工具管理
- 没有使用 golang-migrate 或 goose 等迁移工具
- 迁移版本无校验和验证

**建议**：
- 引入 `golang-migrate/migrate` 或 `pressly/goose`
- CI 中验证迁移的 up/down 可逆性

### 4.5 Docker Compose 生产不适用

**问题**：
- Compose 中所有服务在同一个 bridge 网络，无网络隔离
- `DISABLE_SECURITY_PLUGIN: true`（OpenSearch）
- Neo4j/Zep/OpenSearch 引入了大量重中间件，但代码中未见使用
- 资源无限制，单节点部署无高可用

**建议**：
- 明确区分 dev 和 prod 部署方案
- 清理未使用的中间件（Neo4j、OpenSearch、Zep）
- 为每个服务添加 resource limits

---

## 五、设计过度/过早优化

### 5.1 中间件堆叠过重

**问题**：Docker Compose 引入了 PostgreSQL + Neo4j + OpenSearch + Zep + Redis + VictoriaMetrics + VictoriaTraces + VictoriaLogs + Prometheus + Loki + Grafana + Alertmanager，共 13 个服务。但：
- Neo4j：代码中未见使用
- OpenSearch：代码中未见使用
- Zep：仅作为 Agent 记忆层，但 Agent 域模型全是空壳
- 同时运行 Prometheus + VictoriaMetrics、Loki + VictoriaLogs 是冗余的

**建议**：
- 统一指标存储为 VictoriaMetrics（Prometheus 仅做采集）
- 统一日志存储为 VictoriaLogs（Loki 可移除）
- Neo4j 和 OpenSearch 等确认使用后再引入

### 5.2 插件化架构过早设计

**问题**：`docs/plugin-architecture.md` 规划了完整的插件系统和 K8S Operator，但核心模块都未完成。

**建议**：
- 插件化在核心功能稳定后再实施
- 当前阶段应专注数据流打通

### 5.3 创新路线图过于激进

**问题**：`docs/innovation-roadmap.md` 包含强化学习预聚合优化、多模态诊断、向量嵌入跨模态注意力等，但连基础的 GPU 指标采集都没有实现。

**建议**：
- 创新方向作为北极星可以保留，但实施优先级应大幅降低
- 当前应聚焦 P0（采集层）→ P1（服务层）→ P2（智能层）

---

## 六、修复优先级总结

| 优先级 | 问题 | 原因 | 工作量 |
|---|---|---|---|
| **P0** | 实现 GPU/推理 collector | 数据流断点，AI Infra 差异化核心无数据 | 中 |
| **P0** | 敏感信息移出代码仓库 | 安全风险，泄露即失守 | 小 |
| **P0** | 填充 Topology domain/service | Agent 10 个拓扑工具返回空数据 | 中 |
| **P1** | 数据库选型统一 | 文档代码不一致，维护成本高 | 小 |
| **P1** | Wire ProviderSet 拆分 | 40+ Provider 集中管理，改动成本高 | 中 |
| **P1** | 多租户设计明确化 | 半成品多租户比没有更危险 | 中 |
| **P1** | 核心 E2E 测试 | 无法验证系统整体功能 | 中 |
| **P2** | 清理无用中间件 | 资源浪费，增加开发环境复杂度 | 小 |
| **P2** | 平台自身可观测性 | 运维盲区 | 中 |
| **P2** | Agent 写操作分阶段开放 | 平台闭环能力 | 大 |
| **P3** | 插件化/创新路线图 | 核心功能未完成前不应投入 | — |
