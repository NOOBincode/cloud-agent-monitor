package models

import (
	"database/sql"
	"time"
)

type GPUNode struct {
	ID              string    `json:"id" gorm:"primaryKey;type:char(36)"`
	NodeName        string    `json:"node_name" gorm:"type:varchar(255);not null"`
	GPUIndex        int       `json:"gpu_index" gorm:"not null"`
	GPUUUID         string    `json:"gpu_uuid" gorm:"type:varchar(255);unique;not null"`
	GPUModel        string    `json:"gpu_model" gorm:"type:varchar(255);not null"`
	GPUMemoryTotalMB int      `json:"gpu_memory_total_mb" gorm:"not null"`
	MIGEnabled      bool      `json:"mig_enabled" gorm:"default:false"`
	MIGProfile      sql.NullString `json:"mig_profile" gorm:"type:varchar(50)"`

	K8sNodeName  sql.NullString `json:"k8s_node_name" gorm:"type:varchar(255)"`
	K8sPodName   sql.NullString `json:"k8s_pod_name" gorm:"type:varchar(255)"`
	Namespace    sql.NullString `json:"namespace" gorm:"type:varchar(255)"`

	Status       string    `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	HealthStatus sql.NullString `json:"health_status" gorm:"type:varchar(20)"`
	LastHealthCheck sql.NullTime `json:"last_health_check"`

	Labels      []byte    `json:"labels" gorm:"type:json"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (GPUNode) TableName() string {
	return "gpu_nodes"
}

type GPUMetric struct {
	ID         string    `json:"id" gorm:"primaryKey;type:char(36)"`
	GPUNodeID  string    `json:"gpu_node_id" gorm:"type:char(36);not null"`

	GPUUtilization     sql.NullFloat64 `json:"gpu_utilization" gorm:"type:decimal(5,2)"`
	MemoryUtilization  sql.NullFloat64 `json:"memory_utilization" gorm:"type:decimal(5,2)"`
	MemoryUsedMB       sql.NullInt32   `json:"memory_used_mb"`
	MemoryFreeMB       sql.NullInt32   `json:"memory_free_mb"`

	PowerUsageW   sql.NullFloat64 `json:"power_usage_w" gorm:"type:decimal(6,2)"`
	PowerDrawW    sql.NullFloat64 `json:"power_draw_w" gorm:"type:decimal(6,2)"`
	TemperatureC  sql.NullInt32   `json:"temperature_c"`

	SMClockMHz    sql.NullInt32 `json:"sm_clock_mhz"`
	MemoryClockMHz sql.NullInt32 `json:"memory_clock_mhz"`

	PCIeRxThroughputMB sql.NullFloat64 `json:"pcie_rx_throughput_mb" gorm:"type:decimal(10,2)"`
	PCIeTxThroughputMB sql.NullFloat64 `json:"pcie_tx_throughput_mb" gorm:"type:decimal(10,2)"`

	NVLinkRxBytes sql.NullInt64 `json:"nvlink_rx_bytes"`
	NVLinkTxBytes sql.NullInt64 `json:"nvlink_tx_bytes"`

	XIDErrors  []byte `json:"xid_errors" gorm:"type:json"`
	ECCErrors  []byte `json:"ecc_errors" gorm:"type:json"`

	CollectedAt time.Time `json:"collected_at" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (GPUMetric) TableName() string {
	return "gpu_metrics"
}

type GPUAlert struct {
	ID         string    `json:"id" gorm:"primaryKey;type:char(36)"`
	GPUNodeID  string    `json:"gpu_node_id" gorm:"type:char(36);not null"`

	AlertType  string    `json:"alert_type" gorm:"type:varchar(50);not null"`
	Severity   string    `json:"severity" gorm:"type:varchar(20);not null"`
	AlertName  string    `json:"alert_name" gorm:"type:varchar(255);not null"`
	Message    string    `json:"message" gorm:"type:text;not null"`

	XIDCode       sql.NullInt32   `json:"xid_code"`
	MetricValue   sql.NullFloat64 `json:"metric_value" gorm:"type:decimal(20,6)"`
	Threshold     sql.NullFloat64 `json:"threshold" gorm:"type:decimal(20,6)"`

	Status       string         `json:"status" gorm:"type:varchar(20);not null;default:'firing'"`
	ResolvedAt   sql.NullTime   `json:"resolved_at"`

	FiredAt      time.Time `json:"fired_at" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (GPUAlert) TableName() string {
	return "gpu_alerts"
}
