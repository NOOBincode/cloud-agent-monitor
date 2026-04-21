package middleware

import (
	"context"
	"strings"

	"cloud-agent-monitor/internal/agent/infrastructure/eino"

	"github.com/gin-gonic/gin"
)

type PermissionService interface {
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	ValidateToken(ctx context.Context, token string) (string, error)
}

type EinoAuthMiddleware struct {
	ps           PermissionService
	toolRegistry *eino.ToolRegistry
}

func NewEinoAuthMiddleware(ps PermissionService, registry *eino.ToolRegistry) *EinoAuthMiddleware {
	return &EinoAuthMiddleware{
		ps:           ps,
		toolRegistry: registry,
	}
}

func (m *EinoAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"error": "missing authorization header",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(401, gin.H{
				"error": "invalid authorization format, expected Bearer token",
			})
			return
		}

		userID, err := m.ps.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		ctx := eino.ContextWithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)
		c.Set("user_id", userID)

		c.Next()
	}
}

func (m *EinoAuthMiddleware) ListAvailableTools() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(401, gin.H{
				"error": "user not authenticated",
			})
			return
		}

		tools, err := m.toolRegistry.ListAvailableTools(c.Request.Context(), userID.(string))
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{
				"error": "failed to list tools",
			})
			return
		}

		c.JSON(200, gin.H{
			"tools": tools,
		})
	}
}

func (m *EinoAuthMiddleware) ExecuteTool() gin.HandlerFunc {
	return func(c *gin.Context) {
		toolName := c.Param("name")
		if toolName == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "tool name is required",
			})
			return
		}

		var req struct {
			Arguments string `json:"arguments"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithStatusJSON(400, gin.H{
				"error": "invalid request body",
			})
			return
		}

		result, err := m.toolRegistry.Execute(c.Request.Context(), toolName, req.Arguments)
		if err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				c.AbortWithStatusJSON(403, gin.H{
					"error": err.Error(),
				})
				return
			}
			if strings.Contains(err.Error(), "authentication required") {
				c.AbortWithStatusJSON(401, gin.H{
					"error": err.Error(),
				})
				return
			}
			c.AbortWithStatusJSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"result": result,
		})
	}
}
