package eino

import (
	"cloud-agent-monitor/internal/agent/domain"
)

// BuiltinPools returns the four active tool pools for alerting, topology, SLO,
// and general queries. These pools are always available and cannot be unregistered.
func BuiltinPools() []*domain.ToolPool {
	return []*domain.ToolPool{
		{
			ID:          "alerting",
			Name:        "告警查询",
			Description: "Alert querying, history search, alert statistics, noisy alert detection, and high-risk alert identification",
			Categories:  []domain.ToolCategory{domain.CategoryAlerting},
			ToolNames: []string{
				"alerting_list_active",
				"alerting_list_history",
				"alerting_stats",
				"alerting_noisy",
				"alerting_high_risk",
				"alerting_feedback",
			},
			Keywords:  []string{"告警", "报警", "alert", "alarm", "firing", "resolved", "severity", "critical", "warning"},
			Priority:  9,
			MaxTools:  6,
			IsBuiltin: true,
		},
		{
			ID:          "slo",
			Name:        "SLO查询",
			Description: "SLO status querying, error budget analysis, burn rate alerts, and SLO summary statistics",
			Categories:  []domain.ToolCategory{domain.CategorySLO},
			ToolNames: []string{
				"slo_list",
				"slo_get",
				"slo_get_error_budget",
				"slo_get_burn_rate_alerts",
				"slo_get_summary",
			},
			Keywords:  []string{"SLO", "SLI", "错误预算", "error budget", "burn rate", "燃烧率", "目标", "objective"},
			Priority:  7,
			MaxTools:  5,
			IsBuiltin: true,
		},
		{
			ID:          "topology",
			Name:        "拓扑查询",
			Description: "Service topology querying, dependency analysis, impact analysis, and path finding",
			Categories:  []domain.ToolCategory{domain.CategoryTopology},
			ToolNames: []string{
				"topology_get_service_topology",
				"topology_get_network_topology",
				"topology_get_node",
				"topology_get_upstream",
				"topology_get_downstream",
				"topology_analyze_impact",
				"topology_find_path",
				"topology_find_shortest_path",
				"topology_find_anomalies",
				"topology_get_stats",
			},
			Keywords:  []string{"拓扑", "依赖", "调用链", "topology", "dependency", "upstream", "downstream", "path", "impact"},
			Priority:  8,
			MaxTools:  8,
			IsBuiltin: true,
		},
		{
			ID:          "general",
			Name:        "综合查询",
			Description: "General observability querying across alerts, SLO, and topology — fallback pool for broad questions",
			Categories:  []domain.ToolCategory{domain.CategoryAlerting, domain.CategorySLO, domain.CategoryTopology},
			ToolNames: []string{
				"alerting_list_active",
				"alerting_stats",
				"slo_list",
				"slo_get_summary",
				"topology_get_service_topology",
				"topology_get_stats",
				"topology_find_anomalies",
			},
			Keywords:  []string{"概览", "总览", "状态", "overview", "status", "health", "dashboard", "综合", "general"},
			Priority:  1,
			MaxTools:  7,
			IsBuiltin: true,
		},
	}
}

// ReservedAIPools returns three reserved pools for future AI infrastructure
// observability: GPU monitoring, inference service metrics, and cost analysis.
// These pools have no tools yet and are placeholders for the AI infra extension.
func ReservedAIPools() []*domain.ToolPool {
	return []*domain.ToolPool{
		{
			ID:          "gpu",
			Name:        "GPU监控",
			Description: "GPU utilization, memory, temperature, and health monitoring for AI infrastructure",
			Categories:  []domain.ToolCategory{domain.CategoryGPU},
			ToolNames:   []string{},
			Keywords:    []string{"GPU", "显存", "CUDA", "显卡", "GPU利用率", "GPU温度"},
			Priority:    6,
			MaxTools:    6,
			IsBuiltin:   true,
		},
		{
			ID:          "inference",
			Name:        "推理服务",
			Description: "Model inference service monitoring — latency, throughput, error rates, and model performance",
			Categories:  []domain.ToolCategory{domain.CategoryInference},
			ToolNames:   []string{},
			Keywords:    []string{"推理", "inference", "模型", "model", "LLM", "推理延迟", "吞吐量", "throughput"},
			Priority:    6,
			MaxTools:    6,
			IsBuiltin:   true,
		},
		{
			ID:          "cost",
			Name:        "成本分析",
			Description: "AI infrastructure cost analysis — GPU cost, API token consumption, resource optimization",
			Categories:  []domain.ToolCategory{domain.CategoryCost},
			ToolNames:   []string{},
			Keywords:    []string{"成本", "cost", "费用", "预算", "token", "消耗", "优化", "GPU成本"},
			Priority:    5,
			MaxTools:    5,
			IsBuiltin:   true,
		},
	}
}
