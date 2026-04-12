# 插件化架构设计

> 状态：规划中
> 创建时间：2026-04-04
> 优先级：低（主项目完善后再实施）

## 背景

为了支持 K8S Operator 等可选模块，需要设计一套可插拔的插件架构，使系统能够：

- 按需启用/禁用功能模块
- 保持核心系统轻量
- 支持模块独立演进

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Core System                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Plugin Interface                                    │   │
│  │  - Name() string                                     │   │
│  │  - Init(ctx, deps) error                            │   │
│  │  - Routes() []Route                                 │   │
│  │  - Shutdown() error                                 │   │
│  └─────────────────────────────────────────────────────┘   │
│                           ▲                                 │
│              ┌────────────┼────────────┐                   │
│              │            │            │                   │
│  ┌───────────┴───┐  ┌─────┴─────┐  ┌───┴───────┐          │
│  │ User Plugin   │  │ Operator  │  │ AI-Infra  │          │
│  │ (内置)        │  │ Plugin    │  │ Plugin    │          │
│  │               │  │ (可选)    │  │ (内置)    │          │
│  └───────────────┘  └───────────┘  └───────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### 目录结构

```
internal/
├── core/                    # 核心框架（必须）
│   ├── plugin/
│   │   ├── interface.go     # 插件接口定义
│   │   └── manager.go       # 插件管理器
│   └── bootstrap/           # 启动引导
│
├── modules/                 # 模块目录
│   ├── user/               # 用户模块（内置）
│   │   └── plugin.go       # 实现 Plugin 接口
│   │
│   ├── aiinfra/            # AI Infra 模块（内置）
│   │   └── plugin.go
│   │
│   └── operator/           # K8S Operator 模块（可选）
│       ├── plugin.go       # 实现 Plugin 接口
│       ├── domain/
│       ├── infrastructure/
│       └── interfaces/
│
└── contrib/                 # 第三方贡献模块
    └── example-plugin/
```

## 核心接口定义

```go
// Plugin 插件接口
type Plugin interface {
    // Name 插件名称（唯一标识）
    Name() string
    
    // Init 初始化插件
    Init(ctx context.Context, deps Dependencies) error
    
    // Routes 返回插件需要注册的路由
    Routes() []Route
    
    // Shutdown 关闭插件
    Shutdown() error
}

// Dependencies 插件依赖
type Dependencies struct {
    DB     *gorm.DB
    Logger *slog.Logger
    Config map[string]any  // 插件专属配置
}

// Route 路由定义
type Route struct {
    Method  string
    Path    string
    Handler gin.HandlerFunc
}
```

## 配置格式

```yaml
plugins:
  user:
    enabled: true
  
  operator:
    enabled: false  # 不启用
    kubeconfig: ~/.kube/config
    watch_namespaces:
      - ai-inference
      - ai-training
  
  aiinfra:
    enabled: true
    prometheus_url: http://prometheus:9090
```

## 实施计划

### 阶段 1：核心框架（预计 2-3 天）

- [ ] 定义 Plugin 接口
- [ ] 实现 PluginManager
- [ ] 配置加载与解析
- [ ] 生命周期管理

### 阶段 2：现有模块改造（预计 2-3 天）

- [ ] user 模块改造为插件
- [ ] aiinfra 模块改造为插件
- [ ] platform 模块改造为插件
- [ ] 测试验证

### 阶段 3：Operator 插件（预计 5-7 天）

- [ ] K8S Client 封装
- [ ] 自动伸缩控制器
- [ ] 故障自愈控制器
- [ ] 与观测性联动

## Operator 模块设计

> 核心定位：**Dev 环境生命周期自动化管理**

### 功能范围

| 功能   | 说明                                              |
| ---- | ----------------------------------------------- |
| 环境创建 | 根据模板自动创建 Dev 环境（Namespace、Deployment、Service 等） |
| 环境休眠 | 长时间无活动自动休眠（scale to zero），节省资源                  |
| 环境唤醒 | 检测到访问请求自动唤醒，恢复服务                                |
| 环境回收 | 过期/废弃环境自动清理，释放资源                                |
| 资源配额 | 为每个 Dev 环境设置资源上限，防止资源滥用                         |
| 环境隔离 | 确保 Dev 环境之间网络和资源隔离                              |

### Dev 环境生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                   Dev 环境生命周期                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐            │
│   │  创建    │───▶│  活跃    │───▶│  休眠    │            │
│   │ Created  │    │ Active   │    │ Sleeping │            │
│   └──────────┘    └────┬─────┘    └────┬─────┘            │
│        │              │               │                    │
│        │              │ 无活动 N 小时  │ 有访问请求         │
│        │              ▼               ▼                    │
│        │         ┌──────────┐    ┌──────────┐             │
│        │         │  休眠    │◀───│  唤醒    │             │
│        │         │ Sleeping │    │ Waking   │             │
│        │         └────┬─────┘    └──────────┘             │
│        │              │                                    │
│        │              │ 过期/手动删除                       │
│        │              ▼                                    │
│        │         ┌──────────┐                             │
│        └────────▶│  回收    │                             │
│                  │ Deleted  │                             │
│                  └──────────┘                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 与观测性联动

```
观测性系统                         Operator
    │                                 │
    │  Dev 环境无活动 > 2 小时        │
    ├────────────────────────────────▶│
    │                                 │ 自动休眠（scale to zero）
    │                                 │
    │  检测到休眠环境有访问请求       │
    ├────────────────────────────────▶│
    │                                 │ 自动唤醒（scale up）
    │                                 │
    │  Dev 环境超过 TTL（如 7 天）    │
    ├────────────────────────────────▶│
    │                                 │ 自动回收
    │                                 │
    │  资源使用超过配额               │
    ├────────────────────────────────▶│
    │                                 │ 限制资源/告警
```

### 典型场景

1. **开发人员创建临时环境**
   - 通过 API 或 CLI 创建 Dev 环境
   - 自动分配资源配额
   - 设置 TTL（如 24 小时）
2. **自动休眠节省成本**
   - 下班后无活动自动休眠
   - 第二天访问时自动唤醒
   - 节省 60-80% 的 Dev 环境成本
3. **环境自动清理**
   - 超过 TTL 自动回收
   - 防止"僵尸环境"占用资源
   - 保持集群整洁

### 配置示例

```yaml
plugins:
  operator:
    enabled: true
    kubeconfig: ~/.kube/config
    
    dev_environment:
      default_ttl: 24h              # 默认存活时间
      sleep_after: 2h               # 无活动后休眠时间
      max_environments_per_user: 5  # 每用户最大环境数
      
      resource_quota:
        cpu: "4"                    # 每个 Dev 环境最大 CPU
        memory: "8Gi"               # 每个 Dev 环境最大内存
        gpu: 1                      # 每个 Dev 环境最大 GPU
      
      templates:
        - name: "pytorch-dev"
          image: "pytorch/pytorch:latest"
          resources:
            cpu: "2"
            memory: "4Gi"
        
        - name: "tensorflow-dev"
          image: "tensorflow/tensorflow:latest"
          resources:
            cpu: "2"
            memory: "4Gi"
```

### API 设计

```
POST   /api/v1/dev-environments          # 创建 Dev 环境
GET    /api/v1/dev-environments          # 列出 Dev 环境
GET    /api/v1/dev-environments/:id      # 获取详情
DELETE /api/v1/dev-environments/:id      # 删除 Dev 环境
POST   /api/v1/dev-environments/:id/sleep   # 手动休眠
POST   /api/v1/dev-environments/:id/wake    # 手动唤醒
```

### 依赖

- `k8s.io/client-go`
- `k8s.io/apimachinery`
- 可选：`sigs.k8s.io/controller-runtime`（如果需要完整 Operator）

---

## AI Infra 场景扩展功能

> 以下功能模块与观测性系统深度联动，实现智能化的 AI 基础设施管理

### 功能矩阵

| 功能模块 | 说明 | 与观测性联动 | 优先级 |
|----------|------|--------------|--------|
| Dev 环境生命周期 | 已规划（见上文） | 无活动检测 → 自动休眠 | ⭐⭐⭐ |
| 推理服务自动伸缩 | 基于请求量/GPU利用率扩缩容 | 指标驱动 | ⭐⭐⭐ |
| 模型服务管理 | 部署、版本、回滚 | 健康检查 | ⭐⭐ |
| GPU 资源调度 | 多租户 GPU 分配 | 利用率监控 | ⭐⭐ |
| 训练作业管理 | 任务生命周期、Checkpoint | 进度监控 | ⭐⭐ |
| 成本优化 | Spot 实例、预算控制 | 成本指标 | ⭐ |

---

### 1. 推理服务自动伸缩

#### 与观测性联动

```
观测性系统                         Operator
    │                                 │
    │  请求 QPS > 1000/s             │
    ├────────────────────────────────▶│
    │                                 │ 自动扩容副本数
    │                                 │
    │  GPU 利用率 < 20% 持续 10 分钟  │
    ├────────────────────────────────▶│
    │                                 │ 自动缩容（节省成本）
    │                                 │
    │  P99 延迟 > 500ms               │
    ├────────────────────────────────▶│
    │                                 │ 紧急扩容 + 告警
    │                                 │
    │  模型推理错误率 > 5%            │
    ├────────────────────────────────▶│
    │                                 │ 自动回滚到上一版本
```

#### 配置示例

```yaml
inference_autoscaling:
  enabled: true
  
  scaling_rules:
    - name: "high_qps"
      metric: "requests_per_second"
      threshold: 1000
      action: "scale_up"
      replicas: +2
    
    - name: "low_gpu"
      metric: "gpu_utilization"
      threshold: 20
      duration: 10m
      action: "scale_down"
      replicas: -1
    
    - name: "high_latency"
      metric: "p99_latency_ms"
      threshold: 500
      action: "emergency_scale_up"
      replicas: +3
    
    - name: "high_error_rate"
      metric: "inference_error_rate"
      threshold: 5
      action: "rollback"
```

#### API 设计

```
GET    /api/v1/inference-services                    # 列出推理服务
POST   /api/v1/inference-services                    # 创建推理服务
GET    /api/v1/inference-services/:id                # 获取详情
PUT    /api/v1/inference-services/:id                # 更新配置
DELETE /api/v1/inference-services/:id                # 删除服务
POST   /api/v1/inference-services/:id/scale          # 手动伸缩
GET    /api/v1/inference-services/:id/metrics        # 获取指标
```

---

### 2. 模型服务管理

#### 生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                    模型服务生命周期                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐            │
│   │  上传    │───▶│  部署    │───▶│  服务    │            │
│   │ Upload   │    │ Deploy   │    │ Serving  │            │
│   └──────────┘    └────┬─────┘    └────┬─────┘            │
│                        │               │                    │
│                        │ 金丝雀发布    │ 健康检查失败       │
│                        ▼               ▼                    │
│                   ┌──────────┐    ┌──────────┐             │
│                   │  验证    │    │  回滚    │             │
│                   │ Validate │    │ Rollback │             │
│                   └──────────┘    └──────────┘             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 功能点

| 功能 | 说明 |
|------|------|
| 模型版本管理 | 支持 v1, v2, canary 等多版本共存 |
| 金丝雀发布 | 流量逐步切换：10% → 50% → 100% |
| 自动回滚 | 错误率超阈值自动回滚到上一版本 |
| 模型预热 | 预加载模型避免冷启动延迟 |
| A/B 测试 | 支持多模型流量分配 |

#### 配置示例

```yaml
model_serving:
  enabled: true
  
  deployment:
    strategy: "canary"           # canary / blue-green / rolling
    canary:
      initial_weight: 10         # 初始流量比例
      increment: 10              # 每次增加比例
      interval: 5m               # 增加间隔
    
    rollback:
      error_rate_threshold: 5    # 错误率阈值 %
      latency_threshold: 1000    # 延迟阈值 ms
      auto_rollback: true        # 自动回滚
    
    warmup:
      enabled: true
      requests: 100              # 预热请求数
      timeout: 5m                # 预热超时
```

#### API 设计

```
POST   /api/v1/models                           # 上传模型
GET    /api/v1/models                           # 列出模型
GET    /api/v1/models/:id                       # 获取详情
DELETE /api/v1/models/:id                       # 删除模型

POST   /api/v1/model-services                   # 部署模型服务
GET    /api/v1/model-services                   # 列出服务
GET    /api/v1/model-services/:id               # 获取详情
PUT    /api/v1/model-services/:id               # 更新配置
POST   /api/v1/model-services/:id/rollback      # 回滚版本
POST   /api/v1/model-services/:id/promote       # 提升金丝雀版本
```

---

### 3. GPU 资源调度

#### 资源池管理

```
┌─────────────────────────────────────────────────────────────┐
│                    GPU 资源池管理                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   GPU Pool (8x A100)                                       │
│   ┌─────┬─────┬─────┬─────┬─────┬─────┬─────┬─────┐       │
│   │ 0   │ 1   │ 2   │ 3   │ 4   │ 5   │ 6   │ 7   │       │
│   │用户A│用户A│用户B│空闲 │空闲 │用户C│空闲 │空闲 │       │
│   └─────┴─────┴─────┴─────┴─────┴─────┴─────┴─────┘       │
│                                                             │
│   调度策略：                                                 │
│   - 优先级队列（生产 > 开发）                               │
│   - 时间片轮转（公平共享）                                   │
│   - 抢占机制（高优先级可抢占）                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 与观测性联动

| 观测指标 | 调度决策 |
|----------|----------|
| GPU 利用率 | 低利用率 → 可分配新任务 |
| 显存使用 | 显存不足 → 拒绝新任务 |
| 空闲检测 | 长时间空闲 → 自动回收 |
| 温度监控 | 过热 → 迁移任务 |

#### 配置示例

```yaml
gpu_scheduler:
  enabled: true
  
  pools:
    - name: "production"
      nodes: ["gpu-node-1", "gpu-node-2"]
      type: "A100"
      priority: high
    
    - name: "development"
      nodes: ["gpu-node-3", "gpu-node-4"]
      type: "V100"
      priority: low
  
  scheduling:
    strategy: "priority"         # priority / fair / binpack
    preemption:
      enabled: true
      grace_period: 5m           # 抢占前等待时间
    
    quotas:
      production:
        max_gpus: 10
        max_duration: 24h
      development:
        max_gpus: 4
        max_duration: 8h
```

#### API 设计

```
GET    /api/v1/gpu-pools                        # 列出 GPU 池
GET    /api/v1/gpu-pools/:id                    # 获取详情
GET    /api/v1/gpu-pools/:id/utilization        # 利用率统计
GET    /api/v1/gpu-allocations                  # 列出分配记录
POST   /api/v1/gpu-allocations                  # 申请 GPU
DELETE /api/v1/gpu-allocations/:id              # 释放 GPU
```

---

### 4. 训练作业管理

#### 生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                    训练作业生命周期                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐            │
│   │  提交    │───▶│  排队    │───▶│  运行    │            │
│   │ Submit   │    │ Queue    │    │ Running  │            │
│   └──────────┘    └──────────┘    └────┬─────┘            │
│                                        │                    │
│                       ┌────────────────┼────────────────┐  │
│                       │                │                │  │
│                       ▼                ▼                ▼  │
│                  ┌──────────┐    ┌──────────┐    ┌────────┐│
│                  │ Checkpoint│   │  失败    │    │ 完成   ││
│                  │ (定期保存)│    │ Failed   │    │ Done   ││
│                  └────┬─────┘    └────┬─────┘    └────────┘│
│                       │               │                    │
│                       │ 断点续训      │ 自动重试           │
│                       └───────────────┴───────────────────▶│
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 功能点

| 功能 | 说明 |
|------|------|
| 任务队列 | 优先级队列，支持公平调度 |
| 自动 Checkpoint | 定期保存训练状态 |
| 断点续训 | 从 Checkpoint 恢复训练 |
| 资源预估 | 根据模型大小预估所需资源 |
| 分布式训练 | 支持多节点多 GPU 训练 |

#### 配置示例

```yaml
training_jobs:
  enabled: true
  
  checkpoint:
    enabled: true
    interval: 30m               # Checkpoint 间隔
    max_keep: 5                 # 保留最近 N 个
  
  retry:
    max_attempts: 3             # 最大重试次数
    backoff: exponential        # 退避策略
  
  resource_estimation:
    enabled: true
    model_size_multiplier: 3    # 模型大小 × N = 所需显存
  
  distributed:
    enabled: true
    backend: "nccl"             # nccl / gloo
```

#### API 设计

```
POST   /api/v1/training-jobs                   # 提交训练任务
GET    /api/v1/training-jobs                   # 列出任务
GET    /api/v1/training-jobs/:id               # 获取详情
DELETE /api/v1/training-jobs/:id               # 取消任务
POST   /api/v1/training-jobs/:id/stop          # 停止任务
POST   /api/v1/training-jobs/:id/resume        # 恢复任务（断点续训）
GET    /api/v1/training-jobs/:id/logs          # 获取日志
GET    /api/v1/training-jobs/:id/checkpoints   # 列出 Checkpoint
```

---

### 5. 成本优化

#### 优化策略

| 策略 | 说明 | 节省比例 |
|------|------|----------|
| Spot 实例 | 使用竞价实例运行可中断任务 | 60-80% |
| 自动休眠 | 非工作时间自动休眠 Dev 环境 | 40-60% |
| 资源整合 | 低利用率时合并服务 | 20-30% |
| 预算告警 | 超预算自动限制 | 防止超支 |
| 右sizing | 根据实际使用调整资源配置 | 15-25% |

#### 与观测性联动

```
观测性系统                         Operator
    │                                 │
    │  资源利用率 < 30% 持续 1 小时   │
    ├────────────────────────────────▶│
    │                                 │ 右sizing（减少资源配置）
    │                                 │
    │  月度成本接近预算 80%           │
    ├────────────────────────────────▶│
    │                                 │ 发送告警 + 限制非关键任务
    │                                 │
    │  Spot 实例可获取               │
    ├────────────────────────────────▶│
    │                                 │ 迁移可中断任务到 Spot
```

#### 配置示例

```yaml
cost_optimization:
  enabled: true
  
  spot_instances:
    enabled: true
    types: ["A100-spot", "V100-spot"]
    max_interruption_rate: 20    # 最大中断率 %
  
  auto_sleep:
    enabled: true
    schedule:
      - start: "22:00"
        end: "08:00"
        timezone: "Asia/Shanghai"
  
  budget:
    monthly_limit: 10000         # 月度预算（美元）
    alert_threshold: 80          # 告警阈值 %
    hard_limit: true             # 超预算是否强制限制
  
  right_sizing:
    enabled: true
    evaluation_period: 7d        # 评估周期
    recommendation_threshold: 30 # 利用率低于此值则建议缩容
```

#### API 设计

```
GET    /api/v1/cost/summary                     # 成本概览
GET    /api/v1/cost/breakdown                   # 成本分解（按服务/用户）
GET    /api/v1/cost/recommendations             # 优化建议
POST   /api/v1/cost/budget                      # 设置预算
GET    /api/v1/cost/budget                      # 获取预算配置
```

---

### 实施优先级

```
阶段 1（核心功能）
├── Dev 环境生命周期 ✅ 已规划
└── 推理服务自动伸缩

阶段 2（增强功能）
├── 模型服务管理
└── GPU 资源调度

阶段 3（优化功能）
├── 训练作业管理
└── 成本优化
```

## 风险与缓解

| 风险      | 缓解措施                   |
| ------- | ---------------------- |
| 插件间依赖复杂 | 明确依赖声明，避免循环依赖          |
| 配置验证困难  | 每个插件实现 ConfigValidator |
| 启动顺序问题  | 支持优先级配置                |
| 测试复杂度增加 | 每个插件独立测试 + 集成测试        |

## 参考资料

- [Go Plugin System](https://pkg.go.dev/plugin)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Helm Plugin System](https://helm.sh/docs/topics/plugins/)

***

## 变更记录

| 日期         | 变更                         |
| ------------ | ---------------------------- |
| 2026-04-04   | 初始设计文档                 |
| 2026-04-04   | 添加 AI Infra 场景扩展功能   |

