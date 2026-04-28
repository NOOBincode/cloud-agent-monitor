package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// PermissionChecker abstracts authorization checks for tool access.
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID string, permission string) (bool, error)
}

// contextKey is used for storing user identity in context.
type contextKey string

const (
	// UserIDKey stores the authenticated user ID in context.
	UserIDKey contextKey = "user_id"
	// PermissionsKey stores the user's permission list in context.
	PermissionsKey contextKey = "permissions"
)

// ReadOnlyTool extends eino's InvokableTool with access control metadata.
// Only read-only tools are allowed in the agent's tool pool.
type ReadOnlyTool interface {
	tool.InvokableTool
	IsReadOnly() bool
	RequiredPermission() string
}

// AuthzToolWrapper enforces authentication and authorization on every tool call.
// It checks user context, read-only status, and permission before delegating to the inner tool.
type AuthzToolWrapper struct {
	inner    ReadOnlyTool
	authz    PermissionChecker
	toolName string
}

// NewAuthzToolWrapper wraps a ReadOnlyTool with authorization checks.
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

// ContextWithUserID returns a context with the given user ID stored.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// ContextWithPermissions returns a context with the given permission list stored.
func ContextWithPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, PermissionsKey, permissions)
}

// GetUserIDFromContext extracts the user ID from context. Returns empty string if absent.
func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetPermissionsFromContext extracts the permission list from context. Returns nil if absent.
func GetPermissionsFromContext(ctx context.Context) []string {
	if perms, ok := ctx.Value(PermissionsKey).([]string); ok {
		return perms
	}
	return nil
}

// ToolRegistry manages tool instances and their authorization wrappers.
// All tools are registered as ReadOnlyTool and wrapped with AuthzToolWrapper for access control.
type ToolRegistry struct {
	tools    map[string]ReadOnlyTool
	wrappers map[string]*AuthzToolWrapper
	authz    PermissionChecker
}

// NewToolRegistry creates a new empty registry with the given permission checker.
func NewToolRegistry(authz PermissionChecker) *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]ReadOnlyTool),
		wrappers: make(map[string]*AuthzToolWrapper),
		authz:    authz,
	}
}

// Register adds a ReadOnlyTool to the registry, wrapping it with AuthzToolWrapper.
func (r *ToolRegistry) Register(tool ReadOnlyTool) {
	info, _ := tool.Info(context.Background())
	name := info.Name

	r.tools[name] = tool
	r.wrappers[name] = NewAuthzToolWrapper(tool, r.authz)
}

// GetTool returns a tool wrapper by name.
func (r *ToolRegistry) GetTool(name string) (tool.InvokableTool, bool) {
	wrapper, ok := r.wrappers[name]
	if !ok {
		return nil, false
	}
	return wrapper, true
}

// GetTools returns all registered tool wrappers as InvokableTool slice.
func (r *ToolRegistry) GetTools() []tool.InvokableTool {
	tools := make([]tool.InvokableTool, 0, len(r.wrappers))
	for _, wrapper := range r.wrappers {
		tools = append(tools, wrapper)
	}
	return tools
}

// GetToolInfos returns ToolInfo descriptors for all registered tools.
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

// ListAvailableTools returns tool metadata with permission status for each tool.
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

// Execute runs a tool by name with the given JSON arguments, enforcing authorization.
func (r *ToolRegistry) Execute(ctx context.Context, toolName string, argumentsInJSON string) (string, error) {
	wrapper, ok := r.wrappers[toolName]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}
	return wrapper.InvokableRun(ctx, argumentsInJSON)
}

// MarshalToolInfos serializes all tool descriptors to JSON for LLM consumption.
func (r *ToolRegistry) MarshalToolInfos() ([]byte, error) {
	infos, err := r.GetToolInfos(context.Background())
	if err != nil {
		return nil, err
	}
	return json.Marshal(infos)
}
