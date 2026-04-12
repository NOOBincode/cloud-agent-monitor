package auth

import (
	"context"
	"fmt"
	"log"
	"strings"

	"cloud-agent-monitor/pkg/config"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Context keys for storing authentication information
type ContextKey string

const (
	UserIDKey   ContextKey = "user_id"
	APIKeyIDKey ContextKey = "api_key_id"
	TenantIDKey ContextKey = "tenant_id"
)

// CasbinEnforcer Casbin 权限执行器
// 暴露 Enforcer 以便用户可以直接使用 Casbin 的完整 API
type CasbinEnforcer struct {
	Enforcer *casbin.Enforcer
}

// NewCasbinEnforcer 创建 Casbin 执行器
func NewCasbinEnforcer(cfg *config.Config) (*CasbinEnforcer, error) {
	enforcer, err := casbin.NewEnforcer("configs/casbin/rbac_model.conf", "configs/casbin/policy.csv")
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	enforcer.AddNamedMatchingFunc("obj", "KeyMatch2", util.KeyMatch2)

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	enforcer.EnableAutoSave(true)

	ce := &CasbinEnforcer{
		Enforcer: enforcer,
	}

	if err := ce.initDefaultPolicies(); err != nil {
		log.Printf("Warning: failed to init default policies: %v", err)
	}

	return ce, nil
}

// initDefaultPolicies 初始化默认策略
func (ce *CasbinEnforcer) initDefaultPolicies() error {
	// 检查是否已有策略（修复：处理两个返回值）
	policies, _ := ce.Enforcer.GetPolicy()
	if len(policies) > 0 {
		return nil // 已有策略，跳过初始化
	}

	log.Println("Initializing default Casbin policies...")

	// 定义默认角色权限（基于 RESTful API）
	defaultPolicies := [][]string{
		// admin 角色 - 拥有所有权限
		{"admin", "*", "/api/v1/*", "*"},

		// editor 角色 - 服务、配置、告警的增删改查
		{"editor", "*", "/api/v1/services", "GET"},
		{"editor", "*", "/api/v1/services", "POST"},
		{"editor", "*", "/api/v1/services/*", "GET"},
		{"editor", "*", "/api/v1/services/*", "PUT"},
		{"editor", "*", "/api/v1/services/*", "DELETE"},
		{"editor", "*", "/api/v1/configs", "GET"},
		{"editor", "*", "/api/v1/configs", "POST"},
		{"editor", "*", "/api/v1/configs/*", "GET"},
		{"editor", "*", "/api/v1/configs/*", "PUT"},
		{"editor", "*", "/api/v1/configs/*", "DELETE"},
		{"editor", "*", "/api/v1/alerts", "GET"},
		{"editor", "*", "/api/v1/alerts", "POST"},
		{"editor", "*", "/api/v1/alerts/*", "GET"},
		{"editor", "*", "/api/v1/alerts/*", "POST"},
		{"editor", "*", "/api/v1/silences", "GET"},
		{"editor", "*", "/api/v1/silences", "POST"},
		{"editor", "*", "/api/v1/silences/*", "DELETE"},
		{"editor", "*", "/api/v1/alertmanager/*", "GET"},

		// reader 角色 - 只读访问
		{"reader", "*", "/api/v1/services", "GET"},
		{"reader", "*", "/api/v1/services/*", "GET"},
		{"reader", "*", "/api/v1/configs", "GET"},
		{"reader", "*", "/api/v1/configs/*", "GET"},
		{"reader", "*", "/api/v1/alerts", "GET"},
		{"reader", "*", "/api/v1/alerts/*", "GET"},
		{"reader", "*", "/api/v1/silences", "GET"},
		{"reader", "*", "/api/v1/alertmanager/*", "GET"},
		{"reader", "*", "/api/v1/audit/*", "GET"},

		// agent 角色 - 用于 MCP 工具访问
		{"agent", "*", "/api/v1/services", "GET"},
		{"agent", "*", "/api/v1/services/*", "GET"},
		{"agent", "*", "/api/v1/configs", "GET"},
		{"agent", "*", "/api/v1/configs/*", "GET"},
		{"agent", "*", "/api/v1/alerts", "GET"},
		{"agent", "*", "/api/v1/alerts", "POST"},
		{"agent", "*", "/api/v1/alerts/*", "GET"},
		{"agent", "*", "/api/v1/silences", "GET"},
		{"agent", "*", "/api/v1/alertmanager/*", "GET"},
	}

	for _, policy := range defaultPolicies {
		_, err := ce.Enforcer.AddPolicy(policy)
		if err != nil {
			log.Printf("Warning: failed to add policy %v: %v", policy, err)
		}
	}

	// 添加角色继承（可选）
	roleInheritances := [][]string{
		{"admin", "editor", "*"},  // admin 继承 editor 的权限
		{"editor", "reader", "*"}, // editor 继承 reader 的权限
	}

	for _, inheritance := range roleInheritances {
		_, err := ce.Enforcer.AddGroupingPolicy(inheritance)
		if err != nil {
			log.Printf("Warning: failed to add role inheritance %v: %v", inheritance, err)
		}
	}

	return ce.Enforcer.SavePolicy()
}

// Enforce 权限检查（保留以支持未来添加缓存、审计等逻辑）
func (ce *CasbinEnforcer) Enforce(sub, dom, obj, act string) (bool, error) {
	return ce.Enforcer.Enforce(sub, dom, obj, act)
}

// EnforceWithContext 带上下文的权限检查
func (ce *CasbinEnforcer) EnforceWithContext(ctx context.Context, userID, tenantID, path, method string) (bool, error) {
	domain := tenantID
	if domain == "" {
		domain = "default"
	}
	return ce.Enforce(userID, domain, path, method)
}

// AddRoleForUser 为用户添加角色（保留以添加审计日志）
func (ce *CasbinEnforcer) AddRoleForUser(user, role, domain string) (bool, error) {
	ok, err := ce.Enforcer.AddGroupingPolicy(user, role, domain)
	if ok && err == nil {
		// TODO: 添加审计日志
		log.Printf("Role assigned: user=%s, role=%s, domain=%s", user, role, domain)
	}
	return ok, err
}

// DeleteRole 删除角色（保留：包含多个步骤的复杂逻辑）
func (ce *CasbinEnforcer) DeleteRole(role string) error {
	// 删除角色的所有权限
	_, err := ce.Enforcer.RemoveFilteredPolicy(0, role)
	if err != nil {
		return err
	}

	// 删除所有用户的该角色
	_, err = ce.Enforcer.RemoveFilteredGroupingPolicy(1, role)
	if err != nil {
		return err
	}

	// TODO: 添加审计日志
	log.Printf("Role deleted: role=%s", role)

	return ce.Enforcer.SavePolicy()
}

// CasbinMiddleware Casbin 权限检查中间件
func (ce *CasbinEnforcer) CasbinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetUserIDFromContext(c.Request.Context())
		if !exists {
			if gin.Mode() == gin.DebugMode {
				log.Printf("CasbinMiddleware: no userID in context, path=%s", c.Request.URL.Path)
			}
			c.JSON(401, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "authentication required",
			})
			c.Abort()
			return
		}

		domain := GetTenantFromContext(c.Request.Context())
		if domain == "" {
			domain = "default"
		}

		obj := c.Request.URL.Path
		act := c.Request.Method

		ok, err := ce.Enforce(userID.String(), domain, obj, act)
		if err != nil {
			log.Printf("Casbin enforce error: %v", err)
			c.JSON(500, gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "authorization check failed",
			})
			c.Abort()
			return
		}

		if !ok {
			log.Printf("Permission denied: user=%s, domain=%s, path=%s, method=%s",
				userID, domain, obj, act)
			c.JSON(403, gin.H{
				"code":    "FORBIDDEN",
				"message": "permission denied",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Resource-based permission helpers

// GrantServiceAccess 授予服务访问权限（保留：封装了资源路径构建和批量操作）
func (ce *CasbinEnforcer) GrantServiceAccess(role, domain, serviceID string, actions []string) error {
	for _, action := range actions {
		resource := fmt.Sprintf("/api/v1/services/%s", serviceID)
		_, err := ce.Enforcer.AddPolicy(role, domain, resource, action)
		if err != nil {
			return fmt.Errorf("failed to grant permission for service %s: %w", serviceID, err)
		}
	}

	// TODO: 添加审计日志
	log.Printf("Service access granted: role=%s, service=%s, actions=%v", role, serviceID, actions)

	return ce.Enforcer.SavePolicy()
}

// RevokeServiceAccess 撤销服务访问权限（保留：封装了资源路径构建）
func (ce *CasbinEnforcer) RevokeServiceAccess(role, domain, serviceID string) error {
	resource := fmt.Sprintf("/api/v1/services/%s", serviceID)
	_, err := ce.Enforcer.RemoveFilteredPolicy(1, domain, resource)
	if err != nil {
		return err
	}

	// TODO: 添加审计日志
	log.Printf("Service access revoked: role=%s, service=%s", role, serviceID)

	return ce.Enforcer.SavePolicy()
}

// GrantConfigAccess 授予配置访问权限（保留：封装了资源路径构建和批量操作）
func (ce *CasbinEnforcer) GrantConfigAccess(role, domain, configID string, actions []string) error {
	for _, action := range actions {
		resource := fmt.Sprintf("/api/v1/configs/%s", configID)
		_, err := ce.Enforcer.AddPolicy(role, domain, resource, action)
		if err != nil {
			return fmt.Errorf("failed to grant permission for config %s: %w", configID, err)
		}
	}

	// TODO: 添加审计日志
	log.Printf("Config access granted: role=%s, config=%s, actions=%v", role, configID, actions)

	return ce.Enforcer.SavePolicy()
}

// RevokeConfigAccess 撤销配置访问权限（保留：封装了资源路径构建）
func (ce *CasbinEnforcer) RevokeConfigAccess(role, domain, configID string) error {
	resource := fmt.Sprintf("/api/v1/configs/%s", configID)
	_, err := ce.Enforcer.RemoveFilteredPolicy(1, domain, resource)
	if err != nil {
		return err
	}

	// TODO: 添加审计日志
	log.Printf("Config access revoked: role=%s, config=%s", role, configID)

	return ce.Enforcer.SavePolicy()
}

// Tenant management helpers

// CreateTenantDomain 创建租户域（保留：批量创建租户的默认权限）
func (ce *CasbinEnforcer) CreateTenantDomain(tenantID string) error {
	// 为新租户创建默认角色和权限
	defaultPolicies := [][]string{
		{"admin", tenantID, "/api/v1/*", "*"},
		{"editor", tenantID, "/api/v1/services", "GET"},
		{"editor", tenantID, "/api/v1/services", "POST"},
		{"editor", tenantID, "/api/v1/services/*", "GET"},
		{"editor", tenantID, "/api/v1/services/*", "PUT"},
		{"reader", tenantID, "/api/v1/services", "GET"},
		{"reader", tenantID, "/api/v1/services/*", "GET"},
	}

	for _, policy := range defaultPolicies {
		_, err := ce.Enforcer.AddPolicy(policy)
		if err != nil {
			log.Printf("Warning: failed to add tenant policy %v: %v", policy, err)
		}
	}

	// TODO: 添加审计日志
	log.Printf("Tenant domain created: tenant=%s", tenantID)

	return ce.Enforcer.SavePolicy()
}

// DeleteTenantDomain 删除租户域（保留：批量删除租户的所有权限）
func (ce *CasbinEnforcer) DeleteTenantDomain(tenantID string) error {
	// 删除该租户的所有策略
	_, err := ce.Enforcer.RemoveFilteredPolicy(1, tenantID)
	if err != nil {
		return err
	}

	// 删除该租户的所有用户角色关系
	_, err = ce.Enforcer.RemoveFilteredGroupingPolicy(1, tenantID)
	if err != nil {
		return err
	}

	// TODO: 添加审计日志
	log.Printf("Tenant domain deleted: tenant=%s", tenantID)

	return ce.Enforcer.SavePolicy()
}

// Helper functions for context

// GetTenantFromContext 从上下文获取租户ID
func GetTenantFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(TenantIDKey).(string); ok {
		return tenantID
	}
	return ""
}

// GetUserIDFromContext 从上下文获取用户ID
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// MatchFunc 自定义匹配函数（用于更复杂的路径匹配）
func MatchFunc(key1, key2 string) bool {
	// 简单的通配符匹配
	if key2 == "*" {
		return true
	}

	// 支持路径通配符，如 /api/v1/services/*
	if strings.HasSuffix(key2, "/*") {
		prefix := strings.TrimSuffix(key2, "/*")
		return strings.HasPrefix(key1, prefix+"/") || key1 == prefix
	}

	return key1 == key2
}
