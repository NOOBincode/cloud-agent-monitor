package eino

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud-agent-monitor/internal/slo/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type SLOTool struct {
	service domain.SLOServiceInterface
}

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
