// Package dto 定义 Message 域的 HTTP 请求与响应契约。
package dto

import "time"

// ChatRequest 表示标准推理对话请求。
type ChatRequest struct {
	AgentID              string         `json:"agentId" binding:"required"`
	UserID               string         `json:"userId" binding:"required"`
	Message              string         `json:"message" binding:"required"`
	ConversationID       *string        `json:"conversationId,omitempty"`
	EnableHistoryContext *bool          `json:"enableHistoryContext,omitempty"`
	TraceID              *string        `json:"traceId,omitempty"`
	Metadata             map[string]any `json:"metadata"`
}

// ChatResponse 表示标准推理对话响应。
type ChatResponse struct {
	ConversationID string  `json:"conversationId"`
	Answer         string  `json:"answer"`
	Path           string  `json:"path"`
	Status         string  `json:"status"`
	TraceID        string  `json:"traceId"`
	ErrorCode      *string `json:"errorCode"`
}

// AppChatMessage 表示应用直连接口中的一条模型消息。
type AppChatMessage struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content"`
}

// AppChatRequest 表示跳过推理编排的应用直连请求。
type AppChatRequest struct {
	AgentName            string           `json:"agentName" binding:"required"`
	UserID               *string          `json:"userId,omitempty"`
	EnableHistoryContext bool             `json:"enableHistoryContext"`
	MaxTokens            *int             `json:"max_tokens,omitempty"`
	Temperature          *float64         `json:"temperature,omitempty"`
	Messages             []AppChatMessage `json:"messages" binding:"required,min=1"`
	TraceID              *string          `json:"trace_id,omitempty"`
	Metadata             map[string]any   `json:"metadata"`
}

// AppChatResponse 表示应用直连非流式响应。
type AppChatResponse struct {
	Answer    string  `json:"answer"`
	Path      string  `json:"path"`
	Status    string  `json:"status"`
	TraceID   string  `json:"traceId"`
	ErrorCode *string `json:"errorCode"`
}

// ConversationListItem 表示 Owner 会话列表项。
type ConversationListItem struct {
	ID           string     `json:"id"`
	AgentID      string     `json:"agent_id"`
	UserID       string     `json:"user_id"`
	UserName     *string    `json:"user_name"`
	Pic          *string    `json:"pic"`
	MessageCount int64      `json:"message_count"`
	TotalTokens  int64      `json:"total_tokens"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt *time.Time `json:"last_active_at"`
}

// PaginationMeta 表示会话列表分页信息。
type PaginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ConversationListResponse 表示会话分页响应。
type ConversationListResponse struct {
	Items      []ConversationListItem `json:"items"`
	Pagination PaginationMeta         `json:"pagination"`
}

// MessageItem 表示会话详情中的单条消息。
type MessageItem struct {
	ID             string    `json:"id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	SelfTokenCount int       `json:"self_token_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConversationDetail 表示 Owner 会话详情。
type ConversationDetail struct {
	ID           string        `json:"id"`
	AgentID      string        `json:"agent_id"`
	AgentName    string        `json:"agent_name"`
	UserID       string        `json:"user_id"`
	UserName     *string       `json:"user_name"`
	Pic          *string       `json:"pic"`
	Status       string        `json:"status"`
	CreatedAt    time.Time     `json:"created_at"`
	LastActiveAt *time.Time    `json:"last_active_at"`
	MessageCount int           `json:"message_count"`
	TotalTokens  int           `json:"total_tokens"`
	Messages     []MessageItem `json:"messages"`
}

// ConversationStats 表示 Agent 会话统计。
type ConversationStats struct {
	TotalConversations  int64 `json:"total_conversations"`
	ActiveConversations int64 `json:"active_conversations"`
	TodayNew            int64 `json:"today_new"`
}

// OwnerAgentItem 表示附带会话统计的 Owner Agent。
type OwnerAgentItem struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	ConversationStats ConversationStats `json:"conversation_stats"`
}

// ConversationSummary 表示最新会话摘要。
type ConversationSummary struct {
	ID                  string  `json:"id"`
	ConversationID      string  `json:"conversationId"`
	Status              string  `json:"status"`
	SummaryText         string  `json:"summaryText"`
	CoveredMessageCount int     `json:"coveredMessageCount"`
	TokenCount          int     `json:"tokenCount"`
	ErrorCode           *string `json:"errorCode"`
}

// HumanEnterRequest 表示开启人工介入请求。
type HumanEnterRequest struct {
	AgentID       string  `json:"agent_id" binding:"required,max=64"`
	UserID        string  `json:"user_id" binding:"required,max=64"`
	ForceTakeover bool    `json:"force_takeover"`
	Reason        *string `json:"reason,omitempty"`
}

// HumanExitRequest 表示退出人工介入请求。
type HumanExitRequest struct {
	AgentID string  `json:"agent_id" binding:"required,max=64"`
	UserID  string  `json:"user_id" binding:"required,max=64"`
	Reason  *string `json:"reason,omitempty"`
}

// HumanSendRequest 表示人工发送消息请求。
type HumanSendRequest struct {
	AgentID   string  `json:"agent_id" binding:"required,max=64"`
	UserID    string  `json:"user_id" binding:"required,max=64"`
	Text      string  `json:"text" binding:"required,max=10000"`
	ParseMode string  `json:"parse_mode"`
	BotID     *string `json:"botId,omitempty"`
	BotUserID *string `json:"botUserId,omitempty"`
}

// HumanStatusResponse 表示人工介入状态。
type HumanStatusResponse struct {
	Status           string     `json:"status"`
	Code             string     `json:"code"`
	Message          string     `json:"message"`
	AgentID          string     `json:"agent_id"`
	UserID           string     `json:"user_id"`
	ManualActive     bool       `json:"manual_active"`
	OperatorID       *string    `json:"operator_id"`
	ExpiresInSeconds *int       `json:"expires_in_seconds"`
	ExpireAt         *time.Time `json:"expire_at"`
	LastSendAt       *time.Time `json:"last_send_at"`
}

// HumanOperationResponse 表示人工进入/退出操作结果。
type HumanOperationResponse struct {
	Status           string     `json:"status"`
	Code             string     `json:"code"`
	Message          string     `json:"message"`
	AgentID          string     `json:"agent_id"`
	UserID           string     `json:"user_id"`
	OperatorID       string     `json:"operator_id"`
	ExpiresInSeconds *int       `json:"expires_in_seconds,omitempty"`
	ExpireAt         *time.Time `json:"expire_at,omitempty"`
}

// HumanSendResponse 表示人工消息发送结果。
type HumanSendResponse struct {
	HumanOperationResponse
	MessageID *string `json:"message_id"`
	BotID     *string `json:"botId"`
	BotUserID *string `json:"botUserId"`
}

// HumanPollMessage 表示人工轮询的一条用户消息。
type HumanPollMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Source         string    `json:"source"`
	OperatorID     *string   `json:"operator_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// HumanPollResponse 表示人工轮询增量消息结果。
type HumanPollResponse struct {
	Status      string             `json:"status"`
	Code        string             `json:"code"`
	Message     string             `json:"message"`
	AgentID     string             `json:"agent_id"`
	UserID      string             `json:"user_id"`
	SinceTS     time.Time          `json:"since_ts"`
	NextSinceTS time.Time          `json:"next_since_ts"`
	Messages    []HumanPollMessage `json:"messages"`
}
