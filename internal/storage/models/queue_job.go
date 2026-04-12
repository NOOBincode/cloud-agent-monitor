package models

import (
	"database/sql"
	"time"
)

type QueueJob struct {
	ID       string    `json:"id" gorm:"primaryKey;type:char(36)"`
	JobName  string    `json:"job_name" gorm:"type:varchar(255);not null"`
	JobType  string    `json:"job_type" gorm:"type:varchar(50);not null"`
	QueueName sql.NullString `json:"queue_name" gorm:"type:varchar(255)"`

	K8sNamespace sql.NullString `json:"k8s_namespace" gorm:"type:varchar(255)"`
	K8sJobName   sql.NullString `json:"k8s_job_name" gorm:"type:varchar(255)"`
	K8sUID       sql.NullString `json:"k8s_uid" gorm:"type:varchar(255)"`

	GPUCount  sql.NullInt32 `json:"gpu_count"`
	CPUCores  sql.NullFloat64 `json:"cpu_cores" gorm:"type:decimal(10,2)"`
	MemoryMB  sql.NullInt32 `json:"memory_mb"`

	Priority      sql.NullInt32 `json:"priority"`
	Scheduler     sql.NullString `json:"scheduler" gorm:"type:varchar(50)"`
	QueuePosition sql.NullInt32 `json:"queue_position"`

	SubmittedAt     sql.NullTime `json:"submitted_at"`
	StartedAt       sql.NullTime `json:"started_at"`
	CompletedAt     sql.NullTime `json:"completed_at"`
	QueueWaitTimeMs sql.NullInt64 `json:"queue_wait_time_ms"`
	ExecutionTimeMs sql.NullInt64 `json:"execution_time_ms"`

	Status       string         `json:"status" gorm:"type:varchar(20);not null"`
	RetryCount   int            `json:"retry_count" gorm:"default:0"`
	ErrorMessage sql.NullString `json:"error_message" gorm:"type:text"`

	EstimatedCostUSD sql.NullFloat64 `json:"estimated_cost_usd" gorm:"type:decimal(20,6)"`
	ActualCostUSD    sql.NullFloat64 `json:"actual_cost_usd" gorm:"type:decimal(20,6)"`

	Labels      []byte `json:"labels" gorm:"type:json"`
	Annotations []byte `json:"annotations" gorm:"type:json"`

	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (QueueJob) TableName() string {
	return "queue_jobs"
}

type ResourceAllocation struct {
	ID        string    `json:"id" gorm:"primaryKey;type:char(36)"`
	JobID     sql.NullString `json:"job_id" gorm:"type:char(36)"`

	ResourceType string    `json:"resource_type" gorm:"type:varchar(50);not null"`
	ResourceName sql.NullString `json:"resource_name" gorm:"type:varchar(255)"`

	Requested sql.NullFloat64 `json:"requested" gorm:"type:decimal(20,6)"`
	Allocated sql.NullFloat64 `json:"allocated" gorm:"type:decimal(20,6)"`
	Used      sql.NullFloat64 `json:"used" gorm:"type:decimal(20,6)"`
	Unit      sql.NullString  `json:"unit" gorm:"type:varchar(20)"`

	NodeName sql.NullString `json:"node_name" gorm:"type:varchar(255)"`
	GPUIndex sql.NullInt32  `json:"gpu_index"`

	AllocatedAt time.Time    `json:"allocated_at" gorm:"not null"`
	ReleasedAt  sql.NullTime `json:"released_at"`
	CreatedAt   time.Time    `json:"created_at" gorm:"autoCreateTime"`
}

func (ResourceAllocation) TableName() string {
	return "resource_allocations"
}
