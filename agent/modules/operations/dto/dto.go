// Package dto 定义运营后台 17 个接口的数据契约。
package dto

import "time"

// PageResponse 表示通用分页响应。
type PageResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// AdminAgentItem 表示管理员 Agent 列表项。
type AdminAgentItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerId"`
	Status    string    `json:"status"`
	Usage     int64     `json:"usage"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ConsoleAgentItem 表示运营首屏 Agent 聚合项。
type ConsoleAgentItem struct {
	AgentID        string     `json:"agentId"`
	AgentName      string     `json:"agentName"`
	OwnerID        string     `json:"ownerId"`
	OwnerName      string     `json:"ownerName"`
	OwnerAvatar    *string    `json:"ownerAvatar"`
	PrimaryModel   string     `json:"primaryModel"`
	Status         string     `json:"status"`
	RiskFlag       string     `json:"riskFlag"`
	CreatedAt      time.Time  `json:"createdAt"`
	PublishedAt    *time.Time `json:"publishedAt"`
	KnowledgeCount int64      `json:"knowledgeCount"`
	ToolCount      int64      `json:"toolCount"`
}

// AdminAgentDetail 表示管理员 Agent 详情。
type AdminAgentDetail struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"ownerId"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	Status           string    `json:"status"`
	TotalInvocations int64     `json:"totalInvocations"`
	SuccessCount     int64     `json:"successCount"`
	FailCount        int64     `json:"failCount"`
	FailureRate      float64   `json:"failureRate"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// PauseAgentRequest 表示管理员暂停 Agent 请求。
type PauseAgentRequest struct {
	Reason      string  `json:"reason" binding:"required"`
	Description *string `json:"description,omitempty"`
	Version     int64   `json:"version" binding:"required,min=1"`
}

// ResumeAgentRequest 表示管理员恢复 Agent 请求。
type ResumeAgentRequest struct {
	Version int64 `json:"version" binding:"required,min=1"`
}

// AgentOperationResponse 表示 Agent 管理操作结果。
type AgentOperationResponse struct {
	AgentID          string     `json:"agentId"`
	Status           string     `json:"status"`
	PausedAt         *time.Time `json:"pausedAt"`
	ResumedAt        *time.Time `json:"resumedAt"`
	NotificationSent bool       `json:"notificationSent"`
	AuditID          string     `json:"auditId"`
}

// OwnerListItem 表示运营 Owner 列表项。
type OwnerListItem struct {
	ID         string     `json:"id"`
	OwnerID    string     `json:"ownerId"`
	AgentCount int64      `json:"agentCount"`
	Revenue    int64      `json:"revenue"`
	Status     string     `json:"status"`
	LastActive *time.Time `json:"lastActive"`
}

// OwnerDetail 表示 Owner 聚合详情。
type OwnerDetail struct {
	Owner         map[string]any   `json:"owner"`
	Agents        []map[string]any `json:"agents"`
	Knowledge     []map[string]any `json:"knowledge"`
	Revenue       map[string]any   `json:"revenue"`
	Subscriptions []map[string]any `json:"subscriptions"`
}

// ToolPricing 表示三档工具调用价格。
type ToolPricing struct {
	LowRisk    float64 `json:"lowRisk"`
	MediumRisk float64 `json:"mediumRisk"`
	HighRisk   float64 `json:"highRisk"`
}

// BillingRule 表示当前生效计费规则。
type BillingRule struct {
	Version           int         `json:"version"`
	Status            string      `json:"status"`
	TokenUnitPrice    float64     `json:"tokenUnitPrice"`
	ToolPricing       ToolPricing `json:"toolPricing"`
	MinRechargeAmount int         `json:"minRechargeAmount"`
	Precision         int         `json:"precision"`
	ValidityYears     int         `json:"validityYears"`
	UpdatedAt         time.Time   `json:"updatedAt"`
	UpdatedBy         *string     `json:"updatedBy"`
}

// BillingRuleUpdate 表示计费规则更新请求。
type BillingRuleUpdate struct {
	Version           int         `json:"version" binding:"required,min=1"`
	TokenUnitPrice    float64     `json:"tokenUnitPrice" binding:"required,gt=0"`
	ToolPricing       ToolPricing `json:"toolPricing"`
	MinRechargeAmount int         `json:"minRechargeAmount" binding:"required,gt=0"`
	Precision         int         `json:"precision" binding:"min=0,max=5"`
	ValidityYears     int         `json:"validityYears" binding:"required,min=1,max=100"`
	Reason            *string     `json:"reason,omitempty"`
}

// ToolPricingUpdate 表示独立工具计费更新请求。
type ToolPricingUpdate struct {
	Version     int         `json:"version" binding:"required,min=1"`
	ToolPricing ToolPricing `json:"toolPricing"`
	Reason      *string     `json:"reason,omitempty"`
}

// ModelCoefficient 表示当前模型计费系数。
type ModelCoefficient struct {
	Version      int                `json:"version"`
	Status       string             `json:"status"`
	Coefficients map[string]float64 `json:"coefficients"`
	UpdatedAt    time.Time          `json:"updatedAt"`
	UpdatedBy    *string            `json:"updatedBy"`
}

// ModelCoefficientUpdate 表示模型系数更新请求。
type ModelCoefficientUpdate struct {
	Version      int                `json:"version" binding:"required,min=1"`
	Coefficients map[string]float64 `json:"coefficients" binding:"required"`
	Reason       *string            `json:"reason,omitempty"`
}

// TransactionLimits 表示单次和每日交易限额。
type TransactionLimits struct {
	Single float64 `json:"single"`
	Daily  float64 `json:"daily"`
}

// RateLimits 表示用户频率和异常阈值。
type RateLimits struct {
	PerUserDaily     int `json:"perUserDaily"`
	AnomalyThreshold int `json:"anomalyThreshold"`
}

// PolicyRule 表示当前 Policy Guard 规则。
type PolicyRule struct {
	Version            int               `json:"version"`
	Status             string            `json:"status"`
	HighRiskOperations []string          `json:"highRiskOperations"`
	TransactionLimits  TransactionLimits `json:"transactionLimits"`
	RateLimits         RateLimits        `json:"rateLimits"`
	ApprovalFlow       string            `json:"approvalFlow"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	UpdatedBy          *string           `json:"updatedBy"`
}

// PolicyRuleUpdate 表示 Policy Guard 更新请求。
type PolicyRuleUpdate struct {
	Version            int               `json:"version" binding:"required,min=1"`
	HighRiskOperations []string          `json:"highRiskOperations" binding:"required,min=1"`
	TransactionLimits  TransactionLimits `json:"transactionLimits"`
	RateLimits         RateLimits        `json:"rateLimits"`
	ApprovalFlow       string            `json:"approvalFlow" binding:"required"`
	Reason             *string           `json:"reason,omitempty"`
}

// PlatformStats 表示平台统计总览。
type PlatformStats struct {
	Period map[string]string `json:"period"`
	Stats  map[string]any    `json:"stats"`
	Trends map[string]string `json:"trends"`
}

// AuditConversationItem 表示审计会话列表项。
type AuditConversationItem struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agentId"`
	UserID       string    `json:"userId"`
	MessageCount int64     `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	Status       string    `json:"status"`
}
