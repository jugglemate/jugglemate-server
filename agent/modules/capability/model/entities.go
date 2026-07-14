// Package model 定义旧版 Capability Tool/Skill 持久化实体。
package model

import "time"

// Tool 表示可独立测试和挂载的 HTTP 能力。
type Tool struct {
	ID               string         `gorm:"size:64;primaryKey"`
	OwnerID          string         `gorm:"column:owner_id;size:64;not null"`
	Name             string         `gorm:"size:128;not null"`
	Description      *string        `gorm:"type:text"`
	Visibility       string         `gorm:"size:20;not null"`
	Status           string         `gorm:"size:20;not null"`
	APIEndpoint      string         `gorm:"column:api_endpoint;size:512;not null"`
	HTTPMethod       string         `gorm:"column:http_method;size:16;not null"`
	InputSchema      map[string]any `gorm:"column:input_schema;type:jsonb;serializer:json"`
	OutputSchema     map[string]any `gorm:"column:output_schema;type:jsonb;serializer:json"`
	RiskLevel        string         `gorm:"column:risk_level;size:20;not null"`
	RequiresApproval bool           `gorm:"column:requires_approval;not null"`
	TimeoutSeconds   int64          `gorm:"column:timeout_seconds;not null"`
	CallCount        int64          `gorm:"column:call_count;not null"`
	SuccessRate      float64        `gorm:"column:success_rate;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeprecatedAt     *time.Time     `gorm:"column:deprecated_at"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at"`
}

// TableName 返回 Tool 表名。
func (Tool) TableName() string { return "tools" }

// Skill 表示由多个 Tool 按顺序编排的能力。
type Skill struct {
	ID               string         `gorm:"size:64;primaryKey"`
	OwnerID          string         `gorm:"column:owner_id;size:64;not null"`
	Name             string         `gorm:"size:128;not null"`
	Description      *string        `gorm:"type:text"`
	Visibility       string         `gorm:"size:20;not null"`
	Status           string         `gorm:"size:20;not null"`
	Category         *string        `gorm:"size:64"`
	InputSchema      map[string]any `gorm:"column:input_schema;type:jsonb;serializer:json"`
	OutputSchema     map[string]any `gorm:"column:output_schema;type:jsonb;serializer:json"`
	RiskLevel        string         `gorm:"column:risk_level;size:20;not null"`
	RequiresApproval bool           `gorm:"column:requires_approval;not null"`
	CallCount        int64          `gorm:"column:call_count;not null"`
	SuccessRate      float64        `gorm:"column:success_rate;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeprecatedAt     *time.Time     `gorm:"column:deprecated_at"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at"`
}

// TableName 返回 Skill 表名。
func (Skill) TableName() string { return "skills" }

// SkillTool 表示 Skill 内 Tool 的稳定执行顺序。
type SkillTool struct {
	ID         string    `gorm:"size:64;primaryKey"`
	SkillID    string    `gorm:"column:skill_id;size:64;not null"`
	ToolID     string    `gorm:"column:tool_id;size:64;not null"`
	OrderIndex int       `gorm:"column:order_index;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回 Skill-Tool 关联表名。
func (SkillTool) TableName() string { return "skill_tools" }
