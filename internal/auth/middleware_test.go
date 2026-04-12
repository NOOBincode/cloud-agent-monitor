package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type MockAPIKeyValidator struct {
	mock.Mock
}

func (m *MockAPIKeyValidator) Validate(key string) (*APIKeyInfo, error) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*APIKeyInfo), args.Error(1)
}

func TestAuthMiddleware_RequireAPIKey_APIKey(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name           string
		setupMock      func(m *MockAPIKeyValidator)
		setupRequest   func(req *http.Request)
		expectedStatus int
		expectedError  string
		checkContext   bool
	}{
		{
			name: "valid API key in Authorization header",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_validkey123").Return(&APIKeyInfo{
					ID:       "key-id",
					Name:     "test-key",
					UserID:   userID,
					TenantID: "tenant-123",
					Scopes:   []string{"service:read"},
				}, nil)
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer obs_validkey123")
			},
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name: "valid API key in X-API-Key header",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_validkey456").Return(&APIKeyInfo{
					ID:       "key-id",
					Name:     "test-key",
					UserID:   userID,
					TenantID: "",
					Scopes:   []string{"*"},
				}, nil)
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Key", "obs_validkey456")
			},
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name: "valid API key in query parameter",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_validkey789").Return(&APIKeyInfo{
					ID:       "key-id",
					Name:     "test-key",
					UserID:   userID,
					TenantID: "",
					Scopes:   []string{"service:read"},
				}, nil)
			},
			setupRequest: func(req *http.Request) {
				req.URL.RawQuery = "api_key=obs_validkey789"
			},
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name:           "missing authentication token",
			setupMock:      func(m *MockAPIKeyValidator) {},
			setupRequest:   func(req *http.Request) {},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "missing authentication token",
		},
		{
			name: "invalid API key",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_invalidkey").Return(nil, errors.New("invalid key"))
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer obs_invalidkey")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockValidator := new(MockAPIKeyValidator)
			tt.setupMock(mockValidator)

			middleware := NewAuthMiddleware(mockValidator)

			var contextUserID uuid.UUID
			var contextScopes []string

			router := gin.New()
			router.Use(middleware.RequireAPIKey())
			router.GET("/test", func(c *gin.Context) {
				if tt.checkContext {
					contextUserID, _ = GetUserID(c)
					contextScopes = GetScopes(c)
				}
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupRequest(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}

			if tt.checkContext {
				assert.Equal(t, userID, contextUserID)
				assert.NotNil(t, contextScopes)
			}

			mockValidator.AssertExpectations(t)
		})
	}
}

func TestAuthMiddleware_RequireAPIKey_JWT(t *testing.T) {
	userID := uuid.New()
	jwtService := NewJWTService(JWTConfig{
		SecretKey:      "test-secret-key",
		AccessTokenTTL: 1 * time.Hour,
	})

	tokenPair, err := jwtService.GenerateTokenPair(userID, "testuser", "tenant-123")
	require.NoError(t, err)

	tests := []struct {
		name           string
		setupRequest   func(req *http.Request)
		jwtService     *JWTService
		expectedStatus int
		expectedError  string
		checkContext   bool
	}{
		{
			name: "valid JWT token",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
			},
			jwtService:     jwtService,
			expectedStatus: http.StatusOK,
			checkContext:   true,
		},
		{
			name: "expired JWT token",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer expired_token")
			},
			jwtService:     jwtService,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid token",
		},
		{
			name: "no JWT service configured",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer some_token")
			},
			jwtService:     nil,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "invalid authentication method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockValidator := new(MockAPIKeyValidator)

			var middleware *AuthMiddleware
			if tt.jwtService != nil {
				middleware = NewAuthMiddlewareWithJWT(mockValidator, tt.jwtService)
			} else {
				middleware = NewAuthMiddleware(mockValidator)
			}

			var contextUserID uuid.UUID
			var contextTenantID string

			router := gin.New()
			router.Use(middleware.RequireAPIKey())
			router.GET("/test", func(c *gin.Context) {
				if tt.checkContext {
					contextUserID, _ = GetUserID(c)
					contextTenantID = GetTenantID(c)
				}
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupRequest(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}

			if tt.checkContext {
				assert.Equal(t, userID, contextUserID)
				assert.Equal(t, "tenant-123", contextTenantID)
			}
		})
	}
}

func TestAuthMiddleware_RequireScope(t *testing.T) {
	tests := []struct {
		name           string
		scopes         []string
		requiredScope  string
		expectedStatus int
	}{
		{
			name:           "has exact scope",
			scopes:         []string{"service:read", "service:write"},
			requiredScope:  "service:read",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "has wildcard scope",
			scopes:         []string{"*"},
			requiredScope:  "service:read",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing scope",
			scopes:         []string{"service:read"},
			requiredScope:  "service:write",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "empty scopes",
			scopes:         []string{},
			requiredScope:  "service:read",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockValidator := new(MockAPIKeyValidator)

			middleware := NewAuthMiddleware(mockValidator)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("scopes", tt.scopes)
				c.Next()
			})
			router.Use(middleware.RequireScope(tt.requiredScope))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAuthMiddleware_RequireScope_NoScopes(t *testing.T) {
	mockValidator := new(MockAPIKeyValidator)
	middleware := NewAuthMiddleware(mockValidator)

	router := gin.New()
	router.Use(middleware.RequireScope("service:read"))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_OptionalAuth(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name         string
		setupMock    func(m *MockAPIKeyValidator)
		setupRequest func(req *http.Request)
		expectUserID bool
		expectedID   uuid.UUID
	}{
		{
			name: "no token - continues without auth",
			setupMock: func(m *MockAPIKeyValidator) {
			},
			setupRequest: func(req *http.Request) {
			},
			expectUserID: false,
		},
		{
			name: "valid API key - sets context",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_validkey").Return(&APIKeyInfo{
					ID:       "key-id",
					Name:     "test-key",
					UserID:   userID,
					TenantID: "",
					Scopes:   []string{"service:read"},
				}, nil)
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer obs_validkey")
			},
			expectUserID: true,
			expectedID:   userID,
		},
		{
			name: "invalid API key - continues without auth",
			setupMock: func(m *MockAPIKeyValidator) {
				m.On("Validate", "obs_invalidkey").Return(nil, errors.New("invalid"))
			},
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer obs_invalidkey")
			},
			expectUserID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockValidator := new(MockAPIKeyValidator)
			tt.setupMock(mockValidator)

			middleware := NewAuthMiddleware(mockValidator)

			var contextUserID uuid.UUID
			var hasUserID bool

			router := gin.New()
			router.Use(middleware.OptionalAuth())
			router.GET("/test", func(c *gin.Context) {
				contextUserID, hasUserID = GetUserID(c)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupRequest(req)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectUserID, hasUserID)
			if tt.expectUserID {
				assert.Equal(t, tt.expectedID, contextUserID)
			}

			mockValidator.AssertExpectations(t)
		})
	}
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		target   string
		expected bool
	}{
		{
			name:     "exact match",
			scopes:   []string{"service:read", "service:write"},
			target:   "service:read",
			expected: true,
		},
		{
			name:     "wildcard match",
			scopes:   []string{"*"},
			target:   "service:read",
			expected: true,
		},
		{
			name:     "no match",
			scopes:   []string{"service:read"},
			target:   "service:write",
			expected: false,
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			target:   "service:read",
			expected: false,
		},
		{
			name:     "nil scopes",
			scopes:   nil,
			target:   "service:read",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasScope(tt.scopes, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAuthToken(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func(req *http.Request)
		expected string
	}{
		{
			name: "Bearer token in Authorization header",
			setupReq: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer token123")
			},
			expected: "token123",
		},
		{
			name: "API key in X-API-Key header",
			setupReq: func(req *http.Request) {
				req.Header.Set("X-API-Key", "obs_key123")
			},
			expected: "obs_key123",
		},
		{
			name: "API key in query parameter",
			setupReq: func(req *http.Request) {
				req.URL.RawQuery = "api_key=obs_querykey"
			},
			expected: "obs_querykey",
		},
		{
			name: "Authorization header takes precedence",
			setupReq: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer token123")
				req.Header.Set("X-API-Key", "obs_key123")
			},
			expected: "token123",
		},
		{
			name: "X-API-Key takes precedence over query",
			setupReq: func(req *http.Request) {
				req.Header.Set("X-API-Key", "obs_key123")
				req.URL.RawQuery = "api_key=obs_querykey"
			},
			expected: "obs_key123",
		},
		{
			name:     "no token provided",
			setupReq: func(req *http.Request) {},
			expected: "",
		},
		{
			name: "malformed Authorization header",
			setupReq: func(req *http.Request) {
				req.Header.Set("Authorization", "InvalidFormat")
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupReq(req)

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			token := extractAuthToken(c)
			assert.Equal(t, tt.expected, token)
		})
	}
}

func TestPermissionsToScopes(t *testing.T) {
	tests := []struct {
		name        string
		permissions models.Permissions
		expected    []string
	}{
		{
			name: "admin has wildcard",
			permissions: models.Permissions{
				Admin: true,
			},
			expected: []string{"*"},
		},
		{
			name: "multiple permissions",
			permissions: models.Permissions{
				ServiceRead:  true,
				ServiceWrite: true,
				ConfigRead:   true,
			},
			expected: []string{"service:read", "service:write", "config:read"},
		},
		{
			name:        "no permissions",
			permissions: models.Permissions{},
			expected:    nil,
		},
		{
			name: "all permissions",
			permissions: models.Permissions{
				ServiceRead:   true,
				ServiceWrite:  true,
				ServiceDelete: true,
				ConfigRead:    true,
				ConfigWrite:   true,
				AuditRead:     true,
			},
			expected: []string{
				"service:read", "service:write", "service:delete",
				"config:read", "config:write", "audit:read",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes := permissionsToScopes(tt.permissions)
			assert.Equal(t, tt.expected, scopes)
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name       string
		setupCtx   func(ctx context.Context) context.Context
		expectedID uuid.UUID
		exists     bool
	}{
		{
			name: "user ID exists",
			setupCtx: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, UserIDKey, userID)
			},
			expectedID: userID,
			exists:     true,
		},
		{
			name: "user ID does not exist",
			setupCtx: func(ctx context.Context) context.Context {
				return ctx
			},
			expectedID: uuid.Nil,
			exists:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx(context.Background())
			id, exists := GetUserIDFromContext(ctx)
			assert.Equal(t, tt.exists, exists)
			assert.Equal(t, tt.expectedID, id)
		})
	}
}

func TestGetTenantFromContext(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func(ctx context.Context) context.Context
		expectedVal string
	}{
		{
			name: "tenant ID exists",
			setupCtx: func(ctx context.Context) context.Context {
				return context.WithValue(ctx, TenantIDKey, "tenant-123")
			},
			expectedVal: "tenant-123",
		},
		{
			name: "tenant ID does not exist",
			setupCtx: func(ctx context.Context) context.Context {
				return ctx
			},
			expectedVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx(context.Background())
			val := GetTenantFromContext(ctx)
			assert.Equal(t, tt.expectedVal, val)
		})
	}
}

func TestHashKey(t *testing.T) {
	key := "test-key-123"
	hash := HashKey(key)

	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64)

	hash2 := HashKey(key)
	assert.Equal(t, hash, hash2, "same key should produce same hash")
}
