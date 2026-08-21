package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/adapter"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/model"
	sharedsecurity "github.com/juggleim/jugglemate-server/agent/shared/security"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CallService 负责模型路由、协议调用、成本计算与调用流水落库。
type CallService struct {
	db         *gorm.DB
	encryption *sharedsecurity.EncryptionService
	gateway    *adapter.HTTPGateway
}

// EmbeddingCallResult 表示尚未写入调用事实表的 Embedding 调用结果。
//
// 调用方必须在后续业务事务中持久化 Token 事实；该结构主要供知识库向量化与
// Credit 结算共用同一事务，避免 Token 事实和钱包账务分裂。
type EmbeddingCallResult struct {
	Embeddings [][]float32
	Usage      dto.Usage
	ModelID    string
	ProviderID uuid.UUID
}

// NewCallService 创建统一模型调用服务。
func NewCallService(db *gorm.DB, encryption *sharedsecurity.EncryptionService, gateway *adapter.HTTPGateway) *CallService {
	return &CallService{db: db, encryption: encryption, gateway: gateway}
}

// Embed 使用指定或系统默认的 active 嵌入模型生成文本向量。
func (service *CallService) Embed(ctx context.Context, requestedModelID string, texts []string) ([][]float32, dto.Usage, string, error) {
	return service.EmbedWithMetadata(ctx, requestedModelID, texts, nil)
}

// EmbedForSettlement 生成向量但不持久化成功调用事实，供调用方执行原子计费结算。
//
// 简要描述：外部模型调用无法参与数据库事务，因此本方法只返回调用结果；调用方
// 必须在成功后将 Token 事实与账务流水放入同一数据库事务，失败时不得单独落事实。
func (service *CallService) EmbedForSettlement(ctx context.Context, requestedModelID string, texts []string) (EmbeddingCallResult, error) {
	result, entity, _, err := service.executeEmbedding(ctx, requestedModelID, texts)
	if err != nil {
		return EmbeddingCallResult{}, err
	}
	return EmbeddingCallResult{Embeddings: result.Embeddings, Usage: result.Usage, ModelID: entity.ModelID, ProviderID: entity.ProviderID}, nil
}

// EmbedWithMetadata 生成文本向量，并将调用类型及业务关联信息写入模型调用事实表。
func (service *CallService) EmbedWithMetadata(ctx context.Context, requestedModelID string, texts []string, metadata map[string]interface{}) ([][]float32, dto.Usage, string, error) {
	result, entity, elapsed, err := service.executeEmbedding(ctx, requestedModelID, texts)
	if err != nil {
		if entity != nil {
			if metadata == nil {
				metadata = map[string]interface{}{}
			}
			metadata["call_type"] = "embedding"
			_ = service.record(ctx, entity, &entity.Provider, metadata, dto.Usage{}, 0, elapsed, "failed", err)
		}
		return nil, dto.Usage{}, "", err
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["call_type"] = "embedding"
	if err := service.record(ctx, entity, &entity.Provider, metadata, result.Usage, calculateCost(entity, result.Usage), elapsed, "success", nil); err != nil {
		return nil, dto.Usage{}, "", err
	}
	return result.Embeddings, result.Usage, entity.ModelID, nil
}

func (service *CallService) executeEmbedding(ctx context.Context, requestedModelID string, texts []string) (adapter.EmbeddingResult, *model.LLMModel, int, error) {
	if len(texts) == 0 {
		return adapter.EmbeddingResult{}, nil, 0, validationError("嵌入文本不能为空")
	}
	modelID := strings.TrimSpace(requestedModelID)
	if modelID == "" {
		var raw string
		_ = service.db.WithContext(ctx).Raw(`SELECT config_value #>> '{}' FROM system_config WHERE config_key='default_embedding_model'`).Scan(&raw).Error
		modelID = raw
	}
	var entity model.LLMModel
	query := service.db.WithContext(ctx).Preload("Provider").Where("status = ? AND ? = ANY(model_types)", "active", "embedding")
	if modelID != "" {
		query = query.Where("model_id = ?", modelID)
	}
	if err := query.Order("updated_at DESC").First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return adapter.EmbeddingResult{}, nil, 0, validationError("未找到可用的 Embedding 模型")
		}
		return adapter.EmbeddingResult{}, nil, 0, err
	}
	if entity.Provider.Status != "active" {
		return adapter.EmbeddingResult{}, nil, 0, validationError("Embedding 模型 Provider 不是 active 状态")
	}
	apiKey, err := service.encryption.Decrypt(entity.Provider.APIKeyEncrypted)
	if err != nil {
		return adapter.EmbeddingResult{}, nil, 0, err
	}
	started := time.Now()
	result, err := service.gateway.Embed(ctx, runtimeFromProvider(entity.Provider, apiKey, entity.APIModelName), texts, entity.EmbeddingDimension())
	elapsed := int(time.Since(started).Milliseconds())
	if err != nil {
		return adapter.EmbeddingResult{}, &entity, elapsed, &Error{Status: http.StatusBadGateway, Message: "调用 Embedding 模型失败: " + err.Error()}
	}
	return result, &entity, elapsed, nil
}

// HasEmbeddingModel 检查是否存在 Provider 同时可用的 active 嵌入模型。
func (service *CallService) HasEmbeddingModel(ctx context.Context) (bool, error) {
	var count int64
	err := service.db.WithContext(ctx).Table("llm_models m").Joins("JOIN llm_providers p ON p.id=m.provider_id").Where("m.status=? AND p.status=? AND ? = ANY(m.model_types)", "active", "active", "embedding").Count(&count).Error
	return count > 0, err
}

// Call 执行非流式模型调用并同步记录调用指标。
func (service *CallService) Call(ctx context.Context, request dto.CallRequest) (dto.CallResponse, error) {
	entity, provider, runtime, err := service.resolve(ctx, request.ModelID)
	if err != nil {
		return dto.CallResponse{}, err
	}
	started := time.Now()
	result, callErr := service.gateway.Call(ctx, runtime, request)
	elapsed := int(time.Since(started).Milliseconds())
	if callErr != nil {
		_ = service.record(ctx, entity, provider, request.Metadata, dto.Usage{}, 0, elapsed, "failed", callErr)
		return dto.CallResponse{}, &Error{Status: http.StatusBadGateway, Message: "调用模型失败: " + callErr.Error()}
	}
	cost := calculateCost(entity, result.Usage)
	if err := service.record(ctx, entity, provider, request.Metadata, result.Usage, cost, elapsed, "success", nil); err != nil {
		return dto.CallResponse{}, err
	}
	return dto.CallResponse{Content: result.Content, Usage: result.Usage, Cost: cost, ResponseTimeMS: elapsed, FinishReason: result.FinishReason}, nil
}

// Stream 建立流式模型调用并在结束后持久化聚合指标。
func (service *CallService) Stream(ctx context.Context, request dto.CallRequest) (<-chan adapter.StreamChunk, error) {
	entity, provider, runtime, err := service.resolve(ctx, request.ModelID)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	upstream, err := service.gateway.Stream(ctx, runtime, request)
	if err != nil {
		_ = service.record(ctx, entity, provider, request.Metadata, dto.Usage{}, 0, int(time.Since(started).Milliseconds()), "failed", err)
		return nil, &Error{Status: http.StatusBadGateway, Message: "调用模型失败: " + err.Error()}
	}
	output := make(chan adapter.StreamChunk)
	go func() {
		defer close(output)
		usage := dto.Usage{}
		var streamErr error
		for chunk := range upstream {
			if chunk.Usage != nil {
				if chunk.Usage.InputTokens > 0 {
					usage.InputTokens = chunk.Usage.InputTokens
				}
				if chunk.Usage.OutputTokens > 0 {
					usage.OutputTokens = chunk.Usage.OutputTokens
				}
				if chunk.Usage.TotalTokens > 0 {
					usage.TotalTokens = chunk.Usage.TotalTokens
				}
			}
			if chunk.Err != nil {
				streamErr = chunk.Err
			}
			output <- chunk
		}
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		status := "success"
		if streamErr != nil {
			status = "failed"
		}
		_ = service.record(context.WithoutCancel(ctx), entity, provider, request.Metadata, usage, calculateCost(entity, usage), int(time.Since(started).Milliseconds()), status, streamErr)
	}()
	return output, nil
}

// TestProviderConnection 解密 Provider 凭据并执行连接测试。
func (service *CallService) TestProviderConnection(ctx context.Context, providerID string) (adapter.ConnectionResult, error) {
	id, err := uuid.Parse(providerID)
	if err != nil {
		return adapter.ConnectionResult{}, validationError("provider_id 格式非法")
	}
	var provider model.Provider
	if err := service.db.WithContext(ctx).Where("id = ? AND status <> ?", id, "archived").First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return adapter.ConnectionResult{}, notFoundError("提供商不存在: " + providerID)
		}
		return adapter.ConnectionResult{}, err
	}
	apiKey, err := service.encryption.Decrypt(provider.APIKeyEncrypted)
	if err != nil {
		return adapter.ConnectionResult{}, err
	}
	return service.gateway.TestConnection(ctx, runtimeFromProvider(provider, apiKey, "")), nil
}

func (service *CallService) resolve(ctx context.Context, modelID string) (*model.LLMModel, *model.Provider, adapter.Runtime, error) {
	var entity model.LLMModel
	err := service.db.WithContext(ctx).Preload("Provider").Where("model_id = ? AND status <> ?", modelID, "archived").Order("updated_at DESC").First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, adapter.Runtime{}, validationError("模型不存在: " + modelID)
		}
		return nil, nil, adapter.Runtime{}, err
	}
	if entity.Status != "active" {
		return nil, nil, adapter.Runtime{}, validationError("模型不是 active 状态: " + modelID)
	}
	if entity.Provider.Status != "active" {
		return nil, nil, adapter.Runtime{}, validationError("模型 Provider 不是 active 状态")
	}
	apiKey, err := service.encryption.Decrypt(entity.Provider.APIKeyEncrypted)
	if err != nil {
		return nil, nil, adapter.Runtime{}, fmt.Errorf("读取模型凭据失败: %w", err)
	}
	return &entity, &entity.Provider, runtimeFromProvider(entity.Provider, apiKey, entity.APIModelName), nil
}

func runtimeFromProvider(provider model.Provider, apiKey string, apiModelName string) adapter.Runtime {
	baseURL, proxyURL := "", ""
	if provider.BaseURL != nil {
		baseURL = *provider.BaseURL
	}
	if provider.ProxyURL != nil {
		proxyURL = *provider.ProxyURL
	}
	return adapter.Runtime{Protocol: provider.Protocol, APIKey: apiKey, BaseURL: baseURL, TimeoutSeconds: provider.TimeoutSeconds, ProxyURL: proxyURL, APIModelName: apiModelName, CustomHeaders: provider.CustomHeaders}
}

func (service *CallService) record(ctx context.Context, entity *model.LLMModel, provider *model.Provider, metadata map[string]interface{}, usage dto.Usage, cost float64, elapsedMS int, status string, callErr error) error {
	callType := normalizeCallType(metadataString(metadata, "call_type"))
	requestTime := time.Now().Add(-time.Duration(elapsedMS) * time.Millisecond)
	responseSeconds := decimal.NewFromInt(int64(elapsedMS)).Div(decimal.NewFromInt(1000))
	costDecimal := decimal.NewFromFloat(cost)
	record := model.ModelCall{ModelID: entity.ModelID, ProviderID: provider.ID, AgentID: metadataUUID(metadata, "agent_id"), ConversationID: metadataUUID(metadata, "conversation_id"), CallType: callType, RequestTime: requestTime, ResponseTime: &responseSeconds, Status: status, InputTokens: &usage.InputTokens, OutputTokens: &usage.OutputTokens, TotalTokens: &usage.TotalTokens, Cost: &costDecimal}
	if callErr != nil {
		code, message := "UPSTREAM_ERROR", callErr.Error()
		if strings.Contains(strings.ToLower(message), "timeout") {
			record.Status = "timeout"
			code = "UPSTREAM_TIMEOUT"
		}
		record.ErrorCode = &code
		record.ErrorMessage = &message
	}
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if status == "success" {
			return tx.Model(&model.LLMModel{}).Where("id = ?", entity.ID).Updates(map[string]interface{}{
				"total_calls": gorm.Expr("total_calls + 1"), "total_input_tokens": gorm.Expr("total_input_tokens + ?", usage.InputTokens), "total_output_tokens": gorm.Expr("total_output_tokens + ?", usage.OutputTokens), "total_cost": gorm.Expr("total_cost + ?", costDecimal),
			}).Error
		}
		return nil
	})
}

func normalizeCallType(callType string) string {
	switch callType {
	case "reasoning", "summary", "embedding", "translation":
		return callType
	default:
		return "reasoning"
	}
}

func calculateCost(entity *model.LLMModel, usage dto.Usage) float64 {
	input := entity.InputPricePer1K.Mul(decimal.NewFromInt(int64(usage.InputTokens))).Div(decimal.NewFromInt(1000))
	output := entity.OutputPricePer1K.Mul(decimal.NewFromInt(int64(usage.OutputTokens))).Div(decimal.NewFromInt(1000))
	return input.Add(output).InexactFloat64()
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}
func metadataUUID(metadata map[string]interface{}, key string) *uuid.UUID {
	value := metadataString(metadata, key)
	id, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &id
}
