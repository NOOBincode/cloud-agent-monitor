package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHandler_Register(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			requestBody: RegisterRequest{
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
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid request - missing username",
			requestBody: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid request - short password",
			requestBody: RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "short",
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid request - invalid email",
			requestBody: RegisterRequest{
				Username: "testuser",
				Email:    "invalid-email",
				Password: "password123",
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "user already exists",
			requestBody: RegisterRequest{
				Username: "existinguser",
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("ExistsByUsername", mock.Anything, "existinguser").Return(true, nil)
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "USER_EXISTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			handler := NewHandler(userService, nil)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/auth/register", handler.Register)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_Login(t *testing.T) {
	userID := uuid.New()
	passwordHash := generatePasswordHash("password123")
	jwtService := auth.NewJWTService(auth.JWTConfig{SecretKey: "test-secret"})

	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:           userID,
					Username:     "testuser",
					Email:        "test@example.com",
					PasswordHash: passwordHash,
					DisplayName:  "Test User",
					IsActive:     true,
					CreatedAt:    time.Now(),
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
				apiKeyRepo.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid request - missing password",
			requestBody: map[string]interface{}{
				"username": "testuser",
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid credentials",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "wrongpassword",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(nil, storage.ErrUserNotFound)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_CREDENTIALS",
		},
		{
			name: "user inactive",
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByUsername", mock.Anything, "testuser").Return(&models.User{
					ID:           userID,
					Username:     "testuser",
					PasswordHash: passwordHash,
					IsActive:     false,
				}, nil)
				userRepo.On("CreateLoginLog", mock.Anything, mock.AnythingOfType("*models.LoginLog")).Return(nil)
			},
			expectedStatus: http.StatusForbidden,
			expectedCode:   "USER_INACTIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, jwtService)
			handler := NewHandler(userService, nil)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/auth/login", handler.Login)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_RefreshToken(t *testing.T) {
	jwtService := auth.NewJWTService(auth.JWTConfig{SecretKey: "test-secret"})
	userID := uuid.New()
	tokenPair, err := jwtService.GenerateTokenPair(userID, "testuser", "")
	require.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    interface{}
		jwtService     *auth.JWTService
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			requestBody: RefreshTokenRequest{
				RefreshToken: tokenPair.RefreshToken,
			},
			jwtService:     jwtService,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing refresh token",
			requestBody:    map[string]interface{}{},
			jwtService:     jwtService,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid refresh token",
			requestBody: RefreshTokenRequest{
				RefreshToken: "invalid-token",
			},
			jwtService:     jwtService,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, tt.jwtService)
			handler := NewHandler(userService, nil)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/auth/refresh", handler.RefreshToken)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}
		})
	}
}

func TestHandler_GetCurrentUser(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "test@example.com",
					DisplayName: "Test User",
					IsActive:    true,
					CreatedAt:   time.Now(),
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized - no user_id in context",
			setupContext:   func(c *gin.Context) {},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "user not found",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByID", mock.Anything, userID).Return(nil, storage.ErrUserNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "USER_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			handler := NewHandler(userService, nil)

			req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/users/me", func(c *gin.Context) {
				tt.setupContext(c)
				handler.GetCurrentUser(c)
			})
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_UpdateProfile(t *testing.T) {
	userID := uuid.New()
	newDisplayName := "New Name"

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		requestBody    interface{}
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			requestBody: UpdateProfileRequest{
				DisplayName: &newDisplayName,
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:          userID,
					Username:    "testuser",
					Email:       "test@example.com",
					DisplayName: "Old Name",
					IsActive:    true,
				}, nil)
				userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized",
			setupContext:   func(c *gin.Context) {},
			requestBody:    UpdateProfileRequest{DisplayName: &newDisplayName},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "invalid request - email too long",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			requestBody: map[string]interface{}{
				"display_name": string(make([]byte, 101)),
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			handler := NewHandler(userService, nil)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/users/me", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.PUT("/users/me", func(c *gin.Context) {
				tt.setupContext(c)
				handler.UpdateProfile(c)
			})
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_CreateAPIKey(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		requestBody    interface{}
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			requestBody: CreateAPIKeyRequest{
				Name: "test-key",
				Permissions: models.Permissions{
					ServiceRead: true,
				},
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				apiKeyRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.APIKey")).Return(nil).Run(func(args mock.Arguments) {
					apiKey := args.Get(1).(*models.APIKey)
					apiKey.ID = uuid.New()
					apiKey.CreatedAt = time.Now()
				})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "unauthorized",
			setupContext:   func(c *gin.Context) {},
			requestBody:    CreateAPIKeyRequest{Name: "test-key"},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "invalid request - missing name",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			requestBody: map[string]interface{}{
				"permissions": models.Permissions{},
			},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			apiKeyService := auth.NewAPIKeyService(mockAPIKeyRepo)
			handler := NewHandler(userService, apiKeyService)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/api-keys", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/api-keys", func(c *gin.Context) {
				tt.setupContext(c)
				handler.CreateAPIKey(c)
			})
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockAPIKeyRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_ListAPIKeys(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				apiKeyRepo.On("ListByUserID", mock.Anything, userID).Return([]models.APIKey{
					{ID: uuid.New(), Name: "key1", Key: "obs_test123"},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized",
			setupContext:   func(c *gin.Context) {},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			apiKeyService := auth.NewAPIKeyService(mockAPIKeyRepo)
			handler := NewHandler(userService, apiKeyService)

			req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/api-keys", func(c *gin.Context) {
				tt.setupContext(c)
				handler.ListAPIKeys(c)
			})
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockAPIKeyRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_RevokeAPIKey(t *testing.T) {
	userID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		keyID          string
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			keyID: keyID.String(),
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				apiKeyRepo.On("GetByID", mock.Anything, mock.Anything).Return(&models.APIKey{
					ID:     keyID,
					UserID: userID,
					Name:   "test-key",
				}, nil)
				apiKeyRepo.On("Deactivate", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "unauthorized",
			setupContext:   func(c *gin.Context) {},
			keyID:          keyID.String(),
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "invalid key ID",
			setupContext: func(c *gin.Context) {
				c.Set("user_id", userID)
			},
			keyID:          "invalid-uuid",
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			apiKeyService := auth.NewAPIKeyService(mockAPIKeyRepo)
			handler := NewHandler(userService, apiKeyService)

			req := httptest.NewRequest(http.MethodDelete, "/api-keys/"+tt.keyID, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.DELETE("/api-keys/:id", func(c *gin.Context) {
				tt.setupContext(c)
				handler.RevokeAPIKey(c)
			})
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockAPIKeyRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_ListUsers(t *testing.T) {
	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("List", mock.Anything, mock.AnythingOfType("storage.UserFilter")).Return(&storage.UserListResult{
		Data: []models.User{
			{ID: uuid.New(), Username: "user1"},
			{ID: uuid.New(), Username: "user2"},
		},
		Total:    2,
		Page:     1,
		PageSize: 20,
	}, nil)

	userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	handler := NewHandler(userService, nil)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	router := gin.New()
	router.GET("/users", handler.ListUsers)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "SUCCESS", resp["code"])
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])

	mockUserRepo.AssertExpectations(t)
}

func TestHandler_SetUserStatus(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		userID         string
		requestBody    interface{}
		setupMock      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name:   "deactivate user",
			userID: userID.String(),
			requestBody: SetUserStatusRequest{
				IsActive: false,
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:       userID,
					Username: "testuser",
					IsActive: true,
				}, nil)
				userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid user ID",
			userID:         "invalid-uuid",
			requestBody:    SetUserStatusRequest{IsActive: false},
			setupMock:      func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_ID",
		},
		{
			name:   "user not found",
			userID: userID.String(),
			requestBody: SetUserStatusRequest{
				IsActive: false,
			},
			setupMock: func(userRepo *storage.MockUserRepository, apiKeyRepo *storage.MockAPIKeyRepository) {
				userRepo.On("GetByID", mock.Anything, userID).Return(nil, storage.ErrUserNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "USER_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(storage.MockUserRepository)
			mockAPIKeyRepo := new(storage.MockAPIKeyRepository)
			tt.setupMock(mockUserRepo, mockAPIKeyRepo)

			userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
			handler := NewHandler(userService, nil)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/users/"+tt.userID+"/status", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.PUT("/users/:id/status", handler.SetUserStatus)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestHandler_GetUserRoles(t *testing.T) {
	userID := uuid.New()

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("GetRoles", mock.Anything, userID).Return([]models.Role{
		{ID: uuid.New(), Name: "admin"},
		{ID: uuid.New(), Name: "editor"},
	}, nil)

	userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	handler := NewHandler(userService, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/roles", nil)
	w := httptest.NewRecorder()

	router := gin.New()
	router.GET("/users/:id/roles", handler.GetUserRoles)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "SUCCESS", resp["code"])
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)

	mockUserRepo.AssertExpectations(t)
}

func TestHandler_AssignRole(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("AddRole", mock.Anything, userID, roleID).Return(nil)

	userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	handler := NewHandler(userService, nil)

	body, _ := json.Marshal(AssignRoleRequest{RoleID: roleID.String()})
	req := httptest.NewRequest(http.MethodPost, "/users/"+userID.String()+"/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/users/:id/roles", handler.AssignRole)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockUserRepo.AssertExpectations(t)
}

func TestHandler_RemoveRole(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()

	mockUserRepo := new(storage.MockUserRepository)
	mockAPIKeyRepo := new(storage.MockAPIKeyRepository)

	mockUserRepo.On("RemoveRole", mock.Anything, userID, roleID).Return(nil)

	userService := NewService(mockUserRepo, mockAPIKeyRepo, nil)
	handler := NewHandler(userService, nil)

	req := httptest.NewRequest(http.MethodDelete, "/users/"+userID.String()+"/roles/"+roleID.String(), nil)
	w := httptest.NewRecorder()

	router := gin.New()
	router.DELETE("/users/:id/roles/:role_id", handler.RemoveRole)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mockUserRepo.AssertExpectations(t)
}

func TestHandler_GetUserID(t *testing.T) {
	userID := uuid.New()

	handler := &Handler{}

	t.Run("success", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", userID)

		id, err := handler.getUserID(c)
		assert.NoError(t, err)
		assert.Equal(t, userID, id)
	})

	t.Run("no user_id in context", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		id, err := handler.getUserID(c)
		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
	})

	t.Run("invalid user_id type", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", "invalid")

		id, err := handler.getUserID(c)
		assert.Error(t, err)
		assert.Equal(t, uuid.Nil, id)
	})
}

func generatePasswordHash(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}
