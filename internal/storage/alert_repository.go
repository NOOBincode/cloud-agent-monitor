package storage

import (
	"context"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlertOperationRepositoryInterface interface {
	Create(ctx context.Context, op *models.AlertOperation) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AlertOperation, error)
	GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertOperation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.AlertOperationStatus, errMsg string) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	ListPending(ctx context.Context, limit int) ([]*models.AlertOperation, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AlertOperation, int64, error)
	ListByStatus(ctx context.Context, status models.AlertOperationStatus, limit int) ([]*models.AlertOperation, error)
	Update(ctx context.Context, op *models.AlertOperation) error
}

type AlertNoiseRepositoryInterface interface {
	Upsert(ctx context.Context, record *models.AlertNoiseRecord) error
	GetByFingerprint(ctx context.Context, fingerprint string) (*models.AlertNoiseRecord, error)
	GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	UpdateNoiseScore(ctx context.Context, fingerprint string, score float64, isNoisy bool) error
	IncrementFireCount(ctx context.Context, fingerprint string) error
	IncrementResolveCount(ctx context.Context, fingerprint string) error
}

type AlertNotificationRepositoryInterface interface {
	Create(ctx context.Context, notification *models.AlertNotification) error
	GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertNotification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error
}

type AlertOperationRepository struct {
	db *gorm.DB
}

func NewAlertOperationRepository(db *gorm.DB) AlertOperationRepositoryInterface {
	return &AlertOperationRepository{db: db}
}

func (r *AlertOperationRepository) Create(ctx context.Context, op *models.AlertOperation) error {
	return r.db.WithContext(ctx).Create(op).Error
}

func (r *AlertOperationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AlertOperation, error) {
	var op models.AlertOperation
	err := r.db.WithContext(ctx).First(&op, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (r *AlertOperationRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertOperation, error) {
	var ops []*models.AlertOperation
	err := r.db.WithContext(ctx).
		Where("alert_fingerprint = ?", fingerprint).
		Order("created_at DESC").
		Limit(limit).
		Find(&ops).Error
	return ops, err
}

func (r *AlertOperationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.AlertOperationStatus, errMsg string) error {
	updates := map[string]interface{}{
		"status":       status,
		"error_message": errMsg,
		"processed_at": time.Now(),
	}
	return r.db.WithContext(ctx).Model(&models.AlertOperation{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *AlertOperationRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.AlertOperation{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *AlertOperationRepository) ListPending(ctx context.Context, limit int) ([]*models.AlertOperation, error) {
	var ops []*models.AlertOperation
	err := r.db.WithContext(ctx).
		Where("status IN ?", []models.AlertOperationStatus{models.AlertOpStatusPending, models.AlertOpStatusRetrying}).
		Where("retry_count < max_retries").
		Order("created_at ASC").
		Limit(limit).
		Find(&ops).Error
	return ops, err
}

func (r *AlertOperationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AlertOperation, int64, error) {
	var ops []*models.AlertOperation
	var total int64

	db := r.db.WithContext(ctx).Model(&models.AlertOperation{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ops).Error
	return ops, total, err
}

func (r *AlertOperationRepository) ListByStatus(ctx context.Context, status models.AlertOperationStatus, limit int) ([]*models.AlertOperation, error) {
	var ops []*models.AlertOperation
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC").
		Limit(limit).
		Find(&ops).Error
	return ops, err
}

func (r *AlertOperationRepository) Update(ctx context.Context, op *models.AlertOperation) error {
	return r.db.WithContext(ctx).Save(op).Error
}

type AlertNoiseRepository struct {
	db *gorm.DB
}

func NewAlertNoiseRepository(db *gorm.DB) AlertNoiseRepositoryInterface {
	return &AlertNoiseRepository{db: db}
}

func (r *AlertNoiseRepository) Upsert(ctx context.Context, record *models.AlertNoiseRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

func (r *AlertNoiseRepository) GetByFingerprint(ctx context.Context, fingerprint string) (*models.AlertNoiseRecord, error) {
	var record models.AlertNoiseRecord
	err := r.db.WithContext(ctx).First(&record, "alert_fingerprint = ?", fingerprint).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *AlertNoiseRepository) GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	var records []*models.AlertNoiseRecord
	err := r.db.WithContext(ctx).
		Where("is_noisy = ?", true).
		Order("noise_score DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func (r *AlertNoiseRepository) GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	var records []*models.AlertNoiseRecord
	err := r.db.WithContext(ctx).
		Where("is_high_risk = ?", true).
		Order("last_fired_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func (r *AlertNoiseRepository) UpdateNoiseScore(ctx context.Context, fingerprint string, score float64, isNoisy bool) error {
	return r.db.WithContext(ctx).Model(&models.AlertNoiseRecord{}).
		Where("alert_fingerprint = ?", fingerprint).
		Updates(map[string]interface{}{
			"noise_score": score,
			"is_noisy":    isNoisy,
		}).Error
}

func (r *AlertNoiseRepository) IncrementFireCount(ctx context.Context, fingerprint string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.AlertNoiseRecord{}).
		Where("alert_fingerprint = ?", fingerprint).
		Updates(map[string]interface{}{
			"fire_count":   gorm.Expr("fire_count + 1"),
			"last_fired_at": now,
		}).Error
}

func (r *AlertNoiseRepository) IncrementResolveCount(ctx context.Context, fingerprint string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.AlertNoiseRecord{}).
		Where("alert_fingerprint = ?", fingerprint).
		Updates(map[string]interface{}{
			"resolve_count":    gorm.Expr("resolve_count + 1"),
			"last_resolved_at": now,
		}).Error
}

type AlertNotificationRepository struct {
	db *gorm.DB
}

func NewAlertNotificationRepository(db *gorm.DB) AlertNotificationRepositoryInterface {
	return &AlertNotificationRepository{db: db}
}

func (r *AlertNotificationRepository) Create(ctx context.Context, notification *models.AlertNotification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

func (r *AlertNotificationRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertNotification, error) {
	var notifications []*models.AlertNotification
	err := r.db.WithContext(ctx).
		Where("alert_fingerprint = ?", fingerprint).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifications).Error
	return notifications, err
}

func (r *AlertNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	sentAt := time.Now()
	if status != "sent" {
		sentAt = time.Time{}
	}
	return r.db.WithContext(ctx).Model(&models.AlertNotification{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"error_message": errMsg,
			"sent_at":      sentAt,
		}).Error
}
