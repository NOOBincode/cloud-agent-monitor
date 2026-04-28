package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	alertApp "cloud-agent-monitor/internal/alerting/application"
	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/internal/storage/models"
	sloDomain "cloud-agent-monitor/internal/slo/domain"
	topoDomain "cloud-agent-monitor/internal/topology/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type stubTool struct {
	name        string
	desc        string
	result      string
	err         error
	isReadOnly  bool
	permission  string
	lastArgs    string
	lastCtx     context.Context
}

func newStubTool(name, desc string) *stubTool {
	return &stubTool{
		name:       name,
		desc:       desc,
		result:     `{"status":"ok"}`,
		isReadOnly: true,
		permission: name + ":read",
	}
}

func (s *stubTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        s.name,
		Desc:        s.desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (s *stubTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	s.lastArgs = argsJSON
	s.lastCtx = ctx
	if s.err != nil {
		return "", s.err
	}
	return s.result, nil
}

func (s *stubTool) IsReadOnly() bool          { return s.isReadOnly }
func (s *stubTool) RequiredPermission() string { return s.permission }

type mockPermissionChecker struct {
	permissions map[string]bool
	err         error
	calls       []permissionCall
}

type permissionCall struct {
	userID     string
	permission string
}

func newMockPermissionChecker() *mockPermissionChecker {
	return &mockPermissionChecker{
		permissions: make(map[string]bool),
	}
}

func (m *mockPermissionChecker) grantAll() *mockPermissionChecker {
	m.permissions["__all__"] = true
	return m
}

func (m *mockPermissionChecker) HasPermission(_ context.Context, userID string, permission string) (bool, error) {
	m.calls = append(m.calls, permissionCall{userID: userID, permission: permission})
	if m.err != nil {
		return false, m.err
	}
	if m.permissions["__all__"] {
		return true, nil
	}
	key := userID + ":" + permission
	return m.permissions[key], nil
}

func (m *mockPermissionChecker) grant(userID, permission string) {
	m.permissions[userID+":"+permission] = true
}

type mockAlertingService struct {
	alerts     []*domain.Alert
	records    []*models.AlertRecord
	recordCnt  int64
	stats      *models.AlertRecordStats
	noisy      []*models.AlertNoiseRecord
	highRisk   []*models.AlertNoiseRecord
	feedback   *alertApp.AlertFeedback
	err        error
}

func (m *mockAlertingService) GetAlerts(_ context.Context, _ domain.AlertFilter) ([]*domain.Alert, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.alerts, nil
}

func (m *mockAlertingService) GetAlertRecords(_ context.Context, _ models.AlertRecordFilter) ([]*models.AlertRecord, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.records, m.recordCnt, nil
}

func (m *mockAlertingService) GetAlertRecordStats(_ context.Context, _, _ time.Time) (*models.AlertRecordStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func (m *mockAlertingService) GetNoisyAlerts(_ context.Context, _ int) ([]*models.AlertNoiseRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.noisy, nil
}

func (m *mockAlertingService) GetHighRiskAlerts(_ context.Context, _ int) ([]*models.AlertNoiseRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.highRisk, nil
}

func (m *mockAlertingService) GetAlertFeedback(_ context.Context, _ string) (*alertApp.AlertFeedback, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.feedback, nil
}

type mockSLOService struct {
	slos         []*sloDomain.SLO
	slo          *sloDomain.SLO
	summary      *sloDomain.SLOSummary
	errorBudget  *sloDomain.ErrorBudget
	burnRate     []*sloDomain.BurnRateAlert
	err          error
}

func (m *mockSLOService) CreateSLO(_ context.Context, slo *sloDomain.SLO, _ *sloDomain.SLI) (*sloDomain.SLO, error) {
	return slo, m.err
}
func (m *mockSLOService) UpdateSLO(_ context.Context, slo *sloDomain.SLO) (*sloDomain.SLO, error) {
	return slo, m.err
}
func (m *mockSLOService) DeleteSLO(_ context.Context, _ uuid.UUID) error { return m.err }
func (m *mockSLOService) GetSLO(_ context.Context, _ uuid.UUID) (*sloDomain.SLO, error) {
	return m.slo, m.err
}
func (m *mockSLOService) GetSLOByService(_ context.Context, _ uuid.UUID) ([]*sloDomain.SLO, error) {
	return m.slos, m.err
}
func (m *mockSLOService) ListSLOs(_ context.Context, _ sloDomain.SLOFilter) ([]*sloDomain.SLO, int64, error) {
	return m.slos, int64(len(m.slos)), m.err
}
func (m *mockSLOService) GetSLOSummary(_ context.Context) (*sloDomain.SLOSummary, error) {
	return m.summary, m.err
}
func (m *mockSLOService) RefreshSLOStatus(_ context.Context, _ uuid.UUID) (*sloDomain.SLO, error) {
	return m.slo, m.err
}
func (m *mockSLOService) RefreshAllSLOStatus(_ context.Context) error { return m.err }
func (m *mockSLOService) GetErrorBudget(_ context.Context, _ uuid.UUID) (*sloDomain.ErrorBudget, error) {
	return m.errorBudget, m.err
}
func (m *mockSLOService) GetBurnRateAlerts(_ context.Context) ([]*sloDomain.BurnRateAlert, error) {
	return m.burnRate, m.err
}

type mockTopologyService struct {
	serviceTopology *topoDomain.ServiceTopology
	networkTopology *topoDomain.NetworkTopology
	node            *topoDomain.ServiceNode
	upstream        []*topoDomain.ServiceNode
	downstream      []*topoDomain.ServiceNode
	impact          *topoDomain.ImpactResult
	pathResult      *topoDomain.PathResult
	shortestPath    []topoDomain.PathHop
	anomalies       []*topoDomain.TopologyAnomaly
	stats           *topoDomain.TopologyStats
	err             error
}

func (m *mockTopologyService) GetServiceTopology(_ context.Context, _ topoDomain.TopologyQuery) (*topoDomain.ServiceTopology, error) {
	return m.serviceTopology, m.err
}
func (m *mockTopologyService) GetNetworkTopology(_ context.Context, _ topoDomain.TopologyQuery) (*topoDomain.NetworkTopology, error) {
	return m.networkTopology, m.err
}
func (m *mockTopologyService) GetServiceNode(_ context.Context, _ uuid.UUID) (*topoDomain.ServiceNode, error) {
	return m.node, m.err
}
func (m *mockTopologyService) GetServiceNodeByName(_ context.Context, _, _ string) (*topoDomain.ServiceNode, error) {
	return m.node, m.err
}
func (m *mockTopologyService) GetUpstreamServices(_ context.Context, _ uuid.UUID, _ int) ([]*topoDomain.ServiceNode, error) {
	return m.upstream, m.err
}
func (m *mockTopologyService) GetDownstreamServices(_ context.Context, _ uuid.UUID, _ int) ([]*topoDomain.ServiceNode, error) {
	return m.downstream, m.err
}
func (m *mockTopologyService) AnalyzeImpact(_ context.Context, _ uuid.UUID, _ int) (*topoDomain.ImpactResult, error) {
	return m.impact, m.err
}
func (m *mockTopologyService) FindPath(_ context.Context, _, _ uuid.UUID, _ int) (*topoDomain.PathResult, error) {
	return m.pathResult, m.err
}
func (m *mockTopologyService) FindShortestPath(_ context.Context, _, _ uuid.UUID) ([]topoDomain.PathHop, error) {
	return m.shortestPath, m.err
}
func (m *mockTopologyService) FindAnomalies(_ context.Context) ([]*topoDomain.TopologyAnomaly, error) {
	return m.anomalies, m.err
}
func (m *mockTopologyService) GetTopologyStats(_ context.Context) (*topoDomain.TopologyStats, error) {
	return m.stats, m.err
}

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshal: %v", err))
	}
	return string(b)
}

func ctxWithUser(userID string) context.Context {
	return ContextWithUserID(context.Background(), userID)
}
