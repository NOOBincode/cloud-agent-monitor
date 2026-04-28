package auth

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
	"github.com/stretchr/testify/require"
)

func TestAPIKeyService_CreateAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		req         CreateAPIKeyRequest
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name: "success",
			req: CreateAPIKeyRequest{
				UserID: uuid.New(),
				Name:   "test-key",
				Permissions: models.Permissions{
					ServiceRead: true,
				},
			},
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(nil).Run(func(args mock.Arguments) {
					apiKey := args.Get(1).(*models.APIKey)
					apiKey.ID = uuid.New()
					apiKey.CreatedAt = time.Now()
				})
			},
			expectError: false,
		},
		{
			name: "success with expiration",
			req: CreateAPIKeyRequest{
				UserID:    uuid.New(),
				Name:      "expiring-key",
				ExpiresIn: ptrDuration(24 * time.Hour),
			},
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(nil).Run(func(args mock.Arguments) {
					apiKey := args.Get(1).(*models.APIKey)
					apiKey.ID = uuid.New()
					apiKey.CreatedAt = time.Now()
				})
			},
			expectError: false,
		},
		{
			name: "empty user ID",
			req: CreateAPIKeyRequest{
				UserID: uuid.Nil,
				Name:   "test-key",
			},
			setupMock:   func(m *storage.MockAPIKeyRepository) {},
			expectError: true,
			errorMsg:    "user ID is required",
		},
		{
			name: "empty name",
			req: CreateAPIKeyRequest{
				UserID: uuid.New(),
				Name:   "",
			},
			setupMock:   func(m *storage.MockAPIKeyRepository) {},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "whitespace only name",
			req: CreateAPIKeyRequest{
				UserID: uuid.New(),
				Name:   "   ",
			},
			setupMock:   func(m *storage.MockAPIKeyRepository) {},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "name too long",
			req: CreateAPIKeyRequest{
				UserID: uuid.New(),
				Name:   string(make([]byte, 256)),
			},
			setupMock:   func(m *storage.MockAPIKeyRepository) {},
			expectError: true,
			errorMsg:    "name must be less than 255 characters",
		},
		{
			name: "repository error",
			req: CreateAPIKeyRequest{
				UserID: uuid.New(),
				Name:   "test-key",
			},
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			resp, err := service.CreateAPIKey(context.Background(), tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Key)
				assert.True(t, len(resp.Key) > 10)
				assert.Contains(t, resp.Key, APIKeyPrefix)
				assert.Equal(t, tt.req.Name, resp.Name)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAPIKeyService_ValidateAPIKey(t *testing.T) {
	userID := uuid.New()
	validKey := "obs_" + generateTestKey()

	tests := []struct {
		name        string
		key         string
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectError bool
	}{
		{
			name: "valid key",
			key:  validKey,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByKeyHash", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:        uuid.New(),
					UserID:    userID,
					Name:      "test-key",
					Key:       validKey,
					KeyHash:   HashKey(validKey),
					Prefix:    "obs_a1b2",
					IsActive:  true,
					CreatedAt: time.Now(),
				}, nil)
				m.On("UpdateLastUsed", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:        "invalid prefix",
			key:         "invalid_key",
			setupMock:   func(m *storage.MockAPIKeyRepository) {},
			expectError: true,
		},
		{
			name: "key not found",
			key:  validKey,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByKeyHash", mock.Anything, mock.Anything).Return(nil, storage.ErrNotFound)
			},
			expectError: true,
		},
		{
			name: "inactive key",
			key:  validKey,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByKeyHash", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:        uuid.New(),
					UserID:    userID,
					IsActive:  false,
					CreatedAt: time.Now(),
				}, nil)
			},
			expectError: true,
		},
		{
			name: "expired key",
			key:  validKey,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				expiredTime := time.Now().Add(-24 * time.Hour)
				m.On("GetByKeyHash", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:        uuid.New(),
					UserID:    userID,
					IsActive:  true,
					ExpiresAt: &expiredTime,
					CreatedAt: time.Now().Add(-48 * time.Hour),
				}, nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			apiKey, err := service.ValidateAPIKey(context.Background(), tt.key)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, apiKey)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, apiKey)
				time.Sleep(100 * time.Millisecond)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAPIKeyService_ListUserAPIKeys(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectError bool
	}{
		{
			name:   "success with keys",
			userID: userID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "key1",
						Key:       "obs_abcdef123456",
						Prefix:    "obs_ab",
						IsActive:  true,
						CreatedAt: time.Now(),
					},
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "key2",
						Key:       "obs_xyz789012345",
						Prefix:    "obs_xy",
						IsActive:  false,
						CreatedAt: time.Now(),
					},
				}, nil)
			},
			expectError: false,
		},
		{
			name:   "success with empty list",
			userID: userID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{}, nil)
			},
			expectError: false,
		},
		{
			name:   "repository error",
			userID: userID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("ListByUserID", mock.Anything, userID).Return(nil, errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			keys, err := service.ListUserAPIKeys(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, keys)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, keys)
				for _, key := range keys {
					assert.Contains(t, key.Key, "...")
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAPIKeyService_RevokeAPIKey(t *testing.T) {
	userID := uuid.New()
	keyID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		keyID       uuid.UUID
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectError bool
	}{
		{
			name:   "success",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:     keyID,
					UserID: userID,
					Name:   "test-key",
				}, nil)
				m.On("Deactivate", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:   "key not found",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(nil, storage.ErrAPIKeyNotFound)
			},
			expectError: true,
		},
		{
			name:   "unauthorized - different user",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:     keyID,
					UserID: otherUserID,
					Name:   "test-key",
				}, nil)
			},
			expectError: true,
		},
		{
			name:   "deactivate error",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:     keyID,
					UserID: userID,
					Name:   "test-key",
				}, nil)
				m.On("Deactivate", mock.Anything, mock.Anything).Return(errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			err := service.RevokeAPIKey(context.Background(), tt.userID, tt.keyID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAPIKeyService_RegenerateAPIKey(t *testing.T) {
	userID := uuid.New()
	keyID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		keyID       uuid.UUID
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectError bool
	}{
		{
			name:   "success",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:          keyID,
					UserID:      userID,
					Name:        "test-key",
					Permissions: models.Permissions{ServiceRead: true},
				}, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(nil).Run(func(args mock.Arguments) {
					apiKey := args.Get(1).(*models.APIKey)
					apiKey.ID = uuid.New()
					apiKey.CreatedAt = time.Now()
				})
				m.On("Deactivate", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:   "key not found",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(nil, storage.ErrAPIKeyNotFound)
			},
			expectError: true,
		},
		{
			name:   "unauthorized - different user",
			userID: userID,
			keyID:  keyID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:     keyID,
					UserID: otherUserID,
					Name:   "test-key",
				}, nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			resp, err := service.RegenerateAPIKey(context.Background(), tt.userID, tt.keyID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Key)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAPIKeyService_GetActiveAPIKeys(t *testing.T) {
	userID := uuid.New()
	now := time.Now()
	futureTime := now.Add(24 * time.Hour)
	pastTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name        string
		userID      uuid.UUID
		setupMock   func(m *storage.MockAPIKeyRepository)
		expectedLen int
		expectError bool
	}{
		{
			name:   "filter inactive and expired",
			userID: userID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "active-key",
						IsActive:  true,
						CreatedAt: now,
					},
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "inactive-key",
						IsActive:  false,
						CreatedAt: now,
					},
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "expired-key",
						IsActive:  true,
						ExpiresAt: &pastTime,
						CreatedAt: now.Add(-48 * time.Hour),
					},
					{
						ID:        uuid.New(),
						UserID:    userID,
						Name:      "valid-expiring-key",
						IsActive:  true,
						ExpiresAt: &futureTime,
						CreatedAt: now,
					},
				}, nil)
			},
			expectedLen: 2,
			expectError: false,
		},
		{
			name:   "empty list",
			userID: userID,
			setupMock: func(m *storage.MockAPIKeyRepository) {
				m.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{}, nil)
			},
			expectedLen: 0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockRepo)

			service := NewAPIKeyService(mockRepo)
			keys, err := service.GetActiveAPIKeys(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, keys, tt.expectedLen)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestGenerateSecureKey(t *testing.T) {
	key1, err := generateSecureKey(32)
	require.NoError(t, err)
	assert.Len(t, key1, 64)

	key2, err := generateSecureKey(32)
	require.NoError(t, err)
	assert.NotEqual(t, key1, key2, "generated keys should be unique")
}

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "normal key",
			key:      "obs_abcdef123456789",
			expected: "obs_abcd",
		},
		{
			name:     "short key",
			key:      "obs_ab",
			expected: "obs_ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := extractPrefix(tt.key)
			assert.Equal(t, tt.expected, prefix)
		})
	}
}

func ptrDuration(d time.Duration) *time.Duration {
	return &d
}

func generateTestKey() string {
	key, _ := generateSecureKey(32)
	return key
}

func TestAPIKeyValidatorAdapter_Validate(t *testing.T) {
	userID := uuid.New()
	validKey := "obs_" + generateTestKey()

	mockRepo := new(storage.MockAPIKeyRepository)
	mockRepo.On("GetByKeyHash", mock.Anything, mock.Anything).Return(&models.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      "test-key",
		Key:       validKey,
		KeyHash:   HashKey(validKey),
		Prefix:    "obs_a1b2",
		IsActive:  true,
		CreatedAt: time.Now(),
		Permissions: models.Permissions{
			ServiceRead:  true,
			ServiceWrite: true,
		},
	}, nil)
	mockRepo.On("UpdateLastUsed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	service := NewAPIKeyService(mockRepo)
	adapter := NewAPIKeyValidatorAdapter(service)

	info, err := adapter.Validate(validKey)
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, userID, info.UserID)
	assert.Equal(t, "test-key", info.Name)
	assert.Contains(t, info.Scopes, "service:read")
	assert.Contains(t, info.Scopes, "service:write")

	time.Sleep(100 * time.Millisecond)
	mockRepo.AssertExpectations(t)
}

func TestAPIKeyValidatorAdapter_Validate_AdminPermissions(t *testing.T) {
	userID := uuid.New()
	validKey := "obs_" + generateTestKey()

	mockRepo := new(storage.MockAPIKeyRepository)
	mockRepo.On("GetByKeyHash", mock.Anything, mock.Anything).Return(&models.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      "admin-key",
		Key:       validKey,
		KeyHash:   HashKey(validKey),
		Prefix:    "obs_a1b2",
		IsActive:  true,
		CreatedAt: time.Now(),
		Permissions: models.Permissions{
			Admin: true,
		},
	}, nil)
	mockRepo.On("UpdateLastUsed", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	service := NewAPIKeyService(mockRepo)
	adapter := NewAPIKeyValidatorAdapter(service)

	info, err := adapter.Validate(validKey)
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, []string{"*"}, info.Scopes)

	time.Sleep(100 * time.Millisecond)
	mockRepo.AssertExpectations(t)
}
