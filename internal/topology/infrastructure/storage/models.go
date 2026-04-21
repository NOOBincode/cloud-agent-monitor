package storage

import (
	"time"

	"gorm.io/gorm"
)

type ServiceNode struct {
	ID          string `gorm:"primaryKey;type:varchar(36)"`
	Name        string `gorm:"type:varchar(255);index"`
	Namespace   string `gorm:"type:varchar(255);index"`
	Environment string `gorm:"type:varchar(100)"`
	Status      string `gorm:"type:varchar(50)"`
	Labels      string `gorm:"type:text"`
	RequestRate float64
	ErrorRate   float64
	LatencyP99  float64
	LatencyP95  float64
	LatencyP50  float64
	PodCount    int
	ReadyPods   int
	ServiceType string `gorm:"type:varchar(50)"`
	Maintainer  string `gorm:"type:varchar(255)"`
	Team        string `gorm:"type:varchar(255)"`
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

func (ServiceNode) TableName() string {
	return "topology_service_nodes"
}

type CallEdge struct {
	ID              string `gorm:"primaryKey;type:varchar(36)"`
	SourceID        string `gorm:"type:varchar(36);index"`
	TargetID        string `gorm:"type:varchar(36);index"`
	EdgeType        string `gorm:"type:varchar(50)"`
	IsDirect        bool
	Confidence      float64
	Protocol        string `gorm:"type:varchar(50)"`
	Method          string `gorm:"type:varchar(50)"`
	RequestRate     float64
	ErrorRate       float64
	LatencyP99      float64
	TargetInstances string `gorm:"type:text"`
	UpdatedAt       time.Time
	CreatedAt       time.Time
}

func (CallEdge) TableName() string {
	return "topology_call_edges"
}

type NetworkNode struct {
	ID          string `gorm:"primaryKey;type:varchar(36)"`
	Name        string `gorm:"type:varchar(255);index"`
	Type        string `gorm:"type:varchar(50)"`
	Layer       string `gorm:"type:varchar(50)"`
	IPAddress   string `gorm:"type:varchar(50);index"`
	CIDR        string `gorm:"type:varchar(50)"`
	Ports       string `gorm:"type:text"`
	Namespace   string `gorm:"type:varchar(255);index"`
	PodName     string `gorm:"type:varchar(255)"`
	NodeName    string `gorm:"type:varchar(255)"`
	Zone        string `gorm:"type:varchar(100)"`
	DataCenter  string `gorm:"type:varchar(100)"`
	Connections int64
	BytesIn     int64
	BytesOut    int64
	PacketsIn   int64
	PacketsOut  int64
	PacketLoss  float64
	Latency     float64
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

func (NetworkNode) TableName() string {
	return "topology_network_nodes"
}

type NetworkEdge struct {
	ID              string `gorm:"primaryKey;type:varchar(36)"`
	SourceID        string `gorm:"type:varchar(36);index"`
	TargetID        string `gorm:"type:varchar(36);index"`
	SourceIP        string `gorm:"type:varchar(50)"`
	TargetIP        string `gorm:"type:varchar(50)"`
	SourcePort      int
	TargetPort      int
	Protocol        string `gorm:"type:varchar(20)"`
	BytesSent       int64
	BytesReceived   int64
	PacketsSent     int64
	PacketsLost     int64
	ConnectionCount int
	Established     int
	TimeWait        int
	CloseWait       int
	UpdatedAt       time.Time
	CreatedAt       time.Time
}

func (NetworkEdge) TableName() string {
	return "topology_network_edges"
}

type TopologyChange struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)"`
	Timestamp   time.Time `gorm:"index"`
	ChangeType  string    `gorm:"type:varchar(50)"`
	EntityType  string    `gorm:"type:varchar(50)"`
	EntityID    string    `gorm:"type:varchar(36);index"`
	EntityName  string    `gorm:"type:varchar(255)"`
	Description string    `gorm:"type:text"`
	BeforeState string    `gorm:"type:text"`
	AfterState  string    `gorm:"type:text"`
	CreatedAt   time.Time
}

func (TopologyChange) TableName() string {
	return "topology_changes"
}

type TopologySnapshot struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)"`
	GraphType string    `gorm:"type:varchar(50);index"`
	Timestamp time.Time `gorm:"index"`
	Hash      string    `gorm:"type:varchar(64)"`
	Nodes     string    `gorm:"type:longtext"`
	Edges     string    `gorm:"type:longtext"`
	CreatedAt time.Time
}

func (TopologySnapshot) TableName() string {
	return "topology_snapshots"
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&ServiceNode{},
		&CallEdge{},
		&NetworkNode{},
		&NetworkEdge{},
		&TopologyChange{},
		&TopologySnapshot{},
	)
}
