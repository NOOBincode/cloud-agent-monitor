package domain

import (
	"time"

	"github.com/google/uuid"
)

type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type Alert struct {
	ID          uuid.UUID         `json:"id"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
	GeneratorURL string           `json:"generator_url,omitempty"`
	Status      AlertStatus       `json:"status"`
}

type AlertListResult struct {
	Data     []Alert `json:"data"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

type AlertFilter struct {
	Receiver  string   `form:"receiver"`
	Active    *bool    `form:"active"`
	Silenced  *bool    `form:"silenced"`
	Inhibited *bool    `form:"inhibited"`
	Filter    []string `form:"filter"`
	Page      int      `form:"page"`
	PageSize  int      `form:"page_size"`
}

func NewAlert(labels, annotations map[string]string) *Alert {
	now := time.Now()
	return &Alert{
		ID:          uuid.New(),
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    now,
		Status:      AlertStatusFiring,
	}
}

func (a *Alert) Resolve() {
	now := time.Now()
	a.EndsAt = &now
	a.Status = AlertStatusResolved
}

func (a *Alert) IsFiring() bool {
	return a.Status == AlertStatusFiring && (a.EndsAt == nil || a.EndsAt.After(time.Now()))
}

func (a *Alert) GetLabel(key string) string {
	if a.Labels == nil {
		return ""
	}
	return a.Labels[key]
}

func (a *Alert) GetSeverity() Severity {
	sev := a.GetLabel("severity")
	switch sev {
	case string(SeverityCritical):
		return SeverityCritical
	case string(SeverityWarning):
		return SeverityWarning
	default:
		return SeverityInfo
	}
}
