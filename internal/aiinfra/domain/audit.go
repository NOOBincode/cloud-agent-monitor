package domain

import (
	"context"
	"time"
)

type PromptAuditLog struct {
	ID        string
	SessionID string

	UserID    string
	ServiceID string

	PromptHash   string
	PromptLength int

	InjectionDetected   bool
	InjectionType       string
	InjectionConfidence float64

	PIIDetected bool
	PIITypes    []string

	PolicyViolation     bool
	PolicyViolationType string

	ActionTaken  ActionType
	ActionReason string

	AuditedAt time.Time
	CreatedAt time.Time
}

type ToolExecutionStatus string

const (
	ToolExecutionSuccess ToolExecutionStatus = "success"
	ToolExecutionError   ToolExecutionStatus = "error"
	ToolExecutionBlocked ToolExecutionStatus = "blocked"
	ToolExecutionTimeout ToolExecutionStatus = "timeout"
)

type ToolExecutionLog struct {
	ID         string
	ToolCallID string
	SessionID  string

	AgentID string
	UserID  string

	ToolName string
	ToolType string

	IsWhitelisted         bool
	PermissionCheckPassed bool
	RateLimitExceeded     bool

	DataSourcesAccessed  []string
	SensitiveDataAccessed bool
	DataClassification   DataClassification

	ExecutionStatus ToolExecutionStatus
	ErrorType       string
	ErrorMessage    string

	DurationMs int

	AuditHash string

	ExecutedAt time.Time
	CreatedAt  time.Time
}

type PromptAuditRepository interface {
	Create(ctx context.Context, log *PromptAuditLog) error
	GetByID(ctx context.Context, id string) (*PromptAuditLog, error)
	List(ctx context.Context, filter *PromptAuditFilter) ([]*PromptAuditLog, error)
}

type PromptAuditFilter struct {
	SessionID        string
	UserID           string
	ServiceID        string
	InjectionDetected *bool
	PIIDetected       *bool
	PolicyViolation   *bool
	StartTime        time.Time
	EndTime          time.Time
	Limit            int
	Offset           int
}

type ToolExecutionRepository interface {
	Create(ctx context.Context, log *ToolExecutionLog) error
	GetByID(ctx context.Context, id string) (*ToolExecutionLog, error)
	List(ctx context.Context, filter *ToolExecutionFilter) ([]*ToolExecutionLog, error)
}

type ToolExecutionFilter struct {
	SessionID      string
	ToolCallID     string
	AgentID        string
	UserID         string
	ToolName       string
	ExecutionStatus ToolExecutionStatus
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Offset         int
}
