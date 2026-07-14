package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateAgentWithBot 创建 Agent、注册 IM Bot、持久化绑定并尝试建立长连接。
//
// 简要描述：编排顺序与源服务一致，先创建本地 Agent 和能力，再调用 IM Server；
// 远端注册失败时保留已创建 Agent 便于修复重试，连接失败只返回 connected=false。
func (service *Service) CreateAgentWithBot(ctx context.Context, request dto.CreateWithBotRequest) (dto.CreateWithBotResponse, error) {
	if service.register == nil {
		return dto.CreateWithBotResponse{}, businessError(500, "500_IMBOT_SERVER_API_NOT_CONFIGURED", "IM Bot 注册服务未装配")
	}
	botName := "bot"
	if request.BotName != nil {
		botName = strings.TrimSpace(*request.BotName)
	}
	name, agentType, prompt := request.Name, request.Type, request.Prompt
	created, err := service.CreateAgent(ctx, Actor{OwnerID: request.InviteCode}, dto.CreateRequest{Name: &name, Type: &agentType, Prompt: &prompt, LLMModel: request.LLMModel, LLMProviderID: request.LLMProviderID, SkillIDs: request.SkillIDs, ToolIDs: request.ToolIDs, KnowledgeIDs: request.KnowledgeIDs})
	if err != nil {
		return dto.CreateWithBotResponse{}, err
	}
	registered, err := service.register.RegisterBot(ctx, "bot-"+strings.ReplaceAll(uuid.NewString(), "-", ""), botName)
	if err != nil {
		return dto.CreateWithBotResponse{}, mapIMError(err)
	}
	bot := model.Bot{ID: uuid.NewString(), OwnerID: request.InviteCode, InviteCode: request.InviteCode, BotUserID: registered.UserID, BotName: botName, Token: registered.Token, Status: "active"}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&bot).Error; err != nil {
			return err
		}
		return tx.Create(&model.BotBinding{ID: uuid.NewString(), BotID: bot.ID, AgentID: created.ID, Status: "active"}).Error
	})
	if err != nil {
		return dto.CreateWithBotResponse{}, err
	}
	connected := service.ensureBotConnection(ctx, bot.Token, bot.BotUserID)
	return dto.CreateWithBotResponse{Agent: created, OwnerID: request.InviteCode, BotUserID: bot.BotUserID, BotName: bot.BotName, Connected: connected}, nil
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
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("bot_user_id = ?", botUserID).First(&bot).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if !createBot {
				return businessError(404, "404_BOT_NOT_FOUND", "Bot 不存在且 createBot=false")
			}
			bot = model.Bot{ID: uuid.NewString(), OwnerID: actor.OwnerID, InviteCode: actor.OwnerID, BotUserID: botUserID, BotName: resolvedName, Token: token, Status: "active"}
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
			return tx.Create(&model.BotBinding{ID: uuid.NewString(), BotID: bot.ID, AgentID: agentID, Status: "active"}).Error
		}
		return tx.Model(&binding).Updates(map[string]any{"status": "active"}).Error
	})
	if err != nil {
		return dto.BindBotResponse{}, err
	}
	connected := service.ensureBotConnection(ctx, token, botUserID)
	detail, err := service.GetAgent(ctx, actor, agentID)
	if err != nil {
		return dto.BindBotResponse{}, err
	}
	return dto.BindBotResponse{Agent: detail, OwnerID: actor.OwnerID, BotID: bot.ID, BotUserID: bot.BotUserID, BotName: resolvedName, Connected: connected}, nil
}

func (service *Service) ensureBotConnection(ctx context.Context, token, userID string) bool {
	if service.connections == nil {
		return false
	}
	if _, err := service.connections.EnsureBot(ctx, token, userID); err != nil {
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
