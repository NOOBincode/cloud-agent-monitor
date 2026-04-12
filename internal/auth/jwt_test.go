package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWTService(t *testing.T) {
	tests := []struct {
		name     string
		config   JWTConfig
		expected JWTConfig
	}{
		{
			name: "default values",
			config: JWTConfig{
				SecretKey: "test-secret-key",
			},
			expected: JWTConfig{
				SecretKey:       "test-secret-key",
				AccessTokenTTL:  1 * time.Hour,
				RefreshTokenTTL: 7 * 24 * time.Hour,
				Issuer:          "cloud-agent-monitor",
			},
		},
		{
			name: "custom values",
			config: JWTConfig{
				SecretKey:       "custom-secret",
				AccessTokenTTL:  30 * time.Minute,
				RefreshTokenTTL: 14 * 24 * time.Hour,
				Issuer:          "custom-issuer",
			},
			expected: JWTConfig{
				SecretKey:       "custom-secret",
				AccessTokenTTL:  30 * time.Minute,
				RefreshTokenTTL: 14 * 24 * time.Hour,
				Issuer:          "custom-issuer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewJWTService(tt.config)
			require.NotNil(t, service)

			config := service.GetConfig()
			assert.Equal(t, tt.expected.SecretKey, config.SecretKey)
			assert.Equal(t, tt.expected.AccessTokenTTL, config.AccessTokenTTL)
			assert.Equal(t, tt.expected.RefreshTokenTTL, config.RefreshTokenTTL)
			assert.Equal(t, tt.expected.Issuer, config.Issuer)
		})
	}
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "test-issuer",
	})

	userID := uuid.New()
	username := "testuser"
	tenantID := "tenant-123"

	tokenPair, err := service.GenerateTokenPair(userID, username, tenantID)
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, int64(3600), tokenPair.ExpiresIn)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
}

func TestJWTService_GenerateTokenPair_EmptyTenantID(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey: "test-secret-key",
	})

	userID := uuid.New()
	username := "testuser"

	tokenPair, err := service.GenerateTokenPair(userID, username, "")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	accessClaims, err := service.ValidateAccessToken(tokenPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "", accessClaims.TenantID)
}

func TestJWTService_ValidateToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})

	userID := uuid.New()
	username := "testuser"
	tenantID := "tenant-123"

	tokenPair, err := service.GenerateTokenPair(userID, username, tenantID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError error
	}{
		{
			name:        "valid access token",
			token:       tokenPair.AccessToken,
			expectError: nil,
		},
		{
			name:        "invalid token format",
			token:       "invalid-token",
			expectError: ErrInvalidToken,
		},
		{
			name:        "empty token",
			token:       "",
			expectError: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateToken(tt.token)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectError), "expected error to be %v, got %v", tt.expectError, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, userID, claims.UserID)
				assert.Equal(t, username, claims.Username)
				assert.Equal(t, tenantID, claims.TenantID)
			}
		})
	}
}

func TestJWTService_ValidateAccessToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})

	userID := uuid.New()
	username := "testuser"

	tokenPair, err := service.GenerateTokenPair(userID, username, "")
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError error
	}{
		{
			name:        "valid access token",
			token:       tokenPair.AccessToken,
			expectError: nil,
		},
		{
			name:        "refresh token used as access token",
			token:       tokenPair.RefreshToken,
			expectError: ErrInvalidTokenType,
		},
		{
			name:        "invalid token",
			token:       "invalid-token",
			expectError: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateAccessToken(tt.token)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectError), "expected error to be %v, got %v", tt.expectError, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, AccessToken, claims.TokenType)
			}
		})
	}
}

func TestJWTService_ValidateRefreshToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})

	userID := uuid.New()
	username := "testuser"

	tokenPair, err := service.GenerateTokenPair(userID, username, "")
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError error
	}{
		{
			name:        "valid refresh token",
			token:       tokenPair.RefreshToken,
			expectError: nil,
		},
		{
			name:        "access token used as refresh token",
			token:       tokenPair.AccessToken,
			expectError: ErrInvalidTokenType,
		},
		{
			name:        "invalid token",
			token:       "invalid-token",
			expectError: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateRefreshToken(tt.token)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectError), "expected error to be %v, got %v", tt.expectError, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, RefreshToken, claims.TokenType)
			}
		})
	}
}

func TestJWTService_RefreshTokens(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	})

	userID := uuid.New()
	username := "testuser"
	tenantID := "tenant-123"

	originalTokenPair, err := service.GenerateTokenPair(userID, username, tenantID)
	require.NoError(t, err)

	newTokenPair, err := service.RefreshTokens(originalTokenPair.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, newTokenPair)

	assert.NotEmpty(t, newTokenPair.AccessToken)
	assert.NotEmpty(t, newTokenPair.RefreshToken)

	accessClaims, err := service.ValidateAccessToken(newTokenPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, username, accessClaims.Username)
	assert.Equal(t, tenantID, accessClaims.TenantID)

	refreshClaims, err := service.ValidateRefreshToken(newTokenPair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, userID, refreshClaims.UserID)
}

func TestJWTService_RefreshTokens_WithAccessToken(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey: "test-secret-key",
	})

	userID := uuid.New()
	tokenPair, err := service.GenerateTokenPair(userID, "testuser", "")
	require.NoError(t, err)

	newTokenPair, err := service.RefreshTokens(tokenPair.AccessToken)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidTokenType, err)
	assert.Nil(t, newTokenPair)
}

func TestJWTService_ValidateToken_WrongSecret(t *testing.T) {
	service1 := NewJWTService(JWTConfig{
		SecretKey: "secret-key-1",
	})

	service2 := NewJWTService(JWTConfig{
		SecretKey: "secret-key-2",
	})

	userID := uuid.New()
	tokenPair, err := service1.GenerateTokenPair(userID, "testuser", "")
	require.NoError(t, err)

	claims, err := service2.ValidateToken(tokenPair.AccessToken)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidToken, err)
	assert.Nil(t, claims)
}

func TestJWTService_TokenClaims(t *testing.T) {
	service := NewJWTService(JWTConfig{
		SecretKey:       "test-secret-key",
		AccessTokenTTL:  1 * time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "test-issuer",
	})

	userID := uuid.New()
	username := "testuser"
	tenantID := "tenant-123"

	tokenPair, err := service.GenerateTokenPair(userID, username, tenantID)
	require.NoError(t, err)

	claims, err := service.ValidateAccessToken(tokenPair.AccessToken)
	require.NoError(t, err)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, tenantID, claims.TenantID)
	assert.Equal(t, AccessToken, claims.TokenType)
	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
	assert.NotZero(t, claims.IssuedAt)
	assert.NotZero(t, claims.ExpiresAt)
	assert.NotZero(t, claims.NotBefore)
}
