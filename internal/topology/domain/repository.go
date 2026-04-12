package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TopologyRepository 定义拓扑数据访问接口
type TopologyRepository interface {
	// 服务节点操作
	SaveServiceNode(ctx context.Context, node *ServiceNode) error
	BatchSaveServiceNodes(ctx context.Context, nodes []*ServiceNode) error
	GetServiceNode(ctx context.Context, id uuid.UUID) (*ServiceNode, error)
	GetServiceNodeByName(ctx context.Context, namespace, name string) (*ServiceNode, error)
	ListServiceNodes(ctx context.Context, query TopologyQuery) ([]*ServiceNode, int64, error)
	DeleteServiceNode(ctx context.Context, id uuid.UUID) error

	// 调用边操作
	SaveCallEdge(ctx context.Context, edge *CallEdge) error
	BatchSaveCallEdges(ctx context.Context, edges []*CallEdge) error
	GetCallEdge(ctx context.Context, id uuid.UUID) (*CallEdge, error)
	GetCallEdgeByEndpoints(ctx context.Context, sourceID, targetID uuid.UUID) (*CallEdge, error)
	ListCallEdges(ctx context.Context, query TopologyQuery) ([]*CallEdge, int64, error)
	ListCallEdgesBySource(ctx context.Context, sourceID uuid.UUID) ([]*CallEdge, error)
	ListCallEdgesByTarget(ctx context.Context, targetID uuid.UUID) ([]*CallEdge, error)
	DeleteCallEdge(ctx context.Context, id uuid.UUID) error

	// 网络节点操作
	SaveNetworkNode(ctx context.Context, node *NetworkNode) error
	BatchSaveNetworkNodes(ctx context.Context, nodes []*NetworkNode) error
	GetNetworkNode(ctx context.Context, id uuid.UUID) (*NetworkNode, error)
	GetNetworkNodeByIP(ctx context.Context, ip string) (*NetworkNode, error)
	ListNetworkNodes(ctx context.Context, query TopologyQuery) ([]*NetworkNode, int64, error)
	DeleteNetworkNode(ctx context.Context, id uuid.UUID) error

	// 网络边操作
	SaveNetworkEdge(ctx context.Context, edge *NetworkEdge) error
	BatchSaveNetworkEdges(ctx context.Context, edges []*NetworkEdge) error
	GetNetworkEdge(ctx context.Context, id uuid.UUID) (*NetworkEdge, error)
	ListNetworkEdges(ctx context.Context, query TopologyQuery) ([]*NetworkEdge, int64, error)
	DeleteNetworkEdge(ctx context.Context, id uuid.UUID) error

	// 拓扑变化记录
	SaveTopologyChange(ctx context.Context, change *TopologyChange) error
	ListTopologyChanges(ctx context.Context, from, to time.Time, entityType string) ([]*TopologyChange, error)

	// 拓扑快照
	SaveTopologySnapshot(ctx context.Context, graphType string, snapshot *ServiceTopology) error
	GetTopologySnapshot(ctx context.Context, graphType string, timestamp time.Time) (*ServiceTopology, error)

	// 批量清理
	CleanupOldData(ctx context.Context, retention time.Duration) error
}

// TopologyCache 定义拓扑缓存接口
type TopologyCache interface {
	// 服务拓扑缓存
	GetServiceTopology(ctx context.Context) (*ServiceTopology, error)
	SetServiceTopology(ctx context.Context, topology *ServiceTopology, ttl time.Duration) error

	// 网络拓扑缓存
	GetNetworkTopology(ctx context.Context) (*NetworkTopology, error)
	SetNetworkTopology(ctx context.Context, topology *NetworkTopology, ttl time.Duration) error

	// 节点缓存
	GetNode(ctx context.Context, id uuid.UUID) (*ServiceNode, error)
	SetNode(ctx context.Context, node *ServiceNode, ttl time.Duration) error
	DeleteNode(ctx context.Context, id uuid.UUID) error

	// 边缓存
	GetEdge(ctx context.Context, id uuid.UUID) (*CallEdge, error)
	SetEdge(ctx context.Context, edge *CallEdge, ttl time.Duration) error

	// 邻接表缓存（用于快速遍历）
	GetAdjacencyList(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error)
	SetAdjacencyList(ctx context.Context, adjList map[uuid.UUID][]uuid.UUID, ttl time.Duration) error

	// 影响范围缓存
	GetImpactCache(ctx context.Context, serviceID uuid.UUID) (*ImpactResult, error)
	SetImpactCache(ctx context.Context, serviceID uuid.UUID, result *ImpactResult, ttl time.Duration) error

	// 清除缓存
	InvalidateAll(ctx context.Context) error
	InvalidateService(ctx context.Context, serviceID uuid.UUID) error
}
