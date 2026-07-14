package service

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type capabilityDescriptor struct {
	ID               string
	RiskLevel        string
	ApprovalRequired bool
	ProviderID       string
	V2               bool
}

// ListCapabilities 查询 Agent 当前全部能力挂载。
func (service *Service) ListCapabilities(ctx context.Context, agentID string) (dto.CapabilityRead, error) {
	result := dto.CapabilityRead{Skills: []dto.CapabilityItem{}, Tools: []dto.CapabilityItem{}, Knowledge: []dto.CapabilityItem{}}
	var skills []model.SkillBinding
	if err := service.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("skill_id").Find(&skills).Error; err != nil {
		return result, err
	}
	for _, item := range skills {
		result.Skills = append(result.Skills, dto.CapabilityItem{ID: item.SkillID, PolicyGuardEnabled: item.PolicyGuardEnabled, MountedAt: item.MountedAt})
	}
	var tools []model.ToolBinding
	if err := service.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("tool_id").Find(&tools).Error; err != nil {
		return result, err
	}
	for _, item := range tools {
		result.Tools = append(result.Tools, dto.CapabilityItem{ID: item.ToolID, PolicyGuardEnabled: item.PolicyGuardEnabled, ApprovalRequired: item.ApprovalRequired, MountedAt: item.MountedAt})
	}
	var knowledge []model.KnowledgeBinding
	if err := service.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("knowledge_id").Find(&knowledge).Error; err != nil {
		return result, err
	}
	for _, item := range knowledge {
		result.Knowledge = append(result.Knowledge, dto.CapabilityItem{ID: item.KnowledgeID, MountedAt: item.MountedAt})
	}
	return result, nil
}

// MountSkill 校验可用性、风险和数量上限后幂等挂载 Skill。
func (service *Service) MountSkill(ctx context.Context, actor Actor, agentID, skillID string) (dto.CapabilityRead, error) {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return dto.CapabilityRead{}, err
	}
	descriptor, err := service.skillDescriptor(ctx, skillID)
	if err != nil {
		return dto.CapabilityRead{}, err
	}
	var count int64
	var exists int64
	if err := service.db.WithContext(ctx).Model(&model.SkillBinding{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if err := service.db.WithContext(ctx).Model(&model.SkillBinding{}).Where("agent_id = ? AND skill_id = ?", agentID, skillID).Count(&exists).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if exists == 0 && count >= 20 {
		return dto.CapabilityRead{}, businessError(400, "400_LIMIT_EXCEEDED", "能力挂载数量已达上限")
	}
	highRisk := descriptor.RiskLevel == "high" || descriptor.RiskLevel == "critical"
	entity := model.SkillBinding{ID: uuid.NewString(), AgentID: agentID, SkillID: skillID, PolicyGuardEnabled: highRisk}
	err = service.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}, {Name: "skill_id"}}, DoUpdates: clause.Assignments(map[string]any{"policy_guard_enabled": gorm.Expr("agent_skills.policy_guard_enabled OR EXCLUDED.policy_guard_enabled")})}).Create(&entity).Error
	if err != nil {
		return dto.CapabilityRead{}, err
	}
	return service.ListCapabilities(ctx, agentID)
}

// UnmountSkill 幂等卸载 Skill。
func (service *Service) UnmountSkill(ctx context.Context, actor Actor, agentID, skillID string) error {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return err
	}
	return service.db.WithContext(ctx).Where("agent_id = ? AND skill_id = ?", agentID, skillID).Delete(&model.SkillBinding{}).Error
}

// MountTool 优先挂载 Tools v2 定义，并兼容旧版 Tool。
//
// 简要描述：Tools v2 挂载时按 Owner 凭据、系统 Provider 凭据两级选择默认 Credential；
// 高风险工具强制开启策略守卫与审批，重复挂载只会增强安全属性。
func (service *Service) MountTool(ctx context.Context, actor Actor, agentID, toolID string) (dto.CapabilityRead, error) {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return dto.CapabilityRead{}, err
	}
	descriptor, err := service.toolDescriptor(ctx, toolID)
	if err != nil {
		return dto.CapabilityRead{}, err
	}
	var count, exists int64
	if err := service.db.WithContext(ctx).Model(&model.ToolBinding{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if err := service.db.WithContext(ctx).Model(&model.ToolBinding{}).Where("agent_id = ? AND tool_id = ?", agentID, toolID).Count(&exists).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if exists == 0 && count >= 50 {
		return dto.CapabilityRead{}, businessError(400, "400_LIMIT_EXCEEDED", "能力挂载数量已达上限")
	}
	highRisk := descriptor.RiskLevel == "high" || descriptor.RiskLevel == "critical"
	var credentialID *string
	if descriptor.V2 {
		credentialID, err = service.defaultCredential(ctx, descriptor.ProviderID, actor.OwnerID)
		if err != nil {
			return dto.CapabilityRead{}, err
		}
	}
	entity := model.ToolBinding{ID: uuid.NewString(), AgentID: agentID, ToolID: toolID, Enabled: true, CredentialID: credentialID, PolicyGuardEnabled: highRisk, ApprovalRequired: descriptor.ApprovalRequired || highRisk}
	updates := map[string]any{"enabled": true, "policy_guard_enabled": gorm.Expr("agent_tools.policy_guard_enabled OR EXCLUDED.policy_guard_enabled"), "approval_required": gorm.Expr("agent_tools.approval_required OR EXCLUDED.approval_required"), "credential_id": gorm.Expr("COALESCE(agent_tools.credential_id, EXCLUDED.credential_id)"), "updated_at": gorm.Expr("now()")}
	err = service.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}, {Name: "tool_id"}}, DoUpdates: clause.Assignments(updates)}).Create(&entity).Error
	if err != nil {
		return dto.CapabilityRead{}, err
	}
	return service.ListCapabilities(ctx, agentID)
}

// UnmountTool 幂等卸载 Tool。
func (service *Service) UnmountTool(ctx context.Context, actor Actor, agentID, toolID string) error {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return err
	}
	return service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", agentID, toolID).Delete(&model.ToolBinding{}).Error
}

// MountKnowledge 仅允许挂载 ready 状态知识库。
func (service *Service) MountKnowledge(ctx context.Context, actor Actor, agentID, knowledgeID string) (dto.CapabilityRead, error) {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return dto.CapabilityRead{}, err
	}
	var state struct{ Status string }
	err := service.db.WithContext(ctx).Table("knowledge").Select("status").Where("id = ? AND deleted_at IS NULL", knowledgeID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.CapabilityRead{}, businessError(404, "404_NOT_FOUND", "知识库不存在")
	}
	if err != nil {
		return dto.CapabilityRead{}, err
	}
	if state.Status != "ready" {
		return dto.CapabilityRead{}, businessError(400, "400_KNOWLEDGE_NOT_READY", "知识库状态为 "+state.Status+"，仅 ready 状态可挂载")
	}
	var count, exists int64
	if err := service.db.WithContext(ctx).Model(&model.KnowledgeBinding{}).Where("agent_id = ?", agentID).Count(&count).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if err := service.db.WithContext(ctx).Model(&model.KnowledgeBinding{}).Where("agent_id = ? AND knowledge_id = ?", agentID, knowledgeID).Count(&exists).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	if exists == 0 && count >= 10 {
		return dto.CapabilityRead{}, businessError(400, "400_LIMIT_EXCEEDED", "能力挂载数量已达上限")
	}
	entity := model.KnowledgeBinding{ID: uuid.NewString(), AgentID: agentID, KnowledgeID: knowledgeID}
	if err := service.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agent_id"}, {Name: "knowledge_id"}}, DoNothing: true}).Create(&entity).Error; err != nil {
		return dto.CapabilityRead{}, err
	}
	return service.ListCapabilities(ctx, agentID)
}

// UnmountKnowledge 幂等卸载 Knowledge。
func (service *Service) UnmountKnowledge(ctx context.Context, actor Actor, agentID, knowledgeID string) error {
	if err := service.assertCanEdit(ctx, actor, agentID); err != nil {
		return err
	}
	return service.db.WithContext(ctx).Where("agent_id = ? AND knowledge_id = ?", agentID, knowledgeID).Delete(&model.KnowledgeBinding{}).Error
}

// SyncCapabilities 将非 nil 的目标能力列表同步为完整选择集合。
func (service *Service) SyncCapabilities(ctx context.Context, actor Actor, agentID string, skillIDs, toolIDs, knowledgeIDs *[]string) error {
	if skillIDs == nil && toolIDs == nil && knowledgeIDs == nil {
		return nil
	}
	current, err := service.ListCapabilities(ctx, agentID)
	if err != nil {
		return err
	}
	if skillIDs != nil {
		if err := service.syncSkills(ctx, actor, agentID, current.Skills, *skillIDs); err != nil {
			return err
		}
	}
	if toolIDs != nil {
		if err := service.syncTools(ctx, actor, agentID, current.Tools, *toolIDs); err != nil {
			return err
		}
	}
	if knowledgeIDs != nil {
		if err := service.syncKnowledge(ctx, actor, agentID, current.Knowledge, *knowledgeIDs); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) mountCapabilityBatch(ctx context.Context, actor Actor, agentID string, skillIDs, toolIDs, knowledgeIDs []string) error {
	for _, id := range deduplicate(skillIDs) {
		if _, err := service.MountSkill(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	for _, id := range deduplicate(toolIDs) {
		if _, err := service.MountTool(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	for _, id := range deduplicate(knowledgeIDs) {
		if _, err := service.MountKnowledge(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) syncSkills(ctx context.Context, actor Actor, agentID string, current []dto.CapabilityItem, target []string) error {
	remove, add := differences(current, target)
	for _, id := range remove {
		if err := service.UnmountSkill(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	for _, id := range add {
		if _, err := service.MountSkill(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	return nil
}
func (service *Service) syncTools(ctx context.Context, actor Actor, agentID string, current []dto.CapabilityItem, target []string) error {
	remove, add := differences(current, target)
	for _, id := range remove {
		if err := service.UnmountTool(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	for _, id := range add {
		if _, err := service.MountTool(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	return nil
}
func (service *Service) syncKnowledge(ctx context.Context, actor Actor, agentID string, current []dto.CapabilityItem, target []string) error {
	remove, add := differences(current, target)
	for _, id := range remove {
		if err := service.UnmountKnowledge(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	for _, id := range add {
		if _, err := service.MountKnowledge(ctx, actor, agentID, id); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) assertCanEdit(ctx context.Context, actor Actor, agentID string) error {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return err
	}
	if entity.Status == "archived" {
		return businessError(409, "409_AGENT_ARCHIVED", "Agent 已归档且不可修改")
	}
	return nil
}

func (service *Service) skillDescriptor(ctx context.Context, skillID string) (capabilityDescriptor, error) {
	var value struct {
		ID               string
		RiskLevel        string `gorm:"column:risk_level"`
		RequiresApproval bool   `gorm:"column:requires_approval"`
	}
	err := service.db.WithContext(ctx).Table("skills").Where("id = ? AND status = 'active' AND deleted_at IS NULL", skillID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return capabilityDescriptor{}, businessError(404, "404_NOT_FOUND", "能力不存在或不可用")
	}
	return capabilityDescriptor{ID: value.ID, RiskLevel: value.RiskLevel, ApprovalRequired: value.RequiresApproval}, err
}

func (service *Service) toolDescriptor(ctx context.Context, toolID string) (capabilityDescriptor, error) {
	var v2 struct {
		ID               string
		ProviderID       string `gorm:"column:provider_id"`
		RiskLevel        string `gorm:"column:risk_level"`
		ApprovalRequired bool   `gorm:"column:approval_required"`
	}
	err := service.db.WithContext(ctx).Table("tool_definitions").Where("id = ? AND status = 'active' AND deleted_at IS NULL", toolID).First(&v2).Error
	if err == nil {
		return capabilityDescriptor{ID: v2.ID, ProviderID: v2.ProviderID, RiskLevel: v2.RiskLevel, ApprovalRequired: v2.ApprovalRequired, V2: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return capabilityDescriptor{}, err
	}
	var legacy struct {
		ID               string
		RiskLevel        string `gorm:"column:risk_level"`
		RequiresApproval bool   `gorm:"column:requires_approval"`
	}
	err = service.db.WithContext(ctx).Table("tools").Where("id = ? AND status = 'active' AND deleted_at IS NULL", toolID).First(&legacy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return capabilityDescriptor{}, businessError(404, "404_NOT_FOUND", "能力不存在或不可用")
	}
	return capabilityDescriptor{ID: legacy.ID, RiskLevel: legacy.RiskLevel, ApprovalRequired: legacy.RequiresApproval}, err
}

func (service *Service) defaultCredential(ctx context.Context, providerID, ownerID string) (*string, error) {
	var id string
	err := service.db.WithContext(ctx).Table("tool_credentials").Select("id").Where("provider_id = ? AND owner_id = ? AND status = 'active'", providerID, ownerID).Order("updated_at DESC").Limit(1).Scan(&id).Error
	if err != nil {
		return nil, err
	}
	if id != "" {
		return &id, nil
	}
	err = service.db.WithContext(ctx).Table("tool_credentials c").Select("c.id").Joins("JOIN tool_providers p ON p.id = c.provider_id").Where("c.provider_id = ? AND c.status = 'active' AND p.visibility = 'system' AND p.deleted_at IS NULL", providerID).Order("c.updated_at DESC").Limit(1).Scan(&id).Error
	if err != nil || id == "" {
		return nil, err
	}
	return &id, nil
}

func differences(current []dto.CapabilityItem, target []string) ([]string, []string) {
	currentSet := map[string]bool{}
	for _, item := range current {
		currentSet[item.ID] = true
	}
	target = deduplicate(target)
	targetSet := map[string]bool{}
	for _, id := range target {
		targetSet[id] = true
	}
	remove := []string{}
	for id := range currentSet {
		if !targetSet[id] {
			remove = append(remove, id)
		}
	}
	sort.Strings(remove)
	add := []string{}
	for _, id := range target {
		if !currentSet[id] {
			add = append(add, id)
		}
	}
	return remove, add
}

func deduplicate(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
