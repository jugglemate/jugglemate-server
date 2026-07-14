// Package model 定义 Tools v2 领域的持久化实体。
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Provider 表示工具提供方及其连接、鉴权配置。
type Provider struct {
	ID               string         `gorm:"size:64;primaryKey"`
	OwnerID          string         `gorm:"column:owner_id;size:64;not null"`
	ProviderType     string         `gorm:"column:provider_type;size:32;not null"`
	Name             string         `gorm:"size:128;not null"`
	DisplayName      *string        `gorm:"column:display_name;size:128"`
	ConnectionConfig map[string]any `gorm:"column:connection_config;type:jsonb;serializer:json;not null"`
	AuthType         string         `gorm:"column:auth_type;size:40;not null"`
	AuthSchema       map[string]any `gorm:"column:auth_schema;type:jsonb;serializer:json;not null"`
	Status           string         `gorm:"size:20;not null"`
	Visibility       string         `gorm:"size:20;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at"`
}

// TableName 返回 Provider 表名。
func (Provider) TableName() string { return "tool_providers" }

// Definition 表示可测试、激活、挂载和执行的工具定义。
type Definition struct {
	ID               string         `gorm:"size:64;primaryKey"`
	ProviderID       string         `gorm:"column:provider_id;size:64;not null"`
	OwnerID          string         `gorm:"column:owner_id;size:64;not null"`
	ToolType         string         `gorm:"column:tool_type;size:16;not null"`
	Name             string         `gorm:"size:128;not null"`
	Description      *string        `gorm:"type:text"`
	InputSchema      map[string]any `gorm:"column:input_schema;type:jsonb;serializer:json;not null"`
	OutputSchema     map[string]any `gorm:"column:output_schema;type:jsonb;serializer:json;not null"`
	ExecutionConfig  map[string]any `gorm:"column:execution_config;type:jsonb;serializer:json;not null"`
	RiskLevel        string         `gorm:"column:risk_level;size:20;not null"`
	ApprovalRequired bool           `gorm:"column:approval_required;not null"`
	TimeoutSeconds   int            `gorm:"column:timeout_seconds;not null"`
	Status           string         `gorm:"size:20;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at"`
}

// TableName 返回 Definition 表名。
func (Definition) TableName() string { return "tool_definitions" }

// Credential 表示加密保存的工具调用凭据。
type Credential struct {
	ID                         string     `gorm:"size:64;primaryKey"`
	OwnerID                    string     `gorm:"column:owner_id;size:64;not null"`
	ProviderID                 string     `gorm:"column:provider_id;size:64;not null"`
	Name                       string     `gorm:"size:128;not null"`
	CredentialPayloadEncrypted string     `gorm:"column:credential_payload_encrypted;type:text;not null"`
	Status                     string     `gorm:"size:16;not null"`
	RotatedAt                  *time.Time `gorm:"column:rotated_at"`
	CreatedAt                  time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                  time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回 Credential 表名。
func (Credential) TableName() string { return "tool_credentials" }

// Binding 表示 Agent 与工具定义之间的运行时挂载关系。
type Binding struct {
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

// TableName 返回 Binding 表名。
func (Binding) TableName() string { return "agent_tools" }

// CallRecord 表示一次工具调用的不可变事实记录。
type CallRecord struct {
	ID               string           `gorm:"size:64;primaryKey"`
	TraceID          string           `gorm:"column:trace_id;size:64;not null"`
	ConversationID   *string          `gorm:"column:conversation_id;size:64"`
	AgentID          string           `gorm:"column:agent_id;size:64;not null"`
	OwnerID          string           `gorm:"column:owner_id;size:64;not null"`
	ToolDefinitionID string           `gorm:"column:tool_definition_id;size:64;not null"`
	Status           string           `gorm:"size:16;not null"`
	InputSummary     *string          `gorm:"column:input_summary;type:text"`
	OutputSummary    *string          `gorm:"column:output_summary;type:text"`
	ErrorCode        *string          `gorm:"column:error_code;size:64"`
	ErrorMessage     *string          `gorm:"column:error_message;type:text"`
	DurationMS       int              `gorm:"column:duration_ms;not null"`
	BilledCredit     *decimal.Decimal `gorm:"column:billed_credit;type:numeric(20,6)"`
	IdempotencyKey   *string          `gorm:"column:idempotency_key;size:128"`
	CreatedAt        time.Time        `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回 CallRecord 表名。
func (CallRecord) TableName() string { return "tool_call_records" }

// DecisionRecord 表示单轮工具候选评分与选择结果。
type DecisionRecord struct {
	ID                string           `gorm:"size:64;primaryKey"`
	TraceID           string           `gorm:"column:trace_id;size:64;not null"`
	ConversationID    *string          `gorm:"column:conversation_id;size:64"`
	AgentID           string           `gorm:"column:agent_id;size:64;not null"`
	OwnerID           string           `gorm:"column:owner_id;size:64;not null"`
	IntentSummary     string           `gorm:"column:intent_summary;type:text;not null"`
	CandidateTools    []map[string]any `gorm:"column:candidate_tools;type:jsonb;serializer:json;not null"`
	SelectedTools     []string         `gorm:"column:selected_tools;type:jsonb;serializer:json;not null"`
	DecisionAction    string           `gorm:"column:decision_action;size:32;not null"`
	TerminationReason *string          `gorm:"column:termination_reason;size:128"`
	CreatedAt         time.Time        `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回 DecisionRecord 表名。
func (DecisionRecord) TableName() string { return "tool_decision_records" }
