# Casbin 权限控制集成指南

本文档介绍如何使用 Casbin 替代自定义的轻量级 RBAC 实现，构建企业级的权限控制系统。

## 目录

- [为什么选择 Casbin](#为什么选择-casbin)
- [架构设计](#架构设计)
- [快速开始](#快速开始)
- [权限模型设计](#权限模型设计)
- [集成到 platform-api](#集成到-platform-api)
- [最佳实践](#最佳实践)
- [迁移指南](#迁移指南)

---

## 为什么选择 Casbin

### 与自定义实现对比

| 特性 | Casbin | 自定义实现 | 说明 |
|------|--------|------------|------|
| **权限模型** | RBAC, ABAC, ACL, RESTful | 简单 RBAC | Casbin 支持多种模型 |
| **资源级权限** | ✅ 支持 | ❌ 不支持 | 如：用户只能访问特定服务 |
| **权限继承** | ✅ 支持 | ❌ 不支持 | 角色层级继承 |
| **多租户** | ✅ 内置 | ⚠️ 仅预留 | Casbin 原生支持域（租户） |
| **性能优化** | ✅ 内置缓存 | ❌ 无 | 减少数据库查询 |
| **策略存储** | 多种 Adapter | 仅 PostgreSQL | 支持 File, DB, Redis 等 |
| **生产验证** | ✅ 大量案例 | ❌ 无 | Alibaba, Google 等使用 |
| **维护成本** | ✅ 社区维护 | ❌ 自己维护 | 17k+ GitHub Stars |
| **策略管理** | ✅ 管理界面 | ❌ 无 | Casbin Dashboard |

### 核心优势

1. **成熟稳定**：被多家大公司生产环境使用
2. **功能强大**：支持复杂的权限场景
3. **易于集成**：提供 Go、Java、Node.js 等多语言 SDK
4. **高性能**：内置缓存和策略优化
5. **可视化管理**：提供 Web UI 管理权限策略

---

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────┐
│            Application Layer            │
│  ┌──────────────┐   ┌────────────────┐ │
│  │ platform-api │   │   obs-mcp      │ │
│  │   handlers   │   │   tools        │ │
│  └──────────────┘   └────────────────┘ │
└─────────────────────────────────────────┘
                   ↓
┌─────────────────────────────────────────┐
│         Authorization Layer             │
│  ┌──────────────────────────────────┐  │
│  │      Casbin Enforcer             │  │
│  │  ┌────────────┐  ┌────────────┐ │  │
│  │  │   Model    │  │  Policy    │  │
│  │  │  (RBAC)    │  │  Adapter   │  │
│  │  └────────────┘  └────────────┘ │  │
│  └──────────────────────────────────┘  │
│  ┌──────────────────────────────────┐  │
│  │    Permission Middleware         │  │
│  │  - API Key Auth                  │  │
│  │  - JWT Auth (Future)             │  │
│  │  - Permission Check              │  │
│  └──────────────────────────────────┘  │
└─────────────────────────────────────────┘
                   ↓
┌─────────────────────────────────────────┐
│           Storage Layer                 │
│  ┌──────────┐  ┌──────────┐  ┌───────┐ │
│  │PostgreSQL│  │  Redis   │  │ Memory│ │
│  │ (Policy) │  │ (Cache)  │  │ (Test)│ │
│  └──────────┘  └──────────┘  └───────┘ │
└─────────────────────────────────────────┘
```

### 权限模型选择

针对 Cloud Agent Monitor 的场景，推荐使用 **RBAC with Domains** 模型：

```
用户 → 角色 → 权限
        ↓
      租户
```

这种模型支持：
- 用户在特定租户下拥有特定角色
- 角色继承和权限组合
- 资源级别的权限控制

---

## 快速开始

### 1. 安装依赖

```bash
# 安装 Casbin Core
go get github.com/casbin/casbin/v2

# 安装 GORM Adapter（用于 PostgreSQL 存储）
go get github.com/casbin/gorm-adapter/v2

# 安装 Redis Watcher（用于分布式通知）
go get github.com/casbin/redis-watcher/v2
```

### 2. 创建权限模型配置

创建文件 `configs/casbin/rbac_model.conf`：

```conf
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```

### 3. 创建策略适配器

创建文件 `internal/auth/casbin.go`：

```go
package auth

import (
	"context"
	"fmt"
	"log"

	"cloud-agent-monitor/pkg/config"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CasbinEnforcer Casbin 权限执行器
type CasbinEnforcer struct {
	enforcer *casbin.Enforcer
	adapter  *gormadapter.Adapter
}

// NewCasbinEnforcer 创建 Casbin 执行器
func NewCasbinEnforcer(db *gorm.DB, cfg *config.Config) (*CasbinEnforcer, error) {
	// 创建 GORM 适配器
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin adapter: %w", err)
	}

	// 创建 Enforcer
	enforcer, err := casbin.NewEnforcer("configs/casbin/rbac_model.conf", adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	// 启用自动保存
	enforcer.EnableAutoSave(true)

	ce := &CasbinEnforcer{
		enforcer: enforcer,
		adapter:  adapter,
	}

	// 初始化默认策略
	if err := ce.initDefaultPolicies(); err != nil {
		log.Printf("Warning: failed to init default policies: %v", err)
	}

	return ce, nil
}

// initDefaultPolicies 初始化默认策略
func (ce *CasbinEnforcer) initDefaultPolicies() error {
	// 检查是否已有策略
	policies := ce.enforcer.GetPolicy()
	if len(policies) > 0 {
		return nil // 已有策略，跳过初始化
	}

	log.Println("Initializing default policies...")

	// 定义默认角色权限
	defaultPolicies := [][]string{
		// admin 角色 - 拥有所有权限
		{"admin", "*", "*", "*"},

		// editor 角色 - 服务和配置的增删改查
		{"editor", "*", "/api/v1/services", "GET"},
		{"editor", "*", "/api/v1/services", "POST"},
		{"editor", "*", "/api/v1/services/*", "GET"},
		{"editor", "*", "/api/v1/services/*", "PUT"},
		{"editor", "*", "/api/v1/services/*", "DELETE"},
		{"editor", "*", "/api/v1/configs", "GET"},
		{"editor", "*", "/api/v1/configs", "POST"},
		{"editor", "*", "/api/v1/configs/*", "GET"},
		{"editor", "*", "/api/v1/configs/*", "PUT"},

		// reader 角色 - 只读访问
		{"reader", "*", "/api/v1/services", "GET"},
		{"reader", "*", "/api/v1/services/*", "GET"},
		{"reader", "*", "/api/v1/configs", "GET"},
		{"reader", "*", "/api/v1/configs/*", "GET"},
		{"reader", "*", "/api/v1/audit/*", "GET"},

		// agent 角色 - 用于 MCP 工具访问
		{"agent", "*", "/api/v1/services", "GET"},
		{"agent", "*", "/api/v1/services/*", "GET"},
		{"agent", "*", "/api/v1/configs", "GET"},
		{"agent", "*", "/api/v1/configs/*", "GET"},
	}

	for _, policy := range defaultPolicies {
		_, err := ce.enforcer.AddPolicy(policy)
		if err != nil {
			log.Printf("Warning: failed to add policy %v: %v", policy, err)
		}
	}

	return ce.enforcer.SavePolicy()
}

// Enforce 权限检查
func (ce *CasbinEnforcer) Enforce(sub, dom, obj, act string) (bool, error) {
	return ce.enforcer.Enforce(sub, dom, obj, act)
}

// AddRoleForUser 为用户添加角色
func (ce *CasbinEnforcer) AddRoleForUser(user, role, domain string) (bool, error) {
	return ce.enforcer.AddGroupingPolicy(user, role, domain)
}

// DeleteRoleForUser 移除用户角色
func (ce *CasbinEnforcer) DeleteRoleForUser(user, role, domain string) (bool, error) {
	return ce.enforcer.RemoveGroupingPolicy(user, role, domain)
}

// GetRolesForUser 获取用户的所有角色
func (ce *CasbinEnforcer) GetRolesForUser(user, domain string) ([]string, error) {
	return ce.enforcer.GetRolesForUser(user, domain)
}

// GetUsersForRole 获取角色的所有用户
func (ce *CasbinEnforcer) GetUsersForRole(role, domain string) ([]string, error) {
	return ce.enforcer.GetUsersForRole(role, domain)
}

// AddPermissionForRole 为角色添加权限
func (ce *CasbinEnforcer) AddPermissionForRole(role, domain, resource, action string) (bool, error) {
	return ce.enforcer.AddPolicy(role, domain, resource, action)
}

// DeletePermissionForRole 移除角色权限
func (ce *CasbinEnforcer) DeletePermissionForRole(role, domain, resource, action string) (bool, error) {
	return ce.enforcer.RemovePolicy(role, domain, resource, action)
}

// GetPermissionsForRole 获取角色的所有权限
func (ce *CasbinEnforcer) GetPermissionsForRole(role, domain string) [][]string {
	return ce.enforcer.GetFilteredPolicy(0, role, domain)
}

// CasbinMiddleware Casbin 中间件
func (ce *CasbinEnforcer) CasbinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文获取用户信息
		userID, exists := GetUserIDFromContext(c.Request.Context())
		if !exists {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		// 获取租户 ID（多租户场景）
		domain := GetTenantFromContext(c.Request.Context())
		if domain == "" {
			domain = "default" // 默认租户
		}

		// 获取请求路径和方法
		obj := c.Request.URL.Path
		act := c.Request.Method

		// 权限检查
		ok, err := ce.Enforce(userID.String(), domain, obj, act)
		if err != nil {
			log.Printf("Casbin enforce error: %v", err)
			c.JSON(500, gin.H{"error": "internal error"})
			c.Abort()
			return
		}

		if !ok {
			c.JSON(403, gin.H{"error": "forbidden"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ReloadPolicy 重新加载策略（用于动态更新）
func (ce *CasbinEnforcer) ReloadPolicy() error {
	return ce.enforcer.LoadPolicy()
}

// ClearPolicy 清除所有策略（谨慎使用）
func (ce *CasbinEnforcer) ClearPolicy() error {
	return ce.enforcer.ClearPolicy()
}
```

---

## 权限模型设计

### 1. 基础 RBAC 模型

```
用户  → 角色  → 权限

示例：
alice → admin → (*, *, *)
bob   → editor → (service, read), (service, write)
```

### 2. RBAC with Domains（推荐）

支持多租户场景：

```
用户  → 角色  → 租户  → 权限

示例：
alice → admin   → tenant1 → (所有权限)
bob   → editor  → tenant1 → (服务和配置权限)
charlie → reader → tenant2 → (只读权限)
```

### 3. 资源级权限控制

针对特定资源的权限：

```
alice → admin  → tenant1 → (service:id123, read)
bob   → editor → tenant1 → (service:*, write)
```

### 4. 权限继承

角色层级继承：

```
super_admin → admin → editor → reader
```

配置示例：

```go
// 添加角色继承
ce.enforcer.AddGroupingPolicy("super_admin", "admin", "*")
ce.enforcer.AddGroupingPolicy("admin", "editor", "*")
ce.enforcer.AddGroupingPolicy("editor", "reader", "*")
```

---

## 集成到 platform-api

### 1. 更新 Wire 配置

修改 `cmd/platform-api/wire.go`：

```go
//go:build wireinject

package main

import (
	"github.com/google/wire"
	"gorm.io/gorm"

	"cloud-agent-monitor/internal/auth"
	"cloud-agent-monitor/internal/platform"
	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/pkg/config"
)

func InitializeApp() (*App, error) {
	wire.Build(
		// 配置
		provideConfig,

		// 数据库
		provideDatabase,

		// Casbin
		provideCasbinEnforcer,

		// Repositories
		storage.NewServiceRepository,
		storage.NewUserRepository,
		storage.NewAPIKeyRepository,

		// Auth Services
		provideAPIKeyService,
		provideAuthService,

		// HTTP Server
		provideHTTPServer,

		// App
		provideApp,
	)
	return nil, nil
}

func provideCasbinEnforcer(db *gorm.DB, cfg *config.Config) (*auth.CasbinEnforcer, error) {
	return auth.NewCasbinEnforcer(db, cfg)
}

func provideAPIKeyService(apiKeyRepo storage.APIKeyRepositoryInterface) *auth.APIKeyService {
	return auth.NewAPIKeyService(apiKeyRepo)
}

func provideAuthService(
	userRepo storage.UserRepositoryInterface,
	apiKeyRepo storage.APIKeyRepositoryInterface,
	casbinEnforcer *auth.CasbinEnforcer,
) *auth.AuthMiddleware {
	return auth.NewAuthMiddleware(userRepo, apiKeyRepo, casbinEnforcer)
}

func provideHTTPServer(
	db *gorm.DB,
	serviceRepo storage.ServiceRepositoryInterface,
	authMiddleware *auth.AuthMiddleware,
	casbinEnforcer *auth.CasbinEnforcer,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		// ... 健康检查逻辑
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 认证中间件
		v1.Use(authMiddleware.RequireAuth())
		// 权限检查中间件
		v1.Use(casbinEnforcer.CasbinMiddleware())

		// 服务路由
		platform.RegisterRoutes(v1, serviceRepo)
	}

	return r
}
```

### 2. 简化的认证中间件

修改 `internal/auth/auth.go`，移除权限检查逻辑：

```go
package auth

import (
	"context"
	"net/http"
	"strings"

	"cloud-agent-monitor/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthMiddleware 认证中间件（只负责认证，不负责授权）
type AuthMiddleware struct {
	userRepo      storage.UserRepositoryInterface
	apiKeyRepo    storage.APIKeyRepositoryInterface
	casbinEnforcer *CasbinEnforcer
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(
	userRepo storage.UserRepositoryInterface,
	apiKeyRepo storage.APIKeyRepositoryInterface,
	casbinEnforcer *CasbinEnforcer,
) *AuthMiddleware {
	return &AuthMiddleware{
		userRepo:      userRepo,
		apiKeyRepo:    apiKeyRepo,
		casbinEnforcer: casbinEnforcer,
	}
}

// RequireAuth 要求认证的中间件
func (a *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var userID uuid.UUID
		var apiKeyID uuid.UUID
		var err error

		// API Key 认证
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			userID, apiKeyID, err = a.authenticateAPIKey(c.Request.Context(), apiKey)
		} else {
			err = &AuthError{Code: "MISSING_AUTH", Message: "missing authentication credentials"}
		}

		if err != nil {
			a.handleAuthError(c, err)
			return
		}

		// 将用户信息存入 context
		ctx := context.WithValue(c.Request.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, APIKeyIDKey, apiKeyID)
		c.Request = c.Request.WithContext(ctx)

		c.Set(string(UserIDKey), userID)
		c.Set(string(APIKeyIDKey), apiKeyID)

		c.Next()
	}
}

// authenticateAPIKey 使用 API Key 进行认证
func (a *AuthMiddleware) authenticateAPIKey(ctx context.Context, apiKey string) (uuid.UUID, uuid.UUID, error) {
	// 验证 API Key 格式
	if !strings.HasPrefix(apiKey, "obs_") {
		return uuid.Nil, uuid.Nil, &AuthError{
			Code:    "INVALID_API_KEY_FORMAT",
			Message: "invalid API key format",
		}
	}

	// 计算密钥哈希
	keyHash := hashAPIKey(apiKey)

	// 查询数据库
	key, err := a.apiKeyRepo.GetByKeyHash(ctx, keyHash)
	if err != nil {
		return uuid.Nil, uuid.Nil, &AuthError{
			Code:    "
INVALID_API_KEY",
			Message: "invalid API key",
		}
	}

	// 检查密钥状态
	if !key.IsActive {
		return uuid.Nil, uuid.Nil, &AuthError{
			Code:    "API_KEY_DISABLED",
			Message: "API key is disabled",
		}
	}

	return key.UserID, key.ID, nil
}

// handleAuthError 处理认证错误
func (a *AuthMiddleware) handleAuthError(c *gin.Context, err error) {
	var authErr *AuthError
	var statusCode int

	if ae, ok := err.(*AuthError); ok {
		authErr = ae
		switch authErr.Code {
		case "MISSING_AUTH":
			statusCode = http.StatusUnauthorized
		case "INVALID_API_KEY":
			statusCode = http.StatusUnauthorized
		case "API_KEY_DISABLED":
			statusCode = http.StatusForbidden
		default:
			statusCode = http.StatusInternalServerError
		}
	} else {
		authErr = &AuthError{Code: "INTERNAL_ERROR", Message: "internal error"}
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, gin.H{"error": authErr.Message})
	c.Abort()
}

// Helper functions
func hashAPIKey(key string) string {
	// 使用 SHA256 哈希
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// Context keys
type ContextKey string

const (
	UserIDKey   ContextKey = "user_id"
	APIKeyIDKey ContextKey = "api_key_id"
)

// AuthError 认证错误
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
```

---

## 最佳实践

### 1. 权限策略管理

```go
// 权限策略服务
type PolicyService struct {
	enforcer *CasbinEnforcer
}

// GrantServiceAccess 授予服务访问权限
func (s *PolicyService) GrantServiceAccess(
	ctx context.Context,
	userID, role, tenantID string,
	serviceID string,
	actions []string,
) error {
	for _, action := range actions {
		resource := fmt.Sprintf("/api/v1/services/%s", serviceID)
		_, err := s.enforcer.AddPermissionForRole(role, tenantID, resource, action)
		if err != nil {
			return fmt.Errorf("failed to grant permission: %w", err)
		}
	}
	return nil
}

// RevokeServiceAccess 撤销服务访问权限
func (s *PolicyService) RevokeServiceAccess(
	ctx context.Context,
	role, tenantID string,
	serviceID string,
) error {
	resource := fmt.Sprintf("/api/v1/services/%s", serviceID)
	_, err := s.enforcer.enforcer.RemoveFilteredPolicy(0, role, tenantID, resource)
	return err
}
```

### 2. 权限缓存优化

```go
// 启用 Redis 缓存
import "github.com/casbin/redis-watcher/v2"

func setupCache(cfg *config.Config) error {
	w, err := watcher.NewWatcher(fmt.Sprintf("%s:6379", cfg.Redis.Host), watcher.WatcherOptions{
		Password: cfg.Redis.Password,
	})
	if err != nil {
		return err
	}

	enforcer.SetWatcher(w)
	return nil
}
```

### 3. 权限审计

```go
// 记录权限变更
func (s *PolicyService) auditPolicyChange(
	ctx context.Context,
	action, user, role, resource string,
) {
	auditLog.Info("Policy changed",
		"action", action,
		"user", user,
		"role", role,
		"resource", resource,
		"timestamp", time.Now(),
	)
}
```

---

## 迁移指南

### 从自定义 RBAC 迁移到 Casbin

#### 步骤 1：备份数据

```bash
# 导出现有用户角色关系
pg_dump -U obs -d obs_platform -t users -t roles -t user_roles > backup_auth.sql
```

#### 步骤 2：安装 Casbin

```bash
go get github.com/casbin/casbin/v2
go get github.com/casbin/gorm-adapter/v2
```

#### 步骤 3：数据迁移

```go
// 迁移脚本
func migrateToCasbin(db *gorm.DB, enforcer *CasbinEnforcer) error {
	// 读取旧的用户角色关系
	var userRoles []struct {
		UserID string
		RoleName string
		TenantID string
	}

	db.Table("user_roles").
		Select("user_roles.user_id, roles.name as role_name, users.tenant_id").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Joins("JOIN users ON users.id = user_roles.user_id").
		Scan(&userRoles)

	// 迁移到 Casbin
	for _, ur := range userRoles {
		domain := ur.TenantID
		if domain == "" {
			domain = "default"
		}

		_, err := enforcer.AddRoleForUser(ur.UserID, ur.RoleName, domain)
		if err != nil {
			log.Printf("Failed to migrate user role: %v", err)
		}
	}

	return enforcer.enforcer.SavePolicy()
}
```

#### 步骤 4：验证迁移

```bash
# 运行测试
go test ./internal/auth/... -v

# 验证权限
curl -H "X-API-Key: obs_xxx" http://localhost:8080/api/v1/services
```

---

## 参考资料

### 官方文档

- [Casbin 官网](https://casbin.org/)
- [Casbin GitHub](https://github.com/casbin/casbin)
- [Casbin Editor (在线策略编辑器)](https://casbin.org/editor)

### 学习资源

- [Casbin 权限模型详解](https://casbin.org/docs/supported-models)
- [RBAC with Domains](https://casbin.org/docs/rbac-with-domains)
- [性能优化最佳实践](https://casbin.org/docs/performance)

### 相关项目

- [Casbin Dashboard](https://github.com/casbin/casbin-dashboard) - Web UI 管理界面
- [Casbin Redis Watcher](https://github.com/casbin/redis-watcher) - 分布式通知
- [Casbin GORM Adapter](https://github.com/casbin/gorm-adapter) - GORM 存储

---

## 更新记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2025-01-XX | v1.0 | 初始版本，基于 Casbin 的权限控制方案 |

如有问题或建议，请提交 Issue 或联系开发团队。