package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentDomain "cloud-agent-monitor/internal/agent/domain"
	"cloud-agent-monitor/internal/alerting/application"
	"cloud-agent-monitor/internal/alerting/domain"
	"cloud-agent-monitor/internal/storage/models"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// AlertingServiceInterface defines the alerting operations required by the fat tool.
type AlertingServiceInterface interface {
	GetAlerts(ctx context.Context, filter domain.AlertFilter) ([]*domain.Alert, error)
	GetAlertRecords(ctx context.Context, filter models.AlertRecordFilter) ([]*models.AlertRecord, int64, error)
	GetAlertRecordStats(ctx context.Context, from, to time.Time) (*models.AlertRecordStats, error)
	GetNoisyAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	GetHighRiskAlerts(ctx context.Context, limit int) ([]*models.AlertNoiseRecord, error)
	GetAlertFeedback(ctx context.Context, fingerprint string) (*application.AlertFeedback, error)
}

// AlertingTool is the fat tool that dispatches alerting operations by action.
// Supported actions: list_active, list_history, stats, noisy, high_risk, feedback.
type AlertingTool struct {
	alertService AlertingServiceInterface
}

// NewAlertingTool creates a new AlertingTool fat tool backed by the given service.
func NewAlertingTool(service AlertingServiceInterface) *AlertingTool {
	return &AlertingTool{alertService: service}
}

func (t *AlertingTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_query",
		Desc: `Query alerting information including active alerts, alert history, and noise analysis.

Actions:
- list_active: List currently active alerts
- list_history: List alert history records
- stats: Get alert statistics for a time range
- noisy: Get noisy alerts (frequent alerts that may need tuning)
- high_risk: Get high-risk alerts
- feedback: Get feedback for a specific alert

All operations are READ-ONLY. Agent cannot create, modify, or silence alerts.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type: schema.String,
				Desc: "Action to perform",
				Enum: []string{"list_active", "list_history", "stats", "noisy", "high_risk", "feedback"},
			},
			"fingerprint": {
				Type: schema.String,
				Desc: "Alert fingerprint (required for 'feedback' action)",
			},
			"from": {
				Type: schema.String,
				Desc: "Start time in RFC3339 format (for 'stats' action)",
			},
			"to": {
				Type: schema.String,
				Desc: "End time in RFC3339 format (for 'stats' action)",
			},
			"limit": {
				Type: schema.Integer,
				Desc: "Maximum number of results (default: 20)",
			},
			"severity": {
				Type: schema.String,
				Desc: "Filter by severity (for 'list_history' action)",
				Enum: []string{"critical", "warning", "info"},
			},
		}),
	}, nil
}

type AlertingToolArgs struct {
	Action      string `json:"action"`
	Fingerprint string `json:"fingerprint,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

func (t *AlertingTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args AlertingToolArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	var result any
	var err error

	switch args.Action {
	case "list_active":
		result, err = t.listActiveAlerts(ctx)
	case "list_history":
		result, err = t.listHistory(ctx, args)
	case "stats":
		result, err = t.getStats(ctx, args)
	case "noisy":
		result, err = t.getNoisyAlerts(ctx, args)
	case "high_risk":
		result, err = t.getHighRiskAlerts(ctx, args)
	case "feedback":
		result, err = t.getFeedback(ctx, args)
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

func (t *AlertingTool) listActiveAlerts(ctx context.Context) (any, error) {
	filter := domain.AlertFilter{}
	active := true
	filter.Active = &active

	alerts, err := t.alertService.GetAlerts(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list active alerts: %w", err)
	}

	result := make([]map[string]any, len(alerts))
	for i, alert := range alerts {
		result[i] = map[string]any{
			"id":          alert.ID.String(),
			"name":        alert.Labels["alertname"],
			"severity":    alert.Labels["severity"],
			"service":     alert.Labels["service"],
			"status":      alert.Status,
			"starts_at":   alert.StartsAt,
			"summary":     alert.Annotations["summary"],
			"description": alert.Annotations["description"],
		}
	}

	return map[string]any{
		"total":  len(result),
		"alerts": result,
	}, nil
}

func (t *AlertingTool) listHistory(ctx context.Context, args AlertingToolArgs) (any, error) {
	limit := 20
	if args.Limit > 0 {
		limit = args.Limit
	}

	filter := models.AlertRecordFilter{
		Page:     1,
		PageSize: limit,
	}

	if args.Severity != "" {
		filter.Severity = models.AlertSeverity(args.Severity)
	}

	records, total, err := t.alertService.GetAlertRecords(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list alert history: %w", err)
	}

	result := make([]map[string]any, len(records))
	for i, record := range records {
		alertName := ""
		if record.Labels != nil {
			alertName = record.Labels["alertname"]
		}
		serviceName := ""
		if record.Labels != nil {
			serviceName = record.Labels["service"]
		}
		result[i] = map[string]any{
			"id":          record.ID.String(),
			"fingerprint": record.Fingerprint,
			"alert_name":  alertName,
			"severity":    string(record.Severity),
			"status":      string(record.Status),
			"service":     serviceName,
			"starts_at":   record.StartsAt,
			"ends_at":     record.EndsAt,
		}
	}

	return map[string]any{
		"total":   total,
		"records": result,
	}, nil
}

func (t *AlertingTool) getStats(ctx context.Context, args AlertingToolArgs) (any, error) {
	to := time.Now()
	from := to.Add(-24 * time.Hour)

	if args.From != "" {
		if parsed, err := time.Parse(time.RFC3339, args.From); err == nil {
			from = parsed
		}
	}
	if args.To != "" {
		if parsed, err := time.Parse(time.RFC3339, args.To); err == nil {
			to = parsed
		}
	}

	stats, err := t.alertService.GetAlertRecordStats(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert stats: %w", err)
	}

	return map[string]any{
		"from":              from,
		"to":                to,
		"total_count":       stats.TotalCount,
		"firing_count":      stats.FiringCount,
		"resolved_count":    stats.ResolvedCount,
		"critical_count":    stats.CriticalCount,
		"warning_count":     stats.WarningCount,
		"info_count":        stats.InfoCount,
		"avg_duration_secs": stats.AvgDuration,
	}, nil
}

func (t *AlertingTool) getNoisyAlerts(ctx context.Context, args AlertingToolArgs) (any, error) {
	limit := 20
	if args.Limit > 0 {
		limit = args.Limit
	}

	alerts, err := t.alertService.GetNoisyAlerts(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get noisy alerts: %w", err)
	}

	result := make([]map[string]any, len(alerts))
	for i, alert := range alerts {
		result[i] = map[string]any{
			"fingerprint":   alert.AlertFingerprint,
			"alert_name":    alert.AlertName,
			"fire_count":    alert.FireCount,
			"last_fired_at": alert.LastFiredAt,
		}
	}

	return map[string]any{
		"total":  len(result),
		"alerts": result,
	}, nil
}

func (t *AlertingTool) getHighRiskAlerts(ctx context.Context, args AlertingToolArgs) (any, error) {
	limit := 20
	if args.Limit > 0 {
		limit = args.Limit
	}

	alerts, err := t.alertService.GetHighRiskAlerts(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get high-risk alerts: %w", err)
	}

	result := make([]map[string]any, len(alerts))
	for i, alert := range alerts {
		result[i] = map[string]any{
			"fingerprint":   alert.AlertFingerprint,
			"alert_name":    alert.AlertName,
			"fire_count":    alert.FireCount,
			"last_fired_at": alert.LastFiredAt,
		}
	}

	return map[string]any{
		"total":  len(result),
		"alerts": result,
	}, nil
}

func (t *AlertingTool) getFeedback(ctx context.Context, args AlertingToolArgs) (any, error) {
	if args.Fingerprint == "" {
		return nil, fmt.Errorf("fingerprint is required for 'feedback' action")
	}

	feedback, err := t.alertService.GetAlertFeedback(ctx, args.Fingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert feedback: %w", err)
	}

	return feedback, nil
}

func (t *AlertingTool) IsReadOnly() bool {
	return true
}

func (t *AlertingTool) RequiredPermission() string {
	return "alerting:read"
}

type AlertingListActiveTool struct {
	delegate *FatToolDelegator
}

func NewAlertingListActiveTool(fat *AlertingTool) *AlertingListActiveTool {
	return &AlertingListActiveTool{delegate: NewFatToolDelegator(fat, "list_active")}
}

func (t *AlertingListActiveTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "alerting_list_active",
		Desc:        "List all currently active (firing) alerts with severity, service, and summary details.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}
func (t *AlertingListActiveTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingListActiveTool) IsReadOnly() bool           { return true }
func (t *AlertingListActiveTool) RequiredPermission() string { return "alerting:read" }

type AlertingListHistoryTool struct {
	delegate *FatToolDelegator
}

func NewAlertingListHistoryTool(fat *AlertingTool) *AlertingListHistoryTool {
	return &AlertingListHistoryTool{delegate: NewFatToolDelegator(fat, "list_history")}
}

func (t *AlertingListHistoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_list_history",
		Desc: "List alert history records with optional severity filtering and pagination.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"limit":    {Type: schema.Integer, Desc: "Maximum number of results (default: 20)"},
			"severity": {Type: schema.String, Desc: "Filter by severity", Enum: []string{"critical", "warning", "info"}},
		}),
	}, nil
}
func (t *AlertingListHistoryTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingListHistoryTool) IsReadOnly() bool           { return true }
func (t *AlertingListHistoryTool) RequiredPermission() string { return "alerting:read" }

type AlertingStatsTool struct {
	delegate *FatToolDelegator
}

func NewAlertingStatsTool(fat *AlertingTool) *AlertingStatsTool {
	return &AlertingStatsTool{delegate: NewFatToolDelegator(fat, "stats")}
}

func (t *AlertingStatsTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_stats",
		Desc: "Get alert statistics for a time range (counts by severity, average duration).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"from": {Type: schema.String, Desc: "Start time in RFC3339 format"},
			"to":   {Type: schema.String, Desc: "End time in RFC3339 format"},
		}),
	}, nil
}
func (t *AlertingStatsTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingStatsTool) IsReadOnly() bool           { return true }
func (t *AlertingStatsTool) RequiredPermission() string { return "alerting:read" }

type AlertingNoisyTool struct {
	delegate *FatToolDelegator
}

func NewAlertingNoisyTool(fat *AlertingTool) *AlertingNoisyTool {
	return &AlertingNoisyTool{delegate: NewFatToolDelegator(fat, "noisy")}
}

func (t *AlertingNoisyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_noisy",
		Desc: "Get noisy alerts — frequently firing alerts that may need tuning or silencing.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"limit": {Type: schema.Integer, Desc: "Maximum number of results (default: 20)"},
		}),
	}, nil
}
func (t *AlertingNoisyTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingNoisyTool) IsReadOnly() bool           { return true }
func (t *AlertingNoisyTool) RequiredPermission() string { return "alerting:read" }

type AlertingHighRiskTool struct {
	delegate *FatToolDelegator
}

func NewAlertingHighRiskTool(fat *AlertingTool) *AlertingHighRiskTool {
	return &AlertingHighRiskTool{delegate: NewFatToolDelegator(fat, "high_risk")}
}

func (t *AlertingHighRiskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_high_risk",
		Desc: "Get high-risk alerts based on frequency, severity, and impact analysis.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"limit": {Type: schema.Integer, Desc: "Maximum number of results (default: 20)"},
		}),
	}, nil
}
func (t *AlertingHighRiskTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingHighRiskTool) IsReadOnly() bool           { return true }
func (t *AlertingHighRiskTool) RequiredPermission() string { return "alerting:read" }

type AlertingGetFeedbackTool struct {
	delegate *FatToolDelegator
}

func NewAlertingGetFeedbackTool(fat *AlertingTool) *AlertingGetFeedbackTool {
	return &AlertingGetFeedbackTool{delegate: NewFatToolDelegator(fat, "feedback")}
}

func (t *AlertingGetFeedbackTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "alerting_get_feedback",
		Desc: "Get user feedback (true/false positive, comments) for a specific alert by fingerprint.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"fingerprint": {Type: schema.String, Desc: "Alert fingerprint (required)"},
		}),
	}, nil
}
func (t *AlertingGetFeedbackTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *AlertingGetFeedbackTool) IsReadOnly() bool           { return true }
func (t *AlertingGetFeedbackTool) RequiredPermission() string { return "alerting:read" }

// AlertingToolProvider implements domain.ToolProvider for the alerting category.
// It creates 6 thin tools (list_active, list_history, stats, noisy, high_risk, get_feedback)
// that delegate to a single AlertingTool fat tool.
type AlertingToolProvider struct {
	alertService AlertingServiceInterface
}

// NewAlertingToolProvider creates a new alerting provider backed by the given service.
func NewAlertingToolProvider(svc AlertingServiceInterface) *AlertingToolProvider {
	return &AlertingToolProvider{alertService: svc}
}

func (p *AlertingToolProvider) Category() agentDomain.ToolCategory {
	return agentDomain.CategoryAlerting
}

func (p *AlertingToolProvider) Tools(ctx context.Context) ([]agentDomain.ToolSpec, error) {
	return []agentDomain.ToolSpec{
		{Name: "alerting_list_active", Description: "List all currently active alerts", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
		{Name: "alerting_list_history", Description: "List alert history records", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
		{Name: "alerting_stats", Description: "Get alert statistics", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
		{Name: "alerting_noisy", Description: "Get noisy alerts", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
		{Name: "alerting_high_risk", Description: "Get high-risk alerts", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
		{Name: "alerting_get_feedback", Description: "Get feedback for a specific alert", RequiredPermission: "alerting:read", IsReadOnly: true, Category: agentDomain.CategoryAlerting},
	}, nil
}

func (p *AlertingToolProvider) DefaultPools() []*agentDomain.ToolPool {
	return []*agentDomain.ToolPool{
		{
			ID: "alert", Name: "告警分析",
			Description: "Alert querying, noise analysis, and feedback review",
			Categories:  []agentDomain.ToolCategory{agentDomain.CategoryAlerting},
			ToolNames:   []string{"alerting_list_active", "alerting_list_history", "alerting_stats", "alerting_noisy", "alerting_high_risk", "alerting_get_feedback"},
			Keywords:    []string{"告警", "报警", "alert", "alarm", "noise", "降噪", "误报"},
			Priority:    10, MaxTools: 8, IsBuiltin: true,
		},
	}
}

func (p *AlertingToolProvider) CreateTools() []ReadOnlyTool {
	fat := NewAlertingTool(p.alertService)
	return []ReadOnlyTool{
		NewAlertingListActiveTool(fat),
		NewAlertingListHistoryTool(fat),
		NewAlertingStatsTool(fat),
		NewAlertingNoisyTool(fat),
		NewAlertingHighRiskTool(fat),
		NewAlertingGetFeedbackTool(fat),
	}
}
