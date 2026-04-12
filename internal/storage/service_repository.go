package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
)

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Duplicate") ||
		strings.Contains(err.Error(), "UNIQUE constraint")
}

type ServiceRepository struct {
	db *gorm.DB
}

var _ ServiceRepositoryInterface = (*ServiceRepository)(nil)

func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func (r *ServiceRepository) Create(ctx context.Context, svc *models.Service) error {
	if err := r.db.WithContext(ctx).Create(svc).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("create service: %w", err)
	}
	return nil
}

func (r *ServiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Service, error) {
	var svc models.Service
	if err := r.db.WithContext(ctx).First(&svc, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get service by id: %w", err)
	}
	return &svc, nil
}

func (r *ServiceRepository) GetByName(ctx context.Context, name string) (*models.Service, error) {
	var svc models.Service
	if err := r.db.WithContext(ctx).First(&svc, "name = ?", name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get service by name: %w", err)
	}
	return &svc, nil
}

type ServiceFilter struct {
	Environment  string            `form:"environment"`
	HealthStatus string            `form:"health_status"`
	Name         string            `form:"name"`
	Labels       map[string]string `form:"labels"`
	Page         int               `form:"page"`
	PageSize     int               `form:"page_size"`
}

type ServiceListResult struct {
	Data     []models.Service
	Total    int64
	Page     int
	PageSize int
}

func (r *ServiceRepository) List(ctx context.Context, filter ServiceFilter) (*ServiceListResult, error) {
	var services []models.Service
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Service{})

	if filter.Environment != "" {
		query = query.Where("environment = ?", filter.Environment)
	}

	if filter.HealthStatus != "" {
		query = query.Where("health_status = ?", filter.HealthStatus)
	}

	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}

	if len(filter.Labels) > 0 {
		for key, value := range filter.Labels {
			subQuery := r.db.WithContext(ctx).
				Model(&models.ServiceLabel{}).
				Select("service_id").
				Where("key = ? AND value = ?", key, value)
			query = query.Where("id IN (?)", subQuery)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count services: %w", err)
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(filter.PageSize).Find(&services).Error; err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	return &ServiceListResult{
		Data:     services,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func (r *ServiceRepository) Update(ctx context.Context, svc *models.Service) error {
	result := r.db.WithContext(ctx).Save(svc)
	if result.Error != nil {
		return fmt.Errorf("update service: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Service{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete service: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Service{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check service exists: %w", err)
	}
	return count > 0, nil
}

func (r *ServiceRepository) SyncLabels(ctx context.Context, serviceID uuid.UUID, labels models.Labels) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id = ?", serviceID).Delete(&models.ServiceLabel{}).Error; err != nil {
			return fmt.Errorf("delete old labels: %w", err)
		}

		for key, value := range labels {
			label := &models.ServiceLabel{
				ServiceID: serviceID,
				Key:       key,
				Value:     value,
			}
			if label.ID == uuid.Nil {
				label.ID = uuid.New()
			}
			if err := tx.Create(label).Error; err != nil {
				return fmt.Errorf("create label: %w", err)
			}
		}

		return nil
	})
}

func (r *ServiceRepository) GetLabels(ctx context.Context, serviceID uuid.UUID) (models.Labels, error) {
	var labelRecords []models.ServiceLabel
	if err := r.db.WithContext(ctx).Where("service_id = ?", serviceID).Find(&labelRecords).Error; err != nil {
		return nil, fmt.Errorf("get labels: %w", err)
	}

	labels := make(models.Labels)
	for _, l := range labelRecords {
		labels[l.Key] = l.Value
	}
	return labels, nil
}

func (r *ServiceRepository) FindByLabelKey(ctx context.Context, key string) ([]models.Service, error) {
	var services []models.Service
	subQuery := r.db.WithContext(ctx).
		Model(&models.ServiceLabel{}).
		Select("service_id").
		Where("key = ?", key)

	if err := r.db.WithContext(ctx).Where("id IN (?)", subQuery).Find(&services).Error; err != nil {
		return nil, fmt.Errorf("find services by label key: %w", err)
	}
	return services, nil
}

func (r *ServiceRepository) FindByLabel(ctx context.Context, key, value string) ([]models.Service, error) {
	var services []models.Service
	subQuery := r.db.WithContext(ctx).
		Model(&models.ServiceLabel{}).
		Select("service_id").
		Where("key = ? AND value = ?", key, value)

	if err := r.db.WithContext(ctx).Where("id IN (?)", subQuery).Find(&services).Error; err != nil {
		return nil, fmt.Errorf("find services by label: %w", err)
	}
	return services, nil
}

func (r *ServiceRepository) GetAllLabelKeys(ctx context.Context) ([]string, error) {
	var keys []string
	if err := r.db.WithContext(ctx).
		Model(&models.ServiceLabel{}).
		Distinct("key").
		Pluck("key", &keys).Error; err != nil {
		return nil, fmt.Errorf("get all label keys: %w", err)
	}
	return keys, nil
}

func (r *ServiceRepository) GetLabelValues(ctx context.Context, key string) ([]string, error) {
	var values []string
	if err := r.db.WithContext(ctx).
		Model(&models.ServiceLabel{}).
		Where("key = ?", key).
		Distinct("value").
		Pluck("value", &values).Error; err != nil {
		return nil, fmt.Errorf("get label values: %w", err)
	}
	return values, nil
}

func (r *ServiceRepository) BatchCreate(ctx context.Context, services []*models.Service) ([]models.Service, error) {
	if len(services) == 0 {
		return []models.Service{}, nil
	}

	if err := r.db.WithContext(ctx).CreateInBatches(services, 100).Error; err != nil {
		return nil, fmt.Errorf("batch create services: %w", err)
	}

	result := make([]models.Service, len(services))
	for i, s := range services {
		result[i] = *s
	}
	return result, nil
}

func (r *ServiceRepository) BatchUpdate(ctx context.Context, services []*models.Service) ([]models.Service, error) {
	if len(services) == 0 {
		return []models.Service{}, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, svc := range services {
			if err := tx.Save(svc).Error; err != nil {
				return fmt.Errorf("update service %s: %w", svc.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]models.Service, len(services))
	for i, s := range services {
		result[i] = *s
	}
	return result, nil
}

func (r *ServiceRepository) BatchDelete(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).Delete(&models.Service{}, "id IN ?", ids).Error; err != nil {
		return fmt.Errorf("batch delete services: %w", err)
	}
	return nil
}

func (r *ServiceRepository) Search(ctx context.Context, query ServiceSearchQuery) (*ServiceListResult, error) {
	var services []models.Service
	var total int64

	db := r.db.WithContext(ctx).Model(&models.Service{})

	if query.Query != "" {
		searchPattern := "%" + query.Query + "%"
		db = db.Where("name LIKE ? OR description LIKE ? OR team LIKE ? OR maintainer LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)
	}

	if query.Environment != "" {
		db = db.Where("environment = ?", query.Environment)
	}

	if query.Team != "" {
		db = db.Where("team = ?", query.Team)
	}

	if query.Maintainer != "" {
		db = db.Where("maintainer = ?", query.Maintainer)
	}

	if query.HealthStatus != "" {
		db = db.Where("health_status = ?", query.HealthStatus)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count services: %w", err)
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	offset := (query.Page - 1) * query.PageSize
	if err := db.Order("created_at DESC").Offset(offset).Limit(query.PageSize).Find(&services).Error; err != nil {
		return nil, fmt.Errorf("search services: %w", err)
	}

	return &ServiceListResult{
		Data:     services,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (r *ServiceRepository) UpdateOpenAPI(ctx context.Context, id uuid.UUID, spec string) error {
	result := r.db.WithContext(ctx).Model(&models.Service{}).Where("id = ?", id).Update("open_api_spec", spec)
	if result.Error != nil {
		return fmt.Errorf("update openapi spec: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) GetOpenAPI(ctx context.Context, id uuid.UUID) (string, error) {
	var svc models.Service
	if err := r.db.WithContext(ctx).Select("open_api_spec").First(&svc, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get openapi spec: %w", err)
	}
	return svc.OpenAPISpec, nil
}

func (r *ServiceRepository) AddDependency(ctx context.Context, dep *models.ServiceDependency) error {
	if err := r.db.WithContext(ctx).Create(dep).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || isDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("add dependency: %w", err)
	}
	return nil
}

func (r *ServiceRepository) RemoveDependency(ctx context.Context, serviceID, dependsOnID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("service_id = ? AND depends_on_id = ?", serviceID, dependsOnID).
		Delete(&models.ServiceDependency{})
	if result.Error != nil {
		return fmt.Errorf("remove dependency: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServiceRepository) GetDependencies(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error) {
	var deps []models.ServiceDependency
	if err := r.db.WithContext(ctx).
		Where("service_id = ?", serviceID).
		Preload("DependsOn").
		Find(&deps).Error; err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}
	return deps, nil
}

func (r *ServiceRepository) GetDependents(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error) {
	var deps []models.ServiceDependency
	if err := r.db.WithContext(ctx).
		Where("depends_on_id = ?", serviceID).
		Preload("Service").
		Find(&deps).Error; err != nil {
		return nil, fmt.Errorf("get dependents: %w", err)
	}
	return deps, nil
}

func (r *ServiceRepository) GetDependencyGraph(ctx context.Context) ([]models.ServiceDependency, error) {
	var deps []models.ServiceDependency
	if err := r.db.WithContext(ctx).
		Preload("Service").
		Preload("DependsOn").
		Find(&deps).Error; err != nil {
		return nil, fmt.Errorf("get dependency graph: %w", err)
	}
	return deps, nil
}
