package models

import (
	"database/sql"
	"time"
)

type ToolCall struct {
	ID            string    `json:"id" gorm:"primaryKey;type:char(36)"`
	SessionID     string    `json:"session_id" gorm:"type:char(36);not null"`
	SpanID        string    `json:"span_id" gorm:"type:varchar(64);not null"`
	ParentSpanID  sql.NullString `json:"parent_span_id" gorm:"type:varchar(64)"`

	GenAIToolName        string         `json:"gen_ai_tool_name" gorm:"type:varchar(255);not null"`
	GenAIToolType        string         `json:"gen_ai_tool_type" gorm:"type:varchar(50);not null"`
	GenAIToolDescription sql.NullString `json:"gen_ai_tool_description" gorm:"type:text"`
	GenAIToolCallID      sql.NullString `json:"gen_ai_tool_call_id" gorm:"type:varchar(255)"`

	ToolName sql.NullString `json:"tool_name" gorm:"type:varchar(255)"`
	ToolType sql.NullString `json:"tool_type" gorm:"type:varchar(50)"`

	Arguments []byte `json:"arguments" gorm:"type:json"`
	Result    []byte `json:"result" gorm:"type:json"`

	Status       string         `json:"status" gorm:"type:varchar(20);not null"`
	ErrorType    sql.NullString `json:"error_type" gorm:"type:varchar(100)"`
	ErrorMessage sql.NullString `json:"error_message" gorm:"type:text"`

	DurationMs sql.NullInt32 `json:"duration_ms"`

	IsApproved     bool           `json:"is_approved" gorm:"default:true"`
	ApprovalReason sql.NullString `json:"approval_reason" gorm:"type:varchar(255)"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (ToolCall) TableName() string {
	return "tool_calls"
}
