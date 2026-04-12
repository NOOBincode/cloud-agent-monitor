package domain

import (
	"time"

	"github.com/google/uuid"
)

// ServiceStatus 表示服务状态
type ServiceStatus string

const (
	ServiceStatusHealthy   ServiceStatus = "healthy"
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	ServiceStatusWarning   ServiceStatus = "warning"
	ServiceStatusUnknown   ServiceStatus = "unknown"
)

// EdgeType 表示边的类型
type EdgeType string

const (
	EdgeTypeHTTP         EdgeType = "http"
	EdgeTypeGRPC         EdgeType = "grpc"
	EdgeTypeDatabase     EdgeType = "database"
	EdgeTypeCache        EdgeType = "cache"
	EdgeTypeMessageQueue EdgeType = "mq"
	EdgeTypeIndirect     EdgeType = "indirect"
)

// NetworkLayer 表示网络层级
type NetworkLayer string

const (
	NetworkLayerPod      NetworkLayer = "pod"
	NetworkLayerNode     NetworkLayer = "node"
	NetworkLayerCluster  NetworkLayer = "cluster"
	NetworkLayerIngress  NetworkLayer = "ingress"
	NetworkLayerExternal NetworkLayer = "external"
)

// ServiceNode 表示服务拓扑中的节点
type ServiceNode struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Environment string            `json:"environment"`
	Status      ServiceStatus     `json:"status"`
	Labels      map[string]string `json:"labels"`

	// 实时指标
	RequestRate float64 `json:"request_rate"` // QPS
	ErrorRate   float64 `json:"error_rate"`   // 错误率
	LatencyP99  float64 `json:"latency_p99"`  // P99 延迟
	LatencyP95  float64 `json:"latency_p95"`  // P95 延迟
	LatencyP50  float64 `json:"latency_p50"`  // P50 延迟

	// K8s 元数据
	PodCount    int    `json:"pod_count"`
	ReadyPods   int    `json:"ready_pods"`
	ServiceType string `json:"service_type"` // ClusterIP/NodePort/LoadBalancer

	// 业务元数据
	Maintainer string `json:"maintainer"`
	Team       string `json:"team"`

	UpdatedAt time.Time `json:"updated_at"`
}

// CallEdge 表示服务间的调用关系
type CallEdge struct {
	ID       uuid.UUID `json:"id"`
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`

	// 边的属性
	EdgeType   EdgeType `json:"edge_type"`
	IsDirect   bool     `json:"is_direct"`  // 是否直接调用
	Confidence float64  `json:"confidence"` // 置信度 0-1
	Protocol   string   `json:"protocol"`   // HTTP/1.1, HTTP/2, gRPC
	Method     string   `json:"method"`     // GET/POST/Publish/Consume

	// 流量指标
	RequestRate float64 `json:"request_rate"`
	ErrorRate   float64 `json:"error_rate"`
	LatencyP99  float64 `json:"latency_p99"`

	// 目标实例（用于负载均衡场景）
	TargetInstances []string `json:"target_instances,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
}

// ServiceTopology 表示服务拓扑图
type ServiceTopology struct {
	ID        uuid.UUID      `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Nodes     []*ServiceNode `json:"nodes"`
	Edges     []*CallEdge    `json:"edges"`
	Hash      string         `json:"hash"` // 用于增量更新
}

// NetworkNode 表示网络拓扑中的节点
type NetworkNode struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`  // pod/service/node/ingress
	Layer     NetworkLayer `json:"layer"` // pod/node/cluster/ingress/external
	IPAddress string       `json:"ip_address"`
	CIDR      string       `json:"cidr,omitempty"`
	Ports     []int        `json:"ports,omitempty"`

	// K8s 元数据
	Namespace  string `json:"namespace,omitempty"`
	PodName    string `json:"pod_name,omitempty"`
	NodeName   string `json:"node_name,omitempty"`
	Zone       string `json:"zone,omitempty"`
	DataCenter string `json:"data_center,omitempty"`

	// 网络指标
	Connections int64   `json:"connections"`
	BytesIn     int64   `json:"bytes_in"`
	BytesOut    int64   `json:"bytes_out"`
	PacketsIn   int64   `json:"packets_in"`
	PacketsOut  int64   `json:"packets_out"`
	PacketLoss  float64 `json:"packet_loss"` // 丢包率
	Latency     float64 `json:"latency_ms"`  // 网络延迟

	UpdatedAt time.Time `json:"updated_at"`
}

// NetworkEdge 表示网络连接
type NetworkEdge struct {
	ID       uuid.UUID `json:"id"`
	SourceID uuid.UUID `json:"source_id"`
	TargetID uuid.UUID `json:"target_id"`

	// 连接信息
	SourceIP   string `json:"source_ip"`
	TargetIP   string `json:"target_ip"`
	SourcePort int    `json:"source_port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol"` // TCP/UDP/ICMP

	// 流量指标
	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`
	PacketsSent   int64 `json:"packets_sent"`
	PacketsLost   int64 `json:"packets_lost"`

	// 连接状态
	ConnectionCount int `json:"connection_count"`
	Established     int `json:"established"`
	TimeWait        int `json:"time_wait"`
	CloseWait       int `json:"close_wait"`

	UpdatedAt time.Time `json:"updated_at"`
}

// NetworkTopology 表示网络拓扑图
type NetworkTopology struct {
	ID        uuid.UUID      `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Nodes     []*NetworkNode `json:"nodes"`
	Edges     []*NetworkEdge `json:"edges"`
	Hash      string         `json:"hash"`
}

// ImpactLevel 表示影响级别
type ImpactLevel string

const (
	ImpactLevelCritical ImpactLevel = "critical"
	ImpactLevelHigh     ImpactLevel = "high"
	ImpactLevelMedium   ImpactLevel = "medium"
	ImpactLevelLow      ImpactLevel = "low"
)

// ImpactResult 表示影响分析结果
type ImpactResult struct {
	RootServiceID    uuid.UUID         `json:"root_service_id"`
	RootServiceName  string            `json:"root_service_name"`
	TotalAffected    int               `json:"total_affected"`
	AffectedServices []AffectedService `json:"affected_services"`
}

// AffectedService 表示受影响的服务
type AffectedService struct {
	ServiceID   uuid.UUID   `json:"service_id"`
	ServiceName string      `json:"service_name"`
	ImpactLevel ImpactLevel `json:"impact_level"`
	ImpactPath  []string    `json:"impact_path"` // 从根服务到该服务的路径
	HopCount    int         `json:"hop_count"`   // 距离根服务的跳数
}

// PathResult 表示路径查找结果
type PathResult struct {
	SourceID     uuid.UUID   `json:"source_id"`
	TargetID     uuid.UUID   `json:"target_id"`
	SourceName   string      `json:"source_name"`
	TargetName   string      `json:"target_name"`
	Paths        [][]PathHop `json:"paths"` // 所有可能的路径
	ShortestHops int         `json:"shortest_hops"`
}

// PathHop 表示路径中的一跳
type PathHop struct {
	NodeID   uuid.UUID `json:"node_id"`
	NodeName string    `json:"node_name"`
	EdgeID   uuid.UUID `json:"edge_id"`
	Latency  float64   `json:"latency_ms"`
}

// TopologyChange 表示拓扑变化
type TopologyChange struct {
	ID          uuid.UUID `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	ChangeType  string    `json:"change_type"` // add/remove/modify
	EntityType  string    `json:"entity_type"` // node/edge
	EntityID    uuid.UUID `json:"entity_id"`
	EntityName  string    `json:"entity_name"`
	Description string    `json:"description"`
	BeforeState string    `json:"before_state,omitempty"`
	AfterState  string    `json:"after_state,omitempty"`
}

// TopologyQuery 表示拓扑查询参数
type TopologyQuery struct {
	// 过滤条件
	Namespaces    []string   `json:"namespaces,omitempty"`
	ServicePrefix string     `json:"service_prefix,omitempty"`
	NodeTypes     []string   `json:"node_types,omitempty"`
	EdgeTypes     []EdgeType `json:"edge_types,omitempty"`

	// 深度控制
	MaxDepth int `json:"max_depth,omitempty"`

	// 流量过滤
	ActiveOnly     bool    `json:"active_only,omitempty"`
	MinRequestRate float64 `json:"min_request_rate,omitempty"`

	// 分页
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`

	// 时间（查询历史拓扑）
	Timestamp *time.Time `json:"timestamp,omitempty"`
}
