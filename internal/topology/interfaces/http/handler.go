package http

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud-agent-monitor/internal/topology/application"
	"cloud-agent-monitor/internal/topology/domain"
	"cloud-agent-monitor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	MaxDepth       = 20
	MaxHops        = 30
	DefaultDepth   = 5
	DefaultHops    = 10
	RequestTimeout = 30 * time.Second
)

type Handler struct {
	service *application.TopologyService
}

func NewHandler(service *application.TopologyService) *Handler {
	return &Handler{service: service}
}

type GetServiceTopologyRequest struct {
	Namespace   string `form:"namespace"`
	ServiceName string `form:"service_name"`
}

type GetNetworkTopologyRequest struct {
	Namespace string `form:"namespace"`
}

type FindPathRequest struct {
	SourceID string `form:"source_id" binding:"required"`
	TargetID string `form:"target_id" binding:"required"`
	MaxHops  int    `form:"max_hops"`
}

type AnalyzeImpactRequest struct {
	MaxDepth int `form:"max_depth"`
}

type GetTopologyAtTimeRequest struct {
	Timestamp string `form:"timestamp" binding:"required"`
}

type GetTopologyChangesRequest struct {
	From string `form:"from" binding:"required"`
	To   string `form:"to" binding:"required"`
}

func (h *Handler) GetServiceTopology(c *gin.Context) {
	var req GetServiceTopologyRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	query := domain.TopologyQuery{
		Namespace:   req.Namespace,
		ServiceName: req.ServiceName,
	}

	topology, err := h.service.GetServiceTopology(c.Request.Context(), query)
	if err != nil {
		response.InternalError(c, "failed to get service topology: "+err.Error())
		return
	}

	response.Success(c, topology)
}

func (h *Handler) GetServiceNode(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.InvalidRequest(c, "invalid service ID format")
		return
	}

	node, err := h.service.GetServiceNode(c.Request.Context(), id)
	if err != nil {
		if err == domain.ErrNodeNotFound {
			response.NotFoundDefault(c, "service node not found")
			return
		}
		response.InternalError(c, "failed to get service node: "+err.Error())
		return
	}

	response.Success(c, node)
}

func (h *Handler) GetServiceNodeByName(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	if namespace == "" || name == "" {
		response.InvalidRequest(c, "namespace and name are required")
		return
	}

	node, err := h.service.GetServiceNodeByName(c.Request.Context(), namespace, name)
	if err != nil {
		if err == domain.ErrNodeNotFound {
			response.NotFoundDefault(c, "service node not found")
			return
		}
		response.InternalError(c, "failed to get service node: "+err.Error())
		return
	}

	response.Success(c, node)
}

func (h *Handler) GetUpstreamServices(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.InvalidRequest(c, "invalid service ID format")
		return
	}

	depthStr := c.DefaultQuery("depth", strconv.Itoa(DefaultDepth))
	depth, err := strconv.Atoi(depthStr)
	if err != nil {
		response.InvalidRequest(c, "invalid depth value")
		return
	}
	if depth < 1 || depth > MaxDepth {
		response.InvalidRequest(c, fmt.Sprintf("depth must be between 1 and %d", MaxDepth))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	nodes, err := h.service.GetUpstreamServices(ctx, id, depth)
	if err != nil {
		response.InternalError(c, "failed to get upstream services: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"service_id": id,
		"depth":      depth,
		"upstream":   nodes,
		"count":      len(nodes),
	})
}

func (h *Handler) GetDownstreamServices(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.InvalidRequest(c, "invalid service ID format")
		return
	}

	depthStr := c.DefaultQuery("depth", strconv.Itoa(DefaultDepth))
	depth, err := strconv.Atoi(depthStr)
	if err != nil {
		response.InvalidRequest(c, "invalid depth value")
		return
	}
	if depth < 1 || depth > MaxDepth {
		response.InvalidRequest(c, fmt.Sprintf("depth must be between 1 and %d", MaxDepth))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	nodes, err := h.service.GetDownstreamServices(ctx, id, depth)
	if err != nil {
		response.InternalError(c, "failed to get downstream services: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"service_id": id,
		"depth":      depth,
		"downstream": nodes,
		"count":      len(nodes),
	})
}

func (h *Handler) AnalyzeImpact(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.InvalidRequest(c, "invalid service ID format")
		return
	}

	var req AnalyzeImpactRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultDepth
	}
	if maxDepth > MaxDepth {
		response.InvalidRequest(c, fmt.Sprintf("max_depth must not exceed %d", MaxDepth))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	result, err := h.service.AnalyzeImpact(ctx, id, maxDepth)
	if err != nil {
		if err == domain.ErrNodeNotFound {
			response.NotFoundDefault(c, "service node not found")
			return
		}
		response.InternalError(c, "failed to analyze impact: "+err.Error())
		return
	}

	response.Success(c, result)
}

func (h *Handler) GetNetworkTopology(c *gin.Context) {
	var req GetNetworkTopologyRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	query := domain.TopologyQuery{
		Namespace: req.Namespace,
	}

	topology, err := h.service.GetNetworkTopology(c.Request.Context(), query)
	if err != nil {
		response.InternalError(c, "failed to get network topology: "+err.Error())
		return
	}

	response.Success(c, topology)
}

func (h *Handler) GetNetworkNode(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.InvalidRequest(c, "invalid network node ID format")
		return
	}

	node, err := h.service.GetNetworkNode(c.Request.Context(), id)
	if err != nil {
		if err == domain.ErrNodeNotFound {
			response.NotFoundDefault(c, "network node not found")
			return
		}
		response.InternalError(c, "failed to get network node: "+err.Error())
		return
	}

	response.Success(c, node)
}

func (h *Handler) FindPath(c *gin.Context) {
	var req FindPathRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		response.InvalidRequest(c, "invalid source ID format")
		return
	}

	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		response.InvalidRequest(c, "invalid target ID format")
		return
	}

	maxHops := req.MaxHops
	if maxHops <= 0 {
		maxHops = DefaultHops
	}
	if maxHops > MaxHops {
		response.InvalidRequest(c, fmt.Sprintf("max_hops must not exceed %d", MaxHops))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	result, err := h.service.FindPath(ctx, sourceID, targetID, maxHops)
	if err != nil {
		if err == domain.ErrPathNotFound {
			response.NotFoundDefault(c, "no path found between the specified services")
			return
		}
		response.InternalError(c, "failed to find path: "+err.Error())
		return
	}

	response.Success(c, result)
}

func (h *Handler) FindShortestPath(c *gin.Context) {
	sourceIDStr := c.Query("source_id")
	targetIDStr := c.Query("target_id")

	if sourceIDStr == "" || targetIDStr == "" {
		response.InvalidRequest(c, "source_id and target_id are required")
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		response.InvalidRequest(c, "invalid source ID format")
		return
	}

	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		response.InvalidRequest(c, "invalid target ID format")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	path, err := h.service.FindShortestPath(ctx, sourceID, targetID)
	if err != nil {
		if err == domain.ErrPathNotFound {
			response.NotFoundDefault(c, "no path found between the specified services")
			return
		}
		response.InternalError(c, "failed to find shortest path: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"source_id": sourceID,
		"target_id": targetID,
		"path":      path,
		"hops":      len(path),
	})
}

func (h *Handler) FindAnomalies(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	anomalies, err := h.service.FindAnomalies(ctx)
	if err != nil {
		response.InternalError(c, "failed to find anomalies: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"anomalies": anomalies,
		"count":     len(anomalies),
	})
}

func (h *Handler) GetTopologyAtTime(c *gin.Context) {
	var req GetTopologyAtTimeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	timestamp, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		response.InvalidRequest(c, "invalid timestamp format, use RFC3339")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	topology, err := h.service.GetTopologyAtTime(ctx, timestamp)
	if err != nil {
		if err == domain.ErrGraphNotReady {
			response.NotFoundDefault(c, "no topology snapshot found for the specified time")
			return
		}
		response.InternalError(c, "failed to get topology at time: "+err.Error())
		return
	}

	response.Success(c, topology)
}

func (h *Handler) GetTopologyChanges(c *gin.Context) {
	var req GetTopologyChangesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		response.InvalidRequest(c, "invalid 'from' timestamp format, use RFC3339")
		return
	}

	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		response.InvalidRequest(c, "invalid 'to' timestamp format, use RFC3339")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	changes, err := h.service.GetTopologyChanges(ctx, from, to)
	if err != nil {
		response.InternalError(c, "failed to get topology changes: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"from":    from,
		"to":      to,
		"changes": changes,
		"count":   len(changes),
	})
}

func (h *Handler) GetTopologyStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), RequestTimeout)
	defer cancel()

	stats, err := h.service.GetTopologyStats(ctx)
	if err != nil {
		response.InternalError(c, "failed to get topology stats: "+err.Error())
		return
	}

	response.Success(c, stats)
}

func (h *Handler) RefreshTopology(c *gin.Context) {
	topologyType := c.DefaultQuery("type", "all")

	switch topologyType {
	case "service":
		if err := h.service.RefreshServiceTopology(c.Request.Context()); err != nil {
			response.InternalError(c, "failed to refresh service topology: "+err.Error())
			return
		}
		response.Success(c, gin.H{"message": "service topology refreshed"})

	case "network":
		if err := h.service.RefreshNetworkTopology(c.Request.Context()); err != nil {
			response.InternalError(c, "failed to refresh network topology: "+err.Error())
			return
		}
		response.Success(c, gin.H{"message": "network topology refreshed"})

	case "all":
		var errors []string

		if err := h.service.RefreshServiceTopology(c.Request.Context()); err != nil {
			errors = append(errors, "service: "+err.Error())
		}

		if err := h.service.RefreshNetworkTopology(c.Request.Context()); err != nil {
			errors = append(errors, "network: "+err.Error())
		}

		if len(errors) > 0 {
			response.InternalError(c, "partial refresh failure: "+string(joinErrors(errors)))
			return
		}

		response.Success(c, gin.H{"message": "all topology refreshed"})

	default:
		response.InvalidRequest(c, "invalid topology type, use 'service', 'network', or 'all'")
	}
}

func joinErrors(errors []string) []byte {
	var result []byte
	for i, err := range errors {
		if i > 0 {
			result = append(result, "; "...)
		}
		result = append(result, err...)
	}
	return result
}

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	topology := r.Group("/topology")
	{
		topology.GET("/services", h.GetServiceTopology)
		topology.GET("/services/:id", h.GetServiceNode)
		topology.GET("/services/:id/upstream", h.GetUpstreamServices)
		topology.GET("/services/:id/downstream", h.GetDownstreamServices)
		topology.GET("/services/:id/impact", h.AnalyzeImpact)
		topology.GET("/services/ns/:namespace/:name", h.GetServiceNodeByName)

		topology.GET("/network", h.GetNetworkTopology)
		topology.GET("/network/:id", h.GetNetworkNode)

		topology.GET("/path", h.FindPath)
		topology.GET("/path/shortest", h.FindShortestPath)

		topology.GET("/anomalies", h.FindAnomalies)

		topology.GET("/history", h.GetTopologyAtTime)
		topology.GET("/changes", h.GetTopologyChanges)

		topology.GET("/stats", h.GetTopologyStats)

		topology.POST("/refresh", h.RefreshTopology)
	}
}
