package storage

import (
	"context"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AlertRecordRepositoryInterface interface {
	Create(ctx context.Context, record *models.AlertRecord) error
	CreateBatch(ctx context.Context, records []*models.AlertRecord) error
	GetByID(ctx context.Context, id string) (*models.AlertRecord, error)
	GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertRecord, error)
	List(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error)
	UpdateStatus(ctx context.Context, fingerprint string, status models.AlertStatus, endsAt *time.Time) error
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
	GetStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error)
}

type AlertRecordRepository struct {
	db *gorm.DB
}

func NewAlertRecordRepository(db *gorm.DB) AlertRecordRepositoryInterface {
	return &AlertRecordRepository{db: db}
}

func (r *AlertRecordRepository) Create(ctx context.Context, record *models.AlertRecord) error {
	record.ExtractSeverity()
	record.CalculateDuration()
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *AlertRecordRepository) CreateBatch(ctx context.Context, records []*models.AlertRecord) error {
	if len(records) == 0 {
		return nil
	}

	for _, record := range records {
		record.ExtractSeverity()
		record.CalculateDuration()
	}

	batchSize := 500
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).
		CreateInBatches(records, batchSize).Error
}

func (r *AlertRecordRepository) GetByID(ctx context.Context, id string) (*models.AlertRecord, error) {
	var record models.AlertRecord
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *AlertRecordRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertRecord, error) {
	var records []*models.AlertRecord
	query := r.db.WithContext(ctx).
		Where("fingerprint = ?", fingerprint).
		Order("starts_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&records).Error
	return records, err
}

func (r *AlertRecordRepository) List(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error) {
	var records []*models.AlertRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AlertRecord{})

	if filter.Fingerprint != "" {
		query = query.Where("fingerprint = ?", filter.Fingerprint)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.AlertName != "" {
		query = query.Where("labels->>'$.alertname' = ?", filter.AlertName)
	}
	if filter.StartFrom != nil {
		query = query.Where("starts_at >= ?", filter.StartFrom)
	}
	if filter.StartTo != nil {
		query = query.Where("starts_at <= ?", filter.StartTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := (page - 1) * pageSize
	err := query.Order("starts_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records).Error

	return records, total, err
}

func (r *AlertRecordRepository) UpdateStatus(ctx context.Context, fingerprint string, status models.AlertStatus, endsAt *time.Time) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if endsAt != nil {
		updates["ends_at"] = endsAt
	}

	return r.db.WithContext(ctx).
		Model(&models.AlertRecord{}).
		Where("fingerprint = ? AND status = ?", fingerprint, models.AlertStatusFiring).
		Updates(updates).Error
}

func (r *AlertRecordRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.AlertRecord{})
	return result.RowsAffected, result.Error
}

func (r *AlertRecordRepository) GetStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error) {
	var stats models.AlertRecordStats

	err := r.db.WithContext(ctx).
		Model(&models.AlertRecord{}).
		Where("starts_at >= ? AND starts_at <= ?", from, to).
		Select(`
			COUNT(*) as total_count,
			SUM(CASE WHEN status = 'firing' THEN 1 ELSE 0 END) as firing_count,
			SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved_count,
			SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END) as critical_count,
			SUM(CASE WHEN severity = 'warning' THEN 1 ELSE 0 END) as warning_count,
			SUM(CASE WHEN severity = 'info' THEN 1 ELSE 0 END) as info_count,
			COALESCE(AVG(duration), 0) as avg_duration
		`).
		Scan(&stats).Error

	return &stats, err
}
