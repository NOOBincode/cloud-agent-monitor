package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertOperationType string

const (
	AlertOpSend       AlertOperationType = "send"
	AlertOpAcknowledge AlertOperationType = "acknowledge"
	AlertOpSilence    AlertOperationType = "silence"
	AlertOpUnsilence  AlertOperationType = "unsilence"
	AlertOpResolve    AlertOperationType = "resolve"
)

type AlertOperationStatus string

const (
	AlertOpStatusPending   AlertOperationStatus = "pending"
	AlertOpStatusSuccess   AlertOperationStatus = "success"
	AlertOpStatusFailed    AlertOperationStatus = "failed"
	AlertOpStatusRetrying  AlertOperationStatus = "retrying"
)

type AlertOperation struct {
	ID           uuid.UUID           `gorm:"type:char(36);primaryKey" json:"id"`
	AlertFingerprint string           `gorm:"type:varchar(64);index;not null" json:"alert_fingerprint"`
	OperationType AlertOperationType  `gorm:"type:varchar(20);index;not null" json:"operation_type"`
	Status        AlertOperationStatus `gorm:"type:varchar(20);index;not null;default:'pending'" json:"status"`
	
	UserID        uuid.UUID           `gorm:"type:char(36);index;not null" json:"user_id"`
	TenantID      *uuid.UUID          `gorm:"type:char(36);index" json:"tenant_id,omitempty"`
	
	AlertLabels   Labels              `gorm:"type:json;serializer:json" json:"alert_labels"`
	RequestData   string              `gorm:"type:text" json:"request_data"`
	ResponseData  string              `gorm:"type:text" json:"response_data"`
	ErrorMessage  string              `gorm:"type:text" json:"error_message,omitempty"`
	
	RetryCount    int                 `gorm:"default:0" json:"retry_count"`
	MaxRetries    int                 `gorm:"default:3" json:"max_retries"`
	
	IPAddress     string              `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent     string              `gorm:"type:varchar(500)" json:"user_agent"`
	
	ProcessedAt   *time.Time          `json:"processed_at,omitempty"`
	CreatedAt     time.Time           `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt     time.Time           `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AlertOperation) TableName() string {
	return "alert_operations"
}

func (a *AlertOperation) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type AlertNoiseRecord struct {
	ID              uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	AlertFingerprint string   `gorm:"type:varchar(64);uniqueIndex;not null" json:"alert_fingerprint"`
	AlertName       string    `gorm:"type:varchar(255);index;not null" json:"alert_name"`
	AlertLabels     Labels    `gorm:"type:json;serializer:json" json:"alert_labels"`
	
	FireCount       int       `gorm:"default:1" json:"fire_count"`
	ResolveCount    int       `gorm:"default:0" json:"resolve_count"`
	NoiseScore      float64   `gorm:"type:decimal(5,2);default:0" json:"noise_score"`
	
	IsNoisy         bool      `gorm:"default:false;index" json:"is_noisy"`
	IsHighRisk      bool      `gorm:"default:false;index" json:"is_high_risk"`
	
	SilenceSuggested bool     `gorm:"default:false" json:"silence_suggested"`
	SilenceUntil    *time.Time `json:"silence_until,omitempty"`
	
	LastFiredAt     time.Time `gorm:"autoCreateTime" json:"last_fired_at"`
	LastResolvedAt  *time.Time `json:"last_resolved_at,omitempty"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AlertNoiseRecord) TableName() string {
	return "alert_noise_records"
}

func (a *AlertNoiseRecord) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type AlertNotification struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	AlertFingerprint string `gorm:"type:varchar(64);index;not null" json:"alert_fingerprint"`
	
	NotificationType string    `gorm:"type:varchar(50);index;not null" json:"notification_type"`
	Recipient        string    `gorm:"type:varchar(255);not null" json:"recipient"`
	
	Status       string     `gorm:"type:varchar(20);index;not null" json:"status"`
	ErrorMessage string     `gorm:"type:text" json:"error_message,omitempty"`
	
	SentAt       *time.Time `json:"sent_at,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (AlertNotification) TableName() string {
	return "alert_notifications"
}

func (a *AlertNotification) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
