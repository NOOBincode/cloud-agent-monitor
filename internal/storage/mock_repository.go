package storage

import (
	"context"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockServiceRepository struct {
	mock.Mock
}

var _ ServiceRepositoryInterface = (*MockServiceRepository)(nil)

func (m *MockServiceRepository) Create(ctx context.Context, svc *models.Service) error {
	args := m.Called(ctx, svc)
	return args.Error(0)
}

func (m *MockServiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Service, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Service), args.Error(1)
}

func (m *MockServiceRepository) GetByName(ctx context.Context, name string) (*models.Service, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Service), args.Error(1)
}

func (m *MockServiceRepository) List(ctx context.Context, filter ServiceFilter) (*ServiceListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ServiceListResult), args.Error(1)
}

func (m *MockServiceRepository) Update(ctx context.Context, svc *models.Service) error {
	args := m.Called(ctx, svc)
	return args.Error(0)
}

func (m *MockServiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockServiceRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockServiceRepository) SyncLabels(ctx context.Context, serviceID uuid.UUID, labels models.Labels) error {
	args := m.Called(ctx, serviceID, labels)
	return args.Error(0)
}

func (m *MockServiceRepository) GetLabels(ctx context.Context, serviceID uuid.UUID) (models.Labels, error) {
	args := m.Called(ctx, serviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.Labels), args.Error(1)
}

func (m *MockServiceRepository) FindByLabelKey(ctx context.Context, key string) ([]models.Service, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Service), args.Error(1)
}

func (m *MockServiceRepository) FindByLabel(ctx context.Context, key, value string) ([]models.Service, error) {
	args := m.Called(ctx, key, value)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Service), args.Error(1)
}

func (m *MockServiceRepository) GetAllLabelKeys(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockServiceRepository) GetLabelValues(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockServiceRepository) BatchCreate(ctx context.Context, services []*models.Service) ([]models.Service, error) {
	args := m.Called(ctx, services)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Service), args.Error(1)
}

func (m *MockServiceRepository) BatchUpdate(ctx context.Context, services []*models.Service) ([]models.Service, error) {
	args := m.Called(ctx, services)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Service), args.Error(1)
}

func (m *MockServiceRepository) BatchDelete(ctx context.Context, ids []uuid.UUID) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockServiceRepository) Search(ctx context.Context, query ServiceSearchQuery) (*ServiceListResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ServiceListResult), args.Error(1)
}

func (m *MockServiceRepository) UpdateOpenAPI(ctx context.Context, id uuid.UUID, spec string) error {
	args := m.Called(ctx, id, spec)
	return args.Error(0)
}

func (m *MockServiceRepository) GetOpenAPI(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *MockServiceRepository) AddDependency(ctx context.Context, dep *models.ServiceDependency) error {
	args := m.Called(ctx, dep)
	return args.Error(0)
}

func (m *MockServiceRepository) RemoveDependency(ctx context.Context, serviceID, dependsOnID uuid.UUID) error {
	args := m.Called(ctx, serviceID, dependsOnID)
	return args.Error(0)
}

func (m *MockServiceRepository) GetDependencies(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error) {
	args := m.Called(ctx, serviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ServiceDependency), args.Error(1)
}

func (m *MockServiceRepository) GetDependents(ctx context.Context, serviceID uuid.UUID) ([]models.ServiceDependency, error) {
	args := m.Called(ctx, serviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ServiceDependency), args.Error(1)
}

func (m *MockServiceRepository) GetDependencyGraph(ctx context.Context) ([]models.ServiceDependency, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ServiceDependency), args.Error(1)
}

type MockUserRepository struct {
	mock.Mock
}

var _ UserRepositoryInterface = (*MockUserRepository)(nil)

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByPasswordResetToken(ctx context.Context, token string) (*models.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, filter UserFilter) (*UserListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserListResult), args.Error(1)
}

func (m *MockUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	args := m.Called(ctx, username)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	args := m.Called(ctx, email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) AddRole(ctx context.Context, userID, roleID uuid.UUID) error {
	args := m.Called(ctx, userID, roleID)
	return args.Error(0)
}

func (m *MockUserRepository) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	args := m.Called(ctx, userID, roleID)
	return args.Error(0)
}

func (m *MockUserRepository) GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Role), args.Error(1)
}

func (m *MockUserRepository) CreateLoginLog(ctx context.Context, log *models.LoginLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

func (m *MockUserRepository) GetLoginLogsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]models.LoginLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.LoginLog), args.Error(1)
}

type MockAPIKeyRepository struct {
	mock.Mock
}

var _ APIKeyRepositoryInterface = (*MockAPIKeyRepository)(nil)

func (m *MockAPIKeyRepository) Create(ctx context.Context, apiKey *models.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) Update(ctx context.Context, apiKey *models.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error {
	args := m.Called(ctx, id, lastUsedAt)
	return args.Error(0)
}

func (m *MockAPIKeyRepository) List(ctx context.Context, filter APIKeyFilter) (*APIKeyListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*APIKeyListResult), args.Error(1)
}

type MockRoleRepository struct {
	mock.Mock
}

var _ RoleRepositoryInterface = (*MockRoleRepository)(nil)

func (m *MockRoleRepository) Create(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockRoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}

func (m *MockRoleRepository) Update(ctx context.Context, role *models.Role) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockRoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoleRepository) List(ctx context.Context, filter RoleFilter) (*RoleListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RoleListResult), args.Error(1)
}

func (m *MockRoleRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

type MockAlertOperationRepository struct {
	mock.Mock
}

var _ AlertOperationRepositoryInterface = (*MockAlertOperationRepository)(nil)

func (m *MockAlertOperationRepository) Create(ctx context.Context, op *models.AlertOperation) error {
	args := m.Called(ctx, op)
	return args.Error(0)
}

func (m *MockAlertOperationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AlertOperation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AlertOperation), args.Error(1)
}

func (m *MockAlertOperationRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertOperation, error) {
	args := m.Called(ctx, fingerprint, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertOperation), args.Error(1)
}

func (m *MockAlertOperationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.AlertOperationStatus, errMsg string) error {
	args := m.Called(ctx, id, status, errMsg)
	return args.Error(0)
}

func (m *MockAlertOperationRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAlertOperationRepository) ListPending(ctx context.Context, limit int) ([]*models.AlertOperation, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertOperation), args.Error(1)
}

func (m *MockAlertOperationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AlertOperation, int64, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AlertOperation), args.Get(1).(int64), args.Error(2)
}

func (m *MockAlertOperationRepository) ListByStatus(ctx context.Context, status models.AlertOperationStatus, limit int) ([]*models.AlertOperation, error) {
	args := m.Called(ctx, status, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertOperation), args.Error(1)
}

func (m *MockAlertOperationRepository) Update(ctx context.Context, op *models.AlertOperation) error {
	args := m.Called(ctx, op)
	return args.Error(0)
}

type MockAlertNoiseRepository struct {
	mock.Mock
}

var _ AlertNoiseRepositoryInterface = (*MockAlertNoiseRepository)(nil)

func (m *MockAlertNoiseRepository) Upsert(ctx context.Context, record *models.AlertNoiseRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockAlertNoiseRepository) GetByFingerprint(ctx context.Context, fingerprint string) (*models.AlertNoiseRecord, error) {
	args := m.Called(ctx, fingerprint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AlertNoiseRecord), args.Error(1)
}

func (m *MockAlertNoiseRepository) GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertNoiseRecord), args.Error(1)
}

func (m *MockAlertNoiseRepository) GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertNoiseRecord), args.Error(1)
}

func (m *MockAlertNoiseRepository) UpdateNoiseScore(ctx context.Context, fingerprint string, score float64, isNoisy bool) error {
	args := m.Called(ctx, fingerprint, score, isNoisy)
	return args.Error(0)
}

func (m *MockAlertNoiseRepository) IncrementFireCount(ctx context.Context, fingerprint string) error {
	args := m.Called(ctx, fingerprint)
	return args.Error(0)
}

func (m *MockAlertNoiseRepository) IncrementResolveCount(ctx context.Context, fingerprint string) error {
	args := m.Called(ctx, fingerprint)
	return args.Error(0)
}

type MockAlertNotificationRepository struct {
	mock.Mock
}

var _ AlertNotificationRepositoryInterface = (*MockAlertNotificationRepository)(nil)

func (m *MockAlertNotificationRepository) Create(ctx context.Context, notification *models.AlertNotification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockAlertNotificationRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertNotification, error) {
	args := m.Called(ctx, fingerprint, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertNotification), args.Error(1)
}

func (m *MockAlertNotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	args := m.Called(ctx, id, status, errMsg)
	return args.Error(0)
}

type MockAlertRecordRepository struct {
	mock.Mock
}

var _ AlertRecordRepositoryInterface = (*MockAlertRecordRepository)(nil)

func (m *MockAlertRecordRepository) Create(ctx context.Context, record *models.AlertRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockAlertRecordRepository) CreateBatch(ctx context.Context, records []*models.AlertRecord) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

func (m *MockAlertRecordRepository) GetByID(ctx context.Context, id string) (*models.AlertRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AlertRecord), args.Error(1)
}

func (m *MockAlertRecordRepository) GetByFingerprint(ctx context.Context, fingerprint string, limit int) ([]*models.AlertRecord, error) {
	args := m.Called(ctx, fingerprint, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.AlertRecord), args.Error(1)
}

func (m *MockAlertRecordRepository) List(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.AlertRecord), args.Get(1).(int64), args.Error(2)
}

func (m *MockAlertRecordRepository) UpdateStatus(ctx context.Context, fingerprint string, status models.AlertStatus, endsAt *time.Time) error {
	args := m.Called(ctx, fingerprint, status, endsAt)
	return args.Error(0)
}

func (m *MockAlertRecordRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	args := m.Called(ctx, before)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockAlertRecordRepository) GetStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error) {
	args := m.Called(ctx, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AlertRecordStats), args.Error(1)
}
