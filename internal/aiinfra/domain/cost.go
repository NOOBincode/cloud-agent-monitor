package domain

import (
	"context"
	"time"
)

type BudgetStatus string

const (
	BudgetStatusActive   BudgetStatus = "active"
	BudgetStatusExceeded BudgetStatus = "exceeded"
	BudgetStatusPaused   BudgetStatus = "paused"
)

type CostBudget struct {
	ID        string
	ScopeType BudgetScopeType
	ScopeID   string
	ScopeName string

	Period BudgetPeriod

	TokenLimit   int64
	CostLimitUSD float64
	RequestLimit int64

	CurrentTokens   int64
	CurrentCostUSD  float64
	CurrentRequests int64

	AlertThreshold50  bool
	AlertThreshold80  bool
	AlertThreshold100 bool

	ActionOnExceed BudgetAction
	DowngradeModel string

	Status BudgetStatus

	PeriodStart time.Time
	PeriodEnd   time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

type CostRecord struct {
	ID        string
	SessionID string

	ServiceID string
	UserID    string
	TeamID    string
	ModelID   string

	CostType CostType
	CostUSD  float64

	InputTokens  int
	OutputTokens int
	TotalTokens  int

	GPUHours      float64
	CPUHours      float64
	MemoryGBHours float64

	ResourceID string
	Region     string

	IncurredAt time.Time
	CreatedAt  time.Time
}

type BudgetAlertStatus string

const (
	BudgetAlertStatusActive       BudgetAlertStatus = "active"
	BudgetAlertStatusAcknowledged BudgetAlertStatus = "acknowledged"
	BudgetAlertStatusResolved     BudgetAlertStatus = "resolved"
)

type BudgetAlertType string

const (
	BudgetAlertTypeWarning  BudgetAlertType = "warning"
	BudgetAlertTypeCritical BudgetAlertType = "critical"
	BudgetAlertTypeExceeded BudgetAlertType = "exceeded"
)

type BudgetAlert struct {
	ID        string
	BudgetID  string

	ThresholdPct int
	AlertType    BudgetAlertType

	CurrentTokens   int64
	CurrentCostUSD  float64
	CurrentRequests int64
	UsagePercentage float64

	Notified           bool
	NotifiedAt         time.Time
	NotificationChannels []string

	Status         BudgetAlertStatus
	AcknowledgedAt time.Time
	AcknowledgedBy string

	TriggeredAt time.Time
	CreatedAt   time.Time
}

type CostBudgetRepository interface {
	Create(ctx context.Context, budget *CostBudget) error
	GetByID(ctx context.Context, id string) (*CostBudget, error)
	GetByScope(ctx context.Context, scopeType BudgetScopeType, scopeID string, period BudgetPeriod) (*CostBudget, error)
	List(ctx context.Context, status *BudgetStatus) ([]*CostBudget, error)
	Update(ctx context.Context, budget *CostBudget) error
}

type CostRecordRepository interface {
	Create(ctx context.Context, record *CostRecord) error
	GetByID(ctx context.Context, id string) (*CostRecord, error)
	List(ctx context.Context, filter *CostRecordFilter) ([]*CostRecord, error)
	AggregateByScope(ctx context.Context, scopeType BudgetScopeType, scopeID string, start, end time.Time) (*CostAggregation, error)
}

type CostRecordFilter struct {
	ServiceID  string
	UserID     string
	TeamID     string
	ModelID    string
	CostType   CostType
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
	Offset     int
}

type CostAggregation struct {
	TotalCostUSD     float64
	TotalTokens      int64
	TotalRequests    int64
	TotalGPUHours    float64
	TotalCPUHours    float64
	TotalMemoryGBHours float64
}

type BudgetAlertRepository interface {
	Create(ctx context.Context, alert *BudgetAlert) error
	GetByID(ctx context.Context, id string) (*BudgetAlert, error)
	ListActive(ctx context.Context, budgetID *string) ([]*BudgetAlert, error)
	Acknowledge(ctx context.Context, id string, acknowledgedBy string) error
}
