package eino

import (
	"testing"


	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupToolRegistry(t *testing.T) {
	authz := newMockPermissionChecker().grantAll()
	registry := SetupToolRegistry(authz)
	require.NotNil(t, registry)
}

func TestSetupPoolRegistry(t *testing.T) {
	authz := newMockPermissionChecker().grantAll()
	toolRegistry := SetupToolRegistry(authz)

	alertProvider := NewAlertingToolProvider(&mockAlertingService{})
	sloProvider := NewSLOToolProvider(&mockSLOService{})
	topoProvider := NewTopologyToolProvider(&mockTopologyService{})

	poolRegistry := SetupPoolRegistry(toolRegistry, alertProvider, sloProvider, topoProvider)
	require.NotNil(t, poolRegistry)

	pools := poolRegistry.ListPools(nil)
	assert.Len(t, pools, 4)

	var ids []string
	for _, p := range pools {
		ids = append(ids, p.ID)
	}
	assert.Contains(t, ids, "alerting")
	assert.Contains(t, ids, "slo")
	assert.Contains(t, ids, "topology")
	assert.Contains(t, ids, "general")
}

func TestDefaultToolBudget(t *testing.T) {
	budget := DefaultToolBudget()
	require.NotNil(t, budget)
	assert.Equal(t, 10, budget.MaxToolsPerRequest)
	assert.Equal(t, 8000, budget.MaxTokensForTools)
}

func TestSetupPoolRegistry_PriorityOrder(t *testing.T) {
	authz := newMockPermissionChecker().grantAll()
	toolRegistry := SetupToolRegistry(authz)

	alertProvider := NewAlertingToolProvider(&mockAlertingService{})
	sloProvider := NewSLOToolProvider(&mockSLOService{})
	topoProvider := NewTopologyToolProvider(&mockTopologyService{})

	poolRegistry := SetupPoolRegistry(toolRegistry, alertProvider, sloProvider, topoProvider)

	pools := poolRegistry.ListPools(nil)
	require.Len(t, pools, 4)

	for i := 1; i < len(pools); i++ {
		assert.GreaterOrEqual(t, pools[i-1].Priority, pools[i].Priority,
			"pools should be sorted by priority descending")
	}
}
