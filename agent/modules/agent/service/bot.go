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
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BotUserIDPrefix 是 Bot IM 身份的统一前缀。
const BotUserIDPrefix = "bot-"

// newBotUserID 生成注册 IM Bot 时使用的身份标识。
//
// TIPS: 复用 tools.GenerateUUIDShort22，与 ticket_/customer_ 等业务 ID 保持同一套
// 生成规则；它把 UUID 的 128 位压成 22 个字符，比 UUID 去连字符的 32 位十六进制更短，
// 且同样来自 uuid.New()，碰撞概率不变。
//
// 注意：该函数只影响新注册的 Bot。已入库的 bot_user_id 是 IM 侧身份，不会也不应被回溯
// 改写，因此库里长期会同时存在新旧两种长度的值，任何地方都不要按长度或格式解析它。
func newBotUserID() string {
	return BotUserIDPrefix + tools.GenerateUUIDShort22()
}

// CreateAgentWithBot 创建 Agent、注册 IM Bot、持久化绑定并尝试建立长连接。
//
// TIPS: 外部 Bot 注册无法加入 PostgreSQL 事务，因此先创建可补偿删除的 draft Agent；
// 远端注册或本地 Bot 绑定失败时都删除刚创建的 Agent，避免控制台出现不可用半成品。
func (service *Service) CreateAgentWithBot(ctx context.Context, actor Actor, request dto.CreateWithBotRequest) (dto.CreateWithBotResponse, error) {
	if service.register == nil {
		return dto.CreateWithBotResponse{}, businessError(500, "500_IMBOT_SERVER_API_NOT_CONFIGURED", "IM Bot 注册服务未装配")
	}
	botName := "bot"
	if request.BotName != nil {
		botName = strings.TrimSpace(*request.BotName)
	}
	name, agentType, prompt := request.Name, request.Type, request.Prompt
	created, err := service.CreateAgent(ctx, actor, dto.CreateRequest{Name: &name, Type: &agentType, Prompt: &prompt, LLMModel: request.LLMModel, LLMProviderID: request.LLMProviderID, SkillIDs: request.SkillIDs, ToolIDs: request.ToolIDs, KnowledgeIDs: request.KnowledgeIDs})
	if err != nil {
		return dto.CreateWithBotResponse{}, err
	}
	registered, err := service.register.RegisterBot(ctx, actor.AppKey, newBotUserID(), botName)
	if err != nil {
		if cleanupErr := service.purgeDraftAgent(context.WithoutCancel(ctx), actor.AppKey, created.ID); cleanupErr != nil {
			slog.ErrorContext(ctx, "IM Bot 注册失败且补偿删除 Agent 失败", "agent_id", created.ID, "error", cleanupErr)
		}
		return dto.CreateWithBotResponse{}, mapIMError(err)
	}
	bot := model.Bot{ID: uuid.NewString(), AppKey: actor.AppKey, OwnerID: actor.OwnerID, InviteCode: actor.OwnerID, BotUserID: registered.UserID, BotName: botName, Token: registered.Token, Status: "active"}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&bot).Error; err != nil {
			return err
		}
		return tx.Create(&model.BotBinding{ID: uuid.NewString(), BotID: bot.ID, AgentID: created.ID, Status: "active"}).Error
	})
	if err != nil {
		if cleanupErr := service.purgeDraftAgent(context.WithoutCancel(ctx), actor.AppKey, created.ID); cleanupErr != nil {
			slog.ErrorContext(ctx, "Agent Bot 绑定失败且补偿删除 Agent 失败", "agent_id", created.ID, "bot_user_id", registered.UserID, "error", cleanupErr)
		}
		return dto.CreateWithBotResponse{}, err
	}
	service.grantFirstAgentRecharge(ctx, actor, created.ID)
	connected := service.ensureBotConnection(ctx, bot.AppKey, bot.Token, bot.BotUserID)
	return dto.CreateWithBotResponse{Agent: created, OwnerID: actor.OwnerID, BotUserID: bot.BotUserID, BotName: bot.BotName, Connected: connected}, nil
}

type firstAgentRechargeGrant struct {
	AppKey    string     `gorm:"column:app_key;primaryKey"`
	OwnerID   string     `gorm:"column:owner_id;primaryKey"`
	AgentID   string     `gorm:"column:agent_id;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
	GrantedAt *time.Time `gorm:"column:granted_at"`
}

func (firstAgentRechargeGrant) TableName() string { return "agent_first_recharge_grants" }

func (service *Service) grantFirstAgentRecharge(ctx context.Context, actor Actor, agentID string) {
	if service.billing == nil || !service.config.FirstAgentRechargeAmount.IsPositive() {
		return
	}
	var grant firstAgentRechargeGrant
	result := service.db.WithContext(ctx).Where("app_key=? AND owner_id=? AND agent_id=? AND granted_at IS NULL", actor.AppKey, actor.OwnerID, agentID).First(&grant)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.WarnContext(ctx, "查询首个 Agent 充值赠送资格失败", "agent_id", agentID, "error", result.Error)
		}
		return
	}
	if _, err := service.billing.CreateCompletedRecharge(ctx, actor.OwnerID, service.config.FirstAgentRechargeAmount); err != nil {
		slog.WarnContext(ctx, "首个 Agent 自动充值赠送失败", "owner_id", actor.OwnerID, "error", err)
		return
	}
	now := time.Now().UTC()
	if err := service.db.WithContext(ctx).Model(&firstAgentRechargeGrant{}).
		Where("app_key=? AND owner_id=? AND agent_id=? AND granted_at IS NULL", actor.AppKey, actor.OwnerID, agentID).
		Update("granted_at", now).Error; err != nil {
		// TIPS: 充值已经入账，此处不删除资格记录，避免在不确定状态下自动重复发放。
		slog.ErrorContext(ctx, "标记首个 Agent 充值赠送已完成失败，需人工核对", "agent_id", agentID, "error", err)
	}
}

// BindExistingBot 将已有或请求内创建的 Bot 幂等绑定到 Agent。
func (service *Service) BindExistingBot(ctx context.Context, actor Actor, agentID, botUserID, token string, botName *string, createBot bool) (dto.BindBotResponse, error) {
	entity, err := service.findAgent(ctx, agentID)
	if err != nil {
		return dto.BindBotResponse{}, err
	}
	if err := assertPermission(actor, entity, false); err != nil {
		return dto.BindBotResponse{}, err
	}
	if entity.Status == "archived" {
		return dto.BindBotResponse{}, businessError(409, "409_AGENT_ARCHIVED", "Agent 已归档，不能绑定 Bot")
	}
	resolvedName := strings.TrimSpace(botUserID)
	if botName != nil && strings.TrimSpace(*botName) != "" {
		resolvedName = strings.TrimSpace(*botName)
	}
	var bot model.Bot
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("app_key = ? AND bot_user_id = ?", actor.AppKey, botUserID).First(&bot).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if !createBot {
				return businessError(404, "404_BOT_NOT_FOUND", "Bot 不存在且 createBot=false")
			}
			bot = model.Bot{ID: uuid.NewString(), AppKey: actor.AppKey, OwnerID: actor.OwnerID, InviteCode: actor.OwnerID, BotUserID: botUserID, BotName: resolvedName, Token: token, Status: "active"}
			if err := tx.Create(&bot).Error; err != nil {
				return err
			}
		} else if findErr != nil {
			return findErr
		} else {
			if bot.OwnerID != actor.OwnerID {
				return businessError(409, "409_BOT_OWNER_MISMATCH", "Bot 已归属其他 Owner，无法绑定到当前 Agent")
			}
			if err := tx.Model(&bot).Updates(map[string]any{"token": token, "bot_name": resolvedName, "status": "active"}).Error; err != nil {
				return err
			}
		}
		var binding model.BotBinding
		bindErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("bot_id = ?", bot.ID).First(&binding).Error
		if bindErr == nil && binding.AgentID != agentID {
			return businessError(409, "409_BOT_ALREADY_BOUND", "Bot 已绑定到其他 Agent")
		}
		if bindErr != nil && !errors.Is(bindErr, gorm.ErrRecordNotFound) {
			return bindErr
		}
		if errors.Is(bindErr, gorm.ErrRecordNotFound) {
			var activeCount int64
			if err := tx.Model(&model.BotBinding{}).Where("agent_id=? AND status='active'", agentID).Count(&activeCount).Error; err != nil {
				return err
			}
			if activeCount > 0 {
				return businessError(409, "409_AGENT_ALREADY_HAS_BOT", "Agent 已绑定 active Bot")
			}
			return tx.Create(&model.BotBinding{ID: uuid.NewString(), BotID: bot.ID, AgentID: agentID, Status: "active"}).Error
		}
		return tx.Model(&binding).Updates(map[string]any{"status": "active"}).Error
	})
	if err != nil {
		return dto.BindBotResponse{}, err
	}
	connected := service.ensureBotConnection(ctx, actor.AppKey, token, botUserID)
	detail, err := service.GetAgent(ctx, actor, agentID)
	if err != nil {
		return dto.BindBotResponse{}, err
	}
	return dto.BindBotResponse{Agent: detail, OwnerID: actor.OwnerID, BotID: bot.ID, BotUserID: bot.BotUserID, BotName: resolvedName, Connected: connected}, nil
}

func (service *Service) ensureBotConnection(ctx context.Context, appKey, token, userID string) bool {
	if service.connections == nil {
		return false
	}
	if _, err := service.connections.EnsureBot(ctx, appKey, token, userID); err != nil {
		slog.WarnContext(ctx, "建立 IM Bot 长连接失败，等待启动重连", "bot_user_id", userID, "error", err)
		return false
	}
	return true
}

func mapIMError(err error) error {
	var imError *imbot.Error
	if errors.As(err, &imError) {
		return businessError(imError.Status, imError.Code, imError.Message)
	}
	return err
}
