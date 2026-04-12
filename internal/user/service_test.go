package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestService_Register(t *testing.T) {
	tests := []struct {
		name        string
		req         RegisterRequest
		setupMock   func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectError error
	}{
		{
			name: "success",
			req: RegisterRequest{
				Username:    "testuser",
				Email:       "test@example.com",
				Password:    "password123",
				DisplayName: "Test User",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, nil)
				userRepo.On("ExistsByEmail", mock.Anything, "test@example.com").Return(false, nil)
				userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
					user := args.Get(1).(*models.User)
					user.ID = uuid.New()
					user.CreatedAt = time.Now()
					user.UpdatedAt = time.Now()
				})
			},
			expectError: nil,
		},
		{
			name: "success with default display name",
			req: RegisterRequest{
				Username: "testuser2",
				Email:    "test2@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "testuser2").Return(false, nil)
				userRepo.On("ExistsByEmail", mock.Anything, "test2@example.com").Return(false, nil)
				userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
					user := args.Get(1).(*models.User)
					user.ID = uuid.New()
					user.CreatedAt = time.Now()
					user.UpdatedAt = time.Now()
				})
			},
			expectError: nil,
		},
		{
			name: "username already exists",
			req: RegisterRequest{
				Username: "existinguser",
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "existinguser").Return(true, nil)
			},
			expectError: ErrUserExists,
		},
		{
			name: "email already exists",
			req: RegisterRequest{
				Username: "newuser",
				Email:    "existing@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "newuser").Return(false, nil)
				userRepo.On("ExistsByEmail", mock.Anything, "existing@example.com").Return(true, nil)
			},
			expectError: ErrUserExists,
		},
		{
			name: "repository error on username check",
			req: RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, errors.New("db error"))
			},
			expectError: errors.New("db error"),
		},
		{
			name: "repository error on create",
			req: RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "testuser").Return(false, nil)
				userRepo.On("ExistsByEmail", mock.Anything, "test@example.com").Return(false, nil)
				userRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.User")).Return(errors.New("db error"))
			},
			expectError: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			resp, err := service.Register(context.Background(), tt.req)

			if tt.expectError != nil {
				assert.Error(t, err)
				if tt.expectError == ErrUserExists {
					assert.Equal(t, tt.expectError, err)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.req.Username, resp.Username)
				assert.Equal(t, tt.req.Email, resp.Email)
				assert.True(t, resp.IsActive)
				if tt.req.DisplayName != "" {
					assert.Equal(t, tt.req.DisplayName, resp.DisplayName)
				} else {
					assert.Equal(t, tt.req.Username, resp.DisplayName)
				}
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_Login(t *testing.T) {
	userID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	jwtService := auth.NewJWTService(auth.JWTConfig{
		SecretKey:      "test-secret-key",
		AccessTokenTTL: 1 * time.Hour,
	})

	tests := []struct {
		name        string
		req         LoginRequest
		setupMock   func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		jwtService  *auth.JWTService
		expectError error
	}{
		{
			name: "success",
			req: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:           userID,
					Username:     "testuser",
					Email:        "test@example.com",
					PasswordHash: string(passwordHash),
					DisplayName:  "Test User",
					IsActive:     true,
					CreatedAt:    time.Now(),
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
				apiKeyRepo.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{}, nil)
			},
			jwtService:  jwtService,
			expectError: nil,
		},
		{
			name: "user not found",
			req: LoginRequest{
				Username: "nonexistent",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "nonexistent").Return(nil, storage.ErrUserNotFound)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
			},
			jwtService:  jwtService,
			expectError: ErrInvalidCredentials,
		},
		{
			name: "user inactive",
			req: LoginRequest{
				Username: "inactiveuser",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "inactiveuser").Return(&models.User{
					ID:           userID,
					Username:     "inactiveuser",
					PasswordHash: string(passwordHash),
					IsActive:     false,
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
			},
			jwtService:  jwtService,
			expectError: ErrUserInactive,
		},
		{
			name: "invalid password",
			req: LoginRequest{
				Username: "testuser",
				Password: "wrongpassword",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:           userID,
					Username:     "testuser",
					PasswordHash: string(passwordHash),
					IsActive:     true,
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
			},
			jwtService:  jwtService,
			expectError: ErrInvalidCredentials,
		},
		{
			name: "success without JWT service",
			req: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:           userID,
					Username:     "testuser",
					Email:        "test@example.com",
					PasswordHash: string(passwordHash),
					DisplayName:  "Test User",
					IsActive:     true,
					CreatedAt:    time.Now(),
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
				apiKeyRepo.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{}, nil)
			},
			jwtService:  nil,
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, tt.jwtService)
			resp, err := service.Login(context.Background(), tt.req, "127.0.0.1", "test-agent")

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.req.Username, resp.User.Username)
				if tt.jwtService != nil {
					assert.NotEmpty(t, resp.AccessToken)
					assert.NotEmpty(t, resp.RefreshToken)
				}
			}

			mockUserRepo.AssertExpectations(t)
			mockAPIKeyRepo.AssertExpectations(t)
		})
	}
}

func TestService_Login_WithActiveAPIKey(t *testing.T) {
	userID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	jwtService := auth.NewJWTService(auth.JWTConfig{SecretKey: "test-secret"})

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: string(passwordHash),
		DisplayName:  "Test User",
		IsActive:     true,
		CreatedAt:    time.Now(),
	}, nil)
	mockUserRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)

	futureTime := time.Now().Add(24 * time.Hour)
	mockAPIKeyRepo.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{
		{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "active-key",
			Key:       "obs_testkey123456",
			IsActive:  true,
			ExpiresAt: &futureTime,
		},
	}, nil)

	service := NewService(mockUserRepo, mockAPIKeyRepo, jwtService)
	resp, err := service.Login(context.Background(), LoginRequest{
		Username: "testuser",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.APIKey)

	mockUserRepo.AssertExpectations(t)
	mockAPIKeyRepo.AssertExpectations(t)
}

func TestService_RefreshToken(t *testing.T) {
	jwtService := auth.NewJWTService(auth.JWTConfig{
		SecretKey:      "test-secret-key",
		AccessTokenTTL: 1 * time.Hour,
	})

	userID := uuid.New()
	tokenPair, err := jwtService.GenerateTokenPair(userID, "testuser", "")
	require.NoError(t, err)

	tests := []struct {
		name         string
		refreshToken string
		jwtService   *auth.JWTService
		expectError  bool
	}{
		{
			name:         "success",
			refreshToken: tokenPair.RefreshToken,
			jwtService:   jwtService,
			expectError:  false,
		},
		{
			name:         "no JWT service",
			refreshToken: "some-token",
			jwtService:   nil,
			expectError:  true,
		},
		{
			name:         "invalid token",
			refreshToken: "invalid-token",
			jwtService:   jwtService,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

			service := NewService(mockUserRepo, mockAPIKeyRepo, tt.jwtService)
			resp, err := service.RefreshToken(context.Background(), tt.refreshToken)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.AccessToken)
				assert.NotEmpty(t, resp.RefreshToken)
			}
		})
	}
}

func TestService_GetByID(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		id          uuid.UUID
		setupMock   func(m *storage.MockUserRepository)
		expectError error
	}{
		{
			name: "success",
			id:   userID,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "test@example.com",
					DisplayName: "Test User",
					IsActive:    true,
					CreatedAt:   time.Now(),
				}, nil)
			},
			expectError: nil,
		},
		{
			name: "user not found",
			id:   userID,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(nil, storage.ErrUserNotFound)
			},
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			resp, err := service.GetByID(context.Background(), tt.id)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.id, resp.ID)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetByUsername(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		username    string
		setupMock   func(m *storage.MockUserRepository)
		expectError error
	}{
		{
			name:     "success",
			username: "testuser",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "test@example.com",
					DisplayName: "Test User",
					IsActive:    true,
					CreatedAt:   time.Now(),
				}, nil)
			},
			expectError: nil,
		},
		{
			name:     "user not found",
			username: "nonexistent",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByUsername", mock.Anything, "nonexistent").Return(nil, storage.ErrUserNotFound)
			},
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			resp, err := service.GetByUsername(context.Background(), tt.username)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.username, resp.Username)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_UpdateProfile(t *testing.T) {
	userID := uuid.New()
	newDisplayName := "New Name"
	newEmail := "new@example.com"

	tests := []struct {
		name        string
		id          uuid.UUID
		req         UpdateProfileRequest
		setupMock   func(m *storage.MockUserRepository)
		expectError error
	}{
		{
			name: "update display name",
			id:   userID,
			req: UpdateProfileRequest{
				DisplayName: &newDisplayName,
			},
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "test@example.com",
					DisplayName: "Old Name",
					IsActive:    true,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectError: nil,
		},
		{
			name: "update email",
			id:   userID,
			req: UpdateProfileRequest{
				Email: &newEmail,
			},
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "old@example.com",
					DisplayName: "Test User",
					IsActive:    true,
				}, nil)
				m.On("ExistsByEmail", mock.Anything, newEmail).Return(false, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectError: nil,
		},
		{
			name: "email already in use",
			id:   userID,
			req: UpdateProfileRequest{
				Email: &newEmail,
			},
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "old@example.com",
					DisplayName: "Test User",
					IsActive:    true,
				}, nil)
				m.On("ExistsByEmail", mock.Anything, newEmail).Return(true, nil)
			},
			expectError: ErrUserExists,
		},
		{
			name: "user not found",
			id:   userID,
			req: UpdateProfileRequest{
				DisplayName: &newDisplayName,
			},
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(nil, storage.ErrUserNotFound)
			},
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			resp, err := service.UpdateProfile(context.Background(), tt.id, tt.req)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_SetUserStatus(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		id          uuid.UUID
		isActive    bool
		setupMock   func(m *storage.MockUserRepository)
		expectError bool
	}{
		{
			name:     "deactivate user",
			id:       userID,
			isActive: false,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:        userID,
					Username:  "testuser",
					IsActive:  true,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "activate user",
			id:       userID,
			isActive: true,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:        userID,
					Username:  "testuser",
					IsActive:  false,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectError: false,
		},
		{
			name:     "user not found",
			id:       userID,
			isActive: false,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByID", mock.Anything, userID).Return(nil, storage.ErrUserNotFound)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			err := service.SetUserStatus(context.Background(), tt.id, tt.isActive)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_ForgotPassword(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		email       string
		setupMock   func(m *storage.MockUserRepository)
		expectToken bool
	}{
		{
			name:  "success",
			email: "test@example.com",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByEmail", mock.Anything, "test@example.com").Return(&models.User{
					ID:        userID,
					Username:  "testuser",
					Email:     "test@example.com",
					IsActive:  true,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectToken: true,
		},
		{
			name:  "user not found - returns empty token",
			email: "nonexistent@example.com",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByEmail", mock.Anything, "nonexistent@example.com").Return(nil, storage.ErrUserNotFound)
			},
			expectToken: false,
		},
		{
			name:  "inactive user - returns empty token",
			email: "inactive@example.com",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByEmail", mock.Anything, "inactive@example.com").Return(&models.User{
					ID:        userID,
					Username:  "inactiveuser",
					Email:     "inactive@example.com",
					IsActive:  false,
				}, nil)
			},
			expectToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			token, err := service.ForgotPassword(context.Background(), tt.email)

			assert.NoError(t, err)
			if tt.expectToken {
				assert.NotEmpty(t, token)
			} else {
				assert.Empty(t, token)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_ResetPassword(t *testing.T) {
	userID := uuid.New()
	validToken := "valid-reset-token"
	expiredTime := time.Now().Add(-1 * time.Hour)
	validExpires := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name        string
		token       string
		newPassword string
		setupMock   func(m *storage.MockUserRepository)
		expectError error
	}{
		{
			name:        "success",
			token:       validToken,
			newPassword: "newpassword123",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByPasswordResetToken", mock.Anything, validToken).Return(&models.User{
					ID:                   userID,
					Username:             "testuser",
					PasswordResetToken:   &validToken,
					PasswordResetExpires: &validExpires,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectError: nil,
		},
		{
			name:        "invalid token",
			token:       "invalid-token",
			newPassword: "newpassword123",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByPasswordResetToken", mock.Anything, "invalid-token").Return(nil, storage.ErrUserNotFound)
			},
			expectError: ErrInvalidToken,
		},
		{
			name:        "expired token",
			token:       validToken,
			newPassword: "newpassword123",
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetByPasswordResetToken", mock.Anything, validToken).Return(&models.User{
					ID:                   userID,
					Username:             "testuser",
					PasswordResetToken:   &validToken,
					PasswordResetExpires: &expiredTime,
				}, nil)
			},
			expectError: ErrTokenExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			err := service.ResetPassword(context.Background(), tt.token, tt.newPassword)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				assert.NoError(t, err)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetUserRoles(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		setupMock   func(m *storage.MockUserRepository)
		expectCount int
		expectError bool
	}{
		{
			name:   "success with roles",
			userID: userID,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetRoles", mock.Anything, userID).Return([]models.Role{
					{ID: uuid.New(), Name: "admin"},
					{ID: uuid.New(), Name: "editor"},
				}, nil)
			},
			expectCount: 2,
			expectError: false,
		},
		{
			name:   "success with no roles",
			userID: userID,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetRoles", mock.Anything, userID).Return([]models.Role{}, nil)
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:   "repository error",
			userID: userID,
			setupMock: func(m *storage.MockUserRepository) {
				m.On("GetRoles", mock.Anything, userID).Return(nil, errors.New("db error"))
			},
			expectCount: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo)

			service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			roles, err := service.GetUserRoles(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, roles, tt.expectCount)
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestService_AssignRole(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("AddRole", mock.Anything, userID, roleID).Return(nil)

	service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	err := service.AssignRole(context.Background(), userID, roleID)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestService_RemoveRole(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("RemoveRole", mock.Anything, userID, roleID).Return(nil)

	service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	err := service.RemoveRole(context.Background(), userID, roleID)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
}

func TestService_ListUsers(t *testing.T) {
	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	filter := storage.UserFilter{Page: 1, PageSize: 10}
	mockUserRepo.On("List", mock.Anything, filter).Return(&storage.UserListResult{
		Data: []models.User{
			{ID: uuid.New(), Username: "user1"},
			{ID: uuid.New(), Username: "user2"},
		},
		Total:    2,
		Page:     1,
		PageSize: 10,
	}, nil)

	service := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	result, err := service.ListUsers(context.Background(), filter)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, int64(2), result.Total)

	mockUserRepo.AssertExpectations(t)
}

func TestToUserResponse(t *testing.T) {
	userID := uuid.New()
	now := time.Now()

	user := &models.User{
		ID:          userID,
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: "Test User",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resp := toUserResponse(user)

	assert.Equal(t, userID, resp.ID)
	assert.Equal(t, "testuser", resp.Username)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.Equal(t, "Test User", resp.DisplayName)
	assert.True(t, resp.IsActive)
	assert.Equal(t, now, resp.CreatedAt)
	assert.Equal(t, now, resp.UpdatedAt)
}
