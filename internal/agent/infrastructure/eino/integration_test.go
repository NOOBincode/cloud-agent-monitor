package eino

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentDomain "cloud-agent-monitor/internal/agent/domain"
	alertApp "cloud-agent-monitor/internal/alerting/application"
	alertDomain "cloud-agent-monitor/internal/alerting/domain"
	sloDomain "cloud-agent-monitor/internal/slo/domain"
	"cloud-agent-monitor/internal/storage/models"
	topoDomain "cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFullTestEnv(t *testing.T) (*PoolRegistry, *mockAlertingService, *mockSLOService, *mockTopologyService) {
	t.Helper()

	authz := newMockPermissionChecker().grantAll()
	toolRegistry := SetupToolRegistry(authz)
	budget := DefaultToolBudget()
	poolRegistry := NewPoolRegistry(toolRegistry, budget)

	for _, pool := range BuiltinPools() {
		require.NoError(t, poolRegistry.RegisterPool(pool))
	}

	alertSvc := &mockAlertingService{
		alerts:    []*alertDomain.Alert{{ID: uuid.New(), Labels: map[string]string{"alertname": "TestAlert"}}},
		records:   []*models.AlertRecord{{ID: uuid.New(), Fingerprint: "fp1"}},
		recordCnt: 1,
		stats:     &models.AlertRecordStats{TotalCount: 100},
		noisy:     []*models.AlertNoiseRecord{{ID: uuid.New(), AlertFingerprint: "fp-noise"}},
		highRisk:  []*models.AlertNoiseRecord{{ID: uuid.New(), AlertFingerprint: "fp-risk"}},
		feedback:  &alertApp.AlertFeedback{Fingerprint: "fp1", AlertName: "TestAlert"},
	}
	alertProvider := NewAlertingToolProvider(alertSvc)
	alertTools := alertProvider.CreateTools()
	require.NoError(t, poolRegistry.RegisterProviderWithTools(alertProvider, alertTools))

	sloID := uuid.New()
	sloSvc := &mockSLOService{
		slos:        []*sloDomain.SLO{{ID: sloID, Name: "availability-slo"}},
		slo:         &sloDomain.SLO{ID: sloID, Name: "availability-slo"},
		summary:     &sloDomain.SLOSummary{Total: 5},
		errorBudget: &sloDomain.ErrorBudget{Remaining: 99.5},
		burnRate:    []*sloDomain.BurnRateAlert{{SLOID: sloID}},
	}
	sloProvider := NewSLOToolProvider(sloSvc)
	sloTools := sloProvider.CreateTools()
	require.NoError(t, poolRegistry.RegisterProviderWithTools(sloProvider, sloTools))

	nodeID := uuid.New()
	topoSvc := &mockTopologyService{
		serviceTopology: &topoDomain.ServiceTopology{Nodes: []*topoDomain.ServiceNode{{ID: nodeID, Name: "svc-a"}}},
		networkTopology: &topoDomain.NetworkTopology{},
		node:            &topoDomain.ServiceNode{ID: nodeID, Name: "svc-a"},
		upstream:        []*topoDomain.ServiceNode{{ID: uuid.New(), Name: "upstream"}},
		downstream:      []*topoDomain.ServiceNode{{ID: uuid.New(), Name: "downstream"}},
		impact:          &topoDomain.ImpactResult{RootServiceID: nodeID},
		pathResult:      &topoDomain.PathResult{SourceID: nodeID, TargetID: uuid.New()},
		shortestPath:    []topoDomain.PathHop{},
		anomalies:       []*topoDomain.TopologyAnomaly{{NodeName: "svc-a"}},
		stats:           &topoDomain.TopologyStats{ServiceNodeCount: 10},
	}
	topoProvider := NewTopologyToolProvider(topoSvc)
	topoTools := topoProvider.CreateTools()
	require.NoError(t, poolRegistry.RegisterProviderWithTools(topoProvider, topoTools))

	return poolRegistry, alertSvc, sloSvc, topoSvc
}

func TestIntegration_FullChain_AlertIntent(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "查看活跃告警")
	require.NoError(t, err)
	assert.NotEmpty(t, tools)

	var names []string
	for _, tl := range tools {
		info, _ := tl.Info(ctx)
		if info != nil {
			names = append(names, info.Name)
		}
	}
	assert.Contains(t, names, "alerting_list_active")
}

func TestIntegration_FullChain_SLOIntent(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "查看SLO错误预算")
	require.NoError(t, err)
	assert.NotEmpty(t, tools)

	var names []string
	for _, tl := range tools {
		info, _ := tl.Info(ctx)
		if info != nil {
			names = append(names, info.Name)
		}
	}
	hasSLO := false
	for _, n := range names {
		if len(n) >= 3 && n[:3] == "slo" {
			hasSLO = true
			break
		}
	}
	assert.True(t, hasSLO)
}

func TestIntegration_FullChain_TopologyIntent(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "查看服务拓扑依赖")
	require.NoError(t, err)
	assert.NotEmpty(t, tools)

	var names []string
	for _, tl := range tools {
		info, _ := tl.Info(ctx)
		if info != nil {
			names = append(names, info.Name)
		}
	}
	hasTopo := false
	for _, n := range names {
		if len(n) >= 8 && n[:8] == "topology" {
			hasTopo = true
			break
		}
	}
	assert.True(t, hasTopo)
}

func TestIntegration_FullChain_GeneralIntent(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "系统概览总览")
	require.NoError(t, err)
	assert.NotEmpty(t, tools)
}

func TestIntegration_FullChain_MultipleProviders(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)

	providers := poolRegistry.GetProviders()
	_, hasAlerting := providers[agentDomain.CategoryAlerting]
	_, hasSLO := providers[agentDomain.CategorySLO]
	_, hasTopology := providers[agentDomain.CategoryTopology]

	assert.True(t, hasAlerting)
	assert.True(t, hasSLO)
	assert.True(t, hasTopology)
}

func TestIntegration_FullChain_PoolGetTools(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)

	alertTools, err := poolRegistry.GetToolsForPool(context.Background(), "alert")
	require.NoError(t, err)
	assert.Len(t, alertTools, 6)

	sloTools, err := poolRegistry.GetToolsForPool(context.Background(), "slo")
	require.NoError(t, err)
	assert.Len(t, sloTools, 5)

	topoTools, err := poolRegistry.GetToolsForPool(context.Background(), "topology")
	require.NoError(t, err)
	assert.Len(t, topoTools, 10)
}

func TestIntegration_FullChain_DynamicPoolManagement(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)

	customPool := &agentDomain.ToolPool{
		ID: "custom-test", Name: "Custom Test Pool",
		Keywords: []string{"custom", "test"}, Priority: 3, MaxTools: 5, IsBuiltin: false,
		ToolNames: []string{"alerting_list_active"},
	}
	require.NoError(t, poolRegistry.RegisterPool(customPool))

	pool, ok := poolRegistry.GetPool("custom-test")
	assert.True(t, ok)
	assert.Equal(t, "Custom Test Pool", pool.Name)

	require.NoError(t, poolRegistry.UnregisterPool("custom-test"))
	_, ok = poolRegistry.GetPool("custom-test")
	assert.False(t, ok)
}

func TestIntegration_FullChain_PermissionFiltering(t *testing.T) {
	authz := newMockPermissionChecker()
	toolRegistry := SetupToolRegistry(authz)
	poolRegistry := NewPoolRegistry(toolRegistry, DefaultToolBudget())

	for _, pool := range BuiltinPools() {
		require.NoError(t, poolRegistry.RegisterPool(pool))
	}

	alertSvc := &mockAlertingService{
		alerts: []*alertDomain.Alert{{ID: uuid.New()}},
	}
	alertProvider := NewAlertingToolProvider(alertSvc)
	alertTools := alertProvider.CreateTools()
	require.NoError(t, poolRegistry.RegisterProviderWithTools(alertProvider, alertTools))

	authz.grant("user-admin", "alerting:read")

	ctx1 := ctxWithUser("user-admin")
	tools1, err := poolRegistry.SelectTools(ctx1, "告警")
	require.NoError(t, err)
	assert.NotEmpty(t, tools1)

	ctx2 := ctxWithUser("user-no-perms")
	tools2, err := poolRegistry.SelectTools(ctx2, "告警")
	require.NoError(t, err)
	assert.Empty(t, tools2)
}

func TestIntegration_FullChain_MaxToolsLimit(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "告警")
	require.NoError(t, err)

	alertingPool, ok := poolRegistry.GetPool("alerting")
	require.True(t, ok)
	assert.LessOrEqual(t, len(tools), alertingPool.MaxTools)
}

func TestIntegration_FullChain_EndToEnd_ToolExecution(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)
	ctx := ctxWithUser("user-1")

	tools, err := poolRegistry.SelectTools(ctx, "SLO列表")
	require.NoError(t, err)
	require.NotEmpty(t, tools)

	for _, tl := range tools {
		info, infoErr := tl.Info(ctx)
		if infoErr != nil || info.Name != "slo_list" {
			continue
		}

		result, runErr := tl.InvokableRun(ctx, "")
		require.NoError(t, runErr)
		assert.NotEmpty(t, result)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result), &parsed))

		_, hasTotal := parsed["total"]
		_, hasSLOs := parsed["slos"]
		assert.True(t, hasTotal || hasSLOs)
		return
	}
	t.Skip("slo_list tool not found in selected tools")
}

func TestIntegration_FullChain_TimeoutContext(t *testing.T) {
	poolRegistry, _, _, _ := setupFullTestEnv(t)

	ctx, cancel := context.WithTimeout(ctxWithUser("user-1"), 5*time.Second)
	defer cancel()

	tools, err := poolRegistry.SelectTools(ctx, "告警")
	require.NoError(t, err)
	assert.NotEmpty(t, tools)
}
