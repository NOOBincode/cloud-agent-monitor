package infrastructure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/pkg/config"

	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAlertmanagerClient(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.AlertmanagerConfig
	}{
		{
			name: "http config",
			cfg: config.AlertmanagerConfig{
				URL:     "localhost:9093",
				Timeout: 30 * time.Second,
			},
		},
		{
			name: "https config",
			cfg: config.AlertmanagerConfig{
				URL:     "https://alertmanager.example.com",
				Timeout: 30 * time.Second,
			},
		},
		{
			name: "with port",
			cfg: config.AlertmanagerConfig{
				URL:     "alertmanager.local:9093",
				Timeout: 10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAlertmanagerClient(tt.cfg)
			assert.NotNil(t, client)
			assert.NotNil(t, client.client)
			assert.Equal(t, tt.cfg, client.cfg)
			assert.NotNil(t, client.httpClient)
		})
	}
}

func TestToLabelSet(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]string
		expectedLen int
		checkValues bool
	}{
		{
			name:        "nil map",
			input:       nil,
			expectedLen: 0,
			checkValues: false,
		},
		{
			name:        "empty map",
			input:       map[string]string{},
			expectedLen: 0,
			checkValues: false,
		},
		{
			name:        "with values",
			input:       map[string]string{"alertname": "TestAlert", "severity": "critical"},
			expectedLen: 2,
			checkValues: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLabelSet(tt.input)
			assert.Len(t, result, tt.expectedLen)
			if tt.checkValues {
				assert.Equal(t, "TestAlert", result["alertname"])
				assert.Equal(t, "critical", result["severity"])
			}
		})
	}
}

func TestFromLabelSet(t *testing.T) {
	tests := []struct {
		name        string
		input       models.LabelSet
		expectedLen int
		checkValues bool
	}{
		{
			name:        "nil map",
			input:       nil,
			expectedLen: 0,
			checkValues: false,
		},
		{
			name:        "empty map",
			input:       models.LabelSet{},
			expectedLen: 0,
			checkValues: false,
		},
		{
			name:        "with values",
			input:       models.LabelSet{"alertname": "TestAlert", "severity": "critical"},
			expectedLen: 2,
			checkValues: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fromLabelSet(tt.input)
			assert.Len(t, result, tt.expectedLen)
			if tt.checkValues {
				assert.Equal(t, "TestAlert", result["alertname"])
				assert.Equal(t, "critical", result["severity"])
			}
		})
	}
}

func TestAlertmanagerClient_Interface(t *testing.T) {
	var _ AlertmanagerClientInterface = (*AlertmanagerClient)(nil)
}

func TestAlertmanagerClient_SendAlerts_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/alerts", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	now := time.Now()
	alerts := []*domain.Alert{
		{
			Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
			Annotations: map[string]string{"description": "Test alert"},
			StartsAt:    now,
			Status:      domain.AlertStatusFiring,
		},
		{
			Labels:      map[string]string{"alertname": "AnotherAlert"},
			Annotations: map[string]string{},
			StartsAt:    now,
			EndsAt:      ptrTime(now.Add(time.Hour)),
			Status:      domain.AlertStatusResolved,
		},
	}

	err := client.SendAlerts(context.Background(), alerts)
	assert.NoError(t, err)
}

func TestAlertmanagerClient_SendAlerts_WithGeneratorURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/alerts", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	alerts := []*domain.Alert{
		{
			Labels:       map[string]string{"alertname": "TestAlert"},
			StartsAt:     time.Now(),
			GeneratorURL: "http://prometheus/graph",
		},
	}

	err := client.SendAlerts(context.Background(), alerts)
	assert.NoError(t, err)
}

func TestAlertmanagerClient_SendAlerts_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.SendAlerts(context.Background(), []*domain.Alert{})
	assert.NoError(t, err)
}

func TestAlertmanagerClient_SendAlerts_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	alerts := []*domain.Alert{
		{Labels: map[string]string{"alertname": "TestAlert"}, StartsAt: time.Now()},
	}

	err := client.SendAlerts(context.Background(), alerts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send alerts to alertmanager")
}

func TestAlertmanagerClient_GetAlerts_Success(t *testing.T) {
	now := time.Now()
	startsAt := strfmt.DateTime(now)
	endsAt := strfmt.DateTime(now.Add(time.Hour))
	fingerprint := "fp123"
	state := "active"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/alerts", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		alerts := models.GettableAlerts{
			{
				Alert: models.Alert{
					Labels: models.LabelSet{"alertname": "TestAlert", "severity": "critical"},
				},
				Annotations: models.LabelSet{"description": "Test"},
				StartsAt:    &startsAt,
				EndsAt:      &endsAt,
				Fingerprint: &fingerprint,
				Status: &models.AlertStatus{
					State: &state,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	alerts, err := client.GetAlerts(context.Background(), domain.AlertFilter{})
	assert.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "TestAlert", alerts[0].Labels["alertname"])
	assert.Equal(t, domain.AlertStatus("active"), alerts[0].Status)
}

func TestAlertmanagerClient_GetAlerts_WithFilter(t *testing.T) {
	now := time.Now()
	startsAt := strfmt.DateTime(now)
	endsAt := strfmt.DateTime(now.Add(time.Hour))
	fingerprint := "fp123"
	state := "active"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/alerts", r.URL.Path)

		active := r.URL.Query().Get("active")
		silenced := r.URL.Query().Get("silenced")
		receiver := r.URL.Query().Get("receiver")

		assert.Equal(t, "true", active)
		assert.Equal(t, "false", silenced)
		assert.Equal(t, "webhook", receiver)

		alerts := models.GettableAlerts{
			{
				Alert: models.Alert{
					Labels: models.LabelSet{"alertname": "TestAlert"},
				},
				Annotations: models.LabelSet{},
				StartsAt:    &startsAt,
				EndsAt:      &endsAt,
				Fingerprint: &fingerprint,
				Status: &models.AlertStatus{
					State: &state,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	filter := domain.AlertFilter{
		Active:   ptrBool(true),
		Silenced: ptrBool(false),
		Receiver: "webhook",
	}

	alerts, err := client.GetAlerts(context.Background(), filter)
	assert.NoError(t, err)
	assert.NotNil(t, alerts)
	assert.Len(t, alerts, 1)
}

func TestAlertmanagerClient_GetAlerts_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetAlerts(context.Background(), domain.AlertFilter{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get alerts from alertmanager")
}

func TestAlertmanagerClient_CreateSilence_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		resp := map[string]string{
			"silenceID": "12345678-1234-1234-1234-123456789012",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	now := time.Now()
	silence := &domain.Silence{
		Matchers: []domain.Matcher{
			{Name: "alertname", Value: "TestAlert", IsRegex: false},
			{Name: "instance", Value: ".*", IsRegex: true},
		},
		StartsAt:  now,
		EndsAt:    now.Add(2 * time.Hour),
		CreatedBy: "admin@example.com",
		Comment:   "Planned maintenance",
	}

	silenceID, err := client.CreateSilence(context.Background(), silence)
	assert.NoError(t, err)
	assert.NotEmpty(t, silenceID)
}

func TestAlertmanagerClient_CreateSilence_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	silence := &domain.Silence{
		Matchers:  []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		CreatedBy: "user1",
	}

	_, err := client.CreateSilence(context.Background(), silence)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create silence in alertmanager")
}

func TestAlertmanagerClient_GetSilences_Success(t *testing.T) {
	now := time.Now()
	startsAt := strfmt.DateTime(now)
	endsAt := strfmt.DateTime(now.Add(time.Hour))
	silenceID := "silence-123"
	createdBy := "admin@example.com"
	comment := "Test silence"
	name := "alertname"
	value := "TestAlert"
	isRegex := false
	state := "active"
	updatedAt := strfmt.DateTime(now)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/silences", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		silences := models.GettableSilences{
			{
				Silence: models.Silence{
					Matchers: models.Matchers{
						{Name: &name, Value: &value, IsRegex: &isRegex},
					},
					StartsAt:  &startsAt,
					EndsAt:    &endsAt,
					CreatedBy: &createdBy,
					Comment:   &comment,
				},
				ID:        &silenceID,
				UpdatedAt: &updatedAt,
				Status: &models.SilenceStatus{
					State: &state,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(silences)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	silences, err := client.GetSilences(context.Background())
	assert.NoError(t, err)
	assert.Len(t, silences, 1)
	assert.Equal(t, "admin@example.com", silences[0].CreatedBy)
	assert.Equal(t, "Test silence", silences[0].Comment)
	assert.Equal(t, domain.SilenceStatus("active"), silences[0].Status)
	assert.Len(t, silences[0].Matchers, 1)
	assert.Equal(t, "alertname", silences[0].Matchers[0].Name)
}

func TestAlertmanagerClient_GetSilences_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.GettableSilences{})
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	silences, err := client.GetSilences(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, silences)
}

func TestAlertmanagerClient_GetSilences_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetSilences(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get silences from alertmanager")
}

func TestAlertmanagerClient_DeleteSilence_Success(t *testing.T) {
	silenceID := "12345678-1234-1234-1234-123456789012"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v2/silence/" + silenceID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.DeleteSilence(context.Background(), silenceID)
	assert.NoError(t, err)
}

func TestAlertmanagerClient_DeleteSilence_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.DeleteSilence(context.Background(), "invalid-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete silence from alertmanager")
}

func TestAlertmanagerClient_HealthCheck_Success(t *testing.T) {
	clusterStatus := "ready"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/status", r.URL.Path)

		status := &models.AlertmanagerStatus{
			Cluster: &models.ClusterStatus{
				Name:   "test-cluster",
				Status: &clusterStatus,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestAlertmanagerClient_HealthCheck_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestAlertmanagerClient_GetStatus_Success(t *testing.T) {
	clusterStatus := "ready"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/status", r.URL.Path)

		status := &models.AlertmanagerStatus{
			Cluster: &models.ClusterStatus{
				Name:   "test-cluster",
				Status: &clusterStatus,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	status, err := client.GetStatus(context.Background())
	assert.NoError(t, err)
	require.NotNil(t, status)
	assert.NotNil(t, status.Cluster)
	assert.Equal(t, "test-cluster", status.Cluster.Name)
}

func TestAlertmanagerClient_GetStatus_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetStatus(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get alertmanager status")
}

func TestAlertmanagerClient_ConnectionError(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host-that-does-not-exist:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestAlertmanagerClient_HTTPSScheme(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "https://secure-alertmanager.example.com",
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
}

func TestAlertmanagerClient_SendAlerts_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	alerts := []*domain.Alert{
		{
			Labels:   map[string]string{"alertname": "TestAlert"},
			StartsAt: time.Now(),
		},
	}

	err := client.SendAlerts(context.Background(), alerts)
	assert.Error(t, err)
}

func TestAlertmanagerClient_GetAlerts_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetAlerts(context.Background(), domain.AlertFilter{})
	assert.Error(t, err)
}

func TestAlertmanagerClient_CreateSilence_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	now := time.Now()
	silence := &domain.Silence{
		Matchers:  []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
		StartsAt:  now,
		EndsAt:    now.Add(time.Hour),
		CreatedBy: "user1",
	}

	_, err := client.CreateSilence(context.Background(), silence)
	assert.Error(t, err)
}

func TestAlertmanagerClient_GetSilences_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetSilences(context.Background())
	assert.Error(t, err)
}

func TestAlertmanagerClient_DeleteSilence_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.DeleteSilence(context.Background(), "silence-123")
	assert.Error(t, err)
}

func TestAlertmanagerClient_HealthCheck_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
}

func TestAlertmanagerClient_GetStatus_NilClient(t *testing.T) {
	cfg := config.AlertmanagerConfig{
		URL:     "invalid-host:9093",
		Timeout: 1 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	_, err := client.GetStatus(context.Background())
	assert.Error(t, err)
}

func TestAlertmanagerClient_GetAlerts_WithFilterParams(t *testing.T) {
	now := time.Now()
	startsAt := strfmt.DateTime(now)
	endsAt := strfmt.DateTime(now.Add(time.Hour))
	fingerprint := "fp123"
	state := "active"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		assert.Equal(t, "true", query.Get("active"))
		assert.Equal(t, "false", query.Get("silenced"))
		assert.Equal(t, "false", query.Get("inhibited"))
		assert.Equal(t, "webhook", query.Get("receiver"))
		assert.Contains(t, query["filter"], "alertname=TestAlert")

		alerts := models.GettableAlerts{
			{
				Alert: models.Alert{
					Labels: models.LabelSet{"alertname": "TestAlert"},
				},
				Annotations: models.LabelSet{},
				StartsAt:    &startsAt,
				EndsAt:      &endsAt,
				Fingerprint: &fingerprint,
				Status: &models.AlertStatus{
					State: &state,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	filter := domain.AlertFilter{
		Active:    ptrBool(true),
		Silenced:  ptrBool(false),
		Inhibited: ptrBool(false),
		Receiver:  "webhook",
		Filter:    []string{"alertname=TestAlert"},
	}

	alerts, err := client.GetAlerts(context.Background(), filter)
	assert.NoError(t, err)
	assert.NotNil(t, alerts)
	assert.Len(t, alerts, 1)
}

func TestAlertmanagerClient_CreateSilence_WithMultipleMatchers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var silence models.PostableSilence
		err := json.NewDecoder(r.Body).Decode(&silence)
		require.NoError(t, err)

		assert.Len(t, silence.Matchers, 3)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"silenceID": "test-id"})
	}))
	defer server.Close()

	cfg := config.AlertmanagerConfig{
		URL:     server.URL[7:],
		Timeout: 5 * time.Second,
	}
	client := NewAlertmanagerClient(cfg)

	silence := &domain.Silence{
		Matchers: []domain.Matcher{
			{Name: "alertname", Value: "TestAlert", IsRegex: false},
			{Name: "severity", Value: "critical", IsRegex: false},
			{Name: "instance", Value: ".*\\.example\\.com", IsRegex: true},
		},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour),
		CreatedBy: "admin",
		Comment:   "Test",
	}

	id, err := client.CreateSilence(context.Background(), silence)
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
}

func ptrBool(b bool) *bool {
	return &b
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
