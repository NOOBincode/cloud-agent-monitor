package infrastructure

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIdempotencyKey_String(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	tests := []struct {
		name      string
		key       *IdempotencyKey
		expectLen int
	}{
		{
			name: "send operation",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			expectLen: 32,
		},
		{
			name: "silence operation",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSilence,
				Fingerprint: "fp456",
			},
			expectLen: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.key.String()
			assert.Len(t, result, tt.expectLen)
		})
	}
}

func TestIdempotencyKey_Deterministic(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	key1 := &IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSend,
		Fingerprint: "fp123",
	}

	key2 := &IdempotencyKey{
		UserID:      userID,
		Operation:   models.AlertOpSend,
		Fingerprint: "fp123",
	}

	assert.Equal(t, key1.String(), key2.String(), "same keys should produce same hash")
}

func TestIdempotencyKey_DifferentKeys(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userID2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		name string
		key1 *IdempotencyKey
		key2 *IdempotencyKey
	}{
		{
			name: "different users",
			key1: &IdempotencyKey{UserID: userID, Operation: models.AlertOpSend, Fingerprint: "fp"},
			key2: &IdempotencyKey{UserID: userID2, Operation: models.AlertOpSend, Fingerprint: "fp"},
		},
		{
			name: "different operations",
			key1: &IdempotencyKey{UserID: userID, Operation: models.AlertOpSend, Fingerprint: "fp"},
			key2: &IdempotencyKey{UserID: userID, Operation: models.AlertOpSilence, Fingerprint: "fp"},
		},
		{
			name: "different fingerprints",
			key1: &IdempotencyKey{UserID: userID, Operation: models.AlertOpSend, Fingerprint: "fp1"},
			key2: &IdempotencyKey{UserID: userID, Operation: models.AlertOpSend, Fingerprint: "fp2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEqual(t, tt.key1.String(), tt.key2.String(), "different keys should produce different hashes")
		})
	}
}

func TestOperationManager_BeginOperation(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		key         *IdempotencyKey
		setupMock   func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository)
		expectError error
	}{
		{
			name: "success - new operation",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, "fp123", 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
			},
			expectError: nil,
		},
		{
			name: "duplicate operation",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, "fp123", 5).Return([]*models.AlertOperation{
					{
						ID:            uuid.New(),
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
			name: "duplicate operation - old enough",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, "fp123", 5).Return([]*models.AlertOperation{
					{
						ID:            uuid.New(),
						UserID:        userID,
						OperationType: models.AlertOpSend,
						Status:        models.AlertOpStatusSuccess,
						CreatedAt:     time.Now().Add(-10 * time.Minute),
					},
				}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
			},
			expectError: nil,
		},
		{
			name: "failed operation - not duplicate",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, "fp123", 5).Return([]*models.AlertOperation{
					{
						ID:            uuid.New(),
						UserID:        userID,
						OperationType: models.AlertOpSend,
						Status:        models.AlertOpStatusFailed,
						CreatedAt:     time.Now(),
					},
				}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(nil)
			},
			expectError: nil,
		},
		{
			name: "repository error on create",
			key: &IdempotencyKey{
				UserID:      userID,
				Operation:   models.AlertOpSend,
				Fingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository, noiseRepo *storage.MockAlertNoiseRepository) {
				opRepo.On("GetByFingerprint", mock.Anything, "fp123", 5).Return([]*models.AlertOperation{}, nil)
				opRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertOperation")).Return(errors.New("db error"))
			},
			expectError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			noiseRepo := new(storage.MockAlertNoiseRepository)
			tt.setupMock(opRepo, noiseRepo)

			manager := NewOperationManager(opRepo, noiseRepo)

			op, err := manager.BeginOperation(context.Background(), tt.key, models.Labels{"alertname": "Test"}, `{"test":"data"}`, "127.0.0.1", "test-agent")

			if tt.expectError != nil {
				assert.Error(t, err)
				if tt.expectError == ErrDuplicateOperation {
					assert.Equal(t, tt.expectError, err)
				}
				assert.Nil(t, op)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, op)
				assert.Equal(t, tt.key.Operation, op.OperationType)
				assert.Equal(t, tt.key.UserID, op.UserID)
			}

			opRepo.AssertExpectations(t)
		})
	}
}

func TestOperationManager_CompleteOperation(t *testing.T) {
	opID := uuid.New()

	tests := []struct {
		name        string
		opID        uuid.UUID
		setupMock   func(opRepo *storage.MockAlertOperationRepository)
		expectError bool
	}{
		{
			name: "success",
			opID: opID,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusSuccess, "").Return(nil)
			},
			expectError: false,
		},
		{
			name: "repository error",
			opID: opID,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusSuccess, "").Return(errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			manager := NewOperationManager(opRepo, nil)

			err := manager.CompleteOperation(context.Background(), tt.opID, "")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			opRepo.AssertExpectations(t)
		})
	}
}

func TestOperationManager_FailOperation(t *testing.T) {
	opID := uuid.New()

	tests := []struct {
		name        string
		op          *models.AlertOperation
		setupMock   func(opRepo *storage.MockAlertOperationRepository)
		expectError bool
	}{
		{
			name: "first failure - retry",
			op: &models.AlertOperation{
				ID:         opID,
				RetryCount: 0,
				MaxRetries: 3,
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByID", mock.Anything, opID).Return(&models.AlertOperation{
					ID:         opID,
					RetryCount: 0,
					MaxRetries: 3,
				}, nil)
				opRepo.On("IncrementRetry", mock.Anything, opID).Return(nil)
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusRetrying, "test error").Return(nil)
			},
			expectError: false,
		},
		{
			name: "max retries exceeded",
			op: &models.AlertOperation{
				ID:         opID,
				RetryCount: 3,
				MaxRetries: 3,
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByID", mock.Anything, opID).Return(&models.AlertOperation{
					ID:         opID,
					RetryCount: 3,
					MaxRetries: 3,
				}, nil)
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusFailed, "test error").Return(nil)
			},
			expectError: false,
		},
		{
			name: "get operation error",
			op: &models.AlertOperation{
				ID: opID,
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByID", mock.Anything, opID).Return(nil, errors.New("not found"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			manager := NewOperationManager(opRepo, nil)

			err := manager.FailOperation(context.Background(), tt.op.ID, "test error")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			opRepo.AssertExpectations(t)
		})
	}
}

func TestOperationManager_GetPendingOperations(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		setupMock   func(opRepo *storage.MockAlertOperationRepository)
		expectCount int
		expectError bool
	}{
		{
			name:  "success",
			limit: 10,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("ListPending", mock.Anything, 10).Return([]*models.AlertOperation{
					{ID: uuid.New(), Status: models.AlertOpStatusPending},
				}, nil)
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:  "empty result",
			limit: 10,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("ListPending", mock.Anything, 10).Return([]*models.AlertOperation{}, nil)
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:  "repository error",
			limit: 10,
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("ListPending", mock.Anything, 10).Return(nil, errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			manager := NewOperationManager(opRepo, nil)

			ops, err := manager.GetPendingOperations(context.Background(), tt.limit)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, ops, tt.expectCount)
			}

			opRepo.AssertExpectations(t)
		})
	}
}

func TestRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxRetries)
	assert.Equal(t, 1*time.Second, policy.InitialWait)
	assert.Equal(t, 30*time.Second, policy.MaxWait)
	assert.Equal(t, 2.0, policy.Multiplier)
}

func TestRetryPolicy_WaitDuration(t *testing.T) {
	policy := DefaultRetryPolicy()

	tests := []struct {
		name       string
		retryCount int
		expected   time.Duration
	}{
		{"retry 0", 0, 1 * time.Second},
		{"retry 1", 1, 2 * time.Second},
		{"retry 2", 2, 4 * time.Second},
		{"retry 3", 3, 8 * time.Second},
		{"retry 10 - capped", 10, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait := policy.WaitDuration(tt.retryCount)
			assert.Equal(t, tt.expected, wait)
		})
	}
}

func TestCompensator_Compensate(t *testing.T) {
	opID := uuid.New()

	tests := []struct {
		name      string
		op        *models.AlertOperation
		setupMock func(opRepo *storage.MockAlertOperationRepository)
	}{
		{
			name: "silence operation",
			op: &models.AlertOperation{
				ID:               opID,
				OperationType:    models.AlertOpSilence,
				AlertFingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {},
		},
		{
			name: "unsilence operation",
			op: &models.AlertOperation{
				ID:               opID,
				OperationType:    models.AlertOpUnsilence,
				AlertFingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {},
		},
		{
			name: "send operation - no compensation",
			op: &models.AlertOperation{
				ID:               opID,
				OperationType:    models.AlertOpSend,
				AlertFingerprint: "fp123",
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			compensator := NewCompensator(opRepo)

			err := compensator.Compensate(context.Background(), tt.op)
			assert.NoError(t, err)

			opRepo.AssertExpectations(t)
		})
	}
}

func TestErrorHandler_HandleError(t *testing.T) {
	opID := uuid.New()

	tests := []struct {
		name      string
		op        *models.AlertOperation
		setupMock func(opRepo *storage.MockAlertOperationRepository)
	}{
		{
			name: "retryable error",
			op: &models.AlertOperation{
				ID:         opID,
				RetryCount: 0,
				MaxRetries: 3,
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByID", mock.Anything, opID).Return(&models.AlertOperation{
					ID:         opID,
					RetryCount: 0,
					MaxRetries: 3,
				}, nil)
				opRepo.On("IncrementRetry", mock.Anything, opID).Return(nil)
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusRetrying, "test error").Return(nil)
			},
		},
		{
			name: "max retries exceeded",
			op: &models.AlertOperation{
				ID:         opID,
				RetryCount: 3,
				MaxRetries: 3,
			},
			setupMock: func(opRepo *storage.MockAlertOperationRepository) {
				opRepo.On("GetByID", mock.Anything, opID).Return(&models.AlertOperation{
					ID:         opID,
					RetryCount: 3,
					MaxRetries: 3,
				}, nil)
				opRepo.On("UpdateStatus", mock.Anything, opID, models.AlertOpStatusFailed, "test error").Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opRepo := new(storage.MockAlertOperationRepository)
			tt.setupMock(opRepo)

			opManager := NewOperationManager(opRepo, nil)
			compensator := NewCompensator(opRepo)
			errorHandler := NewErrorHandler(opManager, compensator)

			err := errorHandler.HandleError(context.Background(), tt.op, errors.New("test error"))
			assert.NoError(t, err)

			opRepo.AssertExpectations(t)
		})
	}
}

func TestErrorHandler_ShouldRetry(t *testing.T) {
	errorHandler := NewErrorHandler(nil, nil)

	tests := []struct {
		name     string
		op       *models.AlertOperation
		expected bool
	}{
		{
			name: "should retry",
			op: &models.AlertOperation{
				RetryCount: 1,
				MaxRetries: 3,
				Status:     models.AlertOpStatusRetrying,
			},
			expected: true,
		},
		{
			name: "max retries exceeded",
			op: &models.AlertOperation{
				RetryCount: 3,
				MaxRetries: 3,
				Status:     models.AlertOpStatusRetrying,
			},
			expected: false,
		},
		{
			name: "not in retrying status",
			op: &models.AlertOperation{
				RetryCount: 1,
				MaxRetries: 3,
				Status:     models.AlertOpStatusPending,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errorHandler.ShouldRetry(tt.op)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorHandler_GetRetryDelay(t *testing.T) {
	errorHandler := NewErrorHandler(nil, nil)

	tests := []struct {
		name     string
		op       *models.AlertOperation
		expected time.Duration
	}{
		{
			name:     "first retry",
			op:       &models.AlertOperation{RetryCount: 0},
			expected: 1 * time.Second,
		},
		{
			name:     "second retry",
			op:       &models.AlertOperation{RetryCount: 1},
			expected: 2 * time.Second,
		},
		{
			name:     "third retry",
			op:       &models.AlertOperation{RetryCount: 2},
			expected: 4 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := errorHandler.GetRetryDelay(tt.op)
			assert.Equal(t, tt.expected, delay)
		})
	}
}
