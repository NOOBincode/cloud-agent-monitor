package eino

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

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

func SetupToolRegistry(authz PermissionChecker) *ToolRegistry {
	return NewToolRegistry(authz)
}
