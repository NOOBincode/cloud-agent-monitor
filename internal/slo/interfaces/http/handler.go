package http

import (
	"cloud-agent-monitor/internal/slo/application"
	"cloud-agent-monitor/internal/slo/domain"
	"cloud-agent-monitor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *application.SLOService
}

func NewHandler(service *application.SLOService) *Handler {
	return &Handler{service: service}
}

type CreateSLORequest struct {
	Name         string           `json:"name" binding:"required"`
	Description  string           `json:"description"`
	ServiceID    uuid.UUID        `json:"service_id" binding:"required"`
	Target       float64          `json:"target" binding:"required,min=0,max=100"`
	Window       string           `json:"window" binding:"required"`
	WarningBurn  float64          `json:"warning_burn"`
	CriticalBurn float64          `json:"critical_burn"`
	SLI          CreateSLIRequest `json:"sli" binding:"required"`
}

type CreateSLIRequest struct {
	Name        string         `json:"name" binding:"required"`
	Type        domain.SLIType `json:"type" binding:"required"`
	Query       string         `json:"query" binding:"required"`
	Threshold   float64        `json:"threshold"`
	Unit        string         `json:"unit"`
	Description string         `json:"description"`
}

type UpdateSLORequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Target       float64 `json:"target" binding:"omitempty,min=0,max=100"`
	Window       string  `json:"window"`
	WarningBurn  float64 `json:"warning_burn"`
	CriticalBurn float64 `json:"critical_burn"`
}

func (h *Handler) CreateSLO(c *gin.Context) {
	var req CreateSLORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	slo := domain.NewSLO(req.Name, req.ServiceID, req.Target, req.Window)
	slo.Description = req.Description
	if req.WarningBurn > 0 {
		slo.WarningBurn = req.WarningBurn
	}
	if req.CriticalBurn > 0 {
		slo.CriticalBurn = req.CriticalBurn
	}

	sli := domain.NewSLI(uuid.Nil, req.SLI.Name, req.SLI.Type, req.SLI.Query)
	sli.Threshold = req.SLI.Threshold
	sli.Unit = req.SLI.Unit
	sli.Description = req.SLI.Description

	created, err := h.service.CreateSLO(c.Request.Context(), slo, sli)
	if err != nil {
		response.InternalError(c, "failed to create SLO: "+err.Error())
		return
	}

	response.Created(c, created)
}

func (h *Handler) UpdateSLO(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.InvalidRequest(c, "invalid SLO ID")
		return
	}

	var req UpdateSLORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	slo := &domain.SLO{
		ID:           id,
		Name:         req.Name,
		Description:  req.Description,
		Target:       req.Target,
		Window:       req.Window,
		WarningBurn:  req.WarningBurn,
		CriticalBurn: req.CriticalBurn,
	}

	updated, err := h.service.UpdateSLO(c.Request.Context(), slo)
	if err != nil {
		response.InternalError(c, "failed to update SLO: "+err.Error())
		return
	}

	response.Success(c, updated)
}

func (h *Handler) DeleteSLO(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.InvalidRequest(c, "invalid SLO ID")
		return
	}

	if err := h.service.DeleteSLO(c.Request.Context(), id); err != nil {
		response.InternalError(c, "failed to delete SLO: "+err.Error())
		return
	}

	response.Success(c, nil)
}

func (h *Handler) GetSLO(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.InvalidRequest(c, "invalid SLO ID")
		return
	}

	slo, err := h.service.GetSLO(c.Request.Context(), id)
	if err != nil {
		response.NotFoundDefault(c, "SLO not found")
		return
	}

	response.Success(c, slo)
}

func (h *Handler) ListSLOs(c *gin.Context) {
	var filter domain.SLOFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	slos, total, err := h.service.ListSLOs(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "failed to list SLOs: "+err.Error())
		return
	}

	response.Paged(c, slos, total, filter.Page, filter.PageSize)
}

func (h *Handler) GetSLOSummary(c *gin.Context) {
	summary, err := h.service.GetSLOSummary(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to get SLO summary: "+err.Error())
		return
	}

	response.Success(c, summary)
}

func (h *Handler) GetSLOsByService(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("service_id"))
	if err != nil {
		response.InvalidRequest(c, "invalid service ID")
		return
	}

	slos, err := h.service.GetSLOByService(c.Request.Context(), serviceID)
	if err != nil {
		response.InternalError(c, "failed to get SLOs: "+err.Error())
		return
	}

	response.Success(c, slos)
}

func (h *Handler) RefreshSLOStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.InvalidRequest(c, "invalid SLO ID")
		return
	}

	slo, err := h.service.RefreshSLOStatus(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to refresh SLO status: "+err.Error())
		return
	}

	response.Success(c, slo)
}

func (h *Handler) RefreshAllSLOStatus(c *gin.Context) {
	if err := h.service.RefreshAllSLOStatus(c.Request.Context()); err != nil {
		response.InternalError(c, "failed to refresh all SLO status: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "SLO status refresh completed"})
}

func (h *Handler) GetErrorBudget(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.InvalidRequest(c, "invalid SLO ID")
		return
	}

	budget, err := h.service.GetErrorBudget(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to get error budget: "+err.Error())
		return
	}

	response.Success(c, budget)
}

func (h *Handler) GetBurnRateAlerts(c *gin.Context) {
	alerts, err := h.service.GetBurnRateAlerts(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to get burn rate alerts: "+err.Error())
		return
	}

	response.Success(c, alerts)
}

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	slos := r.Group("/slos")
	{
		slos.POST("", h.CreateSLO)
		slos.GET("", h.ListSLOs)
		slos.GET("/summary", h.GetSLOSummary)
		slos.GET("/burn-rate-alerts", h.GetBurnRateAlerts)
		slos.POST("/refresh", h.RefreshAllSLOStatus)

		slos.GET("/:id", h.GetSLO)
		slos.PUT("/:id", h.UpdateSLO)
		slos.DELETE("/:id", h.DeleteSLO)
		slos.POST("/:id/refresh", h.RefreshSLOStatus)
		slos.GET("/:id/error-budget", h.GetErrorBudget)

		slos.GET("/service/:service_id", h.GetSLOsByService)
	}
}
