package domain

import (
	"context"
	"time"
)

type QueueJob struct {
	ID       string
	JobName  string
	JobType  JobType
	QueueName string

	K8sNamespace string
	K8sJobName   string
	K8sUID       string

	GPUCount int
	CPUCores float64
	MemoryMB int

	Priority      int
	Scheduler     string
	QueuePosition int

	SubmittedAt     time.Time
	StartedAt       time.Time
	CompletedAt     time.Time
	QueueWaitTimeMs int64
	ExecutionTimeMs int64

	Status       JobStatus
	RetryCount   int
	ErrorMessage string

	EstimatedCostUSD float64
	ActualCostUSD    float64

	Labels      map[string]string
	Annotations map[string]string

	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ResourceType string

const (
	ResourceTypeGPU     ResourceType = "gpu"
	ResourceTypeCPU     ResourceType = "cpu"
	ResourceTypeMemory  ResourceType = "memory"
	ResourceTypeStorage ResourceType = "storage"
)

type ResourceAllocation struct {
	ID        string
	JobID     string

	ResourceType ResourceType
	ResourceName string

	Requested float64
	Allocated float64
	Used      float64
	Unit      string

	NodeName string
	GPUIndex int

	AllocatedAt time.Time
	ReleasedAt  time.Time
	CreatedAt   time.Time
}

type QueueJobRepository interface {
	Create(ctx context.Context, job *QueueJob) error
	GetByID(ctx context.Context, id string) (*QueueJob, error)
	GetByK8sUID(ctx context.Context, k8sUID string) (*QueueJob, error)
	List(ctx context.Context, filter *QueueJobFilter) ([]*QueueJob, error)
	Update(ctx context.Context, job *QueueJob) error
}

type QueueJobFilter struct {
	QueueName  string
	Namespace  string
	Status     JobStatus
	JobType    JobType
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
	Offset     int
}

type ResourceAllocationRepository interface {
	Create(ctx context.Context, allocation *ResourceAllocation) error
	ListByJob(ctx context.Context, jobID string) ([]*ResourceAllocation, error)
	ListByNode(ctx context.Context, nodeName string, activeOnly bool) ([]*ResourceAllocation, error)
	Update(ctx context.Context, allocation *ResourceAllocation) error
}
