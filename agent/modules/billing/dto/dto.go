// Package dto 定义 Billing HTTP 数据契约。
package dto

// AmountRequest 创建充值/入金订单请求。
type AmountRequest struct {
	CreditAmount any `json:"creditAmount" binding:"required"`
}

// WalletResponse 钱包余额响应。
type WalletResponse struct {
	ID             string `json:"id"`
	OwnerID        string `json:"ownerId"`
	Balance        string `json:"balance"`
	TotalRecharged string `json:"totalRecharged"`
	TotalConsumed  string `json:"totalConsumed"`
	Status         string `json:"status"`
	Version        int    `json:"version"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// RechargeResponse 充值订单响应。
type RechargeResponse struct {
	ID           string  `json:"id"`
	OwnerID      string  `json:"ownerId"`
	CreditAmount string  `json:"creditAmount"`
	ExpiresAt    *string `json:"expiresAt"`
	Status       string  `json:"status"`
	ConfirmedBy  *string `json:"confirmedBy"`
	CancelledBy  *string `json:"cancelledBy"`
	Reason       *string `json:"reason"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

// RechargeListResponse 充值订单分页响应。
type RechargeListResponse struct {
	Items    []RechargeResponse `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

// DepositResponse 外部入金订单响应。
type DepositResponse struct {
	ID        string  `json:"id"`
	OwnerID   string  `json:"ownerId"`
	Amount    string  `json:"amount"`
	Currency  string  `json:"currency"`
	Status    string  `json:"status"`
	Remark    *string `json:"remark"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

// DepositWebhookRequest 外部入金回调请求。
type DepositWebhookRequest struct {
	ID      string `json:"id" binding:"required"`
	Amount  any    `json:"amount" binding:"required"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// DepositWebhookResponse 外部入金回调响应。
type DepositWebhookResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TokenConsumptionItem 多维 Token 消耗聚合项。
type TokenConsumptionItem struct {
	DimensionValue  string  `json:"dimensionValue"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	CreditsConsumed float64 `json:"creditsConsumed"`
}

// TokenConsumptionResponse 多维 Token 消耗响应。
type TokenConsumptionResponse struct {
	Dimension string                 `json:"dimension"`
	Items     []TokenConsumptionItem `json:"items"`
	StartDate string                 `json:"startDate"`
	EndDate   string                 `json:"endDate"`
}

// DailyConsumptionItem 每日消耗聚合项。
type DailyConsumptionItem struct {
	EventDate       string  `json:"eventDate"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	SessionCount    int64   `json:"sessionCount"`
	CreditsConsumed float64 `json:"creditsConsumed"`
}

// DailyConsumptionResponse 每日消耗分页响应。
type DailyConsumptionResponse struct {
	Items    []DailyConsumptionItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

// SessionConsumptionItem 会话维度消耗明细。
type SessionConsumptionItem struct {
	SessionID       string  `json:"sessionId"`
	AgentID         string  `json:"agentId"`
	AgentName       string  `json:"agentName"`
	ModelID         string  `json:"modelId"`
	InputTokens     int64   `json:"inputTokens"`
	OutputTokens    int64   `json:"outputTokens"`
	TotalTokens     int64   `json:"totalTokens"`
	CreditsConsumed float64 `json:"creditsConsumed"`
	EventCount      int64   `json:"eventCount"`
}

// SessionConsumptionResponse 指定日期的会话消耗响应。
type SessionConsumptionResponse struct {
	Date  string                   `json:"date"`
	Items []SessionConsumptionItem `json:"items"`
}
