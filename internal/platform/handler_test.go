package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestServiceHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			requestBody: CreateServiceRequest{
				Name:        "test-service",
				Description: "Test description",
				Environment: "dev",
				Labels:      models.Labels{"key": "value"},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("ExistsByName", mock.Anything, "test-service").Return(false, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.Service")).Return(nil).Run(func(args mock.Arguments) {
					svc := args.Get(1).(*models.Service)
					svc.ID = uuid.New()
					svc.CreatedAt = time.Now()
					svc.UpdatedAt = time.Now()
				})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "success with default environment",
			requestBody: CreateServiceRequest{
				Name:        "test-service",
				Description: "Test description",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("ExistsByName", mock.Anything, "test-service").Return(false, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.Service")).Return(nil).Run(func(args mock.Arguments) {
					svc := args.Get(1).(*models.Service)
					svc.ID = uuid.New()
					svc.CreatedAt = time.Now()
					svc.UpdatedAt = time.Now()
				})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid request - missing name",
			requestBody: map[string]interface{}{
				"description": "Test description",
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid request - name too long",
			requestBody: CreateServiceRequest{
				Name: string(make([]byte, 256)),
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "invalid request - invalid environment",
			requestBody: CreateServiceRequest{
				Name:        "test-service",
				Environment: "invalid",
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name: "already exists",
			requestBody: CreateServiceRequest{
				Name: "existing-service",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("ExistsByName", mock.Anything, "existing-service").Return(true, nil)
			},
			expectedStatus: http.StatusConflict,
			expectedCode:   "ALREADY_EXISTS",
		},
		{
			name: "exists check error",
			requestBody: CreateServiceRequest{
				Name: "test-service",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("ExistsByName", mock.Anything, "test-service").Return(false, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
		{
			name: "create error",
			requestBody: CreateServiceRequest{
				Name: "test-service",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("ExistsByName", mock.Anything, "test-service").Return(false, nil)
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.Service")).Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/services", handler.Create)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_Get(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		id             string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(&models.Service{
					ID:          testID,
					Name:        "test-service",
					Description: "Test description",
					Environment: "dev",
					CreatedAt:   testTime,
					UpdatedAt:   testTime,
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid uuid",
			id:             "invalid-uuid",
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_ID",
		},
		{
			name: "not found",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name: "internal error",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services/"+tt.id, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services/:id", handler.Get)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_List(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		query          string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
		expectedCode   string
		checkResult    func(t *testing.T, resp ListServicesResponse)
	}{
		{
			name:  "success with default pagination",
			query: "",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("List", mock.Anything, storage.ServiceFilter{}).Return(&storage.ServiceListResult{
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
			},
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp ListServicesResponse) {
				assert.Equal(t, int64(1), resp.Total)
				assert.Equal(t, 1, resp.Page)
				assert.Equal(t, 20, resp.PageSize)
				assert.Len(t, resp.Data, 1)
			},
		},
		{
			name:  "success with filter",
			query: "?environment=prod&page=2&page_size=10",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("List", mock.Anything, storage.ServiceFilter{
					Environment: "prod",
					Page:        2,
					PageSize:    10,
				}).Return(&storage.ServiceListResult{
					Data:     []models.Service{},
					Total:    0,
					Page:     2,
					PageSize: 10,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResult: func(t *testing.T, resp ListServicesResponse) {
				assert.Equal(t, int64(0), resp.Total)
				assert.Equal(t, 2, resp.Page)
				assert.Equal(t, 10, resp.PageSize)
			},
		},
		{
			name:  "internal error",
			query: "",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("List", mock.Anything, storage.ServiceFilter{}).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services"+tt.query, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services", handler.List)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			if tt.checkResult != nil {
				var resp ListServicesResponse
				json.Unmarshal(w.Body.Bytes(), &resp)
				tt.checkResult(t, resp)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_Update(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		id             string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			id:   testID.String(),
			requestBody: UpdateServiceRequest{
				Name:        "updated-name",
				Description: "Updated description",
				Environment: "prod",
				Labels:      models.Labels{"key": "value"},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(&models.Service{
					ID:          testID,
					Name:        "test-service",
					Description: "Test description",
					Environment: "dev",
					CreatedAt:   testTime,
					UpdatedAt:   testTime,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.Service")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "success partial update",
			id:   testID.String(),
			requestBody: UpdateServiceRequest{
				Name: "updated-name",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(&models.Service{
					ID:          testID,
					Name:        "test-service",
					Description: "Test description",
					Environment: "dev",
					CreatedAt:   testTime,
					UpdatedAt:   testTime,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.Service")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid uuid",
			id:             "invalid-uuid",
			requestBody:    UpdateServiceRequest{Name: "test"},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_ID",
		},
		{
			name:        "invalid request - name too long",
			id:          testID.String(),
			requestBody: UpdateServiceRequest{Name: string(make([]byte, 256))},
			setupMock: func(m *storage.MockServiceRepository) {
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
		},
		{
			name:        "not found",
			id:          testID.String(),
			requestBody: UpdateServiceRequest{Name: "updated-name"},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(nil, storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:        "get error",
			id:          testID.String(),
			requestBody: UpdateServiceRequest{Name: "updated-name"},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
		{
			name:        "update error",
			id:          testID.String(),
			requestBody: UpdateServiceRequest{Name: "updated-name"},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetByID", mock.Anything, testID).Return(&models.Service{
					ID:          testID,
					Name:        "test-service",
					Description: "Test description",
					Environment: "dev",
					CreatedAt:   testTime,
					UpdatedAt:   testTime,
				}, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.Service")).Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/services/"+tt.id, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.PUT("/services/:id", handler.Update)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_Delete(t *testing.T) {
	testID := uuid.New()

	tests := []struct {
		name           string
		id             string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "success",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("Delete", mock.Anything, testID).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "invalid uuid",
			id:             "invalid-uuid",
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_ID",
		},
		{
			name: "not found",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("Delete", mock.Anything, testID).Return(storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name: "internal error",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("Delete", mock.Anything, testID).Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodDelete, "/services/"+tt.id, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.DELETE("/services/:id", handler.Delete)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedCode != "" {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				assert.Equal(t, tt.expectedCode, resp["code"])
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestToServiceResponse(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	svc := &models.Service{
		ID:          testID,
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
		Labels:      models.Labels{"key": "value"},
		CreatedAt:   testTime,
		UpdatedAt:   testTime,
	}

	resp := toServiceResponse(svc)

	assert.Equal(t, testID, resp.ID)
	assert.Equal(t, "test-service", resp.Name)
	assert.Equal(t, "Test description", resp.Description)
	assert.Equal(t, "dev", resp.Environment)
	assert.Equal(t, models.Labels{"key": "value"}, resp.Labels)
	assert.Equal(t, testTime, resp.CreatedAt)
	assert.Equal(t, testTime, resp.UpdatedAt)
}

func TestNewServiceHandler(t *testing.T) {
	mockRepo := new(storage.MockServiceRepository)
	handler := NewServiceHandler(mockRepo)

	assert.NotNil(t, handler)
	assert.Equal(t, mockRepo, handler.repo)
}

func TestCreateServiceRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request CreateServiceRequest
		isValid bool
	}{
		{
			name: "valid request",
			request: CreateServiceRequest{
				Name:        "test-service",
				Description: "Test description",
				Environment: "dev",
			},
			isValid: true,
		},
		{
			name: "valid request with all environments",
			request: CreateServiceRequest{
				Name:        "test-service",
				Environment: "prod",
			},
			isValid: true,
		},
		{
			name: "empty name",
			request: CreateServiceRequest{
				Name: "",
			},
			isValid: false,
		},
		{
			name: "name too long",
			request: CreateServiceRequest{
				Name: string(make([]byte, 256)),
			},
			isValid: false,
		},
		{
			name: "invalid environment",
			request: CreateServiceRequest{
				Name:        "test-service",
				Environment: "invalid",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/test", func(c *gin.Context) {
				var req CreateServiceRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.Status(http.StatusOK)
			})

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.isValid {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestUpdateServiceRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request UpdateServiceRequest
		isValid bool
	}{
		{
			name: "valid request with all fields",
			request: UpdateServiceRequest{
				Name:        "updated-name",
				Description: "Updated description",
				Environment: "prod",
				Labels:      models.Labels{"key": "value"},
			},
			isValid: true,
		},
		{
			name:    "valid empty request",
			request: UpdateServiceRequest{},
			isValid: true,
		},
		{
			name: "name too long",
			request: UpdateServiceRequest{
				Name: string(make([]byte, 256)),
			},
			isValid: false,
		},
		{
			name: "invalid environment",
			request: UpdateServiceRequest{
				Name:        "test",
				Environment: "invalid",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/test", func(c *gin.Context) {
				var req UpdateServiceRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.Status(http.StatusOK)
			})

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.isValid {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestServiceHandler_Create_WithContext(t *testing.T) {
	mockRepo := new(storage.MockServiceRepository)
	handler := NewServiceHandler(mockRepo)

	ctx := context.Background()
	mockRepo.On("ExistsByName", mock.Anything, "test-service").Return(false, nil)
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Service")).Return(nil)

	body, _ := json.Marshal(CreateServiceRequest{Name: "test-service"})
	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/services", handler.Create)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestServiceHandler_BatchCreate(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			requestBody: BatchCreateServiceRequest{
				Services: []CreateServiceRequest{
					{Name: "service-1", Environment: "dev"},
					{Name: "service-2", Environment: "prod"},
				},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("BatchCreate", mock.Anything, mock.AnythingOfType("[]*models.Service")).Return([]models.Service{
					{ID: testID, Name: "service-1", Environment: "dev", CreatedAt: testTime, UpdatedAt: testTime},
					{ID: testID, Name: "service-2", Environment: "prod", CreatedAt: testTime, UpdatedAt: testTime},
				}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "empty services",
			requestBody: BatchCreateServiceRequest{
				Services: []CreateServiceRequest{},
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "repository error",
			requestBody: BatchCreateServiceRequest{
				Services: []CreateServiceRequest{{Name: "service-1"}},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("BatchCreate", mock.Anything, mock.AnythingOfType("[]*models.Service")).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/services/batch", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/services/batch", handler.BatchCreate)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_BatchDelete(t *testing.T) {
	testID := uuid.New()

	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			requestBody: BatchDeleteServiceRequest{
				IDs: []uuid.UUID{testID},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("BatchDelete", mock.Anything, []uuid.UUID{testID}).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "empty ids",
			requestBody: BatchDeleteServiceRequest{
				IDs: []uuid.UUID{},
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "repository error",
			requestBody: BatchDeleteServiceRequest{
				IDs: []uuid.UUID{testID},
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("BatchDelete", mock.Anything, []uuid.UUID{testID}).Return(errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodDelete, "/services/batch", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.DELETE("/services/batch", handler.BatchDelete)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_Search(t *testing.T) {
	testID := uuid.New()
	testTime := time.Now()

	tests := []struct {
		name           string
		query          string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name:  "success with query",
			query: "?q=test&environment=dev",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("Search", mock.Anything, storage.ServiceSearchQuery{
					Query:       "test",
					Environment: "dev",
				}).Return(&storage.ServiceListResult{
					Data: []models.Service{
						{ID: testID, Name: "test-service", Environment: "dev", CreatedAt: testTime, UpdatedAt: testTime},
					},
					Total:    1,
					Page:     1,
					PageSize: 20,
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "repository error",
			query: "?q=test",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("Search", mock.Anything, storage.ServiceSearchQuery{Query: "test"}).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services/search"+tt.query, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services/search", handler.Search)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_GetOpenAPI(t *testing.T) {
	testID := uuid.New()

	tests := []struct {
		name           string
		id             string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetOpenAPI", mock.Anything, testID).Return("openapi: 3.0.0", nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid uuid",
			id:             "invalid",
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "not found",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetOpenAPI", mock.Anything, testID).Return("", storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services/"+tt.id+"/openapi", nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services/:id/openapi", handler.GetOpenAPI)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_UpdateOpenAPI(t *testing.T) {
	testID := uuid.New()

	tests := []struct {
		name           string
		id             string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name:        "success",
			id:          testID.String(),
			requestBody: UpdateOpenAPIRequest{Spec: "openapi: 3.0.0"},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("UpdateOpenAPI", mock.Anything, testID, "openapi: 3.0.0").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid uuid",
			id:             "invalid",
			requestBody:    UpdateOpenAPIRequest{Spec: "openapi: 3.0.0"},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "empty spec",
			id:          testID.String(),
			requestBody: UpdateOpenAPIRequest{Spec: ""},
			setupMock:   func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "not found",
			id:          testID.String(),
			requestBody: UpdateOpenAPIRequest{Spec: "openapi: 3.0.0"},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("UpdateOpenAPI", mock.Anything, testID, "openapi: 3.0.0").Return(storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/services/"+tt.id+"/openapi", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.PUT("/services/:id/openapi", handler.UpdateOpenAPI)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_AddDependency(t *testing.T) {
	serviceID := uuid.New()
	dependsOnID := uuid.New()

	tests := []struct {
		name           string
		id             string
		requestBody    interface{}
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			id:   serviceID.String(),
			requestBody: AddDependencyRequest{
				DependsOnID:  dependsOnID,
				RelationType: "depends_on",
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("AddDependency", mock.Anything, mock.AnythingOfType("*models.ServiceDependency")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid uuid",
			id:             "invalid",
			requestBody:    AddDependencyRequest{DependsOnID: dependsOnID},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "self dependency",
			id:   serviceID.String(),
			requestBody: AddDependencyRequest{
				DependsOnID: serviceID,
			},
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "already exists",
			id:   serviceID.String(),
			requestBody: AddDependencyRequest{
				DependsOnID: dependsOnID,
			},
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("AddDependency", mock.Anything, mock.AnythingOfType("*models.ServiceDependency")).Return(storage.ErrAlreadyExists)
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/services/"+tt.id+"/dependencies", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router := gin.New()
			router.POST("/services/:id/dependencies", handler.AddDependency)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_GetDependencies(t *testing.T) {
	testID := uuid.New()
	dependsOnID := uuid.New()

	tests := []struct {
		name           string
		id             string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetDependencies", mock.Anything, testID).Return([]models.ServiceDependency{
					{
						ID:           uuid.New(),
						ServiceID:    testID,
						DependsOnID:  dependsOnID,
						RelationType: "depends_on",
						DependsOn:    &models.Service{Name: "dep-service"},
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid uuid",
			id:             "invalid",
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "repository error",
			id:   testID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetDependencies", mock.Anything, testID).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services/"+tt.id+"/dependencies", nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services/:id/dependencies", handler.GetDependencies)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_RemoveDependency(t *testing.T) {
	serviceID := uuid.New()
	dependsOnID := uuid.New()

	tests := []struct {
		name           string
		id             string
		depID          string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name:  "success",
			id:    serviceID.String(),
			depID: dependsOnID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("RemoveDependency", mock.Anything, serviceID, dependsOnID).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "invalid service uuid",
			id:             "invalid",
			depID:          dependsOnID.String(),
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid dep uuid",
			id:             serviceID.String(),
			depID:          "invalid",
			setupMock:      func(m *storage.MockServiceRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:  "not found",
			id:    serviceID.String(),
			depID: dependsOnID.String(),
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("RemoveDependency", mock.Anything, serviceID, dependsOnID).Return(storage.ErrNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodDelete, "/services/"+tt.id+"/dependencies/"+tt.depID, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.DELETE("/services/:id/dependencies/:dep_id", handler.RemoveDependency)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestServiceHandler_GetDependencyGraph(t *testing.T) {
	serviceID := uuid.New()
	dependsOnID := uuid.New()

	tests := []struct {
		name           string
		setupMock      func(m *storage.MockServiceRepository)
		expectedStatus int
	}{
		{
			name: "success",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetDependencyGraph", mock.Anything).Return([]models.ServiceDependency{
					{
						ID:           uuid.New(),
						ServiceID:    serviceID,
						DependsOnID:  dependsOnID,
						RelationType: "depends_on",
						Service:      &models.Service{ID: serviceID, Name: "service-a"},
						DependsOn:    &models.Service{ID: dependsOnID, Name: "service-b"},
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "repository error",
			setupMock: func(m *storage.MockServiceRepository) {
				m.On("GetDependencyGraph", mock.Anything).Return(nil, errors.New("db error"))
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(storage.MockServiceRepository)
			tt.setupMock(mockRepo)

			handler := NewServiceHandler(mockRepo)

			req := httptest.NewRequest(http.MethodGet, "/services/dependencies", nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/services/dependencies", handler.GetDependencyGraph)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}
