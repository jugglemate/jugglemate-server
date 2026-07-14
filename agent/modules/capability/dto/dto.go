// Package dto 定义 Capability HTTP 数据契约。
package dto

// ToolWrite 创建 Tool 请求。
type ToolWrite struct {
	Name           string         `json:"name" binding:"required,max=128"`
	Description    *string        `json:"description,omitempty"`
	Visibility     string         `json:"visibility"`
	APIEndpoint    string         `json:"apiEndpoint" binding:"required,max=512"`
	HTTPMethod     string         `json:"httpMethod"`
	InputSchema    map[string]any `json:"inputSchema,omitempty"`
	OutputSchema   map[string]any `json:"outputSchema,omitempty"`
	TimeoutSeconds int            `json:"timeoutSeconds"`
}

// ToolUpdate 更新 Tool 请求。
type ToolUpdate struct {
	Name           *string        `json:"name,omitempty"`
	Description    *string        `json:"description,omitempty"`
	Visibility     *string        `json:"visibility,omitempty"`
	APIEndpoint    *string        `json:"apiEndpoint,omitempty"`
	HTTPMethod     *string        `json:"httpMethod,omitempty"`
	InputSchema    map[string]any `json:"inputSchema,omitempty"`
	OutputSchema   map[string]any `json:"outputSchema,omitempty"`
	TimeoutSeconds *int           `json:"timeoutSeconds,omitempty"`
}

// ToolRead Tool 详情响应。
type ToolRead struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      *string        `json:"description"`
	Visibility       string         `json:"visibility"`
	Status           string         `json:"status"`
	APIEndpoint      string         `json:"apiEndpoint"`
	HTTPMethod       string         `json:"httpMethod"`
	InputSchema      map[string]any `json:"inputSchema"`
	OutputSchema     map[string]any `json:"outputSchema"`
	RiskLevel        string         `json:"riskLevel"`
	RequiresApproval bool           `json:"requiresApproval"`
	TimeoutSeconds   int64          `json:"timeoutSeconds"`
	CallCount        int64          `json:"callCount"`
	SuccessRate      float64        `json:"successRate"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

// SkillWrite 创建 Skill 请求。
type SkillWrite struct {
	Name         string         `json:"name" binding:"required,max=128"`
	Description  *string        `json:"description,omitempty"`
	Visibility   string         `json:"visibility"`
	Category     *string        `json:"category,omitempty"`
	ToolIDs      []string       `json:"toolIds"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

// SkillUpdate 更新 Skill 请求。
type SkillUpdate struct {
	Name         *string        `json:"name,omitempty"`
	Description  *string        `json:"description,omitempty"`
	Visibility   *string        `json:"visibility,omitempty"`
	Category     *string        `json:"category,omitempty"`
	ToolIDs      *[]string      `json:"toolIds,omitempty"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

// SkillRead Skill 详情响应。
type SkillRead struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      *string        `json:"description"`
	Visibility       string         `json:"visibility"`
	Status           string         `json:"status"`
	Category         *string        `json:"category"`
	ToolIDs          []string       `json:"toolIds"`
	InputSchema      map[string]any `json:"inputSchema"`
	OutputSchema     map[string]any `json:"outputSchema"`
	RiskLevel        string         `json:"riskLevel"`
	RequiresApproval bool           `json:"requiresApproval"`
	CallCount        int64          `json:"callCount"`
	SuccessRate      float64        `json:"successRate"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

// TestRequest Tool/Skill 测试请求。
type TestRequest struct {
	Payload map[string]any `json:"payload"`
}

// TestResponse Tool/Skill 测试响应。
type TestResponse struct {
	Success      bool           `json:"success"`
	Output       map[string]any `json:"output"`
	ErrorCode    *string        `json:"errorCode"`
	ErrorMessage *string        `json:"errorMessage"`
	LatencyMS    int            `json:"latencyMs"`
	FailedToolID *string        `json:"failedToolId,omitempty"`
}

// DiscoveryItem 公共能力混合列表项。
type DiscoveryItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	Visibility string  `json:"visibility"`
	RiskLevel  string  `json:"riskLevel"`
	Category   *string `json:"category"`
	CallCount  int64   `json:"callCount"`
	UpdatedAt  string  `json:"updatedAt"`
}
