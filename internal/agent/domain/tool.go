package domain

type ToolMetadata struct {
	Name               string        `json:"name"`
	Description        string        `json:"description"`
	Category           ToolCategory  `json:"category"`
	RequiredPermission string        `json:"required_permission"`
	IsReadOnly         bool          `json:"is_readonly"`
	PoolIDs            []string      `json:"pool_ids"`
}

type ToolPermission struct {
	ToolName string `json:"tool_name"`
	Role     string `json:"role"`
	Allowed  bool   `json:"allowed"`
}

type ToolPermissionOverride struct {
	ToolName string `json:"tool_name"`
	Role     string `json:"role"`
	Allowed  bool   `json:"allowed"`
}
