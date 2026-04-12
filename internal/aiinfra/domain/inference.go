package domain

import (
	"context"
	"time"
)

type InferenceServiceStatus string

const (
	InferenceServiceActive   InferenceServiceStatus = "active"
	InferenceServiceDegraded InferenceServiceStatus = "degraded"
	InferenceServiceInactive InferenceServiceStatus = "inactive"
)

type ModelVersionStatus string

const (
	ModelVersionStaging    ModelVersionStatus = "staging"
	ModelVersionProduction ModelVersionStatus = "production"
	ModelVersionDeprecated ModelVersionStatus = "deprecated"
)

type InferenceService struct {
	ID        string
	ServiceID string

	Name      string
	Engine    InferenceEngine
	Version   string

	ModelName      string
	ModelVersion   string
	ModelFramework string

	DeploymentType string
	Replicas       int
	GPUPerReplica  int

	Config       map[string]interface{}

	Status       InferenceServiceStatus
	EndpointURL  string

	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type InferenceRequest struct {
	ID                 string
	InferenceServiceID string
	SessionID          string

	TraceID string
	SpanID  string

	RequestID string
	ModelName string

	TTFTMs       int
	TPOTMs       int
	E2ELatencyMs int

	PromptTokens     int
	CompletionTokens int
	TotalTokens      int

	TokensPerSecond float64

	QueueTimeMs   int
	QueuePosition int

	GPUMemoryUsedMB int
	GPUUtilization  float64

	BatchSize int

	Status       string
	ErrorType    string
	ErrorMessage string

	StartedAt   time.Time
	CompletedAt time.Time
	CreatedAt   time.Time
}

type ModelVersion struct {
	ID                 string
	InferenceServiceID string

	Version    string
	ModelPath  string

	ModelSizeMB     int
	ParametersCount int64
	Quantization    string

	AvgTTFTMs           int
	AvgTPOTMs           int
	MaxThroughputTokens float64

	Status     ModelVersionStatus
	IsDefault  bool

	DeployedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InferenceServiceRepository interface {
	Create(ctx context.Context, service *InferenceService) error
	GetByID(ctx context.Context, id string) (*InferenceService, error)
	GetByName(ctx context.Context, name string) (*InferenceService, error)
	List(ctx context.Context, status *InferenceServiceStatus) ([]*InferenceService, error)
	Update(ctx context.Context, service *InferenceService) error
}

type InferenceRequestRepository interface {
	Create(ctx context.Context, req *InferenceRequest) error
	GetByID(ctx context.Context, id string) (*InferenceRequest, error)
	ListByService(ctx context.Context, serviceID string, filter *InferenceRequestFilter) ([]*InferenceRequest, error)
}

type InferenceRequestFilter struct {
	StartTime  time.Time
	EndTime    time.Time
	Status     string
	ModelName  string
	Limit      int
	Offset     int
}

type ModelVersionRepository interface {
	Create(ctx context.Context, version *ModelVersion) error
	GetByID(ctx context.Context, id string) (*ModelVersion, error)
	ListByService(ctx context.Context, serviceID string) ([]*ModelVersion, error)
	GetDefault(ctx context.Context, serviceID string) (*ModelVersion, error)
	Update(ctx context.Context, version *ModelVersion) error
}
