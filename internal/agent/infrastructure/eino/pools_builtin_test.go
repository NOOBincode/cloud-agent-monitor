package eino

import (
	"testing"

	"cloud-agent-monitor/internal/agent/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinPools(t *testing.T) {
	pools := BuiltinPools()
	require.Len(t, pools, 4)

	ids := make(map[string]bool)
	for _, p := range pools {
		ids[p.ID] = true
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.Keywords)
		assert.NotEmpty(t, p.ToolNames)
		assert.True(t, p.Priority > 0)
		assert.True(t, p.IsBuiltin)
	}

	assert.True(t, ids["alerting"])
	assert.True(t, ids["slo"])
	assert.True(t, ids["topology"])
	assert.True(t, ids["general"])
}

func TestBuiltinPools_Alerting(t *testing.T) {
	pools := BuiltinPools()
	var alerting *domain.ToolPool
	for _, p := range pools {
		if p.ID == "alerting" {
			alerting = p
			break
		}
	}
	require.NotNil(t, alerting)
	assert.Equal(t, 9, alerting.Priority)
	assert.Equal(t, 6, alerting.MaxTools)
	assert.Len(t, alerting.ToolNames, 6)
	assert.Contains(t, alerting.Categories, domain.CategoryAlerting)
}

func TestBuiltinPools_Topology(t *testing.T) {
	pools := BuiltinPools()
	var topo *domain.ToolPool
	for _, p := range pools {
		if p.ID == "topology" {
			topo = p
			break
		}
	}
	require.NotNil(t, topo)
	assert.Equal(t, 8, topo.Priority)
	assert.Len(t, topo.ToolNames, 10)
}

func TestBuiltinPools_SLO(t *testing.T) {
	pools := BuiltinPools()
	var slo *domain.ToolPool
	for _, p := range pools {
		if p.ID == "slo" {
			slo = p
			break
		}
	}
	require.NotNil(t, slo)
	assert.Equal(t, 7, slo.Priority)
	assert.Len(t, slo.ToolNames, 5)
}

func TestBuiltinPools_GeneralLowestPriority(t *testing.T) {
	pools := BuiltinPools()
	for _, p := range pools {
		if p.ID == "general" {
			assert.Equal(t, 1, p.Priority)
			assert.True(t, len(p.Categories) >= 2, "general pool should span multiple categories")
			return
		}
	}
	t.Fatal("general pool not found")
}

func TestReservedAIPools(t *testing.T) {
	pools := ReservedAIPools()
	require.Len(t, pools, 3)

	ids := make(map[string]bool)
	for _, p := range pools {
		ids[p.ID] = true
		assert.NotEmpty(t, p.Name)
		assert.True(t, p.IsBuiltin)
		assert.Empty(t, p.ToolNames, "reserved pools should have no tools yet")
	}

	assert.True(t, ids["gpu"])
	assert.True(t, ids["inference"])
	assert.True(t, ids["cost"])
}
