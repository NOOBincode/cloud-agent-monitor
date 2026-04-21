package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

type Service struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Environment string    `gorm:"type:varchar(50);not null;default:'local';index" json:"environment"`
	Labels      Labels    `gorm:"type:json;serializer:json" json:"labels"`

	Endpoint    string `gorm:"type:varchar(500)" json:"endpoint"`
	OpenAPISpec string `gorm:"type:longtext" json:"openapi_spec,omitempty"`

	HealthStatus       HealthStatus `gorm:"type:varchar(20);default:'unknown';index" json:"health_status"`
	LastHealthCheckAt  *time.Time   `json:"last_health_check_at,omitempty"`
	HealthCheckDetails string       `gorm:"type:text" json:"health_check_details,omitempty"`

	Maintainer       string `gorm:"type:varchar(255)" json:"maintainer,omitempty"`
	Team             string `gorm:"type:varchar(255)" json:"team,omitempty"`
	DocumentationURL string `gorm:"type:varchar(500)" json:"documentation_url,omitempty"`
	RepositoryURL    string `gorm:"type:varchar(500)" json:"repository_url,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Service) TableName() string {
	return "services"
}

func (s *Service) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.HealthStatus == "" {
		s.HealthStatus = HealthStatusUnknown
	}
	return nil
}

type Labels map[string]string

type ServiceHealthMetric struct {
	ServiceName string
	Status      HealthStatus
	Metrics     map[string]float64
	LastChecked time.Time
	Details     string
}

type ServiceDependency struct {
	ID           uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	ServiceID    uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_service_dep;index" json:"service_id"`
	DependsOnID  uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_service_dep;index" json:"depends_on_id"`
	RelationType string         `gorm:"type:varchar(50);default:'depends_on'" json:"relation_type"`
	Description  string         `gorm:"type:text" json:"description,omitempty"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Service   *Service `gorm:"foreignKey:ServiceID" json:"service,omitempty"`
	DependsOn *Service `gorm:"foreignKey:DependsOnID" json:"depends_on,omitempty"`
}

func (ServiceDependency) TableName() string {
	return "service_dependencies"
}

func (d *ServiceDependency) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.RelationType == "" {
		d.RelationType = "depends_on"
	}
	return nil
}

const (
	RelationTypeDependsOn = "depends_on"
	RelationTypeCalls     = "calls"
	RelationTypeProvides  = "provides"
	RelationTypeConsumes  = "consumes"
)
