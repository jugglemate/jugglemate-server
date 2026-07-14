package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"gorm.io/gorm"
)

type resolvedModel struct {
	ModelID    string `gorm:"column:model_id"`
	ProviderID string `gorm:"column:provider_id"`
	Status     string `gorm:"column:status"`
}

// CreateAgent 创建 Agent、初始化默认配置并按请求挂载能力。
//
// 简要描述：Agent 主记录先持久化，能力按源服务顺序逐项挂载；首次创建完成后尝试
// 发放赠送 Credit，赠送侧异常只记录告警，不回滚已经成功创建的 Agent。
func (service *Service) CreateAgent(ctx context.Context, actor Actor, request dto.CreateRequest) (dto.DetailResponse, error) {
	name, err := normalizeRequired(request.Name, "name", 128)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	agentType, err := normalizeRequired(request.Type, "type", 32)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	prompt, err := normalizeRequired(request.Prompt, "prompt", 2000)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	resolved, err := service.resolveReasoningModel(ctx, request.LLMModel, request.LLMProviderID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	summaryModel, err := service.resolveSummaryModel(ctx, resolved.ModelID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	providerID := resolved.ProviderID
	entity := model.Agent{ID: uuid.NewString(), OwnerID: actor.OwnerID, Name: name, Type: agentType, Prompt: &prompt, LLMModel: &resolved.ModelID, LLMProviderID: &providerID, Status: "draft", ReactConfig: reactMap(defaultReactConfig()), MemoryConfig: memoryMap(defaultMemoryConfig(summaryModel))}
	isFirstAgent := false
	// 简要描述：以 Owner 哈希作为事务级 advisory lock，串行化计数与创建，避免并发突破
	// 50 个上限或重复取得“首个 Agent”赠送资格；事务结束后 PostgreSQL 自动释放锁。
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", actor.OwnerID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Agent{}).Where("owner_id = ?", actor.OwnerID).Count(&count).Error; err != nil {
			return err
		}
		if count >= 50 {
			return businessError(400, "400_LIMIT_EXCEEDED", "单个 Owner 的 Agent 数量已达上限")
		}
		isFirstAgent = count == 0
		return tx.Create(&entity).Error
	})
	if err != nil {
		return dto.DetailResponse{}, err
	}
	if isFirstAgent && service.billing != nil && service.config.FirstAgentRechargeAmount.IsPositive() {
		if _, err := service.billing.CreateCompletedRecharge(ctx, actor.OwnerID, service.config.FirstAgentRechargeAmount); err != nil {
			slog.WarnContext(ctx, "首个 Agent 自动充值赠送失败", "owner_id", actor.OwnerID, "error", err)
		}
	}
	if err := service.mountCapabilityBatch(ctx, actor, entity.ID, request.SkillIDs, request.ToolIDs, request.KnowledgeIDs); err != nil {
		return dto.DetailResponse{}, err
	}
	return service.GetAgent(ctx, actor, entity.ID)
}

// GetAgent 查询 Agent 详情并校验 Owner/Admin 权限。
func (service *Service) GetAgent(ctx context.Context, actor Actor, agentID string) (dto.DetailResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	if err := assertPermission(actor, entity, true); err != nil {
		return dto.DetailResponse{}, err
	}
	return service.toDetail(ctx, entity)
}

// ListAgents 分页查询当前 Owner Agent，并在首页前置系统兜底 Agent。
func (service *Service) ListAgents(ctx context.Context, ownerID string, page, pageSize int) (dto.ListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.Agent{}).Where("owner_id = ?", ownerID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ListResponse{}, err
	}
	var entities []model.Agent
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.ListResponse{}, err
	}
	if ownerID != systemOwnerID && page == 1 {
		var builtin model.Agent
		if err := service.db.WithContext(ctx).Where("id = ? AND owner_id = ?", builtinAgentID, systemOwnerID).First(&builtin).Error; err == nil {
			entities = append([]model.Agent{builtin}, entities...)
			total++
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ListResponse{}, err
		}
	}
	knowledgeCounts, err := service.bindingCounts(ctx, "agent_knowledge", entities)
	if err != nil {
		return dto.ListResponse{}, err
	}
	toolCounts, err := service.bindingCounts(ctx, "agent_tools", entities)
	if err != nil {
		return dto.ListResponse{}, err
	}
	items := make([]dto.ConsoleListItem, 0, len(entities))
	for index := range entities {
		items = append(items, service.toListItem(&entities[index], knowledgeCounts[entities[index].ID], toolCounts[entities[index].ID]))
	}
	return dto.ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// UpdateAgent 按补丁更新 Agent Profile、记忆配置和目标能力集合。
func (service *Service) UpdateAgent(ctx context.Context, actor Actor, request dto.UpdateRequest) (dto.DetailResponse, error) {
	entity, err := service.findAgent(ctx, request.AgentID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	if err := assertPermission(actor, entity, true); err != nil {
		return dto.DetailResponse{}, err
	}
	if entity.Status == "archived" {
		return dto.DetailResponse{}, businessError(409, "409_AGENT_ARCHIVED", "Agent 已归档且不可编辑")
	}
	updates := map[string]any{}
	if request.Name != nil {
		value, err := normalizeRequired(request.Name, "name", 128)
		if err != nil {
			return dto.DetailResponse{}, err
		}
		updates["name"] = value
	}
	if request.Type != nil {
		value, err := normalizeRequired(request.Type, "type", 32)
		if err != nil {
			return dto.DetailResponse{}, err
		}
		updates["type"] = value
	}
	if request.Prompt != nil {
		value, err := normalizeRequired(request.Prompt, "prompt", 2000)
		if err != nil {
			return dto.DetailResponse{}, err
		}
		updates["prompt"] = value
	}
	if request.LLMModel != nil {
		resolved, err := service.resolveReasoningModel(ctx, request.LLMModel, request.LLMProviderID)
		if err != nil {
			return dto.DetailResponse{}, err
		}
		updates["llm_model"] = resolved.ModelID
		updates["llm_provider_id"] = resolved.ProviderID
	} else if request.LLMProviderID != nil {
		updates["llm_provider_id"] = strings.TrimSpace(*request.LLMProviderID)
	}
	if request.ReactConfig != nil {
		value := reactFromMap(entity.ReactConfig)
		applyReactPatch(&value, request.ReactConfig)
		if err := validateReact(value); err != nil {
			return dto.DetailResponse{}, err
		}
		updates["react_config"] = reactMap(value)
	}
	if request.MemoryConfig != nil {
		value := memoryFromMap(entity.MemoryConfig)
		applyMemoryPatch(&value, request.MemoryConfig)
		if err := validateMemory(value); err != nil {
			return dto.DetailResponse{}, err
		}
		updates["memory_config"] = memoryMap(value)
	}
	if len(updates) > 0 {
		if err := service.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ?", entity.ID).Updates(updates).Error; err != nil {
			return dto.DetailResponse{}, err
		}
	}
	if err := service.SyncCapabilities(ctx, actor, entity.ID, request.SkillIDs, request.ToolIDs, request.KnowledgeIDs); err != nil {
		return dto.DetailResponse{}, err
	}
	return service.GetAgent(ctx, actor, entity.ID)
}

// DeleteAgent 物理删除仅处于 draft 状态的 Agent。
func (service *Service) DeleteAgent(ctx context.Context, actor Actor, agentID string) error {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return err
	}
	if entity.Status != "draft" {
		return businessError(409, "409_INVALID_STATUS_TRANSITION", "仅 draft 状态 Agent 支持删除")
	}
	result := service.db.WithContext(ctx).Where("id = ? AND owner_id = ? AND status = 'draft'", entity.ID, entity.OwnerID).Delete(&model.Agent{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return businessError(409, "409_INVALID_STATUS_TRANSITION", "仅 draft 状态 Agent 支持删除")
	}
	return nil
}

// ActivateAgent 校验完整配置、模型和余额后启动 Agent。
func (service *Service) ActivateAgent(ctx context.Context, actor Actor, agentID string) (dto.LifecycleResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return dto.LifecycleResponse{}, err
	}
	previous := entity.Status
	if err := assertTransition(previous, "active"); err != nil {
		return dto.LifecycleResponse{}, err
	}
	if entity.Name == "" || entity.Type == "" || entity.Prompt == nil || strings.TrimSpace(*entity.Prompt) == "" || entity.LLMModel == nil || strings.TrimSpace(*entity.LLMModel) == "" {
		code, message := "400_INCOMPLETE_CONFIG", "配置不完整，无法启动"
		if previous == "error" {
			code, message = "400_RECOVERY_CHECK_FAILED", "恢复校验失败，请先修复依赖问题"
		}
		return dto.LifecycleResponse{}, businessError(400, code, message)
	}
	if _, err := service.resolveReasoningModel(ctx, entity.LLMModel, entity.LLMProviderID); err != nil {
		return dto.LifecycleResponse{}, err
	}
	if service.config.ActivationPointsCheckEnabled && service.billing != nil {
		estimated := service.billing.EstimateCreditsForModel(ctx, *entity.LLMModel, 1000)
		if !service.billing.PreCheck(ctx, entity.OwnerID, estimated) {
			return dto.LifecycleResponse{}, businessError(402, "402_INSUFFICIENT_POINTS", "积分不足，无法启动 Agent")
		}
	}
	now := time.Now().UTC()
	result := service.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ? AND status = ?", entity.ID, previous).Updates(map[string]any{"status": "active", "activated_at": now})
	if result.Error != nil {
		return dto.LifecycleResponse{}, result.Error
	}
	if result.RowsAffected == 0 {
		return dto.LifecycleResponse{}, businessError(409, "409_INVALID_STATUS_TRANSITION", "Agent 状态已被并发修改")
	}
	return dto.LifecycleResponse{Status: "activated", PreviousStatus: previous, CurrentStatus: "active", AgentID: entity.ID}, nil
}

// PauseAgent 暂停 Agent，新请求将不再进入推理链路。
func (service *Service) PauseAgent(ctx context.Context, actor Actor, agentID string) (dto.LifecycleResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertTransition(entity.Status, "paused"); err != nil {
		return dto.LifecycleResponse{}, err
	}
	now := time.Now().UTC()
	result := service.db.WithContext(ctx).Model(&model.Agent{}).Where("id = ? AND status = ?", entity.ID, entity.Status).Updates(map[string]any{"status": "paused", "paused_at": now})
	if result.Error != nil {
		return dto.LifecycleResponse{}, result.Error
	}
	if result.RowsAffected == 0 {
		return dto.LifecycleResponse{}, businessError(409, "409_INVALID_STATUS_TRANSITION", "Agent 状态已被并发修改")
	}
	return dto.LifecycleResponse{Status: "paused", PreviousStatus: entity.Status, CurrentStatus: "paused", AgentID: entity.ID}, nil
}

// ArchiveAgent 校验确认名称，中断运行中 ReAct 会话后归档 Agent。
//
// 简要描述：归档只允许 paused -> archived；运行中会话在同一事务改为 stopped，
// 随后更新 Agent 状态，保证不会留下“Agent 已归档但会话仍运行”的数据库状态。
func (service *Service) ArchiveAgent(ctx context.Context, actor Actor, agentID, confirmName string) (dto.LifecycleResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertTransition(entity.Status, "archived"); err != nil {
		return dto.LifecycleResponse{}, err
	}
	if entity.Name != confirmName {
		return dto.LifecycleResponse{}, businessError(400, "400_ARCHIVE_CONFIRMATION_MISMATCH", "归档确认名称不匹配")
	}
	var interrupted int64
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("react_sessions").Where("agent_id = ? AND status IN ?", entity.ID, []string{"initialized", "reasoning", "acting", "evaluating"}).Updates(map[string]any{"status": "stopped", "completed_at": time.Now().UTC(), "error": "agent_archived"})
		if result.Error != nil {
			return result.Error
		}
		interrupted = result.RowsAffected
		updated := tx.Model(&model.Agent{}).Where("id = ? AND status = 'paused'", entity.ID).Updates(map[string]any{"status": "archived", "archived_at": time.Now().UTC()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			return businessError(409, "409_INVALID_STATUS_TRANSITION", "Agent 状态已被并发修改")
		}
		return nil
	})
	if err != nil {
		return dto.LifecycleResponse{}, err
	}
	status := "archived"
	inProgress := interrupted > 0
	if inProgress {
		status = "archive_in_progress"
	}
	return dto.LifecycleResponse{Status: status, PreviousStatus: entity.Status, CurrentStatus: "archived", AgentID: entity.ID, ArchiveInProgress: inProgress, InterruptedSessions: interrupted}, nil
}

// ListLongTermMemories 查询指定 Agent+User 的有效长期记忆。
func (service *Service) ListLongTermMemories(ctx context.Context, actor Actor, agentID, userID string, topK int) ([]dto.LongTermMemoryRead, error) {
	if _, err := service.GetAgent(ctx, actor, agentID); err != nil {
		return nil, err
	}
	if topK < 1 {
		topK = 50
	}
	if topK > 1000 {
		topK = 1000
	}
	now := time.Now().UTC()
	if err := service.db.WithContext(ctx).Model(&model.LongTermMemory{}).Where("status = 'active' AND expires_at IS NOT NULL AND expires_at <= ?", now).Updates(map[string]any{"status": "expired", "updated_at": now}).Error; err != nil {
		return nil, err
	}
	var entities []model.LongTermMemory
	order := "CASE importance WHEN 'high' THEN 3 WHEN 'medium' THEN 2 ELSE 1 END DESC, updated_at DESC"
	if err := service.db.WithContext(ctx).Where("agent_id = ? AND user_id = ? AND status = 'active'", agentID, userID).Order(order).Limit(topK).Find(&entities).Error; err != nil {
		return nil, err
	}
	items := make([]dto.LongTermMemoryRead, 0, len(entities))
	for _, entity := range entities {
		source := ""
		if entity.SourceConversationID != nil {
			source = *entity.SourceConversationID
		}
		items = append(items, dto.LongTermMemoryRead{ID: entity.ID, AgentID: entity.AgentID, UserID: entity.UserID, MemoryType: entity.MemoryType, Content: entity.Content, Importance: entity.Importance, SourceConversationID: source, Status: entity.Status, ExpiresAt: entity.ExpiresAt})
	}
	return items, nil
}

// DeleteLongTermMemory 删除指定 Agent+User 下的一条长期记忆。
func (service *Service) DeleteLongTermMemory(ctx context.Context, actor Actor, agentID, userID, memoryID string) error {
	if _, err := service.GetAgent(ctx, actor, agentID); err != nil {
		return err
	}
	now := time.Now().UTC()
	result := service.db.WithContext(ctx).Where("id = ? AND agent_id = ? AND user_id = ? AND status = 'active' AND (expires_at IS NULL OR expires_at > ?)", memoryID, agentID, userID, now).Delete(&model.LongTermMemory{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return businessError(404, "404_NOT_FOUND", "长期记忆不存在")
	}
	return nil
}

func (service *Service) resolveReasoningModel(ctx context.Context, modelID, providerID *string) (resolvedModel, error) {
	target := ""
	if modelID != nil {
		target = strings.TrimSpace(*modelID)
	}
	if target == "" {
		if err := service.db.WithContext(ctx).Raw(`SELECT config_value #>> '{}' FROM system_config WHERE config_key = 'default_reasoning_model'`).Scan(&target).Error; err != nil {
			return resolvedModel{}, err
		}
	}
	if target == "" {
		return resolvedModel{}, businessError(400, "400_INVALID_CONFIG", "未指定 llmModel 且系统默认推理模型未配置")
	}
	query := service.db.WithContext(ctx).Table("llm_models").Select("model_id, provider_id::text AS provider_id, status").Where("model_id = ?", target)
	if providerID != nil && strings.TrimSpace(*providerID) != "" {
		query = query.Where("provider_id::text = ?", strings.TrimSpace(*providerID))
	}
	var resolved resolvedModel
	if err := query.Order("updated_at DESC").First(&resolved).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return resolvedModel{}, businessError(400, "400_MODEL_NOT_FOUND", "模型不存在: "+target)
		}
		return resolvedModel{}, err
	}
	if resolved.Status != "active" {
		return resolvedModel{}, businessError(400, "400_MODEL_INACTIVE", "模型未激活: "+target+"，当前状态: "+resolved.Status)
	}
	var valid int64
	if err := service.db.WithContext(ctx).Table("llm_models").Where("model_id = ? AND provider_id::text = ? AND ? = ANY(model_types)", resolved.ModelID, resolved.ProviderID, "reasoning").Count(&valid).Error; err != nil {
		return resolvedModel{}, err
	}
	if valid == 0 {
		return resolvedModel{}, businessError(400, "400_MODEL_CAPABILITY_MISMATCH", "模型不支持推理能力: "+target)
	}
	return resolved, nil
}

func (service *Service) resolveSummaryModel(ctx context.Context, reasoningModel string) (string, error) {
	var configured string
	if err := service.db.WithContext(ctx).Raw(`SELECT config_value #>> '{}' FROM system_config WHERE config_key = 'default_summary_model'`).Scan(&configured).Error; err != nil {
		return "", err
	}
	if configured != "" && configured != "claude-3-haiku" {
		supported, err := service.modelSupports(ctx, configured, "summary")
		if err != nil {
			return "", err
		}
		if supported {
			return configured, nil
		}
	}
	if reasoningModel != "" && reasoningModel != "claude-3-haiku" {
		return reasoningModel, nil
	}
	var candidate string
	if err := service.db.WithContext(ctx).Table("llm_models").Select("model_id").Where("status = 'active' AND model_id <> ? AND ? = ANY(model_types)", "claude-3-haiku", "summary").Order("updated_at DESC").Limit(1).Scan(&candidate).Error; err != nil {
		return "", err
	}
	if candidate != "" {
		return candidate, nil
	}
	if err := service.db.WithContext(ctx).Table("llm_models").Select("model_id").Where("status = 'active' AND model_id <> ?", "claude-3-haiku").Order("updated_at DESC").Limit(1).Scan(&candidate).Error; err != nil {
		return "", err
	}
	if candidate != "" {
		return candidate, nil
	}
	if reasoningModel != "" {
		return reasoningModel, nil
	}
	return "", businessError(400, "400_INVALID_CONFIG", "未找到可用摘要模型")
}

func (service *Service) modelSupports(ctx context.Context, modelID, capability string) (bool, error) {
	var count int64
	err := service.db.WithContext(ctx).Table("llm_models").Where("model_id = ? AND status = 'active' AND ? = ANY(model_types)", modelID, capability).Count(&count).Error
	return count > 0, err
}

func (service *Service) toDetail(ctx context.Context, entity *model.Agent) (dto.DetailResponse, error) {
	capabilities, err := service.ListCapabilities(ctx, entity.ID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	result := dto.DetailResponse{ID: entity.ID, OwnerID: entity.OwnerID, Name: entity.Name, Status: entity.Status, Type: entity.Type, Prompt: entity.Prompt, LLMModel: entity.LLMModel, LLMProviderID: entity.LLMProviderID, ReactConfig: reactFromMap(entity.ReactConfig), MemoryConfig: memoryFromMap(entity.MemoryConfig), TotalInvocations: entity.TotalInvocations, SuccessCount: entity.SuccessCount, FailCount: entity.FailCount, FailureRate: entity.FailureRate, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt, ActivatedAt: entity.ActivatedAt, PausedAt: entity.PausedAt, ArchivedAt: entity.ArchivedAt}
	for _, item := range capabilities.Skills {
		result.Skills = append(result.Skills, item.ID)
	}
	for _, item := range capabilities.Tools {
		result.Tools = append(result.Tools, item.ID)
	}
	for _, item := range capabilities.Knowledge {
		result.Knowledge = append(result.Knowledge, item.ID)
	}
	return result, nil
}

func (service *Service) toListItem(entity *model.Agent, knowledgeCount, toolCount int64) dto.ConsoleListItem {
	primaryModel := "unknown"
	if entity.LLMModel != nil && *entity.LLMModel != "" {
		primaryModel = *entity.LLMModel
	}
	risk := "normal"
	if entity.TotalInvocations >= 1000 {
		risk = "high_load"
	} else if entity.FailureRate >= 0.2 {
		risk = "quota_limit"
	}
	published := entity.UpdatedAt
	if entity.ActivatedAt != nil {
		published = *entity.ActivatedAt
	}
	return dto.ConsoleListItem{AgentID: entity.ID, AgentName: entity.Name, OwnerID: entity.OwnerID, OwnerName: "Owner " + entity.OwnerID, PrimaryModel: primaryModel, Status: entity.Status, RiskFlag: risk, CreatedAt: entity.CreatedAt, PublishedAt: &published, KnowledgeCount: knowledgeCount, ToolCount: toolCount}
}

func (service *Service) bindingCounts(ctx context.Context, table string, entities []model.Agent) (map[string]int64, error) {
	result := map[string]int64{}
	if len(entities) == 0 {
		return result, nil
	}
	ids := make([]string, 0, len(entities))
	for _, entity := range entities {
		ids = append(ids, entity.ID)
	}
	var rows []struct {
		AgentID string `gorm:"column:agent_id"`
		Count   int64  `gorm:"column:count"`
	}
	if err := service.db.WithContext(ctx).Table(table).Select("agent_id, COUNT(*) AS count").Where("agent_id IN ?", ids).Group("agent_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AgentID] = row.Count
	}
	return result, nil
}

func applyReactPatch(value *dto.ReactConfig, patch *dto.ReactConfigPatch) {
	if patch.MaxRounds != nil {
		value.MaxRounds = *patch.MaxRounds
	}
	if patch.TargetRounds != nil {
		value.TargetRounds = *patch.TargetRounds
	}
	if patch.ToolsPerRound != nil {
		value.ToolsPerRound = *patch.ToolsPerRound
	}
	if patch.ConfidenceThreshold != nil {
		value.ConfidenceThreshold = *patch.ConfidenceThreshold
	}
}

func validateReact(value dto.ReactConfig) error {
	if value.MaxRounds < 3 || value.MaxRounds > 10 || value.TargetRounds < 2 || value.TargetRounds > 8 || value.ToolsPerRound < 1 || value.ToolsPerRound > 5 || value.ConfidenceThreshold < 0.5 || value.ConfidenceThreshold > 1 || value.TargetRounds > value.MaxRounds {
		return businessError(400, "400_INVALID_CONFIG", "ReAct 配置参数非法")
	}
	return nil
}

func applyMemoryPatch(value *dto.MemoryConfig, patch *dto.MemoryConfigPatch) {
	if patch.ShortTermEnabled != nil {
		value.ShortTermEnabled = *patch.ShortTermEnabled
	}
	if patch.ShortTermWindowSize != nil {
		value.ShortTermWindowSize = *patch.ShortTermWindowSize
	}
	if patch.SummaryEnabled != nil {
		value.SummaryEnabled = *patch.SummaryEnabled
	}
	if patch.SummaryTriggerTokens != nil {
		value.SummaryTriggerTokens = *patch.SummaryTriggerTokens
	}
	if patch.SummaryMaxTokens != nil {
		value.SummaryMaxTokens = *patch.SummaryMaxTokens
	}
	if patch.SummaryModel != nil {
		value.SummaryModel = *patch.SummaryModel
	}
	if patch.LongTermEnabled != nil {
		value.LongTermEnabled = *patch.LongTermEnabled
	}
	if patch.LongTermExtractOnClose != nil {
		value.LongTermExtractOnClose = *patch.LongTermExtractOnClose
	}
	if patch.LongTermInjectTopK != nil {
		value.LongTermInjectTopK = *patch.LongTermInjectTopK
	}
	if patch.LongTermExpireDays != nil {
		value.LongTermExpireDays = *patch.LongTermExpireDays
	}
}

func validateMemory(value dto.MemoryConfig) error {
	if value.ShortTermWindowSize < 1 || value.ShortTermWindowSize > 20 || value.SummaryTriggerTokens < 100 || value.SummaryMaxTokens < 50 || value.LongTermInjectTopK < 1 || value.LongTermInjectTopK > 10 || value.LongTermExpireDays < 1 {
		return businessError(400, "400_INVALID_CONFIG", "memoryConfig 参数非法")
	}
	return nil
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
