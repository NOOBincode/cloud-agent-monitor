package domain

import (
	"context"
	"time"

	alertDomain "cloud-agent-monitor/internal/alerting/domain"
)

type GPUNodeStatus string

const (
	GPUNodeStatusActive     GPUNodeStatus = "active"
	GPUNodeStatusMaintenance GPUNodeStatus = "maintenance"
	GPUNodeStatusFailed     GPUNodeStatus = "failed"
)

type GPUHealthStatus string

const (
	GPUHealthHealthy   GPUHealthStatus = "healthy"
	GPUHealthDegraded  GPUHealthStatus = "degraded"
	GPUHealthCritical  GPUHealthStatus = "critical"
)

type GPUNode struct {
	ID              string
	NodeName        string
	GPUIndex        int
	GPUUUID         string
	GPUModel        string
	GPUMemoryTotalMB int
	MIGEnabled      bool
	MIGProfile      string

	K8sNodeName string
	K8sPodName  string
	Namespace   string

	Status       GPUNodeStatus
	HealthStatus GPUHealthStatus
	LastHealthCheck time.Time

	Labels      map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GPUMetric struct {
	ID         string
	GPUNodeID  string

	GPUUtilization     float64
	MemoryUtilization  float64
	MemoryUsedMB       int
	MemoryFreeMB       int

	PowerUsageW  float64
	PowerDrawW   float64
	TemperatureC int

	SMClockMHz     int
	MemoryClockMHz int

	PCIeRxThroughputMB float64
	PCIeTxThroughputMB float64

	NVLinkRxBytes int64
	NVLinkTxBytes int64

	XIDErrors []int
	ECCErrors map[string]int

	CollectedAt time.Time
	CreatedAt   time.Time
}

type GPUAlert struct {
	ID        string
	GPUNodeID string

	AlertType  AlertType
	Severity   alertDomain.Severity
	AlertName  string
	Message    string

	XIDCode     int
	MetricValue float64
	Threshold   float64

	Status     string
	ResolvedAt time.Time

	FiredAt   time.Time
	CreatedAt time.Time
}

type GPUNodeRepository interface {
	Create(ctx context.Context, node *GPUNode) error
	GetByID(ctx context.Context, id string) (*GPUNode, error)
	GetByUUID(ctx context.Context, uuid string) (*GPUNode, error)
	List(ctx context.Context, status *GPUNodeStatus) ([]*GPUNode, error)
	Update(ctx context.Context, node *GPUNode) error
}

type GPUMetricRepository interface {
	Create(ctx context.Context, metric *GPUMetric) error
	ListByNode(ctx context.Context, nodeID string, start, end time.Time, limit int) ([]*GPUMetric, error)
	GetLatest(ctx context.Context, nodeID string) (*GPUMetric, error)
}

type GPUAlertRepository interface {
	Create(ctx context.Context, alert *GPUAlert) error
	GetByID(ctx context.Context, id string) (*GPUAlert, error)
	ListActive(ctx context.Context, nodeID *string) ([]*GPUAlert, error)
	Resolve(ctx context.Context, id string, resolvedAt time.Time) error
}
