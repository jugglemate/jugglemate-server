// Package dto 定义 Knowledge HTTP 数据契约。
package dto

import "time"

// Create 创建知识库请求。
type Create struct {
	Name             string  `json:"name" binding:"required,max=200"`
	Type             string  `json:"type"`
	SourceType       string  `json:"sourceType"`
	SourceURL        *string `json:"sourceUrl,omitempty"`
	AgentID          *string `json:"agentId,omitempty"`
	TriggerVectorize *bool   `json:"triggerVectorize,omitempty"`
}

// Update 显式更新知识库请求。
type Update struct {
	KnowledgeID        string  `json:"knowledgeId" binding:"required"`
	Name               *string `json:"name,omitempty"`
	SourceURL          *string `json:"sourceUrl,omitempty"`
	ChunkSize          *int    `json:"chunkSize,omitempty"`
	OverlapWindow      *int    `json:"overlapWindow,omitempty"`
	TriggerRevectorize bool    `json:"triggerRevectorize"`
	EmbeddingModel     *string `json:"embeddingModel,omitempty"`
}

// Delete 显式删除知识库请求。
type Delete struct {
	KnowledgeID string `json:"knowledgeId" binding:"required"`
}

// Read 知识库详情响应。
type Read struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	SourceType        string    `json:"sourceType"`
	SourceURL         *string   `json:"sourceUrl"`
	Status            string    `json:"status"`
	Config            Config    `json:"config"`
	Stats             Stats     `json:"stats"`
	OwnerID           string    `json:"owner_id"`
	WarningLowHitRate bool      `json:"warning_low_hit_rate"`
	LastErrorCode     *string   `json:"last_error_code"`
	LastErrorMessage  *string   `json:"last_error_message"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Config 知识分块配置。
type Config struct {
	ChunkSize     int `json:"chunk_size"`
	OverlapWindow int `json:"overlap_window"`
}

// Stats 知识库统计信息。
type Stats struct {
	DocumentCount    int        `json:"document_count"`
	VectorCount      int        `json:"vector_count"`
	HitRate          float64    `json:"hit_rate"`
	LastCalculatedAt *time.Time `json:"last_calculated_at"`
}

// FileMetadataUpload 元数据上传兼容请求。
type FileMetadataUpload struct {
	FileName         string  `json:"fileName" binding:"required,max=200"`
	TotalBytes       int64   `json:"totalBytes" binding:"required,min=1"`
	MIMEType         string  `json:"mimeType" binding:"required,max=64"`
	ContentHash      *string `json:"contentHash,omitempty"`
	TriggerVectorize *bool   `json:"triggerVectorize,omitempty"`
}

// FileProgress 文件接收进度。
type FileProgress struct {
	FileID        string `json:"fileId"`
	Status        string `json:"status"`
	UploadedBytes int64  `json:"uploadedBytes"`
	TotalBytes    int64  `json:"totalBytes"`
}

// TaskProgress 向量化任务进度。
type TaskProgress struct {
	TaskID       *string `json:"taskId"`
	Status       string  `json:"status"`
	Stage        string  `json:"stage"`
	Progress     float64 `json:"progress"`
	ErrorCode    *string `json:"errorCode"`
	ErrorMessage *string `json:"errorMessage"`
	RetryCount   int     `json:"retryCount"`
}

// Chunk 知识分块内容。
type Chunk struct {
	ChunkIndex int    `json:"chunkIndex"`
	Content    string `json:"content"`
}

// SearchRequest 关键词检索请求。
type SearchRequest struct {
	Query string `json:"query" binding:"required,max=200"`
	TopK  int    `json:"topK"`
}

// SearchResult 知识检索结果。
type SearchResult struct {
	KnowledgeID string  `json:"knowledgeId"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
}
