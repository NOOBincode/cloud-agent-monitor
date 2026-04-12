package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SLOStatus string

const (
	SLOStatusHealthy   SLOStatus = "healthy"
	SLOStatusWarning   SLOStatus = "warning"
	SLOStatusCritical  SLOStatus = "critical"
	SLOStatusUnknown   SLOStatus = "unknown"
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
	ID           uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(255);not null" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	ServiceID    uuid.UUID      `gorm:"type:char(36);not null;index" json:"service_id"`
	Target       float64        `gorm:"type:decimal(5,2);not null;default:99.90" json:"target"`
	Window       string         `gorm:"type:varchar(50);not null;default:'30d'" json:"window"`
	WarningBurn  float64        `gorm:"type:decimal(10,2);not null;default:2.00" json:"warning_burn"`
	CriticalBurn float64        `gorm:"type:decimal(10,2);not null;default:10.00" json:"critical_burn"`
	Status       SLOStatus      `gorm:"type:varchar(20);not null;default:'unknown';index" json:"status"`
	CurrentValue float64        `gorm:"type:decimal(10,4);default:0" json:"current_value"`
	BurnRate     float64        `gorm:"type:decimal(10,4);default:0" json:"burn_rate"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Service *Service `gorm:"foreignKey:ServiceID" json:"service,omitempty"`
	SLIs    []SLI    `gorm:"foreignKey:SLOID" json:"slis,omitempty"`
}

func (SLO) TableName() string {
	return "slos"
}

func (s *SLO) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.Status == "" {
		s.Status = SLOStatusUnknown
	}
	return nil
}

type SLI struct {
	ID          uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	SLOID       uuid.UUID      `gorm:"type:char(36);not null;index" json:"slo_id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Type        SLIType        `gorm:"type:varchar(50);not null;index" json:"type"`
	Query       string         `gorm:"type:text;not null" json:"query"`
	Threshold   float64        `gorm:"type:decimal(20,4);default:0" json:"threshold"`
	Unit        string         `gorm:"type:varchar(50);default:''" json:"unit"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	SLO *SLO `gorm:"foreignKey:SLOID" json:"slo,omitempty"`
}

func (SLI) TableName() string {
	return "slis"
}

func (s *SLI) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

type ErrorBudgetHistory struct {
	ID         uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	SLOID      uuid.UUID `gorm:"type:char(36);not null;index" json:"slo_id"`
	Total      float64   `gorm:"type:decimal(10,4);not null" json:"total"`
	Remaining  float64   `gorm:"type:decimal(10,4);not null" json:"remaining"`
	Consumed   float64   `gorm:"type:decimal(10,4);not null" json:"consumed"`
	Percentage float64   `gorm:"type:decimal(10,4);not null" json:"percentage"`
	BurnRate   float64   `gorm:"type:decimal(10,4);not null" json:"burn_rate"`
	RecordedAt time.Time `gorm:"autoCreateTime;index" json:"recorded_at"`

	SLO *SLO `gorm:"foreignKey:SLOID" json:"slo,omitempty"`
}

func (ErrorBudgetHistory) TableName() string {
	return "error_budget_history"
}

func (e *ErrorBudgetHistory) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

type BurnRateAlert struct {
	ID          uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	SLOID       uuid.UUID      `gorm:"type:char(36);not null;index" json:"slo_id"`
	SLOName     string         `gorm:"type:varchar(255);not null" json:"slo_name"`
	ServiceName string         `gorm:"type:varchar(255);not null" json:"service_name"`
	CurrentRate float64        `gorm:"type:decimal(10,4);not null" json:"current_rate"`
	Threshold   float64        `gorm:"type:decimal(10,4);not null" json:"threshold"`
	Severity    string         `gorm:"type:varchar(20);not null;index" json:"severity"`
	Window      string         `gorm:"type:varchar(50);not null" json:"window"`
	FiredAt     time.Time      `gorm:"autoCreateTime;index" json:"fired_at"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`

	SLO *SLO `gorm:"foreignKey:SLOID" json:"slo,omitempty"`
}

func (BurnRateAlert) TableName() string {
	return "burn_rate_alerts"
}

func (b *BurnRateAlert) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
