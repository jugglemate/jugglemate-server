// Package model 定义 Agent 聚合及其能力挂载、长期记忆实体。
package model

import "time"

// Agent 表示可配置、启停和挂载能力的智能体。
type Agent struct {
	ID               string         `gorm:"size:64;primaryKey"`
	OwnerID          string         `gorm:"column:owner_id;size:64;not null"`
	Name             string         `gorm:"size:128;not null"`
	Type             string         `gorm:"size:32;not null"`
	Prompt           *string        `gorm:"type:text"`
	LLMModel         *string        `gorm:"column:llm_model;size:100"`
	LLMProviderID    *string        `gorm:"column:llm_provider_id;size:64"`
	Status           string         `gorm:"size:20;not null"`
	ReactConfig      map[string]any `gorm:"column:react_config;type:jsonb;serializer:json;not null"`
	MemoryConfig     map[string]any `gorm:"column:memory_config;type:jsonb;serializer:json;not null"`
	TotalInvocations int64          `gorm:"column:total_invocations;not null"`
	SuccessCount     int64          `gorm:"column:success_count;not null"`
	FailCount        int64          `gorm:"column:fail_count;not null"`
	FailureRate      float64        `gorm:"column:failure_rate;not null"`
	ActivatedAt      *time.Time     `gorm:"column:activated_at"`
	PausedAt         *time.Time     `gorm:"column:paused_at"`
	ArchivedAt       *time.Time     `gorm:"column:archived_at"`
	CreatedAt        time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Agent 表名。
func (Agent) TableName() string { return "agents" }

// SkillBinding 表示 Agent 与旧版 Skill 的挂载关系。
type SkillBinding struct {
	ID                 string    `gorm:"size:64;primaryKey"`
	AgentID            string    `gorm:"column:agent_id;size:64;not null"`
	SkillID            string    `gorm:"column:skill_id;size:64;not null"`
	PolicyGuardEnabled bool      `gorm:"column:policy_guard_enabled;not null"`
	MountedAt          time.Time `gorm:"column:mounted_at;autoCreateTime"`
}

// TableName 返回 Agent-Skill 挂载表名。
func (SkillBinding) TableName() string { return "agent_skills" }

// ToolBinding 表示 Agent 与 Tools v2/旧版 Tool 的挂载关系。
type ToolBinding struct {
	ID                 string         `gorm:"size:64;primaryKey"`
	AgentID            string         `gorm:"column:agent_id;size:64;not null"`
	ToolID             string         `gorm:"column:tool_id;size:64;not null"`
	Enabled            bool           `gorm:"not null"`
	RuntimeOverrides   map[string]any `gorm:"column:runtime_overrides;type:jsonb;serializer:json"`
	CredentialID       *string        `gorm:"column:credential_id;size:64"`
	PolicyGuardEnabled bool           `gorm:"column:policy_guard_enabled;not null"`
	ApprovalRequired   bool           `gorm:"column:approval_required;not null"`
	MountedAt          time.Time      `gorm:"column:mounted_at;autoCreateTime"`
	UpdatedAt          time.Time      `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Agent-Tool 挂载表名。
func (ToolBinding) TableName() string { return "agent_tools" }

// KnowledgeBinding 表示 Agent 与知识库的挂载关系。
type KnowledgeBinding struct {
	ID          string    `gorm:"size:64;primaryKey"`
	AgentID     string    `gorm:"column:agent_id;size:64;not null"`
	KnowledgeID string    `gorm:"column:knowledge_id;size:64;not null"`
	MountedAt   time.Time `gorm:"column:mounted_at;autoCreateTime"`
}

// TableName 返回 Agent-Knowledge 挂载表名。
func (KnowledgeBinding) TableName() string { return "agent_knowledge" }

// LongTermMemory 表示可过期、可按重要度注入的用户长期记忆。
type LongTermMemory struct {
	ID                   string     `gorm:"size:64;primaryKey"`
	AgentID              string     `gorm:"column:agent_id;size:64;not null"`
	UserID               string     `gorm:"column:user_id;size:64;not null"`
	MemoryType           string     `gorm:"column:memory_type;size:20;not null"`
	Content              string     `gorm:"type:text;not null"`
	Importance           string     `gorm:"size:20;not null"`
	SourceConversationID *string    `gorm:"column:source_conversation_id;size:64"`
	Status               string     `gorm:"size:20;not null"`
	ExpiresAt            *time.Time `gorm:"column:expires_at"`
	CreatedAt            time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回长期记忆表名。
func (LongTermMemory) TableName() string { return "long_term_memories" }

// Bot 表示已注册并可通过 SDK 连接的 IM Bot。
type Bot struct {
	ID         string    `gorm:"size:64;primaryKey"`
	OwnerID    string    `gorm:"column:owner_id;size:64;not null"`
	InviteCode string    `gorm:"column:invite_code;size:64;not null"`
	BotUserID  string    `gorm:"column:bot_user_id;size:64;not null"`
	BotName    string    `gorm:"column:bot_name;size:128;not null"`
	Token      string    `gorm:"type:text;not null"`
	Status     string    `gorm:"size:16;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Bot 表名。
func (Bot) TableName() string { return "bots" }

// BotBinding 表示一个 Bot 到一个 Agent 的唯一绑定。
type BotBinding struct {
	ID        string    `gorm:"size:64;primaryKey"`
	BotID     string    `gorm:"column:bot_id;size:64;not null"`
	AgentID   string    `gorm:"column:agent_id;size:64;not null"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Bot-Agent 绑定表名。
func (BotBinding) TableName() string { return "bot_agent_bindings" }
