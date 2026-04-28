package eino

import (
	"cloud-agent-monitor/internal/agent/domain"
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// NewToolsNodeWithAuth creates an eino ToolsNode containing all registered tools.
// Every tool is wrapped with AuthzToolWrapper, so authentication and permission
// checks are enforced automatically during execution.
func NewToolsNodeWithAuth(ctx context.Context, registry *ToolRegistry) (*compose.ToolsNode, error) {
	tools := make([]tool.BaseTool, 0)
	for _, t := range registry.GetTools() {
		tools = append(tools, t)
	}

	config := &compose.ToolsNodeConfig{
		Tools: tools,
	}

	return compose.NewToolNode(ctx, config)
}

// NewToolsNodeFromPool creates an eino ToolsNode by selecting tools from a pool
// based on the given intent. This is the primary entry point for building agent
// graphs that dynamically select tools per request.
func NewToolsNodeFromPool(ctx context.Context, poolRegistry *PoolRegistry, intent string) (*compose.ToolsNode, error) {
	selected, err := poolRegistry.SelectTools(ctx, intent)
	if err != nil {
		return nil, fmt.Errorf("pool selection failed for intent %q: %w", intent, err)
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no tools available for intent %q", intent)
	}

	tools := make([]tool.BaseTool, 0, len(selected))
	for _, t := range selected {
		tools = append(tools, t)
	}

	config := &compose.ToolsNodeConfig{
		Tools: tools,
	}

	return compose.NewToolNode(ctx, config)
}

// SetupToolRegistry creates a new ToolRegistry with the given permission checker.
func SetupToolRegistry(authz PermissionChecker) *ToolRegistry {
	return NewToolRegistry(authz)
}

// SetupPoolRegistry creates a PoolRegistry pre-loaded with all builtin pools
// and registers the given providers' tools into the tool registry.
// BuiltinPools already define all pool configurations, so provider DefaultPools
// are not re-registered to avoid duplicates.
func SetupPoolRegistry(toolRegistry *ToolRegistry, alertProvider *AlertingToolProvider, sloProvider *SLOToolProvider, topoProvider *TopologyToolProvider) *PoolRegistry {
	budget := DefaultToolBudget()
	pr := NewPoolRegistry(toolRegistry, budget)

	for _, pool := range BuiltinPools() {
		_ = pr.RegisterPool(pool)
	}

	registerProviderWithToolsOnly(pr, alertProvider, alertProvider.CreateTools())
	registerProviderWithToolsOnly(pr, sloProvider, sloProvider.CreateTools())
	registerProviderWithToolsOnly(pr, topoProvider, topoProvider.CreateTools())

	return pr
}

func registerProviderWithToolsOnly(pr *PoolRegistry, provider domain.ToolProvider, tools []ReadOnlyTool) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.providers[provider.Category()] = provider

	for _, t := range tools {
		pr.registry.Register(t)
	}
}

// DefaultToolBudget returns the default tool budget: 10 tools per request, 8000 tokens.
func DefaultToolBudget() *domain.ToolBudget {
	return &domain.ToolBudget{
		MaxToolsPerRequest: 10,
		MaxTokensForTools:  8000,
	}
}
