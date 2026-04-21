# Cloud Agent Monitor 文档中心

本目录包含项目的核心技术文档，按功能分类组织。

---

## 快速导航

### 入门指南

| 文档 | 说明 |
|------|------|
| [technical-overview.md](./technical-overview.md) | **技术概览** - 技术栈、模块架构、关键设计决策（推荐首先阅读） |
| [architecture.md](./architecture.md) | **架构设计** - 分层架构、数据流、部署视图、实施路线图 |
| [开发计划.md](./开发计划.md) | **开发计划** - Docker Compose 环境、阶段任务、模块依赖 |

### 核心规范

| 文档 | 说明 |
|------|------|
| [testing-standards.md](./testing-standards.md) | **测试标准** - 单元测试、并发测试、性能测试规范 |
| [observability-maturity.md](./observability-maturity.md) | **成熟度模型** - 能力评估、演进路线、业界对标 |

### 集成指南

| 文档 | 说明 |
|------|------|
| [casbin-integration-guide.md](./casbin-integration-guide.md) | **Casbin 集成** - 权限控制配置与使用 |

### 扩展设计

| 文档 | 说明 |
|------|------|
| [plugin-architecture.md](./plugin-architecture.md) | **插件架构** - 可插拔模块设计（规划中） |
| [innovation-roadmap.md](./innovation-roadmap.md) | **创新路线图** - 差异化能力规划 |
| [ai-infra-module-analysis.md](./ai-infra-module-analysis.md) | **AI Infra 分析** - 模块设计、行业标准、差距分析 |

### 运维手册

| 目录 | 说明 |
|------|------|
| [runbooks/](./runbooks/) | **排障手册** - 告警对应的诊断步骤 |
| [adr/](./adr/) | **架构决策记录** - 重大技术决策文档 |

---

## 文档体系结构

```
docs/
├── README.md                      # 本文档（文档导航）
├── technical-overview.md          # 技术概览（核心参考）
├── architecture.md                # 架构设计
├── 开发计划.md                    # 开发计划与环境配置
├── testing-standards.md           # 测试标准
├── observability-maturity.md      # 成熟度模型
├── casbin-integration-guide.md    # Casbin 集成指南
├── plugin-architecture.md         # 插件架构设计
├── innovation-roadmap.md          # 创新路线图
├── ai-infra-module-analysis.md    # AI Infra 模块分析
├── runbooks/                      # 排障手册
│   └── checkout-sim.md
└── adr/                           # 架构决策记录
    └── .gitkeep
```

---

## 按角色推荐阅读

### 新成员入门

1. [technical-overview.md](./technical-overview.md) - 了解技术栈和架构
2. [architecture.md](./architecture.md) - 理解分层设计和数据流
3. [开发计划.md](./开发计划.md) - 搭建本地开发环境

### 开发人员

1. [testing-standards.md](./testing-standards.md) - 测试规范
2. [casbin-integration-guide.md](./casbin-integration-guide.md) - 权限控制
3. [runbooks/](./runbooks/) - 了解告警处理流程

### 架构师

1. [architecture.md](./architecture.md) - 架构设计细节
2. [observability-maturity.md](./observability-maturity.md) - 成熟度评估
3. [innovation-roadmap.md](./innovation-roadmap.md) - 技术演进规划

### 运维人员

1. [开发计划.md](./开发计划.md) - Docker Compose 配置
2. [runbooks/](./runbooks/) - 排障手册
3. [observability-maturity.md](./observability-maturity.md) - 能力对标

---

## 文档维护规范

### 更新原则

1. **保持简洁** - 避免重复内容，每个主题只在一个文档中详细说明
2. **及时更新** - 代码变更后同步更新相关文档
3. **版本记录** - 重要变更在文档末尾添加修订记录

### 文档命名规范

- 使用小写字母和连字符：`technical-overview.md`
- 中文文档保持原有命名：`开发计划.md`
- 子目录使用复数形式：`runbooks/`, `adr/`

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-04-17 | 初版：整理文档体系，删除冗余文档，创建导航索引 |