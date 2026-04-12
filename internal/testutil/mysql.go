package testutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func GetTestDB(t *testing.T) *gorm.DB {
	if testDB != nil {
		return testDB
	}

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		host := getEnvOrDefault("TEST_DATABASE_HOST", "127.0.0.1")
		port := getEnvOrDefault("TEST_DATABASE_PORT", "3306")
		user := getEnvOrDefault("TEST_DATABASE_USER", "root")
		password := getEnvOrDefault("TEST_DATABASE_PASSWORD", "root")
		database := getEnvOrDefault("TEST_DATABASE_NAME", "obs_platform_test")

		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			user, password, host, port, database,
		)
	}

	var err error
	testDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = testDB.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.UserRole{},
		&models.APIKey{},
		&models.LoginLog{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return testDB
}

func CleanupTestDB(t *testing.T, db *gorm.DB) {
	db.Exec("DELETE FROM user_roles")
	db.Exec("DELETE FROM api_keys")
	db.Exec("DELETE FROM login_logs")
	db.Exec("DELETE FROM users")
	db.Exec("DELETE FROM roles")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func CreateTestUser(db *gorm.DB, username string, isActive bool) *models.User {
	user := &models.User{
		ID:           uuid.New(),
		Username:     username,
		Email:        username + "@test.com",
		PasswordHash: "hashedpassword",
		DisplayName:  username,
		IsActive:     isActive,
	}
	isActiveInt := 0
	if isActive {
		isActiveInt = 1
	}
	db.Exec(
		"INSERT INTO users (id, username, email, password_hash, display_name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())",
		user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName, isActiveInt,
	)
	var createdUser models.User
	db.Where("id = ?", user.ID).First(&createdUser)
	user.CreatedAt = createdUser.CreatedAt
	user.UpdatedAt = createdUser.UpdatedAt
	return user
}

func CreateTestRole(db *gorm.DB, name string) *models.Role {
	role := &models.Role{
		ID:          uuid.New(),
		Name:        name,
		Description: name + " role",
		IsSystem:    false,
	}
	db.Create(role)
	return role
}

func CreateTestAPIKey(db *gorm.DB, userID uuid.UUID, name string, isActive bool) *models.APIKey {
	key := &models.APIKey{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     name,
		Key:      "obs_test_" + uuid.New().String()[:8],
		KeyHash:  uuid.New().String(),
		Prefix:   "obs_test_",
		IsActive: isActive,
	}
	db.Create(key)
	return key
}
