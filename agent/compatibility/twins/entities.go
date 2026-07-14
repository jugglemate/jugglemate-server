// Package twins 实现当前项目旧 Twin 契约到 Go Agent 聚合的进程内适配。
package twins

import "time"

type mapping struct {
	ID             string    `gorm:"size:64;primaryKey"`
	OwnerID        string    `gorm:"column:owner_id;size:64;not null"`
	UniqueName     string    `gorm:"column:unique_name;size:64;not null"`
	AgentID        string    `gorm:"column:agent_id;size:64;not null"`
	KnowledgeID    string    `gorm:"column:knowledge_id;size:64;not null"`
	DisplayName    string    `gorm:"column:display_name;size:128;not null"`
	AvatarURL      string    `gorm:"column:avatar_url;type:text;not null"`
	Greeting       string    `gorm:"type:text;not null"`
	Status         string    `gorm:"size:20;not null"`
	ActiveVersion  string    `gorm:"column:active_version;size:32;not null"`
	TrainingMode   string    `gorm:"column:training_mode;size:20;not null"`
	MaterialsCount int       `gorm:"column:materials_count;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (mapping) TableName() string { return "twin_compat_mappings" }

type material struct {
	ID           string    `gorm:"size:64;primaryKey"`
	MappingID    string    `gorm:"column:mapping_id;size:64;not null"`
	MaterialType string    `gorm:"column:material_type;size:20;not null"`
	Title        string    `gorm:"size:200;not null"`
	Source       string    `gorm:"type:text;not null"`
	SizeBytes    int64     `gorm:"column:size_bytes;not null"`
	Content      string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (material) TableName() string { return "twin_compat_materials" }

type job struct {
	ID         string         `gorm:"size:64;primaryKey"`
	MappingID  string         `gorm:"column:mapping_id;size:64;not null"`
	JobType    string         `gorm:"column:job_type;size:20;not null"`
	Status     string         `gorm:"size:20;not null"`
	Progress   *int           `gorm:"column:progress"`
	Result     map[string]any `gorm:"type:jsonb;serializer:json;not null"`
	ErrorValue map[string]any `gorm:"column:error;type:jsonb;serializer:json;not null"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime"`
	StartedAt  *time.Time     `gorm:"column:started_at"`
	FinishedAt *time.Time     `gorm:"column:finished_at"`
}

func (job) TableName() string { return "twin_compat_jobs" }

type version struct {
	ID             string    `gorm:"size:64;primaryKey"`
	MappingID      string    `gorm:"column:mapping_id;size:64;not null"`
	Version        string    `gorm:"size:32;not null"`
	Mode           string    `gorm:"size:20;not null"`
	Active         bool      `gorm:"not null"`
	TrainingJobID  string    `gorm:"column:training_job_id;size:64;not null"`
	MaterialsCount int       `gorm:"column:materials_count;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (version) TableName() string { return "twin_compat_versions" }

type evaluation struct {
	ID           string           `gorm:"size:64;primaryKey"`
	MappingID    string           `gorm:"column:mapping_id;size:64;not null"`
	Version      string           `gorm:"size:32;not null"`
	OverallScore float64          `gorm:"column:overall_score;not null"`
	Dimensions   []map[string]any `gorm:"type:jsonb;serializer:json;not null"`
	SummaryMD    string           `gorm:"column:summary_md;type:text;not null"`
	CreatedAt    time.Time        `gorm:"column:created_at;autoCreateTime"`
}

func (evaluation) TableName() string { return "twin_compat_evaluations" }
