package http

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	pools := r.Group("/agent/pools")
	{
		pools.GET("", h.ListPools)
		pools.GET("/:poolId", h.GetPool)
		pools.GET("/:poolId/tools", h.GetPoolTools)
	}

	tools := r.Group("/agent/tools")
	{
		tools.GET("", h.ListAvailableTools)
		tools.GET("/:toolName/permission", h.GetToolPermission)
	}
}
