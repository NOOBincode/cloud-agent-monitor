package models

import (
	"database/sql"
	"time"
)

type CostBudget struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	ScopeType string    `json:"scope_type" gorm:"type:varchar(20);not null"`
	ScopeID   string    `json:"scope_id" gorm:"type:varchar(255);not null"`
	ScopeName string    `json:"scope_name" gorm:"type:varchar(255);not null"`

	Period string `json:"period" gorm:"type:varchar(20);not null"`

	TokenLimit    sql.NullInt64 `json:"token_limit"`
	CostLimitUSD  sql.NullFloat64 `json:"cost_limit_usd" gorm:"type:decimal(20,6)"`
	RequestLimit  sql.NullInt64 `json:"request_limit"`

	CurrentTokens   int64 `json:"current_tokens" gorm:"default:0"`
	CurrentCostUSD  float64 `json:"current_cost_usd" gorm:"type:decimal(20,6);default:0"`
	CurrentRequests int64 `json:"current_requests" gorm:"default:0"`

	AlertThreshold50  bool `json:"alert_threshold_50" gorm:"default:false"`
	AlertThreshold80  bool `json:"alert_threshold_80" gorm:"default:true"`
	AlertThreshold100 bool `json:"alert_threshold_100" gorm:"default:true"`

	ActionOnExceed sql.NullString `json:"action_on_exceed" gorm:"type:varchar(20)"`
	DowngradeModel sql.NullString `json:"downgrade_model" gorm:"type:varchar(255)"`

	Status string `json:"status" gorm:"type:varchar(20);not null;default:'active'"`

	PeriodStart time.Time `json:"period_start" gorm:"not null"`
	PeriodEnd   time.Time `json:"period_end" gorm:"not null"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CostBudget) TableName() string {
	return "cost_budgets"
}

type CostRecord struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	SessionID sql.NullString `json:"session_id" gorm:"type:char(36)"`

	ServiceID sql.NullString `json:"service_id" gorm:"type:char(36)"`
	UserID    sql.NullString `json:"user_id" gorm:"type:varchar(255)"`
	TeamID    sql.NullString `json:"team_id" gorm:"type:varchar(255)"`
	ModelID   sql.NullString `json:"model_id" gorm:"type:char(36)"`

	CostType string    `json:"cost_type" gorm:"type:varchar(20);not null"`
	CostUSD  float64   `json:"cost_usd" gorm:"type:decimal(20,6);not null"`

	InputTokens  sql.NullInt32 `json:"input_tokens"`
	OutputTokens sql.NullInt32 `json:"output_tokens"`
	TotalTokens  sql.NullInt32 `json:"total_tokens"`

	GPUHours      sql.NullFloat64 `json:"gpu_hours" gorm:"type:decimal(10,6)"`
	CPUHours      sql.NullFloat64 `json:"cpu_hours" gorm:"type:decimal(10,6)"`
	MemoryGBHours sql.NullFloat64 `json:"memory_gb_hours" gorm:"type:decimal(10,6)"`

	ResourceID sql.NullString `json:"resource_id" gorm:"type:varchar(255)"`
	Region     sql.NullString `json:"region" gorm:"type:varchar(50)"`

	IncurredAt time.Time `json:"incurred_at" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (CostRecord) TableName() string {
	return "cost_records"
}

type BudgetAlert struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	BudgetID  string    `json:"budget_id" gorm:"type:char(36);not null"`

	ThresholdPct int    `json:"threshold_pct" gorm:"not null"`
	AlertType    string `json:"alert_type" gorm:"type:varchar(20);not null"`

	CurrentTokens    sql.NullInt64 `json:"current_tokens"`
	CurrentCostUSD   sql.NullFloat64 `json:"current_cost_usd" gorm:"type:decimal(20,6)"`
	CurrentRequests  sql.NullInt64 `json:"current_requests"`
	UsagePercentage  sql.NullFloat64 `json:"usage_percentage" gorm:"type:decimal(5,2)"`

	Notified           bool         `json:"notified" gorm:"default:false"`
	NotifiedAt         sql.NullTime `json:"notified_at"`
	NotificationChannels []byte      `json:"notification_channels" gorm:"type:json"`

	Status         string         `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	AcknowledgedAt sql.NullTime   `json:"acknowledged_at"`
	AcknowledgedBy sql.NullString `json:"acknowledged_by" gorm:"type:varchar(255)"`

	TriggeredAt time.Time `json:"triggered_at" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (BudgetAlert) TableName() string {
	return "budget_alerts"
}
