package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/google/uuid"
)

const (
	// API Key prefix
	APIKeyPrefix = "obs_"
)

// hashAPIKey 计算 API Key 的 SHA256 哈希值
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// APIKeyService API密钥管理服务
type APIKeyService struct {
	repo storage.APIKeyRepositoryInterface
}

// NewAPIKeyService 创建API密钥服务实例
func NewAPIKeyService(repo storage.APIKeyRepositoryInterface) *APIKeyService {
	return &APIKeyService{
		repo: repo,
	}
}

// CreateAPIKeyRequest 创建API密钥请求
type CreateAPIKeyRequest struct {
	UserID      uuid.UUID
	Name        string
	Permissions models.Permissions
	ExpiresIn   *time.Duration // 过期时间，nil表示永不过期
}

// APIKeyResponse API密钥响应（包含明文密钥，只在创建时返回一次）
type APIKeyResponse struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	Key         string             `json:"key"` // 明文密钥，只在创建时返回
	Prefix      string             `json:"prefix"`
	Permissions models.Permissions `json:"permissions"`
	ExpiresAt   *time.Time         `json:"expires_at,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

// CreateAPIKey 创建新的API密钥
// 注意：明文密钥只在创建时返回一次，后续无法再获取
func (s *APIKeyService) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*APIKeyResponse, error) {
	// 验证请求
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// 生成密钥
	plainKey, err := generateSecureKey(32) // 32字节 = 64个十六进制字符
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// 添加前缀
	fullKey := APIKeyPrefix + plainKey

	// 计算哈希
	keyHash := hashAPIKey(fullKey)

	// 提取前缀（用于识别）
	prefix := extractPrefix(fullKey)

	// 计算过期时间
	var expiresAt *time.Time
	if req.ExpiresIn != nil {
		exp := time.Now().Add(*req.ExpiresIn)
		expiresAt = &exp
	}

	// 创建数据库记录
	apiKey := &models.APIKey{
		UserID:      req.UserID,
		Name:        req.Name,
		Key:         fullKey, // 数据库中存储完整密钥（用于显示）
		KeyHash:     keyHash,
		Prefix:      prefix,
		Permissions: req.Permissions,
		ExpiresAt:   expiresAt,
		IsActive:    true,
	}

	if err := s.repo.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to save API key: %w", err)
	}

	return &APIKeyResponse{
		ID:          apiKey.ID,
		Name:        apiKey.Name,
		Key:         fullKey, // 只在创建时返回明文密钥
		Prefix:      prefix,
		Permissions: apiKey.Permissions,
		ExpiresAt:   expiresAt,
		CreatedAt:   apiKey.CreatedAt,
	}, nil
}

// ValidateAPIKey 验证API密钥
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, key string) (*models.APIKey, error) {
	// 检查格式
	if !strings.HasPrefix(key, APIKeyPrefix) {
		return nil, errors.New("invalid API key format")
	}

	// 计算哈希
	keyHash := hashAPIKey(key)

	// 查询数据库
	apiKey, err := s.repo.GetByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// 检查状态
	if !apiKey.IsActive {
		return nil, errors.New("API key is disabled")
	}

	// 检查过期
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("API key has expired")
	}

	// 更新最后使用时间（异步）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.UpdateLastUsed(ctx, apiKey.ID, time.Now())
	}()

	return apiKey, nil
}

// ListUserAPIKeys 列出用户的所有API密钥（不包含完整密钥）
func (s *APIKeyService) ListUserAPIKeys(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	keys, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	// 清理敏感信息
	for i := range keys {
		keys[i].Key = keys[i].Prefix + "..." + keys[i].Key[len(keys[i].Key)-4:]
	}

	return keys, nil
}

// RevokeAPIKey 撤销API密钥
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, userID, keyID uuid.UUID) error {
	// 获取密钥
	apiKey, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("API key not found")
	}

	// 验证所有权
	if apiKey.UserID != userID {
		return errors.New("unauthorized: API key does not belong to user")
	}

	// 设置为不活跃
	if err := s.repo.Deactivate(ctx, keyID); err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}

// RegenerateAPIKey 重新生成API密钥（撤销旧的，创建新的）
func (s *APIKeyService) RegenerateAPIKey(ctx context.Context, userID, keyID uuid.UUID) (*APIKeyResponse, error) {
	// 获取旧密钥
	oldKey, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	// 验证所有权
	if oldKey.UserID != userID {
		return nil, errors.New("unauthorized: API key does not belong to user")
	}

	// 创建新密钥
	newKey, err := s.CreateAPIKey(ctx, CreateAPIKeyRequest{
		UserID:      userID,
		Name:        oldKey.Name,
		Permissions: oldKey.Permissions,
		ExpiresIn:   nil, // 重新生成的密钥不过期
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new API key: %w", err)
	}

	// 撤销旧密钥
	if err := s.repo.Deactivate(ctx, keyID); err != nil {
		// 不返回错误，因为新密钥已经创建成功
		// 但记录日志
		fmt.Printf("warning: failed to deactivate old API key %s: %v\n", keyID, err)
	}

	return newKey, nil
}

// GetActiveAPIKeys 获取用户的活跃API密钥（排除过期的）
func (s *APIKeyService) GetActiveAPIKeys(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	keys, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 过滤活跃且未过期的密钥
	var activeKeys []models.APIKey
	now := time.Now()
	for _, key := range keys {
		if key.IsActive && (key.ExpiresAt == nil || key.ExpiresAt.After(now)) {
			activeKeys = append(activeKeys, key)
		}
	}

	return activeKeys, nil
}

// validateCreateRequest 验证创建请求
func (s *APIKeyService) validateCreateRequest(req CreateAPIKeyRequest) error {
	if req.UserID == uuid.Nil {
		return errors.New("user ID is required")
	}

	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}

	if len(req.Name) > 255 {
		return errors.New("name must be less than 255 characters")
	}

	return nil
}

// Helper functions

// generateSecureKey 生成安全的随机密钥
func generateSecureKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length*2], nil // 转换为十六进制并取前 length*2 个字符
}

// extractPrefix 从完整密钥中提取安全的前缀
func extractPrefix(key string) string {
	// 返回前缀 + 前4个字符，例如 "obs_a1b2"
	if len(key) > len(APIKeyPrefix)+4 {
		return key[:len(APIKeyPrefix)+4]
	}
	return key[:len(APIKeyPrefix)+2] // 如果密钥太短，返回前缀+前2个字符
}

// CleanupExpiredKeys 清理过期的API密钥（定时任务调用）
func (s *APIKeyService) CleanupExpiredKeys(ctx context.Context) error {
	// 这个功能需要数据库支持批量更新过期密钥
	// 简化实现：可以定期扫描并标记过期的密钥
	return nil
}
