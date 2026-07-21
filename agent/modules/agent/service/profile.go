package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type resolvedModel struct {
	ModelID    string `gorm:"column:model_id"`
	ProviderID string `gorm:"column:provider_id"`
	Status     string `gorm:"column:status"`
}

// CreateAgent 创建 Agent、初始化默认配置并按请求挂载能力。
//
// 简要描述：Agent 主记录先持久化，能力按源服务顺序逐项挂载；
// 首个 Agent 的赠送资格在同一事务内预留，等 Bot 绑定完成后才实际发放。
func (service *Service) CreateAgent(ctx context.Context, actor Actor, request dto.CreateRequest) (dto.DetailResponse, error) {
	if strings.TrimSpace(actor.AppKey) == "" {
		return dto.DetailResponse{}, businessError(400, "400_APP_KEY_REQUIRED", "创建 Agent 必须提供应用 AppKey")
	}
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
	entity := model.Agent{ID: uuid.NewString(), AppKey: actor.AppKey, OwnerID: actor.OwnerID, Name: name, Type: agentType, Prompt: &prompt, LLMModel: &resolved.ModelID, LLMProviderID: &providerID, Status: "draft", ReactConfig: reactMap(defaultReactConfig()), MemoryConfig: memoryMap(defaultMemoryConfig(summaryModel))}
	// TIPS: 以 AppKey+Owner 哈希作为事务级 advisory lock，串行化计数、创建和赠送资格预留；
	// 预留记录外键跟随 draft Agent 级联删除，外部 Bot 创建失败后可由下一次创建重新取得资格。
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", actor.AppKey+":"+actor.OwnerID).Error; err != nil {
			return err
		}
		var count int64
		// 已删除的 Agent 不占用数量上限。
		if err := tx.Model(&model.Agent{}).Where("app_key = ? AND owner_id = ? AND status <> ?", actor.AppKey, actor.OwnerID, statusDeleted).Count(&count).Error; err != nil {
			return err
		}
		if count >= 50 {
			return businessError(400, "400_LIMIT_EXCEEDED", "单个 Owner 的 Agent 数量已达上限")
		}
		if err := tx.Create(&entity).Error; err != nil {
			return err
		}
		if count == 0 {
			// TIPS: 必须 DO NOTHING 而不是 Create。预留记录以 (app_key, owner_id) 为主键，且只有
			// 物理删除 Agent 才会级联清掉；用户软删掉全部 Agent 后 count 会重新变成 0，直接 Create
			// 会主键冲突并让创建接口报 500。冲突时保留原记录也正好保住“每个 Owner 只赠一次”——
			// 旧记录的 granted_at 已经写上，grantFirstAgentRecharge 不会重复发放。
			return tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "app_key"}, {Name: "owner_id"}},
				DoNothing: true,
			}).Create(&firstAgentRechargeGrant{AppKey: actor.AppKey, OwnerID: actor.OwnerID, AgentID: entity.ID}).Error
		}
		return nil
	})
	if err != nil {
		return dto.DetailResponse{}, err
	}
	if err := service.mountCapabilityBatch(ctx, actor, entity.ID, request.SkillIDs, request.ToolIDs, request.KnowledgeIDs); err != nil {
		// TIPS: 能力挂载属于创建流程的一部分；失败时清理刚创建的 draft Agent，禁止留下半成品。
		if cleanupErr := service.db.WithContext(context.WithoutCancel(ctx)).Where("id=? AND app_key=? AND status='draft'", entity.ID, actor.AppKey).Delete(&model.Agent{}).Error; cleanupErr != nil {
			slog.ErrorContext(ctx, "Agent 能力挂载失败且补偿删除失败", "agent_id", entity.ID, "error", cleanupErr)
		}
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
	if err := assertPermission(actor, entity); err != nil {
		return dto.DetailResponse{}, err
	}
	return service.toDetail(ctx, entity)
}

// ListAgents 分页查询当前 Owner 的 Agent。
func (service *Service) ListAgents(ctx context.Context, appKey, ownerID string, page, pageSize int) (dto.ListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)
	// TIPS: 软删除的 Agent 对列表不可见（行仍在库里用于计费与会话历史追溯）。
	query := service.db.WithContext(ctx).Model(&model.Agent{}).
		Where("app_key = ? AND owner_id = ? AND status <> ?", appKey, ownerID, statusDeleted)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ListResponse{}, err
	}
	var entities []model.Agent
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.ListResponse{}, err
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

// ListActiveAgents 分页查询当前应用下处于 active 状态的 Agent。
//
// TIPS: 与 ListAgents 的差别只有状态口径 —— ListAgents 排除 deleted（draft/paused/
// archived 都会返回），这里只保留 active。前端做「选一个可用 Agent 去对话」时用这个，
// 免得把还没激活或已暂停的 Agent 也摆出来。
func (service *Service) ListActiveAgents(ctx context.Context, appKey, sessionID string, page, pageSize int) (dto.ListResponse, error) {
	page, pageSize = normalizePage(page, pageSize)
	// TIPS: 不按 owner_id 过滤 —— 该接口用于给工单挑选可用 Agent，同一应用下任意 Owner
	// 创建的 active Agent 都应可选。app_key 仍然保留，保证多租户隔离。
	query := service.db.WithContext(ctx).Model(&model.Agent{}).
		Where("app_key = ? AND status = ?", appKey, statusActive)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.ListResponse{}, err
	}
	var entities []model.Agent
	if err := query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.ListResponse{}, err
	}
	knowledgeCounts, err := service.bindingCounts(ctx, "agent_knowledge", entities)
	if err != nil {
		return dto.ListResponse{}, err
	}
	toolCounts, err := service.bindingCounts(ctx, "agent_tools", entities)
	if err != nil {
		return dto.ListResponse{}, err
	}
	// TIPS: 只查一次绑定关系再逐项标记，避免在循环里对每个 Agent 各查一次库。
	// 一个工单至多绑定一个 Agent（唯一索引保证），所以列表里最多有一项 binded=true。
	bindedAgentID, err := service.ticketBoundAgentID(ctx, appKey, sessionID)
	if err != nil {
		return dto.ListResponse{}, err
	}
	items := make([]dto.ConsoleListItem, 0, len(entities))
	for index := range entities {
		item := service.toListItem(&entities[index], knowledgeCounts[entities[index].ID], toolCounts[entities[index].ID])
		item.Binded = bindedAgentID != "" && bindedAgentID == entities[index].ID
		items = append(items, item)
	}
	return dto.ListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ticketBoundAgentID 返回指定工单当前绑定的 Agent ID；未传工单或未绑定时返回空串。
//
// @param sessionID 工单 ID（对前端沿用 session_id 的叫法）
func (service *Service) ticketBoundAgentID(ctx context.Context, appKey, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", nil
	}
	var binding model.TicketBinding
	err := service.db.WithContext(ctx).
		Where("app_key = ? AND ticket_id = ?", appKey, sessionID).
		Take(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return binding.AgentID, nil
}

// BindTicketAgent 建立或覆盖工单与 Agent 的关联关系。
//
// TIPS: 只写绑定关系，不做任何 IM 侧操作 —— 不改群成员、不动 Bot、不影响入站路由。
// 重复绑定按覆盖处理（同一工单换绑另一个 Agent），依赖唯一索引 uq_tab_app_ticket。
//
// @param appKey 应用 AppKey
// @param sessionID 工单 ID
// @param agentID 目标 Agent ID，必须是同一应用下的 active Agent
func (service *Service) BindTicketAgent(ctx context.Context, appKey, sessionID, agentID string) (dto.BindTicketResponse, error) {
	appKey, sessionID, agentID = strings.TrimSpace(appKey), strings.TrimSpace(sessionID), strings.TrimSpace(agentID)
	if appKey == "" || sessionID == "" || agentID == "" {
		return dto.BindTicketResponse{}, businessError(400, "400_INVALID_REQUEST", "appKey、sessionId、agentId 不能为空")
	}
	var ticketCount int64
	if err := service.db.WithContext(ctx).Table("tickets").
		Where("app_key = ? AND ticket_id = ?", appKey, sessionID).Count(&ticketCount).Error; err != nil {
		return dto.BindTicketResponse{}, err
	}
	if ticketCount == 0 {
		return dto.BindTicketResponse{}, businessError(404, "404_TICKET_NOT_FOUND", "工单不存在")
	}
	var agentCount int64
	if err := service.db.WithContext(ctx).Model(&model.Agent{}).
		Where("id = ? AND status = ? AND app_key = ?", agentID, statusActive, appKey).
		Count(&agentCount).Error; err != nil {
		return dto.BindTicketResponse{}, err
	}
	if agentCount == 0 {
		return dto.BindTicketResponse{}, businessError(404, "404_AGENT_NOT_BINDABLE", "Agent 不存在、未激活或不属于当前应用")
	}
	binding := model.TicketBinding{ID: uuid.NewString(), AppKey: appKey, TicketID: sessionID, AgentID: agentID}
	if err := service.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_key"}, {Name: "ticket_id"}},
		DoUpdates: clause.Assignments(map[string]any{"agent_id": agentID, "updated_at": time.Now().UTC()}),
	}).Create(&binding).Error; err != nil {
		return dto.BindTicketResponse{}, err
	}
	return dto.BindTicketResponse{SessionID: sessionID, AgentID: agentID, Binded: true}, nil
}

// UnbindTicketAgent 解除工单当前的 Agent 绑定。
//
// TIPS: 解绑只删除选择关系，不产生 IM 侧动作。删除使用 RETURNING 原子返回原 Agent ID，
// 避免“先查再删”期间发生换绑时返回错误的 Agent；未绑定时按幂等成功处理。
//
// @param appKey 应用 AppKey
// @param sessionID 工单 ID
func (service *Service) UnbindTicketAgent(ctx context.Context, appKey, sessionID string) (dto.BindTicketResponse, error) {
	appKey, sessionID = strings.TrimSpace(appKey), strings.TrimSpace(sessionID)
	if appKey == "" || sessionID == "" {
		return dto.BindTicketResponse{}, businessError(400, "400_INVALID_REQUEST", "appKey、sessionId 不能为空")
	}
	binding := model.TicketBinding{}
	result := service.db.WithContext(ctx).Clauses(clause.Returning{Columns: []clause.Column{{Name: "agent_id"}}}).
		Where("app_key = ? AND ticket_id = ?", appKey, sessionID).
		Delete(&binding)
	if result.Error != nil {
		return dto.BindTicketResponse{}, result.Error
	}
	return dto.BindTicketResponse{SessionID: sessionID, AgentID: binding.AgentID, Binded: false}, nil
}

// UpdateAgent 按补丁更新 Agent Profile、记忆配置和目标能力集合。
func (service *Service) UpdateAgent(ctx context.Context, actor Actor, request dto.UpdateRequest) (dto.DetailResponse, error) {
	entity, err := service.findAgent(ctx, request.AgentID)
	if err != nil {
		return dto.DetailResponse{}, err
	}
	if err := assertPermission(actor, entity); err != nil {
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

// DeleteAgent 软删除 Agent，并清理它的全部关联关系。
//
// 简要描述：任意状态均可删除。删除会依次解绑 Inbox（含把 Bot 移出未关闭
// Ticket 群）、停用 Bot 与 Bot-Agent 绑定、卸载知识/技能/工具，最后把 Agent 置为 deleted。
//
// TIPS: 这里刻意不物理删除。conversations、consumption_records（计费）、llm_model_calls 等表
// 带 agent_id 但没有外键，物理删除会留下孤儿数据并丢失账务追溯；置为 deleted 后由查询侧统一排除。
//
// TIPS: 先做外部副作用（移出 Ticket 群）再落库，顺序与 BindInboxAgent 一致 —— 群没清干净时
// 保留原状态，用户可以重试；反过来先落库会让群里永远留下一个不再响应的僵尸 Bot。
func (service *Service) DeleteAgent(ctx context.Context, actor Actor, agentID string) error {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return err
	}
	// 已删除的 Agent 在 findAgent 处就会返回 404，这里不需要再判断 deleted。
	if err := assertPermission(actor, entity); err != nil {
		return err
	}

	if err := service.unbindAgentInboxes(ctx, entity); err != nil {
		return err
	}

	var botUserIDs []string
	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Bot 与 Bot 绑定一并停用：Bot 常驻连接是按 bots.status='active' 建立的，
		// 只删绑定会让 Bot 继续连着 IM 收消息，却再也路由不到任何 Agent。
		var bots []struct {
			ID        string
			BotUserID string
		}
		if err := tx.Table("bots b").Select("b.id,b.bot_user_id").
			Joins("JOIN bot_agent_bindings bab ON bab.bot_id=b.id AND bab.status='active'").
			Where("bab.agent_id = ? AND b.app_key = ?", entity.ID, entity.AppKey).
			Find(&bots).Error; err != nil {
			return err
		}
		botIDs := make([]string, 0, len(bots))
		for _, bot := range bots {
			botIDs = append(botIDs, bot.ID)
			botUserIDs = append(botUserIDs, bot.BotUserID)
		}
		if err := tx.Table("bot_agent_bindings").Where("agent_id = ?", entity.ID).
			Update("status", "inactive").Error; err != nil {
			return err
		}
		if len(botIDs) > 0 {
			if err := tx.Table("bots").Where("id IN ? AND app_key = ?", botIDs, entity.AppKey).
				Update("status", "inactive").Error; err != nil {
				return err
			}
		}
		// 卸载能力挂载（这些表有 ON DELETE CASCADE，但软删不会触发级联，需要显式清理）。
		for _, table := range []string{"agent_knowledge", "agent_skills", "agent_tools"} {
			if err := tx.Table(table).Where("agent_id = ?", entity.ID).Delete(nil).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&model.Agent{}).Where("id = ? AND status <> ?", entity.ID, statusDeleted).
			Updates(map[string]any{"status": statusDeleted, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 并发删除：另一个请求已经把它置为 deleted。
			return businessError(404, "404_NOT_FOUND", "Agent 不存在")
		}
		return nil
	}); err != nil {
		return err
	}
	// 落库成功后再断连：先断连若事务回滚，会白白干掉一个仍然有效的 Bot 连接。
	for _, botUserID := range botUserIDs {
		service.connections.DisconnectBot(entity.AppKey, botUserID)
	}
	return nil
}

// purgeDraftAgent 物理删除刚创建但未完成的 draft Agent，仅供创建流程的补偿路径使用。
//
// TIPS: 这里必须是物理删除，不能复用 DeleteAgent 的软删。agent_first_recharge_grants 以
// (app_key, owner_id) 为主键、并靠外键跟随 Agent 级联删除；半成品若只置为 deleted，预留记录
// 会残留且 agent_id 永远指向那个失败的 Agent，导致：下次创建插入预留记录时主键冲突（表现为
// 创建接口 500），且 grantFirstAgentRecharge 再也匹配不到，用户永远拿不到首个 Agent 赠送。
func (service *Service) purgeDraftAgent(ctx context.Context, appKey, agentID string) error {
	return service.db.WithContext(ctx).
		Where("id = ? AND app_key = ? AND status = 'draft'", agentID, appKey).
		Delete(&model.Agent{}).Error
}

// unbindAgentInboxes 解除该 Agent 当前生效的全部 Inbox 绑定，并把 Bot 移出对应 Ticket 群。
func (service *Service) unbindAgentInboxes(ctx context.Context, entity *model.Agent) error {
	var inboxIDs []string
	if err := service.db.WithContext(ctx).Table("inbox_agent_bindings").
		Where("app_key = ? AND agent_id = ? AND status = 'active'", entity.AppKey, entity.ID).
		Pluck("inbox_id", &inboxIDs).Error; err != nil {
		return err
	}
	if len(inboxIDs) == 0 {
		return nil
	}
	if service.unbindInboxAgent == nil {
		return businessError(500, "500_UNBIND_INBOX_NOT_CONFIGURED", "Inbox 解绑能力未装配，无法删除已关联 Inbox 的 Agent")
	}
	for _, inboxID := range inboxIDs {
		if err := service.unbindInboxAgent(ctx, entity.AppKey, inboxID); err != nil {
			slog.ErrorContext(ctx, "删除 Agent 时解绑 Inbox 失败", "agent_id", entity.ID, "inbox_id", inboxID, "error", err)
			return businessError(502, "502_INBOX_UNBIND_FAILED", fmt.Sprintf("解绑 Inbox %s 失败，请重试：%v", inboxID, err))
		}
	}
	return nil
}

// ActivateAgent 校验完整配置、模型和余额后启动 Agent。
func (service *Service) ActivateAgent(ctx context.Context, actor Actor, agentID string) (dto.LifecycleResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.LifecycleResponse{}, err
	}
	if err := assertPermission(actor, entity); err != nil {
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
	if err := assertPermission(actor, entity); err != nil {
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
	if err := assertPermission(actor, entity); err != nil {
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
