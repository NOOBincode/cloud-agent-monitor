package eino

import (
	"context"
	"encoding/json"
	"fmt"

	agentDomain "cloud-agent-monitor/internal/agent/domain"
	"cloud-agent-monitor/internal/slo/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// SLOTool is the fat tool for SLO queries. It dispatches by action to the SLOServiceInterface.
// Supported actions: list, get, error_budget, burn_rate_alerts, summary.
type SLOTool struct {
	service domain.SLOServiceInterface
}

// NewSLOTool creates a new SLOTool fat tool backed by the given service.
func NewSLOTool(service domain.SLOServiceInterface) *SLOTool {
	return &SLOTool{service: service}
}

func (t *SLOTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "slo_query",
		Desc: `Query SLO (Service Level Objective) status, error budgets, and burn rates.

Actions:
- list: List all SLOs, optionally filtered by service
- get: Get detailed SLO information by ID
- error_budget: Get error budget for a specific SLO
- burn_rate_alerts: Get current burn rate alerts
- summary: Get SLO summary statistics

All operations are READ-ONLY.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type: schema.String,
				Desc: "Action to perform: list, get, error_budget, burn_rate_alerts, summary",
				Enum: []string{"list", "get", "error_budget", "burn_rate_alerts", "summary"},
			},
			"slo_id": {
				Type: schema.String,
				Desc: "SLO ID (required for 'get' and 'error_budget' actions)",
			},
			"service_id": {
				Type: schema.String,
				Desc: "Filter by service ID (optional for 'list' action)",
			},
			"status": {
				Type: schema.String,
				Desc: "Filter by status (optional for 'list' action)",
				Enum: []string{"healthy", "warning", "critical", "unknown"},
			},
		}),
	}, nil
}

type SLOToolArgs struct {
	Action    string `json:"action"`
	SLOID     string `json:"slo_id,omitempty"`
	ServiceID string `json:"service_id,omitempty"`
	Status    string `json:"status,omitempty"`
}

func (t *SLOTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args SLOToolArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	var result any
	var err error

	switch args.Action {
	case "list":
		result, err = t.listSLOs(ctx, args)
	case "get":
		result, err = t.getSLO(ctx, args)
	case "error_budget":
		result, err = t.getErrorBudget(ctx, args)
	case "burn_rate_alerts":
		result, err = t.getBurnRateAlerts(ctx)
	case "summary":
		result, err = t.getSummary(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s", args.Action)
	}

	if err != nil {
		return "", err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(data), nil
}

func (t *SLOTool) listSLOs(ctx context.Context, args SLOToolArgs) (any, error) {
	filter := domain.SLOFilter{Page: 1, PageSize: 100}

	if args.ServiceID != "" {
		serviceID, err := uuid.Parse(args.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("invalid service_id: %w", err)
		}
		filter.ServiceID = serviceID
	}

	if args.Status != "" {
		filter.Status = domain.SLOStatus(args.Status)
	}

	slos, total, err := t.service.ListSLOs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list SLOs: %w", err)
	}

	return map[string]any{
		"total": total,
		"slos":  t.simplifySLOs(slos),
	}, nil
}

func (t *SLOTool) getSLO(ctx context.Context, args SLOToolArgs) (any, error) {
	if args.SLOID == "" {
		return nil, fmt.Errorf("slo_id is required for 'get' action")
	}

	sloID, err := uuid.Parse(args.SLOID)
	if err != nil {
		return nil, fmt.Errorf("invalid slo_id: %w", err)
	}

	slo, err := t.service.GetSLO(ctx, sloID)
	if err != nil {
		return nil, fmt.Errorf("failed to get SLO: %w", err)
	}

	return t.simplifySLO(slo), nil
}

func (t *SLOTool) getErrorBudget(ctx context.Context, args SLOToolArgs) (any, error) {
	if args.SLOID == "" {
		return nil, fmt.Errorf("slo_id is required for 'error_budget' action")
	}

	sloID, err := uuid.Parse(args.SLOID)
	if err != nil {
		return nil, fmt.Errorf("invalid slo_id: %w", err)
	}

	budget, err := t.service.GetErrorBudget(ctx, sloID)
	if err != nil {
		return nil, fmt.Errorf("failed to get error budget: %w", err)
	}

	return map[string]any{
		"total":      budget.Total,
		"remaining":  budget.Remaining,
		"consumed":   budget.Consumed,
		"percentage": budget.Percentage,
		"window":     budget.Window,
		"updated_at": budget.UpdatedAt,
	}, nil
}

func (t *SLOTool) getBurnRateAlerts(ctx context.Context) (any, error) {
	alerts, err := t.service.GetBurnRateAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get burn rate alerts: %w", err)
	}

	result := make([]map[string]any, len(alerts))
	for i, alert := range alerts {
		result[i] = map[string]any{
			"slo_id":       alert.SLOID.String(),
			"slo_name":     alert.SLOName,
			"service_name": alert.ServiceName,
			"current_rate": alert.CurrentRate,
			"threshold":    alert.Threshold,
			"severity":     alert.Severity,
			"window":       alert.Window,
			"fired_at":     alert.FiredAt,
		}
	}

	return map[string]any{
		"total":  len(result),
		"alerts": result,
	}, nil
}

func (t *SLOTool) getSummary(ctx context.Context) (any, error) {
	summary, err := t.service.GetSLOSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SLO summary: %w", err)
	}

	return map[string]any{
		"total":         summary.Total,
		"by_status":     summary.ByStatus,
		"by_service":    summary.ByService,
		"avg_burn_rate": summary.AvgBurnRate,
	}, nil
}

func (t *SLOTool) simplifySLOs(slos []*domain.SLO) []map[string]any {
	result := make([]map[string]any, len(slos))
	for i, slo := range slos {
		result[i] = t.simplifySLO(slo)
	}
	return result
}

func (t *SLOTool) simplifySLO(slo *domain.SLO) map[string]any {
	return map[string]any{
		"id":           slo.ID.String(),
		"name":         slo.Name,
		"description":  slo.Description,
		"service_id":   slo.ServiceID.String(),
		"service_name": slo.ServiceName,
		"target":       slo.Target,
		"window":       slo.Window,
		"status":       string(slo.Status),
		"current":      slo.Current,
		"burn_rate":    slo.BurnRate,
		"error_budget": map[string]any{
			"total":      slo.ErrorBudget.Total,
			"remaining":  slo.ErrorBudget.Remaining,
			"consumed":   slo.ErrorBudget.Consumed,
			"percentage": slo.ErrorBudget.Percentage,
		},
		"sli": map[string]any{
			"name":  slo.SLI.Name,
			"type":  string(slo.SLI.Type),
			"query": slo.SLI.Query,
		},
	}
}

func (t *SLOTool) IsReadOnly() bool {
	return true
}

func (t *SLOTool) RequiredPermission() string {
	return "slo:read"
}

type SLOListTool struct {
	delegate *FatToolDelegator
}

func NewSLOListTool(fat *SLOTool) *SLOListTool {
	return &SLOListTool{delegate: NewFatToolDelegator(fat, "list")}
}
func (t *SLOListTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "slo_list",
		Desc: "List all SLOs with optional filtering by service and status.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"service_id": {Type: schema.String, Desc: "Filter by service ID"},
			"status":     {Type: schema.String, Desc: "Filter by status", Enum: []string{"healthy", "warning", "critical", "unknown"}},
		}),
	}, nil
}
func (t *SLOListTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *SLOListTool) IsReadOnly() bool           { return true }
func (t *SLOListTool) RequiredPermission() string { return "slo:read" }

type SLOGetTool struct {
	delegate *FatToolDelegator
}

func NewSLOGetTool(fat *SLOTool) *SLOGetTool {
	return &SLOGetTool{delegate: NewFatToolDelegator(fat, "get")}
}
func (t *SLOGetTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "slo_get",
		Desc: "Get detailed SLO information by ID, including SLI configuration and error budget.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"slo_id": {Type: schema.String, Desc: "SLO ID (required)"},
		}),
	}, nil
}
func (t *SLOGetTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *SLOGetTool) IsReadOnly() bool           { return true }
func (t *SLOGetTool) RequiredPermission() string { return "slo:read" }

type SLOGetErrorBudgetTool struct {
	delegate *FatToolDelegator
}

func NewSLOGetErrorBudgetTool(fat *SLOTool) *SLOGetErrorBudgetTool {
	return &SLOGetErrorBudgetTool{delegate: NewFatToolDelegator(fat, "error_budget")}
}
func (t *SLOGetErrorBudgetTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "slo_get_error_budget",
		Desc: "Get error budget details for a specific SLO — remaining, consumed, and percentage.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"slo_id": {Type: schema.String, Desc: "SLO ID (required)"},
		}),
	}, nil
}
func (t *SLOGetErrorBudgetTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *SLOGetErrorBudgetTool) IsReadOnly() bool           { return true }
func (t *SLOGetErrorBudgetTool) RequiredPermission() string { return "slo:read" }

type SLOGetBurnRateAlertsTool struct {
	delegate *FatToolDelegator
}

func NewSLOGetBurnRateAlertsTool(fat *SLOTool) *SLOGetBurnRateAlertsTool {
	return &SLOGetBurnRateAlertsTool{delegate: NewFatToolDelegator(fat, "burn_rate_alerts")}
}
func (t *SLOGetBurnRateAlertsTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "slo_get_burn_rate_alerts",
		Desc:        "Get current burn rate alerts — SLOs burning through error budget too fast.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}
func (t *SLOGetBurnRateAlertsTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *SLOGetBurnRateAlertsTool) IsReadOnly() bool           { return true }
func (t *SLOGetBurnRateAlertsTool) RequiredPermission() string { return "slo:read" }

type SLOGetSummaryTool struct {
	delegate *FatToolDelegator
}

func NewSLOGetSummaryTool(fat *SLOTool) *SLOGetSummaryTool {
	return &SLOGetSummaryTool{delegate: NewFatToolDelegator(fat, "summary")}
}
func (t *SLOGetSummaryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "slo_get_summary",
		Desc:        "Get SLO summary statistics — counts by status, average burn rate, breakdown by service.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}
func (t *SLOGetSummaryTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *SLOGetSummaryTool) IsReadOnly() bool           { return true }
func (t *SLOGetSummaryTool) RequiredPermission() string { return "slo:read" }

// SLOToolProvider implements domain.ToolProvider for the SLO category.
// It creates 5 thin tools (list, get, error_budget, burn_rate_alerts, summary)
// that delegate to a single SLOTool fat tool.
type SLOToolProvider struct {
	service domain.SLOServiceInterface
}

// NewSLOToolProvider creates a new SLO provider backed by the given service.
func NewSLOToolProvider(svc domain.SLOServiceInterface) *SLOToolProvider {
	return &SLOToolProvider{service: svc}
}

func (p *SLOToolProvider) Category() agentDomain.ToolCategory {
	return agentDomain.CategorySLO
}

func (p *SLOToolProvider) Tools(ctx context.Context) ([]agentDomain.ToolSpec, error) {
	return []agentDomain.ToolSpec{
		{Name: "slo_list", Description: "List all SLOs", RequiredPermission: "slo:read", IsReadOnly: true, Category: agentDomain.CategorySLO},
		{Name: "slo_get", Description: "Get SLO details by ID", RequiredPermission: "slo:read", IsReadOnly: true, Category: agentDomain.CategorySLO},
		{Name: "slo_get_error_budget", Description: "Get error budget for an SLO", RequiredPermission: "slo:read", IsReadOnly: true, Category: agentDomain.CategorySLO},
		{Name: "slo_get_burn_rate_alerts", Description: "Get burn rate alerts", RequiredPermission: "slo:read", IsReadOnly: true, Category: agentDomain.CategorySLO},
		{Name: "slo_get_summary", Description: "Get SLO summary statistics", RequiredPermission: "slo:read", IsReadOnly: true, Category: agentDomain.CategorySLO},
	}, nil
}

func (p *SLOToolProvider) DefaultPools() []*agentDomain.ToolPool {
	return []*agentDomain.ToolPool{}
}

func (p *SLOToolProvider) CreateTools() []ReadOnlyTool {
	fat := NewSLOTool(p.service)
	return []ReadOnlyTool{
		NewSLOListTool(fat),
		NewSLOGetTool(fat),
		NewSLOGetErrorBudgetTool(fat),
		NewSLOGetBurnRateAlertsTool(fat),
		NewSLOGetSummaryTool(fat),
	}
}
