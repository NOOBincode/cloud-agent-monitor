package storage

import (
	"context"
	"errors"

	"cloud-agent-monitor/internal/slo/domain"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrSLONotFound = errors.New("slo not found")
	ErrSLINotFound = errors.New("sli not found")
)

type SLORepository struct {
	db *gorm.DB
}

func NewSLORepository(db *gorm.DB) *SLORepository {
	return &SLORepository{db: db}
}

func (r *SLORepository) Create(ctx context.Context, slo *domain.SLO) error {
	model := r.domainToModel(slo)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *SLORepository) Update(ctx context.Context, slo *domain.SLO) error {
	model := r.domainToModel(slo)
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *SLORepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.SLO{}, id).Error
}

func (r *SLORepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SLO, error) {
	var model models.SLO
	err := r.db.WithContext(ctx).
		Preload("SLIs").
		Preload("Service").
		First(&model, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLONotFound
		}
		return nil, err
	}
	return r.modelToDomain(&model), nil
}

func (r *SLORepository) GetByServiceID(ctx context.Context, serviceID uuid.UUID) ([]*domain.SLO, error) {
	var modelList []models.SLO
	err := r.db.WithContext(ctx).
		Preload("SLIs").
		Where("service_id = ?", serviceID).
		Find(&modelList).Error
	if err != nil {
		return nil, err
	}

	slos := make([]*domain.SLO, len(modelList))
	for i, m := range modelList {
		slos[i] = r.modelToDomain(&m)
	}
	return slos, nil
}

func (r *SLORepository) List(ctx context.Context, filter domain.SLOFilter) ([]*domain.SLO, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.SLO{}).Preload("SLIs")

	if filter.ServiceID != uuid.Nil {
		query = query.Where("service_id = ?", filter.ServiceID)
	}
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	var modelList []models.SLO
	if err := query.Offset(offset).Limit(filter.PageSize).Order("created_at DESC").Find(&modelList).Error; err != nil {
		return nil, 0, err
	}

	slos := make([]*domain.SLO, len(modelList))
	for i, m := range modelList {
		slos[i] = r.modelToDomain(&m)
	}

	return slos, total, nil
}

func (r *SLORepository) GetSummary(ctx context.Context) (*domain.SLOSummary, error) {
	var slos []models.SLO
	if err := r.db.WithContext(ctx).Find(&slos).Error; err != nil {
		return nil, err
	}

	summary := &domain.SLOSummary{
		Total:      len(slos),
		ByStatus:   make(map[domain.SLOStatus]int),
		ByService:  make(map[string]int),
		AvgBurnRate: 0,
	}

	var totalBurnRate float64
	for _, slo := range slos {
		summary.ByStatus[domain.SLOStatus(slo.Status)]++
		if slo.Service != nil {
			summary.ByService[slo.Service.Name]++
		}
		totalBurnRate += slo.BurnRate
	}

	if len(slos) > 0 {
		summary.AvgBurnRate = totalBurnRate / float64(len(slos))
	}

	return summary, nil
}

func (r *SLORepository) domainToModel(slo *domain.SLO) *models.SLO {
	return &models.SLO{
		ID:           slo.ID,
		Name:         slo.Name,
		Description:  slo.Description,
		ServiceID:    slo.ServiceID,
		Target:       slo.Target,
		Window:       slo.Window,
		WarningBurn:  slo.WarningBurn,
		CriticalBurn: slo.CriticalBurn,
		Status:       models.SLOStatus(slo.Status),
		CurrentValue: slo.Current,
		BurnRate:     slo.BurnRate,
	}
}

func (r *SLORepository) modelToDomain(m *models.SLO) *domain.SLO {
	slo := &domain.SLO{
		ID:           m.ID,
		Name:         m.Name,
		Description:  m.Description,
		ServiceID:    m.ServiceID,
		Target:       m.Target,
		Window:       m.Window,
		WarningBurn:  m.WarningBurn,
		CriticalBurn: m.CriticalBurn,
		Status:       domain.SLOStatus(m.Status),
		Current:      m.CurrentValue,
		BurnRate:     m.BurnRate,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}

	if m.Service != nil {
		slo.ServiceName = m.Service.Name
	}

	if len(m.SLIs) > 0 {
		sli := m.SLIs[0]
		slo.SLI = domain.SLI{
			ID:          sli.ID,
			SLOID:       sli.SLOID,
			Name:        sli.Name,
			Type:        domain.SLIType(sli.Type),
			Query:       sli.Query,
			Threshold:   sli.Threshold,
			Unit:        sli.Unit,
			Description: sli.Description,
		}
	}

	return slo
}

type SLIRepository struct {
	db *gorm.DB
}

func NewSLIRepository(db *gorm.DB) *SLIRepository {
	return &SLIRepository{db: db}
}

func (r *SLIRepository) Create(ctx context.Context, sli *domain.SLI) error {
	model := &models.SLI{
		ID:          sli.ID,
		SLOID:       sli.SLOID,
		Name:        sli.Name,
		Type:        models.SLIType(sli.Type),
		Query:       sli.Query,
		Threshold:   sli.Threshold,
		Unit:        sli.Unit,
		Description: sli.Description,
	}
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *SLIRepository) Update(ctx context.Context, sli *domain.SLI) error {
	model := &models.SLI{
		ID:          sli.ID,
		SLOID:       sli.SLOID,
		Name:        sli.Name,
		Type:        models.SLIType(sli.Type),
		Query:       sli.Query,
		Threshold:   sli.Threshold,
		Unit:        sli.Unit,
		Description: sli.Description,
	}
	return r.db.WithContext(ctx).Save(model).Error
}

func (r *SLIRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.SLI{}, id).Error
}

func (r *SLIRepository) GetBySLOID(ctx context.Context, sloID uuid.UUID) ([]*domain.SLI, error) {
	var modelList []models.SLI
	err := r.db.WithContext(ctx).Where("slo_id = ?", sloID).Find(&modelList).Error
	if err != nil {
		return nil, err
	}

	slis := make([]*domain.SLI, len(modelList))
	for i, m := range modelList {
		slis[i] = &domain.SLI{
			ID:          m.ID,
			SLOID:       m.SLOID,
			Name:        m.Name,
			Type:        domain.SLIType(m.Type),
			Query:       m.Query,
			Threshold:   m.Threshold,
			Unit:        m.Unit,
			Description: m.Description,
		}
	}
	return slis, nil
}
