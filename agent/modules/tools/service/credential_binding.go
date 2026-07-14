package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateCredential 校验 Provider authSchema 后加密保存凭据。
func (service *Service) CreateCredential(ctx context.Context, ownerID string, command dto.CredentialCreateRequest) (dto.CredentialResponse, error) {
	provider, err := service.visibleProvider(ctx, ownerID, command.ProviderID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	if err := validateCredential(provider, command.CredentialPayload); err != nil {
		return dto.CredentialResponse{}, err
	}
	var count int64
	if err := service.db.WithContext(ctx).Model(&model.Credential{}).Where("owner_id = ? AND provider_id = ? AND name = ?", ownerID, command.ProviderID, command.Name).Count(&count).Error; err != nil {
		return dto.CredentialResponse{}, err
	}
	if count > 0 {
		return dto.CredentialResponse{}, businessError("409_CREDENTIAL_NAME_CONFLICT")
	}
	encrypted, err := service.encryptCredential(command.CredentialPayload)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	entity := model.Credential{ID: "credential-" + uuid.NewString(), OwnerID: ownerID, ProviderID: command.ProviderID, Name: command.Name, CredentialPayloadEncrypted: encrypted, Status: "active"}
	if err := service.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dto.CredentialResponse{}, err
	}
	return credentialResponse(entity), nil
}

// GetCredential 查询凭据元数据，不返回或解密明文。
func (service *Service) GetCredential(ctx context.Context, ownerID, credentialID string) (dto.CredentialResponse, error) {
	entity, err := service.ownedCredential(ctx, ownerID, credentialID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	return credentialResponse(entity), nil
}

// ListCredentials 分页查询当前 Owner 的凭据元数据。
func (service *Service) ListCredentials(ctx context.Context, ownerID, providerID, status string, page, pageSize int) (dto.CredentialListResponse, error) {
	page, pageSize = pageValues(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.Credential{}).Where("owner_id = ?", ownerID)
	if providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.CredentialListResponse{}, err
	}
	var entities []model.Credential
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.CredentialListResponse{}, err
	}
	items := make([]dto.CredentialResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, credentialResponse(entity))
	}
	return dto.CredentialListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// RotateCredential 校验并替换密文，同时恢复 active 状态。
func (service *Service) RotateCredential(ctx context.Context, ownerID, credentialID string, command dto.CredentialRotateRequest) (dto.CredentialResponse, error) {
	entity, err := service.ownedCredential(ctx, ownerID, credentialID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	provider, err := service.providerByID(ctx, entity.ProviderID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	if err := validateCredential(provider, command.CredentialPayload); err != nil {
		return dto.CredentialResponse{}, err
	}
	encrypted, err := service.encryptCredential(command.CredentialPayload)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	now := time.Now().UTC()
	entity.CredentialPayloadEncrypted = encrypted
	entity.Status = "active"
	entity.RotatedAt = &now
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.CredentialResponse{}, err
	}
	return credentialResponse(entity), nil
}

// RevokeCredential 吊销凭据但保留审计元数据。
func (service *Service) RevokeCredential(ctx context.Context, ownerID, credentialID string) (dto.CredentialResponse, error) {
	entity, err := service.ownedCredential(ctx, ownerID, credentialID)
	if err != nil {
		return dto.CredentialResponse{}, err
	}
	entity.Status = "revoked"
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.CredentialResponse{}, err
	}
	return credentialResponse(entity), nil
}

// MountTool 将 active 工具定义挂载到 Agent，重复挂载时执行幂等更新。
func (service *Service) MountTool(ctx context.Context, agentID string, command dto.BindingCreateRequest) (dto.BindingResponse, error) {
	var definition model.Definition
	err := service.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", command.ToolDefinitionID).First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.BindingResponse{}, businessError("404_TOOL_NOT_FOUND")
	}
	if err != nil {
		return dto.BindingResponse{}, err
	}
	if definition.Status != "active" {
		return dto.BindingResponse{}, businessError("409_CAPABILITY_NOT_ACTIVE")
	}
	var existing model.Binding
	lookup := service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", agentID, command.ToolDefinitionID).First(&existing).Error
	if errors.Is(lookup, gorm.ErrRecordNotFound) {
		var count int64
		if err := service.db.WithContext(ctx).Model(&model.Binding{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
			return dto.BindingResponse{}, err
		}
		if count >= 50 {
			return dto.BindingResponse{}, businessError("400_LIMIT_EXCEEDED")
		}
	} else if lookup != nil {
		return dto.BindingResponse{}, lookup
	}
	enabled := true
	if command.Enabled != nil {
		enabled = *command.Enabled
	}
	guard := definition.RiskLevel == "high" || definition.RiskLevel == "critical"
	entity := model.Binding{ID: "binding-" + uuid.NewString(), AgentID: agentID, ToolID: command.ToolDefinitionID, Enabled: enabled, RuntimeOverrides: command.RuntimeOverrides, CredentialID: command.CredentialID, PolicyGuardEnabled: guard, ApprovalRequired: definition.ApprovalRequired || guard}
	if existing.ID != "" {
		entity.ID = existing.ID
		entity.MountedAt = existing.MountedAt
	}
	if err := service.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}, {Name: "tool_id"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "runtime_overrides", "credential_id", "policy_guard_enabled", "approval_required", "updated_at"})}).Create(&entity).Error; err != nil {
		return dto.BindingResponse{}, err
	}
	if err := service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", agentID, command.ToolDefinitionID).First(&entity).Error; err != nil {
		return dto.BindingResponse{}, err
	}
	return bindingResponse(entity), nil
}

// UpdateBinding 更新 Agent 工具挂载的启用、覆盖配置和凭据选择。
func (service *Service) UpdateBinding(ctx context.Context, agentID, definitionID string, command dto.BindingUpdateRequest) (dto.BindingResponse, error) {
	var entity model.Binding
	err := service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", agentID, definitionID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.BindingResponse{}, businessError("404_BINDING_NOT_FOUND")
	}
	if err != nil {
		return dto.BindingResponse{}, err
	}
	if command.Enabled != nil {
		entity.Enabled = *command.Enabled
	}
	if command.RuntimeOverrides != nil {
		entity.RuntimeOverrides = command.RuntimeOverrides
	}
	if command.CredentialID != nil {
		entity.CredentialID = command.CredentialID
	}
	if err := service.db.WithContext(ctx).Save(&entity).Error; err != nil {
		return dto.BindingResponse{}, err
	}
	return bindingResponse(entity), nil
}

// UnmountTool 删除 Agent 工具挂载关系。
func (service *Service) UnmountTool(ctx context.Context, agentID, definitionID string) error {
	result := service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", agentID, definitionID).Delete(&model.Binding{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return businessError("404_BINDING_NOT_FOUND")
	}
	return nil
}

// ListBindings 查询 Agent 已挂载的全部工具。
func (service *Service) ListBindings(ctx context.Context, agentID string) (dto.BindingListResponse, error) {
	var entities []model.Binding
	if err := service.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("mounted_at ASC").Find(&entities).Error; err != nil {
		return dto.BindingListResponse{}, err
	}
	items := make([]dto.BindingResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, bindingResponse(entity))
	}
	return dto.BindingListResponse{Items: items, Total: len(items)}, nil
}

func (service *Service) ownedCredential(ctx context.Context, ownerID, credentialID string) (model.Credential, error) {
	var entity model.Credential
	err := service.db.WithContext(ctx).Where("id = ? AND owner_id = ?", credentialID, ownerID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Credential{}, businessError("404_CREDENTIAL_NOT_FOUND")
	}
	return entity, err
}

func (service *Service) encryptCredential(payload map[string]any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return service.encryption.Encrypt(string(encoded))
}
func (service *Service) decryptCredential(value string) (map[string]any, error) {
	plain, err := service.encryption.Decrypt(value)
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	if json.Unmarshal([]byte(plain), &result) != nil {
		return nil, businessError("400_CREDENTIAL_SCHEMA_INVALID")
	}
	return result, nil
}

func validateCredential(provider model.Provider, payload map[string]any) error {
	if provider.AuthType == "none" {
		if len(payload) > 0 {
			return businessError("400_AUTH_TYPE_MISMATCH")
		}
		return nil
	}
	if len(payload) == 0 {
		return businessError("400_EMPTY_CREDENTIAL_NOT_ALLOWED")
	}
	if validatePayload(provider.AuthSchema, payload) != nil {
		return businessError("400_CREDENTIAL_SCHEMA_INVALID")
	}
	return nil
}
