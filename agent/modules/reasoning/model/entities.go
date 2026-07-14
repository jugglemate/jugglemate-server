// Package model 定义推理、会话、消息、摘要与 ReAct 持久化实体。
package model

import "time"

// Conversation 表示同一 Agent 与用户之间可续用的对话上下文。
type Conversation struct {
	ID                 string    `gorm:"size:64;primaryKey"`
	AgentID            string    `gorm:"column:agent_id;size:64;not null"`
	UserID             string    `gorm:"column:user_id;size:64;not null"`
	InviteCode         *string   `gorm:"column:invite_code;size:64"`
	UserName           *string   `gorm:"column:user_name;size:128"`
	Pic                *string   `gorm:"type:text"`
	SummaryID          *string   `gorm:"column:summary_id;size:64"`
	ContextTokenBudget *int      `gorm:"column:context_token_budget"`
	Status             string    `gorm:"size:20;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回会话表名。
func (Conversation) TableName() string { return "conversations" }

// Message 表示会话中的用户、助手、系统或工具消息。
type Message struct {
	ID             string    `gorm:"size:64;primaryKey"`
	ConversationID string    `gorm:"column:conversation_id;size:64;not null"`
	UserID         *string   `gorm:"column:user_id;size:64"`
	MessageID      *string   `gorm:"column:message_id;size:128"`
	Role           string    `gorm:"size:16;not null"`
	Content        string    `gorm:"type:text;not null"`
	IsSummarized   bool      `gorm:"column:is_summarized;not null"`
	SelfTokenCount int       `gorm:"column:self_token_count;not null"`
	Source         string    `gorm:"size:16;not null"`
	OperatorID     *string   `gorm:"column:operator_id;size:64"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回消息表名。
func (Message) TableName() string { return "messages" }

// ReActSession 表示一次可审计的多轮推理会话。
type ReActSession struct {
	ID              string     `gorm:"size:64;primaryKey"`
	ConversationID  string     `gorm:"column:conversation_id;size:64;not null"`
	AgentID         string     `gorm:"column:agent_id;size:64;not null"`
	UserID          string     `gorm:"column:user_id;size:64;not null"`
	Status          string     `gorm:"size:20;not null"`
	CurrentRound    int        `gorm:"column:current_round;not null"`
	MaxRounds       int        `gorm:"column:max_rounds;not null"`
	TargetRounds    int        `gorm:"column:target_rounds;not null"`
	ConfidenceScore float64    `gorm:"column:confidence_score;not null"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	Error           *string    `gorm:"type:text"`
}

// TableName 返回 ReAct 会话表名。
func (ReActSession) TableName() string { return "react_sessions" }

// ReActRound 表示 ReAct 的单轮思考、行动与观察事实。
type ReActRound struct {
	ID                 string           `gorm:"size:64;primaryKey"`
	SessionID          string           `gorm:"column:session_id;size:64;not null"`
	RoundNumber        int              `gorm:"column:round_number;not null"`
	Thought            *string          `gorm:"type:text"`
	ToolCalls          []map[string]any `gorm:"column:tool_calls;type:jsonb;serializer:json"`
	Observation        *string          `gorm:"type:text"`
	ObservationSummary *string          `gorm:"column:observation_summary;size:500"`
	ConfidenceScore    *float64         `gorm:"column:confidence_score"`
	Decision           *string          `gorm:"size:16"`
	CreatedAt          time.Time        `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回 ReAct 轮次表名。
func (ReActRound) TableName() string { return "react_rounds" }

// ConversationSummary 表示已覆盖一段消息窗口的会话摘要。
type ConversationSummary struct {
	ID                  string    `gorm:"size:64;primaryKey"`
	ConversationID      string    `gorm:"column:conversation_id;size:64;not null"`
	AgentID             string    `gorm:"column:agent_id;size:64;not null"`
	UserID              string    `gorm:"column:user_id;size:64;not null"`
	SummaryText         string    `gorm:"column:summary_text;type:text;not null"`
	CoveredMessageIDs   []string  `gorm:"column:covered_message_ids;type:jsonb;serializer:json"`
	CoveredMessageCount int       `gorm:"column:covered_message_count;not null"`
	TokenCount          int       `gorm:"column:token_count;not null"`
	Status              string    `gorm:"size:20;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回会话摘要表名。
func (ConversationSummary) TableName() string { return "conversation_summaries" }
