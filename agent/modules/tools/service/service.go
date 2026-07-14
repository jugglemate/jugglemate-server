// Package service 实现 Tools v2 的管理、执行、决策与观测业务逻辑。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/adapter"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/model"
	sharedsecurity "github.com/juggleim/jugglemate-server/agent/shared/security"
	"gorm.io/gorm"
)

type testSnapshot struct {
	success bool
	at      time.Time
}

// Service 聚合 Tools v2 全部用例，并作为 API 层唯一入口。
type Service struct {
	db         *gorm.DB
	encryption *sharedsecurity.EncryptionService
	executor   *adapter.HTTPExecutor
	testMu     sync.RWMutex
	lastTests  map[string]testSnapshot
}

// New 创建 Tools v2 服务。
func New(db *gorm.DB, encryption *sharedsecurity.EncryptionService, executor *adapter.HTTPExecutor) *Service {
	if executor == nil {
		executor = adapter.NewHTTPExecutor()
	}
	return &Service{db: db, encryption: encryption, executor: executor, lastTests: map[string]testSnapshot{}}
}

// CreateProvider 创建工具提供方。
func (service *Service) CreateProvider(ctx context.Context, ownerID string, command dto.ProviderCreateRequest) (dto.ProviderResponse, error) {
	if !contains([]string{"plugin", "custom"}, command.ProviderType) || !contains([]string{"none", "api_key", "bearer", "basic", "oauth2_client_credentials"}, command.AuthType) {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	if command.ProviderType == "custom" && stringValue(command.ConnectionConfig["base_url"]) == "" {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	if command.Visibility == "" {
		command.Visibility = "private"
	}
	if !contains([]string{"system", "private"}, command.Visibility) {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	authSchema := command.AuthSchema
	if authSchema == nil {
		authSchema = defaultAuthSchemas[command.AuthType]
	}
	if !validateSchemaShape(authSchema) {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	var count int64
	if err := service.db.WithContext(ctx).Model(&model.Provider{}).Where("owner_id = ? AND name = ? AND deleted_at IS NULL", ownerID, command.Name).Count(&count).Error; err != nil {
		return dto.ProviderResponse{}, err
	}
	if count > 0 {
		return dto.ProviderResponse{}, businessError("409_PROVIDER_NAME_CONFLICT")
	}
	entity := model.Provider{ID: "provider-" + uuid.NewString(), OwnerID: ownerID, ProviderType: command.ProviderType, Name: command.Name, DisplayName: command.DisplayName, ConnectionConfig: nonNilMap(command.ConnectionConfig), AuthType: command.AuthType, AuthSchema: authSchema, Status: "draft", Visibility: command.Visibility}
	if err := service.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dto.ProviderResponse{}, err
	}
	return providerResponse(entity), nil
}

// GetProvider 查询当前 Owner 可见的工具提供方。
func (service *Service) GetProvider(ctx context.Context, ownerID, providerID string) (dto.ProviderResponse, error) {
	entity, err := service.visibleProvider(ctx, ownerID, providerID)
	if err != nil {
		return dto.ProviderResponse{}, err
	}
	return providerResponse(entity), nil
}

// ListProviders 分页查询 Owner 自有和 system 级工具提供方。
func (service *Service) ListProviders(ctx context.Context, ownerID, q, status, providerType string, page, pageSize int) (dto.ProviderListResponse, error) {
	page, pageSize = pageValues(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.Provider{}).Where("deleted_at IS NULL AND (owner_id = ? OR visibility = 'system')", ownerID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if providerType != "" {
		query = query.Where("provider_type = ?", providerType)
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("name ILIKE ? OR display_name ILIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ProviderListResponse{}, err
	}
	var entities []model.Provider
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.ProviderListResponse{}, err
	}
	items := make([]dto.ProviderResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, providerResponse(entity))
	}
	return dto.ProviderListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// UpdateProvider 更新当前 Owner 可见的工具提供方配置。
func (service *Service) UpdateProvider(ctx context.Context, ownerID, providerID string, command dto.ProviderUpdateRequest) (dto.ProviderResponse, error) {
	entity, err := service.visibleProvider(ctx, ownerID, providerID)
	if err != nil {
		return dto.ProviderResponse{}, err
	}
	if command.AuthSchema != nil && !validateSchemaShape(command.AuthSchema) {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	if command.Visibility != nil && !contains([]string{"system", "private"}, *command.Visibility) {
		return dto.ProviderResponse{}, businessError("400_PROVIDER_INVALID_CONFIG")
	}
	if command.DisplayName != nil {
		entity.DisplayName = command.DisplayName
	}
	if command.ConnectionConfig != nil {
		entity.ConnectionConfig = command.ConnectionConfig
	}
	if command.AuthSchema != nil {
		entity.AuthSchema = command.AuthSchema
	}
	if command.Visibility != nil {
		entity.Visibility = *command.Visibility
	}
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.ProviderResponse{}, err
	}
	return providerResponse(entity), nil
}

// DeleteProvider 在没有 active 工具定义时软删除提供方。
func (service *Service) DeleteProvider(ctx context.Context, ownerID, providerID string) error {
	entity, err := service.visibleProvider(ctx, ownerID, providerID)
	if err != nil {
		return err
	}
	var count int64
	if err := service.db.WithContext(ctx).Model(&model.Definition{}).Where("provider_id = ? AND status = 'active' AND deleted_at IS NULL", providerID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return businessError("423_PROVIDER_IN_USE")
	}
	now := time.Now().UTC()
	entity.Status = "deleted"
	entity.DeletedAt = &now
	return service.db.WithContext(ctx).Save(&entity).Error
}

// TransitionProvider 执行提供方状态机流转。
func (service *Service) TransitionProvider(ctx context.Context, ownerID, providerID, target string) (dto.ProviderResponse, error) {
	entity, err := service.visibleProvider(ctx, ownerID, providerID)
	if err != nil {
		return dto.ProviderResponse{}, err
	}
	if !allowedTransitions[entity.Status][target] {
		return dto.ProviderResponse{}, businessError("409_INVALID_STATE_TRANSITION")
	}
	entity.Status = target
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.ProviderResponse{}, err
	}
	return providerResponse(entity), nil
}

// CreateDefinition 创建 plugin 或 custom 工具定义。
func (service *Service) CreateDefinition(ctx context.Context, ownerID, toolType string, command dto.DefinitionCreateRequest) (dto.DefinitionResponse, error) {
	if _, err := service.visibleProvider(ctx, ownerID, command.ProviderID); err != nil {
		return dto.DefinitionResponse{}, err
	}
	if !validateSchemaShape(command.InputSchema) || !validateSchemaShape(command.OutputSchema) {
		return dto.DefinitionResponse{}, businessError("400_INVALID_SCHEMA")
	}
	if command.TimeoutSeconds == 0 {
		command.TimeoutSeconds = 30
	}
	if command.TimeoutSeconds < 1 || command.TimeoutSeconds > 300 {
		return dto.DefinitionResponse{}, businessError("400_INVALID_EXECUTION_CONFIG")
	}
	method := strings.ToUpper(stringValue(command.ExecutionConfig["method"]))
	path := stringValue(command.ExecutionConfig["path"])
	if toolType == "plugin" {
		if method == "" {
			method = http.MethodPost
		}
		if path == "" {
			path = stringValue(command.ExecutionConfig["entrypoint"])
		}
	}
	if method == "" || path == "" {
		return dto.DefinitionResponse{}, businessError("400_INVALID_EXECUTION_CONFIG")
	}
	command.ExecutionConfig["method"] = method
	command.ExecutionConfig["path"] = path
	var nameCount int64
	if err := service.db.WithContext(ctx).Model(&model.Definition{}).Where("provider_id = ? AND name = ? AND deleted_at IS NULL", command.ProviderID, command.Name).Count(&nameCount).Error; err != nil {
		return dto.DefinitionResponse{}, err
	}
	if nameCount > 0 {
		return dto.DefinitionResponse{}, businessError("409_TOOL_NAME_CONFLICT")
	}
	var pathCount int64
	if err := service.db.WithContext(ctx).Model(&model.Definition{}).Where("provider_id = ? AND tool_type = ? AND execution_config ->> 'method' = ? AND execution_config ->> 'path' = ? AND deleted_at IS NULL", command.ProviderID, toolType, method, path).Count(&pathCount).Error; err != nil {
		return dto.DefinitionResponse{}, err
	}
	if pathCount > 0 {
		return dto.DefinitionResponse{}, businessError("409_TOOL_PATH_CONFLICT")
	}
	risk, approval := evaluateRisk(method, path, command.Name, command.Description)
	if command.RiskLevel != nil && contains([]string{"low", "medium", "high", "critical"}, *command.RiskLevel) {
		risk = *command.RiskLevel
	}
	if risk == "high" || risk == "critical" {
		approval = true
	}
	entity := model.Definition{ID: "tooldef-" + uuid.NewString(), ProviderID: command.ProviderID, OwnerID: ownerID, ToolType: toolType, Name: command.Name, Description: command.Description, InputSchema: command.InputSchema, OutputSchema: command.OutputSchema, ExecutionConfig: command.ExecutionConfig, RiskLevel: risk, ApprovalRequired: approval, TimeoutSeconds: command.TimeoutSeconds, Status: "draft"}
	if err := service.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dto.DefinitionResponse{}, err
	}
	return definitionResponse(entity), nil
}

// ImportOpenAPI 获取并解析 JSON 格式 OpenAPI 文档中的 operations。
func (service *Service) ImportOpenAPI(ctx context.Context, command dto.OpenAPIImportRequest) (dto.OpenAPIImportResponse, error) {
	var document map[string]any
	switch spec := command.OpenAPISpec.(type) {
	case map[string]any:
		document = spec
	case string:
		if json.Unmarshal([]byte(spec), &document) != nil {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
	case nil:
		if command.OpenAPIURL == nil || *command.OpenAPIURL == "" {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, *command.OpenAPIURL, nil)
		if err != nil {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
		defer response.Body.Close()
		if response.StatusCode >= 400 {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
		content, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		if err != nil || json.Unmarshal(content, &document) != nil {
			return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
		}
	default:
		return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
	}
	operations := extractOperations(document)
	if len(operations) == 0 {
		return dto.OpenAPIImportResponse{}, businessError("400_OPENAPI_PARSE_FAILED")
	}
	return dto.OpenAPIImportResponse{Operations: operations}, nil
}

// GetDefinition 查询当前 Owner 可见的工具定义。
func (service *Service) GetDefinition(ctx context.Context, ownerID, definitionID string) (dto.DefinitionResponse, error) {
	entity, err := service.visibleDefinition(ctx, ownerID, definitionID)
	if err != nil {
		return dto.DefinitionResponse{}, err
	}
	return definitionResponse(entity), nil
}

// ListDefinitions 分页查询当前 Owner 可见的工具定义。
func (service *Service) ListDefinitions(ctx context.Context, ownerID, providerID, toolType, status, riskLevel, q string, page, pageSize int) (dto.DefinitionListResponse, error) {
	page, pageSize = pageValues(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.Definition{}).Joins("JOIN tool_providers ON tool_providers.id = tool_definitions.provider_id").Where("tool_definitions.deleted_at IS NULL AND tool_providers.deleted_at IS NULL AND (tool_providers.owner_id = ? OR tool_providers.visibility = 'system')", ownerID)
	if providerID != "" {
		query = query.Where("tool_definitions.provider_id = ?", providerID)
	}
	if toolType != "" {
		query = query.Where("tool_definitions.tool_type = ?", toolType)
	}
	if status != "" {
		query = query.Where("tool_definitions.status = ?", status)
	}
	if riskLevel != "" {
		query = query.Where("tool_definitions.risk_level = ?", riskLevel)
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("tool_definitions.name ILIKE ? OR tool_definitions.description ILIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.DefinitionListResponse{}, err
	}
	var entities []model.Definition
	if err := query.Order("tool_definitions.updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.DefinitionListResponse{}, err
	}
	items := make([]dto.DefinitionResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, definitionResponse(entity))
	}
	return dto.DefinitionListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// UpdateDefinition 更新工具定义及其执行约束。
func (service *Service) UpdateDefinition(ctx context.Context, ownerID, definitionID string, command dto.DefinitionUpdateRequest) (dto.DefinitionResponse, error) {
	entity, err := service.visibleDefinition(ctx, ownerID, definitionID)
	if err != nil {
		return dto.DefinitionResponse{}, err
	}
	if command.InputSchema != nil && !validateSchemaShape(command.InputSchema) || command.OutputSchema != nil && !validateSchemaShape(command.OutputSchema) {
		return dto.DefinitionResponse{}, businessError("400_INVALID_SCHEMA")
	}
	if command.TimeoutSeconds != nil && (*command.TimeoutSeconds < 1 || *command.TimeoutSeconds > 300) {
		return dto.DefinitionResponse{}, businessError("400_INVALID_EXECUTION_CONFIG")
	}
	if command.RiskLevel != nil && !contains([]string{"low", "medium", "high", "critical"}, *command.RiskLevel) {
		return dto.DefinitionResponse{}, businessError("400_INVALID_EXECUTION_CONFIG")
	}
	if command.ExecutionConfig != nil {
		method := strings.ToUpper(stringValue(command.ExecutionConfig["method"]))
		path := stringValue(command.ExecutionConfig["path"])
		if method == "" || path == "" {
			return dto.DefinitionResponse{}, businessError("400_INVALID_EXECUTION_CONFIG")
		}
		command.ExecutionConfig["method"] = method
		var conflict int64
		if err := service.db.WithContext(ctx).Model(&model.Definition{}).Where("provider_id = ? AND tool_type = ? AND id <> ? AND execution_config ->> 'method' = ? AND execution_config ->> 'path' = ? AND deleted_at IS NULL", entity.ProviderID, entity.ToolType, entity.ID, method, path).Count(&conflict).Error; err != nil {
			return dto.DefinitionResponse{}, err
		}
		if conflict > 0 {
			return dto.DefinitionResponse{}, businessError("409_TOOL_PATH_CONFLICT")
		}
		entity.ExecutionConfig = command.ExecutionConfig
	}
	if command.Description != nil {
		entity.Description = command.Description
	}
	if command.InputSchema != nil {
		entity.InputSchema = command.InputSchema
	}
	if command.OutputSchema != nil {
		entity.OutputSchema = command.OutputSchema
	}
	if command.RiskLevel != nil {
		entity.RiskLevel = *command.RiskLevel
	}
	if command.ApprovalRequired != nil {
		entity.ApprovalRequired = *command.ApprovalRequired
	}
	if command.TimeoutSeconds != nil {
		entity.TimeoutSeconds = *command.TimeoutSeconds
	}
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.DefinitionResponse{}, err
	}
	return definitionResponse(entity), nil
}

// DeleteDefinition 软删除工具定义。
func (service *Service) DeleteDefinition(ctx context.Context, ownerID, definitionID string) error {
	entity, err := service.visibleDefinition(ctx, ownerID, definitionID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	entity.Status = "deleted"
	entity.DeletedAt = &now
	return service.db.WithContext(ctx).Save(&entity).Error
}

func (service *Service) visibleProvider(ctx context.Context, ownerID, providerID string) (model.Provider, error) {
	var entity model.Provider
	err := service.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL AND (owner_id = ? OR visibility = 'system')", providerID, ownerID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Provider{}, businessError("404_PROVIDER_NOT_FOUND")
	}
	return entity, err
}

func (service *Service) providerByID(ctx context.Context, providerID string) (model.Provider, error) {
	var entity model.Provider
	err := service.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", providerID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Provider{}, businessError("404_PROVIDER_NOT_FOUND")
	}
	return entity, err
}

func (service *Service) visibleDefinition(ctx context.Context, ownerID, definitionID string) (model.Definition, error) {
	var entity model.Definition
	err := service.db.WithContext(ctx).Joins("JOIN tool_providers ON tool_providers.id = tool_definitions.provider_id").Where("tool_definitions.id = ? AND tool_definitions.deleted_at IS NULL AND tool_providers.deleted_at IS NULL AND (tool_providers.owner_id = ? OR tool_providers.visibility = 'system')", definitionID, ownerID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Definition{}, businessError("404_TOOL_NOT_FOUND")
	}
	return entity, err
}

func (service *Service) definitionByID(ctx context.Context, definitionID string) (model.Definition, error) {
	var entity model.Definition
	err := service.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", definitionID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Definition{}, businessError("404_TOOL_NOT_FOUND")
	}
	return entity, err
}

func extractOperations(document map[string]any) []dto.OpenAPIOperation {
	paths, _ := document["paths"].(map[string]any)
	result := []dto.OpenAPIOperation{}
	for path, rawOperations := range paths {
		operations, _ := rawOperations.(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
			raw, exists := operations[method]
			if !exists {
				continue
			}
			operation, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			operationID := stringValue(operation["operationId"])
			if operationID == "" {
				operationID = method + "_" + path
			}
			var summary *string
			if value := stringValue(operation["summary"]); value != "" {
				summary = &value
			}
			properties := map[string]any{}
			required := []string{}
			if parameters, ok := operation["parameters"].([]any); ok {
				for _, rawParameter := range parameters {
					parameter, _ := rawParameter.(map[string]any)
					name := stringValue(parameter["name"])
					if name == "" {
						continue
					}
					schema, _ := parameter["schema"].(map[string]any)
					if schema == nil {
						schema = map[string]any{"type": "string"}
					}
					properties[name] = schema
					if requiredValue, _ := parameter["required"].(bool); requiredValue {
						required = append(required, name)
					}
				}
			}
			requestBody, _ := operation["requestBody"].(map[string]any)
			content, _ := requestBody["content"].(map[string]any)
			jsonContent, _ := content["application/json"].(map[string]any)
			schema, _ := jsonContent["schema"].(map[string]any)
			if schema["type"] == "object" {
				if bodyProperties, ok := schema["properties"].(map[string]any); ok {
					for key, value := range bodyProperties {
						properties[key] = value
					}
				}
				for _, key := range stringSlice(schema["required"]) {
					if !contains(required, key) {
						required = append(required, key)
					}
				}
			}
			result = append(result, dto.OpenAPIOperation{OperationID: operationID, Method: strings.ToUpper(method), Path: path, Summary: summary, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}})
		}
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func stringValue(value any) string { result, _ := value.(string); return result }
