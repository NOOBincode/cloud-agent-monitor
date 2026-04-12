package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud-agent-monitor/internal/promclient"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/config"
	"cloud-agent-monitor/pkg/logger"

	"github.com/alexliesenfeld/health"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type HealthCheckService struct {
	checker health.Checker
	handler http.Handler
	repo    storage.ServiceRepositoryInterface
	promCli *promclient.Client
	cfg     config.HealthCheckConfig
}

func NewHealthCheckService(
	repo storage.ServiceRepositoryInterface,
	promClient *promclient.Client,
	cfg config.HealthCheckConfig,
) *HealthCheckService {
	return &HealthCheckService{
		repo:    repo,
		promCli: promClient,
		cfg:     cfg,
	}
}

func (s *HealthCheckService) Start() {
	if !s.cfg.Enabled {
		logger.Info("health check service is disabled")
		return
	}

	checks := []health.CheckerOption{
		health.WithCacheDuration(s.cfg.CheckInterval),
		health.WithTimeout(30 * time.Second),
	}

	services, err := s.repo.List(context.Background(), storage.ServiceFilter{PageSize: 1000})
	if err != nil {
		logger.Error("failed to list services for health check", zap.Error(err))
		return
	}

	for i := range services.Data {
		svc := &services.Data[i]
		checks = append(checks, health.WithPeriodicCheck(
			s.cfg.CheckInterval,
			3*time.Second,
			health.Check{
				Name: fmt.Sprintf("service:%s", svc.Name),
				Check: func(ctx context.Context) error {
					return s.checkServiceHealth(ctx, svc)
				},
			},
		))
	}

	s.checker = health.NewChecker(checks...)
	s.handler = health.NewHandler(s.checker)

	logger.Info("health check service started",
		zap.Duration("interval", s.cfg.CheckInterval),
		zap.Int("services", len(services.Data)),
	)
}

func (s *HealthCheckService) Stop() {
	logger.Info("health check service stopped")
}

func (s *HealthCheckService) Handler() http.Handler {
	return s.handler
}

func (s *HealthCheckService) Checker() health.Checker {
	return s.checker
}

func (s *HealthCheckService) checkServiceHealth(ctx context.Context, svc *models.Service) error {
	upQuery := fmt.Sprintf(`up{job="%s"}`, svc.Name)
	upValue, err := s.promCli.QueryScalar(ctx, upQuery)
	if err != nil {
		s.updateServiceStatus(ctx, svc, models.HealthStatusUnknown, err.Error())
		return fmt.Errorf("query up metric: %w", err)
	}

	if upValue != 1 {
		s.updateServiceStatus(ctx, svc, models.HealthStatusUnhealthy, "service is down")
		return fmt.Errorf("service %s is down", svc.Name)
	}

	errorRateQuery := fmt.Sprintf(
		`sum(rate(http_requests_total{job="%s",status=~"5.."}[5m])) / sum(rate(http_requests_total{job="%s"}[5m]))`,
		svc.Name, svc.Name,
	)
	if errorRate, err := s.promCli.QueryScalar(ctx, errorRateQuery); err == nil && errorRate > 0.05 {
		details := fmt.Sprintf("high error rate: %.2f%%", errorRate*100)
		s.updateServiceStatus(ctx, svc, models.HealthStatusUnhealthy, details)
		return fmt.Errorf("%s", details)
	}

	latencyQuery := fmt.Sprintf(
		`histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="%s"}[5m])) by (le))`,
		svc.Name,
	)
	if latency, err := s.promCli.QueryScalar(ctx, latencyQuery); err == nil && latency > 1.0 {
		details := fmt.Sprintf("high latency p99: %.2fs", latency)
		s.updateServiceStatus(ctx, svc, models.HealthStatusUnhealthy, details)
		return fmt.Errorf("%s", details)
	}

	s.updateServiceStatus(ctx, svc, models.HealthStatusHealthy, "service is healthy")
	return nil
}

func (s *HealthCheckService) updateServiceStatus(ctx context.Context, svc *models.Service, status models.HealthStatus, details string) {
	now := time.Now()
	svc.HealthStatus = status
	svc.LastHealthCheckAt = &now

	if len(details) > 500 {
		details = details[:500]
	}
	svc.HealthCheckDetails = details

	if err := s.repo.Update(ctx, svc); err != nil {
		logger.Error("failed to update service health status",
			zap.String("service", svc.Name),
			zap.Error(err),
		)
	}
}

func (s *HealthCheckService) CheckServiceNow(ctx context.Context, serviceID uuid.UUID) (*models.ServiceHealthMetric, error) {
	svc, err := s.repo.GetByID(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	metric := &models.ServiceHealthMetric{
		ServiceName: svc.Name,
		Status:      models.HealthStatusUnknown,
		Metrics:     make(map[string]float64),
		LastChecked: time.Now(),
	}

	upQuery := fmt.Sprintf(`up{job="%s"}`, svc.Name)
	upValue, err := s.promCli.QueryScalar(ctx, upQuery)
	if err != nil {
		metric.Details = err.Error()
		s.updateServiceStatus(ctx, svc, models.HealthStatusUnknown, err.Error())
		return metric, nil
	}

	metric.Metrics["up"] = upValue
	if upValue == 1 {
		metric.Status = models.HealthStatusHealthy
		metric.Details = "service is up"

		if errorRate, err := s.promCli.QueryScalar(ctx, fmt.Sprintf(
			`sum(rate(http_requests_total{job="%s",status=~"5.."}[5m])) / sum(rate(http_requests_total{job="%s"}[5m]))`,
			svc.Name, svc.Name,
		)); err == nil {
			metric.Metrics["error_rate"] = errorRate
			if errorRate > 0.05 {
				metric.Status = models.HealthStatusUnhealthy
				metric.Details = fmt.Sprintf("high error rate: %.2f%%", errorRate*100)
			}
		}

		if latency, err := s.promCli.QueryScalar(ctx, fmt.Sprintf(
			`histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{job="%s"}[5m])) by (le))`,
			svc.Name,
		)); err == nil {
			metric.Metrics["latency_p99"] = latency
			if latency > 1.0 && metric.Status == models.HealthStatusHealthy {
				metric.Status = models.HealthStatusUnhealthy
				metric.Details = fmt.Sprintf("high latency p99: %.2fs", latency)
			}
		}
	} else {
		metric.Status = models.HealthStatusUnhealthy
		metric.Details = "service is down"
	}

	s.updateServiceStatus(ctx, svc, metric.Status, metric.Details)
	return metric, nil
}

func (s *HealthCheckService) GetServiceHealth(ctx context.Context, serviceID uuid.UUID) (*ServiceHealthDetail, error) {
	svc, err := s.repo.GetByID(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	detail := &ServiceHealthDetail{
		ServiceID:         svc.ID,
		ServiceName:       svc.Name,
		Status:            svc.HealthStatus,
		LastHealthCheckAt: svc.LastHealthCheckAt,
		Details:           svc.HealthCheckDetails,
	}

	metricsQuery := fmt.Sprintf(`up{job="%s"}`, svc.Name)
	if results, err := s.promCli.QueryInstantVector(ctx, metricsQuery); err == nil && len(results) > 0 {
		detail.CurrentMetrics = make(map[string]interface{})
		for k, v := range results[0].Metric {
			detail.CurrentMetrics[string(k)] = string(v)
		}
		detail.CurrentMetrics["up"] = results[0].Value
	}

	return detail, nil
}

type ServiceHealthDetail struct {
	ServiceID         uuid.UUID              `json:"service_id"`
	ServiceName       string                 `json:"service_name"`
	Status            models.HealthStatus    `json:"status"`
	LastHealthCheckAt *time.Time             `json:"last_health_check_at,omitempty"`
	Details           string                 `json:"details,omitempty"`
	CurrentMetrics    map[string]interface{} `json:"current_metrics,omitempty"`
}

func (d *ServiceHealthDetail) MarshalJSON() ([]byte, error) {
	type Alias ServiceHealthDetail
	return json.Marshal(&struct {
		*Alias
		LastHealthCheckAt *string `json:"last_health_check_at,omitempty"`
	}{
		Alias:             (*Alias)(d),
		LastHealthCheckAt: formatTimePtr(d.LastHealthCheckAt),
	})
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(time.RFC3339)
	return &formatted
}
