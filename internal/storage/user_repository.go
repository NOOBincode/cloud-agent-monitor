package storage

import (
	"context"
	"errors"
	"time"

	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserNotFound 用户未找到错误
var ErrUserNotFound = errors.New("user not found")

// ErrAPIKeyNotFound API密钥未找到错误
var ErrAPIKeyNotFound = errors.New("api key not found")

// UserRepositoryInterface 用户仓储接口
type UserRepositoryInterface interface {
	// Create 创建用户
	Create(ctx context.Context, user *models.User) error

	// GetByID 根据ID获取用户
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)

	// GetByUsername 根据用户名获取用户
	GetByUsername(ctx context.Context, username string) (*models.User, error)

	// GetByEmail 根据邮箱获取用户
	GetByEmail(ctx context.Context, email string) (*models.User, error)

	// GetByPasswordResetToken 根据密码重置令牌获取用户
	GetByPasswordResetToken(ctx context.Context, token string) (*models.User, error)

	// Update 更新用户
	Update(ctx context.Context, user *models.User) error

	// Delete 删除用户（软删除）
	Delete(ctx context.Context, id uuid.UUID) error

	// List 列出用户
	List(ctx context.Context, filter UserFilter) (*UserListResult, error)

	// ExistsByUsername 检查用户名是否存在
	ExistsByUsername(ctx context.Context, username string) (bool, error)

	// ExistsByEmail 检查邮箱是否存在
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// AddRole 给用户添加角色
	AddRole(ctx context.Context, userID, roleID uuid.UUID) error

	// RemoveRole 移除用户角色
	RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error

	// GetRoles 获取用户的所有角色
	GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error)

	// CreateLoginLog 创建登录日志
	CreateLoginLog(ctx context.Context, log *models.LoginLog) error

	// GetLoginLogsByUserID 获取用户的登录日志
	GetLoginLogsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]models.LoginLog, error)
}

// APIKeyRepositoryInterface API密钥仓储接口
type APIKeyRepositoryInterface interface {
	// Create 创建API密钥
	Create(ctx context.Context, apiKey *models.APIKey) error

	// GetByID 根据ID获取API密钥
	GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error)

	// GetByKeyHash 根据密钥哈希获取API密钥
	GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error)

	// ListByUserID 列出用户的所有API密钥
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error)

	// Update 更新API密钥
	Update(ctx context.Context, apiKey *models.APIKey) error

	// Delete 删除API密钥（软删除）
	Delete(ctx context.Context, id uuid.UUID) error

	// Deactivate 停用API密钥
	Deactivate(ctx context.Context, id uuid.UUID) error

	// UpdateLastUsed 更新最后使用时间
	UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error

	// List 列出API密钥（支持过滤）
	List(ctx context.Context, filter APIKeyFilter) (*APIKeyListResult, error)
}

// RoleRepositoryInterface 角色仓储接口
type RoleRepositoryInterface interface {
	// Create 创建角色
	Create(ctx context.Context, role *models.Role) error

	// GetByID 根据ID获取角色
	GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error)

	// GetByName 根据名称获取角色
	GetByName(ctx context.Context, name string) (*models.Role, error)

	// Update 更新角色
	Update(ctx context.Context, role *models.Role) error

	// Delete 删除角色
	Delete(ctx context.Context, id uuid.UUID) error

	// List 列出角色
	List(ctx context.Context, filter RoleFilter) (*RoleListResult, error)

	// ExistsByName 检查角色名是否存在
	ExistsByName(ctx context.Context, name string) (bool, error)
}

// UserFilter 用户过滤条件
type UserFilter struct {
	Username string     `form:"username"`
	Email    string     `form:"email"`
	IsActive *bool      `form:"is_active"`
	TenantID *uuid.UUID `form:"tenant_id"`
	Page     int        `form:"page"`
	PageSize int        `form:"page_size"`
}

// UserListResult 用户列表结果
type UserListResult struct {
	Data     []models.User `json:"data"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// APIKeyFilter API密钥过滤条件
type APIKeyFilter struct {
	UserID   uuid.UUID  `form:"user_id"`
	IsActive *bool      `form:"is_active"`
	TenantID *uuid.UUID `form:"tenant_id"`
	Page     int        `form:"page"`
	PageSize int        `form:"page_size"`
}

// APIKeyListResult API密钥列表结果
type APIKeyListResult struct {
	Data     []models.APIKey `json:"data"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// RoleFilter 角色过滤条件
type RoleFilter struct {
	Name     string     `form:"name"`
	IsSystem *bool      `form:"is_system"`
	TenantID *uuid.UUID `form:"tenant_id"`
	Page     int        `form:"page"`
	PageSize int        `form:"page_size"`
}

// RoleListResult 角色列表结果
type RoleListResult struct {
	Data     []models.Role `json:"data"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// UserRepository 用户仓储实现
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储实例
func NewUserRepository(db *gorm.DB) UserRepositoryInterface {
	return &UserRepository{db: db}
}

// Create 创建用户
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID 根据ID获取用户
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Preload("APIKeys").
		First(&user, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		First(&user, "username = ?", username).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		First(&user, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetByPasswordResetToken 根据密码重置令牌获取用户
func (r *UserRepository) GetByPasswordResetToken(ctx context.Context, token string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		First(&user, "password_reset_token = ?", token).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Update 更新用户
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 删除用户（软删除）
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// List 列出用户
func (r *UserRepository) List(ctx context.Context, filter UserFilter) (*UserListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	baseQuery := r.db.WithContext(ctx).Model(&models.User{})

	// 应用过滤条件
	if filter.Username != "" {
		baseQuery = baseQuery.Where("username LIKE ?", "%"+filter.Username+"%")
	}
	if filter.Email != "" {
		baseQuery = baseQuery.Where("email LIKE ?", "%"+filter.Email+"%")
	}
	if filter.IsActive != nil {
		baseQuery = baseQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.TenantID != nil {
		baseQuery = baseQuery.Where("tenant_id = ?", *filter.TenantID)
	}

	// 计算总数 - 使用Session创建新查询避免Count影响原查询
	var total int64
	countQuery := r.db.Session(&gorm.Session{}).WithContext(ctx).Model(&models.User{})
	if filter.Username != "" {
		countQuery = countQuery.Where("username LIKE ?", "%"+filter.Username+"%")
	}
	if filter.Email != "" {
		countQuery = countQuery.Where("email LIKE ?", "%"+filter.Email+"%")
	}
	if filter.IsActive != nil {
		countQuery = countQuery.Where("is_active = ?", *filter.IsActive)
	}
	if filter.TenantID != nil {
		countQuery = countQuery.Where("tenant_id = ?", *filter.TenantID)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	// 获取数据
	var users []models.User
	offset := (filter.Page - 1) * filter.PageSize
	if err := baseQuery.Offset(offset).Limit(filter.PageSize).Find(&users).Error; err != nil {
		return nil, err
	}

	return &UserListResult{
		Data:     users,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("username = ?", username).
		Count(&count).Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("email = ?", email).
		Count(&count).Error
	return count > 0, err
}

// AddRole 给用户添加角色
func (r *UserRepository) AddRole(ctx context.Context, userID, roleID uuid.UUID) error {
	userRole := models.UserRole{
		UserID: userID,
		RoleID: roleID,
	}
	return r.db.WithContext(ctx).Create(&userRole).Error
}

// RemoveRole 移除用户角色
func (r *UserRepository) RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.UserRole{}, "user_id = ? AND role_id = ?", userID, roleID).Error
}

// GetRoles 获取用户的所有角色
func (r *UserRepository) GetRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	var roles []models.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

// CreateLoginLog 创建登录日志
func (r *UserRepository) CreateLoginLog(ctx context.Context, log *models.LoginLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetLoginLogsByUserID 获取用户的登录日志
func (r *UserRepository) GetLoginLogsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]models.LoginLog, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var logs []models.LoginLog
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// APIKeyRepository API密钥仓储实现
type APIKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository 创建API密钥仓储实例
func NewAPIKeyRepository(db *gorm.DB) APIKeyRepositoryInterface {
	return &APIKeyRepository{db: db}
}

// Create 创建API密钥
func (r *APIKeyRepository) Create(ctx context.Context, apiKey *models.APIKey) error {
	return r.db.WithContext(ctx).Create(apiKey).Error
}

// GetByID 根据ID获取API密钥
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var apiKey models.APIKey
	err := r.db.WithContext(ctx).
		Preload("User").
		First(&apiKey, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	return &apiKey, nil
}

// GetByKeyHash 根据密钥哈希获取API密钥
func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	var apiKey models.APIKey
	err := r.db.WithContext(ctx).
		Preload("User").
		First(&apiKey, "key_hash = ?", keyHash).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &apiKey, nil
}

// ListByUserID 列出用户的所有API密钥
func (r *APIKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	var apiKeys []models.APIKey
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&apiKeys).Error
	return apiKeys, err
}

// Update 更新API密钥
func (r *APIKeyRepository) Update(ctx context.Context, apiKey *models.APIKey) error {
	return r.db.WithContext(ctx).Save(apiKey).Error
}

// Delete 删除API密钥（软删除）
func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.APIKey{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAPIKeyNotFound
	}
	return nil
}

// Deactivate 停用API密钥
func (r *APIKeyRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ?", id).
		Update("is_active", false).Error
}

// UpdateLastUsed 更新最后使用时间
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.APIKey{}).
		Where("id = ?", id).
		Update("last_used_at", lastUsedAt).Error
}

// List 列出API密钥（支持过滤）
func (r *APIKeyRepository) List(ctx context.Context, filter APIKeyFilter) (*APIKeyListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&models.APIKey{})

	// 应用过滤条件
	if filter.UserID != uuid.Nil {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.TenantID != nil {
		query = query.Where("tenant_id = ?", *filter.TenantID)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 获取数据
	var apiKeys []models.APIKey
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&apiKeys).Error; err != nil {
		return nil, err
	}

	return &APIKeyListResult{
		Data:     apiKeys,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// RoleRepository 角色仓储实现
type RoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository 创建角色仓储实例
func NewRoleRepository(db *gorm.DB) RoleRepositoryInterface {
	return &RoleRepository{db: db}
}

// Create 创建角色
func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID 根据ID获取角色
func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	var role models.Role
	err := r.db.WithContext(ctx).First(&role, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &role, nil
}

// GetByName 根据名称获取角色
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	var role models.Role
	err := r.db.WithContext(ctx).First(&role, "name = ?", name).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &role, nil
}

// Update 更新角色
func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete 删除角色
func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&models.Role{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// List 列出角色
func (r *RoleRepository) List(ctx context.Context, filter RoleFilter) (*RoleListResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&models.Role{})

	// 应用过滤条件
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.IsSystem != nil {
		query = query.Where("is_system = ?", *filter.IsSystem)
	}
	if filter.TenantID != nil {
		query = query.Where("tenant_id = ?", *filter.TenantID)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 获取数据
	var roles []models.Role
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&roles).Error; err != nil {
		return nil, err
	}

	return &RoleListResult{
		Data:     roles,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

// ExistsByName 检查角色名是否存在
func (r *RoleRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Role{}).
		Where("name = ?", name).
		Count(&count).Error
	return count > 0, err
}
