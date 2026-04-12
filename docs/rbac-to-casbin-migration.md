# 从自定义 RBAC 迁移到 Casbin 指南

本文档详细说明如何从自定义的轻量级 RBAC 实现迁移到基于 Casbin 的企业级权限控制方案。

## 目录

- [迁移原因](#迁移原因)
- [代码清理](#代码清理)
- [迁移步骤](#迁移步骤)
- [数据迁移](#数据迁移)
- [验证测试](#验证测试)
- [回滚方案](#回滚方案)

---

## 迁移原因

### 自定义 RBAC 存在的问题

| 问题类型 | 具体问题 | 影响 |
|---------|---------|------|
| **重复造轮子** | 自己实现了完整的 RBAC 系统 | 维护成本高，功能有限 |
| **设计简陋** | 只有 7 个硬编码权限 | 无法满足复杂业务需求 |
| **功能缺失** | 不支持资源级权限、权限继承、多租户 | 扩展性差 |
| **性能问题** | 没有缓存机制 | 数据库压力大 |
| **安全风险** | 没有经过大规模生产验证 | 潜在安全漏洞 |
| **社区支持** | 无文档、无案例、无社区 | 学习成本高 |

### Casbin 的优势

| 优势 | 说明 |
|------|------|
| ✅ **成熟稳定** | 17k+ GitHub Stars，被多家大公司使用 |
| ✅ **功能强大** | 支持 RBAC、ABAC、ACL、RESTful 等多种模型 |
| ✅ **资源级权限** | 支持细粒度的资源访问控制 |
| ✅ **权限继承** | 支持角色层级和权限继承 |
| ✅ **多租户** | 内置域（Domain）概念，原生支持多租户 |
| ✅ **高性能** | 内置缓存机制，支持 Redis 分布式缓存 |
| ✅ **多种存储** | 支持 MySQL、PostgreSQL、Redis、MongoDB 等 |
| ✅ **可视化管理** | 提供 Casbin Dashboard Web UI |

---

## 代码清理

### 需要删除的文件

以下文件是实现自定义 RBAC 的代码，应该删除或重命名备份：

```bash
# 1. 权限检查相关（完全由 Casbin 替代）
❌ internal/auth/rbac.go                    # 删除：自定义的权限检查辅助函数
❌ internal/auth/init.go                    # 删除：自定义的角色初始化逻辑

# 2. 权限模型相关（简化或删除）
⚠️  internal/storage/models/user.go         # 修改：删除 Permissions 字段，保留 User/Role/APIKey 模型

# 3. 文档和示例（需要更新）
❌ docs/auth-guide.md                       # 删除：自定义 RBAC 的使用文档
❌ docs/auth-integration-example.go         # 删除：自定义 RBAC 的集成示例

# 4. 测试文件（如果有）
❌ internal/auth/rbac_test.go               # 删除：RBAC 测试
❌ internal/auth/init_test.go               # 删除：初始化测试
```

### 需要保留的文件

以下文件提供的功能 Casbin 不包含，应该保留：

```bash
# 1. 用户管理（Casbin 不提供）
✅ internal/storage/models/user.go          # 保留：User 模型（删除 Permissions 字段）
✅ internal/storage/user_repository.go      # 保留：用户数据访问

# 2. API Key 管理（Casbin 不提供）
✅ internal/auth/apikey.go                  # 保留：API Key 生成和管理
✅ internal/storage/models/user.go          # 保留：APIKey 模型

# 3. 认证中间件（简化后保留）
⚠️  internal/auth/auth.go                   # 保留但简化：移除权限检查逻辑
```

### 需要修改的文件

#### 1. `internal/storage/models/user.go`

删除 Permissions 字段：

```go
// ❌ 删除这个结构体
type Permissions struct {
    ServiceRead   bool `json:"service_read"`
    ServiceWrite  bool `json:"service_write"`
    ServiceDelete bool `json:"service_delete"`
    ConfigRead    bool `json:"config_read"`
    ConfigWrite   bool `json:"config_write"`
    AuditRead     bool `json:"audit_read"`
    Admin         bool `json:"admin"`
}

// ✅ 保留 User 和 Role 模型，但删除 Permissions 字段
type Role struct {
    ID          uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
    Name        string         `gorm:"type:varchar(100);uniqueIndex;not null" json:"name"`
    Description string         `gorm:"type:text" json:"description"`
    IsSystem    bool           `gorm:"default:false;index" json:"is_system"`
    // ❌ 删除这行
    // Permissions Permissions    `gorm:"type:json;serializer:json" json:"permissions"`
    TenantID    *uuid.UUID     `gorm:"type:char(36);index" json:"tenant_id,omitempty"`
    CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// ✅ 保留 APIKey 模型，但删除 Permissions 字段
type APIKey struct {
    ID          uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
    UserID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"user_id"`
    Name        string         `gorm:"type:varchar(255);not null" json:"name"`
    Key         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"key"`
    KeyHash     string         `gorm:"type:varchar(255);not null" json:"-"`
    Prefix      string         `gorm:"type:varchar(20);not null" json:"prefix"`
    // ❌ 删除这行
    // Permissions Permissions    `gorm:"type:json;serializer:json" json:"permissions"`
    ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
    LastUsedAt  *time.Time     `json:"last_used_at,omitempty"`
    IsActive    bool           `gorm:"default:true;index" json:"is_active"`
    TenantID    *uuid.UUID     `gorm:"type:char(36);index" json:"tenant_id,omitempty"`
    CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
    User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
```

#### 2. `internal/auth/auth.go`

简化认证中间件，移除权限检查：

```go
// ✅ 简化后的认证中间件（只负责认证）
type AuthMiddleware struct {
    userRepo      storage.UserRepositoryInterface
    apiKeyRepo    storage.APIKeyRepositoryInterface
    // ❌ 删除：不再需要 CasbinEnforcer（权限检查由中间件负责）
    // casbinEnforcer *CasbinEnforcer
}

// ✅ 简化后的 RequireAuth（只做认证，不做授权）
func (a *AuthMiddleware) RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 认证逻辑保持不变
        var userID uuid.UUID
        var apiKeyID uuid.UUID
        var err error

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

// ❌ 删除 RequirePermission 方法（由 Casbin 中间件替代）
// ❌ 删除 OptionalAuth 方法（暂不需要）
// ❌ 删除所有权限检查相关的代码
```

---

## 迁移步骤

### 第一步：备份现有数据

```bash
# 1. 备份数据库
mysqldump -u root -p obs_platform > backup_before_casbin_$(date +%Y%m%d).sql

# 2. 备份代码
git add .
git commit -m "backup: before migrating to Casbin"
git checkout -b feature/casbin-migration
```

### 第二步：安装 Casbin 依赖

```bash
# 安装 Casbin 核心库
go get github.com/casbin/casbin/v2

# 安装 GORM 适配器（用于 MySQL 存储）
go get github.com/casbin/gorm-adapter/v2

# 可选：安装 Redis Watcher（用于分布式通知）
go get github.com/casbin/redis-watcher/v2

# 更新依赖
go mod tidy
```

### 第三步：创建 Casbin 配置文件

创建 `configs/casbin/rbac_model.conf`：

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

### 第四步：创建 Casbin 集成代码

创建 `internal/auth/casbin.go`（已在前面创建）。

### 第五步：更新 Wire 配置

修改 `cmd/platform-api/wire.go`：

```go
func InitializeApp() (*App, error) {
    wire.Build(
        // 配置
        provideConfig,

        // 数据库
        provideDatabase,

        // ✅ 新增：Casbin
        provideCasbinEnforcer,

        // Repositories
        storage.NewServiceRepository,
        storage.NewUserRepository,
        storage.NewAPIKeyRepository,

        // ✅ 修改：简化后的认证服务
        provideAuthService,

        // HTTP Server
        provideHTTPServer,

        // App
        provideApp,
    )
    return nil, nil
}

// ✅ 新增：提供 Casbin Enforcer
func provideCasbinEnforcer(db *gorm.DB, cfg *config.Config) (*auth.CasbinEnforcer, error) {
    return auth.NewCasbinEnforcer(db, cfg)
}

// ✅ 修改：简化认证服务
func provideAuthService(
    userRepo storage.UserRepositoryInterface,
    apiKeyRepo storage.APIKeyRepositoryInterface,
) *auth.AuthMiddleware {
    return auth.NewAuthMiddleware(userRepo, apiKeyRepo)
}

// ✅ 修改：HTTP Server 集成 Casbin
func provideHTTPServer(
    db *gorm.DB,
    serviceRepo storage.ServiceRepositoryInterface,
    authMiddleware *auth.AuthMiddleware,
    casbinEnforcer *auth.CasbinEnforcer, // ✅ 新增参数
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
        // ✅ 认证中间件
        v1.Use(authMiddleware.RequireAuth())
        
        // ✅ 权限检查中间件（新增）
        v1.Use(casbinEnforcer.CasbinMiddleware())

        // 服务路由
        platform.RegisterRoutes(v1, serviceRepo)
    }

    return r
}
```

### 第六步：删除废弃代码

```bash
# 删除自定义 RBAC 实现
rm internal/auth/rbac.go
rm internal/auth/init.go

# 删除旧文档
rm docs/auth-guide.md
rm docs/auth-integration-example.go

# 如果有测试文件
rm internal/auth/rbac_test.go
rm internal/auth/init_test.go
```

### 第七步：修改模型文件

编辑 `internal/storage/models/user.go`，删除 `Permissions` 结构体和相关字段。

---

## 数据迁移

### 迁移用户角色关系

创建迁移脚本 `scripts/migrate_to_casbin.go`：

```go
package main

import (
    "context"
    "log"
    
    "cloud-agent-monitor/internal/auth"
    "cloud-agent-monitor/internal/storage"
    "cloud-agent-monitor/pkg/config"
    
    "gorm.io/gorm"
)

func main() {
    // 初始化数据库和 Casbin
    cfg, _ := config.Load()
    db, _ := storage.NewMySQLDB(cfg.Database)
    enforcer, _ := auth.NewCasbinEnforcer(db, cfg)
    
    // 迁移用户角色关系
    if err := migrateUserRoles(db
, enforcer); err != nil {
        log.Fatalf("Migration failed: %v", err)
    }
    
    log.Println("Migration completed successfully!")
}

func migrateUserRoles(db *gorm.DB, enforcer *auth.CasbinEnforcer) error {
    // 查询现有的用户角色关系
    var userRoles []struct {
        UserID   string
        RoleName string
        TenantID string
    }
    
    err := db.Table("user_roles").
        Select("user_roles.user_id, roles.name as role_name, users.tenant_id").
        Joins("JOIN roles ON roles.id = user_roles.role_id").
        Joins("JOIN users ON users.id = user_roles.user_id").
        Scan(&userRoles).Error
        
    if err != nil {
        return err
    }
    
    log.Printf("Found %d user-role relationships to migrate", len(userRoles))
    
    // 迁移到 Casbin
    for _, ur := range userRoles {
        domain := ur.TenantID
        if domain == "" {
            domain = "default"
        }
        
        _, err := enforcer.AddRoleForUser(ur.UserID, ur.RoleName, domain)
        if err != nil {
            log.Printf("Warning: failed to migrate user %s role %s: %v", 
                ur.UserID, ur.RoleName, err)
        } else {
            log.Printf("Migrated: user=%s, role=%s, domain=%s", 
                ur.UserID, ur.RoleName, domain)
        }
    }
    
    return enforcer.SavePolicy()
}
```

运行迁移：

```bash
# 编译迁移脚本
go run scripts/migrate_to_casbin.go

# 验证迁移结果
# 检查 Casbin 表 casbin_rule 是否有数据
mysql> SELECT COUNT(*) FROM casbin_rule;
mysql> SELECT * FROM casbin_rule LIMIT 10;
```

---

## 验证测试

### 功能测试

```bash
# 1. 启动服务
go run cmd/platform-api/main.go

# 2. 测试认证（应该成功）
curl -H "X-API-Key: obs_your_api_key" \
  http://localhost:8080/api/v1/services

# 3. 测试权限检查（应该根据角色返回不同结果）
# 使用 admin API Key（应该成功）
curl -H "X-API-Key: obs_admin_key" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"test-service"}' \
  http://localhost:8080/api/v1/services

# 使用 reader API Key（应该返回 403 Forbidden）
curl -H "X-API-Key: obs_reader_key" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"test-service"}' \
  http://localhost:8080/api/v1/services
```

### 单元测试

创建 `internal/auth/casbin_test.go`：

```go
package auth

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestCasbinEnforcer_Enforce(t *testing.T) {
    // 使用内存适配器进行测试
    enforcer := setupTestEnforcer()
    
    tests := []struct {
        name     string
        sub      string
        dom      string
        obj      string
        act      string
        expected bool
    }{
        {
            name:     "admin can access all",
            sub:      "admin",
            dom:      "default",
            obj:      "/api/v1/services",
            act:      "GET",
            expected: true,
        },
        {
            name:     "reader cannot create service",
            sub:      "reader",
            dom:      "default",
            obj:      "/api/v1/services",
            act:      "POST",
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ok, err := enforcer.Enforce(tt.sub, tt.dom, tt.obj, tt.act)
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, ok)
        })
    }
}
```

### 集成测试

```bash
# 运行所有测试
go test ./internal/auth/... -v

# 运行集成测试
go test ./test/integration/... -v
```

---

## 回滚方案

如果迁移失败，可以快速回滚：

### 代码回滚

```bash
# 切回主分支
git checkout main

# 或者恢复备份分支
git checkout feature/before-casbin-migration
```

### 数据库回滚

```bash
# 恢复数据库备份
mysql -u root -p obs_platform < backup_before_casbin_$(date +%Y%m%d).sql

# 删除 Casbin 表
mysql -u root -p obs_platform -e "DROP TABLE IF EXISTS casbin_rule;"
```

### 依赖回滚

```bash
# 删除 Casbin 依赖
go mod edit -droprequire github.com/casbin/casbin/v2
go mod edit -droprequire github.com/casbin/gorm-adapter/v2
go mod tidy
```

---

## 迁移后的优势

### 功能提升

| 功能 | 迁移前 | 迁移后 |
|------|--------|--------|
| **权限模型** | 简单 RBAC | RBAC + ABAC + ACL |
| **资源级权限** | ❌ 不支持 | ✅ 支持 |
| **权限继承** | ❌ 不支持 | ✅ 支持 |
| **多租户** | ⚠️ 仅预留 | ✅ 原生支持 |
| **性能优化** | ❌ 无缓存 | ✅ 内置缓存 |
| **可视化管理** | ❌ 无 | ✅ Casbin Dashboard |

### 维护成本

| 方面 | 迁移前 | 迁移后 |
|------|--------|--------|
| **代码行数** | ~1000 行 | ~400 行 |
| **测试覆盖** | 需要自己编写 | 社区已覆盖 |
| **Bug 修复** | 自己修复 | 社区修复 |
| **功能扩展** | 自己开发 | 社区贡献 |
| **文档支持** | 需要自己编写 | 官方文档齐全 |

### 性能对比

```bash
# 压力测试对比（假设数据）
# 迁移前：自定义 RBAC
Requests/sec:  500
Latency:       200ms (P99)

# 迁移后：Casbin + 缓存
Requests/sec:  2000
Latency:       50ms (P99)

# 性能提升：4x
```

---

## 下一步计划

迁移完成后，可以进一步优化：

1. **启用 Redis 缓存**
   ```bash
   go get github.com/casbin/redis-watcher/v2
   ```

2. **集成 Casbin Dashboard**
   ```bash
   docker run -p 7001:7001 casbin/casbin-dashboard
   ```

3. **添加权限审计**
   - 记录所有权限变更
   - 定期权限审查
   - 异常权限告警

4. **实现更细粒度的权限**
   - 服务级权限控制
   - 字段级权限控制
   - 数据行级权限控制

---

## 参考资料

- [Casbin 官方文档](https://casbin.org/)
- [Casbin GitHub](https://github.com/casbin/casbin)
- [Casbin 模型语法](https://casbin.org/docs/supported-models)
- [Casbin 性能优化](https://casbin.org/docs/performance)
- [Casbin GORM Adapter](https://github.com/casbin/gorm-adapter)

---

## 问题排查

### 常见问题

**Q1: 迁移后权限检查失败？**
```bash
# 检查 Casbin 策略是否正确加载
mysql> SELECT * FROM casbin_rule;

# 检查用户角色关系
mysql> SELECT * FROM casbin_rule WHERE v1 = 'user_id';

# 重新加载策略
curl -X POST http://localhost:8080/api/v1/admin/reload-policy
```

**Q2: 性能下降？**
```bash
# 启用缓存
# 在 casbin.go 中添加：
enforcer.EnableEnforce(true)

# 使用 Redis Watcher
# 参考：https://github.com/casbin/redis-watcher
```

**Q3: 权限策略不一致？**
```bash
# 清空策略重新初始化
mysql> TRUNCATE TABLE casbin_rule;
# 重启服务，自动初始化默认策略
```

---

## 更新记录

| 日期 | 版本 | 变更 |
|------|------|------|
| 2025-01-XX | v1.0 | 初始版本，迁移指南 |

如有问题或建议，请提交 Issue 或联系开发团队。