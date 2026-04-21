package storage

import (
	"context"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNewPostgresDB_Success(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	assert.NotNil(t, gormDB)
}

func TestServiceRepository_Create_GormErrDuplicatedKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	repo := NewServiceRepository(gormDB)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `services`").
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()

	err = repo.Create(ctx, &models.Service{Name: "test"})
	assert.Error(t, err)
	assert.Equal(t, ErrAlreadyExists, err)
}

func TestServiceRepository_GetByID_GormRecordNotFound(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	repo := NewServiceRepository(gormDB)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM `services`").
		WillReturnError(gorm.ErrRecordNotFound)

	_, err = repo.GetByID(ctx, uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestServiceRepository_GetByName_GormRecordNotFound(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	repo := NewServiceRepository(gormDB)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM `services`").
		WillReturnError(gorm.ErrRecordNotFound)

	_, err = repo.GetByName(ctx, "test")
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestServiceRepository_Update_RowsAffected(t *testing.T) {
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

	svc.Description = "Updated description"
	err = repo.Update(ctx, svc)
	assert.NoError(t, err)

	result, err := repo.GetByID(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", result.Description)
}

func TestServiceRepository_Delete_NoRowsAffected(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	assert.Equal(t, ErrNotFound, err)
}

func TestPing_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	<-ctx.Done()
	assert.Error(t, ctx.Err())
}

func TestPing_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, ctx.Err())
}

func TestErrors(t *testing.T) {
	assert.Equal(t, "resource not found", ErrNotFound.Error())
	assert.Equal(t, "resource already exists", ErrAlreadyExists.Error())
}

func TestServiceFilter_DefaultValues(t *testing.T) {
	filter := ServiceFilter{}
	assert.Empty(t, filter.Environment)
	assert.Zero(t, filter.Page)
	assert.Zero(t, filter.PageSize)
}

func TestServiceListResult_Empty(t *testing.T) {
	result := ServiceListResult{}
	assert.Nil(t, result.Data)
	assert.Zero(t, result.Total)
	assert.Zero(t, result.Page)
	assert.Zero(t, result.PageSize)
}

func TestServiceRepository_Create_WithAllFields(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "full-service",
		Description: "Full description",
		Environment: "prod",
		Labels:      models.Labels{"team": "backend", "version": "v1"},
	}

	err := repo.Create(ctx, svc)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, svc.ID)

	result, err := repo.GetByID(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, "full-service", result.Name)
	assert.Equal(t, "Full description", result.Description)
	assert.Equal(t, "prod", result.Environment)
	assert.Equal(t, models.Labels{"team": "backend", "version": "v1"}, result.Labels)
}

func TestServiceRepository_List_WithPagination(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		svc := &models.Service{
			Name:        "service-" + string(rune('a'+i)),
			Description: "Description",
			Environment: "dev",
		}
		err := repo.Create(ctx, svc)
		require.NoError(t, err)
	}

	result, err := repo.List(ctx, ServiceFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(25), result.Total)
	assert.Len(t, result.Data, 10)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)

	result2, err := repo.List(ctx, ServiceFilter{Page: 2, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, result2.Data, 10)
	assert.Equal(t, 2, result2.Page)
}

func TestServiceRepository_ExistsByName_AfterDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "to-delete",
		Description: "Test",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	exists, err := repo.ExistsByName(ctx, "to-delete")
	require.NoError(t, err)
	assert.True(t, exists)

	err = repo.Delete(ctx, svc.ID)
	require.NoError(t, err)

	exists, err = repo.ExistsByName(ctx, "to-delete")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestServiceRepository_GetByName_AfterUpdate(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "original-name",
		Description: "Original",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	result, err := repo.GetByName(ctx, "original-name")
	require.NoError(t, err)
	assert.Equal(t, "Original", result.Description)
}

func TestServiceRepository_Update_Environment(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "env-test",
		Description: "Test",
		Environment: "dev",
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	svc.Environment = "prod"
	err = repo.Update(ctx, svc)
	require.NoError(t, err)

	result, err := repo.GetByID(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, "prod", result.Environment)
}

func TestServiceRepository_Update_Labels(t *testing.T) {
	db := setupTestDB(t)
	repo := NewServiceRepository(db)
	ctx := context.Background()

	svc := &models.Service{
		Name:        "labels-test",
		Description: "Test",
		Environment: "dev",
		Labels:      models.Labels{"key1": "value1"},
	}
	err := repo.Create(ctx, svc)
	require.NoError(t, err)

	svc.Labels = models.Labels{"key1": "updated", "key2": "new"}
	err = repo.Update(ctx, svc)
	require.NoError(t, err)

	result, err := repo.GetByID(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, models.Labels{"key1": "updated", "key2": "new"}, result.Labels)
}