// Package dto 定义 Agent HTTP 请求与响应契约。
package dto

import "time"

// ReactConfig 表示 Agent 完整 ReAct 配置。
type ReactConfig struct {
	MaxRounds           int     `json:"maxRounds"`
	TargetRounds        int     `json:"targetRounds"`
	ToolsPerRound       int     `json:"toolsPerRound"`
	ConfidenceThreshold float64 `json:"confidenceThreshold"`
}

// ReactConfigPatch 表示 ReAct 配置的部分更新。
type ReactConfigPatch struct {
	MaxRounds           *int     `json:"maxRounds"`
	TargetRounds        *int     `json:"targetRounds"`
	ToolsPerRound       *int     `json:"toolsPerRound"`
	ConfidenceThreshold *float64 `json:"confidenceThreshold"`
}

// MemoryConfig 表示 Agent 完整记忆配置。
type MemoryConfig struct {
	ShortTermEnabled       bool   `json:"shortTermEnabled"`
	ShortTermWindowSize    int    `json:"shortTermWindowSize"`
	SummaryEnabled         bool   `json:"summaryEnabled"`
	SummaryTriggerTokens   int    `json:"summaryTriggerTokens"`
	SummaryMaxTokens       int    `json:"summaryMaxTokens"`
	SummaryModel           string `json:"summaryModel"`
	LongTermEnabled        bool   `json:"longTermEnabled"`
	LongTermExtractOnClose bool   `json:"longTermExtractOnClose"`
	LongTermInjectTopK     int    `json:"longTermInjectTopK"`
	LongTermExpireDays     int    `json:"longTermExpireDays"`
}

// MemoryConfigPatch 表示记忆配置的部分更新。
type MemoryConfigPatch struct {
	ShortTermEnabled       *bool   `json:"shortTermEnabled"`
	ShortTermWindowSize    *int    `json:"shortTermWindowSize"`
	SummaryEnabled         *bool   `json:"summaryEnabled"`
	SummaryTriggerTokens   *int    `json:"summaryTriggerTokens"`
	SummaryMaxTokens       *int    `json:"summaryMaxTokens"`
	SummaryModel           *string `json:"summaryModel"`
	LongTermEnabled        *bool   `json:"longTermEnabled"`
	LongTermExtractOnClose *bool   `json:"longTermExtractOnClose"`
	LongTermInjectTopK     *int    `json:"longTermInjectTopK"`
	LongTermExpireDays     *int    `json:"longTermExpireDays"`
}

// CreateRequest 表示直连创建 Agent 请求。
type CreateRequest struct {
	Name          *string  `json:"name"`
	Type          *string  `json:"type"`
	Prompt        *string  `json:"prompt"`
	LLMModel      *string  `json:"llmModel"`
	LLMProviderID *string  `json:"llmProviderId"`
	SkillIDs      []string `json:"skillIds"`
	ToolIDs       []string `json:"toolIds"`
	KnowledgeIDs  []string `json:"knowledgeIds"`
}

// UpdateRequest 表示显式更新 Agent 及目标能力集合的请求。
type UpdateRequest struct {
	AgentID       string             `json:"agentId" binding:"required"`
	Name          *string            `json:"name"`
	Type          *string            `json:"type"`
	Prompt        *string            `json:"prompt"`
	LLMModel      *string            `json:"llmModel"`
	LLMProviderID *string            `json:"llmProviderId"`
	ReactConfig   *ReactConfigPatch  `json:"reactConfig"`
	MemoryConfig  *MemoryConfigPatch `json:"memoryConfig"`
	SkillIDs      *[]string          `json:"skillIds"`
	ToolIDs       *[]string          `json:"toolIds"`
	KnowledgeIDs  *[]string          `json:"knowledgeIds"`
}

// DeleteRequest 表示显式删除 draft Agent 请求。
type DeleteRequest struct {
	AgentID string `json:"agentId" binding:"required"`
}

// ArchiveRequest 表示归档名称二次确认请求。
type ArchiveRequest struct {
	ConfirmName string `json:"confirmName" binding:"required"`
}

// CapabilityMountRequest 表示能力挂载请求。
type CapabilityMountRequest struct {
	CapabilityID string `json:"capabilityId" binding:"required"`
}

// CapabilityItem 表示一条能力挂载及其策略属性。
type CapabilityItem struct {
	ID                 string    `json:"id"`
	PolicyGuardEnabled bool      `json:"policyGuardEnabled"`
	ApprovalRequired   bool      `json:"approvalRequired"`
	MountedAt          time.Time `json:"mountedAt"`
}

// CapabilityRead 表示 Agent 当前全部能力挂载。
type CapabilityRead struct {
	Skills    []CapabilityItem `json:"skills"`
	Tools     []CapabilityItem `json:"tools"`
	Knowledge []CapabilityItem `json:"knowledge"`
}

// DetailResponse 表示 Agent 完整详情。
type DetailResponse struct {
	ID               string       `json:"id"`
	OwnerID          string       `json:"ownerId"`
	Name             string       `json:"name"`
	Status           string       `json:"status"`
	Type             string       `json:"type"`
	Prompt           *string      `json:"prompt"`
	LLMModel         *string      `json:"llmModel"`
	LLMProviderID    *string      `json:"llmProviderId"`
	ReactConfig      ReactConfig  `json:"reactConfig"`
	MemoryConfig     MemoryConfig `json:"memoryConfig"`
	Skills           []string     `json:"skills"`
	Tools            []string     `json:"tools"`
	Knowledge        []string     `json:"knowledge"`
	TotalInvocations int64        `json:"totalInvocations"`
	SuccessCount     int64        `json:"successCount"`
	FailCount        int64        `json:"failCount"`
	FailureRate      float64      `json:"failureRate"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt"`
	ActivatedAt      *time.Time   `json:"activatedAt"`
	PausedAt         *time.Time   `json:"pausedAt"`
	ArchivedAt       *time.Time   `json:"archivedAt"`
}

// ConsoleListItem 表示 Agent 列表项。
type ConsoleListItem struct {
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
	// Binded 表示该 Agent 是否已绑定到查询时传入的工单；未传 session_id 时恒为 false。
	Binded bool `json:"binded"`
}

// BindTicketRequest 表示工单与 Agent 的绑定请求。
type BindTicketRequest struct {
	// SessionID 为工单 ID，沿用前端 session_id 的叫法。
	SessionID string `json:"sessionId" binding:"required,max=64"`
	AgentID   string `json:"agentId" binding:"required,max=64"`
}

// BindTicketResponse 表示工单与 Agent 的绑定结果。
type BindTicketResponse struct {
	SessionID string `json:"sessionId"`
	AgentID   string `json:"agentId"`
	Binded    bool   `json:"binded"`
}

// ListResponse 表示 Agent 分页列表。
type ListResponse struct {
	Items    []ConsoleListItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// LifecycleResponse 表示生命周期状态转换结果。
type LifecycleResponse struct {
	Status              string `json:"status"`
	PreviousStatus      string `json:"previousStatus"`
	CurrentStatus       string `json:"currentStatus"`
	AgentID             string `json:"agentId"`
	ArchiveInProgress   bool   `json:"archiveInProgress"`
	InterruptedSessions int64  `json:"interruptedSessions"`
}

// LongTermMemoryRead 表示一条长期记忆。
type LongTermMemoryRead struct {
	ID                   string     `json:"id"`
	AgentID              string     `json:"agentId"`
	UserID               string     `json:"userId"`
	MemoryType           string     `json:"memoryType"`
	Content              string     `json:"content"`
	Importance           string     `json:"importance"`
	SourceConversationID string     `json:"sourceConversationId"`
	Status               string     `json:"status"`
	ExpiresAt            *time.Time `json:"expiresAt"`
}

// CreateWithBotRequest 表示创建 Agent 并注册 Bot 的编排请求。
type CreateWithBotRequest struct {
	BotName       *string  `json:"bot_name"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Prompt        string   `json:"prompt"`
	LLMModel      *string  `json:"llmModel"`
	LLMProviderID *string  `json:"llmProviderId"`
	SkillIDs      []string `json:"skillIds"`
	ToolIDs       []string `json:"toolIds"`
	KnowledgeIDs  []string `json:"knowledgeIds"`
}

// BindBotRequest 表示绑定已有 Bot 到路径 Agent 的请求。
type BindBotRequest struct {
	BotUserID string  `json:"botUserId"`
	Token     string  `json:"token"`
	BotName   *string `json:"botName"`
	CreateBot *bool   `json:"createBot"`
}

// BindBotLegacyRequest 表示兼容脚本使用的 Bot 绑定请求。
type BindBotLegacyRequest struct {
	InviteCode string  `json:"invite_code"`
	AgentID    string  `json:"agent_id"`
	BotUserID  string  `json:"bot_user_id"`
	Name       *string `json:"name"`
	Token      string  `json:"token"`
	CreateBot  *bool   `json:"create_bot"`
}

// CreateWithBotResponse 表示 Agent 与新注册 Bot 的编排结果。
type CreateWithBotResponse struct {
	Agent     DetailResponse `json:"agent"`
	OwnerID   string         `json:"ownerId"`
	BotUserID string         `json:"botUserId"`
	BotName   string         `json:"botName"`
	Connected bool           `json:"connected"`
}

// BindBotResponse 表示已有 Bot 的绑定结果。
type BindBotResponse struct {
	Agent     DetailResponse `json:"agent"`
	OwnerID   string         `json:"ownerId"`
	BotID     string         `json:"botId"`
	BotUserID string         `json:"botUserId"`
	BotName   string         `json:"botName"`
	Connected bool           `json:"connected"`
}
