package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SLOStatus string

const (
	SLOStatusHealthy  SLOStatus = "healthy"
	SLOStatusWarning  SLOStatus = "warning"
	SLOStatusCritical SLOStatus = "critical"
	SLOStatusUnknown  SLOStatus = "unknown"
)

type SLIType string

const (
	SLITypeAvailability SLIType = "availability"
	SLITypeLatency      SLIType = "latency"
	SLITypeThroughput   SLIType = "throughput"
	SLITypeErrorRate    SLIType = "error_rate"
	SLITypeCustom       SLIType = "custom"
)

type SLO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ServiceID   uuid.UUID `json:"service_id"`
	ServiceName string    `json:"service_name"`

	Target       float64 `json:"target"`
	Window       string  `json:"window"`
	WarningBurn  float64 `json:"warning_burn"`
	CriticalBurn float64 `json:"critical_burn"`

	SLI         SLI         `json:"sli"`
	ErrorBudget ErrorBudget `json:"error_budget"`

	Status   SLOStatus `json:"status"`
	Current  float64   `json:"current"`
	BurnRate float64   `json:"burn_rate"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SLI struct {
	ID          uuid.UUID `json:"id"`
	SLOID       uuid.UUID `json:"slo_id"`
	Name        string    `json:"name"`
	Type        SLIType   `json:"type"`
	Query       string    `json:"query"`
	Threshold   float64   `json:"threshold"`
	Unit        string    `json:"unit"`
	Description string    `json:"description"`
}

type ErrorBudget struct {
	Total      float64   `json:"total"`
	Remaining  float64   `json:"remaining"`
	Consumed   float64   `json:"consumed"`
	Percentage float64   `json:"percentage"`
	Window     string    `json:"window"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type BurnRateAlert struct {
	SLOID       uuid.UUID `json:"slo_id"`
	SLOName     string    `json:"slo_name"`
	ServiceName string    `json:"service_name"`
	CurrentRate float64   `json:"current_rate"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Window      string    `json:"window"`
	FiredAt     time.Time `json:"fired_at"`
}

type SLOFilter struct {
	ServiceID uuid.UUID `form:"service_id"`
	Name      string    `form:"name"`
	Status    SLOStatus `form:"status"`
	Page      int       `form:"page"`
	PageSize  int       `form:"page_size"`
}

type SLOSummary struct {
	Total       int               `json:"total"`
	ByStatus    map[SLOStatus]int `json:"by_status"`
	ByService   map[string]int    `json:"by_service"`
	AvgBurnRate float64           `json:"avg_burn_rate"`
}

func NewSLO(name string, serviceID uuid.UUID, target float64, window string) *SLO {
	now := time.Now()
	return &SLO{
		ID:           uuid.New(),
		Name:         name,
		ServiceID:    serviceID,
		Target:       target,
		Window:       window,
		WarningBurn:  2.0,
		CriticalBurn: 10.0,
		Status:       SLOStatusUnknown,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (s *SLO) CalculateStatus() SLOStatus {
	if s.BurnRate <= 0 {
		return SLOStatusHealthy
	}
	if s.BurnRate >= s.CriticalBurn {
		return SLOStatusCritical
	}
	if s.BurnRate >= s.WarningBurn {
		return SLOStatusWarning
	}
	return SLOStatusHealthy
}

func (s *SLO) CalculateErrorBudget() ErrorBudget {
	total := 100.0 - s.Target
	if total <= 0 {
		total = 100 - 99.9
	}

	remaining := total * (1 - s.BurnRate/100)
	if remaining < 0 {
		remaining = 0
	}
	if remaining > total {
		remaining = total
	}

	consumed := total - remaining
	percentage := (remaining / total) * 100

	return ErrorBudget{
		Total:      total,
		Remaining:  remaining,
		Consumed:   consumed,
		Percentage: percentage,
		Window:     s.Window,
		UpdatedAt:  time.Now(),
	}
}

func NewSLI(sloID uuid.UUID, name string, sliType SLIType, query string) *SLI {
	return &SLI{
		ID:    uuid.New(),
		SLOID: sloID,
		Name:  name,
		Type:  sliType,
		Query: query,
	}
}

type SLORepositoryInterface interface {
	Create(ctx context.Context, slo *SLO) error
	Update(ctx context.Context, slo *SLO) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*SLO, error)
	GetByServiceID(ctx context.Context, serviceID uuid.UUID) ([]*SLO, error)
	List(ctx context.Context, filter SLOFilter) ([]*SLO, int64, error)
	GetSummary(ctx context.Context) (*SLOSummary, error)
}

type SLIRepositoryInterface interface {
	Create(ctx context.Context, sli *SLI) error
	Update(ctx context.Context, sli *SLI) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetBySLOID(ctx context.Context, sloID uuid.UUID) ([]*SLI, error)
}

type SLICollectorInterface interface {
	CollectAvailability(ctx context.Context, query string, window string) (float64, error)
	CollectLatency(ctx context.Context, query string, threshold float64, window string) (float64, error)
	CollectThroughput(ctx context.Context, query string, threshold float64, window string) (float64, error)
	CollectErrorRate(ctx context.Context, query string, window string) (float64, error)
	CollectCustom(ctx context.Context, query string, window string) (float64, error)
	CollectBurnRate(ctx context.Context, slo *SLO) (float64, error)
	CollectSLI(ctx context.Context, sli *SLI, window string) (float64, error)
}

type SLOServiceInterface interface {
	CreateSLO(ctx context.Context, slo *SLO, sli *SLI) (*SLO, error)
	UpdateSLO(ctx context.Context, slo *SLO) (*SLO, error)
	DeleteSLO(ctx context.Context, id uuid.UUID) error
	GetSLO(ctx context.Context, id uuid.UUID) (*SLO, error)
	GetSLOByService(ctx context.Context, serviceID uuid.UUID) ([]*SLO, error)
	ListSLOs(ctx context.Context, filter SLOFilter) ([]*SLO, int64, error)
	GetSLOSummary(ctx context.Context) (*SLOSummary, error)
	RefreshSLOStatus(ctx context.Context, id uuid.UUID) (*SLO, error)
	RefreshAllSLOStatus(ctx context.Context) error
	GetErrorBudget(ctx context.Context, id uuid.UUID) (*ErrorBudget, error)
	GetBurnRateAlerts(ctx context.Context) ([]*BurnRateAlert, error)
}
