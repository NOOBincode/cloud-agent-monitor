package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/pkg/config"

	"github.com/go-openapi/strfmt"
	alertclient "github.com/prometheus/alertmanager/api/v2/client"
	alertops "github.com/prometheus/alertmanager/api/v2/client/alert"
	silenceclient "github.com/prometheus/alertmanager/api/v2/client/silence"
	"github.com/prometheus/alertmanager/api/v2/models"
)

type AlertmanagerClientInterface interface {
	SendAlerts(ctx context.Context, alerts []*domain.Alert) error
	GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error)
	CreateSilence(ctx context.Context, silence *domain.Silence) (string, error)
	GetSilences(ctx context.Context) ([]*domain.Silence, error)
	DeleteSilence(ctx context.Context, silenceID string) error
	HealthCheck(ctx context.Context) error
	GetStatus(ctx context.Context) (*models.AlertmanagerStatus, error)
}

type AlertmanagerClient struct {
	client     *alertclient.AlertmanagerAPI
	cfg        config.AlertmanagerConfig
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

func NewAlertmanagerClient(cfg config.AlertmanagerConfig) *AlertmanagerClient {
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	transport := alertclient.DefaultTransportConfig().
		WithHost(cfg.URL).
		WithSchemes([]string{"http"})

	if len(cfg.URL) > 4 && cfg.URL[:5] == "https" {
		transport = transport.WithSchemes([]string{"https"})
	}

	client := alertclient.NewHTTPClientWithConfig(nil, transport)

	return &AlertmanagerClient{
		client:     client,
		cfg:        cfg,
		httpClient: httpClient,
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

func (c *AlertmanagerClient) withRetry(fn func() error) error {
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		if err := fn(); err != nil {
			lastErr = err
			if isRetryableError(err) {
				time.Sleep(c.retryDelay * time.Duration(i+1))
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	return true
}

func (c *AlertmanagerClient) SendAlerts(ctx context.Context, alerts []*domain.Alert) error {
	var postAlerts models.PostableAlerts
	for _, a := range alerts {
		postAlert := &models.PostableAlert{
			Alert: models.Alert{
				Labels: toLabelSet(a.Labels),
			},
			Annotations: toLabelSet(a.Annotations),
			StartsAt:    strfmt.DateTime(a.StartsAt),
		}
		if a.EndsAt != nil {
			postAlert.EndsAt = strfmt.DateTime(*a.EndsAt)
		}
		if a.GeneratorURL != "" {
			postAlert.GeneratorURL = strfmt.URI(a.GeneratorURL)
		}
		postAlerts = append(postAlerts, postAlert)
	}

	return c.withRetry(func() error {
		params := alertops.NewPostAlertsParams().
			WithContext(ctx).
			WithAlerts(postAlerts)

		_, err := c.client.Alert.PostAlerts(params)
		if err != nil {
			return fmt.Errorf("send alerts to alertmanager: %w", err)
		}
		return nil
	})
}

func (c *AlertmanagerClient) GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error) {
	params := alertops.NewGetAlertsParams().
		WithContext(ctx)

	if filter.Active != nil {
		params.SetActive(filter.Active)
	}
	if filter.Silenced != nil {
		params.SetSilenced(filter.Silenced)
	}
	if filter.Inhibited != nil {
		params.SetInhibited(filter.Inhibited)
	}
	if filter.Receiver != "" {
		params.SetReceiver(&filter.Receiver)
	}
	if len(filter.Filter) > 0 {
		params.SetFilter(filter.Filter)
	}

	resp, err := c.client.Alert.GetAlerts(params)
	if err != nil {
		return nil, fmt.Errorf("get alerts from alertmanager: %w", err)
	}

	var alerts []*domain.Alert
	for _, a := range resp.Payload {
		alert := &domain.Alert{
			Labels:      fromLabelSet(a.Labels),
			Annotations: fromLabelSet(a.Annotations),
			Status:      domain.AlertStatus(*a.Status.State),
		}
		if a.StartsAt != nil {
			alert.StartsAt = time.Time(*a.StartsAt)
		}
		if a.EndsAt != nil {
			endsAt := time.Time(*a.EndsAt)
			alert.EndsAt = &endsAt
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (c *AlertmanagerClient) CreateSilence(ctx context.Context, silence *domain.Silence) (string, error) {
	var matchers models.Matchers
	for _, m := range silence.Matchers {
		matchers = append(matchers, &models.Matcher{
			Name:    &m.Name,
			Value:   &m.Value,
			IsRegex: &m.IsRegex,
		})
	}

	startsAt := strfmt.DateTime(silence.StartsAt)
	endsAt := strfmt.DateTime(silence.EndsAt)
	createdBy := silence.CreatedBy
	comment := silence.Comment

	postableSilence := &models.PostableSilence{
		Silence: models.Silence{
			Matchers:  matchers,
			StartsAt:  &startsAt,
			EndsAt:    &endsAt,
			CreatedBy: &createdBy,
			Comment:   &comment,
		},
	}

	var silenceID string
	err := c.withRetry(func() error {
		params := silenceclient.NewPostSilencesParams().
			WithContext(ctx).
			WithSilence(postableSilence)

		resp, err := c.client.Silence.PostSilences(params)
		if err != nil {
			return fmt.Errorf("create silence in alertmanager: %w", err)
		}
		silenceID = resp.Payload.SilenceID
		return nil
	})

	return silenceID, err
}

func (c *AlertmanagerClient) GetSilences(ctx context.Context) ([]*domain.Silence, error) {
	params := silenceclient.NewGetSilencesParams().WithContext(ctx)

	resp, err := c.client.Silence.GetSilences(params)
	if err != nil {
		return nil, fmt.Errorf("get silences from alertmanager: %w", err)
	}

	var silences []*domain.Silence
	for _, s := range resp.Payload {
		silence := &domain.Silence{
			Status: domain.SilenceStatus(*s.Status.State),
		}

		if s.StartsAt != nil {
			silence.StartsAt = time.Time(*s.StartsAt)
		}
		if s.EndsAt != nil {
			silence.EndsAt = time.Time(*s.EndsAt)
		}
		if s.CreatedBy != nil {
			silence.CreatedBy = *s.CreatedBy
		}
		if s.Comment != nil {
			silence.Comment = *s.Comment
		}

		for _, m := range s.Matchers {
			silence.Matchers = append(silence.Matchers, domain.Matcher{
				Name:    *m.Name,
				Value:   *m.Value,
				IsRegex: *m.IsRegex,
			})
		}

		silences = append(silences, silence)
	}

	return silences, nil
}

func (c *AlertmanagerClient) DeleteSilence(ctx context.Context, silenceID string) error {
	return c.withRetry(func() error {
		params := silenceclient.NewDeleteSilenceParams().
			WithContext(ctx).
			WithSilenceID(strfmt.UUID(silenceID))

		_, err := c.client.Silence.DeleteSilence(params)
		if err != nil {
			return fmt.Errorf("delete silence from alertmanager: %w", err)
		}
		return nil
	})
}

func (c *AlertmanagerClient) HealthCheck(ctx context.Context) error {
	resp, err := c.client.General.GetStatus(nil)
	if err != nil {
		return fmt.Errorf("alertmanager health check failed: %w", err)
	}
	if resp.Payload == nil {
		return fmt.Errorf("alertmanager returned empty status")
	}
	return nil
}

func (c *AlertmanagerClient) GetStatus(ctx context.Context) (*models.AlertmanagerStatus, error) {
	statusResp, err := c.client.General.GetStatus(nil)
	if err != nil {
		return nil, fmt.Errorf("get alertmanager status: %w", err)
	}

	return statusResp.Payload, nil
}

func toLabelSet(m map[string]string) models.LabelSet {
	if m == nil {
		return nil
	}
	ls := make(models.LabelSet)
	for k, v := range m {
		ls[k] = v
	}
	return ls
}

func fromLabelSet(ls models.LabelSet) map[string]string {
	if ls == nil {
		return nil
	}
	m := make(map[string]string)
	for k, v := range ls {
		m[k] = v
	}
	return m
}
