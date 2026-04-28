package eino

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"cloud-agent-monitor/internal/agent/domain"
	alertApp "cloud-agent-monitor/internal/alerting/application"
	alertDomain "cloud-agent-monitor/internal/alerting/domain"
	sloDomain "cloud-agent-monitor/internal/slo/domain"
	"cloud-agent-monitor/internal/storage/models"
	topoDomain "cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertingToolProvider_Category(t *testing.T) {
	provider := NewAlertingToolProvider(&mockAlertingService{})
	assert.Equal(t, domain.CategoryAlerting, provider.Category())
}

func TestAlertingToolProvider_Tools(t *testing.T) {
	provider := NewAlertingToolProvider(&mockAlertingService{})
	tools, err := provider.Tools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 6)

	names := make(map[string]bool)
	for _, spec := range tools {
		names[spec.Name] = true
		assert.Equal(t, "alerting:read", spec.RequiredPermission)
		assert.True(t, spec.IsReadOnly)
	}
	assert.True(t, names["alerting_list_active"])
	assert.True(t, names["alerting_list_history"])
	assert.True(t, names["alerting_stats"])
	assert.True(t, names["alerting_noisy"])
	assert.True(t, names["alerting_high_risk"])
	assert.True(t, names["alerting_get_feedback"])
}

func TestAlertingToolProvider_DefaultPools(t *testing.T) {
	provider := NewAlertingToolProvider(&mockAlertingService{})
	pools := provider.DefaultPools()
	require.Len(t, pools, 1)
	assert.Equal(t, "alert", pools[0].ID)
	assert.Equal(t, 10, pools[0].Priority)
	assert.True(t, pools[0].IsBuiltin)
}

func TestAlertingToolProvider_CreateTools(t *testing.T) {
	provider := NewAlertingToolProvider(&mockAlertingService{})
	tools := provider.CreateTools()
	assert.Len(t, tools, 6)

	for _, tl := range tools {
		assert.True(t, tl.IsReadOnly())
		assert.Equal(t, "alerting:read", tl.RequiredPermission())
	}
}

func TestAlertingTool_FatTool_Actions(t *testing.T) {
	now := time.Now()
	_ = now
	mockSvc := &mockAlertingService{
		alerts:    []*alertDomain.Alert{{ID: uuid.New(), Labels: map[string]string{"alertname": "TestAlert"}}},
		records:   []*models.AlertRecord{{ID: uuid.New(), Fingerprint: "fp1"}},
		recordCnt: 1,
		stats:     &models.AlertRecordStats{TotalCount: 100},
		noisy:     []*models.AlertNoiseRecord{{AlertFingerprint: "fp-noise"}},
		highRisk:  []*models.AlertNoiseRecord{{AlertFingerprint: "fp-risk"}},
		feedback:  &alertApp.AlertFeedback{Fingerprint: "fp1", AlertName: "TestAlert"},
	}
	fat := NewAlertingTool(mockSvc)

	tests := []struct {
		name   string
		action string
		args   string
	}{
		{"list_active", "list_active", ""},
		{"list_history", "list_history", `{"severity":"critical"}`},
		{"stats", "stats", ""},
		{"noisy", "noisy", ""},
		{"high_risk", "high_risk", ""},
		{"feedback", "feedback", `{"fingerprint":"fp1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.args
			if args == "" {
				args = "{}"
			}
			var m map[string]interface{}
			_ = json.Unmarshal([]byte(args), &m)
			m["action"] = tt.action
			argsJSON, _ := json.Marshal(m)

			result, err := fat.InvokableRun(context.Background(), string(argsJSON))
			require.NoError(t, err)
			assert.NotEmpty(t, result)

			var parsed map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(result), &parsed))
		})
	}

	t.Run("unknown action returns error", func(t *testing.T) {
		_, err := fat.InvokableRun(context.Background(), `{"action":"unknown"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown action")
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		_, err := fat.InvokableRun(context.Background(), "not-json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid arguments")
	})
}

func TestAlertingThinTools_Delegate(t *testing.T) {
	mockSvc := &mockAlertingService{
		alerts:    []*alertDomain.Alert{{ID: uuid.New()}},
		records:   []*models.AlertRecord{{ID: uuid.New()}},
		recordCnt: 1,
		stats:     &models.AlertRecordStats{TotalCount: 50},
		noisy:     []*models.AlertNoiseRecord{{AlertFingerprint: "fp-n"}},
		highRisk:  []*models.AlertNoiseRecord{{AlertFingerprint: "fp-h"}},
		feedback:  &alertApp.AlertFeedback{Fingerprint: "fp1"},
	}
	fat := NewAlertingTool(mockSvc)

	tests := []struct {
		name     string
		tool     ReadOnlyTool
		action   string
		argsJSON string
	}{
		{"list_active", NewAlertingListActiveTool(fat), "list_active", ""},
		{"list_history", NewAlertingListHistoryTool(fat), "list_history", `{"limit":10}`},
		{"stats", NewAlertingStatsTool(fat), "stats", ""},
		{"noisy", NewAlertingNoisyTool(fat), "noisy", ""},
		{"high_risk", NewAlertingHighRiskTool(fat), "high_risk", ""},
		{"feedback", NewAlertingGetFeedbackTool(fat), "feedback", `{"fingerprint":"fp1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := tt.tool.Info(context.Background())
			require.NoError(t, err)
			assert.NotEmpty(t, info.Name)

			result, err := tt.tool.InvokableRun(context.Background(), tt.argsJSON)
			require.NoError(t, err)
			assert.NotEmpty(t, result)

			var parsed map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(result), &parsed))
		})
	}
}

func TestSLOToolProvider_Category(t *testing.T) {
	provider := NewSLOToolProvider(&mockSLOService{})
	assert.Equal(t, domain.CategorySLO, provider.Category())
}

func TestSLOToolProvider_Tools(t *testing.T) {
	provider := NewSLOToolProvider(&mockSLOService{})
	tools, err := provider.Tools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 5)
}

func TestSLOToolProvider_CreateTools(t *testing.T) {
	provider := NewSLOToolProvider(&mockSLOService{})
	tools := provider.CreateTools()
	assert.Len(t, tools, 5)
	for _, tl := range tools {
		assert.True(t, tl.IsReadOnly())
	}
}

func TestSLOThinTools_Delegate(t *testing.T) {
	id := uuid.New()
	mockSvc := &mockSLOService{
		slos:        []*sloDomain.SLO{{ID: id, Name: "availability-slo"}},
		slo:         &sloDomain.SLO{ID: id, Name: "availability-slo"},
		summary:     &sloDomain.SLOSummary{Total: 5},
		errorBudget: &sloDomain.ErrorBudget{Remaining: 99.5},
		burnRate:    []*sloDomain.BurnRateAlert{{SLOID: id}},
	}
	fat := NewSLOTool(mockSvc)

	tests := []struct {
		name     string
		tool     ReadOnlyTool
		argsJSON string
	}{
		{"list", NewSLOListTool(fat), ""},
		{"get", NewSLOGetTool(fat), mustMarshal(map[string]string{"slo_id": id.String()})},
		{"error_budget", NewSLOGetErrorBudgetTool(fat), mustMarshal(map[string]string{"slo_id": id.String()})},
		{"burn_rate", NewSLOGetBurnRateAlertsTool(fat), ""},
		{"summary", NewSLOGetSummaryTool(fat), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := tt.tool.Info(context.Background())
			require.NoError(t, err)
			assert.NotEmpty(t, info.Name)

			result, err := tt.tool.InvokableRun(context.Background(), tt.argsJSON)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
		})
	}
}

func TestTopologyToolProvider_Category(t *testing.T) {
	provider := NewTopologyToolProvider(&mockTopologyService{})
	assert.Equal(t, domain.CategoryTopology, provider.Category())
}

func TestTopologyToolProvider_Tools(t *testing.T) {
	provider := NewTopologyToolProvider(&mockTopologyService{})
	tools, err := provider.Tools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 10)
}

func TestTopologyToolProvider_CreateTools(t *testing.T) {
	provider := NewTopologyToolProvider(&mockTopologyService{})
	tools := provider.CreateTools()
	assert.Len(t, tools, 10)
	for _, tl := range tools {
		assert.True(t, tl.IsReadOnly())
	}
}

func TestTopologyThinTools_Delegate(t *testing.T) {
	nodeID := uuid.New()
	mockSvc := &mockTopologyService{
		serviceTopology: &topoDomain.ServiceTopology{Nodes: []*topoDomain.ServiceNode{{ID: nodeID}}},
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
	fat := NewTopologyTool(mockSvc)

	thinTools := []struct {
		name     string
		tool     ReadOnlyTool
		argsJSON string
	}{
		{"get_service_topology", NewTopologyGetServiceTopologyTool(fat), ""},
		{"get_network_topology", NewTopologyGetNetworkTopologyTool(fat), ""},
		{"get_node", NewTopologyGetNodeTool(fat), mustMarshal(map[string]string{"service_id": nodeID.String()})},
		{"get_upstream", NewTopologyGetUpstreamTool(fat), mustMarshal(map[string]string{"service_id": nodeID.String()})},
		{"get_downstream", NewTopologyGetDownstreamTool(fat), mustMarshal(map[string]string{"service_id": nodeID.String()})},
		{"analyze_impact", NewTopologyAnalyzeImpactTool(fat), mustMarshal(map[string]string{"service_id": nodeID.String()})},
		{"find_path", NewTopologyFindPathTool(fat), mustMarshal(map[string]string{"source_id": nodeID.String(), "target_id": uuid.New().String()})},
		{"find_shortest_path", NewTopologyFindShortestPathTool(fat), mustMarshal(map[string]string{"source_id": nodeID.String(), "target_id": uuid.New().String()})},
		{"find_anomalies", NewTopologyFindAnomaliesTool(fat), ""},
		{"get_stats", NewTopologyGetStatsTool(fat), ""},
	}

	for _, tt := range thinTools {
		t.Run(tt.name+"_info", func(t *testing.T) {
			info, err := tt.tool.Info(context.Background())
			require.NoError(t, err)
			assert.NotEmpty(t, info.Name)
			assert.True(t, tt.tool.IsReadOnly())
			assert.Equal(t, "service:read", tt.tool.RequiredPermission())
		})
	}

	for _, tt := range thinTools {
		t.Run(tt.name+"_delegate", func(t *testing.T) {
			result, err := tt.tool.InvokableRun(context.Background(), tt.argsJSON)
			require.NoError(t, err)
			assert.NotEmpty(t, result)
		})
	}
}

func TestAlertingTool_ServiceError(t *testing.T) {
	mockSvc := &mockAlertingService{err: assert.AnError}
	fat := NewAlertingTool(mockSvc)

	_, err := fat.InvokableRun(context.Background(), `{"action":"list_active"}`)
	assert.Error(t, err)
}

func TestSLOTool_ServiceError(t *testing.T) {
	mockSvc := &mockSLOService{err: assert.AnError}
	fat := NewSLOTool(mockSvc)

	_, err := fat.InvokableRun(context.Background(), `{"action":"list"}`)
	assert.Error(t, err)
}

func TestTopologyTool_ServiceError(t *testing.T) {
	mockSvc := &mockTopologyService{err: assert.AnError}
	fat := NewTopologyTool(mockSvc)

	_, err := fat.InvokableRun(context.Background(), `{"action":"get_service_topology"}`)
	assert.Error(t, err)
}
