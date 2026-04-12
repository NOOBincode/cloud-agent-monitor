package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sort"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/config"

	"github.com/google/uuid"
)

type NoiseAnalyzer struct {
	noiseRepo  storage.AlertNoiseRepositoryInterface
	opRepo     storage.AlertOperationRepositoryInterface
	notifyRepo storage.AlertNotificationRepositoryInterface
	cfg        *config.Config
}

func NewNoiseAnalyzer(
	noiseRepo storage.AlertNoiseRepositoryInterface,
	opRepo storage.AlertOperationRepositoryInterface,
	notifyRepo storage.AlertNotificationRepositoryInterface,
	cfg *config.Config,
) *NoiseAnalyzer {
	return &NoiseAnalyzer{
		noiseRepo:  noiseRepo,
		opRepo:     opRepo,
		notifyRepo: notifyRepo,
		cfg:        cfg,
	}
}

func (a *NoiseAnalyzer) CalculateFingerprint(labels map[string]string) string {
	h := sha256.New()
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k + "=" + labels[k] + ";"))
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (a *NoiseAnalyzer) RecordAlertFired(ctx context.Context, labels map[string]string) error {
	fingerprint := a.CalculateFingerprint(labels)
	alertName := labels["alertname"]
	if alertName == "" {
		alertName = "unknown"
	}

	record, err := a.noiseRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		record = &models.AlertNoiseRecord{
			ID:               uuid.New(),
			AlertFingerprint: fingerprint,
			AlertName:        alertName,
			AlertLabels:      labels,
			FireCount:        0,
			ResolveCount:     0,
			NoiseScore:       0,
			IsNoisy:          false,
			IsHighRisk:       false,
		}
	}

	record.FireCount++
	record.LastFiredAt = time.Now()

	a.updateNoiseScore(record)
	a.checkHighRisk(record, labels)

	return a.noiseRepo.Upsert(ctx, record)
}

func (a *NoiseAnalyzer) RecordAlertResolved(ctx context.Context, labels map[string]string) error {
	fingerprint := a.CalculateFingerprint(labels)

	record, err := a.noiseRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil
	}

	record.ResolveCount++
	now := time.Now()
	record.LastResolvedAt = &now

	a.updateNoiseScore(record)

	return a.noiseRepo.Upsert(ctx, record)
}

func (a *NoiseAnalyzer) updateNoiseScore(record *models.AlertNoiseRecord) {
	if record.FireCount == 0 {
		record.NoiseScore = 0
		return
	}

	fireToResolveRatio := float64(record.FireCount)
	if record.ResolveCount > 0 {
		fireToResolveRatio = float64(record.FireCount) / float64(record.ResolveCount)
	}

	timeWindow := time.Since(record.CreatedAt).Hours()
	if timeWindow == 0 {
		timeWindow = 1
	}
	fireFrequency := float64(record.FireCount) / timeWindow

	record.NoiseScore = (fireToResolveRatio * 0.4) + (fireFrequency * 0.6)

	record.IsNoisy = record.NoiseScore > 5.0 && record.FireCount > 10
	record.SilenceSuggested = record.IsNoisy && record.NoiseScore > 8.0
}

func (a *NoiseAnalyzer) checkHighRisk(record *models.AlertNoiseRecord, labels map[string]string) {
	severity := labels["severity"]
	if severity == "critical" || severity == "high" {
		record.IsHighRisk = true
	}
}

func (a *NoiseAnalyzer) GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	return a.noiseRepo.GetNoisyAlerts(ctx, limit)
}

func (a *NoiseAnalyzer) GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	return a.noiseRepo.GetHighRiskAlerts(ctx, limit)
}

func (a *NoiseAnalyzer) ShouldSuggestSilence(ctx context.Context, fingerprint string) (bool, string) {
	record, err := a.noiseRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return false, ""
	}

	if record.SilenceSuggested {
		return true, "This alert has been identified as noisy with high fire frequency. Consider silencing it."
	}
	return false, ""
}

type FeedbackService struct {
	analyzer   *NoiseAnalyzer
	notifyRepo storage.AlertNotificationRepositoryInterface
	opRepo     storage.AlertOperationRepositoryInterface
}

func NewFeedbackService(
	analyzer *NoiseAnalyzer,
	notifyRepo storage.AlertNotificationRepositoryInterface,
	opRepo storage.AlertOperationRepositoryInterface,
) *FeedbackService {
	return &FeedbackService{
		analyzer:   analyzer,
		notifyRepo: notifyRepo,
		opRepo:     opRepo,
	}
}

func (s *FeedbackService) ProcessAlertEvent(ctx context.Context, eventType string, labels map[string]string) {
	switch eventType {
	case "firing":
		if err := s.analyzer.RecordAlertFired(ctx, labels); err != nil {
			log.Printf("Failed to record alert fired: %v", err)
		}
		s.checkAndNotifyHighRisk(ctx, labels)
	case "resolved":
		if err := s.analyzer.RecordAlertResolved(ctx, labels); err != nil {
			log.Printf("Failed to record alert resolved: %v", err)
		}
	}
}

func (s *FeedbackService) checkAndNotifyHighRisk(ctx context.Context, labels map[string]string) {
	severity := labels["severity"]
	if severity != "critical" && severity != "high" {
		return
	}

	fingerprint := s.analyzer.CalculateFingerprint(labels)

	notification := &models.AlertNotification{
		ID:               uuid.New(),
		AlertFingerprint: fingerprint,
		NotificationType: "high_risk_alert",
		Recipient:        "oncall",
		Status:           "pending",
	}

	if err := s.notifyRepo.Create(ctx, notification); err != nil {
		log.Printf("Failed to create high risk notification: %v", err)
	}
}

func (s *FeedbackService) GetAlertFeedback(ctx context.Context, fingerprint string) (*AlertFeedback, error) {
	record, err := s.analyzer.noiseRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, err
	}

	ops, err := s.opRepo.GetByFingerprint(ctx, fingerprint, 10)
	if err != nil {
		ops = []*models.AlertOperation{}
	}

	return &AlertFeedback{
		Fingerprint:      fingerprint,
		AlertName:        record.AlertName,
		FireCount:        record.FireCount,
		ResolveCount:     record.ResolveCount,
		NoiseScore:       record.NoiseScore,
		IsNoisy:          record.IsNoisy,
		IsHighRisk:       record.IsHighRisk,
		SilenceSuggested: record.SilenceSuggested,
		RecentOperations: ops,
	}, nil
}

type AlertFeedback struct {
	Fingerprint      string                   `json:"fingerprint"`
	AlertName        string                   `json:"alert_name"`
	FireCount        int                      `json:"fire_count"`
	ResolveCount     int                      `json:"resolve_count"`
	NoiseScore       float64                  `json:"noise_score"`
	IsNoisy          bool                     `json:"is_noisy"`
	IsHighRisk       bool                     `json:"is_high_risk"`
	SilenceSuggested bool                     `json:"silence_suggested"`
	RecentOperations []*models.AlertOperation `json:"recent_operations"`
}
