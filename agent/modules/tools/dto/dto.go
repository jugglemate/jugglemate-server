// Package dto 定义 Tools v2 的 HTTP 请求与响应契约。
package dto

// ProviderCreateRequest 创建工具提供方请求。
type ProviderCreateRequest struct {
	Name             string         `json:"name" binding:"required,max=128"`
	ProviderType     string         `json:"providerType" binding:"required"`
	DisplayName      *string        `json:"displayName,omitempty"`
	ConnectionConfig map[string]any `json:"connectionConfig"`
	AuthType         string         `json:"authType" binding:"required"`
	AuthSchema       map[string]any `json:"authSchema,omitempty"`
	Visibility       string         `json:"visibility"`
}

// ProviderUpdateRequest 更新工具提供方请求。
type ProviderUpdateRequest struct {
	DisplayName      *string        `json:"displayName,omitempty"`
	ConnectionConfig map[string]any `json:"connectionConfig,omitempty"`
	AuthSchema       map[string]any `json:"authSchema,omitempty"`
	Visibility       *string        `json:"visibility,omitempty"`
}

// ProviderResponse 工具提供方详情。
type ProviderResponse struct {
	ID               string         `json:"id"`
	OwnerID          string         `json:"ownerId"`
	Name             string         `json:"name"`
	DisplayName      *string        `json:"displayName"`
	ProviderType     string         `json:"providerType"`
	AuthType         string         `json:"authType"`
	AuthSchema       map[string]any `json:"authSchema"`
	ConnectionConfig map[string]any `json:"connectionConfig"`
	Status           string         `json:"status"`
	Visibility       string         `json:"visibility"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

// ProviderListResponse 工具提供方分页结果。
type ProviderListResponse struct {
	Items    []ProviderResponse `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

// DefinitionCreateRequest 创建插件或自定义工具定义请求。
type DefinitionCreateRequest struct {
	ProviderID      string         `json:"providerId" binding:"required"`
	Name            string         `json:"name" binding:"required,max=128"`
	Description     *string        `json:"description,omitempty"`
	OperationID     *string        `json:"operationId,omitempty"`
	InputSchema     map[string]any `json:"inputSchema" binding:"required"`
	OutputSchema    map[string]any `json:"outputSchema" binding:"required"`
	ExecutionConfig map[string]any `json:"executionConfig" binding:"required"`
	RiskLevel       *string        `json:"riskLevel,omitempty"`
	TimeoutSeconds  int            `json:"timeoutSeconds"`
}

// DefinitionUpdateRequest 更新工具定义请求。
type DefinitionUpdateRequest struct {
	Description      *string        `json:"description,omitempty"`
	InputSchema      map[string]any `json:"inputSchema,omitempty"`
	OutputSchema     map[string]any `json:"outputSchema,omitempty"`
	ExecutionConfig  map[string]any `json:"executionConfig,omitempty"`
	RiskLevel        *string        `json:"riskLevel,omitempty"`
	ApprovalRequired *bool          `json:"approvalRequired,omitempty"`
	TimeoutSeconds   *int           `json:"timeoutSeconds,omitempty"`
}

// DefinitionResponse 工具定义详情。
type DefinitionResponse struct {
	ID               string         `json:"id"`
	ProviderID       string         `json:"providerId"`
	OwnerID          string         `json:"ownerId"`
	Name             string         `json:"name"`
	Description      *string        `json:"description"`
	ToolType         string         `json:"toolType"`
	InputSchema      map[string]any `json:"inputSchema"`
	OutputSchema     map[string]any `json:"outputSchema"`
	ExecutionConfig  map[string]any `json:"executionConfig"`
	RiskLevel        string         `json:"riskLevel"`
	ApprovalRequired bool           `json:"approvalRequired"`
	TimeoutSeconds   int            `json:"timeoutSeconds"`
	Status           string         `json:"status"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
}

// DefinitionListResponse 工具定义分页结果。
type DefinitionListResponse struct {
	Items    []DefinitionResponse `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

// OpenAPIImportRequest 导入 OpenAPI 文档请求。
type OpenAPIImportRequest struct {
	OpenAPIURL  *string `json:"openapiUrl,omitempty"`
	OpenAPISpec any     `json:"openapiSpec,omitempty"`
}

// OpenAPIOperation 表示 OpenAPI 中可导入的单个操作。
type OpenAPIOperation struct {
	OperationID string         `json:"operationId"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Summary     *string        `json:"summary"`
	InputSchema map[string]any `json:"inputSchema"`
}

// OpenAPIImportResponse OpenAPI 解析结果。
type OpenAPIImportResponse struct {
	Operations []OpenAPIOperation `json:"operations"`
}

// DefinitionTestRequest 测试工具定义请求。
type DefinitionTestRequest struct {
	Payload      map[string]any `json:"payload"`
	CredentialID *string        `json:"credentialId,omitempty"`
}

// DefinitionTestResponse 工具定义测试结果。
type DefinitionTestResponse struct {
	Success       bool    `json:"success"`
	Status        string  `json:"status"`
	ErrorCode     *string `json:"errorCode"`
	ErrorMessage  *string `json:"errorMessage"`
	DurationMS    int     `json:"durationMs"`
	TraceID       string  `json:"traceId"`
	OutputSummary *string `json:"outputSummary"`
}

// CredentialCreateRequest 创建工具凭据请求。
type CredentialCreateRequest struct {
	ProviderID        string         `json:"providerId" binding:"required"`
	Name              string         `json:"name" binding:"required,max=128"`
	CredentialPayload map[string]any `json:"credentialPayload"`
}

// CredentialRotateRequest 轮转工具凭据请求。
type CredentialRotateRequest struct {
	CredentialPayload map[string]any `json:"credentialPayload" binding:"required"`
}

// CredentialResponse 工具凭据元数据；永不包含明文载荷。
type CredentialResponse struct {
	ID         string  `json:"id"`
	OwnerID    string  `json:"ownerId"`
	ProviderID string  `json:"providerId"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	RotatedAt  *string `json:"rotatedAt"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

// CredentialListResponse 工具凭据分页结果。
type CredentialListResponse struct {
	Items    []CredentialResponse `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

// BindingCreateRequest 挂载工具到 Agent 请求。
type BindingCreateRequest struct {
	ToolDefinitionID string         `json:"toolDefinitionId" binding:"required"`
	Enabled          *bool          `json:"enabled,omitempty"`
	RuntimeOverrides map[string]any `json:"runtimeOverrides,omitempty"`
	CredentialID     *string        `json:"credentialId,omitempty"`
}

// BindingUpdateRequest 更新工具挂载请求。
type BindingUpdateRequest struct {
	Enabled          *bool          `json:"enabled,omitempty"`
	RuntimeOverrides map[string]any `json:"runtimeOverrides,omitempty"`
	CredentialID     *string        `json:"credentialId,omitempty"`
}

// BindingResponse Agent 工具挂载详情。
type BindingResponse struct {
	ID                 string         `json:"id"`
	AgentID            string         `json:"agentId"`
	ToolDefinitionID   string         `json:"toolDefinitionId"`
	Enabled            bool           `json:"enabled"`
	RuntimeOverrides   map[string]any `json:"runtimeOverrides"`
	CredentialID       *string        `json:"credentialId"`
	PolicyGuardEnabled bool           `json:"policyGuardEnabled"`
	ApprovalRequired   bool           `json:"approvalRequired"`
	MountedAt          string         `json:"mountedAt"`
	UpdatedAt          string         `json:"updatedAt"`
}

// BindingListResponse Agent 工具挂载列表。
type BindingListResponse struct {
	Items []BindingResponse `json:"items"`
	Total int               `json:"total"`
}

// ExecuteRequest 内部统一工具执行请求。
type ExecuteRequest struct {
	TraceID          string         `json:"traceId" binding:"required"`
	TurnID           string         `json:"turnId" binding:"required"`
	AgentID          string         `json:"agentId" binding:"required"`
	OwnerID          string         `json:"ownerId" binding:"required"`
	ToolDefinitionID string         `json:"toolDefinitionId" binding:"required"`
	ConversationID   *string        `json:"conversationId,omitempty"`
	Input            map[string]any `json:"input"`
	IdempotencyKey   *string        `json:"idempotencyKey,omitempty"`
}

// ExecuteResponse 内部统一工具执行结果。
type ExecuteResponse struct {
	Status       string  `json:"status"`
	Output       any     `json:"output"`
	ErrorCode    *string `json:"errorCode"`
	ErrorMessage *string `json:"errorMessage"`
	DurationMS   int     `json:"durationMs"`
	TraceID      string  `json:"traceId"`
}

// OrchestrateRequest 单轮工具决策与执行请求。
type OrchestrateRequest struct {
	TraceID        string         `json:"traceId" binding:"required"`
	TurnID         string         `json:"turnId" binding:"required"`
	AgentID        string         `json:"agentId" binding:"required"`
	OwnerID        string         `json:"ownerId" binding:"required"`
	Intent         string         `json:"intent" binding:"required"`
	Payload        map[string]any `json:"payload"`
	ConversationID *string        `json:"conversationId,omitempty"`
	MaxCalls       *int           `json:"maxCalls,omitempty"`
	IdempotencyKey *string        `json:"idempotencyKey,omitempty"`
}

// DecisionCandidate 工具决策候选及评分明细。
type DecisionCandidate struct {
	ToolDefinitionID string   `json:"toolDefinitionId"`
	Name             string   `json:"name"`
	Score            float64  `json:"score"`
	IntentScore      float64  `json:"intentScore"`
	SchemaScore      float64  `json:"schemaScore"`
	SuccessRateScore float64  `json:"successRateScore"`
	Matched          []string `json:"matched"`
	Unmatched        []string `json:"unmatched"`
}

// DecisionOutcome 工具决策结果。
type DecisionOutcome struct {
	Action            string              `json:"action"`
	SelectedTools     []string            `json:"selectedTools"`
	Candidates        []DecisionCandidate `json:"candidates"`
	TerminationReason *string             `json:"terminationReason"`
}

// OrchestrateExecutionResponse 单轮编排中精简的执行结果。
type OrchestrateExecutionResponse struct {
	Status       string  `json:"status"`
	Output       any     `json:"output,omitempty"`
	ErrorCode    *string `json:"errorCode"`
	ErrorMessage *string `json:"errorMessage"`
	DurationMS   int     `json:"durationMs"`
	TraceID      string  `json:"traceId"`
}

// OrchestrateStep 单轮编排中的决策与执行步骤。
type OrchestrateStep struct {
	Decision  DecisionOutcome               `json:"decision"`
	Execution *OrchestrateExecutionResponse `json:"execution"`
}

// OrchestrateResponse 单轮工具编排结果。
type OrchestrateResponse struct {
	Terminated string            `json:"terminated"`
	Steps      []OrchestrateStep `json:"steps"`
}

// CallRecordResponse 工具调用事实详情。
type CallRecordResponse struct {
	ID               string   `json:"id"`
	TraceID          string   `json:"traceId"`
	ConversationID   *string  `json:"conversationId"`
	AgentID          string   `json:"agentId"`
	OwnerID          string   `json:"ownerId"`
	ToolDefinitionID string   `json:"toolDefinitionId"`
	Status           string   `json:"status"`
	InputSummary     *string  `json:"inputSummary"`
	OutputSummary    *string  `json:"outputSummary"`
	ErrorCode        *string  `json:"errorCode"`
	ErrorMessage     *string  `json:"errorMessage"`
	DurationMS       int      `json:"durationMs"`
	BilledCredit     *float64 `json:"billedCredit"`
	IdempotencyKey   *string  `json:"idempotencyKey"`
	CreatedAt        string   `json:"createdAt"`
}

// CallRecordListResponse 工具调用事实分页结果。
type CallRecordListResponse struct {
	Items    []CallRecordResponse `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}

// DecisionRecordResponse 工具决策记录详情。
type DecisionRecordResponse struct {
	ID                string           `json:"id"`
	TraceID           string           `json:"traceId"`
	ConversationID    *string          `json:"conversationId"`
	AgentID           string           `json:"agentId"`
	OwnerID           string           `json:"ownerId"`
	IntentSummary     string           `json:"intentSummary"`
	CandidateTools    []map[string]any `json:"candidateTools"`
	SelectedTools     []string         `json:"selectedTools"`
	DecisionAction    string           `json:"decisionAction"`
	TerminationReason *string          `json:"terminationReason"`
	CreatedAt         string           `json:"createdAt"`
}

// DecisionRecordListResponse 工具决策记录分页结果。
type DecisionRecordListResponse struct {
	Items    []DecisionRecordResponse `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}
