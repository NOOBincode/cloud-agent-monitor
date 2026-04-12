package storage

import (
	"time"

	"gorm.io/gorm"
)

// Repository MySQL 存储实现
type Repository struct {
	db *gorm.DB
}

// NewRepository 创建存储仓库
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ServiceNode 数据模型
type ServiceNode struct {
	ID          string    `gorm:"type:char(36);primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null;index"`
	Namespace   string    `gorm:"type:varchar(255);not null;index"`
	Environment string    `gorm:"type:varchar(50);index"`
	Status      string    `gorm:"type:varchar(20);index"`
	Labels      string    `gorm:"type:json"`
	RequestRate float64   `gorm:"type:double"`
	ErrorRate   float64   `gorm:"type:double"`
	LatencyP99  float64   `gorm:"type:double"`
	LatencyP95  float64   `gorm:"type:double"`
	LatencyP50  float64   `gorm:"type:double"`
	PodCount    int       `gorm:"type:int"`
	ReadyPods   int       `gorm:"type:int"`
	ServiceType string    `gorm:"type:varchar(50)"`
	Maintainer  string    `gorm:"type:varchar(255)"`
	Team        string    `gorm:"type:varchar(255)"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (ServiceNode) TableName() string {
	return "topology_service_nodes"
}

// CallEdge 数据模型
type CallEdge struct {
	ID              string    `gorm:"type:char(36);primaryKey"`
	SourceID        string    `gorm:"type:char(36);not null;index:idx_source_target"`
	TargetID        string    `gorm:"type:char(36);not null;index:idx_source_target"`
	EdgeType        string    `gorm:"type:varchar(50);index"`
	IsDirect        bool      `gorm:"type:tinyint(1)"`
	Confidence      float64   `gorm:"type:double"`
	Protocol        string    `gorm:"type:varchar(50)"`
	Method          string    `gorm:"type:varchar(50)"`
	RequestRate     float64   `gorm:"type:double"`
	ErrorRate       float64   `gorm:"type:double"`
	LatencyP99      float64   `gorm:"type:double"`
	TargetInstances string    `gorm:"type:json"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (CallEdge) TableName() string {
	return "topology_call_edges"
}

// NetworkNode 数据模型
type NetworkNode struct {
	ID          string    `gorm:"type:char(36);primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null;index"`
	Type        string    `gorm:"type:varchar(50);index"`
	Layer       string    `gorm:"type:varchar(50);index"`
	IPAddress   string    `gorm:"type:varchar(50);index"`
	CIDR        string    `gorm:"type:varchar(50)"`
	Ports       string    `gorm:"type:json"`
	Namespace   string    `gorm:"type:varchar(255);index"`
	PodName     string    `gorm:"type:varchar(255)"`
	NodeName    string    `gorm:"type:varchar(255);index"`
	Zone        string    `gorm:"type:varchar(100);index"`
	DataCenter  string    `gorm:"type:varchar(100)"`
	Connections int64     `gorm:"type:bigint"`
	BytesIn     int64     `gorm:"type:bigint"`
	BytesOut    int64     `gorm:"type:bigint"`
	PacketLoss  float64   `gorm:"type:double"`
	Latency     float64   `gorm:"type:double"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (NetworkNode) TableName() string {
	return "topology_network_nodes"
}

// NetworkEdge 数据模型
type NetworkEdge struct {
	ID              string    `gorm:"type:char(36);primaryKey"`
	SourceID        string    `gorm:"type:char(36);not null;index:idx_net_source_target"`
	TargetID        string    `gorm:"type:char(36);not null;index:idx_net_source_target"`
	SourceIP        string    `gorm:"type:varchar(50)"`
	TargetIP        string    `gorm:"type:varchar(50)"`
	SourcePort      int       `gorm:"type:int"`
	TargetPort      int       `gorm:"type:int"`
	Protocol        string    `gorm:"type:varchar(20)"`
	BytesSent       int64     `gorm:"type:bigint"`
	BytesReceived   int64     `gorm:"type:bigint"`
	PacketsSent     int64     `gorm:"type:bigint"`
	PacketsLost     int64     `gorm:"type:bigint"`
	ConnectionCount int       `gorm:"type:int"`
	Established     int       `gorm:"type:int"`
	TimeWait        int       `gorm:"type:int"`
	CloseWait       int       `gorm:"type:int"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

func (NetworkEdge) TableName() string {
	return "topology_network_edges"
}

// TopologyChange 数据模型
type TopologyChange struct {
	ID          string    `gorm:"type:char(36);primaryKey"`
	Timestamp   time.Time `gorm:"index:idx_time"`
	ChangeType  string    `gorm:"type:varchar(50);index"`
	EntityType  string    `gorm:"type:varchar(50);index"`
	EntityID    string    `gorm:"type:char(36);index"`
	EntityName  string    `gorm:"type:varchar(255)"`
	Description string    `gorm:"type:text"`
	BeforeState string    `gorm:"type:json"`
	AfterState  string    `gorm:"type:json"`
}

func (TopologyChange) TableName() string {
	return "topology_changes"
}

// TopologySnapshot 数据模型
type TopologySnapshot struct {
	ID        string    `gorm:"type:char(36);primaryKey"`
	GraphType string    `gorm:"type:varchar(50);not null;index:idx_type_time"`
	Timestamp time.Time `gorm:"not null;index:idx_type_time"`
	Hash      string    `gorm:"type:varchar(64)"`
	Nodes     string    `gorm:"type:longtext"`
	Edges     string    `gorm:"type:longtext"`
}

func (TopologySnapshot) TableName() string {
	return "topology_snapshots"
}
