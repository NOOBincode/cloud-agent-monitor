package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegisterRoutes(t *testing.T) {
	mockRepo := new(storage.MockServiceRepository)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, mockRepo)

	routes := router.Routes()
	routeMap := make(map[string]string)
	for _, route := range routes {
		routeMap[route.Method+route.Path] = route.Path
	}

	assert.Contains(t, routeMap, "GET/api/v1/services")
	assert.Contains(t, routeMap, "POST/api/v1/services")
	assert.Contains(t, routeMap, "GET/api/v1/services/:id")
	assert.Contains(t, routeMap, "PUT/api/v1/services/:id")
	assert.Contains(t, routeMap, "DELETE/api/v1/services/:id")
}

func TestRoutes_InvalidUUID(t *testing.T) {
	mockRepo := new(storage.MockServiceRepository)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, mockRepo)

	tests := []struct {
		name       string
		method     string
		path       string
		expectCode int
	}{
		{
			name:       "GET /services/:id with invalid id",
			method:     http.MethodGet,
			path:       "/api/v1/services/invalid",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "PUT /services/:id with invalid id",
			method:     http.MethodPut,
			path:       "/api/v1/services/invalid",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "DELETE /services/:id with invalid id",
			method:     http.MethodDelete,
			path:       "/api/v1/services/invalid",
			expectCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Reader
			if tt.method == http.MethodPut {
				jsonBody, _ := json.Marshal(map[string]string{"name": "test"})
				body = *bytes.NewReader(jsonBody)
			}
			req := httptest.NewRequest(tt.method, tt.path, &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectCode, w.Code)
		})
	}
}

func TestRoutes_POST_InvalidRequest(t *testing.T) {
	mockRepo := new(storage.MockServiceRepository)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, mockRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRoutes_GET_List(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	mockRepo := new(storage.MockServiceRepository)
	mockRepo.On("List", mock.Anything, storage.ServiceFilter{}).Return(&storage.ServiceListResult{
		Data: []models.Service{
			{
				ID:          testID,
				Name:        "test-service",
				Description: "Test description",
				Environment: "dev",
				CreatedAt:   testTime,
				UpdatedAt:   testTime,
			},
		},
		Total:    1,
		Page:     1,
		PageSize: 20,
	}, nil)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, mockRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}
