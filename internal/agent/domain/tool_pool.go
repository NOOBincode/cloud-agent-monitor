package domain

import "context"

type ToolCategory string

const (
	CategoryTopology  ToolCategory = "topology"
	CategoryAlerting  ToolCategory = "alerting"
	CategorySLO       ToolCategory = "slo"
	CategoryGPU       ToolCategory = "gpu"
	CategoryInference ToolCategory = "inference"
	CategoryCost      ToolCategory = "cost"
	CategoryQueue     ToolCategory = "queue"
	CategoryAudit     ToolCategory = "audit"
)

type ToolPool struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Categories  []ToolCategory `json:"categories"`
	ToolNames   []string      `json:"tool_names"`
	Keywords    []string      `json:"keywords"`
	Priority    int           `json:"priority"`
	MaxTools    int           `json:"max_tools"`
	IsBuiltin   bool          `json:"is_builtin"`
}

type ToolBudget struct {
	MaxToolsPerRequest int `json:"max_tools_per_request"`
	MaxTokensForTools  int `json:"max_tokens_for_tools"`
}

type ToolProvider interface {
	Category() ToolCategory
	Tools(ctx context.Context) ([]ToolSpec, error)
	DefaultPools() []*ToolPool
}

type ToolSpec struct {
	Name               string
	Description        string
	RequiredPermission string
	IsReadOnly         bool
	Category           ToolCategory
}
