package domain

import (
	"context"
	"time"
)

type SessionStatus string

const (
	SessionStatusSuccess   SessionStatus = "success"
	SessionStatusError     SessionStatus = "error"
	SessionStatusThrottled SessionStatus = "throttled"
	SessionStatusPending   SessionStatus = "pending"
)

type SessionType string

const (
	SessionTypeChat       SessionType = "chat"
	SessionTypeAgent      SessionType = "agent"
	SessionTypeBatch      SessionType = "batch"
	SessionTypeEvaluation SessionType = "evaluation"
)

type AISession struct {
	ID        string
	CreatedAt time.Time

	TraceID      string
	SpanID       string
	ParentSpanID string
	ServiceID    string
	ModelID      string

	GenAISystem        string
	GenAIOperationName string

	GenAIRequestModel            string
	GenAIRequestMaxTokens        int
	GenAIRequestTemperature      float64
	GenAIRequestTopP             float64
	GenAIRequestPresencePenalty  float64
	GenAIRequestFrequencyPenalty float64
	GenAIRequestStopSequences    []string

	GenAIResponseModel        string
	GenAIResponseFinishReasons []string
	GenAIResponseID           string

	GenAIUsageInputTokens  int
	GenAIUsageOutputTokens int
	GenAIUsageTotalTokens  int

	DurationMs int
	TTFTMs     int
	TPOTMs     int

	Status       SessionStatus
	ErrorType    string
	ErrorMessage string

	ResourceAttributes map[string]interface{}
	UserID             string
	SessionType        SessionType
	Metadata           map[string]interface{}
	CostUSD            float64
}

type AISessionRepository interface {
	Create(ctx context.Context, session *AISession) error
	GetByID(ctx context.Context, id string) (*AISession, error)
	GetByTraceID(ctx context.Context, traceID string) (*AISession, error)
	List(ctx context.Context, filter *SessionFilter) ([]*AISession, error)
	Update(ctx context.Context, session *AISession) error
}

type SessionFilter struct {
	ServiceID  string
	ModelID    string
	UserID     string
	Status     SessionStatus
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
	Offset     int
}

type AIModel struct {
	ID                 string
	Name               string
	Provider           string
	ModelID            string
	Config             map[string]interface{}
	CostPerInputToken  float64
	CostPerOutputToken float64
	Enabled            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AIModelRepository interface {
	Create(ctx context.Context, model *AIModel) error
	GetByID(ctx context.Context, id string) (*AIModel, error)
	GetByProviderModel(ctx context.Context, provider, modelID string) (*AIModel, error)
	List(ctx context.Context, enabled *bool) ([]*AIModel, error)
	Update(ctx context.Context, model *AIModel) error
}

type ToolCallStatus string

const (
	ToolCallStatusSuccess ToolCallStatus = "success"
	ToolCallStatusError   ToolCallStatus = "error"
	ToolCallStatusBlocked ToolCallStatus = "blocked"
)

type ToolCall struct {
	ID            string
	SessionID     string
	SpanID        string
	ParentSpanID  string

	GenAIToolName        string
	GenAIToolType        string
	GenAIToolDescription string
	GenAIToolCallID      string

	Arguments map[string]interface{}
	Result    map[string]interface{}

	Status       ToolCallStatus
	ErrorType    string
	ErrorMessage string

	DurationMs int

	IsApproved     bool
	ApprovalReason string

	CreatedAt time.Time
}

type ToolCallRepository interface {
	Create(ctx context.Context, call *ToolCall) error
	GetByID(ctx context.Context, id string) (*ToolCall, error)
	ListBySession(ctx context.Context, sessionID string) ([]*ToolCall, error)
}

type PromptTemplate struct {
	ID          string
	Name        string
	Description string
	Template    string
	Variables   map[string]interface{}
	Version     int
	Labels      map[string]string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PromptTemplateRepository interface {
	Create(ctx context.Context, template *PromptTemplate) error
	GetByID(ctx context.Context, id string) (*PromptTemplate, error)
	GetByName(ctx context.Context, name string) (*PromptTemplate, error)
	List(ctx context.Context, active *bool) ([]*PromptTemplate, error)
	Update(ctx context.Context, template *PromptTemplate) error
}
