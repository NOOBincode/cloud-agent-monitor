package platform

import (
	"errors"
	"net/http"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ServiceHandler struct {
	repo               storage.ServiceRepositoryInterface
	healthCheckService *HealthCheckService
}

func NewServiceHandler(repo storage.ServiceRepositoryInterface) *ServiceHandler {
	return &ServiceHandler{repo: repo}
}

func NewServiceHandlerWithHealthCheck(repo storage.ServiceRepositoryInterface, healthCheckService *HealthCheckService) *ServiceHandler {
	return &ServiceHandler{
		repo:               repo,
		healthCheckService: healthCheckService,
	}
}

type CreateServiceRequest struct {
	Name             string        `json:"name" binding:"required,min=1,max=255"`
	Description      string        `json:"description"`
	Environment      string        `json:"environment" binding:"omitempty,oneof=local dev staging prod"`
	Labels           models.Labels `json:"labels"`
	Endpoint         string        `json:"endpoint" binding:"omitempty,url"`
	OpenAPISpec      string        `json:"openapi_spec"`
	Maintainer       string        `json:"maintainer"`
	Team             string        `json:"team"`
	DocumentationURL string        `json:"documentation_url" binding:"omitempty,url"`
	RepositoryURL    string        `json:"repository_url" binding:"omitempty,url"`
}

type UpdateServiceRequest struct {
	Name             string        `json:"name" binding:"omitempty,min=1,max=255"`
	Description      string        `json:"description"`
	Environment      string        `json:"environment" binding:"omitempty,oneof=local dev staging prod"`
	Labels           models.Labels `json:"labels"`
	Endpoint         string        `json:"endpoint" binding:"omitempty,url"`
	OpenAPISpec      string        `json:"openapi_spec"`
	Maintainer       string        `json:"maintainer"`
	Team             string        `json:"team"`
	DocumentationURL string        `json:"documentation_url" binding:"omitempty,url"`
	RepositoryURL    string        `json:"repository_url" binding:"omitempty,url"`
}

type ServiceResponse struct {
	ID                 uuid.UUID           `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Environment        string              `json:"environment"`
	Labels             models.Labels       `json:"labels"`
	Endpoint           string              `json:"endpoint"`
	HealthStatus       models.HealthStatus `json:"health_status"`
	LastHealthCheckAt  *time.Time          `json:"last_health_check_at,omitempty"`
	HealthCheckDetails string              `json:"health_check_details,omitempty"`
	Maintainer         string              `json:"maintainer"`
	Team               string              `json:"team"`
	DocumentationURL   string              `json:"documentation_url"`
	RepositoryURL      string              `json:"repository_url"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

type ListServicesResponse struct {
	Data     []ServiceResponse `json:"data"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

func (h *ServiceHandler) Create(c *gin.Context) {
	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	exists, err := h.repo.ExistsByName(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to check service existence",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, model.APIError{
			Code:    "ALREADY_EXISTS",
			Message: "service with this name already exists",
		})
		return
	}

	svc := &models.Service{
		Name:             req.Name,
		Description:      req.Description,
		Environment:      req.Environment,
		Labels:           req.Labels,
		Endpoint:         req.Endpoint,
		OpenAPISpec:      req.OpenAPISpec,
		Maintainer:       req.Maintainer,
		Team:             req.Team,
		DocumentationURL: req.DocumentationURL,
		RepositoryURL:    req.RepositoryURL,
	}
	if svc.Environment == "" {
		svc.Environment = "local"
	}

	if err := h.repo.Create(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to create service",
		})
		return
	}

	c.JSON(http.StatusCreated, toServiceResponse(svc))
}

func (h *ServiceHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	svc, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get service",
		})
		return
	}

	c.JSON(http.StatusOK, toServiceResponse(svc))
}

func (h *ServiceHandler) List(c *gin.Context) {
	var filter storage.ServiceFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	result, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to list services",
		})
		return
	}

	resp := ListServicesResponse{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Data:     make([]ServiceResponse, len(result.Data)),
	}
	for i, svc := range result.Data {
		resp.Data[i] = toServiceResponse(&svc)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ServiceHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	svc, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get service",
		})
		return
	}

	if req.Name != "" {
		svc.Name = req.Name
	}
	if req.Description != "" {
		svc.Description = req.Description
	}
	if req.Environment != "" {
		svc.Environment = req.Environment
	}
	if req.Labels != nil {
		svc.Labels = req.Labels
	}
	if req.Endpoint != "" {
		svc.Endpoint = req.Endpoint
	}
	if req.OpenAPISpec != "" {
		svc.OpenAPISpec = req.OpenAPISpec
	}
	if req.Maintainer != "" {
		svc.Maintainer = req.Maintainer
	}
	if req.Team != "" {
		svc.Team = req.Team
	}
	if req.DocumentationURL != "" {
		svc.DocumentationURL = req.DocumentationURL
	}
	if req.RepositoryURL != "" {
		svc.RepositoryURL = req.RepositoryURL
	}

	if err := h.repo.Update(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to update service",
		})
		return
	}

	c.JSON(http.StatusOK, toServiceResponse(svc))
}

func (h *ServiceHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to delete service",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ServiceHandler) GetHealth(c *gin.Context) {
	if h.healthCheckService == nil || h.healthCheckService.Handler() == nil {
		c.JSON(http.StatusServiceUnavailable, model.APIError{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "health check service is not enabled",
		})
		return
	}

	h.healthCheckService.Handler().ServeHTTP(c.Writer, c.Request)
}

func (h *ServiceHandler) GetServiceHealth(c *gin.Context) {
	if h.healthCheckService == nil {
		c.JSON(http.StatusServiceUnavailable, model.APIError{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "health check service is not enabled",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	detail, err := h.healthCheckService.GetServiceHealth(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get service health",
		})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *ServiceHandler) CheckHealthNow(c *gin.Context) {
	if h.healthCheckService == nil {
		c.JSON(http.StatusServiceUnavailable, model.APIError{
			Code:    "SERVICE_UNAVAILABLE",
			Message: "health check service is not enabled",
		})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	metric, err := h.healthCheckService.CheckServiceNow(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to check service health",
		})
		return
	}

	c.JSON(http.StatusOK, metric)
}

func (h *ServiceHandler) GetHealthSummary(c *gin.Context) {
	var filter storage.ServiceFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	result, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to list services",
		})
		return
	}

	summary := HealthSummary{
		Total:    len(result.Data),
		ByStatus: make(map[models.HealthStatus]int),
		Services: make([]ServiceHealthInfo, 0, len(result.Data)),
	}

	for _, svc := range result.Data {
		summary.ByStatus[svc.HealthStatus]++
		summary.Services = append(summary.Services, ServiceHealthInfo{
			ID:          svc.ID,
			Name:        svc.Name,
			Environment: svc.Environment,
			Status:      svc.HealthStatus,
		})
	}

	c.JSON(http.StatusOK, summary)
}

func (h *ServiceHandler) GetLabelKeys(c *gin.Context) {
	keys, err := h.repo.GetAllLabelKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get label keys",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"keys": keys,
	})
}

func (h *ServiceHandler) GetLabelValues(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: "label key is required",
		})
		return
	}

	values, err := h.repo.GetLabelValues(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get label values",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key":    key,
		"values": values,
	})
}

type UpdateLabelsRequest struct {
	Labels models.Labels `json:"labels" binding:"required"`
}

func (h *ServiceHandler) UpdateLabels(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	var req UpdateLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	svc, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get service",
		})
		return
	}

	if err := h.repo.SyncLabels(c.Request.Context(), id, req.Labels); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to sync labels",
		})
		return
	}

	svc.Labels = req.Labels
	if err := h.repo.Update(c.Request.Context(), svc); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to update service",
		})
		return
	}

	c.JSON(http.StatusOK, toServiceResponse(svc))
}

type HealthSummary struct {
	Total    int                         `json:"total"`
	ByStatus map[models.HealthStatus]int `json:"by_status"`
	Services []ServiceHealthInfo         `json:"services"`
}

type ServiceHealthInfo struct {
	ID          uuid.UUID           `json:"id"`
	Name        string              `json:"name"`
	Environment string              `json:"environment"`
	Status      models.HealthStatus `json:"status"`
}

func toServiceResponse(svc *models.Service) ServiceResponse {
	return ServiceResponse{
		ID:                 svc.ID,
		Name:               svc.Name,
		Description:        svc.Description,
		Environment:        svc.Environment,
		Labels:             svc.Labels,
		Endpoint:           svc.Endpoint,
		HealthStatus:       svc.HealthStatus,
		LastHealthCheckAt:  svc.LastHealthCheckAt,
		HealthCheckDetails: svc.HealthCheckDetails,
		Maintainer:         svc.Maintainer,
		Team:               svc.Team,
		DocumentationURL:   svc.DocumentationURL,
		RepositoryURL:      svc.RepositoryURL,
		CreatedAt:          svc.CreatedAt,
		UpdatedAt:          svc.UpdatedAt,
	}
}

type BatchCreateServiceRequest struct {
	Services []CreateServiceRequest `json:"services" binding:"required,min=1,max=100"`
}

type BatchCreateServiceResponse struct {
	Data    []ServiceResponse `json:"data"`
	Created int               `json:"created"`
	Failed  int               `json:"failed"`
	Errors  []BatchError      `json:"errors,omitempty"`
}

type BatchError struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (h *ServiceHandler) BatchCreate(c *gin.Context) {
	var req BatchCreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	services := make([]*models.Service, 0, len(req.Services))
	for _, s := range req.Services {
		env := s.Environment
		if env == "" {
			env = "local"
		}
		services = append(services, &models.Service{
			Name:             s.Name,
			Description:      s.Description,
			Environment:      env,
			Labels:           s.Labels,
			Endpoint:         s.Endpoint,
			OpenAPISpec:      s.OpenAPISpec,
			Maintainer:       s.Maintainer,
			Team:             s.Team,
			DocumentationURL: s.DocumentationURL,
			RepositoryURL:    s.RepositoryURL,
		})
	}

	created, err := h.repo.BatchCreate(c.Request.Context(), services)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to batch create services",
		})
		return
	}

	resp := BatchCreateServiceResponse{
		Data:    make([]ServiceResponse, len(created)),
		Created: len(created),
		Failed:  0,
	}
	for i, s := range created {
		resp.Data[i] = toServiceResponse(&s)
	}

	c.JSON(http.StatusCreated, resp)
}

type BatchUpdateServiceRequest struct {
	Services []BatchUpdateServiceItem `json:"services" binding:"required,min=1,max=100"`
}

type BatchUpdateServiceItem struct {
	ID               uuid.UUID     `json:"id" binding:"required"`
	Name             string        `json:"name" binding:"omitempty,min=1,max=255"`
	Description      string        `json:"description"`
	Environment      string        `json:"environment" binding:"omitempty,oneof=local dev staging prod"`
	Labels           models.Labels `json:"labels"`
	Endpoint         string        `json:"endpoint" binding:"omitempty,url"`
	Maintainer       string        `json:"maintainer"`
	Team             string        `json:"team"`
	DocumentationURL string        `json:"documentation_url" binding:"omitempty,url"`
	RepositoryURL    string        `json:"repository_url" binding:"omitempty,url"`
}

type BatchUpdateServiceResponse struct {
	Data    []ServiceResponse `json:"data"`
	Updated int               `json:"updated"`
	Failed  int               `json:"failed"`
	Errors  []BatchError      `json:"errors,omitempty"`
}

func (h *ServiceHandler) BatchUpdate(c *gin.Context) {
	var req BatchUpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	services := make([]*models.Service, 0, len(req.Services))
	for _, item := range req.Services {
		svc, err := h.repo.GetByID(c.Request.Context(), item.ID)
		if err != nil {
			continue
		}
		if item.Name != "" {
			svc.Name = item.Name
		}
		if item.Description != "" {
			svc.Description = item.Description
		}
		if item.Environment != "" {
			svc.Environment = item.Environment
		}
		if item.Labels != nil {
			svc.Labels = item.Labels
		}
		if item.Endpoint != "" {
			svc.Endpoint = item.Endpoint
		}
		if item.Maintainer != "" {
			svc.Maintainer = item.Maintainer
		}
		if item.Team != "" {
			svc.Team = item.Team
		}
		if item.DocumentationURL != "" {
			svc.DocumentationURL = item.DocumentationURL
		}
		if item.RepositoryURL != "" {
			svc.RepositoryURL = item.RepositoryURL
		}
		services = append(services, svc)
	}

	updated, err := h.repo.BatchUpdate(c.Request.Context(), services)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to batch update services",
		})
		return
	}

	resp := BatchUpdateServiceResponse{
		Data:    make([]ServiceResponse, len(updated)),
		Updated: len(updated),
		Failed:  0,
	}
	for i, s := range updated {
		resp.Data[i] = toServiceResponse(&s)
	}

	c.JSON(http.StatusOK, resp)
}

type BatchDeleteServiceRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1,max=100"`
}

type BatchDeleteServiceResponse struct {
	Deleted int          `json:"deleted"`
	Failed  int          `json:"failed"`
	Errors  []BatchError `json:"errors,omitempty"`
}

func (h *ServiceHandler) BatchDelete(c *gin.Context) {
	var req BatchDeleteServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	if err := h.repo.BatchDelete(c.Request.Context(), req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to batch delete services",
		})
		return
	}

	c.JSON(http.StatusOK, BatchDeleteServiceResponse{
		Deleted: len(req.IDs),
		Failed:  0,
	})
}

func (h *ServiceHandler) Search(c *gin.Context) {
	var query storage.ServiceSearchQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	result, err := h.repo.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to search services",
		})
		return
	}

	resp := ListServicesResponse{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Data:     make([]ServiceResponse, len(result.Data)),
	}
	for i, svc := range result.Data {
		resp.Data[i] = toServiceResponse(&svc)
	}

	c.JSON(http.StatusOK, resp)
}

type UpdateOpenAPIRequest struct {
	Spec string `json:"spec" binding:"required"`
}

func (h *ServiceHandler) GetOpenAPI(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	spec, err := h.repo.GetOpenAPI(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get openapi spec",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":   id,
		"spec": spec,
	})
}

func (h *ServiceHandler) UpdateOpenAPI(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	var req UpdateOpenAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	if err := h.repo.UpdateOpenAPI(c.Request.Context(), id, req.Spec); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to update openapi spec",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"message": "openapi spec updated successfully",
	})
}

type AddDependencyRequest struct {
	DependsOnID  uuid.UUID `json:"depends_on_id" binding:"required"`
	RelationType string    `json:"relation_type" binding:"omitempty,oneof=depends_on calls provides consumes"`
	Description  string    `json:"description"`
}

type DependencyResponse struct {
	ID            uuid.UUID `json:"id"`
	ServiceID     uuid.UUID `json:"service_id"`
	DependsOnID   uuid.UUID `json:"depends_on_id"`
	RelationType  string    `json:"relation_type"`
	Description   string    `json:"description"`
	ServiceName   string    `json:"service_name,omitempty"`
	DependsOnName string    `json:"depends_on_name,omitempty"`
}

func (h *ServiceHandler) GetDependencies(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	deps, err := h.repo.GetDependencies(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get dependencies",
		})
		return
	}

	resp := make([]DependencyResponse, len(deps))
	for i, d := range deps {
		resp[i] = DependencyResponse{
			ID:           d.ID,
			ServiceID:    d.ServiceID,
			DependsOnID:  d.DependsOnID,
			RelationType: d.RelationType,
			Description:  d.Description,
		}
		if d.DependsOn != nil {
			resp[i].DependsOnName = d.DependsOn.Name
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"dependencies": resp,
	})
}

func (h *ServiceHandler) AddDependency(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	var req AddDependencyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		})
		return
	}

	if id == req.DependsOnID {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_REQUEST",
			Message: "service cannot depend on itself",
		})
		return
	}

	dep := &models.ServiceDependency{
		ServiceID:    id,
		DependsOnID:  req.DependsOnID,
		RelationType: req.RelationType,
		Description:  req.Description,
	}
	if dep.RelationType == "" {
		dep.RelationType = models.RelationTypeDependsOn
	}

	if err := h.repo.AddDependency(c.Request.Context(), dep); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, model.APIError{
				Code:    "ALREADY_EXISTS",
				Message: "dependency already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to add dependency",
		})
		return
	}

	c.JSON(http.StatusCreated, DependencyResponse{
		ID:           dep.ID,
		ServiceID:    dep.ServiceID,
		DependsOnID:  dep.DependsOnID,
		RelationType: dep.RelationType,
		Description:  dep.Description,
	})
}

func (h *ServiceHandler) RemoveDependency(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	depIDStr := c.Param("dep_id")
	depID, err := uuid.Parse(depIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid dependency id format",
		})
		return
	}

	if err := h.repo.RemoveDependency(c.Request.Context(), id, depID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, model.APIError{
				Code:    "NOT_FOUND",
				Message: "dependency not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to remove dependency",
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *ServiceHandler) GetDependents(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.APIError{
			Code:    "INVALID_ID",
			Message: "invalid service id format",
		})
		return
	}

	deps, err := h.repo.GetDependents(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get dependents",
		})
		return
	}

	resp := make([]DependencyResponse, len(deps))
	for i, d := range deps {
		resp[i] = DependencyResponse{
			ID:           d.ID,
			ServiceID:    d.ServiceID,
			DependsOnID:  d.DependsOnID,
			RelationType: d.RelationType,
			Description:  d.Description,
		}
		if d.Service != nil {
			resp[i].ServiceName = d.Service.Name
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"dependents": resp,
	})
}

func (h *ServiceHandler) GetDependencyGraph(c *gin.Context) {
	deps, err := h.repo.GetDependencyGraph(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.APIError{
			Code:    "INTERNAL_ERROR",
			Message: "failed to get dependency graph",
		})
		return
	}

	nodes := make(map[uuid.UUID]ServiceResponse)
	edges := make([]DependencyResponse, 0, len(deps))

	for _, d := range deps {
		if d.Service != nil {
			if _, exists := nodes[d.ServiceID]; !exists {
				nodes[d.ServiceID] = toServiceResponse(d.Service)
			}
		}
		if d.DependsOn != nil {
			if _, exists := nodes[d.DependsOnID]; !exists {
				nodes[d.DependsOnID] = toServiceResponse(d.DependsOn)
			}
		}

		edge := DependencyResponse{
			ID:           d.ID,
			ServiceID:    d.ServiceID,
			DependsOnID:  d.DependsOnID,
			RelationType: d.RelationType,
			Description:  d.Description,
		}
		if d.Service != nil {
			edge.ServiceName = d.Service.Name
		}
		if d.DependsOn != nil {
			edge.DependsOnName = d.DependsOn.Name
		}
		edges = append(edges, edge)
	}

	nodeList := make([]ServiceResponse, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, n)
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodeList,
		"edges": edges,
	})
}
