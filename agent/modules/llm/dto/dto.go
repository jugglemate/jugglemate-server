// Package dto 定义 LLM 对外接口的数据契约。
package dto

import "time"

// ModelWrite 模型创建请求，Provider 创建时也复用该结构。
type ModelWrite struct {
	ModelID            string   `json:"model_id" binding:"required,max=100"`
	DisplayName        string   `json:"display_name" binding:"required,max=200"`
	ProviderID         string   `json:"provider_id,omitempty"`
	Status             string   `json:"status"`
	SupportsReasoning  bool     `json:"supports_reasoning"`
	SupportsSummary    bool     `json:"supports_summary"`
	SupportsEmbedding  bool     `json:"supports_embedding"`
	ContextLength      int      `json:"context_length" binding:"required,min=1"`
	MaxOutput          int      `json:"max_output" binding:"required,min=1"`
	EmbeddingDimension *int     `json:"embedding_dimension,omitempty"`
	InputPricePer1K    float64  `json:"input_price_per_1k" binding:"min=0"`
	OutputPricePer1K   float64  `json:"output_price_per_1k" binding:"min=0"`
	APIModelName       string   `json:"api_model_name" binding:"required,max=200"`
	Tags               []string `json:"tags"`
	Description        *string  `json:"description,omitempty"`
}

// ModelUpdate 模型更新请求。
type ModelUpdate struct {
	DisplayName        *string   `json:"display_name,omitempty"`
	Status             *string   `json:"status,omitempty"`
	SupportsReasoning  *bool     `json:"supports_reasoning,omitempty"`
	SupportsSummary    *bool     `json:"supports_summary,omitempty"`
	SupportsEmbedding  *bool     `json:"supports_embedding,omitempty"`
	ContextLength      *int      `json:"context_length,omitempty"`
	MaxOutput          *int      `json:"max_output,omitempty"`
	EmbeddingDimension *int      `json:"embedding_dimension,omitempty"`
	InputPricePer1K    *float64  `json:"input_price_per_1k,omitempty"`
	OutputPricePer1K   *float64  `json:"output_price_per_1k,omitempty"`
	APIModelName       *string   `json:"api_model_name,omitempty"`
	Tags               *[]string `json:"tags,omitempty"`
	Description        *string   `json:"description,omitempty"`
}

// ModelRead 模型读取响应。
type ModelRead struct {
	ID                 string    `json:"id"`
	ModelID            string    `json:"modelId"`
	DisplayName        string    `json:"displayName"`
	ProviderID         string    `json:"providerId"`
	ProviderName       string    `json:"providerName"`
	Status             string    `json:"status"`
	SupportsReasoning  bool      `json:"supportsReasoning"`
	SupportsSummary    bool      `json:"supportsSummary"`
	SupportsEmbedding  bool      `json:"supportsEmbedding"`
	ContextLength      int       `json:"contextLength"`
	MaxOutput          int       `json:"maxOutput"`
	EmbeddingDimension *int      `json:"embeddingDimension"`
	InputPricePer1K    float64   `json:"inputPricePer1k"`
	OutputPricePer1K   float64   `json:"outputPricePer1k"`
	CostTier           string    `json:"costTier"`
	APIModelName       string    `json:"apiModelName"`
	Tags               []string  `json:"tags"`
	Description        *string   `json:"description"`
	LinkedAgentCount   int64     `json:"linkedAgentCount"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// ProviderCreate Provider 创建请求。
type ProviderCreate struct {
	Name           string            `json:"name" binding:"required,max=100"`
	Protocol       string            `json:"protocol" binding:"required"`
	Status         string            `json:"status"`
	APIKey         string            `json:"api_key" binding:"required"`
	BaseURL        *string           `json:"base_url,omitempty"`
	TimeoutSeconds *int              `json:"timeout_seconds,omitempty"`
	RetryCount     *int              `json:"retry_count,omitempty"`
	CustomHeaders  map[string]string `json:"custom_headers,omitempty"`
	ProxyURL       *string           `json:"proxy_url,omitempty"`
	Description    *string           `json:"description,omitempty"`
	Models         []ModelWrite      `json:"models,omitempty"`
}

// ProviderUpdate Provider 显式更新请求。
type ProviderUpdate struct {
	ProviderID     string             `json:"providerId" binding:"required"`
	Name           *string            `json:"name,omitempty"`
	Protocol       *string            `json:"protocol,omitempty"`
	Status         *string            `json:"status,omitempty"`
	APIKey         *string            `json:"api_key,omitempty"`
	ReplaceAPIKey  *bool              `json:"replace_api_key,omitempty"`
	BaseURL        *string            `json:"base_url,omitempty"`
	TimeoutSeconds *int               `json:"timeout_seconds,omitempty"`
	RetryCount     *int               `json:"retry_count,omitempty"`
	CustomHeaders  *map[string]string `json:"custom_headers,omitempty"`
	ProxyURL       *string            `json:"proxy_url,omitempty"`
	Description    *string            `json:"description,omitempty"`
	Models         []ModelWrite       `json:"models,omitempty"`
}

// ProviderDelete Provider 显式删除请求。
type ProviderDelete struct {
	ProviderID string `json:"providerId" binding:"required"`
}

// ProviderRead Provider 读取响应。
type ProviderRead struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	Status         string            `json:"status"`
	IsDefault      bool              `json:"isDefault"`
	APIKeyMasked   string            `json:"api_key_masked"`
	BaseURL        *string           `json:"base_url"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	RetryCount     int               `json:"retry_count"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	ProxyURL       *string           `json:"proxy_url"`
	Description    *string           `json:"description"`
	ModelCount     int               `json:"modelCount"`
	Models         []ModelRead       `json:"models"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// DefaultModels 系统默认模型响应。
type DefaultModels struct {
	ReasoningModel *string `json:"reasoning_model"`
	SummaryModel   *string `json:"summary_model"`
	EmbeddingModel *string `json:"embedding_model"`
}

// Message 模型调用中的单条文本消息。
type Message struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content"`
}

// CallRequest 统一模型调用请求。
type CallRequest struct {
	ModelID     string                   `json:"model_id" binding:"required"`
	Messages    []Message                `json:"messages"`
	Tools       []map[string]interface{} `json:"tools"`
	Temperature *float64                 `json:"temperature,omitempty"`
	MaxTokens   *int                     `json:"max_tokens,omitempty"`
	Metadata    map[string]interface{}   `json:"metadata"`
}

// Usage 模型调用 Token 用量。
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// CallResponse 统一模型调用响应。
type CallResponse struct {
	Content        string  `json:"content"`
	Usage          Usage   `json:"usage"`
	Cost           float64 `json:"cost"`
	ResponseTimeMS int     `json:"response_time_ms"`
	FinishReason   *string `json:"finish_reason"`
}
