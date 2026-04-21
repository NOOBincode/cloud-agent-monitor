// Package domain 定义拓扑模块的核心领域模型和业务规则。
//
// 该包包含服务拓扑和网络拓扑的数据结构定义，以及拓扑分析相关的
// 错误类型、查询参数和仓储接口。所有类型设计遵循 DDD 原则，
// 确保领域模型的纯粹性和业务语义的清晰表达。
//
// 核心概念:
//   - ServiceNode: 服务拓扑节点，表示一个 K8s Service 或应用服务
//   - CallEdge: 服务调用边，表示服务间的依赖关系
//   - NetworkNode: 网络拓扑节点，表示 Pod/Node/Ingress 等网络实体
//   - NetworkEdge: 网络连接边，表示网络层的连通关系
//
// 使用示例:
//
//	node := &domain.ServiceNode{
//	    ID:        uuid.New(),
//	    Name:      "user-service",
//	    Namespace: "production",
//	    Status:    domain.ServiceStatusHealthy,
//	}
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ServiceStatus 表示服务的健康状态。
// 状态值基于 K8s Pod 状态和 Prometheus 指标综合判定。
type ServiceStatus string

const (
	// ServiceStatusHealthy 表示服务健康：所有 Pod 就绪，错误率低于阈值
	ServiceStatusHealthy ServiceStatus = "healthy"
	// ServiceStatusUnhealthy 表示服务不健康：无就绪 Pod 或错误率过高
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	// ServiceStatusWarning 表示服务警告：部分 Pod 不健康或延迟较高
	ServiceStatusWarning ServiceStatus = "warning"
	// ServiceStatusUnknown 表示状态未知：无法获取服务状态信息
	ServiceStatusUnknown ServiceStatus = "unknown"
)

// EdgeType 表示服务间调用关系的类型。
// 不同类型的边对应不同的发现机制和置信度。
type EdgeType string

const (
	// EdgeTypeHTTP 表示 HTTP/REST 调用关系，通过 Prometheus HTTP 客户端指标发现
	EdgeTypeHTTP EdgeType = "http"
	// EdgeTypeGRPC 表示 gRPC 调用关系，通过 gRPC 客户端指标发现
	EdgeTypeGRPC EdgeType = "grpc"
	// EdgeTypeDatabase 表示数据库连接关系，通过数据库连接池指标发现
	EdgeTypeDatabase EdgeType = "database"
	// EdgeTypeCache 表示缓存访问关系，通过 Redis/Memcached 客户端指标发现
	EdgeTypeCache EdgeType = "cache"
	// EdgeTypeMessageQueue 表示消息队列关系，通过消息生产/消费指标发现
	EdgeTypeMessageQueue EdgeType = "mq"
	// EdgeTypeIndirect 表示间接推断的关系，通过环境变量/配置分析发现，置信度较低
	EdgeTypeIndirect EdgeType = "indirect"
)

// NetworkLayer 表示网络拓扑的层级。
// 用于区分不同抽象层次的网络实体。
type NetworkLayer string

const (
	// NetworkLayerPod 表示 Pod 层级网络实体
	NetworkLayerPod NetworkLayer = "pod"
	// NetworkLayerNode 表示 Node 层级网络实体
	NetworkLayerNode NetworkLayer = "node"
	// NetworkLayerCluster 表示集群层级网络实体
	NetworkLayerCluster NetworkLayer = "cluster"
	// NetworkLayerIngress 表示入口层级网络实体
	NetworkLayerIngress NetworkLayer = "ingress"
	// NetworkLayerExternal 表示外部网络实体
	NetworkLayerExternal NetworkLayer = "external"
)

// ServiceImportance 表示服务的重要性级别。
// 用于加权图算法，核心服务故障影响更大。
type ServiceImportance string

const (
	// ImportanceCritical 核心服务：故障会导致整个系统不可用
	// 例如：认证服务、网关、核心业务服务
	ImportanceCritical ServiceImportance = "critical"
	// ImportanceImportant 重要服务：故障会影响部分功能
	// 例如：订单服务、支付服务
	ImportanceImportant ServiceImportance = "important"
	// ImportanceNormal 普通服务：故障影响有限
	// 例如：评论服务、通知服务
	ImportanceNormal ServiceImportance = "normal"
	// ImportanceEdge 边缘服务：故障几乎不影响主流程
	// 例如：日志收集、监控上报
	ImportanceEdge ServiceImportance = "edge"
)

// ImportanceWeight 返回重要性对应的权重值
//
// TODO: 实现重要性权重映射
// 提示：
// - critical: 1.0 (最高权重)
// - important: 0.75
// - normal: 0.5
// - edge: 0.25
func (i ServiceImportance) Weight() float64 {
	// TODO: 实现权重映射
	// 骨架代码：
	// switch i {
	// case ImportanceCritical:
	//     return 1.0
	// case ImportanceImportant:
	//     return 0.75
	// case ImportanceNormal:
	//     return 0.5
	// case ImportanceEdge:
	//     return 0.25
	// default:
	//     return 0.5
	// }
	return 0.5
}

// ServiceNode 表示服务拓扑中的一个节点。
//
// 节点对应 Kubernetes 中的一个 Service 资源，包含服务的基本信息、
// 实时性能指标和业务元数据。节点状态由 Pod 健康状态和 Prometheus
// 指标综合计算得出。
//
// 字段说明:
//   - ID: 基于 namespace/name 生成的确定性 UUID，保证同一服务 ID 不变
//   - RequestRate: 最近 5 分钟的平均 QPS
//   - ErrorRate: 最近 5 分钟的错误率百分比 (0-100)
//   - LatencyP99/P95/P50: 延迟百分位值，单位毫秒
type ServiceNode struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Environment string            `json:"environment"`
	Status      ServiceStatus     `json:"status"`
	Labels      map[string]string `json:"labels"`

	RequestRate float64 `json:"request_rate"`
	ErrorRate   float64 `json:"error_rate"`
	LatencyP99  float64 `json:"latency_p99"`
	LatencyP95  float64 `json:"latency_p95"`
	LatencyP50  float64 `json:"latency_p50"`

	PodCount    int    `json:"pod_count"`
	ReadyPods   int    `json:"ready_pods"`
	ServiceType string `json:"service_type"`

	// Importance 服务重要性级别，用于加权图算法
	// 可从 K8s Label "importance.cloud-agent.io/level" 读取
	Importance ServiceImportance `json:"importance"`
	// Weight 自定义权重 (0-1)，优先级高于 Importance
	// 用于特殊场景下的精细控制
	Weight float64 `json:"weight,omitempty"`

	Maintainer string `json:"maintainer"`
	Team       string `json:"team"`

	UpdatedAt time.Time `json:"updated_at"`
}

// CallEdge 表示服务间的调用关系边。
//
// 边描述了源服务到目标服务的依赖关系，包含调用类型、置信度、
// 协议信息和流量指标。边的发现来源包括：
//   - Prometheus HTTP/gRPC 客户端指标（高置信度）
//   - 数据库连接池指标（高置信度）
//   - 环境变量和配置分析（低置信度）
//
// 字段说明:
//   - IsDirect: true 表示直接调用，false 表示间接推断
//   - Confidence: 置信度 0-1，HTTP/gRPC 边通常为 0.9，间接推断边通常为 0.5-0.6
//   - RequestRate: 该调用链路的 QPS
type CallEdge struct {
	ID       uuid.UUID `json:"id"`
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`

	EdgeType   EdgeType `json:"edge_type"`
	IsDirect   bool     `json:"is_direct"`
	Confidence float64  `json:"confidence"`
	Protocol   string   `json:"protocol"`
	Method     string   `json:"method"`

	RequestRate float64 `json:"request_rate"`
	ErrorRate   float64 `json:"error_rate"`
	LatencyP99  float64 `json:"latency_p99"`

	// Weight 边权重，用于加权最短路径算法
	// 计算公式：weight = latency_factor * error_factor * type_factor
	// 权重越高表示该边"代价越大"，路径查找时应尽量避开
	Weight float64 `json:"weight,omitempty"`

	TargetInstances []string `json:"target_instances,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceTopology 表示某一时刻的服务拓扑快照。
// 包含完整的节点和边信息，用于历史查询和变更对比。
type ServiceTopology struct {
	ID        uuid.UUID      `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Nodes     []*ServiceNode `json:"nodes"`
	Edges     []*CallEdge    `json:"edges"`
	Hash      string         `json:"hash"`
}

// NetworkNode 表示网络拓扑中的一个节点。
//
// 网络节点可以是 Pod、Node、Service、Ingress 或外部服务，
// 通过 Layer 字段区分层级。每个节点包含网络配置信息和流量指标。
//
// 字段说明:
//   - Type: 节点类型，如 "pod"/"service"/"node"/"ingress"
//   - Layer: 网络层级，用于拓扑分层展示
//   - PacketLoss: 丢包率百分比 (0-100)
//   - Latency: 网络延迟，单位毫秒
type NetworkNode struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Layer     NetworkLayer `json:"layer"`
	IPAddress string       `json:"ip_address"`
	CIDR      string       `json:"cidr,omitempty"`
	Ports     []int        `json:"ports,omitempty"`

	Namespace  string `json:"namespace,omitempty"`
	PodName    string `json:"pod_name,omitempty"`
	NodeName   string `json:"node_name,omitempty"`
	Zone       string `json:"zone,omitempty"`
	DataCenter string `json:"data_center,omitempty"`

	Connections int64   `json:"connections"`
	BytesIn     int64   `json:"bytes_in"`
	BytesOut    int64   `json:"bytes_out"`
	PacketsIn   int64   `json:"packets_in"`
	PacketsOut  int64   `json:"packets_out"`
	PacketLoss  float64 `json:"packet_loss"`
	Latency     float64 `json:"latency_ms"`

	UpdatedAt time.Time `json:"updated_at"`
}

// NetworkEdge 表示网络层的连接关系。
//
// 网络边描述两个网络实体之间的连接，包含连接信息、流量指标
// 和连接状态统计。主要用于网络问题诊断和流量分析。
//
// 字段说明:
//   - Protocol: 传输层协议，如 "TCP"/"UDP"/"ICMP"
//   - Established/TimeWait/CloseWait: TCP 连接状态统计
type NetworkEdge struct {
	ID       uuid.UUID `json:"id"`
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`

	SourceIP   string `json:"source_ip"`
	TargetIP   string `json:"target_ip"`
	SourcePort int    `json:"source_port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"`

	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`
	PacketsSent   int64 `json:"packets_sent"`
	PacketsLost   int64 `json:"packets_lost"`

	ConnectionCount int `json:"connection_count"`
	Established     int `json:"established"`
	TimeWait        int `json:"time_wait"`
	CloseWait       int `json:"close_wait"`

	UpdatedAt time.Time `json:"updated_at"`
}

// NetworkTopology 表示某一时刻的网络拓扑快照。
type NetworkTopology struct {
	ID        uuid.UUID      `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Nodes     []*NetworkNode `json:"nodes"`
	Edges     []*NetworkEdge `json:"edges"`
	Hash      string         `json:"hash"`
}

// ImpactLevel 表示服务故障的影响级别。
// 级别基于受影响服务的数量、重要性和调用深度综合判定。
type ImpactLevel string

const (
	// ImpactLevelCritical 表示关键影响：核心服务受影响，可能导致系统不可用
	ImpactLevelCritical ImpactLevel = "critical"
	// ImpactLevelHigh 表示高影响：多个重要服务受影响
	ImpactLevelHigh ImpactLevel = "high"
	// ImpactLevelMedium 表示中等影响：部分服务受影响
	ImpactLevelMedium ImpactLevel = "medium"
	// ImpactLevelLow 表示低影响：仅边缘服务受影响
	ImpactLevelLow ImpactLevel = "low"
)

// ImpactResult 表示服务故障影响分析结果。
//
// 当某个服务发生故障时，ImpactResult 描述了该故障对上下游服务的影响范围。
// 分析基于服务调用图进行 BFS 遍历，计算受影响的服务列表和关键路径。
//
// 字段说明:
//   - Upstream: 依赖故障服务的上游服务（调用方）
//   - Downstream: 故障服务依赖的下游服务（被调用方）
//   - CriticalPath: 从入口服务到故障服务的关键调用路径
//   - AffectedServices: 按影响级别分类的受影响服务列表
type ImpactResult struct {
	RootService      *ServiceNode      `json:"root_service"`
	RootServiceID    uuid.UUID         `json:"root_service_id"`
	RootServiceName  string            `json:"root_service_name"`
	TotalAffected    int               `json:"total_affected"`
	UpstreamDepth    int               `json:"upstream_depth"`
	DownstreamDepth  int               `json:"downstream_depth"`
	Upstream         []*ImpactNode     `json:"upstream"`
	Downstream       []*ImpactNode     `json:"downstream"`
	CriticalPath     []PathHop         `json:"critical_path"`
	AnalyzedAt       time.Time         `json:"analyzed_at"`
	AffectedServices []AffectedService `json:"affected_services"`
}

// ImpactNode 表示受影响的服务节点及其影响深度。
type ImpactNode struct {
	Node       *ServiceNode `json:"node"`
	Depth      int          `json:"depth"`
	Impact     float64      `json:"impact"`
	IsCritical bool         `json:"is_critical"`
}

// AffectedService 表示受影响的服务详情。
// 包含服务标识、影响级别和从根服务到该服务的传播路径。
type AffectedService struct {
	ServiceID   uuid.UUID   `json:"service_id"`
	ServiceName string      `json:"service_name"`
	ImpactLevel ImpactLevel `json:"impact_level"`
	ImpactPath  []string    `json:"impact_path"`
	HopCount    int         `json:"hop_count"`
}

// PathResult 表示两个服务间的路径查找结果。
//
// 路径查找支持查找所有可能路径和最短路径。最短路径基于
// 边的权重（默认为延迟）使用 Dijkstra 算法计算。
//
// 字段说明:
//   - Paths: 所有找到的路径（受 maxHops 限制）
//   - ShortestPath: 延迟最小的路径
//   - ShortestHops: 最短路径的跳数
type PathResult struct {
	SourceID     uuid.UUID   `json:"source_id"`
	TargetID     uuid.UUID   `json:"target_id"`
	SourceName   string      `json:"source_name"`
	TargetName   string      `json:"target_name"`
	Paths        [][]PathHop `json:"paths"`
	ShortestPath []PathHop   `json:"shortest_path"`
	ShortestHops int         `json:"shortest_hops"`
	FoundAt      time.Time   `json:"found_at"`
}

// PathHop 表示服务调用路径中的一跳。
// 包含节点信息和边的延迟信息。
type PathHop struct {
	NodeID    uuid.UUID `json:"node_id"`
	NodeName  string    `json:"node_name"`
	Namespace string    `json:"namespace"`
	EdgeID    uuid.UUID `json:"edge_id"`
	Latency   float64   `json:"latency_ms"`
}

// TopologyAnomaly 表示拓扑中检测到的异常。
//
// 异常类型包括：
//   - unhealthy_service: 服务状态不健康
//   - high_error_rate: 错误率超过阈值 (默认 5%)
//   - high_latency: P99 延迟超过阈值 (默认 1000ms)
//   - pod_degradation: Pod 数量下降
//   - circular_dependency: 循环依赖
//   - orphan_service: 孤立服务（无上下游）
type TopologyAnomaly struct {
	ID          uuid.UUID          `json:"id"`
	Type        string             `json:"type"`
	NodeID      uuid.UUID          `json:"node_id"`
	NodeName    string             `json:"node_name"`
	Namespace   string             `json:"namespace"`
	Description string             `json:"description"`
	Severity    string             `json:"severity"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	DetectedAt  time.Time          `json:"detected_at"`
	RelatedIDs  []uuid.UUID        `json:"related_ids"`
}

// TopologyChange 表示拓扑结构的变化记录。
// 用于追踪拓扑的演进历史和变更审计。
type TopologyChange struct {
	ID          uuid.UUID `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	ChangeType  string    `json:"change_type"`
	EntityType  string    `json:"entity_type"`
	EntityID    uuid.UUID `json:"entity_id"`
	EntityName  string    `json:"entity_name"`
	Description string    `json:"description"`
	BeforeState string    `json:"before_state,omitempty"`
	AfterState  string    `json:"after_state,omitempty"`
}

// TopologyQuery 表示拓扑查询的过滤和分页参数。
//
// 查询支持多种过滤条件组合：
//   - Namespace/Namespaces: 按命名空间过滤
//   - ServiceName/ServicePrefix: 按服务名称过滤
//   - NodeTypes/EdgeTypes: 按节点/边类型过滤
//   - Labels: 按 K8s 标签过滤
//   - ActiveOnly: 仅返回有流量的服务
//   - MinRequestRate: 按 QPS 过滤
//
// 时间范围查询用于获取历史拓扑快照。
type TopologyQuery struct {
	Namespace     string            `json:"namespace,omitempty"`
	Namespaces    []string          `json:"namespaces,omitempty"`
	ServiceName   string            `json:"service_name,omitempty"`
	ServicePrefix string            `json:"service_prefix,omitempty"`
	NodeTypes     []string          `json:"node_types,omitempty"`
	EdgeTypes     []EdgeType        `json:"edge_types,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`

	Depth    int `json:"depth,omitempty"`
	MaxDepth int `json:"max_depth,omitempty"`

	ActiveOnly     bool    `json:"active_only,omitempty"`
	MinRequestRate float64 `json:"min_request_rate,omitempty"`

	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
	Limit    int `json:"limit,omitempty"`
	Offset   int `json:"offset,omitempty"`

	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

func (q *TopologyQuery) HasNamespace() bool {
	return q.Namespace != "" || len(q.Namespaces) > 0
}

func (q *TopologyQuery) HasLabels() bool {
	return len(q.Labels) > 0
}

func (q *TopologyQuery) HasTimeRange() bool {
	return q.StartTime != nil && q.EndTime != nil
}

func (q *TopologyQuery) GetLimit() int {
	if q.Limit <= 0 {
		return 100
	}
	if q.Limit > 10000 {
		return 10000
	}
	return q.Limit
}

func (q *TopologyQuery) GetDepth() int {
	if q.Depth <= 0 {
		return 3
	}
	if q.Depth > 10 {
		return 10
	}
	return q.Depth
}

// GetEffectiveWeight 获取服务节点的有效权重
//
// TODO: 实现有效权重计算
// 优先级：Weight > Importance.Weight() > 默认值 0.5
// 提示：
// 1. 如果 n.Weight > 0，直接返回 n.Weight
// 2. 否则返回 n.Importance.Weight()
func (n *ServiceNode) GetEffectiveWeight() float64 {
	// TODO: 实现有效权重计算
	// 骨架代码：
	// if n.Weight > 0 {
	//     return n.Weight
	// }
	// return n.Importance.Weight()
	return 0.5
}

// CalculateEdgeWeight 计算边的权重
//
// TODO: 实现边权重计算
// 权重公式：weight = latency_factor * error_factor * type_factor
// 其中：
// - latency_factor = 1 + (LatencyP99 / 1000) // 延迟越高权重越大
// - error_factor = 1 + ErrorRate // 错误率越高权重越大
// - type_factor 根据边类型确定：
//   - HTTP/gRPC: 1.0 (正常)
//   - Database: 1.2 (数据库调用代价更高)
//   - Cache: 0.8 (缓存调用代价较低)
//   - MQ: 1.1 (消息队列)
//   - Indirect: 1.5 (间接推断，不确定性高)
func (e *CallEdge) CalculateEdgeWeight() float64 {
	// TODO: 实现边权重计算
	// 骨架代码：
	// latencyFactor := 1.0 + e.LatencyP99/1000.0
	// errorFactor := 1.0 + e.ErrorRate
	// typeFactor := 1.0
	// switch e.EdgeType {
	// case EdgeTypeHTTP, EdgeTypeGRPC:
	//     typeFactor = 1.0
	// case EdgeTypeDatabase:
	//     typeFactor = 1.2
	// case EdgeTypeCache:
	//     typeFactor = 0.8
	// case EdgeTypeMessageQueue:
	//     typeFactor = 1.1
	// case EdgeTypeIndirect:
	//     typeFactor = 1.5
	// }
	// return latencyFactor * errorFactor * typeFactor
	return 1.0
}

// WeightedImpactResult 加权影响分析结果
// 在原有 ImpactResult 基础上增加加权分数
type WeightedImpactResult struct {
	*ImpactResult
	// WeightedScore 加权影响分数 (0-100)
	// 综合考虑服务重要性、调用深度、流量大小
	WeightedScore float64 `json:"weighted_score"`
	// CriticalServices 关键受影响服务列表（重要性 >= Important）
	CriticalServices []*ImpactNode `json:"critical_services"`
	// ImpactByImportance 按重要性分组的影响统计
	ImpactByImportance map[ServiceImportance]int `json:"impact_by_importance"`
}

// WeightedPathResult 加权路径结果
// 在原有 PathResult 基础上增加加权信息
type WeightedPathResult struct {
	*PathResult
	// TotalWeight 路径总权重（所有边权重之和）
	TotalWeight float64 `json:"total_weight"`
	// AverageLatency 平均延迟
	AverageLatency float64 `json:"average_latency_ms"`
	// MaxErrorRate 路径上最大错误率
	MaxErrorRate float64 `json:"max_error_rate"`
}
