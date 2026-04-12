package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
)

type AlertSeverity string

const (
	AlertSeverityCritical AlertSeverity = "critical"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityInfo     AlertSeverity = "info"
)

type AlertRecord struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Fingerprint string    `gorm:"type:varchar(64);index:idx_alert_record_fp;not null" json:"fingerprint"`
	
	Labels      Labels `gorm:"type:json;serializer:json" json:"labels"`
	Annotations Labels `gorm:"type:json;serializer:json" json:"annotations"`
	
	Status   AlertStatus   `gorm:"type:varchar(20);index:idx_alert_record_status;not null" json:"status"`
	Severity AlertSeverity `gorm:"type:varchar(20);index:idx_alert_record_severity;not null;default:'info'" json:"severity"`
	
	StartsAt time.Time  `gorm:"type:datetime;index:idx_alert_record_starts;not null" json:"starts_at"`
	EndsAt   *time.Time `gorm:"type:datetime" json:"ends_at,omitempty"`
	Duration int64      `gorm:"type:bigint;default:0" json:"duration"`
	
	Source    string    `gorm:"type:varchar(50);default:'alertmanager'" json:"source"`
	CreatedAt time.Time `gorm:"type:datetime;autoCreateTime;index:idx_alert_record_created" json:"created_at"`
}

func (AlertRecord) TableName() string {
	return "alert_records"
}

func (a *AlertRecord) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Severity == "" {
		a.Severity = AlertSeverityInfo
	}
	if a.Status == "" {
		a.Status = AlertStatusFiring
	}
	return nil
}

func (a *AlertRecord) CalculateDuration() {
	if a.EndsAt != nil && !a.StartsAt.IsZero() {
		a.Duration = int64(a.EndsAt.Sub(a.StartsAt).Seconds())
		if a.Duration < 0 {
			a.Duration = 0
		}
	}
}

func (a *AlertRecord) ExtractSeverity() {
	if a.Labels == nil {
		a.Severity = AlertSeverityInfo
		return
	}
	
	sev, ok := a.Labels["severity"]
	if !ok {
		a.Severity = AlertSeverityInfo
		return
	}
	
	switch sev {
	case string(AlertSeverityCritical):
		a.Severity = AlertSeverityCritical
	case string(AlertSeverityWarning):
		a.Severity = AlertSeverityWarning
	default:
		a.Severity = AlertSeverityInfo
	}
}

type AlertRecordFilter struct {
	Fingerprint string        `form:"fingerprint"`
	Status      AlertStatus   `form:"status"`
	Severity    AlertSeverity `form:"severity"`
	AlertName   string        `form:"alertname"`
	StartFrom   *time.Time    `form:"start_from"`
	StartTo     *time.Time    `form:"start_to"`
	Page        int           `form:"page"`
	PageSize    int           `form:"page_size"`
}

type AlertRecordStats struct {
	TotalCount    int64 `json:"total_count"`
	FiringCount   int64 `json:"firing_count"`
	ResolvedCount int64 `json:"resolved_count"`
	CriticalCount int64 `json:"critical_count"`
	WarningCount  int64 `json:"warning_count"`
	InfoCount     int64 `json:"info_count"`
	AvgDuration   int64 `json:"avg_duration_seconds"`
}
