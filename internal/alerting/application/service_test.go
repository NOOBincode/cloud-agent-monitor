package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/internal/alerting/infrastructure"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/infra"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	ammodels "github.com/prometheus/alertmanager/api/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAlertService_SendAlert(t *testing.T) {
	tests := []struct {
		name        string
		alert       *domain.Alert
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError error
	}{
		{
			name: "success",
			alert: &domain.Alert{
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "warning"},
				Annotations: map[string]string{"description": "test"},
				StartsAt:    time.Now(),
			},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("SendAlerts", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: nil,
		},
		{
			name:        "nil alert",
			alert:       nil,
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidAlert,
		},
		{
			name: "empty labels",
			alert: &domain.Alert{
				Labels:   map[string]string{},
				StartsAt: time.Now(),
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidAlert,
		},
		{
			name: "alertmanager error",
			alert: &domain.Alert{
				Labels:   map[string]string{"alertname": "TestAlert"},
				StartsAt: time.Now(),
			},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("SendAlerts", mock.Anything, mock.Anything).Return(errors.New("connection refused"))
			},
			expectError: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(
				amClient,
				nil, nil, nil, nil,
				nil, nil, nil,
			)

			err := service.SendAlert(context.Background(), tt.alert)

			if tt.expectError != nil {
				assert.Error(t, err)
				if tt.expectError == ErrInvalidAlert {
					assert.Equal(t, tt.expectError, err)
				}
			} else {
				assert.NoError(t, err)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_SendAlerts(t *testing.T) {
	tests := []struct {
		name        string
		alerts      []*domain.Alert
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError bool
	}{
		{
			name: "success with multiple alerts",
			alerts: []*domain.Alert{
				{Labels: map[string]string{"alertname": "Alert1"}, StartsAt: time.Now()},
				{Labels: map[string]string{"alertname": "Alert2"}, StartsAt: time.Now()},
			},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("SendAlerts", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:        "empty alerts",
			alerts:      []*domain.Alert{},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: false,
		},
		{
			name: "nil alert in slice",
			alerts: []*domain.Alert{
				{Labels: map[string]string{"alertname": "Alert1"}, StartsAt: time.Now()},
				nil,
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: true,
		},
		{
			name: "alert without labels",
			alerts: []*domain.Alert{
				{Labels: map[string]string{}, StartsAt: time.Now()},
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			err := service.SendAlerts(context.Background(), tt.alerts)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_SendAlertWithRecord(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		alert       *domain.Alert
		setupMock   func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository)
		expectError error
	}{
		{
			name: "success",
			alert: &domain.Alert{
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"description": "test"},
				StartsAt:    time.Now(),
			},
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, mock.Anything, 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
				amClient.On("SendAlerts", mock.Anything, mock.Anything).Return(nil)
				opRepo.On("UpdateStatus", mock.Anything, mock.Anything, models.AlertOpStatusSuccess, "").Return(nil)
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
				notifyRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertNotification")).Return(nil)
			},
			expectError: nil,
		},
		{
			name:  "nil alert",
			alert: nil,
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
			},
			expectError: ErrInvalidAlert,
		},
		{
			name: "duplicate operation",
			alert: &domain.Alert{
				Labels:   map[string]string{"alertname": "TestAlert"},
				StartsAt: time.Now(),
			},
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
				opID := uuid.New()
				opRepo.On("GetByFingerprint", mock.Anything, mock.Anything, 5).Return([]*models.AlertOperation{
					{
						ID:            opID,
						UserID:        userID,
						OperationType: models.AlertOpSend,
						Status:        models.AlertOpStatusSuccess,
						CreatedAt:     time.Now(),
					},
				}, nil)
			},
			expectError: ErrDuplicateOperation,
		},
		{
			name: "alertmanager error",
			alert: &domain.Alert{
				Labels:   map[string]string{"alertname": "TestAlert"},
				StartsAt: time.Now(),
			},
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, mock.Anything, 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
				amClient.On("SendAlerts", mock.Anything, mock.Anything).Return(errors.New("connection refused"))
				opRepo.On("GetByID", mock.Anything, mock.Anything).Return(&models.AlertOperation{
					ID:         uuid.New(),
					RetryCount: 0,
					MaxRetries: 3,
				}, nil)
				opRepo.On("IncrementRetry", mock.Anything, mock.Anything).Return(nil)
				opRepo.On("UpdateStatus", mock.Anything, mock.Anything, models.AlertOpStatusRetrying, mock.Anything).Return(nil)
			},
			expectError: errors.New("failed to send alert"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			opRepo := new(storage.MockAlertOperationRepository)
			noiseRepo := new(storage.MockAlertNoiseRepository)
			notifyRepo := new(storage.MockAlertNotificationRepository)
			recordRepo := new(storage.MockAlertRecordRepository)
			tt.setupMock(amClient, opRepo, noiseRepo, notifyRepo)

			cache := infra.NewCache(10)
			queue := &infra.Queue{}

			service := NewAlertService(amClient, opRepo, noiseRepo, notifyRepo, recordRepo, cache, queue, nil)

			op, err := service.SendAlertWithRecord(context.Background(), tt.alert, userID, "127.0.0.1", "test-agent")

			if tt.expectError != nil {
				assert.Error(t, err)
				if tt.expectError == ErrInvalidAlert || tt.expectError == ErrDuplicateOperation {
					assert.Equal(t, tt.expectError, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, op)
			}

			amClient.AssertExpectations(t)
			opRepo.AssertExpectations(t)
			noiseRepo.AssertExpectations(t)
			notifyRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetAlerts(t *testing.T) {
	tests := []struct {
		name        string
		filter      domain.AlertFilter
		setupMock   func(amClient *MockAlertmanagerClient)
		expectCount int
		expectError bool
	}{
		{
			name:   "success",
			filter: domain.AlertFilter{Active: boolPtr(true)},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return([]*domain.Alert{
					{Labels: map[string]string{"alertname": "Alert1"}, StartsAt: time.Now()},
					{Labels: map[string]string{"alertname": "Alert2"}, StartsAt: time.Now()},
				}, nil)
			},
			expectCount: 2,
			expectError: false,
		},
		{
			name:   "empty result",
			filter: domain.AlertFilter{},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return([]*domain.Alert{}, nil)
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:   "alertmanager error",
			filter: domain.AlertFilter{},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return(nil, errors.New("connection refused"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			alerts, err := service.GetAlerts(context.Background(), tt.filter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, alerts, tt.expectCount)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetAlert(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError error
	}{
		{
			name:        "success",
			fingerprint: "abc123",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return([]*domain.Alert{
					{Labels: map[string]string{"alertname": "TestAlert"}, StartsAt: time.Now()},
				}, nil)
			},
			expectError: nil,
		},
		{
			name:        "not found",
			fingerprint: "nonexistent",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return([]*domain.Alert{}, nil)
			},
			expectError: ErrAlertNotFound,
		},
		{
			name:        "alertmanager error",
			fingerprint: "abc123",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetAlerts", mock.Anything, mock.Anything).Return(nil, errors.New("connection refused"))
			},
			expectError: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			alert, err := service.GetAlert(context.Background(), tt.fingerprint)

			if tt.expectError != nil {
				assert.Error(t, err)
				if tt.expectError == ErrAlertNotFound {
					assert.Equal(t, tt.expectError, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, alert)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_CreateSilence(t *testing.T) {
	now := time.Now()
	later := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		silence     *domain.Silence
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError error
	}{
		{
			name: "success",
			silence: &domain.Silence{
				Matchers:  []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
				StartsAt:  now,
				EndsAt:    later,
				CreatedBy: "user1",
				Comment:   "test silence",
			},
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("CreateSilence", mock.Anything, mock.Anything).Return("silence-123", nil)
			},
			expectError: nil,
		},
		{
			name:        "nil silence",
			silence:     nil,
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidSilence,
		},
		{
			name: "empty matchers",
			silence: &domain.Silence{
				Matchers: []domain.Matcher{},
				StartsAt: now,
				EndsAt:   later,
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidSilence,
		},
		{
			name: "missing starts_at",
			silence: &domain.Silence{
				Matchers: []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
				EndsAt:   later,
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidSilence,
		},
		{
			name: "ends_at before starts_at",
			silence: &domain.Silence{
				Matchers: []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
				StartsAt: later,
				EndsAt:   now,
			},
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: ErrInvalidSilence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			id, err := service.CreateSilence(context.Background(), tt.silence)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectError.Error())
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, id)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_DeleteSilence(t *testing.T) {
	tests := []struct {
		name        string
		silenceID   string
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError bool
	}{
		{
			name:      "success",
			silenceID: "silence-123",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("DeleteSilence", mock.Anything, "silence-123").Return(nil)
			},
			expectError: false,
		},
		{
			name:        "empty silence id",
			silenceID:   "",
			setupMock:   func(amClient *MockAlertmanagerClient) {},
			expectError: true,
		},
		{
			name:      "alertmanager error",
			silenceID: "silence-123",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("DeleteSilence", mock.Anything, "silence-123").Return(errors.New("not found"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			err := service.DeleteSilence(context.Background(), tt.silenceID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetAlertRecords(t *testing.T) {
	tests := []struct {
		name        string
		filter      models.AlertRecordFilter
		setupMock   func(recordRepo *storage.MockAlertRecordRepository)
		expectCount int
		expectError bool
	}{
		{
			name:   "success",
			filter: models.AlertRecordFilter{Page: 1, PageSize: 10},
			setupMock: func(recordRepo *storage.MockAlertRecordRepository) {
				recordRepo.On("List", mock.Anything, mock.Anything).Return([]*models.AlertRecord{
					{ID: uuid.New(), Fingerprint: "fp1", Status: models.AlertStatusFiring},
				}, int64(1), nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:   "repository error",
			filter: models.AlertRecordFilter{},
			setupMock: func(recordRepo *storage.MockAlertRecordRepository) {
				recordRepo.On("List", mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordRepo := new(storage.MockAlertRecordRepository)
			tt.setupMock(recordRepo)

			service := NewAlertService(nil, nil, nil, nil, recordRepo, nil, nil, nil)

			records, total, err := service.GetAlertRecords(context.Background(), tt.filter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, records, tt.expectCount)
				assert.Equal(t, int64(tt.expectCount), total)
			}

			recordRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetOperationHistory(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		setupMock   func(opRepo *storage.MockAlertOperationRepository)
		expectCount int
		expectError bool
	}{
		{
			name:   "success",
			userID: userID,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("ListByUser", mock.Anything, userID, 10, 0).Return([]*models.AlertOperation{
					{ID: uuid.New(), OperationType: models.AlertOpSend, Status: models.AlertOpStatusSuccess},
				}, int64(1), nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:   "repository error",
			userID: userID,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("ListByUser", mock.Anything, userID, 10, 0).Return(nil, int64(0), errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			service := NewAlertService(nil, opRepo, nil, nil, nil, nil, nil, nil)

			ops, total, err := service.GetOperationHistory(context.Background(), tt.userID, 10, 0)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, ops, tt.expectCount)
				assert.Equal(t, int64(tt.expectCount), total)
			}

			opRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_HealthCheck(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError bool
	}{
		{
			name: "healthy",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("HealthCheck", mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name: "unhealthy",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("HealthCheck", mock.Anything).Return(errors.New("connection refused"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			err := service.HealthCheck(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetNoisyAlerts(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		setupMock   func(noiseRepo *storage.MockAlertNoiseRepository, cache *infra.Cache)
		expectCount int
		expectError bool
	}{
		{
			name:  "success from repo",
			limit: 10,
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, cache *infra.Cache) {
				noiseRepo.On("GetNoisyAlerts", mock.Anything, 10).Return([]*models.AlertNoiseRecord{
					{ID: uuid.New(), AlertFingerprint: "fp1", IsNoisy: true},
				}, nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:  "repository error",
			limit: 10,
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, cache *infra.Cache) {
				noiseRepo.On("GetNoisyAlerts", mock.Anything, 10).Return(nil, errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			cache := infra.NewCache(10)
			tt.setupMock(noiseRepo, cache)

			service := NewAlertService(nil, nil, noiseRepo, nil, nil, cache, nil, nil)

			records, err := service.GetNoisyAlerts(context.Background(), tt.limit)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, records, tt.expectCount)
			}

			noiseRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_CalculateFingerprint(t *testing.T) {
	noiseRepo := new(storage.MockAlertNoiseRepository)
	service := NewAlertService(nil, nil, noiseRepo, nil, nil, nil, nil, nil)

	labels1 := map[string]string{"alertname": "TestAlert", "severity": "warning"}
	labels2 := map[string]string{"severity": "warning", "alertname": "TestAlert"}
	labels3 := map[string]string{"alertname": "DifferentAlert"}

	fp1 := service.calculateFingerprint(labels1)
	fp2 := service.calculateFingerprint(labels2)
	fp3 := service.calculateFingerprint(labels3)

	assert.NotEmpty(t, fp1)
	assert.Equal(t, fp1, fp2, "same labels in different order should produce same fingerprint")
	assert.NotEqual(t, fp1, fp3, "different labels should produce different fingerprint")
}

func boolPtr(b bool) *bool {
	return &b
}

type MockAlertmanagerClient struct {
	mock.Mock
}

func (m *MockAlertmanagerClient) SendAlerts(ctx context.Context, alerts []*domain.Alert) error {
	args := m.Called(ctx, alerts)
	return args.Error(0)
}

func (m *MockAlertmanagerClient) GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertmanagerClient) CreateSilence(ctx context.Context, silence *domain.Silence) (string, error) {
	args := m.Called(ctx, silence)
	return args.String(0), args.Error(1)
}

func (m *MockAlertmanagerClient) GetSilences(ctx context.Context) ([]*domain.Silence, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Silence), args.Error(1)
}

func (m *MockAlertmanagerClient) DeleteSilence(ctx context.Context, silenceID string) error {
	args := m.Called(ctx, silenceID)
	return args.Error(0)
}

func (m *MockAlertmanagerClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockAlertmanagerClient) GetStatus(ctx context.Context) (*ammodels.AlertmanagerStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ammodels.AlertmanagerStatus), args.Error(1)
}

func TestAlertService_StartStop(t *testing.T) {
	t.Run("start and stop with buffer", func(t *testing.T) {
		recordRepo := new(storage.MockAlertRecordRepository)
		buffer := infrastructure.NewAlertRecordBuffer(recordRepo, nil, infrastructure.DefaultAlertRecordBufferConfig())

		service := NewAlertService(nil, nil, nil, nil, recordRepo, nil, nil, buffer)

		service.Start()
		time.Sleep(100 * time.Millisecond)
		service.Stop()
	})

	t.Run("start and stop without buffer", func(t *testing.T) {
		service := NewAlertService(nil, nil, nil, nil, nil, nil, nil, nil)

		service.Start()
		service.Stop()
	})
}

func TestAlertService_PersistAlertsFromAlertmanager(t *testing.T) {
	tests := []struct {
		name      string
		alerts    []*domain.Alert
		setupMock func(buffer *MockAlertRecordBuffer)
	}{
		{
			name: "persist firing alerts",
			alerts: []*domain.Alert{
				{
					Labels:   map[string]string{"alertname": "TestAlert"},
					StartsAt: time.Now(),
					Status:   domain.AlertStatusFiring,
				},
			},
			setupMock: func(buffer *MockAlertRecordBuffer) {
				buffer.On("AddFromAlert", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "persist resolved alerts",
			alerts: []*domain.Alert{
				{
					Labels:   map[string]string{"alertname": "TestAlert"},
					StartsAt: time.Now(),
					EndsAt:   timePtr(time.Now().Add(5 * time.Minute)),
					Status:   domain.AlertStatusResolved,
				},
			},
			setupMock: func(buffer *MockAlertRecordBuffer) {
				buffer.On("AddFromAlert", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "skip nil alerts",
			alerts: []*domain.Alert{
				nil,
				{Labels: map[string]string{"alertname": "TestAlert"}, StartsAt: time.Now()},
			},
			setupMock: func(buffer *MockAlertRecordBuffer) {
				buffer.On("AddFromAlert", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once()
			},
		},
		{
			name:   "empty alerts",
			alerts: []*domain.Alert{},
			setupMock: func(buffer *MockAlertRecordBuffer) {
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buffer := new(MockAlertRecordBuffer)
			tt.setupMock(buffer)

			service := &AlertService{
				recordBuffer: buffer,
				feedback:     NewFeedbackService(nil, nil, nil),
			}
			service.feedback.analyzer = &NoiseAnalyzer{}

			err := service.PersistAlertsFromAlertmanager(context.Background(), tt.alerts)
			assert.NoError(t, err)

			buffer.AssertExpectations(t)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

type MockAlertRecordBuffer struct {
	mock.Mock
}

func (m *MockAlertRecordBuffer) Add(record *models.AlertRecord) {
	m.Called(record)
}

func (m *MockAlertRecordBuffer) AddFromAlert(
	fingerprint string,
	labels, annotations map[string]string,
	status models.AlertStatus,
	startsAt time.Time,
	endsAt *time.Time,
	source string,
) {
	m.Called(fingerprint, labels, annotations, status, startsAt, endsAt, source)
}

func (m *MockAlertRecordBuffer) Start() {
	m.Called()
}

func (m *MockAlertRecordBuffer) Stop() {
	m.Called()
}

func (m *MockAlertRecordBuffer) Flush() {
	m.Called()
}

func (m *MockAlertRecordBuffer) GetBufferSize() int {
	args := m.Called()
	return args.Int(0)
}

func TestAlertService_GetAlertRecordStats(t *testing.T) {
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	tests := []struct {
		name        string
		from        time.Time
		to          time.Time
		setupMock   func(recordRepo *storage.MockAlertRecordRepository)
		expectError bool
	}{
		{
			name: "success",
			from: from,
			to:   to,
			setupMock: func(recordRepo *storage.MockAlertRecordRepository) {
				recordRepo.On("GetStats", mock.Anything, mock.Anything, mock.Anything).Return(&models.AlertRecordStats{
					TotalCount:  100,
					FiringCount: 80,
				}, nil)
			},
			expectError: false,
		},
		{
			name: "repository error",
			from: from,
			to:   to,
			setupMock: func(recordRepo *storage.MockAlertRecordRepository) {
				recordRepo.On("GetStats", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recordRepo := new(storage.MockAlertRecordRepository)
			tt.setupMock(recordRepo)

			service := NewAlertService(nil, nil, nil, nil, recordRepo, nil, nil, nil)

			stats, err := service.GetAlertRecordStats(context.Background(), tt.from, tt.to)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stats)
				assert.Equal(t, int64(100), stats.TotalCount)
			}

			recordRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetHighRiskAlerts(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		setupMock   func(noiseRepo *storage.MockAlertNoiseRepository)
		expectCount int
		expectError bool
	}{
		{
			name:  "success",
			limit: 10,
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetHighRiskAlerts", mock.Anything, 10).Return([]*models.AlertNoiseRecord{
					{ID: uuid.New(), AlertFingerprint: "fp1", IsHighRisk: true},
				}, nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:  "repository error",
			limit: 10,
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetHighRiskAlerts", mock.Anything, 10).Return(nil, errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(noiseRepo)

			analyzer := &NoiseAnalyzer{noiseRepo: noiseRepo}
			feedback := NewFeedbackService(analyzer, nil, nil)

			service := &AlertService{feedback: feedback}

			records, err := service.GetHighRiskAlerts(context.Background(), tt.limit)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, records, tt.expectCount)
			}

			noiseRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_CreateSilenceWithRecord(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	later := now.Add(1 * time.Hour)

	tests := []struct {
		name        string
		silence     *domain.Silence
		setupMock   func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository)
		expectError error
	}{
		{
			name: "success",
			silence: &domain.Silence{
				Matchers:  []domain.Matcher{{Name: "alertname", Value: "TestAlert"}},
				StartsAt:  now,
				EndsAt:    later,
				CreatedBy: "user1",
			},
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, mock.Anything, 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
				amClient.On("CreateSilence", mock.Anything, mock.Anything).Return("silence-123", nil)
				opRepo.On("UpdateStatus", mock.Anything, mock.Anything, models.AlertOpStatusSuccess, "silence-123").Return(nil)
			},
			expectError: nil,
		},
		{
			name:        "nil silence",
			silence:     nil,
			setupMock:   func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository) {},
			expectError: ErrInvalidSilence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			opRepo := new(storage.MockAlertOperationRepository)
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(amClient, opRepo)

			service := NewAlertService(amClient, opRepo, noiseRepo, nil, nil, nil, nil, nil)

			op, silenceID, err := service.CreateSilenceWithRecord(context.Background(), tt.silence, userID, "127.0.0.1", "test-agent")

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, op)
				assert.NotEmpty(t, silenceID)
			}

			amClient.AssertExpectations(t)
			opRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_DeleteSilenceWithRecord(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		silenceID   string
		setupMock   func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository)
		expectError error
	}{
		{
			name:      "success",
			silenceID: "silence-123",
			setupMock: func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, mock.Anything, 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
				amClient.On("DeleteSilence", mock.Anything, "silence-123").Return(nil)
				opRepo.On("UpdateStatus", mock.Anything, mock.Anything, models.AlertOpStatusSuccess, "").Return(nil)
			},
			expectError: nil,
		},
		{
			name:        "empty silence id",
			silenceID:   "",
			setupMock:   func(amClient *MockAlertmanagerClient, opRepo *storage.MockAlertOperationRepository) {},
			expectError: ErrInvalidSilence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			opRepo := new(storage.MockAlertOperationRepository)
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(amClient, opRepo)

			service := NewAlertService(amClient, opRepo, noiseRepo, nil, nil, nil, nil, nil)

			op, err := service.DeleteSilenceWithRecord(context.Background(), tt.silenceID, userID, "127.0.0.1", "test-agent")

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, op)
			}

			amClient.AssertExpectations(t)
			opRepo.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetSilences(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(amClient *MockAlertmanagerClient)
		expectCount int
		expectError bool
	}{
		{
			name: "success",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetSilences", mock.Anything).Return([]*domain.Silence{
					{CreatedBy: "user1"},
				}, nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name: "error",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetSilences", mock.Anything).Return(nil, errors.New("connection refused"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			silences, err := service.GetSilences(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, silences, tt.expectCount)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_GetAlertmanagerStatus(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(amClient *MockAlertmanagerClient)
		expectError bool
	}{
		{
			name: "success",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetStatus", mock.Anything).Return(&ammodels.AlertmanagerStatus{}, nil)
			},
			expectError: false,
		},
		{
			name: "error",
			setupMock: func(amClient *MockAlertmanagerClient) {
				amClient.On("GetStatus", mock.Anything).Return(nil, errors.New("connection refused"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amClient := new(MockAlertmanagerClient)
			tt.setupMock(amClient)

			service := NewAlertService(amClient, nil, nil, nil, nil, nil, nil, nil)

			status, err := service.GetAlertmanagerStatus(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, status)
			}

			amClient.AssertExpectations(t)
		})
	}
}

func TestAlertService_EnqueueNotification(t *testing.T) {
	t.Run("enqueue notification", func(t *testing.T) {
		queue := new(MockQueue)

		service := &AlertService{queue: queue}

		queue.On("Enqueue", mock.Anything, "alert:notify", mock.Anything).Return(nil, nil)

		err := service.EnqueueNotification(context.Background(), "fp1", "email", "user@example.com")
		assert.NoError(t, err)

		queue.AssertExpectations(t)
	})
}

type MockQueue struct {
	mock.Mock
}

func (m *MockQueue) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	args := m.Called(ctx, taskType, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*asynq.TaskInfo), args.Error(1)
}

func (m *MockQueue) EnqueueWithDelay(ctx context.Context, taskType string, payload any, delay time.Duration, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	args := m.Called(ctx, taskType, payload, delay)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*asynq.TaskInfo), args.Error(1)
}

func (m *MockQueue) RegisterHandler(taskType string, handler infra.TaskHandler) {
	m.Called(taskType, handler)
}

func (m *MockQueue) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockQueue) Stop() {
	m.Called()
}

func (m *MockQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockQueue) GetInspector() *asynq.Inspector {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*asynq.Inspector)
}

func TestAlertService_ProcessRetryQueue(t *testing.T) {
	opID := uuid.New()

	tests := []struct {
		name        string
		setupMock   func(opRepo *storage.MockAlertOperationRepository, queue *MockQueue)
		expectError bool
	}{
		{
			name: "process pending operations",
			setupMock: func(opRepo *storage.MockAlertOperationRepository, queue *MockQueue) {
				opRepo.On("ListPending", mock.Anything, 100).Return([]*models.AlertOperation{
					{ID: opID, OperationType: models.AlertOpSend, RetryCount: 1, MaxRetries: 3, Status: models.AlertOpStatusRetrying},
				}, nil)
				queue.On("EnqueueWithDelay", mock.Anything, "alert:send", mock.Anything, mock.Anything).Return((*asynq.TaskInfo)(nil), nil)
			},
			expectError: false,
		},
		{
			name: "no pending operations",
			setupMock: func(opRepo *storage.MockAlertOperationRepository, queue *MockQueue) {
				opRepo.On("ListPending", mock.Anything, 100).Return([]*models.AlertOperation{}, nil)
			},
			expectError: false,
		},
		{
			name: "repository error",
			setupMock: func(opRepo *storage.MockAlertOperationRepository, queue *MockQueue) {
				opRepo.On("ListPending", mock.Anything, 100).Return(nil, errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			queue := new(MockQueue)
			tt.setupMock(opRepo, queue)

			opManager := infrastructure.NewOperationManager(opRepo, nil)
			service := &AlertService{
				opRepo:       opRepo,
				queue:        queue,
				opManager:    opManager,
				errorHandler: infrastructure.NewErrorHandler(opManager, infrastructure.NewCompensator(opRepo)),
			}

			err := service.ProcessRetryQueue(context.Background())
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			opRepo.AssertExpectations(t)
			queue.AssertExpectations(t)
		})
	}
}

func TestNoiseAnalyzer_CalculateFingerprint(t *testing.T) {
	analyzer := NewNoiseAnalyzer(nil, nil, nil, nil)

	tests := []struct {
		name   string
		labels map[string]string
	}{
		{"simple labels", map[string]string{"alertname": "TestAlert"}},
		{"multiple labels", map[string]string{"alertname": "TestAlert", "severity": "critical", "instance": "localhost:9090"}},
		{"empty labels", map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := analyzer.CalculateFingerprint(tt.labels)
			assert.NotEmpty(t, fp)
			assert.Len(t, fp, 32)
		})
	}
}

func TestNoiseAnalyzer_RecordAlertFired(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		setupMock func(noiseRepo *storage.MockAlertNoiseRepository)
	}{
		{
			name:   "new alert",
			labels: map[string]string{"alertname": "NewAlert", "severity": "critical"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
			},
		},
		{
			name:   "existing alert",
			labels: map[string]string{"alertname": "ExistingAlert"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(&models.AlertNoiseRecord{
					ID:               uuid.New(),
					AlertFingerprint: "existing-fp",
					FireCount:        5,
				}, nil)
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(noiseRepo)

			analyzer := NewNoiseAnalyzer(noiseRepo, nil, nil, nil)

			err := analyzer.RecordAlertFired(context.Background(), tt.labels)
			assert.NoError(t, err)

			noiseRepo.AssertExpectations(t)
		})
	}
}

func TestNoiseAnalyzer_RecordAlertResolved(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		setupMock func(noiseRepo *storage.MockAlertNoiseRepository)
	}{
		{
			name:   "existing alert",
			labels: map[string]string{"alertname": "TestAlert"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(&models.AlertNoiseRecord{
					ID:               uuid.New(),
					AlertFingerprint: "fp1",
					FireCount:        5,
					ResolveCount:     2,
				}, nil)
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
			},
		},
		{
			name:   "non-existing alert",
			labels: map[string]string{"alertname": "UnknownAlert"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(noiseRepo)

			analyzer := NewNoiseAnalyzer(noiseRepo, nil, nil, nil)

			err := analyzer.RecordAlertResolved(context.Background(), tt.labels)
			assert.NoError(t, err)

			noiseRepo.AssertExpectations(t)
		})
	}
}

func TestNoiseAnalyzer_ShouldSuggestSilence(t *testing.T) {
	tests := []struct {
		name          string
		fingerprint   string
		setupMock     func(noiseRepo *storage.MockAlertNoiseRepository)
		expectSuggest bool
		expectMessage bool
	}{
		{
			name:        "should suggest silence",
			fingerprint: "noisy-fp",
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, "noisy-fp").Return(&models.AlertNoiseRecord{
					SilenceSuggested: true,
				}, nil)
			},
			expectSuggest: true,
			expectMessage: true,
		},
		{
			name:        "should not suggest silence",
			fingerprint: "quiet-fp",
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, "quiet-fp").Return(&models.AlertNoiseRecord{
					SilenceSuggested: false,
				}, nil)
			},
			expectSuggest: false,
			expectMessage: false,
		},
		{
			name:        "not found",
			fingerprint: "unknown-fp",
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, "unknown-fp").Return(nil, errors.New("not found"))
			},
			expectSuggest: false,
			expectMessage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(noiseRepo)

			analyzer := NewNoiseAnalyzer(noiseRepo, nil, nil, nil)

			suggest, msg := analyzer.ShouldSuggestSilence(context.Background(), tt.fingerprint)
			assert.Equal(t, tt.expectSuggest, suggest)
			if tt.expectMessage {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}

			noiseRepo.AssertExpectations(t)
		})
	}
}

func TestFeedbackService_ProcessAlertEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		labels    map[string]string
		setupMock func(noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository)
	}{
		{
			name:      "firing event",
			eventType: "firing",
			labels:    map[string]string{"alertname": "TestAlert", "severity": "critical"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
				notifyRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertNotification")).Return(nil)
			},
		},
		{
			name:      "resolved event",
			eventType: "resolved",
			labels:    map[string]string{"alertname": "TestAlert"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, mock.Anything).Return(&models.AlertNoiseRecord{
					ID: uuid.New(),
				}, nil)
				noiseRepo.On("Upsert", mock.Anything, mock.AnythingOfType("*models.AlertNoiseRecord")).Return(nil)
			},
		},
		{
			name:      "unknown event type",
			eventType: "unknown",
			labels:    map[string]string{"alertname": "TestAlert"},
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, notifyRepo *storage.MockAlertNotificationRepository) {
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			notifyRepo := new(storage.MockAlertNotificationRepository)
			tt.setupMock(noiseRepo, notifyRepo)

			analyzer := NewNoiseAnalyzer(noiseRepo, nil, nil, nil)
			feedback := NewFeedbackService(analyzer, notifyRepo, nil)

			feedback.ProcessAlertEvent(context.Background(), tt.eventType, tt.labels)

			noiseRepo.AssertExpectations(t)
			notifyRepo.AssertExpectations(t)
		})
	}
}

func TestFeedbackService_GetAlertFeedback(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		setupMock   func(noiseRepo *storage.MockAlertNoiseRepository, opRepo *storage.MockAlertOperationRepository)
		expectError bool
	}{
		{
			name:        "success",
			fingerprint: "fp1",
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, opRepo *storage.MockAlertOperationRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, "fp1").Return(&models.AlertNoiseRecord{
					ID:               uuid.New(),
					AlertFingerprint: "fp1",
					AlertName:        "TestAlert",
					FireCount:        10,
					NoiseScore:       5.5,
				}, nil)
				opRepo.On("GetByFingerprint", mock.Anything, "fp1", 10).Return([]*models.AlertOperation{}, nil)
			},
			expectError: false,
		},
		{
			name:        "not found",
			fingerprint: "unknown",
			setupMock: func(noiseRepo *storage.MockAlertNoiseRepository, opRepo *storage.MockAlertOperationRepository) {
				noiseRepo.On("GetByFingerprint", mock.Anything, "unknown").Return(nil, errors.New("not found"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noiseRepo := new(storage.MockAlertNoiseRepository)
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(noiseRepo, opRepo)

			analyzer := NewNoiseAnalyzer(noiseRepo, nil, nil, nil)
			feedback := NewFeedbackService(analyzer, nil, opRepo)

			result, err := feedback.GetAlertFeedback(context.Background(), tt.fingerprint)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.fingerprint, result.Fingerprint)
			}

			noiseRepo.AssertExpectations(t)
			opRepo.AssertExpectations(t)
		})
	}
}

func TestRetryPolicy_WaitDuration(t *testing.T) {
	policy := infrastructure.DefaultRetryPolicy()

	tests := []struct {
		name       string
		retryCount int
		expected   time.Duration
	}{
		{"first retry", 0, 1 * time.Second},
		{"second retry", 1, 2 * time.Second},
		{"third retry", 2, 4 * time.Second},
		{"max wait", 10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait := policy.WaitDuration(tt.retryCount)
			assert.Equal(t, tt.expected, wait)
		})
	}
}

func TestIdempotencyKey_String(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	key1 := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSend,
		Fingerprint: "fp1",
	}

	key2 := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSend,
		Fingerprint: "fp1",
	}

	key3 := &infrastructure.IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSilence,
		Fingerprint: "fp1",
	}

	str1 := key1.String()
	str2 := key2.String()
	str3 := key3.String()

	require.NotEmpty(t, str1)
	assert.Equal(t, str1, str2, "same keys should produce same string")
	assert.NotEqual(t, str1, str3, "different operations should produce different strings")
}
