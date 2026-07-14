// Package model 定义 LLM 域的 PostgreSQL 持久化实体。
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Provider 表示一个模型服务提供商。
type Provider struct {
	ID              uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name            string            `gorm:"size:100;not null;uniqueIndex"`
	Protocol        string            `gorm:"size:50;not null"`
	Status          string            `gorm:"size:20;not null"`
	APIKeyEncrypted string            `gorm:"column:api_key_encrypted;type:text;not null"`
	BaseURL         *string           `gorm:"column:base_url;size:500"`
	TimeoutSeconds  int               `gorm:"column:timeout_seconds;not null"`
	RetryCount      int               `gorm:"column:retry_count;not null"`
	CustomHeaders   map[string]string `gorm:"column:custom_headers;type:jsonb;serializer:json"`
	ProxyURL        *string           `gorm:"column:proxy_url;size:500"`
	Description     *string           `gorm:"type:text"`
	CreatedBy       *uuid.UUID        `gorm:"column:created_by;type:uuid"`
	CreatedAt       time.Time         `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time         `gorm:"column:updated_at;autoUpdateTime"`
	Models          []LLMModel        `gorm:"foreignKey:ProviderID"`
}

// TableName 返回提供商表名。
func (Provider) TableName() string { return "llm_providers" }

// LLMModel 表示一个可路由的模型配置。
type LLMModel struct {
	ID                      uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ModelID                 string                 `gorm:"column:model_id;size:100;not null"`
	DisplayName             string                 `gorm:"column:display_name;size:200;not null"`
	ProviderID              uuid.UUID              `gorm:"column:provider_id;type:uuid;not null"`
	Status                  string                 `gorm:"size:20;not null"`
	ModelTypes              pq.StringArray         `gorm:"column:model_types;type:varchar(50)[];not null"`
	Tags                    pq.StringArray         `gorm:"type:varchar(50)[]"`
	ContextLength           int                    `gorm:"column:context_length;not null"`
	MaxOutput               int                    `gorm:"column:max_output;not null"`
	SupportsStreaming       bool                   `gorm:"column:supports_streaming;not null"`
	SupportsFunctionCalling bool                   `gorm:"column:supports_function_calling;not null"`
	SupportsVision          bool                   `gorm:"column:supports_vision;not null"`
	SupportsTools           bool                   `gorm:"column:supports_tools;not null"`
	InputPricePer1K         decimal.Decimal        `gorm:"column:input_price_per_1k;type:numeric(10,6);not null"`
	OutputPricePer1K        decimal.Decimal        `gorm:"column:output_price_per_1k;type:numeric(10,6);not null"`
	CostTier                *string                `gorm:"column:cost_tier;size:20"`
	APIModelName            string                 `gorm:"column:api_model_name;size:200;not null"`
	APIVersion              *string                `gorm:"column:api_version;size:50"`
	ProviderConfig          map[string]interface{} `gorm:"column:provider_config;type:jsonb;serializer:json"`
	TotalCalls              int64                  `gorm:"column:total_calls;not null"`
	TotalInputTokens        int64                  `gorm:"column:total_input_tokens;not null"`
	TotalOutputTokens       int64                  `gorm:"column:total_output_tokens;not null"`
	TotalCost               decimal.Decimal        `gorm:"column:total_cost;type:numeric(15,2);not null"`
	AvgResponseTime         *decimal.Decimal       `gorm:"column:avg_response_time;type:numeric(10,3)"`
	ErrorRate               *decimal.Decimal       `gorm:"column:error_rate;type:numeric(5,4)"`
	Description             *string                `gorm:"type:text"`
	CreatedBy               *uuid.UUID             `gorm:"column:created_by;type:uuid"`
	CreatedAt               time.Time              `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt               time.Time              `gorm:"column:updated_at;autoUpdateTime"`
	Provider                Provider               `gorm:"foreignKey:ProviderID"`
}

// TableName 返回模型表名。
func (LLMModel) TableName() string { return "llm_models" }

// ModelCall 表示一次模型调用审计与计费流水。
type ModelCall struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ModelID        string           `gorm:"column:model_id;size:100;not null"`
	ProviderID     uuid.UUID        `gorm:"column:provider_id;type:uuid;not null"`
	AgentID        *uuid.UUID       `gorm:"column:agent_id;type:uuid"`
	ConversationID *uuid.UUID       `gorm:"column:conversation_id;type:uuid"`
	CallType       string           `gorm:"column:call_type;size:20;not null"`
	RequestTime    time.Time        `gorm:"column:request_time;not null"`
	ResponseTime   *decimal.Decimal `gorm:"column:response_time;type:numeric(10,3)"`
	Status         string           `gorm:"size:20;not null"`
	InputTokens    *int             `gorm:"column:input_tokens"`
	OutputTokens   *int             `gorm:"column:output_tokens"`
	TotalTokens    *int             `gorm:"column:total_tokens"`
	Cost           *decimal.Decimal `gorm:"type:numeric(10,6)"`
	ErrorCode      *string          `gorm:"column:error_code;size:50"`
	ErrorMessage   *string          `gorm:"column:error_message;type:text"`
	CreatedAt      time.Time        `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回模型调用表名。
func (ModelCall) TableName() string { return "llm_model_calls" }

// HasCapability 判断模型是否声明指定能力。
func (entity LLMModel) HasCapability(capability string) bool {
	for _, item := range entity.ModelTypes {
		if item == capability {
			return true
		}
	}
	return false
}

// EmbeddingDimension 从最终态 JSON 配置读取嵌入维度。
func (entity LLMModel) EmbeddingDimension() *int {
	value, ok := entity.ProviderConfig["embedding_dimension"]
	if !ok {
		return nil
	}
	switch number := value.(type) {
	case float64:
		result := int(number)
		return &result
	case int:
		result := number
		return &result
	default:
		return nil
	}
}
