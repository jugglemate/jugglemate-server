// Package service 实现 LLM Provider、模型与默认配置的业务规则。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/model"
	sharedsecurity "github.com/juggleim/jugglemate-server/agent/shared/security"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const defaultProviderKey = "default_provider_id"

var defaultModelKeys = map[string]string{
	"reasoning_model": "default_reasoning_model",
	"summary_model":   "default_summary_model",
	"embedding_model": "default_embedding_model",
}

var providerTransitions = map[string]map[string]bool{
	"draft":    {"draft": true, "active": true},
	"active":   {"active": true, "disabled": true},
	"disabled": {"disabled": true, "active": true, "archived": true},
	"archived": {"archived": true},
}

var modelTransitions = map[string]map[string]bool{
	"draft":    {"draft": true, "active": true, "disabled": true},
	"active":   {"active": true, "disabled": true},
	"disabled": {"disabled": true, "active": true, "archived": true},
	"archived": {"archived": true},
}

var configurableDimensions = map[string]map[int]bool{
	"qwen-text-embedding-v2": {256: true, 512: true, 768: true, 1024: true, 1536: true},
	"qwen-text-embedding-v3": {256: true, 512: true, 768: true, 1024: true, 1536: true},
	"text-embedding-v2":      {256: true, 512: true, 768: true, 1024: true, 1536: true},
	"text-embedding-v3":      {256: true, 512: true, 768: true, 1024: true, 1536: true},
}

// Error 表示可映射为 HTTP 语义码的 LLM 业务异常。
type Error struct {
	Status  int
	Message string
}

// Error 返回业务异常消息。
func (err *Error) Error() string { return err.Message }

// RegistryService 管理 Provider、模型和系统默认模型。
type RegistryService struct {
	db         *gorm.DB
	encryption *sharedsecurity.EncryptionService
}

// NewRegistryService 创建 LLM 注册表服务。
func NewRegistryService(db *gorm.DB, encryption *sharedsecurity.EncryptionService) *RegistryService {
	return &RegistryService{db: db, encryption: encryption}
}

// ListProviders 查询 Provider 列表及其模型。
func (service *RegistryService) ListProviders(ctx context.Context, protocol string, status string) ([]dto.ProviderRead, error) {
	query := service.db.WithContext(ctx).Model(&model.Provider{}).Where("status <> ?", "archived")
	if protocol != "" {
		query = query.Where("protocol = ?", protocol)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var providers []model.Provider
	if err := query.Order("updated_at DESC").Find(&providers).Error; err != nil {
		return nil, err
	}
	defaultID, err := service.getConfigString(ctx, service.db, defaultProviderKey)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ProviderRead, 0, len(providers))
	for index := range providers {
		read, err := service.providerRead(ctx, service.db, &providers[index], defaultID)
		if err != nil {
			return nil, err
		}
		result = append(result, read)
	}
	return result, nil
}

// GetProvider 按主键读取 Provider。
func (service *RegistryService) GetProvider(ctx context.Context, providerID string) (dto.ProviderRead, error) {
	provider, err := service.findProvider(ctx, service.db, providerID)
	if err != nil {
		return dto.ProviderRead{}, err
	}
	defaultID, err := service.getConfigString(ctx, service.db, defaultProviderKey)
	if err != nil {
		return dto.ProviderRead{}, err
	}
	return service.providerRead(ctx, service.db, provider, defaultID)
}

// CreateProvider 原子创建 Provider 及最多 20 个内联模型。
//
// 简要描述：全部请求先完成状态、能力、唯一性和密钥校验，再在同一 PostgreSQL 事务内
// 写入 Provider 与 Models，防止中途失败留下没有完整模型配置的 Provider。
func (service *RegistryService) CreateProvider(ctx context.Context, payload dto.ProviderCreate) (dto.ProviderRead, error) {
	service.applyProviderCreateDefaults(&payload)
	if err := validateProvider(payload.Protocol, payload.Status, *payload.TimeoutSeconds, *payload.RetryCount); err != nil {
		return dto.ProviderRead{}, err
	}
	if len(payload.Models) > 20 {
		return dto.ProviderRead{}, validationError("models 最多允许 20 个")
	}
	for index := range payload.Models {
		if err := validateModelWrite(&payload.Models[index]); err != nil {
			return dto.ProviderRead{}, err
		}
	}
	cipherText, err := service.encryption.Encrypt(strings.TrimSpace(payload.APIKey))
	if err != nil {
		return dto.ProviderRead{}, err
	}
	var created model.Provider
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Provider{}).Where("name = ?", payload.Name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return conflictError("提供商名称已存在: " + payload.Name)
		}
		created = model.Provider{
			Name: payload.Name, Protocol: payload.Protocol, Status: payload.Status,
			APIKeyEncrypted: cipherText, BaseURL: payload.BaseURL,
			TimeoutSeconds: *payload.TimeoutSeconds, RetryCount: *payload.RetryCount,
			CustomHeaders: payload.CustomHeaders, ProxyURL: payload.ProxyURL, Description: payload.Description,
		}
		if err := tx.Create(&created).Error; err != nil {
			return mapDatabaseConflict(err, "提供商名称已存在")
		}
		seen := map[string]bool{}
		for index := range payload.Models {
			item := payload.Models[index]
			if seen[item.ModelID] {
				return conflictError("模型标识重复: " + item.ModelID)
			}
			seen[item.ModelID] = true
			if payload.Status == "draft" {
				item.Status = "draft"
			}
			if _, err := service.createModel(ctx, tx, created.ID, item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dto.ProviderRead{}, err
	}
	return service.GetProvider(ctx, created.ID.String())
}

// UpdateProvider 更新 Provider，并可在同一事务中追加模型。
func (service *RegistryService) UpdateProvider(ctx context.Context, payload dto.ProviderUpdate) (dto.ProviderRead, error) {
	if len(payload.Models) > 20 {
		return dto.ProviderRead{}, validationError("models 最多允许 20 个")
	}
	var provider model.Provider
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		found, err := service.findProvider(ctx, tx, payload.ProviderID)
		if err != nil {
			return err
		}
		provider = *found
		updates := map[string]interface{}{}
		if payload.Name != nil {
			name := strings.TrimSpace(*payload.Name)
			if name == "" || len(name) > 100 {
				return validationError("name 长度必须为 1-100")
			}
			updates["name"] = name
		}
		if payload.Protocol != nil {
			if !validProtocol(*payload.Protocol) {
				return validationError("不支持的 Provider protocol")
			}
			updates["protocol"] = *payload.Protocol
		}
		if payload.Status != nil {
			if !providerTransitions[provider.Status][*payload.Status] {
				return validationError(fmt.Sprintf("非法状态迁移: %s -> %s", provider.Status, *payload.Status))
			}
			updates["status"] = *payload.Status
		}
		if payload.TimeoutSeconds != nil {
			if *payload.TimeoutSeconds < 10 || *payload.TimeoutSeconds > 300 {
				return validationError("timeout_seconds 必须为 10-300")
			}
			updates["timeout_seconds"] = *payload.TimeoutSeconds
		}
		if payload.RetryCount != nil {
			if *payload.RetryCount < 0 || *payload.RetryCount > 5 {
				return validationError("retry_count 必须为 0-5")
			}
			updates["retry_count"] = *payload.RetryCount
		}
		if payload.BaseURL != nil {
			updates["base_url"] = payload.BaseURL
		}
		if payload.ProxyURL != nil {
			updates["proxy_url"] = payload.ProxyURL
		}
		if payload.Description != nil {
			updates["description"] = payload.Description
		}
		if payload.CustomHeaders != nil {
			updates["custom_headers"] = *payload.CustomHeaders
		}
		replace := payload.ReplaceAPIKey != nil && *payload.ReplaceAPIKey
		if replace {
			if payload.APIKey == nil || strings.TrimSpace(*payload.APIKey) == "" {
				return validationError("replace_api_key=true 时，必须传入非空 api_key")
			}
			cipherText, err := service.encryption.Encrypt(strings.TrimSpace(*payload.APIKey))
			if err != nil {
				return err
			}
			updates["api_key_encrypted"] = cipherText
		}
		if len(updates) > 0 {
			if err := tx.Model(&provider).Updates(updates).Error; err != nil {
				return mapDatabaseConflict(err, "提供商名称已存在")
			}
		}
		for index := range payload.Models {
			item := payload.Models[index]
			if err := validateModelWrite(&item); err != nil {
				return err
			}
			if payload.Status != nil && *payload.Status == "draft" {
				item.Status = "draft"
			}
			if _, err := service.createModel(ctx, tx, provider.ID, item); err != nil {
				return err
			}
		}
		return tx.First(&provider, "id = ?", provider.ID).Error
	})
	if err != nil {
		return dto.ProviderRead{}, err
	}
	return service.GetProvider(ctx, provider.ID.String())
}

// DeleteProvider 软删除 Provider 并级联归档模型。
func (service *RegistryService) DeleteProvider(ctx context.Context, providerID string) error {
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		defaultID, err := service.getConfigString(ctx, tx, defaultProviderKey)
		if err != nil {
			return err
		}
		if defaultID != nil && *defaultID == providerID {
			return conflictError("默认 Provider 不允许删除，请先切换默认")
		}
		provider, err := service.findProvider(ctx, tx, providerID)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.LLMModel{}).Where("provider_id = ? AND status <> ?", provider.ID, "archived").Updates(map[string]interface{}{"status": "archived", "updated_at": gorm.Expr("now()")}).Error; err != nil {
			return err
		}
		return tx.Model(provider).Updates(map[string]interface{}{"status": "archived", "updated_at": gorm.Expr("now()")}).Error
	})
}

// SetDefaultProvider 设置默认 Provider 并按成本、上下文自动选择默认模型。
func (service *RegistryService) SetDefaultProvider(ctx context.Context, providerID string) (map[string]interface{}, error) {
	selected := map[string]*string{"reasoning_model": nil, "summary_model": nil, "embedding_model": nil}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		provider, err := service.findProvider(ctx, tx, providerID)
		if err != nil {
			return err
		}
		if provider.Status != "active" {
			return validationError("仅 active 状态 Provider 可设置为默认")
		}
		if err := service.upsertConfig(ctx, tx, defaultProviderKey, providerID, "llm-default-provider", "系统默认 Provider 配置"); err != nil {
			return err
		}
		var models []model.LLMModel
		if err := tx.Where("provider_id = ? AND status = ?", provider.ID, "active").Find(&models).Error; err != nil {
			return err
		}
		for _, capability := range []string{"reasoning", "summary", "embedding"} {
			candidates := make([]model.LLMModel, 0)
			for _, item := range models {
				if item.HasCapability(capability) {
					candidates = append(candidates, item)
				}
			}
			sort.SliceStable(candidates, func(i, j int) bool {
				left, right := costRank(candidates[i]), costRank(candidates[j])
				if left == right {
					return candidates[i].ContextLength > candidates[j].ContextLength
				}
				return left < right
			})
			if len(candidates) > 0 {
				value := candidates[0].ModelID
				selected[capability+"_model"] = &value
				if err := service.upsertConfig(ctx, tx, defaultModelKeys[capability+"_model"], value, "llm-default-model", "系统默认模型配置"); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	count := 0
	output := map[string]interface{}{}
	for key, value := range selected {
		if value != nil {
			output[key] = *value
			count++
		} else {
			output[key] = nil
		}
	}
	message := "该提供商无可用模型，本次仅更新默认 Provider，保留原默认模型配置"
	if count > 0 {
		message = fmt.Sprintf("已自动选择 %d 个默认模型（优先低成本）", count)
	}
	return map[string]interface{}{"success": true, "selected_defaults": output, "message": message}, nil
}

// ListModels 查询模型列表。
func (service *RegistryService) ListModels(ctx context.Context, providerID string, status string, capability string) ([]dto.ModelRead, error) {
	query := service.db.WithContext(ctx).Model(&model.LLMModel{}).Preload("Provider")
	if providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if capability != "" {
		query = query.Where("? = ANY(model_types)", capability)
	}
	var entities []model.LLMModel
	if err := query.Order("updated_at DESC").Find(&entities).Error; err != nil {
		return nil, err
	}
	result := make([]dto.ModelRead, 0, len(entities))
	for index := range entities {
		read, err := service.modelRead(ctx, service.db, &entities[index])
		if err != nil {
			return nil, err
		}
		result = append(result, read)
	}
	return result, nil
}

// GetModel 按模型主键查询详情。
func (service *RegistryService) GetModel(ctx context.Context, modelPK string) (dto.ModelRead, error) {
	entity, err := service.findModel(ctx, service.db, modelPK)
	if err != nil {
		return dto.ModelRead{}, err
	}
	return service.modelRead(ctx, service.db, entity)
}

// CreateModel 创建独立模型。
func (service *RegistryService) CreateModel(ctx context.Context, payload dto.ModelWrite) (dto.ModelRead, error) {
	providerID, err := uuid.Parse(payload.ProviderID)
	if err != nil {
		return dto.ModelRead{}, validationError("provider_id 格式非法")
	}
	if err := validateModelWrite(&payload); err != nil {
		return dto.ModelRead{}, err
	}
	provider, err := service.findProvider(ctx, service.db, providerID.String())
	if err != nil {
		return dto.ModelRead{}, err
	}
	if provider.Status == "archived" {
		return dto.ModelRead{}, validationError("提供商已归档，不可绑定模型")
	}
	entity, err := service.createModel(ctx, service.db, providerID, payload)
	if err != nil {
		return dto.ModelRead{}, err
	}
	return service.modelRead(ctx, service.db, entity)
}

// UpdateModel 更新模型及能力配置。
func (service *RegistryService) UpdateModel(ctx context.Context, modelPK string, payload dto.ModelUpdate) (dto.ModelRead, error) {
	entity, err := service.findModel(ctx, service.db, modelPK)
	if err != nil {
		return dto.ModelRead{}, err
	}
	read := modelWriteFromEntity(entity)
	updates := map[string]interface{}{}
	if payload.DisplayName != nil {
		read.DisplayName = *payload.DisplayName
		updates["display_name"] = *payload.DisplayName
	}
	if payload.Status != nil {
		if !modelTransitions[entity.Status][*payload.Status] {
			return dto.ModelRead{}, validationError(fmt.Sprintf("非法状态迁移: %s -> %s", entity.Status, *payload.Status))
		}
		read.Status = *payload.Status
		updates["status"] = *payload.Status
	}
	if payload.SupportsReasoning != nil {
		read.SupportsReasoning = *payload.SupportsReasoning
	}
	if payload.SupportsSummary != nil {
		read.SupportsSummary = *payload.SupportsSummary
	}
	if payload.SupportsEmbedding != nil {
		read.SupportsEmbedding = *payload.SupportsEmbedding
	}
	if payload.ContextLength != nil {
		read.ContextLength = *payload.ContextLength
		updates["context_length"] = *payload.ContextLength
	}
	if payload.MaxOutput != nil {
		read.MaxOutput = *payload.MaxOutput
		updates["max_output"] = *payload.MaxOutput
	}
	if payload.EmbeddingDimension != nil {
		read.EmbeddingDimension = payload.EmbeddingDimension
	}
	if payload.InputPricePer1K != nil {
		read.InputPricePer1K = *payload.InputPricePer1K
		updates["input_price_per_1k"] = decimal.NewFromFloat(*payload.InputPricePer1K)
	}
	if payload.OutputPricePer1K != nil {
		read.OutputPricePer1K = *payload.OutputPricePer1K
		updates["output_price_per_1k"] = decimal.NewFromFloat(*payload.OutputPricePer1K)
	}
	if payload.APIModelName != nil {
		read.APIModelName = *payload.APIModelName
		updates["api_model_name"] = *payload.APIModelName
	}
	if payload.Tags != nil {
		read.Tags = *payload.Tags
		updates["tags"] = pq.StringArray(*payload.Tags)
	}
	if payload.Description != nil {
		read.Description = payload.Description
		updates["description"] = payload.Description
	}
	if err := validateModelWrite(&read); err != nil {
		return dto.ModelRead{}, err
	}
	updates["model_types"] = modelTypes(read)
	updates["cost_tier"] = calculateCostTier(read.InputPricePer1K, read.OutputPricePer1K)
	config := entity.ProviderConfig
	if config == nil {
		config = map[string]interface{}{}
	}
	if read.SupportsEmbedding && read.EmbeddingDimension != nil {
		config["embedding_dimension"] = *read.EmbeddingDimension
	} else {
		delete(config, "embedding_dimension")
	}
	updates["provider_config"] = config
	if err := service.db.WithContext(ctx).Model(entity).Updates(updates).Error; err != nil {
		return dto.ModelRead{}, err
	}
	return service.GetModel(ctx, modelPK)
}

// DeleteModel 检查默认配置和 Agent 引用后软删除模型。
func (service *RegistryService) DeleteModel(ctx context.Context, modelPK string) error {
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entity, err := service.findModel(ctx, tx, modelPK)
		if err != nil {
			return err
		}
		for _, key := range defaultModelKeys {
			value, err := service.getConfigString(ctx, tx, key)
			if err != nil {
				return err
			}
			if value != nil && *value == entity.ModelID {
				return conflictError("模型已被系统默认配置引用，禁止删除: " + entity.ModelID)
			}
		}
		var count int64
		if err := tx.Table("agents").Where("llm_model = ? AND status <> ?", entity.ModelID, "archived").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return conflictError(fmt.Sprintf("模型已被 Agent 绑定，禁止删除: model_id=%s, linked_agent_count=%d", entity.ModelID, count))
		}
		return tx.Model(entity).Updates(map[string]interface{}{"status": "archived", "updated_at": gorm.Expr("now()")}).Error
	})
}

// GetDefaults 读取三类系统默认模型。
func (service *RegistryService) GetDefaults(ctx context.Context) (dto.DefaultModels, error) {
	values := dto.DefaultModels{}
	var err error
	if values.ReasoningModel, err = service.getConfigString(ctx, service.db, defaultModelKeys["reasoning_model"]); err != nil {
		return values, err
	}
	if values.SummaryModel, err = service.getConfigString(ctx, service.db, defaultModelKeys["summary_model"]); err != nil {
		return values, err
	}
	if values.EmbeddingModel, err = service.getConfigString(ctx, service.db, defaultModelKeys["embedding_model"]); err != nil {
		return values, err
	}
	return values, nil
}

// SetDefaults 校验并更新请求中显式出现的默认模型字段。
func (service *RegistryService) SetDefaults(ctx context.Context, updates map[string]*string) (dto.DefaultModels, error) {
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for field, value := range updates {
			key, ok := defaultModelKeys[field]
			if !ok {
				return validationError("不支持的默认模型字段: " + field)
			}
			if value != nil {
				var entity model.LLMModel
				if err := tx.Where("model_id = ? AND status <> ?", *value, "archived").First(&entity).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return validationError("默认模型不存在: " + *value)
					}
					return err
				}
				if entity.Status != "active" {
					return validationError("默认模型必须为 active 状态: " + *value)
				}
				capability := strings.TrimSuffix(field, "_model")
				if !entity.HasCapability(capability) {
					return validationError("默认模型能力不匹配: " + field)
				}
			}
			if err := service.upsertConfig(ctx, tx, key, value, "llm-default-model", "系统默认模型配置: "+field); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dto.DefaultModels{}, err
	}
	return service.GetDefaults(ctx)
}

func (service *RegistryService) createModel(ctx context.Context, db *gorm.DB, providerID uuid.UUID, payload dto.ModelWrite) (*model.LLMModel, error) {
	if err := validateModelWrite(&payload); err != nil {
		return nil, err
	}
	config := map[string]interface{}{}
	if payload.SupportsEmbedding {
		dimension := payload.EmbeddingDimension
		if dimension == nil && configurableDimensions[payload.ModelID] != nil {
			fallback := 1024
			dimension = &fallback
		}
		if dimension != nil {
			config["embedding_dimension"] = *dimension
		}
	}
	costTier := calculateCostTier(payload.InputPricePer1K, payload.OutputPricePer1K)
	entity := &model.LLMModel{
		ModelID: payload.ModelID, DisplayName: payload.DisplayName, ProviderID: providerID,
		Status: payload.Status, ModelTypes: modelTypes(payload), Tags: pq.StringArray(payload.Tags),
		ContextLength: payload.ContextLength, MaxOutput: payload.MaxOutput, SupportsStreaming: true,
		InputPricePer1K: decimal.NewFromFloat(payload.InputPricePer1K), OutputPricePer1K: decimal.NewFromFloat(payload.OutputPricePer1K),
		CostTier: &costTier, APIModelName: payload.APIModelName, ProviderConfig: config,
		TotalCost: decimal.Zero, Description: payload.Description,
	}
	if err := db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, mapDatabaseConflict(err, "该提供商下已存在模型标识: "+payload.ModelID)
	}
	return entity, nil
}

func validateProvider(protocol string, status string, timeout int, retries int) error {
	if !validProtocol(protocol) {
		return validationError("不支持的 Provider protocol")
	}
	if _, ok := providerTransitions[status]; !ok {
		return validationError("不支持的 Provider status")
	}
	if timeout < 10 || timeout > 300 {
		return validationError("timeout_seconds 必须为 10-300")
	}
	if retries < 0 || retries > 5 {
		return validationError("retry_count 必须为 0-5")
	}
	return nil
}

func validProtocol(value string) bool {
	return value == "anthropic" || value == "openai" || value == "openai-compatible"
}

func validateModelWrite(payload *dto.ModelWrite) error {
	if strings.TrimSpace(payload.ModelID) == "" || strings.TrimSpace(payload.DisplayName) == "" || strings.TrimSpace(payload.APIModelName) == "" {
		return validationError("model_id、display_name、api_model_name 不能为空")
	}
	if payload.Status == "" {
		payload.Status = "draft"
	}
	if _, ok := modelTransitions[payload.Status]; !ok {
		return validationError("不支持的模型状态")
	}
	if !payload.SupportsReasoning && !payload.SupportsSummary && !payload.SupportsEmbedding {
		return validationError("模型至少需要声明一种能力（推理/摘要/嵌入）")
	}
	if payload.ContextLength < 1 || payload.MaxOutput < 1 || payload.MaxOutput > payload.ContextLength {
		return validationError("max_output 必须大于 0 且不能大于 context_length")
	}
	if payload.InputPricePer1K < 0 || payload.OutputPricePer1K < 0 {
		return validationError("模型价格不能为负数")
	}
	if payload.SupportsEmbedding {
		if payload.EmbeddingDimension == nil && configurableDimensions[payload.ModelID] == nil {
			return validationError("嵌入模型必须配置 embedding_dimension: " + payload.ModelID)
		}
		if payload.EmbeddingDimension != nil {
			if *payload.EmbeddingDimension < 256 || *payload.EmbeddingDimension > 4096 {
				return validationError("embedding_dimension 必须为 256-4096")
			}
			if supported := configurableDimensions[payload.ModelID]; supported != nil && !supported[*payload.EmbeddingDimension] {
				return validationError("嵌入模型不支持指定维度")
			}
		}
	} else if payload.EmbeddingDimension != nil {
		return validationError("非嵌入模型不允许设置 embedding_dimension")
	}
	return nil
}

func modelTypes(payload dto.ModelWrite) pq.StringArray {
	result := pq.StringArray{}
	if payload.SupportsReasoning {
		result = append(result, "reasoning")
	}
	if payload.SupportsSummary {
		result = append(result, "summary")
	}
	if payload.SupportsEmbedding {
		result = append(result, "embedding")
	}
	return result
}

func calculateCostTier(input float64, output float64) string {
	value := input
	if output > value {
		value = output
	}
	if value <= 0.003 {
		return "low"
	}
	if value <= 0.02 {
		return "medium"
	}
	return "high"
}

func costRank(entity model.LLMModel) int {
	if entity.CostTier == nil {
		return 2
	}
	switch *entity.CostTier {
	case "low":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func (service *RegistryService) findProvider(ctx context.Context, db *gorm.DB, providerID string) (*model.Provider, error) {
	id, err := uuid.Parse(providerID)
	if err != nil {
		return nil, validationError("provider_id 格式非法")
	}
	var entity model.Provider
	if err := db.WithContext(ctx).Where("id = ? AND status <> ?", id, "archived").First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFoundError("提供商不存在: " + providerID)
		}
		return nil, err
	}
	return &entity, nil
}

func (service *RegistryService) findModel(ctx context.Context, db *gorm.DB, modelPK string) (*model.LLMModel, error) {
	id, err := uuid.Parse(modelPK)
	if err != nil {
		return nil, validationError("model_pk 格式非法")
	}
	var entity model.LLMModel
	if err := db.WithContext(ctx).Preload("Provider").Where("id = ? AND status <> ?", id, "archived").First(&entity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, notFoundError("模型不存在: " + modelPK)
		}
		return nil, err
	}
	return &entity, nil
}

func (service *RegistryService) providerRead(ctx context.Context, db *gorm.DB, entity *model.Provider, defaultID *string) (dto.ProviderRead, error) {
	plain, err := service.encryption.Decrypt(entity.APIKeyEncrypted)
	if err != nil {
		return dto.ProviderRead{}, fmt.Errorf("读取 Provider 密钥失败: %w", err)
	}
	var models []model.LLMModel
	if err := db.WithContext(ctx).Preload("Provider").Where("provider_id = ?", entity.ID).Order("updated_at DESC").Find(&models).Error; err != nil {
		return dto.ProviderRead{}, err
	}
	reads := make([]dto.ModelRead, 0, len(models))
	for index := range models {
		read, err := service.modelRead(ctx, db, &models[index])
		if err != nil {
			return dto.ProviderRead{}, err
		}
		reads = append(reads, read)
	}
	return dto.ProviderRead{ID: entity.ID.String(), Name: entity.Name, Protocol: entity.Protocol, Status: entity.Status, IsDefault: defaultID != nil && *defaultID == entity.ID.String(), APIKeyMasked: service.encryption.Mask(plain), BaseURL: entity.BaseURL, TimeoutSeconds: entity.TimeoutSeconds, RetryCount: entity.RetryCount, CustomHeaders: entity.CustomHeaders, ProxyURL: entity.ProxyURL, Description: entity.Description, ModelCount: len(reads), Models: reads, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}, nil
}

func (service *RegistryService) modelRead(ctx context.Context, db *gorm.DB, entity *model.LLMModel) (dto.ModelRead, error) {
	if entity.Provider.Name == "" {
		if err := db.WithContext(ctx).First(&entity.Provider, "id = ?", entity.ProviderID).Error; err != nil {
			return dto.ModelRead{}, err
		}
	}
	var count int64
	if err := db.WithContext(ctx).Table("agents").Where("llm_model = ? AND status <> ?", entity.ModelID, "archived").Count(&count).Error; err != nil {
		return dto.ModelRead{}, err
	}
	costTier := "high"
	if entity.CostTier != nil {
		costTier = *entity.CostTier
	}
	return dto.ModelRead{ID: entity.ID.String(), ModelID: entity.ModelID, DisplayName: entity.DisplayName, ProviderID: entity.ProviderID.String(), ProviderName: entity.Provider.Name, Status: entity.Status, SupportsReasoning: entity.HasCapability("reasoning"), SupportsSummary: entity.HasCapability("summary"), SupportsEmbedding: entity.HasCapability("embedding"), ContextLength: entity.ContextLength, MaxOutput: entity.MaxOutput, EmbeddingDimension: entity.EmbeddingDimension(), InputPricePer1K: entity.InputPricePer1K.InexactFloat64(), OutputPricePer1K: entity.OutputPricePer1K.InexactFloat64(), CostTier: costTier, APIModelName: entity.APIModelName, Tags: []string(entity.Tags), Description: entity.Description, LinkedAgentCount: count, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}, nil
}

func modelWriteFromEntity(entity *model.LLMModel) dto.ModelWrite {
	return dto.ModelWrite{ModelID: entity.ModelID, DisplayName: entity.DisplayName, ProviderID: entity.ProviderID.String(), Status: entity.Status, SupportsReasoning: entity.HasCapability("reasoning"), SupportsSummary: entity.HasCapability("summary"), SupportsEmbedding: entity.HasCapability("embedding"), ContextLength: entity.ContextLength, MaxOutput: entity.MaxOutput, EmbeddingDimension: entity.EmbeddingDimension(), InputPricePer1K: entity.InputPricePer1K.InexactFloat64(), OutputPricePer1K: entity.OutputPricePer1K.InexactFloat64(), APIModelName: entity.APIModelName, Tags: []string(entity.Tags), Description: entity.Description}
}

func (service *RegistryService) getConfigString(ctx context.Context, db *gorm.DB, key string) (*string, error) {
	var raw string
	err := db.WithContext(ctx).Raw("SELECT config_value::text FROM system_config WHERE config_key = ?", key).Scan(&raw).Error
	if err != nil {
		return nil, err
	}
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var result string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("系统配置 %s 类型错误: %w", key, err)
	}
	return &result, nil
}

func (service *RegistryService) upsertConfig(ctx context.Context, db *gorm.DB, key string, value interface{}, category string, description string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Exec(`INSERT INTO system_config(config_key, config_value, config_category, description, created_at, updated_at) VALUES (?, CAST(? AS jsonb), ?, ?, now(), now()) ON CONFLICT(config_key) DO UPDATE SET config_value=EXCLUDED.config_value, config_category=EXCLUDED.config_category, description=EXCLUDED.description, updated_at=now()`, key, string(raw), category, description).Error
}

func (service *RegistryService) applyProviderCreateDefaults(payload *dto.ProviderCreate) {
	if payload.Status == "" {
		payload.Status = "draft"
	}
	if payload.TimeoutSeconds == nil {
		value := 60
		payload.TimeoutSeconds = &value
	}
	if payload.RetryCount == nil {
		value := 3
		payload.RetryCount = &value
	}
}

func validationError(message string) error {
	return &Error{Status: http.StatusBadRequest, Message: message}
}
func notFoundError(message string) error {
	return &Error{Status: http.StatusNotFound, Message: message}
}
func conflictError(message string) error {
	return &Error{Status: http.StatusConflict, Message: message}
}
func mapDatabaseConflict(err error, message string) error {
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
		return conflictError(message)
	}
	return err
}
