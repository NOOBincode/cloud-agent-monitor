package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"
	"cloud-agent-monitor/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	m.Run()
}

func TestUserRepository_Create(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		user        *models.User
		expectError bool
	}{
		{
			name: "success",
			user: &models.User{
				Username:     "testuser",
				Email:        "test@example.com",
				PasswordHash: "hashedpassword",
				DisplayName:  "Test User",
				IsActive:     true,
			},
			expectError: false,
		},
		{
			name: "success with tenant ID",
			user: &models.User{
				Username:     "tenantuser",
				Email:        "tenant@example.com",
				PasswordHash: "hashedpassword",
				DisplayName:  "Tenant User",
				IsActive:     true,
				TenantID:     ptrUUID(uuid.New()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.user)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.user.ID)
				assert.NotZero(t, tt.user.CreatedAt)
				assert.NotZero(t, tt.user.UpdatedAt)
			}
		})
	}
}

func TestUserRepository_Create_Duplicate(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{
		Username:     "duplicateuser",
		Email:        "duplicate@example.com",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	duplicate := &models.User{
		Username:     "duplicateuser",
		Email:        "another@example.com",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}

	err = repo.Create(ctx, duplicate)
	assert.Error(t, err)
}

func TestUserRepository_GetByID(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	tests := []struct {
		name        string
		id          uuid.UUID
		expectError error
	}{
		{
			name:        "success",
			id:          user.ID,
			expectError: nil,
		},
		{
			name:        "not found",
			id:          uuid.New(),
			expectError: ErrUserNotFound,
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
				assert.Equal(t, user.ID, result.ID)
				assert.Equal(t, user.Username, result.Username)
				assert.Equal(t, user.Email, result.Email)
			}
		})
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	tests := []struct {
		name        string
		username    string
		expectError error
	}{
		{
			name:        "success",
			username:    "testuser",
			expectError: nil,
		},
		{
			name:        "not found",
			username:    "nonexistent",
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByUsername(ctx, tt.username)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, user.Username, result.Username)
			}
		})
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	tests := []struct {
		name        string
		email       string
		expectError error
	}{
		{
			name:        "success",
			email:       user.Email,
			expectError: nil,
		},
		{
			name:        "not found",
			email:       "nonexistent@example.com",
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByEmail(ctx, tt.email)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, user.Email, result.Email)
			}
		})
	}
}

func TestUserRepository_GetByPasswordResetToken(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	token := "reset-token-123"
	user := &models.User{
		Username:           "testuser",
		Email:              "test@example.com",
		PasswordHash:       "hashedpassword",
		PasswordResetToken: &token,
		IsActive:           true,
	}
	err := repo.Create(ctx, user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError error
	}{
		{
			name:        "success",
			token:       token,
			expectError: nil,
		},
		{
			name:        "not found",
			token:       "invalid-token",
			expectError: ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByPasswordResetToken(ctx, tt.token)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, token, *result.PasswordResetToken)
			}
		})
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	user.DisplayName = "Updated Name"
	user.IsActive = false

	err := repo.Update(ctx, user)
	require.NoError(t, err)

	result, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", result.DisplayName)
	assert.False(t, result.IsActive)
}

func TestUserRepository_Delete(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	err := repo.Delete(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, user.ID)
	assert.Equal(t, ErrUserNotFound, err)

	err = repo.Delete(ctx, uuid.New())
	assert.Equal(t, ErrUserNotFound, err)
}

func TestUserRepository_List(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		testutil.CreateTestUser(db, fmt.Sprintf("user%02d", i), i%2 == 0)
	}

	tests := []struct {
		name        string
		filter      UserFilter
		checkResult func(t *testing.T, result *UserListResult)
	}{
		{
			name:   "default pagination",
			filter: UserFilter{},
			checkResult: func(t *testing.T, result *UserListResult) {
				assert.Len(t, result.Data, 20)
				assert.Equal(t, int64(25), result.Total)
				assert.Equal(t, 1, result.Page)
			},
		},
		{
			name:   "page 2",
			filter: UserFilter{Page: 2, PageSize: 10},
			checkResult: func(t *testing.T, result *UserListResult) {
				assert.Len(t, result.Data, 10)
				assert.Equal(t, int64(25), result.Total)
				assert.Equal(t, 2, result.Page)
			},
		},
		{
			name:   "filter by active status",
			filter: UserFilter{IsActive: ptrBool(true)},
			checkResult: func(t *testing.T, result *UserListResult) {
				assert.Equal(t, int64(13), result.Total)
				for _, u := range result.Data {
					assert.True(t, u.IsActive)
				}
			},
		},
		{
			name:   "filter by username",
			filter: UserFilter{Username: "user00"},
			checkResult: func(t *testing.T, result *UserListResult) {
				assert.Len(t, result.Data, 1)
				assert.Equal(t, int64(1), result.Total)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.List(ctx, tt.filter)

			require.NoError(t, err)
			assert.NotNil(t, result)
			tt.checkResult(t, result)
		})
	}
}

func TestUserRepository_ExistsByUsername(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	testutil.CreateTestUser(db, "existinguser", true)

	tests := []struct {
		name     string
		username string
		expected bool
	}{
		{
			name:     "exists",
			username: "existinguser",
			expected: true,
		},
		{
			name:     "not exists",
			username: "nonexistentuser",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := repo.ExistsByUsername(ctx, tt.username)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

func TestUserRepository_ExistsByEmail(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "exists",
			email:    user.Email,
			expected: true,
		},
		{
			name:     "not exists",
			email:    "nonexistent@example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := repo.ExistsByEmail(ctx, tt.email)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

func TestUserRepository_Roles(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	userRepo := NewUserRepository(db)
	_ = NewRoleRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)
	role1 := testutil.CreateTestRole(db, "admin")
	role2 := testutil.CreateTestRole(db, "editor")

	err := userRepo.AddRole(ctx, user.ID, role1.ID)
	require.NoError(t, err)
	err = userRepo.AddRole(ctx, user.ID, role2.ID)
	require.NoError(t, err)

	roles, err := userRepo.GetRoles(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 2)

	err = userRepo.RemoveRole(ctx, user.ID, role1.ID)
	require.NoError(t, err)

	roles, err = userRepo.GetRoles(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "editor", roles[0].Name)
}

func TestUserRepository_LoginLogs(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	for i := 0; i < 5; i++ {
		log := &models.LoginLog{
			UserID:    &user.ID,
			Username:  "testuser",
			IPAddress: "127.0.0.1",
			UserAgent: "test-agent",
			Success:   i%2 == 0,
		}
		err := repo.CreateLoginLog(ctx, log)
		require.NoError(t, err)
	}

	logs, err := repo.GetLoginLogsByUserID(ctx, user.ID, 10)
	require.NoError(t, err)
	assert.Len(t, logs, 5)

	logs, err = repo.GetLoginLogsByUserID(ctx, user.ID, 3)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestAPIKeyRepository_Create(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	apiKey := &models.APIKey{
		UserID:      user.ID,
		Name:        "test-key",
		Key:         "obs_testkey123",
		KeyHash:     "hash123",
		Prefix:      "obs_test",
		IsActive:    true,
		Permissions: models.Permissions{ServiceRead: true},
	}

	err := apiKeyRepo.Create(ctx, apiKey)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, apiKey.ID)
	assert.NotZero(t, apiKey.CreatedAt)
}

func TestAPIKeyRepository_GetByID(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)
	apiKey := testutil.CreateTestAPIKey(db, user.ID, "test-key", true)

	result, err := apiKeyRepo.GetByID(ctx, apiKey.ID)
	require.NoError(t, err)
	assert.Equal(t, apiKey.ID, result.ID)
	assert.Equal(t, apiKey.Name, result.Name)

	_, err = apiKeyRepo.GetByID(ctx, uuid.New())
	assert.Equal(t, ErrAPIKeyNotFound, err)
}

func TestAPIKeyRepository_GetByKeyHash(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	keyHash := "unique-hash-123"
	apiKey := &models.APIKey{
		UserID:   user.ID,
		Name:     "test-key",
		Key:      "obs_testkey123",
		KeyHash:  keyHash,
		Prefix:   "obs_test",
		IsActive: true,
	}
	err := apiKeyRepo.Create(ctx, apiKey)
	require.NoError(t, err)

	result, err := apiKeyRepo.GetByKeyHash(ctx, keyHash)
	require.NoError(t, err)
	assert.Equal(t, keyHash, result.KeyHash)

	_, err = apiKeyRepo.GetByKeyHash(ctx, "nonexistent-hash")
	assert.Equal(t, ErrNotFound, err)
}

func TestAPIKeyRepository_ListByUserID(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)

	for i := 0; i < 3; i++ {
		testutil.CreateTestAPIKey(db, user.ID, fmt.Sprintf("key-%d", i), true)
	}

	keys, err := apiKeyRepo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 3)
}

func TestAPIKeyRepository_Deactivate(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)
	apiKey := testutil.CreateTestAPIKey(db, user.ID, "test-key", true)

	err := apiKeyRepo.Deactivate(ctx, apiKey.ID)
	require.NoError(t, err)

	result, err := apiKeyRepo.GetByID(ctx, apiKey.ID)
	require.NoError(t, err)
	assert.False(t, result.IsActive)
}

func TestAPIKeyRepository_UpdateLastUsed(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	apiKeyRepo := NewAPIKeyRepository(db)
	ctx := context.Background()

	user := testutil.CreateTestUser(db, "testuser", true)
	apiKey := testutil.CreateTestAPIKey(db, user.ID, "test-key", true)

	lastUsed := time.Now()
	err := apiKeyRepo.UpdateLastUsed(ctx, apiKey.ID, lastUsed)
	require.NoError(t, err)

	result, err := apiKeyRepo.GetByID(ctx, apiKey.ID)
	require.NoError(t, err)
	assert.NotNil(t, result.LastUsedAt)
}

func TestRoleRepository_Create(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewRoleRepository(db)
	ctx := context.Background()

	role := &models.Role{
		Name:        "admin",
		Description: "Administrator role",
		IsSystem:    true,
		Permissions: models.Permissions{
			Admin: true,
		},
	}

	err := repo.Create(ctx, role)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, role.ID)
	assert.NotZero(t, role.CreatedAt)
}

func TestRoleRepository_GetByName(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewRoleRepository(db)
	ctx := context.Background()

	_ = testutil.CreateTestRole(db, "editor")

	result, err := repo.GetByName(ctx, "editor")
	require.NoError(t, err)
	assert.Equal(t, "editor", result.Name)

	_, err = repo.GetByName(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRoleRepository_ExistsByName(t *testing.T) {
	db := testutil.GetTestDB(t)
	defer testutil.CleanupTestDB(t, db)
	repo := NewRoleRepository(db)
	ctx := context.Background()

	testutil.CreateTestRole(db, "viewer")

	exists, err := repo.ExistsByName(ctx, "viewer")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.ExistsByName(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}

func ptrBool(b bool) *bool {
	return &b
}
