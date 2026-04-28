package http

import (
	"net/http"

	agentDomain "cloud-agent-monitor/internal/agent/domain"
	"cloud-agent-monitor/internal/agent/infrastructure/eino"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	toolRegistry *eino.ToolRegistry
	poolRegistry *eino.PoolRegistry
}

func NewHandler(toolRegistry *eino.ToolRegistry, poolRegistry *eino.PoolRegistry) *Handler {
	return &Handler{
		toolRegistry: toolRegistry,
		poolRegistry: poolRegistry,
	}
}

func (h *Handler) ListPools(c *gin.Context) {
	pools := h.poolRegistry.ListPools(c.Request.Context())

	userID := eino.GetUserIDFromContext(c.Request.Context())
	type poolWithPermission struct {
		*agentDomain.ToolPool
		UserCanAccess bool `json:"user_can_access"`
	}

	result := make([]poolWithPermission, 0, len(pools))
	for _, pool := range pools {
		item := poolWithPermission{
			ToolPool:      pool,
			UserCanAccess: userID != "",
		}
		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{"pools": result})
}

func (h *Handler) GetPool(c *gin.Context) {
	poolID := c.Param("poolId")

	pool, ok := h.poolRegistry.GetPool(poolID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pool not found"})
		return
	}

	c.JSON(http.StatusOK, pool)
}

func (h *Handler) GetPoolTools(c *gin.Context) {
	poolID := c.Param("poolId")

	tools, err := h.poolRegistry.GetToolsForPool(c.Request.Context(), poolID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pool_id":     poolID,
		"tools_count": len(tools),
	})
}

func (h *Handler) ListAvailableTools(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		userID = eino.GetUserIDFromContext(c.Request.Context())
	}

	tools, err := h.toolRegistry.ListAvailableTools(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tools": tools})
}

func (h *Handler) GetToolPermission(c *gin.Context) {
	toolName := c.Param("toolName")
	userID := c.GetString("user_id")

	tool, ok := h.toolRegistry.GetTool(toolName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}

	_ = tool

	c.JSON(http.StatusOK, gin.H{
		"tool_name": toolName,
		"user_id":   userID,
	})
}
