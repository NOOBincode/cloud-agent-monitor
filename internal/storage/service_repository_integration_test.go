//go:build integration

package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/pkg/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ServiceRepositoryIntegrationSuite struct {
	suite.Suite
	db   *DB
	repo *ServiceRepository
}

func TestServiceRepositoryIntegration(t *testing.T) {
	suite.Run(t, new(ServiceRepositoryIntegrationSuite))
}

func (s *ServiceRepositoryIntegrationSuite) SetupSuite() {
	cfg := config.DatabaseConfig{
		Host:     getEnv("TEST_DATABASE_HOST", "127.0.0.1"),
		Port:     3306,
		User:     getEnv("TEST_DATABASE_USER", "root"),
		Password: getEnv("TEST_DATABASE_PASSWORD", "root"),
		Database: getEnv("TEST_DATABASE_NAME", "obs_platform_test"),
		Charset:  "utf8mb4",
	}

	var err error
	s.db, err = NewMySQLDB(cfg)
	require.NoError(s.T(), err)

	err = s.db.Exec("DROP TABLE IF EXISTS services").Error
	require.NoError(s.T(), err)

	err = s.db.AutoMigrate(&models.Service{})
	require.NoError(s.T(), err)

	s.repo = NewServiceRepository(s.db)
}

func (s *ServiceRepositoryIntegrationSuite) TearDownSuite() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

func (s *ServiceRepositoryIntegrationSuite) SetupTest() {
	s.db.Exec("TRUNCATE TABLE services")
}

func (s *ServiceRepositoryIntegrationSuite) TestCRUD() {
	ctx := context.Background()

	svc := &models.Service{
		Name:        "integration-test-service",
		Description: "Integration test service",
		Environment: "dev",
		Labels:      models.Labels{"team": "backend", "region": "cn"},
	}

	err := s.repo.Create(ctx, svc)
	s.NoError(err)
	s.NotEqual(uuid.Nil, svc.ID)

	retrieved, err := s.repo.GetByID(ctx, svc.ID)
	s.NoError(err)
	s.Equal(svc.Name, retrieved.Name)
	s.Equal(svc.Description, retrieved.Description)
	s.Equal(svc.Environment, retrieved.Environment)
	s.Equal(svc.Labels, retrieved.Labels)

	retrieved.Description = "Updated description"
	retrieved.Environment = "prod"
	retrieved.Labels = models.Labels{"team": "frontend", "region": "us"}

	err = s.repo.Update(ctx, retrieved)
	s.NoError(err)

	updated, err := s.repo.GetByID(ctx, svc.ID)
	s.NoError(err)
	s.Equal("Updated description", updated.Description)
	s.Equal("prod", updated.Environment)
	s.Equal(models.Labels{"team": "frontend", "region": "us"}, updated.Labels)

	err = s.repo.Delete(ctx, svc.ID)
	s.NoError(err)

	_, err = s.repo.GetByID(ctx, svc.ID)
	s.Equal(ErrNotFound, err)
}

func (s *ServiceRepositoryIntegrationSuite) TestList() {
	ctx := context.Background()

	for i := 1; i <= 25; i++ {
		env := "dev"
		if i%3 == 0 {
			env = "prod"
		} else if i%3 == 1 {
			env = "staging"
		}

		svc := &models.Service{
			Name:        fmt.Sprintf("service-%d", i),
			Description: fmt.Sprintf("Service %d", i),
			Environment: env,
		}
		err := s.repo.Create(ctx, svc)
		require.NoError(s.T(), err)
		time.Sleep(time.Millisecond)
	}

	result, err := s.repo.List(ctx, ServiceFilter{})
	s.NoError(err)
	s.Equal(int64(25), result.Total)
	s.Len(result.Data, 20)
	s.Equal(1, result.Page)
	s.Equal(20, result.PageSize)

	result, err = s.repo.List(ctx, ServiceFilter{Environment: "prod"})
	s.NoError(err)
	s.Equal(int64(8), result.Total)

	result, err = s.repo.List(ctx, ServiceFilter{Page: 2, PageSize: 10})
	s.NoError(err)
	s.Equal(int64(25), result.Total)
	s.Len(result.Data, 10)
	s.Equal(2, result.Page)
	s.Equal(10, result.PageSize)
}

func (s *ServiceRepositoryIntegrationSuite) TestDuplicateName() {
	ctx := context.Background()

	svc1 := &models.Service{
		Name:        "duplicate-test",
		Description: "First service",
		Environment: "dev",
	}
	err := s.repo.Create(ctx, svc1)
	s.NoError(err)

	svc2 := &models.Service{
		Name:        "duplicate-test",
		Description: "Second service",
		Environment: "prod",
	}
	err = s.repo.Create(ctx, svc2)
	s.Equal(ErrAlreadyExists, err)
}

func (s *ServiceRepositoryIntegrationSuite) TestExistsByName() {
	ctx := context.Background()

	svc := &models.Service{
		Name:        "exists-test",
		Description: "Test service",
		Environment: "dev",
	}
	err := s.repo.Create(ctx, svc)
	s.NoError(err)

	exists, err := s.repo.ExistsByName(ctx, "exists-test")
	s.NoError(err)
	s.True(exists)

	exists, err = s.repo.ExistsByName(ctx, "non-existent")
	s.NoError(err)
	s.False(exists)
}

func (s *ServiceRepositoryIntegrationSuite) TestGetByName() {
	ctx := context.Background()

	svc := &models.Service{
		Name:        "get-by-name-test",
		Description: "Test service",
		Environment: "dev",
		Labels:      models.Labels{"key": "value"},
	}
	err := s.repo.Create(ctx, svc)
	s.NoError(err)

	retrieved, err := s.repo.GetByName(ctx, "get-by-name-test")
	s.NoError(err)
	s.Equal(svc.ID, retrieved.ID)
	s.Equal(svc.Description, retrieved.Description)
	s.Equal(svc.Labels, retrieved.Labels)

	_, err = s.repo.GetByName(ctx, "non-existent")
	s.Equal(ErrNotFound, err)
}

func (s *ServiceRepositoryIntegrationSuite) TestSoftDelete() {
	ctx := context.Background()

	svc := &models.Service{
		Name:        "soft-delete-test",
		Description: "Test service",
		Environment: "dev",
	}
	err := s.repo.Create(ctx, svc)
	s.NoError(err)

	err = s.repo.Delete(ctx, svc.ID)
	s.NoError(err)

	var count int64
	s.db.Unscoped().Model(&models.Service{}).Where("id = ?", svc.ID).Count(&count)
	s.Equal(int64(1), count)

	var deletedSvc models.Service
	s.db.Unscoped().First(&deletedSvc, "id = ?", svc.ID)
	s.NotNil(deletedSvc.DeletedAt)
}

func (s *ServiceRepositoryIntegrationSuite) TestConcurrentCreate() {
	ctx := context.Background()

	const numGoroutines = 10
	errCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			svc := &models.Service{
				Name:        fmt.Sprintf("concurrent-test-%d", idx),
				Description: "Concurrent test",
				Environment: "dev",
			}
			errCh <- s.repo.Create(ctx, svc)
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		err := <-errCh
		s.NoError(err)
	}

	result, err := s.repo.List(ctx, ServiceFilter{})
	s.NoError(err)
	s.Equal(int64(numGoroutines), result.Total)
}

func (s *ServiceRepositoryIntegrationSuite) TestLargeLabels() {
	ctx := context.Background()

	largeLabels := make(models.Labels)
	for i := 0; i < 50; i++ {
		largeLabels[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	svc := &models.Service{
		Name:        "large-labels-test",
		Description: "Test service with large labels",
		Environment: "dev",
		Labels:      largeLabels,
	}
	err := s.repo.Create(ctx, svc)
	s.NoError(err)

	retrieved, err := s.repo.GetByID(ctx, svc.ID)
	s.NoError(err)
	s.Equal(largeLabels, retrieved.Labels)
}

func (s *ServiceRepositoryIntegrationSuite) TestSpecialCharacters() {
	ctx := context.Background()

	svc := &models.Service{
		Name:        "special-chars-test",
		Description: "Test with special chars: <>&\"'`",
		Environment: "dev",
		Labels:      models.Labels{"emoji": "🎉", "chinese": "中文测试", "special": "<>&\"'"},
	}
	err := s.repo.Create(ctx, svc)
	s.NoError(err)

	retrieved, err := s.repo.GetByID(ctx, svc.ID)
	s.NoError(err)
	s.Equal("Test with special chars: <>&\"'`", retrieved.Description)
	s.Equal("🎉", retrieved.Labels["emoji"])
	s.Equal("中文测试", retrieved.Labels["chinese"])
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
