package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Service{}, &models.ServiceDependency{})
	require.NoError(t, err)

	return db
}

func TestServiceRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		service     *models.Service
		expectError bool
	}{
		{
			name: "success",
			service: &models.Service{
				Name:        "test-service",
				Description: "Test description",
				Environment: "dev",
				Labels:      models.Labels{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "success with nil labels",
			service: &models.Service{
				Name:        "test-service-2",
				Description: "Test description",
				Environment: "prod",
			},
			expectError: false,
		},
		{
			name: "success with empty labels",
			service: &models.Service{
				Name:        "test-service-3",
				Description: "Test description",
				Environment: "staging",
				Labels:      models.Labels{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.service)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.service.ID)
				assert.NotZero(t, tt.service.CreatedAt)
				assert.NotZero(t, tt.service.UpdatedAt)
			}
		})
	}
}

func TestServiceRepository_Create_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "duplicate-service",
		Description: "Test description",
		Environment: "dev",
	}

	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	duplicate := &models.Service{
		Name:        "duplicate-service",
		Description: "Another description",
		Environment: "prod",
	}

	err = repo.Create(ctx, duplicate)
	assert.Error(t, err)
}

func TestServiceRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
		Labels:      models.Labels{"key": "value"},
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	tests := []struct {
		name        string
		id          uuid.UUID
		expectError error
	}{
		{
			name:        "success",
			id:          svc.ID,
			expectError: nil,
		},
		{
			name:        "not found",
			id:          uuid.New(),
			expectError: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByID(ctx, tt.id)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, svc.ID, result.ID)
				assert.Equal(t, svc.Name, result.Name)
				assert.Equal(t, svc.Description, result.Description)
				assert.Equal(t, svc.Environment, result.Environment)
				assert.Equal(t, svc.Labels, result.Labels)
			}
		})
	}
}

func TestServiceRepository_GetByName(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	tests := []struct {
		name        string
		serviceName string
		expectError error
	}{
		{
			name:        "success",
			serviceName: "test-service",
			expectError: nil,
		},
		{
			name:        "not found",
			serviceName: "non-existent",
			expectError: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByName(ctx, tt.serviceName)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, svc.Name, result.Name)
			}
		})
	}
}

func TestServiceRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	services := []*models.Service{
		{Name: "service-1", Description: "Service 1", Environment: "dev"},
		{Name: "service-2", Description: "Service 2", Environment: "prod"},
		{Name: "service-3", Description: "Service 3", Environment: "dev"},
		{Name: "service-4", Description: "Service 4", Environment: "staging"},
	}

	for _, svc := range services {
		err := repo.Create(ctx, svc)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}

	tests := []struct {
		name          string
		filter        ServiceFilter
		expectedCount int
		expectedTotal int64
		checkResult   func(t *testing.T, result *ServiceListResult)
	}{
		{
			name:          "list all with default pagination",
			filter:        ServiceFilter{},
			expectedCount: 4,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 1, result.Page)
				assert.Equal(t, 20, result.PageSize)
			},
		},
		{
			name:          "filter by environment",
			filter:        ServiceFilter{Environment: "dev"},
			expectedCount: 2,
			expectedTotal: 2,
		},
		{
			name:          "filter by non-existent environment",
			filter:        ServiceFilter{Environment: "nonexistent"},
			expectedCount: 0,
			expectedTotal: 0,
		},
		{
			name:          "pagination page 1",
			filter:        ServiceFilter{Page: 1, PageSize: 2},
			expectedCount: 2,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 1, result.Page)
				assert.Equal(t, 2, result.PageSize)
			},
		},
		{
			name:          "pagination page 2",
			filter:        ServiceFilter{Page: 2, PageSize: 2},
			expectedCount: 2,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 2, result.Page)
				assert.Equal(t, 2, result.PageSize)
			},
		},
		{
			name:          "pagination beyond total",
			filter:        ServiceFilter{Page: 10, PageSize: 10},
			expectedCount: 0,
			expectedTotal: 4,
		},
		{
			name:          "negative page defaults to 1",
			filter:        ServiceFilter{Page: -1, PageSize: 2},
			expectedCount: 2,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 1, result.Page)
			},
		},
		{
			name:          "zero page size defaults to 20",
			filter:        ServiceFilter{Page: 1, PageSize: 0},
			expectedCount: 4,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 20, result.PageSize)
			},
		},
		{
			name:          "page size capped at 100",
			filter:        ServiceFilter{Page: 1, PageSize: 200},
			expectedCount: 4,
			expectedTotal: 4,
			checkResult: func(t *testing.T, result *ServiceListResult) {
				assert.Equal(t, 100, result.PageSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.List(ctx, tt.filter)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedTotal, result.Total)
			assert.Len(t, result.Data, tt.expectedCount)

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestServiceRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
		Labels:      models.Labels{"key": "value"},
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setup       func()
		update      func(svc *models.Service)
		expectError error
	}{
		{
			name:  "update all fields",
			setup: func() {},
			update: func(svc *models.Service) {
				svc.Name = "updated-service"
				svc.Description = "Updated description"
				svc.Environment = "prod"
				svc.Labels = models.Labels{"new-key": "new-value"}
			},
			expectError: nil,
		},
		{
			name:  "update name only",
			setup: func() {},
			update: func(svc *models.Service) {
				svc.Name = "another-name"
			},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			tt.update(svc)

			err := repo.Update(ctx, svc)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
			} else {
				assert.NoError(t, err)

				result, err := repo.GetByID(ctx, svc.ID)
				require.NoError(t, err)
				assert.Equal(t, svc.Name, result.Name)
				assert.Equal(t, svc.Description, result.Description)
				assert.Equal(t, svc.Environment, result.Environment)
				assert.Equal(t, svc.Labels, result.Labels)
			}
		})
	}
}

func TestServiceRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	err = repo.Delete(ctx, svc.ID)
	assert.NoError(t, err)

	_, err = repo.GetByID(ctx, svc.ID)
	assert.Equal(t, ErrNotFound, err)
}

func TestServiceRepository_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestServiceRepository_ExistsByName(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "existing-service",
		Description: "Test description",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	tests := []struct {
		name         string
		serviceName  string
		expectExists bool
	}{
		{
			name:         "exists",
			serviceName:  "existing-service",
			expectExists: true,
		},
		{
			name:         "not exists",
			serviceName:  "non-existent-service",
			expectExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := repo.ExistsByName(ctx, tt.serviceName)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectExists, exists)
		})
	}
}

func TestServiceRepository_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "soft-delete-test",
		Description: "Test description",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	err = repo.Delete(ctx, svc.ID)
	require.NoError(t, err)

	var count int64
	db.Unscoped().Model(&models.Service{}).Where("id = ?", svc.ID).Count(&count)
	assert.Equal(t, int64(1), count)

	var deletedSvc models.Service
	db.Unscoped().First(&deletedSvc, "id = ?", svc.ID)
	assert.NotNil(t, deletedSvc.DeletedAt)
}

func TestServiceRepository_Labels(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	tests := []struct {
		name   string
		labels models.Labels
	}{
		{
			name:   "nil labels",
			labels: nil,
		},
		{
			name:   "empty labels",
			labels: models.Labels{},
		},
		{
			name:   "single label",
			labels: models.Labels{"key": "value"},
		},
		{
			name:   "multiple labels",
			labels: models.Labels{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &models.Service{
				Name:        "labels-test-" + tt.name,
				Description: "Test description",
				Environment: "dev",
				Labels:      tt.labels,
			}

			err := repo.Create(ctx, svc)
			require.NoError(t, err)

			result, err := repo.GetByID(ctx, svc.ID)
			require.NoError(t, err)

			if tt.labels == nil {
				assert.Nil(t, result.Labels)
			} else {
				assert.Equal(t, tt.labels, result.Labels)
			}
		})
	}
}

func TestServiceRepository_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := &models.Service{
		Name:        "context-test",
		Description: "Test description",
		Environment: "dev",
	}

	err := repo.Create(ctx, svc)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || err != nil)
}

func TestNewServiceRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestServiceRepository_Interface(t *testing.T) {
	db := setupTestDB(t)
	var _ ServiceRepositoryInterface = NewServiceRepository(db)
}

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	return gormDB, mock
}

func TestServiceRepository_Create_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `services`").
		WillReturnError(errors.New("connection error"))
	mock.ExpectRollback()

	err := repo.Create(ctx, &models.Service{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create service")
}

func TestServiceRepository_GetByID_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM `services`").
		WillReturnError(errors.New("connection error"))

	_, err := repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get service by id")
}

func TestServiceRepository_GetByName_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM `services`").
		WillReturnError(errors.New("connection error"))

	_, err := repo.GetByName(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get service by name")
}

func TestServiceRepository_List_CountError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT count\\(\\*\\)").
		WillReturnError(errors.New("count error"))

	_, err := repo.List(ctx, ServiceFilter{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count services")
}

func TestServiceRepository_List_FindError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery("SELECT count\\(\\*\\)").WillReturnRows(rows)
	mock.ExpectQuery("SELECT \\* FROM `services`").
		WillReturnError(errors.New("find error"))

	_, err := repo.List(ctx, ServiceFilter{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list services")
}

func TestServiceRepository_Update_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	testID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `services`").
		WillReturnError(errors.New("update error"))
	mock.ExpectRollback()

	err := repo.Update(ctx, &models.Service{ID: testID, Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update service")
}

func TestServiceRepository_Delete_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `services`").
		WillReturnError(errors.New("delete error"))
	mock.ExpectRollback()

	err := repo.Delete(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete service")
}

func TestServiceRepository_ExistsByName_DBError(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT count\\(\\*\\)").
		WillReturnError(errors.New("count error"))

	_, err := repo.ExistsByName(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check service exists")
}

func TestServiceRepository_Create_DuplicateKey(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `services`").
		WillReturnError(errors.New("Duplicate entry"))
	mock.ExpectRollback()

	err := repo.Create(ctx, &models.Service{Name: "test"})
	assert.Error(t, err)
}

func TestServiceRepository_List_WithEnvironment(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery("SELECT count\\(\\*\\)").WillReturnRows(rows)

	dataRows := sqlmock.NewRows([]string{"id", "name", "description", "environment", "labels", "created_at", "updated_at", "deleted_at"}).
		AddRow(uuid.New(), "test", "desc", "prod", nil, time.Now(), time.Now(), nil)
	mock.ExpectQuery("SELECT \\* FROM `services`").WillReturnRows(dataRows)

	result, err := repo.List(ctx, ServiceFilter{Environment: "prod"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

func TestServiceRepository_GetByID_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	testID := uuid.New()
	testTime := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "description", "environment", "labels", "created_at", "updated_at", "deleted_at"}).
		AddRow(testID, "test-service", "desc", "dev", nil, testTime, testTime, nil)
	mock.ExpectQuery("SELECT \\* FROM `services`").WillReturnRows(rows)

	svc, err := repo.GetByID(ctx, testID)
	assert.NoError(t, err)
	assert.Equal(t, "test-service", svc.Name)
}

func TestServiceRepository_GetByName_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	testID := uuid.New()
	testTime := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "description", "environment", "labels", "created_at", "updated_at", "deleted_at"}).
		AddRow(testID, "test-service", "desc", "dev", nil, testTime, testTime, nil)
	mock.ExpectQuery("SELECT \\* FROM `services`").WillReturnRows(rows)

	svc, err := repo.GetByName(ctx, "test-service")
	assert.NoError(t, err)
	assert.Equal(t, "test-service", svc.Name)
}

func TestServiceRepository_ExistsByName_True(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery("SELECT count\\(\\*\\)").WillReturnRows(rows)

	exists, err := repo.ExistsByName(ctx, "existing-service")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestServiceRepository_ExistsByName_False(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery("SELECT count\\(\\*\\)").WillReturnRows(rows)

	exists, err := repo.ExistsByName(ctx, "non-existing-service")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestServiceRepository_Create_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `services`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.Create(ctx, &models.Service{Name: "test"})
	assert.NoError(t, err)
}

func TestServiceRepository_Delete_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `services`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.Delete(ctx, uuid.New())
	assert.NoError(t, err)
}

func TestServiceRepository_Update_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	testID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `services`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.Update(ctx, &models.Service{ID: testID, Name: "test"})
	assert.NoError(t, err)
}

func TestServiceRepository_List_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery("SELECT count\\(\\*\\)").WillReturnRows(rows)

	testID := uuid.New()
	testTime := time.Now()
	dataRows := sqlmock.NewRows([]string{"id", "name", "description", "environment", "labels", "created_at", "updated_at", "deleted_at"}).
		AddRow(testID, "test-service", "desc", "dev", nil, testTime, testTime, nil)
	mock.ExpectQuery("SELECT \\* FROM `services`").WillReturnRows(dataRows)

	result, err := repo.List(ctx, ServiceFilter{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Data, 1)
}

func TestErrNotFound(t *testing.T) {
	assert.Equal(t, "resource not found", ErrNotFound.Error())
}

func TestErrAlreadyExists(t *testing.T) {
	assert.Equal(t, "resource already exists", ErrAlreadyExists.Error())
}

func TestServiceFilter_Fields(t *testing.T) {
	filter := ServiceFilter{
		Environment: "prod",
		Page:        2,
		PageSize:    50,
	}
	assert.Equal(t, "prod", filter.Environment)
	assert.Equal(t, 2, filter.Page)
	assert.Equal(t, 50, filter.PageSize)
}

func TestServiceListResult_Fields(t *testing.T) {
	result := ServiceListResult{
		Data:     []models.Service{{Name: "test"}},
		Total:    1,
		Page:     1,
		PageSize: 20,
	}
	assert.Len(t, result.Data, 1)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
}

func TestServiceRepository_BatchCreate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	services := []*models.Service{
		{Name: "batch-1", Environment: "dev"},
		{Name: "batch-2", Environment: "prod"},
	}

	result, err := repo.BatchCreate(ctx, services)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NotEqual(t, uuid.Nil, result[0].ID)
	assert.NotEqual(t, uuid.Nil, result[1].ID)

	for _, svc := range result {
		found, err := repo.GetByID(ctx, svc.ID)
		assert.NoError(t, err)
		assert.Equal(t, svc.Name, found.Name)
	}
}

func TestServiceRepository_BatchCreate_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	result, err := repo.BatchCreate(ctx, []*models.Service{})
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestServiceRepository_BatchUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	services := []*models.Service{
		{Name: "batch-update-1", Environment: "dev"},
		{Name: "batch-update-2", Environment: "prod"},
	}

	created, err := repo.BatchCreate(ctx, services)
	require.NoError(t, err)

	updateServices := make([]*models.Service, len(created))
	for i := range created {
		created[i].Description = fmt.Sprintf("Updated %d", i)
		updateServices[i] = &created[i]
	}

	updated, err := repo.BatchUpdate(ctx, updateServices)
	assert.NoError(t, err)
	assert.Len(t, updated, 2)

	for _, svc := range updated {
		found, err := repo.GetByID(ctx, svc.ID)
		assert.NoError(t, err)
		assert.Contains(t, found.Description, "Updated")
	}
}

func TestServiceRepository_BatchDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	services := []*models.Service{
		{Name: "batch-delete-1", Environment: "dev"},
		{Name: "batch-delete-2", Environment: "prod"},
		{Name: "batch-delete-3", Environment: "staging"},
	}

	created, err := repo.BatchCreate(ctx, services)
	require.NoError(t, err)

	ids := make([]uuid.UUID, len(created))
	for i := range created {
		ids[i] = created[i].ID
	}

	err = repo.BatchDelete(ctx, ids[:2])
	assert.NoError(t, err)

	for _, id := range ids[:2] {
		_, err := repo.GetByID(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	}

	_, err = repo.GetByID(ctx, ids[2])
	assert.NoError(t, err)
}

func TestServiceRepository_Search(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	services := []*models.Service{
		{Name: "search-api", Description: "API service", Environment: "dev"},
		{Name: "search-web", Description: "Web frontend", Environment: "prod"},
		{Name: "api-gateway", Description: "Gateway service", Environment: "dev"},
	}

	for _, svc := range services {
		err := repo.Create(ctx, svc)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		query         ServiceSearchQuery
		expectedCount int
	}{
		{
			name:          "search by query",
			query:         ServiceSearchQuery{Query: "api"},
			expectedCount: 2,
		},
		{
			name:          "search by environment",
			query:         ServiceSearchQuery{Environment: "dev"},
			expectedCount: 2,
		},
		{
			name:          "search by query and environment",
			query:         ServiceSearchQuery{Query: "api", Environment: "dev"},
			expectedCount: 2,
		},
		{
			name:          "search no results",
			query:         ServiceSearchQuery{Query: "nonexistent"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.Search(ctx, tt.query)
			assert.NoError(t, err)
			assert.Len(t, result.Data, tt.expectedCount)
		})
	}
}

func TestServiceRepository_OpenAPI(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "openapi-test",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	spec := "openapi: 3.0.0\ninfo:\n  title: Test API\n  version: 1.0.0"
	err = repo.UpdateOpenAPI(ctx, svc.ID, spec)
	assert.NoError(t, err)

	retrieved, err := repo.GetOpenAPI(ctx, svc.ID)
	assert.NoError(t, err)
	assert.Equal(t, spec, retrieved)

	_, err = repo.GetOpenAPI(ctx, uuid.New())
	assert.Equal(t, ErrNotFound, err)
}

func TestServiceRepository_Dependencies(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	serviceA := &models.Service{Name: "service-a", Environment: "dev"}
	serviceB := &models.Service{Name: "service-b", Environment: "dev"}
	serviceC := &models.Service{Name: "service-c", Environment: "dev"}

	err := repo.Create(ctx, serviceA)
	require.NoError(t, err)
	err = repo.Create(ctx, serviceB)
	require.NoError(t, err)
	err = repo.Create(ctx, serviceC)
	require.NoError(t, err)

	dep := &models.ServiceDependency{
		ServiceID:    serviceA.ID,
		DependsOnID:  serviceB.ID,
		RelationType: models.RelationTypeDependsOn,
		Description:  "A depends on B",
	}
	err = repo.AddDependency(ctx, dep)
	assert.NoError(t, err)

	dep2 := &models.ServiceDependency{
		ServiceID:    serviceA.ID,
		DependsOnID:  serviceC.ID,
		RelationType: models.RelationTypeCalls,
		Description:  "A calls C",
	}
	err = repo.AddDependency(ctx, dep2)
	assert.NoError(t, err)

	deps, err := repo.GetDependencies(ctx, serviceA.ID)
	assert.NoError(t, err)
	assert.Len(t, deps, 2)

	dependents, err := repo.GetDependents(ctx, serviceB.ID)
	assert.NoError(t, err)
	assert.Len(t, dependents, 1)
	assert.Equal(t, serviceA.ID, dependents[0].ServiceID)

	err = repo.RemoveDependency(ctx, serviceA.ID, serviceB.ID)
	assert.NoError(t, err)

	deps, err = repo.GetDependencies(ctx, serviceA.ID)
	assert.NoError(t, err)
	assert.Len(t, deps, 1)
}

func TestServiceRepository_DependencyGraph(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	serviceA := &models.Service{Name: "graph-a", Environment: "dev"}
	serviceB := &models.Service{Name: "graph-b", Environment: "dev"}
	serviceC := &models.Service{Name: "graph-c", Environment: "dev"}

	err := repo.Create(ctx, serviceA)
	require.NoError(t, err)
	err = repo.Create(ctx, serviceB)
	require.NoError(t, err)
	err = repo.Create(ctx, serviceC)
	require.NoError(t, err)

	err = repo.AddDependency(ctx, &models.ServiceDependency{
		ServiceID:    serviceA.ID,
		DependsOnID:  serviceB.ID,
		RelationType: models.RelationTypeDependsOn,
	})
	require.NoError(t, err)

	err = repo.AddDependency(ctx, &models.ServiceDependency{
		ServiceID:    serviceB.ID,
		DependsOnID:  serviceC.ID,
		RelationType: models.RelationTypeDependsOn,
	})
	require.NoError(t, err)

	graph, err := repo.GetDependencyGraph(ctx)
	assert.NoError(t, err)
	assert.Len(t, graph, 2)
}

func TestServiceRepository_AddDependency_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	serviceA := &models.Service{Name: "dup-a", Environment: "dev"}
	serviceB := &models.Service{Name: "dup-b", Environment: "dev"}

	err := repo.Create(ctx, serviceA)
	require.NoError(t, err)
	err = repo.Create(ctx, serviceB)
	require.NoError(t, err)

	dep := &models.ServiceDependency{
		ServiceID:    serviceA.ID,
		DependsOnID:  serviceB.ID,
		RelationType: models.RelationTypeDependsOn,
	}
	err = repo.AddDependency(ctx, dep)
	require.NoError(t, err)

	err = repo.AddDependency(ctx, dep)
	assert.Equal(t, ErrAlreadyExists, err)
}

func TestServiceRepository_RemoveDependency_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	err := repo.RemoveDependency(ctx, uuid.New(), uuid.New())
	assert.Equal(t, ErrNotFound, err)
}
