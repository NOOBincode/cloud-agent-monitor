# 当前能力缺陷清单

> 版本: v1.0 | 日期: 2026-04-24 | 状态: 记录存档

## 一、核心判断

项目处于"设计完整、实现残缺"状态：域模型和数据库 schema 覆盖了大厂能力画像的 80%，但运行时代码只完成约 20%。上层分析（异常检测、故障预测、自动修复）均依赖底层数据流，而采集层大面积空壳导致无米之炊。

---

## 二、空壳模块清单（域模型/schema 完整，运行时代码为 stub）

| 模块 | 域模型 | DB Schema | Service/Repo | Collector | Agent Tools | 缺失影响 |
|---|---|---|---|---|---|---|
| GPU 监控 | 完整 | 完整 | **空 stub** | **空 stub** | **仅占位池，无工具** | 无法采集任何 GPU 指标，DCGM 字段全部无数据 |
| 推理服务 | 完整 | 完整 | **空 stub** | **空 stub** | **仅占位池，无工具** | vLLM/SGLang TTFT/TPOT/吞吐无数据 |
| Queue/调度 | 完整 | 完整 | **空 stub** | **空 stub** | — | 调度等待时间、资源分配率无法观测 |
| Cost/Budget | 完整 | 完整 | **空 stub** | **空 stub** | — | 成本治理动作（throttle/downgrade/block）仅有定义，无执行逻辑 |
| GenAI Session | 完整 | 完整 | **空 stub** | **空 stub** | — | LLM 调用追踪仅 Agent 自身有，业务侧 Session 无采集 |
| Policy/Governance | **空 stub** | 完整 | **空 stub** | — | — | 策略引擎完全空壳 |

---

## 三、半完成模块（接口完整，底层为空壳）

| 模块 | 缺陷描述 |
|---|---|
| Topology | Agent 工具接口 10 个方法全部定义（GetServiceTopology, FindAnomalies, AnalyzeImpact 等），但域模型层全是空 struct，Service 层为空 stub。工具调用返回空数据 |
| Agent Workflow/Skill | `diagnose.go`, `alert_analyze.go`, `capacity_check.go` 等内置工作流文件存在但内容为空，Agent 无法执行多步诊断流程 |
| Agent Domain | workflow, skill, context, task, session, memory, fork, trace 八个域模型文件全是空 struct |

---

## 四、完全缺失的能力

| 能力 | 大厂对应 | 当前状态 |
|---|---|---|
| CUDA Kernel 级 Trace | Nsight Systems / PyTorch Profiler 按需采样 | 无任何代码 |
| NCCL 通信拓扑热力图 | Ring/Tree 算法可视化 + 每对 Rank 带宽/延迟 | 无任何代码 |
| GPU 物理拓扑（NVLink/PCIe/NUMA） | `nvidia-smi topo -m` 采集 + 调度关联 | 域模型有字段，无采集 |
| RDMA 链路质量（per-port） | `perfquery` + `ibdiagnet` | DB 字段有定义，无采集 |
| Checkpoint I/O 监控 | 保存耗时分解（序列化→写入→同步） | 无任何代码 |
| 数据加载瓶颈观测 | DataLoader Worker 级 Profile | 无任何代码 |
| 调度决策追踪 | K8S Scheduler 打分过程可视化 | 无任何代码 |
| 统计异常检测 | 3-sigma / Prophet / VAE | 唯一"检测"是噪声评分（频率过高），不是时序异常检测 |
| ECC 趋势预测 | XID/ECC 错误率趋势 + 显存泄漏检测 | 无数据采集，更无趋势分析 |
| 故障预测与预防 | 事前预警 + 自动隔离 | 无任何代码 |
| 多租户干扰检测 | Noisy Neighbor 关联分析 | 无任何代码 |
| eBPF 无侵入采集 | TCP 重传 / IO 延迟 / 上下文切换 | 无任何代码 |
| 变更关联归因 | CI/CD Webhook + K8S Event + 指标异常时间线对齐 | `topology_changes` 表有 before/after_state 字段设计，无实现 |
| 存储系统观测 | Lustre/GPFS 客户端延迟 / 元数据瓶颈 | 无任何代码 |

---

## 五、功能已定义但强制锁死的能力

| 能力 | 锁死机制 | 代码位置 |
|---|---|---|
| Agent 写操作（创建/修改/静默告警等） | `AuthzToolWrapper` 拒绝所有 `IsReadOnly() == false` 的工具 | [registry.go](internal/agent/infrastructure/eino/registry.go) |
| 自动修复执行 | `Compensator` 只打日志（"would delete silence"），不执行 | [resilience.go](internal/alerting/infrastructure/resilience.go) |
| Budget 动作执行 | `BudgetAction` 定义了 throttle/downgrade/block，无执行实现 | [cost.go](internal/aiinfra/domain/cost.go) |

所有 Agent 工具描述明确声明 "All operations are READ-ONLY"。

---

## 六、实际可工作的子系统

| 子系统 | 能力 | 边界 |
|---|---|---|
| 告警管理 | Alertmanager client 完整：CRUD + Silence 管理 | 仅转发 Prometheus/Alertmanager 数据，平台自身不做检测判断 |
| 噪声检测 | `NoiseAnalyzer` 频率评分（fire frequency + resolve ratio） | 不是异常检测，仅识别高频重复告警 |
| SLO burn rate | 短窗口/长窗口对比，符合 Google SRE 方法论 | 仅 burn rate 计算，无趋势预测 |
| Agent OTel 追踪 | Span processor + interceptor + metrics 完整 | 仅追踪 Agent 自身的 LLM/工具调用，不覆盖业务侧 |
| Agent 意图路由 | 中文/英文关键词匹配 + 优先级池选择 | 仅路由到 3 个可用池（告警/SLO/拓扑） |
| 3 个 Tool Provider | 告警(6工具)/SLO(5工具)/拓扑(10工具) | 拓扑工具返回空数据 |

---

## 七、修复优先级

| 优先级 | 任务 | 原因 | 预估工作量 |
|---|---|---|---|
| **P0** | 实现 GPU collector（DCGM Exporter → Prometheus → 仓储） | 域模型已完整，采集层是唯一断点。没有 GPU 数据，GPU 池工具、异常检测、故障预测全无法启动 | 中 |
| **P0** | 实现推理服务 collector（vLLM metrics endpoint → 仓储） | 同理，推理指标是 AI Infra 观测的核心数据源 | 中 |
| **P1** | 实现 Topology domain/service 层（填充空 struct + service 逻辑） | Agent 工具接口已定义，只缺底层实现。填上后 10 个拓扑工具立即可用 | 中 |
| **P1** | 实现 Cost/Budget service + repository | Budget 动作定义完整，补上执行逻辑即可接入告警联动 | 小 |
| **P2** | NCCL 通信拓扑采集 + GPU 物理拓扑关联 | 需要 GPU collector 先上线，数据流打通后再做 | 大 |
| **P2** | Agent Workflow 实现（diagnose/alert_analyze 多步流程） | 需要更多可用工具（GPU/推理池）才能编排有意义的多步诊断 | 中 |
| **P3** | 统计异常检测（3-sigma / 同比环比） | 需要连续数据流（P0/P1 完成后才有意义） | 中 |
| **P3** | 变更关联归因（K8S Event + CI/CD Webhook） | topology_changes 表设计已就绪，补上采集和查询逻辑 | 中 |
| **P4** | Agent 写操作能力 + 一键修复 | 打破 ReadOnly 限制，从人工审批模式开始（P0-P3 完成后才有足够诊断能力支撑修复决策） | 大 |
| **P4** | ECC 趋势预测 + 显存泄漏检测 | 需 GPU 数据连续积累数周后才可做趋势分析 | 大 |