package models

import (
	"database/sql"
	"time"
)

type InferenceService struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	ServiceID sql.NullString `json:"service_id" gorm:"type:char(36)"`

	Name      string    `json:"name" gorm:"type:varchar(255);unique;not null"`
	Engine    string    `json:"engine" gorm:"type:varchar(50);not null"`
	Version   sql.NullString `json:"version" gorm:"type:varchar(50)"`

	ModelName      string    `json:"model_name" gorm:"type:varchar(255);not null"`
	ModelVersion   sql.NullString `json:"model_version" gorm:"type:varchar(50)"`
	ModelFramework sql.NullString `json:"model_framework" gorm:"type:varchar(50)"`

	DeploymentType string `json:"deployment_type" gorm:"type:varchar(20);not null"`
	Replicas       int    `json:"replicas" gorm:"default:1"`
	GPUPerReplica  int    `json:"gpu_per_replica" gorm:"default:1"`

	Config       []byte `json:"config" gorm:"type:json"`

	Status       string    `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	EndpointURL  sql.NullString `json:"endpoint_url" gorm:"type:varchar(500)"`

	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (InferenceService) TableName() string {
	return "inference_services"
}

type InferenceRequest struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:char(36)"`
	InferenceServiceID string    `json:"inference_service_id" gorm:"type:char(36);not null"`
	SessionID          sql.NullString `json:"session_id" gorm:"type:char(36)"`

	TraceID sql.NullString `json:"trace_id" gorm:"type:varchar(64)"`
	SpanID  sql.NullString `json:"span_id" gorm:"type:varchar(64)"`

	RequestID  sql.NullString `json:"request_id" gorm:"type:varchar(255)"`
	ModelName  sql.NullString `json:"model_name" gorm:"type:varchar(255)"`

	TTFTMs         sql.NullInt32 `json:"ttft_ms"`
	TPOTMs         sql.NullInt32 `json:"tpot_ms"`
	E2ELatencyMs   sql.NullInt32 `json:"e2e_latency_ms"`

	PromptTokens     sql.NullInt32 `json:"prompt_tokens"`
	CompletionTokens sql.NullInt32 `json:"completion_tokens"`
	TotalTokens      sql.NullInt32 `json:"total_tokens"`

	TokensPerSecond sql.NullFloat64 `json:"tokens_per_second" gorm:"type:decimal(10,2)"`

	QueueTimeMs   sql.NullInt32 `json:"queue_time_ms"`
	QueuePosition sql.NullInt32 `json:"queue_position"`

	GPUMemoryUsedMB sql.NullInt32   `json:"gpu_memory_used_mb"`
	GPUUtilization  sql.NullFloat64 `json:"gpu_utilization" gorm:"type:decimal(5,2)"`

	BatchSize sql.NullInt32 `json:"batch_size"`

	Status       string         `json:"status" gorm:"type:varchar(20);not null"`
	ErrorType    sql.NullString `json:"error_type" gorm:"type:varchar(100)"`
	ErrorMessage sql.NullString `json:"error_message" gorm:"type:text"`

	StartedAt    time.Time    `json:"started_at" gorm:"not null"`
	CompletedAt  sql.NullTime `json:"completed_at"`
	CreatedAt    time.Time    `json:"created_at" gorm:"autoCreateTime"`
}

func (InferenceRequest) TableName() string {
	return "inference_requests"
}

type ModelVersion struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:char(36)"`
	InferenceServiceID string    `json:"inference_service_id" gorm:"type:char(36);not null"`

	Version    string    `json:"version" gorm:"type:varchar(50);not null"`
	ModelPath  sql.NullString `json:"model_path" gorm:"type:varchar(500)"`

	ModelSizeMB      sql.NullInt32 `json:"model_size_mb"`
	ParametersCount  sql.NullInt64 `json:"parameters_count"`
	Quantization     sql.NullString `json:"quantization" gorm:"type:varchar(50)"`

	AvgTTFTMs            sql.NullInt32 `json:"avg_ttft_ms"`
	AvgTPOTMs            sql.NullInt32 `json:"avg_tpot_ms"`
	MaxThroughputTokens  sql.NullFloat64 `json:"max_throughput_tokens" gorm:"type:decimal(10,2)"`

	Status     string    `json:"status" gorm:"type:varchar(20);not null;default:'staging'"`
	IsDefault  bool      `json:"is_default" gorm:"default:false"`

	DeployedAt sql.NullTime `json:"deployed_at"`
	CreatedAt  time.Time    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time    `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ModelVersion) TableName() string {
	return "model_versions"
}
