package user

import (
	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/storage"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler, authMiddleware *auth.AuthMiddleware) {
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", handler.Register)
		authGroup.POST("/login", handler.Login)
		authGroup.POST("/refresh", handler.RefreshToken)
		authGroup.POST("/forgot-password", handler.ForgotPassword)
		authGroup.POST("/reset-password", handler.ResetPassword)
	}

	users := r.Group("/users")
	users.Use(authMiddleware.RequireAPIKey())
	{
		users.GET("/me", handler.GetCurrentUser)
		users.PUT("/me", handler.UpdateProfile)
		users.GET("/me/login-logs", handler.GetLoginLogs)
	}

	apiKeys := r.Group("/api-keys")
	apiKeys.Use(authMiddleware.RequireAPIKey())
	{
		apiKeys.GET("", handler.ListAPIKeys)
		apiKeys.POST("", handler.CreateAPIKey)
		apiKeys.DELETE("/:id", handler.RevokeAPIKey)
	}
}

func RegisterAdminRoutes(r *gin.RouterGroup, handler *Handler, authMiddleware *auth.AuthMiddleware) {
	users := r.Group("/users")
	users.Use(authMiddleware.RequireAPIKey())
	{
		users.GET("", handler.ListUsers)
		users.GET("/:id", handler.GetCurrentUser)
		users.PUT("/:id/status", handler.SetUserStatus)
		users.GET("/:id/roles", handler.GetUserRoles)
		users.POST("/:id/roles", handler.AssignRole)
		users.DELETE("/:id/roles/:role_id", handler.RemoveRole)
	}

	roles := r.Group("/roles")
	roles.Use(authMiddleware.RequireAPIKey())
	{
		roles.GET("", handler.ListRoles)
	}
}

func RegisterRoutesWithRole(r *gin.RouterGroup, handler *Handler, authMiddleware *auth.AuthMiddleware, roleRepo storage.RoleRepositoryInterface) {
	h := NewHandlerWithRole(handler.userService, handler.apiKeyService, roleRepo)

	RegisterRoutes(r, h, authMiddleware)
	RegisterAdminRoutes(r, h, authMiddleware)
}
