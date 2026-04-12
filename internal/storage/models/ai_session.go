package models

import (
	"database/sql"
	"time"
)

type AISession struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`

	TraceID       sql.NullString `json:"trace_id" gorm:"type:varchar(64);not null"`
	SpanID        sql.NullString `json:"span_id" gorm:"type:varchar(64)"`
	ParentSpanID  sql.NullString `json:"parent_span_id" gorm:"type:varchar(64)"`
	ServiceID     sql.NullString `json:"service_id" gorm:"type:char(36)"`
	ModelID       sql.NullString `json:"model_id" gorm:"type:char(36)"`

	GenAISystem        sql.NullString `json:"gen_ai_system" gorm:"type:varchar(50)"`
	GenAIOperationName string         `json:"gen_ai_operation_name" gorm:"type:varchar(50);not null"`

	GenAIRequestModel             sql.NullString  `json:"gen_ai_request_model" gorm:"type:varchar(255)"`
	GenAIRequestMaxTokens         sql.NullInt32   `json:"gen_ai_request_max_tokens"`
	GenAIRequestTemperature       sql.NullFloat64 `json:"gen_ai_request_temperature" gorm:"type:decimal(3,2)"`
	GenAIRequestTopP              sql.NullFloat64 `json:"gen_ai_request_top_p" gorm:"type:decimal(3,2)"`
	GenAIRequestPresencePenalty   sql.NullFloat64 `json:"gen_ai_request_presence_penalty" gorm:"type:decimal(3,2)"`
	GenAIRequestFrequencyPenalty  sql.NullFloat64 `json:"gen_ai_request_frequency_penalty" gorm:"type:decimal(3,2)"`
	GenAIRequestStopSequences     []byte          `json:"gen_ai_request_stop_sequences" gorm:"type:json"`

	GenAIResponseModel        sql.NullString `json:"gen_ai_response_model" gorm:"type:varchar(255)"`
	GenAIResponseFinishReasons []byte        `json:"gen_ai_response_finish_reasons" gorm:"type:json"`
	GenAIResponseID           sql.NullString `json:"gen_ai_response_id" gorm:"type:varchar(255)"`

	GenAIUsageInputTokens  sql.NullInt32 `json:"gen_ai_usage_input_tokens"`
	GenAIUsageOutputTokens sql.NullInt32 `json:"gen_ai_usage_output_tokens"`
	GenAIUsageTotalTokens  sql.NullInt32 `json:"gen_ai_usage_total_tokens"`

	Operation       sql.NullString `json:"operation" gorm:"type:varchar(50)"`
	PromptTokens    sql.NullInt32  `json:"prompt_tokens"`
	CompletionTokens sql.NullInt32 `json:"completion_tokens"`
	TotalTokens     sql.NullInt32  `json:"total_tokens"`

	DurationMs sql.NullInt32 `json:"duration_ms"`
	TTFTMs     sql.NullInt32 `json:"ttft_ms"`
	TPOTMs     sql.NullInt32 `json:"tpot_ms"`

	Status       string         `json:"status" gorm:"type:varchar(20);not null"`
	ErrorType    sql.NullString `json:"error_type" gorm:"type:varchar(100)"`
	ErrorMessage sql.NullString `json:"error_message" gorm:"type:text"`

	ResourceAttributes []byte          `json:"resource_attributes" gorm:"type:json"`
	UserID             sql.NullString  `json:"user_id" gorm:"type:varchar(255)"`
	SessionType        sql.NullString  `json:"session_type" gorm:"type:varchar(50)"`
	Metadata           []byte          `json:"metadata" gorm:"type:json"`
	CostUSD            sql.NullFloat64 `json:"cost_usd" gorm:"type:decimal(20,6)"`
}

func (AISession) TableName() string {
	return "ai_sessions"
}
