# Cloud Agent Monitor

云原生智能监控平台 - 集成告警管理、SLO治理、AI基础设施监控与智能诊断能力

## 项目概述

Cloud Agent Monitor 是一个面向云原生环境的全栈监控解决方案，提供从基础设施到应用层的全方位可观测性能力。平台深度集成 Prometheus 生态，并引入 AI 驱动的智能诊断与告警降噪能力。

### 核心特性

- **告警全生命周期管理** - 与 Alertmanager 深度集成，支持告警发送、静默管理、噪音分析与反馈闭环
- **SLO 治理平台** - 服务水平目标定义、SLI 采集、错误预算追踪与燃烧率告警
- **AI 基础设施监控** - GPU 利用率、推理服务性能、模型调用追踪与成本分析
- **智能诊断顾问** - 基于 Eino 框架的多 Agent 协作诊断，支持故障根因分析
- **MCP 服务集成** - Model Context Protocol 支持，为 AI Agent 提供标准化工具接口
- **分布式追踪** - Jaeger/Tempro 集成，服务依赖拓扑可视化
- **边缘 Agent 协调** - 分布式 Agent 注册、心跳管理与命令下发

## 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        Platform API Gateway                      │
├─────────────────────────────────────────────────────────────────┤
│  Alerting  │   SLO    │  AI Infra  │  Advisor  │  MCP  │ Trace │
│  Service   │ Service  │  Service   │  Service  │ Server│Service│
├─────────────────────────────────────────────────────────────────┤
│                     Application Layer                            │
├─────────────────────────────────────────────────────────────────┤
│  Domain Layer: Alert │ SLO │ AIInfra │ Advisor │ Agent │ Policy │
├─────────────────────────────────────────────────────────────────┤
│  Infrastructure: MySQL │ Redis │ Prometheus │ Alertmanager │ K8s │
└─────────────────────────────────────────────────────────────────┘
```

## 技术栈

| 类别 | 技术选型 |
|------|----------|
| 语言 | Go 1.25 |
| Web框架 | Gin |
| ORM | GORM |
| 依赖注入 | Google Wire |
| 配置管理 | Viper |
| 任务队列 | Asynq |
| 可观测性 | Prometheus, Alertmanager, Jaeger, Tempo |
| AI框架 | CloudWeGo Eino |
| 权限控制 | Casbin |
| 数据库 | MySQL / SQLite |
| 缓存 | Redis, FreeCache |

## 项目结构

```
cloud-agent-monitor/
├── cmd/                        # 应用入口
│   ├── platform-api/           # 主API服务
│   ├── agent/                  # 边缘Agent
│   ├── worker/                 # 后台任务Worker
│   ├── advisor-worker/         # 诊断任务Worker
│   └── obs-mcp/                # MCP服务
├── internal/                   # 内部模块
│   ├── alerting/               # 告警管理
│   ├── slo/                    # SLO治理
│   ├── aiinfra/                # AI基础设施监控
│   ├── advisor/                 # 智能诊断顾问
│   ├── mcp/                    # MCP服务
│   ├── trace/                  # 分布式追踪
│   ├── agentcoord/             # Agent协调
│   ├── policy/                 # 策略管理
│   ├── audit/                  # 审计日志
│   ├── cost/                   # 成本分析
│   ├── auth/                   # 认证授权
│   ├── user/                   # 用户管理
│   ├── platform/               # 平台基础
│   └── storage/                # 数据存储
├── pkg/                        # 公共包
│   ├── config/                 # 配置
│   ├── logger/                 # 日志
│   ├── model/                  # 公共模型
│   ├── response/               # 响应封装
│   ├── version/                # 版本信息
│   └── infra/                  # 基础设施
└── demo-apps/                  # 演示应用
```

## 快速开始

### 环境要求

- Go 1.25+
- MySQL 8.0+ 或 SQLite
- Redis 6.0+
- Prometheus + Alertmanager (可选)

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/your-org/cloud-agent-monitor.git
cd cloud-agent-monitor

# 安装依赖
go mod download

# 生成依赖注入代码
go generate ./cmd/platform-api/...

# 运行服务
go run ./cmd/platform-api
```

### 配置说明

配置文件支持 YAML/JSON/TOML 格式，通过环境变量 `CONFIG_PATH` 指定路径。

主要配置项：

```yaml
server:
  addr: ":8080"

database:
  driver: "mysql"
  dsn: "user:password@tcp(localhost:3306)/cloud_monitor?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  addr: "localhost:6379"

prometheus:
  url: "http://localhost:9090"

alertmanager:
  url: "http://localhost:9093"
```

## API 文档

### 告警管理

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/alerts` | GET | 获取告警列表 |
| `/api/v1/alerts` | POST | 发送告警 |
| `/api/v1/silences` | GET | 获取静默列表 |
| `/api/v1/silences` | POST | 创建静默 |
| `/api/v1/silences/:id` | DELETE | 删除静默 |
| `/api/v1/alerts/noisy` | GET | 获取噪音告警 |
| `/api/v1/alerts/feedback/:fingerprint` | GET | 获取告警反馈 |

### SLO 管理

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/slos` | GET | 获取SLO列表 |
| `/api/v1/slos` | POST | 创建SLO |
| `/api/v1/slos/:id` | GET | 获取SLO详情 |
| `/api/v1/slos/:id/error-budget` | GET | 获取错误预算 |
| `/api/v1/slos/burn-rate-alerts` | GET | 获取燃烧率告警 |

### AI 基础设施

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/ai-infra/gpu-metrics` | GET | GPU指标 |
| `/api/v1/ai-infra/inference-services` | GET | 推理服务列表 |
| `/api/v1/ai-infra/models` | GET | 模型列表 |
| `/api/v1/ai-infra/costs` | GET | 成本分析 |

## 核心模块

### 告警服务 (Alerting)

告警服务提供完整的告警生命周期管理：

- **告警发送** - 支持单条/批量发送，自动指纹计算
- **静默管理** - 创建/删除静默规则，支持审计记录
- **噪音分析** - 识别高频重复告警，提供降噪建议
- **反馈闭环** - 记录告警处理反馈，持续优化告警质量
- **多通道通知** - 支持 Slack、钉钉、邮件、PagerDuty

### SLO 服务

SLO 服务实现服务水平目标治理：

- **SLO 定义** - 目标值、时间窗口、燃烧率阈值配置
- **SLI 采集** - 从 Prometheus 自动采集可用性指标
- **错误预算** - 实时计算剩余错误预算
- **燃烧率告警** - 基于多窗口燃烧率的智能告警

### 智能诊断顾问 (Advisor)

基于 Eino 框架的 AI 诊断系统：

- **多 Agent 协作** - 探索、诊断、规划、执行 Agent 编排
- **图引擎驱动** - 可视化诊断流程编排
- **规则引擎** - 内置诊断规则，支持自定义扩展
- **工具集成** - Prometheus 查询、日志分析、拓扑探索

### MCP 服务

Model Context Protocol 实现：

- **标准化工具接口** - 为 AI Agent 提供统一的工具调用协议
- **告警工具** - 查询告警、创建静默
- **SLO 工具** - 查询 SLO 状态、错误预算
- **权限控制** - 基于 Casbin 的细粒度权限管理

## 开发指南

### 代码规范

```bash
# 运行代码检查
golangci-lint run

# 运行测试
go test ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
```

### 依赖注入

项目使用 Google Wire 进行依赖注入，修改依赖后需重新生成：

```bash
cd cmd/platform-api
wire
```

### 添加新模块

1. 在 `internal/` 下创建模块目录
2. 按照领域驱动设计组织代码：`domain/`, `application/`, `infrastructure/`, `interfaces/`
3. 在 `cmd/platform-api/wire.go` 中注册依赖
4. 在 `internal/platform/routes.go` 中注册路由

## 许可证

MIT License

## 贡献指南

欢迎提交 Issue 和 Pull Request。请确保：

1. 代码通过 lint 检查
2. 新功能包含单元测试
3. 遵循现有的代码风格和架构模式
