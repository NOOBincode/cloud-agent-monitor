package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/internal/alerting/infrastructure"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
)

var (
	ErrAlertNotFound      = errors.New("alert not found")
	ErrSilenceNotFound    = errors.New("silence not found")
	ErrReceiverNotFound   = errors.New("receiver not found")
	ErrInvalidAlert       = errors.New("invalid alert data")
	ErrInvalidSilence     = errors.New("invalid silence data")
	ErrDuplicateOperation = errors.New("duplicate operation")
)

type AlertServiceInterface interface {
	SendAlert(ctx context.Context, alert *domain.Alert) error
	SendAlerts(ctx context.Context, alerts []*domain.Alert) error
	SendAlertWithRecord(ctx context.Context, alert *domain.Alert, userID uuid.UUID, ipAddress, userAgent string) (*models.AlertOperation, error)
	GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error)
	GetAlert(ctx context.Context, fingerprint string) (*domain.Alert, error)
	CreateSilence(ctx context.Context, silence *domain.Silence) (string, error)
	CreateSilenceWithRecord(ctx context.Context, silence *domain.Silence, userID uuid.UUID, ipAddress, userAgent string) (*models.AlertOperation, string, error)
	GetSilences(ctx context.Context) ([]*domain.Silence, error)
	DeleteSilence(ctx context.Context, silenceID string) error
	DeleteSilenceWithRecord(ctx context.Context, silenceID string, userID uuid.UUID, ipAddress, userAgent string) (*models.AlertOperation, error)
	HealthCheck(ctx context.Context) error
	GetAlertmanagerStatus(ctx context.Context) (interface{}, error)
	GetAlertRecords(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error)
	GetAlertRecordStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error)
	GetOperationHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AlertOperation, int64, error)
	GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	GetAlertFeedback(ctx context.Context, fingerprint string) (*AlertFeedback, error)
	SyncAlertFromWebhook(ctx context.Context, fingerprint string, labels, annotations map[string]string, status models.AlertStatus, startsAt time.Time, endsAt *time.Time, generatorURL string) error
	Start()
	Stop()
}

type AlertService struct {
	amClient     infrastructure.AlertmanagerClientInterface
	opRepo       storage.AlertOperationRepositoryInterface
	noiseRepo    storage.AlertNoiseRepositoryInterface
	notifyRepo   storage.AlertNotificationRepositoryInterface
	recordRepo   storage.AlertRecordRepositoryInterface
	cache        *infra.Cache
	queue        infra.QueueInterface
	opManager    *infrastructure.OperationManager
	errorHandler *infrastructure.ErrorHandler
	feedback     *FeedbackService
	recordBuffer infrastructure.AlertRecordBufferInterface
	stopCh       chan struct{}
	running      bool
}

func NewAlertService(
	amClient infrastructure.AlertmanagerClientInterface,
	opRepo storage.AlertOperationRepositoryInterface,
	noiseRepo storage.AlertNoiseRepositoryInterface,
	notifyRepo storage.AlertNotificationRepositoryInterface,
	recordRepo storage.AlertRecordRepositoryInterface,
	cache *infra.Cache,
	queue infra.QueueInterface,
	recordBuffer infrastructure.AlertRecordBufferInterface,
) *AlertService {
	opManager := infrastructure.NewOperationManager(opRepo, noiseRepo)
	compensator := infrastructure.NewCompensator(opRepo)

	analyzer := NewNoiseAnalyzer(noiseRepo, opRepo, notifyRepo, nil)
	feedback := NewFeedbackService(analyzer, notifyRepo, opRepo)

	return &AlertService{
		amClient:     amClient,
		opRepo:       opRepo,
		noiseRepo:    noiseRepo,
		notifyRepo:   notifyRepo,
		recordRepo:   recordRepo,
		cache:        cache,
		queue:        queue,
		opManager:    opManager,
		errorHandler: infrastructure.NewErrorHandler(opManager, compensator),
		feedback:     feedback,
		recordBuffer: recordBuffer,
		stopCh:       make(chan struct{}),
	}
}

func (s *AlertService) Start() {
	if s.running {
		return
	}
	s.running = true

	if s.recordBuffer != nil {
		s.recordBuffer.Start()
	}

	go s.runRetryScheduler()
}

func (s *AlertService) Stop() {
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)

	if s.recordBuffer != nil {
		s.recordBuffer.Stop()
	}
}

func (s *AlertService) runRetryScheduler() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := s.ProcessRetryQueue(ctx); err != nil {
				log.Printf("[RetryScheduler] Failed to process retry queue: %v", err)
			}
			cancel()
		}
	}
}

func (s *AlertService) SendAlert(ctx context.Context, alert *domain.Alert) error {
	if alert == nil || len(alert.Labels) == 0 {
		return ErrInvalidAlert
	}
	return s.amClient.SendAlerts(ctx, []*domain.Alert{alert})
}

func (s *AlertService) SendAlerts(ctx context.Context, alerts []*domain.Alert) error {
	if len(alerts) == 0 {
		return nil
	}
	for _, a := range alerts {
		if a == nil || len(a.Labels) == 0 {
			return fmt.Errorf("%w: all alerts must have labels", ErrInvalidAlert)
		}
	}
	return s.amClient.SendAlerts(ctx, alerts)
}

func (s *AlertService) SendAlertWithRecord(
	ctx context.Context,
	alert *domain.Alert,
	userID uuid.UUID,
	ipAddress, userAgent string,
) (*models.AlertOperation, error) {
	if alert == nil || len(alert.Labels) == 0 {
		return nil, ErrInvalidAlert
	}

	fingerprint := s.calculateFingerprint(alert.Labels)

	key := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSend,
		Fingerprint: fingerprint,
	}

	labels := make(models.Labels)
	for k, v := range alert.Labels {
		labels[k] = v
	}

	requestData, _ := json.Marshal(alert)

	op, err := s.opManager.BeginOperation(ctx, key, labels, string(requestData), ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, infrastructure.ErrDuplicateOperation) {
			return nil, ErrDuplicateOperation
		}
		return nil, fmt.Errorf("failed to begin operation: %w", err)
	}

	if err := s.amClient.SendAlerts(ctx, []*domain.Alert{alert}); err != nil {
		if handleErr := s.errorHandler.HandleError(ctx, op, err); handleErr != nil {
			log.Printf("Failed to handle error: %v", handleErr)
		}
		return op, fmt.Errorf("failed to send alert: %w", err)
	}

	if err := s.opManager.CompleteOperation(ctx, op.ID, ""); err != nil {
		log.Printf("Failed to complete operation: %v", err)
	}

	s.feedback.ProcessAlertEvent(ctx, "firing", alert.Labels)

	s.persistAlertRecord(fingerprint, alert.Labels, alert.Annotations, models.AlertStatusFiring, alert.StartsAt, nil, "api")

	return op, nil
}

func (s *AlertService) GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error) {
	return s.amClient.GetAlerts(ctx, filter)
}

func (s *AlertService) GetAlert(ctx context.Context, fingerprint string) (*domain.Alert, error) {
	filter := domain.AlertFilter{
		Filter: []string{fmt.Sprintf("fingerprint=%s", fingerprint)},
	}
	alerts, err := s.amClient.GetAlerts(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(alerts) == 0 {
		return nil, ErrAlertNotFound
	}
	return alerts[0], nil
}

func (s *AlertService) CreateSilence(ctx context.Context, silence *domain.Silence) (string, error) {
	if silence == nil || len(silence.Matchers) == 0 {
		return "", ErrInvalidSilence
	}
	if silence.StartsAt.IsZero() || silence.EndsAt.IsZero() {
		return "", fmt.Errorf("%w: starts_at and ends_at are required", ErrInvalidSilence)
	}
	if silence.EndsAt.Before(silence.StartsAt) {
		return "", fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidSilence)
	}
	return s.amClient.CreateSilence(ctx, silence)
}

func (s *AlertService) CreateSilenceWithRecord(
	ctx context.Context,
	silence *domain.Silence,
	userID uuid.UUID,
	ipAddress, userAgent string,
) (*models.AlertOperation, string, error) {
	if silence == nil || len(silence.Matchers) == 0 {
		return nil, "", ErrInvalidSilence
	}

	fingerprint := s.calculateFingerprintFromMatchers(silence.Matchers)

	key := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSilence,
		Fingerprint: fingerprint,
	}

	labels := s.matchersToLabels(silence.Matchers)
	requestData, _ := json.Marshal(silence)

	op, err := s.opManager.BeginOperation(ctx, key, labels, string(requestData), ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, infrastructure.ErrDuplicateOperation) {
			return nil, "", ErrDuplicateOperation
		}
		return nil, "", fmt.Errorf("failed to begin operation: %w", err)
	}

	silenceID, err := s.amClient.CreateSilence(ctx, silence)
	if err != nil {
		if handleErr := s.errorHandler.HandleError(ctx, op, err); handleErr != nil {
			log.Printf("Failed to handle error: %v", handleErr)
		}
		return op, "", fmt.Errorf("failed to create silence: %w", err)
	}

	if err := s.opManager.CompleteOperation(ctx, op.ID, silenceID); err != nil {
		log.Printf("Failed to complete operation: %v", err)
	}

	return op, silenceID, nil
}

func (s *AlertService) GetSilences(ctx context.Context) ([]*domain.Silence, error) {
	return s.amClient.GetSilences(ctx)
}

func (s *AlertService) DeleteSilence(ctx context.Context, silenceID string) error {
	if silenceID == "" {
		return fmt.Errorf("%w: silence_id is required", ErrInvalidSilence)
	}
	return s.amClient.DeleteSilence(ctx, silenceID)
}

func (s *AlertService) DeleteSilenceWithRecord(
	ctx context.Context,
	silenceID string,
	userID uuid.UUID,
	ipAddress, userAgent string,
) (*models.AlertOperation, error) {
	if silenceID == "" {
		return nil, ErrInvalidSilence
	}

	key := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpUnsilence,
		Fingerprint: silenceID,
	}

	requestData := fmt.Sprintf(`{"silence_id":"%s"}`, silenceID)

	op, err := s.opManager.BeginOperation(ctx, key, nil, requestData, ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, infrastructure.ErrDuplicateOperation) {
			return nil, ErrDuplicateOperation
		}
		return nil, fmt.Errorf("failed to begin operation: %w", err)
	}

	if err := s.amClient.DeleteSilence(ctx, silenceID); err != nil {
		if handleErr := s.errorHandler.HandleError(ctx, op, err); handleErr != nil {
			log.Printf("Failed to handle error: %v", handleErr)
		}
		return op, fmt.Errorf("failed to delete silence: %w", err)
	}

	if err := s.opManager.CompleteOperation(ctx, op.ID, ""); err != nil {
		log.Printf("Failed to complete operation: %v", err)
	}

	return op, nil
}

func (s *AlertService) HealthCheck(ctx context.Context) error {
	return s.amClient.HealthCheck(ctx)
}

func (s *AlertService) GetAlertmanagerStatus(ctx context.Context) (interface{}, error) {
	return s.amClient.GetStatus(ctx)
}

func (s *AlertService) GetAlertRecords(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error) {
	return s.recordRepo.List(ctx, filter)
}

func (s *AlertService) GetAlertRecordStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error) {
	return s.recordRepo.GetStats(ctx, from, to)
}

func (s *AlertService) GetOperationHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AlertOperation, int64, error) {
	return s.opRepo.ListByUser(ctx, userID, limit, offset)
}

func (s *AlertService) GetAlertFeedback(ctx context.Context, fingerprint string) (*AlertFeedback, error) {
	return s.feedback.GetAlertFeedback(ctx, fingerprint)
}

func (s *AlertService) GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	cacheKey := fmt.Sprintf("noisy_alerts:%d", limit)
	if data, err := s.cache.Get(ctx, cacheKey); err == nil {
		var records []*models.AlertNoiseRecord
		if json.Unmarshal(data, &records) == nil {
			return records, nil
		}
	}

	records, err := s.feedback.analyzer.GetNoisyAlerts(ctx, limit)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(records); err == nil {
		_ = s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
	}
	return records, nil
}

func (s *AlertService) GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	return s.feedback.analyzer.GetHighRiskAlerts(ctx, limit)
}

func (s *AlertService) EnqueueNotification(ctx context.Context, fingerprint, notifyType, recipient string) error {
	_, err := s.queue.Enqueue(ctx, "alert:notify", map[string]string{
		"fingerprint":       fingerprint,
		"notification_type": notifyType,
		"recipient":         recipient,
	})
	return err
}

func (s *AlertService) ProcessRetryQueue(ctx context.Context) error {
	ops, err := s.opManager.GetPendingOperations(ctx, 100)
	if err != nil {
		return err
	}

	for _, op := range ops {
		if !s.errorHandler.ShouldRetry(op) {
			continue
		}
		delay := s.errorHandler.GetRetryDelay(op)
		s.enqueueRetryOperation(op, delay)
	}

	return nil
}

func (s *AlertService) persistAlertRecord(
	fingerprint string,
	labels, annotations map[string]string,
	status models.AlertStatus,
	startsAt time.Time,
	endsAt *time.Time,
	source string,
) {
	if s.recordBuffer != nil {
		s.recordBuffer.AddFromAlert(fingerprint, labels, annotations, status, startsAt, endsAt, source)
	}
}

func (s *AlertService) PersistAlertsFromAlertmanager(ctx context.Context, alerts []*domain.Alert) error {
	for _, alert := range alerts {
		if alert == nil || len(alert.Labels) == 0 {
			continue
		}
		fingerprint := s.calculateFingerprint(alert.Labels)
		status := models.AlertStatusFiring
		if alert.Status == domain.AlertStatusResolved {
			status = models.AlertStatusResolved
		}
		s.persistAlertRecord(fingerprint, alert.Labels, alert.Annotations, status, alert.StartsAt, alert.EndsAt, "alertmanager")
	}
	return nil
}

func (s *AlertService) SyncAlertFromWebhook(ctx context.Context, fingerprint string, labels, annotations map[string]string, status models.AlertStatus, startsAt time.Time, endsAt *time.Time, generatorURL string) error {
	if fingerprint == "" && len(labels) > 0 {
		fingerprint = s.calculateFingerprint(labels)
	}

	if fingerprint == "" {
		return ErrInvalidAlert
	}

	s.persistAlertRecord(fingerprint, labels, annotations, status, startsAt, endsAt, "webhook")

	if s.noiseRepo != nil {
		alertName := labels["alertname"]

		noiseRecord := &models.AlertNoiseRecord{
			AlertFingerprint: fingerprint,
			AlertName:        alertName,
			AlertLabels:      labels,
			FireCount:        1,
		}

		_ = s.noiseRepo.Upsert(ctx, noiseRecord)
	}

	log.Printf("[Webhook] Synced alert: fingerprint=%s, status=%s, labels=%v", fingerprint, status, labels)

	return nil
}

func (s *AlertService) enqueueRetryOperation(op *models.AlertOperation, delay time.Duration) {
	var taskType string
	switch op.OperationType {
	case models.AlertOpSend:
		taskType = "alert:send"
	case models.AlertOpSilence:
		taskType = "silence:create"
	case models.AlertOpUnsilence:
		taskType = "silence:delete"
	default:
		return
	}
	_, _ = s.queue.EnqueueWithDelay(context.Background(), taskType, map[string]any{
		"operation_id": op.ID.String(),
		"request_data": op.RequestData,
	}, delay)
}

func (s *AlertService) calculateFingerprint(labels map[string]string) string {
	return s.feedback.analyzer.CalculateFingerprint(labels)
}

func (s *AlertService) calculateFingerprintFromMatchers(matchers []domain.Matcher) string {
	labels := make(map[string]string)
	for _, m := range matchers {
		labels[m.Name] = m.Value
	}
	return s.calculateFingerprint(labels)
}

func (s *AlertService) matchersToLabels(matchers []domain.Matcher) models.Labels {
	labels := make(models.Labels)
	for _, m := range matchers {
		labels[m.Name] = m.Value
	}
	return labels
}
