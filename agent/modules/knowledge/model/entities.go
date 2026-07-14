// Package model 定义 Knowledge 域的持久化实体。
package model

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

// Knowledge 表示知识库聚合根。
type Knowledge struct {
	ID                string     `gorm:"size:64;primaryKey"`
	OwnerID           string     `gorm:"column:owner_id;size:64;not null"`
	Name              string     `gorm:"size:200;not null"`
	Type              string     `gorm:"size:32;not null"`
	SourceType        string     `gorm:"column:source_type;size:16;not null"`
	SourceURL         *string    `gorm:"column:source_url;size:2000"`
	FileID            *string    `gorm:"column:file_id;size:64"`
	Status            string     `gorm:"size:20;not null"`
	ChunkSize         int        `gorm:"column:chunk_size;not null"`
	OverlapWindow     int        `gorm:"column:overlap_window;not null"`
	DocumentCount     int        `gorm:"column:document_count;not null"`
	VectorCount       int        `gorm:"column:vector_count;not null"`
	HitRate           float64    `gorm:"column:hit_rate;not null"`
	LastCalculatedAt  *time.Time `gorm:"column:last_calculated_at"`
	WarningLowHitRate bool       `gorm:"column:warning_low_hit_rate;not null"`
	LastErrorCode     *string    `gorm:"column:last_error_code;size:64"`
	LastErrorMessage  *string    `gorm:"column:last_error_message;type:text"`
	CurrentTaskID     *string    `gorm:"column:current_task_id;size:100"`
	FailedAt          *time.Time `gorm:"column:failed_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt         *time.Time `gorm:"column:deleted_at"`
}

// TableName 返回知识库表名。
func (Knowledge) TableName() string { return "knowledge" }

// File 表示一份已接收的知识源文件。
type File struct {
	ID              string     `gorm:"size:64;primaryKey"`
	KnowledgeID     string     `gorm:"column:knowledge_id;size:64;not null"`
	OriginalName    string     `gorm:"column:original_name;size:200;not null"`
	StoredPath      string     `gorm:"column:stored_path;size:500;not null"`
	FileSize        int64      `gorm:"column:file_size;not null"`
	MIMEType        string     `gorm:"column:mime_type;size:64;not null"`
	ContentHash     *string    `gorm:"column:content_hash;size:64"`
	TextContent     *string    `gorm:"column:text_content;type:text"`
	TextExtractedAt *time.Time `gorm:"column:text_extracted_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回知识文件表名。
func (File) TableName() string { return "knowledge_file" }

// Task 表示一次向量化任务。
type Task struct {
	ID           string     `gorm:"size:64;primaryKey"`
	KnowledgeID  string     `gorm:"column:knowledge_id;size:64;not null"`
	Status       string     `gorm:"size:20;not null"`
	Stage        string     `gorm:"size:32;not null"`
	Progress     float64    `gorm:"not null"`
	RetryCount   int        `gorm:"column:retry_count;not null"`
	ErrorCode    *string    `gorm:"column:error_code;size:64"`
	ErrorMessage *string    `gorm:"column:error_message;type:text"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回知识任务表名。
func (Task) TableName() string { return "knowledge_task" }

// Vector 表示一个知识文本分块及其向量。
type Vector struct {
	ID             string          `gorm:"size:64;primaryKey"`
	KnowledgeID    string          `gorm:"column:knowledge_id;size:64;not null"`
	ChunkID        *string         `gorm:"column:chunk_id;size:64"`
	DocumentIndex  int             `gorm:"column:document_index;not null"`
	ChunkIndex     int             `gorm:"column:chunk_index;not null"`
	ContentChunk   string          `gorm:"column:content_chunk;type:text;not null"`
	Embedding      pgvector.Vector `gorm:"type:vector(1536)"`
	SourcePosition map[string]any  `gorm:"column:source_position;type:jsonb;serializer:json"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回知识向量表名。
func (Vector) TableName() string { return "knowledge_vectors" }

// RetrievalLog 表示一次知识检索是否命中。
type RetrievalLog struct {
	ID          string    `gorm:"size:64;primaryKey"`
	KnowledgeID string    `gorm:"column:knowledge_id;size:64;not null"`
	IsHit       bool      `gorm:"column:is_hit;not null"`
	RetrievedAt time.Time `gorm:"column:retrieved_at;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回检索日志表名。
func (RetrievalLog) TableName() string { return "knowledge_retrieval_logs" }
