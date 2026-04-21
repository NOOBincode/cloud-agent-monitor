package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type PermissionChecker interface {
	HasPermission(ctx context.Context, userID string, permission string) (bool, error)
}

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	PermissionsKey contextKey = "permissions"
)

type ReadOnlyTool interface {
	tool.InvokableTool
	IsReadOnly() bool
	RequiredPermission() string
}

type AuthzToolWrapper struct {
	inner    ReadOnlyTool
	authz    PermissionChecker
	toolName string
}

func NewAuthzToolWrapper(inner ReadOnlyTool, authz PermissionChecker) *AuthzToolWrapper {
	info, _ := inner.Info(context.Background())
	return &AuthzToolWrapper{
		inner:    inner,
		authz:    authz,
		toolName: info.Name,
	}
}

func (w *AuthzToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w *AuthzToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if !w.inner.IsReadOnly() {
		return "", fmt.Errorf("tool %s is not read-only and cannot be executed by agent", w.toolName)
	}

	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		return "", fmt.Errorf("authentication required: no user context in request")
	}

	permission := w.inner.RequiredPermission()
	hasPermission, err := w.authz.HasPermission(ctx, userID, permission)
	if err != nil {
		return "", fmt.Errorf("permission check failed: %w", err)
	}
	if !hasPermission {
		return "", fmt.Errorf("permission denied: user '%s' lacks '%s' permission", userID, permission)
	}

	return w.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func ContextWithPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, PermissionsKey, permissions)
}

func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

func GetPermissionsFromContext(ctx context.Context) []string {
	if perms, ok := ctx.Value(PermissionsKey).([]string); ok {
		return perms
	}
	return nil
}

type ToolRegistry struct {
	tools    map[string]ReadOnlyTool
	wrappers map[string]*AuthzToolWrapper
	authz    PermissionChecker
}

func NewToolRegistry(authz PermissionChecker) *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]ReadOnlyTool),
		wrappers: make(map[string]*AuthzToolWrapper),
		authz:    authz,
	}
}

func (r *ToolRegistry) Register(tool ReadOnlyTool) {
	info, _ := tool.Info(context.Background())
	name := info.Name

	r.tools[name] = tool
	r.wrappers[name] = NewAuthzToolWrapper(tool, r.authz)
}

func (r *ToolRegistry) GetTool(name string) (tool.InvokableTool, bool) {
	wrapper, ok := r.wrappers[name]
	if !ok {
		return nil, false
	}
	return wrapper, true
}

func (r *ToolRegistry) GetTools() []tool.InvokableTool {
	tools := make([]tool.InvokableTool, 0, len(r.wrappers))
	for _, wrapper := range r.wrappers {
		tools = append(tools, wrapper)
	}
	return tools
}

func (r *ToolRegistry) GetToolInfos(ctx context.Context) ([]*schema.ToolInfo, error) {
	infos := make([]*schema.ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (r *ToolRegistry) ListAvailableTools(ctx context.Context, userID string) ([]map[string]any, error) {
	result := make([]map[string]any, 0, len(r.tools))

	for name, t := range r.tools {
		info, _ := t.Info(ctx)
		permission := t.RequiredPermission()

		hasPermission := false
		if r.authz != nil {
			hasPermission, _ = r.authz.HasPermission(ctx, userID, permission)
		}

		result = append(result, map[string]any{
			"name":           name,
			"description":    info.Desc,
			"permission":     permission,
			"has_permission": hasPermission,
			"is_readonly":    t.IsReadOnly(),
		})
	}

	return result, nil
}

func (r *ToolRegistry) Execute(ctx context.Context, toolName string, argumentsInJSON string) (string, error) {
	wrapper, ok := r.wrappers[toolName]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
	return wrapper.InvokableRun(ctx, argumentsInJSON)
}

func (r *ToolRegistry) MarshalToolInfos() ([]byte, error) {
	infos, err := r.GetToolInfos(context.Background())
	if err != nil {
		return nil, err
	}
	return json.Marshal(infos)
}
