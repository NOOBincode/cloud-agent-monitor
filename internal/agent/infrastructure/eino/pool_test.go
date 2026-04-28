package eino

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"cloud-agent-monitor/internal/agent/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		exact    []string
	}{
		{"simple space", "hello world", []string{"hello", "world"}, nil},
		{"chinese comma", "告警,报警", []string{"告警", "报警"}, nil},
		{"mixed separators", "告警 报警;firing", []string{"告警", "报警", "firing"}, nil},
		{"empty string", "", nil, []string{}},
		{"single word", "alert", []string{"alert"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			if tt.exact != nil {
				if len(tt.exact) == 0 {
					assert.Empty(t, result)
				} else {
					assert.Equal(t, tt.exact, result)
				}
			}
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

func TestMergeAction(t *testing.T) {
	tests := []struct {
		name     string
		argsJSON string
		action   string
		want     map[string]interface{}
	}{
		{
			"empty args",
			"", "list_active",
			map[string]interface{}{"action": "list_active"},
		},
		{
			"empty json object",
			"{}", "list_active",
			map[string]interface{}{"action": "list_active"},
		},
		{
			"with existing params",
			`{"severity":"critical","limit":10}`, "list_history",
			map[string]interface{}{"action": "list_history", "severity": "critical", "limit": float64(10)},
		},
		{
			"invalid json",
			"not-json", "list_active",
			map[string]interface{}{"action": "list_active"},
		},
		{
			"override existing action",
			`{"action":"old","severity":"warning"}`, "new_action",
			map[string]interface{}{"action": "new_action", "severity": "warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeAction(tt.argsJSON, tt.action)
			var got map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(result), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFatToolDelegator_Delegate(t *testing.T) {
	t.Run("merges action into empty args", func(t *testing.T) {
		stub := newStubTool("test_tool", "desc")
		stub.result = `{"result":"ok"}`
		delegate := NewFatToolDelegator(stub, "list_active")

		result, err := delegate.Delegate(context.Background(), "")
		require.NoError(t, err)
		assert.Equal(t, `{"result":"ok"}`, result)

		var args map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(stub.lastArgs), &args))
		assert.Equal(t, "list_active", args["action"])
	})

	t.Run("merges action into existing args", func(t *testing.T) {
		stub := newStubTool("test_tool", "desc")
		delegate := NewFatToolDelegator(stub, "get_stats")

		_, err := delegate.Delegate(context.Background(), `{"severity":"critical"}`)
		require.NoError(t, err)

		var args map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(stub.lastArgs), &args))
		assert.Equal(t, "get_stats", args["action"])
		assert.Equal(t, "critical", args["severity"])
	})

	t.Run("propagates inner error", func(t *testing.T) {
		stub := newStubTool("test_tool", "desc")
		stub.err = errors.New("service unavailable")
		delegate := NewFatToolDelegator(stub, "list")

		_, err := delegate.Delegate(context.Background(), "")
		assert.ErrorIs(t, err, stub.err)
	})
}

func TestIntentRouter_Route(t *testing.T) {
	t.Run("matches keyword to pool", func(t *testing.T) {
		router := NewIntentRouter()
		alertPool := &domain.ToolPool{ID: "alerting", Keywords: []string{"告警", "alert"}, Priority: 9}
		router.AddPool(alertPool)

		pool := router.Route("查看告警")
		require.NotNil(t, pool)
		assert.Equal(t, "alerting", pool.ID)
	})

	t.Run("matches by pool keyword substring", func(t *testing.T) {
		router := NewIntentRouter()
		sloPool := &domain.ToolPool{ID: "slo", Keywords: []string{"SLO", "error budget"}, Priority: 7}
		router.AddPool(sloPool)

		pool := router.Route("error budget status")
		require.NotNil(t, pool)
		assert.Equal(t, "slo", pool.ID)
	})

	t.Run("selects higher priority pool on conflict", func(t *testing.T) {
		router := NewIntentRouter()
		alertPool := &domain.ToolPool{ID: "alerting", Keywords: []string{"告警", "alert"}, Priority: 9}
		generalPool := &domain.ToolPool{ID: "general", Keywords: []string{"alert", "status"}, Priority: 1}
		router.AddPool(alertPool)
		router.AddPool(generalPool)

		pool := router.Route("alert status")
		require.NotNil(t, pool)
		assert.Equal(t, "alerting", pool.ID)
	})

	t.Run("falls back to general pool", func(t *testing.T) {
		router := NewIntentRouter()
		alertPool := &domain.ToolPool{ID: "alerting", Keywords: []string{"告警"}, Priority: 9}
		generalPool := &domain.ToolPool{ID: "general", Keywords: []string{"overview"}, Priority: 1}
		router.AddPool(alertPool)
		router.AddPool(generalPool)

		pool := router.Route("something completely unrelated")
		require.NotNil(t, pool)
		assert.Equal(t, "general", pool.ID)
	})

	t.Run("falls back to first pool when no general", func(t *testing.T) {
		router := NewIntentRouter()
		alertPool := &domain.ToolPool{ID: "alerting", Keywords: []string{"告警"}, Priority: 9}
		router.AddPool(alertPool)

		pool := router.Route("random query")
		require.NotNil(t, pool)
		assert.Equal(t, "alerting", pool.ID)
	})

	t.Run("returns nil with no pools", func(t *testing.T) {
		router := NewIntentRouter()
		pool := router.Route("anything")
		assert.Nil(t, pool)
	})

	t.Run("RemovePool cleans up", func(t *testing.T) {
		router := NewIntentRouter()
		alertPool := &domain.ToolPool{ID: "alerting", Keywords: []string{"告警"}, Priority: 9}
		router.AddPool(alertPool)
		router.RemovePool("alerting")

		pool := router.Route("告警")
		assert.Nil(t, pool)
	})

	t.Run("RemovePool non-existent is no-op", func(t *testing.T) {
		router := NewIntentRouter()
		assert.NotPanics(t, func() {
			router.RemovePool("nonexistent")
		})
	})
}

func TestTracedToolDecorator(t *testing.T) {
	t.Run("delegates Info call", func(t *testing.T) {
		stub := newStubTool("my_tool", "my description")
		decorator := NewTracedToolDecorator(stub, domain.CategoryAlerting)

		info, err := decorator.Info(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "my_tool", info.Name)
		assert.Equal(t, "my description", info.Desc)
	})

	t.Run("delegates InvokableRun and captures span", func(t *testing.T) {
		stub := newStubTool("my_tool", "desc")
		stub.result = `{"data":"hello"}`
		decorator := NewTracedToolDecorator(stub, domain.CategorySLO)

		result, err := decorator.InvokableRun(context.Background(), `{"input":"test"}`)
		require.NoError(t, err)
		assert.Equal(t, `{"data":"hello"}`, result)
	})

	t.Run("records error in span", func(t *testing.T) {
		stub := newStubTool("my_tool", "desc")
		stub.err = errors.New("timeout")
		decorator := NewTracedToolDecorator(stub, domain.CategoryTopology)

		_, err := decorator.InvokableRun(context.Background(), "")
		assert.Error(t, err)
		assert.Equal(t, "timeout", err.Error())
	})

	t.Run("preserves IsReadOnly and RequiredPermission", func(t *testing.T) {
		stub := newStubTool("my_tool", "desc")
		decorator := NewTracedToolDecorator(stub, domain.CategoryAlerting)

		assert.True(t, decorator.IsReadOnly())
		assert.Equal(t, "my_tool:read", decorator.RequiredPermission())
	})
}

func newTestPoolRegistry(authz *mockPermissionChecker) *PoolRegistry {
	registry := NewToolRegistry(authz)
	budget := &domain.ToolBudget{MaxToolsPerRequest: 10, MaxTokensForTools: 8000}
	return NewPoolRegistry(registry, budget)
}

func TestPoolRegistry_RegisterProvider(t *testing.T) {
	t.Run("registers provider with default pools", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)

		err := pr.RegisterProvider(provider)
		require.NoError(t, err)

		pools := pr.ListPools(context.Background())
		assert.NotEmpty(t, pools)

		providers := pr.GetProviders()
		_, ok := providers[domain.CategoryAlerting]
		assert.True(t, ok)
	})

	t.Run("registers provider with tools via RegisterProviderWithTools", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)
		tools := provider.CreateTools()

		err := pr.RegisterProviderWithTools(provider, tools)
		require.NoError(t, err)

		pools := pr.ListPools(context.Background())
		assert.NotEmpty(t, pools)
	})
}

func TestPoolRegistry_RegisterPool(t *testing.T) {
	t.Run("registers custom pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		pool := &domain.ToolPool{
			ID: "custom", Name: "Custom Pool", Keywords: []string{"custom"},
			Priority: 5, MaxTools: 5, IsBuiltin: false,
		}
		err := pr.RegisterPool(pool)
		require.NoError(t, err)

		got, ok := pr.GetPool("custom")
		assert.True(t, ok)
		assert.Equal(t, "Custom Pool", got.Name)
	})

	t.Run("rejects duplicate pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		pool := &domain.ToolPool{ID: "dup", Name: "Dup", Keywords: []string{}, Priority: 5}
		require.NoError(t, pr.RegisterPool(pool))
		err := pr.RegisterPool(pool)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestPoolRegistry_UnregisterPool(t *testing.T) {
	t.Run("unregisters non-builtin pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		pool := &domain.ToolPool{ID: "custom", Name: "Custom", Keywords: []string{}, Priority: 5, IsBuiltin: false}
		require.NoError(t, pr.RegisterPool(pool))

		err := pr.UnregisterPool("custom")
		require.NoError(t, err)

		_, ok := pr.GetPool("custom")
		assert.False(t, ok)
	})

	t.Run("rejects unregistering builtin pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		pool := &domain.ToolPool{ID: "builtin", Name: "Builtin", Keywords: []string{}, Priority: 5, IsBuiltin: true}
		require.NoError(t, pr.RegisterPool(pool))

		err := pr.UnregisterPool("builtin")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unregister builtin")
	})

	t.Run("returns error for non-existent pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		err := pr.UnregisterPool("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestPoolRegistry_SelectTools(t *testing.T) {
	t.Run("selects tools based on intent with permission", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)
		tools := provider.CreateTools()

		err := pr.RegisterProviderWithTools(provider, tools)
		require.NoError(t, err)

		ctx := ctxWithUser("user-1")
		selected, err := pr.SelectTools(ctx, "查看告警")
		require.NoError(t, err)
		assert.NotEmpty(t, selected)
	})

	t.Run("rejects without user context", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)
		tools := provider.CreateTools()
		require.NoError(t, pr.RegisterProviderWithTools(provider, tools))

		_, err := pr.SelectTools(context.Background(), "告警")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authentication required")
	})

	t.Run("rejects when no pool matches", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		ctx := ctxWithUser("user-1")
		_, err := pr.SelectTools(ctx, "random query")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no tool pool matched")
	})

	t.Run("filters tools by permission", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)
		tools := provider.CreateTools()

		err := pr.RegisterProviderWithTools(provider, tools)
		require.NoError(t, err)

		ctx := ctxWithUser("user-no-perms")
		selected, err := pr.SelectTools(ctx, "查看告警")
		require.NoError(t, err)
		assert.Empty(t, selected)
	})
}

func TestPoolRegistry_GetToolsForPool(t *testing.T) {
	t.Run("returns tools for valid pool", func(t *testing.T) {
		authz := newMockPermissionChecker().grantAll()
		pr := newTestPoolRegistry(authz)
		mockSvc := &mockAlertingService{}
		provider := NewAlertingToolProvider(mockSvc)
		tools := provider.CreateTools()

		err := pr.RegisterProviderWithTools(provider, tools)
		require.NoError(t, err)

		result, err := pr.GetToolsForPool(context.Background(), "alert")
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("returns error for non-existent pool", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		_, err := pr.GetToolsForPool(context.Background(), "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestPoolRegistry_ListPools(t *testing.T) {
	t.Run("returns pools sorted by priority", func(t *testing.T) {
		authz := newMockPermissionChecker()
		pr := newTestPoolRegistry(authz)

		p1 := &domain.ToolPool{ID: "low", Name: "Low", Priority: 1, Keywords: []string{}}
		p2 := &domain.ToolPool{ID: "high", Name: "High", Priority: 9, Keywords: []string{}}
		require.NoError(t, pr.RegisterPool(p1))
		require.NoError(t, pr.RegisterPool(p2))

		pools := pr.ListPools(context.Background())
		require.Len(t, pools, 2)
		assert.Equal(t, "high", pools[0].ID)
		assert.Equal(t, "low", pools[1].ID)
	})
}
