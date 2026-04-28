package eino

import (
	"context"
	"encoding/json"
	"fmt"

	agentDomain "cloud-agent-monitor/internal/agent/domain"
	"cloud-agent-monitor/internal/topology/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// TopologyServiceInterface defines the topology operations required by the fat tool.
type TopologyServiceInterface interface {
	GetServiceTopology(ctx context.Context, query domain.TopologyQuery) (*domain.ServiceTopology, error)
	GetNetworkTopology(ctx context.Context, query domain.TopologyQuery) (*domain.NetworkTopology, error)
	GetServiceNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error)
	GetServiceNodeByName(ctx context.Context, namespace, name string) (*domain.ServiceNode, error)
	GetUpstreamServices(ctx context.Context, id uuid.UUID, depth int) ([]*domain.ServiceNode, error)
	GetDownstreamServices(ctx context.Context, id uuid.UUID, depth int) ([]*domain.ServiceNode, error)
	AnalyzeImpact(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.ImpactResult, error)
	FindPath(ctx context.Context, sourceID, targetID uuid.UUID, maxHops int) (*domain.PathResult, error)
	FindShortestPath(ctx context.Context, sourceID, targetID uuid.UUID) ([]domain.PathHop, error)
	FindAnomalies(ctx context.Context) ([]*domain.TopologyAnomaly, error)
	GetTopologyStats(ctx context.Context) (*domain.TopologyStats, error)
}

// TopologyTool is the fat tool for topology queries. It dispatches by action to the TopologyServiceInterface.
// Supported actions: get_topology, get_network_topology, get_node, get_upstream, get_downstream,
// analyze_impact, find_path, find_shortest_path, find_anomalies, get_stats.
type TopologyTool struct {
	topologyService TopologyServiceInterface
}

// NewTopologyTool creates a new TopologyTool fat tool backed by the given service.
func NewTopologyTool(service TopologyServiceInterface) *TopologyTool {
	return &TopologyTool{topologyService: service}
}

func (t *TopologyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_query",
		Desc: `Query service topology information including service nodes, dependencies, impact analysis, and anomalies.

Actions:
- get_topology: Get current service topology (optionally filtered by namespace)
- get_network_topology: Get current network topology
- get_node: Get service node by ID or name
- get_upstream: Get upstream dependencies of a service
- get_downstream: Get downstream dependents of a service
- analyze_impact: Analyze impact scope if a service fails
- find_path: Find all call paths between two services
- find_shortest_path: Find the shortest call path between two services
- find_anomalies: Detect anomalies in the topology
- get_stats: Get topology statistics

All operations are READ-ONLY.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type: schema.String,
				Desc: "Action to perform",
				Enum: []string{
					"get_topology", "get_network_topology", "get_node",
					"get_upstream", "get_downstream", "analyze_impact",
					"find_path", "find_shortest_path", "find_anomalies", "get_stats",
				},
			},
			"service_id":   {Type: schema.String, Desc: "Service node UUID"},
			"namespace":    {Type: schema.String, Desc: "Kubernetes namespace"},
			"service_name": {Type: schema.String, Desc: "Service name (requires namespace)"},
			"source_id":    {Type: schema.String, Desc: "Source service UUID"},
			"target_id":    {Type: schema.String, Desc: "Target service UUID"},
			"depth":        {Type: schema.Integer, Desc: "Traversal depth (default: 3, max: 10)"},
			"max_depth":    {Type: schema.Integer, Desc: "Max depth for impact analysis (default: 5, max: 20)"},
			"max_hops":     {Type: schema.Integer, Desc: "Max hops for path finding (default: 10, max: 30)"},
		}),
	}, nil
}

type TopologyToolArgs struct {
	Action      string `json:"action"`
	ServiceID   string `json:"service_id,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
	Depth       int    `json:"depth,omitempty"`
	MaxDepth    int    `json:"max_depth,omitempty"`
	MaxHops     int    `json:"max_hops,omitempty"`
}

func (t *TopologyTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args TopologyToolArgs
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	var result any
	var err error

	switch args.Action {
	case "get_topology":
		result, err = t.getTopology(ctx, args)
	case "get_network_topology":
		result, err = t.getNetworkTopology(ctx, args)
	case "get_node":
		result, err = t.getNode(ctx, args)
	case "get_upstream":
		result, err = t.getUpstream(ctx, args)
	case "get_downstream":
		result, err = t.getDownstream(ctx, args)
	case "analyze_impact":
		result, err = t.analyzeImpact(ctx, args)
	case "find_path":
		result, err = t.findPath(ctx, args)
	case "find_shortest_path":
		result, err = t.findShortestPath(ctx, args)
	case "find_anomalies":
		result, err = t.findAnomalies(ctx)
	case "get_stats":
		result, err = t.getStats(ctx)
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

func (t *TopologyTool) getTopology(ctx context.Context, args TopologyToolArgs) (any, error) {
	query := domain.TopologyQuery{Namespace: args.Namespace}
	topo, err := t.topologyService.GetServiceTopology(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get service topology: %w", err)
	}
	return topo, nil
}

func (t *TopologyTool) getNetworkTopology(ctx context.Context, args TopologyToolArgs) (any, error) {
	query := domain.TopologyQuery{Namespace: args.Namespace}
	topo, err := t.topologyService.GetNetworkTopology(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get network topology: %w", err)
	}
	return topo, nil
}

func (t *TopologyTool) getNode(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.ServiceID != "" {
		id, err := uuid.Parse(args.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("invalid service_id: %w", err)
		}
		node, err := t.topologyService.GetServiceNode(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get service node: %w", err)
		}
		return node, nil
	}
	if args.Namespace != "" && args.ServiceName != "" {
		node, err := t.topologyService.GetServiceNodeByName(ctx, args.Namespace, args.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get service node by name: %w", err)
		}
		return node, nil
	}
	return nil, fmt.Errorf("service_id or namespace+service_name is required")
}

func (t *TopologyTool) getUpstream(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	id, err := uuid.Parse(args.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("invalid service_id: %w", err)
	}
	depth := args.Depth
	if depth <= 0 {
		depth = 3
	}
	nodes, err := t.topologyService.GetUpstreamServices(ctx, id, depth)
	if err != nil {
		return nil, fmt.Errorf("failed to get upstream services: %w", err)
	}
	return nodes, nil
}

func (t *TopologyTool) getDownstream(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	id, err := uuid.Parse(args.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("invalid service_id: %w", err)
	}
	depth := args.Depth
	if depth <= 0 {
		depth = 3
	}
	nodes, err := t.topologyService.GetDownstreamServices(ctx, id, depth)
	if err != nil {
		return nil, fmt.Errorf("failed to get downstream services: %w", err)
	}
	return nodes, nil
}

func (t *TopologyTool) analyzeImpact(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	id, err := uuid.Parse(args.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("invalid service_id: %w", err)
	}
	maxDepth := args.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5
	}
	result, err := t.topologyService.AnalyzeImpact(ctx, id, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze impact: %w", err)
	}
	return result, nil
}

func (t *TopologyTool) findPath(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.SourceID == "" || args.TargetID == "" {
		return nil, fmt.Errorf("source_id and target_id are required")
	}
	sourceID, err := uuid.Parse(args.SourceID)
	if err != nil {
		return nil, fmt.Errorf("invalid source_id: %w", err)
	}
	targetID, err := uuid.Parse(args.TargetID)
	if err != nil {
		return nil, fmt.Errorf("invalid target_id: %w", err)
	}
	maxHops := args.MaxHops
	if maxHops <= 0 {
		maxHops = 10
	}
	result, err := t.topologyService.FindPath(ctx, sourceID, targetID, maxHops)
	if err != nil {
		return nil, fmt.Errorf("failed to find path: %w", err)
	}
	return result, nil
}

func (t *TopologyTool) findShortestPath(ctx context.Context, args TopologyToolArgs) (any, error) {
	if args.SourceID == "" || args.TargetID == "" {
		return nil, fmt.Errorf("source_id and target_id are required")
	}
	sourceID, err := uuid.Parse(args.SourceID)
	if err != nil {
		return nil, fmt.Errorf("invalid source_id: %w", err)
	}
	targetID, err := uuid.Parse(args.TargetID)
	if err != nil {
		return nil, fmt.Errorf("invalid target_id: %w", err)
	}
	hops, err := t.topologyService.FindShortestPath(ctx, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to find shortest path: %w", err)
	}
	return hops, nil
}

func (t *TopologyTool) findAnomalies(ctx context.Context) (any, error) {
	anomalies, err := t.topologyService.FindAnomalies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find anomalies: %w", err)
	}
	return anomalies, nil
}

func (t *TopologyTool) getStats(ctx context.Context) (any, error) {
	stats, err := t.topologyService.GetTopologyStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get topology stats: %w", err)
	}
	return stats, nil
}

func (t *TopologyTool) IsReadOnly() bool {
	return true
}

func (t *TopologyTool) RequiredPermission() string {
	return "service:read"
}

type TopologyGetServiceTopologyTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetServiceTopologyTool(fat *TopologyTool) *TopologyGetServiceTopologyTool {
	return &TopologyGetServiceTopologyTool{delegate: NewFatToolDelegator(fat, "get_topology")}
}
func (t *TopologyGetServiceTopologyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_get_service_topology",
		Desc: "Get current service topology map — all service nodes and their dependency edges.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"namespace": {Type: schema.String, Desc: "Filter by Kubernetes namespace"},
		}),
	}, nil
}
func (t *TopologyGetServiceTopologyTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetServiceTopologyTool) IsReadOnly() bool           { return true }
func (t *TopologyGetServiceTopologyTool) RequiredPermission() string { return "service:read" }

type TopologyGetNetworkTopologyTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetNetworkTopologyTool(fat *TopologyTool) *TopologyGetNetworkTopologyTool {
	return &TopologyGetNetworkTopologyTool{delegate: NewFatToolDelegator(fat, "get_network_topology")}
}
func (t *TopologyGetNetworkTopologyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_get_network_topology",
		Desc: "Get current network topology — network-layer nodes and connections.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"namespace": {Type: schema.String, Desc: "Filter by Kubernetes namespace"},
		}),
	}, nil
}
func (t *TopologyGetNetworkTopologyTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetNetworkTopologyTool) IsReadOnly() bool           { return true }
func (t *TopologyGetNetworkTopologyTool) RequiredPermission() string { return "service:read" }

type TopologyGetNodeTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetNodeTool(fat *TopologyTool) *TopologyGetNodeTool {
	return &TopologyGetNodeTool{delegate: NewFatToolDelegator(fat, "get_node")}
}
func (t *TopologyGetNodeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_get_node",
		Desc: "Get a service node by ID or by namespace+name, including health metrics.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"service_id":   {Type: schema.String, Desc: "Service node UUID"},
			"namespace":    {Type: schema.String, Desc: "Kubernetes namespace (used with service_name)"},
			"service_name": {Type: schema.String, Desc: "Service name (requires namespace)"},
		}),
	}, nil
}
func (t *TopologyGetNodeTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetNodeTool) IsReadOnly() bool           { return true }
func (t *TopologyGetNodeTool) RequiredPermission() string { return "service:read" }

type TopologyGetUpstreamTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetUpstreamTool(fat *TopologyTool) *TopologyGetUpstreamTool {
	return &TopologyGetUpstreamTool{delegate: NewFatToolDelegator(fat, "get_upstream")}
}
func (t *TopologyGetUpstreamTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_get_upstream",
		Desc: "Get upstream dependencies of a service (services that call this service).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"service_id": {Type: schema.String, Desc: "Service node UUID (required)"},
			"depth":      {Type: schema.Integer, Desc: "Traversal depth (default: 3, max: 10)"},
		}),
	}, nil
}
func (t *TopologyGetUpstreamTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetUpstreamTool) IsReadOnly() bool           { return true }
func (t *TopologyGetUpstreamTool) RequiredPermission() string { return "service:read" }

type TopologyGetDownstreamTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetDownstreamTool(fat *TopologyTool) *TopologyGetDownstreamTool {
	return &TopologyGetDownstreamTool{delegate: NewFatToolDelegator(fat, "get_downstream")}
}
func (t *TopologyGetDownstreamTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_get_downstream",
		Desc: "Get downstream dependents of a service (services that this service calls).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"service_id": {Type: schema.String, Desc: "Service node UUID (required)"},
			"depth":      {Type: schema.Integer, Desc: "Traversal depth (default: 3, max: 10)"},
		}),
	}, nil
}
func (t *TopologyGetDownstreamTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetDownstreamTool) IsReadOnly() bool           { return true }
func (t *TopologyGetDownstreamTool) RequiredPermission() string { return "service:read" }

type TopologyAnalyzeImpactTool struct {
	delegate *FatToolDelegator
}

func NewTopologyAnalyzeImpactTool(fat *TopologyTool) *TopologyAnalyzeImpactTool {
	return &TopologyAnalyzeImpactTool{delegate: NewFatToolDelegator(fat, "analyze_impact")}
}
func (t *TopologyAnalyzeImpactTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_analyze_impact",
		Desc: "Analyze impact scope if a service fails — upstream and downstream blast radius.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"service_id": {Type: schema.String, Desc: "Service node UUID (required)"},
			"max_depth":  {Type: schema.Integer, Desc: "Maximum analysis depth (default: 5, max: 20)"},
		}),
	}, nil
}
func (t *TopologyAnalyzeImpactTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyAnalyzeImpactTool) IsReadOnly() bool           { return true }
func (t *TopologyAnalyzeImpactTool) RequiredPermission() string { return "service:read" }

type TopologyFindPathTool struct {
	delegate *FatToolDelegator
}

func NewTopologyFindPathTool(fat *TopologyTool) *TopologyFindPathTool {
	return &TopologyFindPathTool{delegate: NewFatToolDelegator(fat, "find_path")}
}
func (t *TopologyFindPathTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_find_path",
		Desc: "Find all call paths between two services.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"source_id": {Type: schema.String, Desc: "Source service UUID (required)"},
			"target_id": {Type: schema.String, Desc: "Target service UUID (required)"},
			"max_hops":  {Type: schema.Integer, Desc: "Maximum hops (default: 10, max: 30)"},
		}),
	}, nil
}
func (t *TopologyFindPathTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyFindPathTool) IsReadOnly() bool           { return true }
func (t *TopologyFindPathTool) RequiredPermission() string { return "service:read" }

type TopologyFindShortestPathTool struct {
	delegate *FatToolDelegator
}

func NewTopologyFindShortestPathTool(fat *TopologyTool) *TopologyFindShortestPathTool {
	return &TopologyFindShortestPathTool{delegate: NewFatToolDelegator(fat, "find_shortest_path")}
}
func (t *TopologyFindShortestPathTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "topology_find_shortest_path",
		Desc: "Find the shortest call path between two services.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"source_id": {Type: schema.String, Desc: "Source service UUID (required)"},
			"target_id": {Type: schema.String, Desc: "Target service UUID (required)"},
		}),
	}, nil
}
func (t *TopologyFindShortestPathTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyFindShortestPathTool) IsReadOnly() bool           { return true }
func (t *TopologyFindShortestPathTool) RequiredPermission() string { return "service:read" }

type TopologyFindAnomaliesTool struct {
	delegate *FatToolDelegator
}

func NewTopologyFindAnomaliesTool(fat *TopologyTool) *TopologyFindAnomaliesTool {
	return &TopologyFindAnomaliesTool{delegate: NewFatToolDelegator(fat, "find_anomalies")}
}
func (t *TopologyFindAnomaliesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "topology_find_anomalies",
		Desc:        "Detect anomalies in the topology — unhealthy services, high error rates, high latency.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}
func (t *TopologyFindAnomaliesTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyFindAnomaliesTool) IsReadOnly() bool           { return true }
func (t *TopologyFindAnomaliesTool) RequiredPermission() string { return "service:read" }

type TopologyGetStatsTool struct {
	delegate *FatToolDelegator
}

func NewTopologyGetStatsTool(fat *TopologyTool) *TopologyGetStatsTool {
	return &TopologyGetStatsTool{delegate: NewFatToolDelegator(fat, "get_stats")}
}
func (t *TopologyGetStatsTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "topology_get_stats",
		Desc:        "Get topology statistics — node counts, edge counts, health distribution.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}
func (t *TopologyGetStatsTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	return t.delegate.Delegate(ctx, argsJSON, opts...)
}
func (t *TopologyGetStatsTool) IsReadOnly() bool           { return true }
func (t *TopologyGetStatsTool) RequiredPermission() string { return "service:read" }

// TopologyToolProvider implements domain.ToolProvider for the topology category.
// It creates 10 thin tools that delegate to a single TopologyTool fat tool.
type TopologyToolProvider struct {
	topologyService TopologyServiceInterface
}

// NewTopologyToolProvider creates a new topology provider backed by the given service.
func NewTopologyToolProvider(svc TopologyServiceInterface) *TopologyToolProvider {
	return &TopologyToolProvider{topologyService: svc}
}

func (p *TopologyToolProvider) Category() agentDomain.ToolCategory {
	return agentDomain.CategoryTopology
}

func (p *TopologyToolProvider) Tools(ctx context.Context) ([]agentDomain.ToolSpec, error) {
	return []agentDomain.ToolSpec{
		{Name: "topology_get_service_topology", Description: "Get service topology map", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_get_network_topology", Description: "Get network topology", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_get_node", Description: "Get service node details", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_get_upstream", Description: "Get upstream dependencies", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_get_downstream", Description: "Get downstream dependents", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_analyze_impact", Description: "Analyze impact scope of service failure", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_find_path", Description: "Find all paths between services", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_find_shortest_path", Description: "Find shortest path between services", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_find_anomalies", Description: "Detect topology anomalies", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
		{Name: "topology_get_stats", Description: "Get topology statistics", RequiredPermission: "service:read", IsReadOnly: true, Category: agentDomain.CategoryTopology},
	}, nil
}

func (p *TopologyToolProvider) DefaultPools() []*agentDomain.ToolPool {
	return []*agentDomain.ToolPool{
		{
			ID: "topology", Name: "拓扑查询",
			Description: "Service topology querying, dependency analysis, impact analysis, and path finding",
			Categories:  []agentDomain.ToolCategory{agentDomain.CategoryTopology},
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
			Keywords: []string{"拓扑", "依赖", "调用链", "topology", "dependency", "upstream", "downstream", "path", "impact"},
			Priority: 8, MaxTools: 10, IsBuiltin: true,
		},
	}
}

func (p *TopologyToolProvider) CreateTools() []ReadOnlyTool {
	fat := NewTopologyTool(p.topologyService)
	return []ReadOnlyTool{
		NewTopologyGetServiceTopologyTool(fat),
		NewTopologyGetNetworkTopologyTool(fat),
		NewTopologyGetNodeTool(fat),
		NewTopologyGetUpstreamTool(fat),
		NewTopologyGetDownstreamTool(fat),
		NewTopologyAnalyzeImpactTool(fat),
		NewTopologyFindPathTool(fat),
		NewTopologyFindShortestPathTool(fat),
		NewTopologyFindAnomaliesTool(fat),
		NewTopologyGetStatsTool(fat),
	}
}

type WeightedTopologyTool struct {
	*TopologyTool
}

func NewWeightedTopologyTool(service TopologyServiceInterface) *WeightedTopologyTool {
	return &WeightedTopologyTool{
		TopologyTool: NewTopologyTool(service),
	}
}

func (t *WeightedTopologyTool) findShortestPathWeighted(ctx context.Context, args TopologyToolArgs) (any, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *WeightedTopologyTool) analyzeImpactWeighted(ctx context.Context, args TopologyToolArgs) (any, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *WeightedTopologyTool) detectBottlenecks(ctx context.Context) (any, error) {
	return nil, fmt.Errorf("not implemented")
}
