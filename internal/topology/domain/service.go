package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DiscoveryBackend 定义拓扑发现后端接口
type DiscoveryBackend interface {
	// 发现服务节点
	DiscoverNodes(ctx context.Context) ([]*ServiceNode, error)
	// 发现调用边
	DiscoverEdges(ctx context.Context) ([]*CallEdge, error)
	// 发现网络节点
	DiscoverNetworkNodes(ctx context.Context) ([]*NetworkNode, error)
	// 发现网络边
	DiscoverNetworkEdges(ctx context.Context) ([]*NetworkEdge, error)
	// 健康检查
	HealthCheck(ctx context.Context) error
}

// TopologyService 定义拓扑服务接口
type TopologyService interface {
	// 服务拓扑查询
	GetServiceTopology(ctx context.Context, query TopologyQuery) (*ServiceTopology, error)
	GetServiceNode(ctx context.Context, id uuid.UUID) (*ServiceNode, error)
	GetServiceNodeByName(ctx context.Context, namespace, name string) (*ServiceNode, error)

	// 网络拓扑查询
	GetNetworkTopology(ctx context.Context, query TopologyQuery) (*NetworkTopology, error)
	GetNetworkNode(ctx context.Context, id uuid.UUID) (*NetworkNode, error)

	// 影响分析
	AnalyzeImpact(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*ImpactResult, error)
	AnalyzeImpactBatch(ctx context.Context, serviceIDs []uuid.UUID, maxDepth int) (map[uuid.UUID]*ImpactResult, error)

	// 路径查找
	FindPath(ctx context.Context, sourceID, targetID uuid.UUID, maxHops int) (*PathResult, error)
	FindShortestPath(ctx context.Context, sourceID, targetID uuid.UUID) ([]PathHop, error)

	// 上游/下游查询
	GetUpstreamServices(ctx context.Context, serviceID uuid.UUID, depth int) ([]*ServiceNode, error)
	GetDownstreamServices(ctx context.Context, serviceID uuid.UUID, depth int) ([]*ServiceNode, error)

	// 拓扑刷新
	RefreshServiceTopology(ctx context.Context) error
	RefreshNetworkTopology(ctx context.Context) error

	// 历史查询
	GetTopologyAtTime(ctx context.Context, timestamp time.Time) (*ServiceTopology, error)
	GetTopologyChanges(ctx context.Context, from, to time.Time) ([]*TopologyChange, error)

	// 异常检测
	FindAnomalies(ctx context.Context) ([]*TopologyAnomaly, error)

	// 统计信息
	GetTopologyStats(ctx context.Context) (*TopologyStats, error)
}

// TopologyStats 表示拓扑统计信息
type TopologyStats struct {
	ServiceNodeCount  int           `json:"service_node_count"`
	ServiceEdgeCount  int           `json:"service_edge_count"`
	NetworkNodeCount  int           `json:"network_node_count"`
	NetworkEdgeCount  int           `json:"network_edge_count"`
	NamespaceCount    int           `json:"namespace_count"`
	LastDiscoveryTime time.Time     `json:"last_discovery_time"`
	DataFreshness     time.Duration `json:"data_freshness"`
}
