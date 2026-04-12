package models

import (
	"database/sql"
	"time"
)

type PromptAuditLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	SessionID sql.NullString `json:"session_id" gorm:"type:char(36)"`

	UserID    sql.NullString `json:"user_id" gorm:"type:varchar(255)"`
	ServiceID sql.NullString `json:"service_id" gorm:"type:char(36)"`

	PromptHash   sql.NullString `json:"prompt_hash" gorm:"type:varchar(255)"`
	PromptLength sql.NullInt32  `json:"prompt_length"`

	InjectionDetected  bool           `json:"injection_detected" gorm:"default:false"`
	InjectionType      sql.NullString `json:"injection_type" gorm:"type:varchar(50)"`
	InjectionConfidence sql.NullFloat64 `json:"injection_confidence" gorm:"type:decimal(5,2)"`

	PIIDetected bool           `json:"pii_detected" gorm:"default:false"`
	PIITypes    []byte         `json:"pii_types" gorm:"type:json"`

	PolicyViolation     bool           `json:"policy_violation" gorm:"default:false"`
	PolicyViolationType sql.NullString `json:"policy_violation_type" gorm:"type:varchar(50)"`

	ActionTaken sql.NullString `json:"action_taken" gorm:"type:varchar(20)"`
	ActionReason sql.NullString `json:"action_reason" gorm:"type:text"`

	AuditedAt time.Time `json:"audited_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (PromptAuditLog) TableName() string {
	return "prompt_audit_logs"
}

type ToolExecutionLog struct {
	ID            string    `json:"id" gorm:"primaryKey;type:char(36)"`
	ToolCallID    sql.NullString `json:"tool_call_id" gorm:"type:char(36)"`
	SessionID     sql.NullString `json:"session_id" gorm:"type:char(36)"`

	AgentID sql.NullString `json:"agent_id" gorm:"type:varchar(255)"`
	UserID  sql.NullString `json:"user_id" gorm:"type:varchar(255)"`

	ToolName string `json:"tool_name" gorm:"type:varchar(255);not null"`
	ToolType string `json:"tool_type" gorm:"type:varchar(50);not null"`

	IsWhitelisted         bool `json:"is_whitelisted" gorm:"default:true"`
	PermissionCheckPassed bool `json:"permission_check_passed" gorm:"default:true"`
	RateLimitExceeded     bool `json:"rate_limit_exceeded" gorm:"default:false"`

	DataSourcesAccessed  []byte `json:"data_sources_accessed" gorm:"type:json"`
	SensitiveDataAccessed bool   `json:"sensitive_data_accessed" gorm:"default:false"`
	DataClassification   sql.NullString `json:"data_classification" gorm:"type:varchar(20)"`

	ExecutionStatus string         `json:"execution_status" gorm:"type:varchar(20);not null"`
	ErrorType       sql.NullString `json:"error_type" gorm:"type:varchar(100)"`
	ErrorMessage    sql.NullString `json:"error_message" gorm:"type:text"`

	DurationMs sql.NullInt32 `json:"duration_ms"`

	AuditHash sql.NullString `json:"audit_hash" gorm:"type:varchar(255)"`

	ExecutedAt time.Time `json:"executed_at" gorm:"not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (ToolExecutionLog) TableName() string {
	return "tool_execution_logs"
}
