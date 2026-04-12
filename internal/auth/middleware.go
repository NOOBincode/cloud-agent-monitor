package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type APIKeyValidator interface {
	Validate(key string) (*APIKeyInfo, error)
}

type APIKeyInfo struct {
	ID        string
	Name      string
	TenantID  string
	Scopes    []string
	ExpiresAt *int64
	UserID    uuid.UUID
	Role      string
}

type AuthMiddleware struct {
	validator  APIKeyValidator
	jwtService *JWTService
}

func NewAuthMiddleware(validator APIKeyValidator) *AuthMiddleware {
	return &AuthMiddleware{validator: validator}
}

func NewAuthMiddlewareWithJWT(validator APIKeyValidator, jwtService *JWTService) *AuthMiddleware {
	return &AuthMiddleware{
		validator:  validator,
		jwtService: jwtService,
	}
}

func (a *AuthMiddleware) RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractAuthToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authentication token",
			})
			return
		}

		var userID uuid.UUID
		var tenantID string
		var scopes []string
		var apiKeyID string
		var apiKeyName string

		if strings.HasPrefix(token, APIKeyPrefix) {
			info, err := a.validator.Validate(token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid API key",
				})
				return
			}
			userID = info.UserID
			tenantID = info.TenantID
			scopes = info.Scopes
			apiKeyID = info.ID
			apiKeyName = info.Name
		} else if a.jwtService != nil {
			claims, err := a.jwtService.ValidateAccessToken(token)
			if err != nil {
				if err == ErrExpiredToken {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error": "token expired",
						"code":  "TOKEN_EXPIRED",
					})
					return
				}
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid token",
				})
				return
			}
			userID = claims.UserID
			tenantID = claims.TenantID
			scopes = []string{"*"}
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authentication method",
			})
			return
		}

		c.Set("user_id", userID)
		if apiKeyID != "" {
			c.Set("api_key_id", apiKeyID)
			c.Set("api_key_name", apiKeyName)
		}
		c.Set("tenant_id", tenantID)
		c.Set("scopes", scopes)

		ctx := context.WithValue(c.Request.Context(), UserIDKey, userID)
		if tenantID != "" {
			ctx = context.WithValue(ctx, TenantIDKey, tenantID)
		} else {
			ctx = context.WithValue(ctx, TenantIDKey, "default")
		}
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func (a *AuthMiddleware) RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes, exists := c.Get("scopes")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "not authenticated",
			})
			return
		}

		scopeList, ok := scopes.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "invalid scope data",
			})
			return
		}

		if !hasScope(scopeList, scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			return
		}

		c.Next()
	}
}

func (a *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractAuthToken(c)
		if token == "" {
			c.Next()
			return
		}

		if strings.HasPrefix(token, APIKeyPrefix) {
			info, err := a.validator.Validate(token)
			if err == nil {
				c.Set("user_id", info.UserID)
				c.Set("tenant_id", info.TenantID)
				c.Set("scopes", info.Scopes)
				c.Set("api_key_id", info.ID)
				c.Set("api_key_name", info.Name)

				ctx := context.WithValue(c.Request.Context(), UserIDKey, info.UserID)
				c.Request = c.Request.WithContext(ctx)
			}
		} else if a.jwtService != nil {
			claims, err := a.jwtService.ValidateAccessToken(token)
			if err == nil {
				c.Set("user_id", claims.UserID)
				c.Set("tenant_id", claims.TenantID)
				c.Set("scopes", []string{"*"})

				ctx := context.WithValue(c.Request.Context(), UserIDKey, claims.UserID)
				c.Request = c.Request.WithContext(ctx)
			}
		}

		c.Next()
	}
}

func extractAuthToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	key := c.GetHeader("X-API-Key")
	if key != "" {
		return key
	}

	key = c.Query("api_key")
	return key
}

func hasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target || s == "*" {
			return true
		}
	}
	return false
}

func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func permissionsToScopes(p models.Permissions) []string {
	var scopes []string
	if p.Admin {
		return []string{"*"}
	}
	if p.ServiceRead {
		scopes = append(scopes, "service:read")
	}
	if p.ServiceWrite {
		scopes = append(scopes, "service:write")
	}
	if p.ServiceDelete {
		scopes = append(scopes, "service:delete")
	}
	if p.ConfigRead {
		scopes = append(scopes, "config:read")
	}
	if p.ConfigWrite {
		scopes = append(scopes, "config:write")
	}
	if p.AuditRead {
		scopes = append(scopes, "audit:read")
	}
	return scopes
}

type APIKeyValidatorAdapter struct {
	service *APIKeyService
}

func NewAPIKeyValidatorAdapter(service *APIKeyService) *APIKeyValidatorAdapter {
	return &APIKeyValidatorAdapter{service: service}
}

func (a *APIKeyValidatorAdapter) Validate(key string) (*APIKeyInfo, error) {
	ctx := context.Background()
	apiKey, err := a.service.ValidateAPIKey(ctx, key)
	if err != nil {
		return nil, err
	}

	info := &APIKeyInfo{
		ID:        apiKey.ID.String(),
		Name:      apiKey.Name,
		TenantID:  "",
		Scopes:    permissionsToScopes(apiKey.Permissions),
		ExpiresAt: nil,
		UserID:    apiKey.UserID,
	}

	if apiKey.TenantID != nil {
		info.TenantID = apiKey.TenantID.String()
	}

	if apiKey.ExpiresAt != nil {
		expiresAt := apiKey.ExpiresAt.Unix()
		info.ExpiresAt = &expiresAt
	}

	return info, nil
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	id, ok := userID.(uuid.UUID)
	return id, ok
}

func GetTenantID(c *gin.Context) string {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "default"
	}
	id, ok := tenantID.(string)
	if !ok {
		return "default"
	}
	return id
}

func GetScopes(c *gin.Context) []string {
	scopes, exists := c.Get("scopes")
	if !exists {
		return nil
	}
	s, ok := scopes.([]string)
	if !ok {
		return nil
	}
	return s
}

func HasScope(c *gin.Context, scope string) bool {
	return hasScope(GetScopes(c), scope)
}
