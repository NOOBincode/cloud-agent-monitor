package storage

import (
	"context"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
)

type ServiceRepositoryInterface interface {
	Create(ctx context.Context, svc *models.Service) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Service, error)
	GetByName(ctx context.Context, name string) (*models.Service, error)
	List(ctx context.Context, filter ServiceFilter) (*ServiceListResult, error)
	Update(ctx context.Context, svc *models.Service) error
	Delete(ctx context.Context, id uuid.UUID) error
	ExistsByName(ctx context.Context, name string) (bool, error)
	SyncLabels(ctx context.Context, serviceID uuid.UUID, labels models.Labels) error
	GetLabels(ctx context.Context, serviceID uuid.UUID) (models.Labels, error)
	FindByLabelKey(ctx context.Context, key string) ([]models.Service, error)
	FindByLabel(ctx context.Context, key, value string) ([]models.Service, error)
	GetAllLabelKeys(ctx context.Context) ([]string, error)
	GetLabelValues(ctx context.Context, key string) ([]string, error)

	BatchCreate(ctx context.Context, services []*models.Service) ([]models.Service, error)
	BatchUpdate(ctx context.Context, services []*models.Service) ([]models.Service, error)
	BatchDelete(ctx context.Context, ids []uuid.UUID) error
	Search(ctx context.Context, query ServiceSearchQuery) (*ServiceListResult, error)
	UpdateOpenAPI(ctx context.Context, id uuid.UUID, spec string) error
	GetOpenAPI(ctx context.Context, id uuid.UUID) (string, error)

	AddDependency(ctx context.Context, dep *models.ServiceDependency) error
	RemoveDependency(ctx context.Context, serviceID, dependsOnID uuid.UUID) error
	GetDependencies(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error)
	GetDependents(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error)
	GetDependencyGraph(ctx context.Context) ([]models.ServiceDependency, error)
}

type ServiceSearchQuery struct {
	Query       string `form:"q"`
	Environment string `form:"environment"`
	Team        string `form:"team"`
	Maintainer  string `form:"maintainer"`
	HealthStatus string `form:"health_status"`
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
}
