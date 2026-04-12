package http

import (
	"errors"
	"strconv"
	"time"

	"cloud-agent-monitor/internal/alerting/application"
	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/model"
	"cloud-agent-monitor/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service application.AlertServiceInterface
}

func NewHandler(service application.AlertServiceInterface) *Handler {
	return &Handler{service: service}
}

type SendAlertRequest struct {
	Labels       map[string]string `json:"labels" binding:"required"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     *time.Time        `json:"starts_at,omitempty"`
	EndsAt       *time.Time        `json:"ends_at,omitempty"`
	GeneratorURL string            `json:"generator_url,omitempty"`
}

type SendAlertsRequest struct {
	Alerts []SendAlertRequest `json:"alerts" binding:"required,min=1"`
}

type CreateSilenceRequest struct {
	Matchers  []domain.Matcher `json:"matchers" binding:"required,min=1"`
	StartsAt  time.Time        `json:"starts_at" binding:"required"`
	EndsAt    time.Time        `json:"ends_at" binding:"required"`
	CreatedBy string           `json:"created_by" binding:"required"`
	Comment   string           `json:"comment"`
}

type AlertResponse struct {
	Labels       map[string]string  `json:"labels"`
	Annotations  map[string]string  `json:"annotations"`
	StartsAt     time.Time          `json:"starts_at"`
	EndsAt       *time.Time         `json:"ends_at,omitempty"`
	Status       domain.AlertStatus `json:"status"`
	GeneratorURL string             `json:"generator_url,omitempty"`
}

type SilenceResponse struct {
	ID        string               `json:"id"`
	Matchers  []domain.Matcher     `json:"matchers"`
	StartsAt  time.Time            `json:"starts_at"`
	EndsAt    time.Time            `json:"ends_at"`
	CreatedBy string               `json:"created_by"`
	Comment   string               `json:"comment"`
	Status    domain.SilenceStatus `json:"status"`
}

type OperationResponse struct {
	ID            uuid.UUID         `json:"id"`
	OperationType string            `json:"operation_type"`
	Fingerprint   string            `json:"fingerprint"`
	Labels        map[string]string `json:"labels"`
	Status        string            `json:"status"`
	IPAddress     string            `json:"ip_address"`
	UserAgent     string            `json:"user_agent"`
	CreatedAt     time.Time         `json:"created_at"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
}

type AlertRecordResponse struct {
	ID          uuid.UUID            `json:"id"`
	Fingerprint string               `json:"fingerprint"`
	Labels      map[string]string    `json:"labels"`
	Annotations map[string]string    `json:"annotations"`
	Status      models.AlertStatus   `json:"status"`
	Severity    models.AlertSeverity `json:"severity"`
	StartsAt    time.Time            `json:"starts_at"`
	EndsAt      *time.Time           `json:"ends_at,omitempty"`
	Duration    int64                `json:"duration_seconds"`
	Source      string               `json:"source"`
	CreatedAt   time.Time            `json:"created_at"`
}

type AlertRecordStatsResponse struct {
	TotalCount    int64 `json:"total_count"`
	FiringCount   int64 `json:"firing_count"`
	ResolvedCount int64 `json:"resolved_count"`
	CriticalCount int64 `json:"critical_count"`
	WarningCount  int64 `json:"warning_count"`
	InfoCount     int64 `json:"info_count"`
	AvgDuration   int64 `json:"avg_duration_seconds"`
}

type NoisyAlertResponse struct {
	Fingerprint     string            `json:"fingerprint"`
	Labels          map[string]string `json:"labels"`
	NoiseScore      float64           `json:"noise_score"`
	OccurrenceCount int64             `json:"occurrence_count"`
	LastOccurredAt  time.Time         `json:"last_occurred_at"`
	RiskLevel       string            `json:"risk_level"`
}

func (h *Handler) SendAlert(c *gin.Context) {
	var req SendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	alert := domain.NewAlert(req.Labels, req.Annotations)
	if req.StartsAt != nil {
		alert.StartsAt = *req.StartsAt
	}
	if req.EndsAt != nil {
		alert.EndsAt = req.EndsAt
	}
	alert.GeneratorURL = req.GeneratorURL

	if err := h.service.SendAlert(c.Request.Context(), alert); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "alert sent successfully", gin.H{"alert_id": alert.ID})
}

func (h *Handler) SendAlerts(c *gin.Context) {
	var req SendAlertsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	var alerts []*domain.Alert
	for _, a := range req.Alerts {
		alert := domain.NewAlert(a.Labels, a.Annotations)
		if a.StartsAt != nil {
			alert.StartsAt = *a.StartsAt
		}
		if a.EndsAt != nil {
			alert.EndsAt = a.EndsAt
		}
		alert.GeneratorURL = a.GeneratorURL
		alerts = append(alerts, alert)
	}

	if err := h.service.SendAlerts(c.Request.Context(), alerts); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "alerts sent successfully", gin.H{"count": len(alerts)})
}

func (h *Handler) SendAlertWithRecord(c *gin.Context) {
	var req SendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	alert := domain.NewAlert(req.Labels, req.Annotations)
	if req.StartsAt != nil {
		alert.StartsAt = *req.StartsAt
	}
	if req.EndsAt != nil {
		alert.EndsAt = req.EndsAt
	}
	alert.GeneratorURL = req.GeneratorURL

	op, err := h.service.SendAlertWithRecord(
		c.Request.Context(),
		alert,
		userID.(uuid.UUID),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		if errors.Is(err, application.ErrDuplicateOperation) {
			response.Conflict(c, "duplicate operation")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, gin.H{
		"alert_id":     alert.ID,
		"operation_id": op.ID,
		"fingerprint":  op.AlertFingerprint,
	})
}

func (h *Handler) GetAlerts(c *gin.Context) {
	var filter domain.AlertFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	alerts, err := h.service.GetAlerts(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]AlertResponse, len(alerts))
	for i, a := range alerts {
		resp[i] = AlertResponse{
			Labels:       a.Labels,
			Annotations:  a.Annotations,
			StartsAt:     a.StartsAt,
			EndsAt:       a.EndsAt,
			Status:       a.Status,
			GeneratorURL: a.GeneratorURL,
		}
	}

	response.Success(c, resp)
}

func (h *Handler) GetAlert(c *gin.Context) {
	fingerprint := c.Param("fingerprint")
	if fingerprint == "" {
		response.InvalidRequest(c, "fingerprint is required")
		return
	}

	alert, err := h.service.GetAlert(c.Request.Context(), fingerprint)
	if err != nil {
		if errors.Is(err, application.ErrAlertNotFound) {
			response.NotFound(c, model.CodeAlertNotFound, "alert not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	resp := AlertResponse{
		Labels:       alert.Labels,
		Annotations:  alert.Annotations,
		StartsAt:     alert.StartsAt,
		EndsAt:       alert.EndsAt,
		Status:       alert.Status,
		GeneratorURL: alert.GeneratorURL,
	}

	response.Success(c, resp)
}

func (h *Handler) CreateSilence(c *gin.Context) {
	var req CreateSilenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	silence := domain.NewSilence(req.Matchers, req.StartsAt, req.EndsAt, req.CreatedBy, req.Comment)

	silenceID, err := h.service.CreateSilence(c.Request.Context(), silence)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, gin.H{"silence_id": silenceID})
}

func (h *Handler) CreateSilenceWithRecord(c *gin.Context) {
	var req CreateSilenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	silence := domain.NewSilence(req.Matchers, req.StartsAt, req.EndsAt, req.CreatedBy, req.Comment)

	op, silenceID, err := h.service.CreateSilenceWithRecord(
		c.Request.Context(),
		silence,
		userID.(uuid.UUID),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		if errors.Is(err, application.ErrDuplicateOperation) {
			response.Conflict(c, "duplicate operation")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, gin.H{
		"silence_id":   silenceID,
		"operation_id": op.ID,
	})
}

func (h *Handler) GetSilences(c *gin.Context) {
	silences, err := h.service.GetSilences(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]SilenceResponse, len(silences))
	for i, s := range silences {
		resp[i] = SilenceResponse{
			ID:        s.ID.String(),
			Matchers:  s.Matchers,
			StartsAt:  s.StartsAt,
			EndsAt:    s.EndsAt,
			CreatedBy: s.CreatedBy,
			Comment:   s.Comment,
			Status:    s.Status,
		}
	}

	response.Success(c, resp)
}

func (h *Handler) DeleteSilence(c *gin.Context) {
	silenceID := c.Param("id")
	if silenceID == "" {
		response.InvalidRequest(c, "silence_id is required")
		return
	}

	err := h.service.DeleteSilence(c.Request.Context(), silenceID)
	if err != nil {
		if errors.Is(err, application.ErrSilenceNotFound) {
			response.NotFound(c, model.CodeSilenceNotFound, "silence not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "silence deleted", gin.H{"silence_id": silenceID})
}

func (h *Handler) DeleteSilenceWithRecord(c *gin.Context) {
	silenceID := c.Param("id")
	if silenceID == "" {
		response.InvalidRequest(c, "silence_id is required")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	op, err := h.service.DeleteSilenceWithRecord(
		c.Request.Context(),
		silenceID,
		userID.(uuid.UUID),
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		if errors.Is(err, application.ErrDuplicateOperation) {
			response.Conflict(c, "duplicate operation")
			return
		}
		if errors.Is(err, application.ErrSilenceNotFound) {
			response.NotFound(c, model.CodeSilenceNotFound, "silence not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.SuccessWithMessage(c, "silence deleted", gin.H{
		"silence_id":   silenceID,
		"operation_id": op.ID,
	})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	if err := h.service.HealthCheck(c.Request.Context()); err != nil {
		response.ServiceUnavailable(c, err.Error())
		return
	}
	response.Success(c, gin.H{"status": "healthy"})
}

func (h *Handler) GetStatus(c *gin.Context) {
	status, err := h.service.GetAlertmanagerStatus(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, status)
}

func (h *Handler) GetAlertRecords(c *gin.Context) {
	var filter models.AlertRecordFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.InvalidRequest(c, err.Error())
		return
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	records, total, err := h.service.GetAlertRecords(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]AlertRecordResponse, len(records))
	for i, r := range records {
		resp[i] = AlertRecordResponse{
			ID:          r.ID,
			Fingerprint: r.Fingerprint,
			Labels:      r.Labels,
			Annotations: r.Annotations,
			Status:      r.Status,
			Severity:    r.Severity,
			StartsAt:    r.StartsAt,
			EndsAt:      r.EndsAt,
			Duration:    r.Duration,
			Source:      r.Source,
			CreatedAt:   r.CreatedAt,
		}
	}

	response.Success(c, gin.H{
		"records": resp,
		"total":   total,
		"page":    filter.Page,
		"size":    filter.PageSize,
	})
}

func (h *Handler) GetAlertRecordStats(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	stats, err := h.service.GetAlertRecordStats(c.Request.Context(), from, to)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := AlertRecordStatsResponse{
		TotalCount:    stats.TotalCount,
		FiringCount:   stats.FiringCount,
		ResolvedCount: stats.ResolvedCount,
		CriticalCount: stats.CriticalCount,
		WarningCount:  stats.WarningCount,
		InfoCount:     stats.InfoCount,
		AvgDuration:   stats.AvgDuration,
	}

	response.Success(c, resp)
}

func (h *Handler) GetOperationHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	ops, total, err := h.service.GetOperationHistory(c.Request.Context(), userID.(uuid.UUID), limit, offset)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]OperationResponse, len(ops))
	for i, op := range ops {
		resp[i] = OperationResponse{
			ID:            op.ID,
			OperationType: string(op.OperationType),
			Fingerprint:   op.AlertFingerprint,
			Labels:        op.AlertLabels,
			Status:        string(op.Status),
			IPAddress:     op.IPAddress,
			UserAgent:     op.UserAgent,
			CreatedAt:     op.CreatedAt,
			ProcessedAt:   op.ProcessedAt,
		}
	}

	response.Success(c, gin.H{
		"operations": resp,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

func (h *Handler) GetNoisyAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	records, err := h.service.GetNoisyAlerts(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]NoisyAlertResponse, len(records))
	for i, r := range records {
		riskLevel := "low"
		if r.NoiseScore >= 0.7 {
			riskLevel = "high"
		} else if r.NoiseScore >= 0.4 {
			riskLevel = "medium"
		}

		resp[i] = NoisyAlertResponse{
			Fingerprint:     r.AlertFingerprint,
			Labels:          r.AlertLabels,
			NoiseScore:      r.NoiseScore,
			OccurrenceCount: int64(r.FireCount),
			LastOccurredAt:  r.LastFiredAt,
			RiskLevel:       riskLevel,
		}
	}

	response.Success(c, resp)
}

func (h *Handler) GetHighRiskAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	records, err := h.service.GetHighRiskAlerts(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	resp := make([]NoisyAlertResponse, len(records))
	for i, r := range records {
		riskLevel := "low"
		if r.NoiseScore >= 0.7 {
			riskLevel = "high"
		} else if r.NoiseScore >= 0.4 {
			riskLevel = "medium"
		}

		resp[i] = NoisyAlertResponse{
			Fingerprint:     r.AlertFingerprint,
			Labels:          r.AlertLabels,
			NoiseScore:      r.NoiseScore,
			OccurrenceCount: int64(r.FireCount),
			LastOccurredAt:  r.LastFiredAt,
			RiskLevel:       riskLevel,
		}
	}

	response.Success(c, resp)
}

func (h *Handler) GetAlertFeedback(c *gin.Context) {
	fingerprint := c.Param("fingerprint")
	if fingerprint == "" {
		response.InvalidRequest(c, "fingerprint is required")
		return
	}

	feedback, err := h.service.GetAlertFeedback(c.Request.Context(), fingerprint)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, feedback)
}

type WebhookAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       *time.Time        `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

type WebhookMessage struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []WebhookAlert    `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts,omitempty"`
}

func (h *Handler) ReceiveWebhook(c *gin.Context) {
	var msg WebhookMessage
	if err := c.ShouldBindJSON(&msg); err != nil {
		response.InvalidRequest(c, "invalid webhook payload: "+err.Error())
		return
	}

	if len(msg.Alerts) == 0 {
		response.Success(c, gin.H{"received": 0, "message": "no alerts in webhook"})
		return
	}

	ctx := c.Request.Context()
	received := 0
	var errors []string

	for _, alert := range msg.Alerts {
		status := models.AlertStatusFiring
		if alert.Status == "resolved" {
			status = models.AlertStatusResolved
		}

		err := h.service.SyncAlertFromWebhook(ctx, alert.Fingerprint, alert.Labels, alert.Annotations, status, alert.StartsAt, alert.EndsAt, alert.GeneratorURL)
		if err != nil {
			errors = append(errors, alert.Fingerprint+": "+err.Error())
			continue
		}
		received++
	}

	resp := gin.H{
		"receiver": msg.Receiver,
		"status":   msg.Status,
		"received": received,
		"total":    len(msg.Alerts),
	}

	if len(errors) > 0 {
		resp["errors"] = errors
	}

	response.Success(c, resp)
}

func RegisterRoutes(r *gin.RouterGroup, h *Handler) {
	alerts := r.Group("/alerts")
	{
		alerts.POST("", h.SendAlert)
		alerts.POST("/batch", h.SendAlerts)
		alerts.POST("/record", h.SendAlertWithRecord)
		alerts.GET("", h.GetAlerts)
		alerts.GET("/:fingerprint", h.GetAlert)
		alerts.GET("/records", h.GetAlertRecords)
		alerts.GET("/records/stats", h.GetAlertRecordStats)
		alerts.GET("/noisy", h.GetNoisyAlerts)
		alerts.GET("/high-risk", h.GetHighRiskAlerts)
		alerts.GET("/:fingerprint/feedback", h.GetAlertFeedback)
	}

	silences := r.Group("/silences")
	{
		silences.POST("", h.CreateSilence)
		silences.POST("/record", h.CreateSilenceWithRecord)
		silences.GET("", h.GetSilences)
		silences.DELETE("/:id", h.DeleteSilence)
		silences.DELETE("/:id/record", h.DeleteSilenceWithRecord)
	}

	operations := r.Group("/operations")
	{
		operations.GET("/history", h.GetOperationHistory)
	}

	r.GET("/health", h.HealthCheck)
	r.GET("/status", h.GetStatus)
}

func RegisterPublicRoutes(r *gin.RouterGroup, h *Handler) {
	r.POST("/alerts/webhook", h.ReceiveWebhook)
}
