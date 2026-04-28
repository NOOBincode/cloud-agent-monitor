package eino

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHelpers(t *testing.T) {
	t.Run("ContextWithUserID and GetUserIDFromContext", func(t *testing.T) {
		ctx := ContextWithUserID(context.Background(), "user-123")
		assert.Equal(t, "user-123", GetUserIDFromContext(ctx))
	})

	t.Run("GetUserIDFromContext returns empty when missing", func(t *testing.T) {
		assert.Equal(t, "", GetUserIDFromContext(context.Background()))
	})

	t.Run("ContextWithPermissions and GetPermissionsFromContext", func(t *testing.T) {
		perms := []string{"read", "write"}
		ctx := ContextWithPermissions(context.Background(), perms)
		assert.Equal(t, perms, GetPermissionsFromContext(ctx))
	})

	t.Run("GetPermissionsFromContext returns nil when missing", func(t *testing.T) {
		assert.Nil(t, GetPermissionsFromContext(context.Background()))
	})
}

func TestToolRegistry_Register(t *testing.T) {
	t.Run("registers tool and creates wrapper", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		registry := NewToolRegistry(authz)
		stub := newStubTool("my_tool", "my desc")

		registry.Register(stub)

		got, ok := registry.GetTool("my_tool")
		assert.True(t, ok)
		assert.NotNil(t, got)
	})

	t.Run("GetTools returns all registered tools", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		registry := NewToolRegistry(authz)
		registry.Register(newStubTool("tool_a", "desc a"))
		registry.Register(newStubTool("tool_b", "desc b"))

		tools := registry.GetTools()
		assert.Len(t, tools, 2)
	})

	t.Run("GetTool returns nil for unknown", func(t *testing.T) {
		authz := newMockPermissionChecker()
		registry := NewToolRegistry(authz)

		got, ok := registry.GetTool("unknown")
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestToolRegistry_GetToolInfos(t *testing.T) {
	t.Run("returns info for all tools", func(t *testing.T) {
		authz := newMockPermissionChecker()
		registry := NewToolRegistry(authz)
		registry.Register(newStubTool("tool_a", "desc a"))
		registry.Register(newStubTool("tool_b", "desc b"))

		infos, err := registry.GetToolInfos(context.Background())
		require.NoError(t, err)
		assert.Len(t, infos, 2)

		names := make(map[string]bool)
		for _, info := range infos {
			names[info.Name] = true
		}
		assert.True(t, names["tool_a"])
		assert.True(t, names["tool_b"])
	})
}

func TestToolRegistry_ListAvailableTools(t *testing.T) {
	t.Run("lists tools with permission status", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		registry := NewToolRegistry(authz)
		registry.Register(newStubTool("tool_a", "desc a"))

		result, err := registry.ListAvailableTools(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "tool_a", result[0]["name"])
		assert.Equal(t, true, result[0]["has_permission"])
		assert.Equal(t, true, result[0]["is_readonly"])
	})

	t.Run("shows no permission when not granted", func(t *testing.T) {
		authz := newMockPermissionChecker()
		registry := NewToolRegistry(authz)
		registry.Register(newStubTool("tool_a", "desc a"))

		result, err := registry.ListAvailableTools(context.Background(), "user-no-perms")
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, false, result[0]["has_permission"])
	})
}

func TestToolRegistry_Execute(t *testing.T) {
	t.Run("executes tool via authz wrapper", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		registry := NewToolRegistry(authz)
		stub := newStubTool("exec_tool", "desc")
		stub.result = `{"executed":true}`
		registry.Register(stub)

		ctx := ctxWithUser("user-1")
		result, err := registry.Execute(ctx, "exec_tool", `{"input":"test"}`)
		require.NoError(t, err)
		assert.Equal(t, `{"executed":true}`, result)
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		registry := NewToolRegistry(authz)

		_, err := registry.Execute(context.Background(), "unknown", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool not found")
	})
}

func TestToolRegistry_MarshalToolInfos(t *testing.T) {
	t.Run("marshals tool infos to JSON", func(t *testing.T) {
		authz := newMockPermissionChecker()
		registry := NewToolRegistry(authz)
		registry.Register(newStubTool("tool_a", "desc a"))

		data, err := registry.MarshalToolInfos()
		require.NoError(t, err)
		assert.Contains(t, string(data), "tool_a")
	})
}

func TestAuthzToolWrapper(t *testing.T) {
	t.Run("allows read-only tool with permission", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		stub := newStubTool("allowed_tool", "desc")
		wrapper := NewAuthzToolWrapper(stub, authz)

		ctx := ctxWithUser("user-1")
		result, err := wrapper.InvokableRun(ctx, `{"test":true}`)
		require.NoError(t, err)
		assert.Equal(t, `{"status":"ok"}`, result)
	})

	t.Run("rejects without user context", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		stub := newStubTool("no_ctx_tool", "desc")
		wrapper := NewAuthzToolWrapper(stub, authz)

		_, err := wrapper.InvokableRun(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication required")
	})

	t.Run("rejects when permission denied", func(t *testing.T) {
		authz := newMockPermissionChecker()
		stub := newStubTool("denied_tool", "desc")
		wrapper := NewAuthzToolWrapper(stub, authz)

		ctx := ctxWithUser("user-1")
		_, err := wrapper.InvokableRun(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("rejects non-read-only tool", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		stub := newStubTool("write_tool", "desc")
		stub.isReadOnly = false
		wrapper := NewAuthzToolWrapper(stub, authz)

		ctx := ctxWithUser("user-1")
		_, err := wrapper.InvokableRun(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not read-only")
	})

	t.Run("handles permission check error", func(t *testing.T) {
		authz := newMockPermissionChecker()
		authz.err = errors.New("db error")
		stub := newStubTool("error_tool", "desc")
		wrapper := NewAuthzToolWrapper(stub, authz)

		ctx := ctxWithUser("user-1")
		_, err := wrapper.InvokableRun(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "permission check failed")
	})

	t.Run("delegates Info call", func(t *testing.T) {
		authz := newMockPermissionChecker()
		stub := newStubTool("info_tool", "info desc")
		wrapper := NewAuthzToolWrapper(stub, authz)

		info, err := wrapper.Info(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "info_tool", info.Name)
		assert.Equal(t, "info desc", info.Desc)
	})
}
