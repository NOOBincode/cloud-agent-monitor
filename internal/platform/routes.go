package platform

import (
	"cloud-agent-monitor/internal/storage"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, repo storage.ServiceRepositoryInterface) {
	h := NewServiceHandler(repo)
	registerRoutes(r, h)
}

func RegisterRoutesWithHealthCheck(r *gin.RouterGroup, repo storage.ServiceRepositoryInterface, healthCheckService *HealthCheckService) {
	h := NewServiceHandlerWithHealthCheck(repo, healthCheckService)
	registerRoutes(r, h)
}

func registerRoutes(r *gin.RouterGroup, h *ServiceHandler) {
	services := r.Group("/services")
	{
		services.GET("", h.List)
		services.POST("", h.Create)
		services.POST("/batch", h.BatchCreate)
		services.PUT("/batch", h.BatchUpdate)
		services.DELETE("/batch", h.BatchDelete)
		services.GET("/search", h.Search)
		services.GET("/health", h.GetHealth)
		services.GET("/health/summary", h.GetHealthSummary)
		services.GET("/labels", h.GetLabelKeys)
		services.GET("/labels/:key", h.GetLabelValues)
		services.GET("/dependencies", h.GetDependencyGraph)
		services.GET("/:id", h.Get)
		services.PUT("/:id", h.Update)
		services.DELETE("/:id", h.Delete)
		services.GET("/:id/health", h.GetServiceHealth)
		services.POST("/:id/health/check", h.CheckHealthNow)
		services.PUT("/:id/labels", h.UpdateLabels)
		services.GET("/:id/openapi", h.GetOpenAPI)
		services.PUT("/:id/openapi", h.UpdateOpenAPI)
		services.GET("/:id/dependencies", h.GetDependencies)
		services.POST("/:id/dependencies", h.AddDependency)
		services.DELETE("/:id/dependencies/:dep_id", h.RemoveDependency)
		services.GET("/:id/dependents", h.GetDependents)
	}
}
