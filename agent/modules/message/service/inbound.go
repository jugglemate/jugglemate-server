package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	reasoningmodel "github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	"gorm.io/gorm"
)

// HandleInbound 处理 IM Bot 入站文本并完成人工分流或 Agent 自动回复。
//
// 简要描述：数据库 message_id 是跨重连幂等依据；路由优先使用 active Bot 绑定，未绑定时
// 回退系统 Agent。命中人工 TTL 时只落用户消息和通知，不触发任何 LLM 调用。
func (service *Service) HandleInbound(ctx context.Context, inbound imbot.InboundMessage) {
	if inbound.MessageID != "" {
		var count int64
		if service.db.WithContext(ctx).Model(&reasoningmodel.Message{}).Where("message_id = ?", inbound.MessageID).Count(&count).Error == nil && count > 0 {
			return
		}
	}
	agent, inviteCode, err := service.resolveInboundAgent(ctx, inbound.BotUserID)
	if err != nil {
		slog.ErrorContext(ctx, "IM 入站路由失败", "bot_user_id", inbound.BotUserID, "error", err)
		return
	}
	manual, _, err := service.activeHumanSession(ctx, agent.ID, inbound.SenderID)
	if err != nil {
		slog.ErrorContext(ctx, "IM 人工状态查询失败", "agent_id", agent.ID, "error", err)
	}
	if manual != nil {
		conversation, createErr := service.latestOrCreateConversation(ctx, agent.ID, inbound.SenderID, map[string]any{"invite_code": inviteCode})
		if createErr != nil {
			slog.ErrorContext(ctx, "IM 人工消息会话创建失败", "error", createErr)
			return
		}
		messageID := inbound.MessageID
		message := reasoningmodel.Message{ID: uuid.NewString(), ConversationID: conversation.ID, UserID: &inbound.SenderID, MessageID: &messageID, Role: "user", Content: inbound.Text, Source: "webhook", SelfTokenCount: estimate(inbound.Text)}
		if createErr = service.db.WithContext(ctx).Create(&message).Error; createErr != nil {
			slog.ErrorContext(ctx, "IM 人工消息落库失败", "error", createErr)
			return
		}
		if createErr = service.redis.Set(ctx, humanPollKey(agent.ID, inbound.SenderID), "1", humanSessionTTL).Err(); createErr != nil {
			slog.ErrorContext(ctx, "IM 人工轮询通知失败", "error", createErr)
		}
		return
	}
	request := reasoningservice.Request{AgentID: agent.ID, OwnerID: agent.OwnerID, UserID: inbound.SenderID, Input: inbound.Text, EnableHistoryContext: true, Metadata: map[string]any{"source": "webhook", "invite_code": inviteCode, "bot_user_id": inbound.BotUserID, "message_id": inbound.MessageID}}
	buffer, platformMessageID, sequence := "", "", 0
	_, err = service.reasoning.Stream(ctx, request, func(event reasoningservice.Event) error {
		if event.Event != "token" {
			return nil
		}
		token, _ := event.Payload["token"].(string)
		if token == "" {
			return nil
		}
		buffer += token
		if len([]rune(buffer))%30 != 0 {
			return nil
		}
		if platformMessageID == "" {
			messageID, sendErr := service.connections.SendStreamText(ctx, inbound.BotUserID, inbound.SenderID, buffer, sequence, false)
			if sendErr != nil {
				return sendErr
			}
			platformMessageID = messageID
		} else if modifyErr := service.connections.ModifyStreamText(ctx, inbound.BotUserID, inbound.SenderID, platformMessageID, buffer, sequence, false); modifyErr != nil {
			return modifyErr
		}
		sequence++
		return nil
	})
	if err != nil {
		slog.ErrorContext(ctx, "IM 入站推理或流式回发失败", "agent_id", agent.ID, "error", err)
		if platformMessageID == "" {
			_, _ = service.connections.SendText(context.WithoutCancel(ctx), inbound.BotUserID, inbound.SenderID, "系统繁忙，请稍后重试")
		}
		return
	}
	buffer = strings.TrimSpace(buffer)
	if buffer == "" {
		buffer = "系统繁忙，请稍后重试"
	}
	if platformMessageID == "" {
		platformMessageID, err = service.connections.SendStreamText(context.WithoutCancel(ctx), inbound.BotUserID, inbound.SenderID, buffer, sequence, true)
	} else {
		err = service.connections.ModifyStreamText(context.WithoutCancel(ctx), inbound.BotUserID, inbound.SenderID, platformMessageID, buffer, sequence, true)
	}
	if err != nil {
		slog.ErrorContext(ctx, "IM 最终流式回发失败", "agent_id", agent.ID, "error", err)
	}
}

func (service *Service) resolveInboundAgent(ctx context.Context, botUserID string) (agentmodel.Agent, string, error) {
	var row struct {
		AgentID    string
		InviteCode string
	}
	err := service.db.WithContext(ctx).Table("bots b").Select("bab.agent_id,b.invite_code").Joins("JOIN bot_agent_bindings bab ON bab.bot_id=b.id").Where("b.bot_user_id=? AND b.status='active' AND bab.status='active'", botUserID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row.AgentID = "Juggle_Agent"
		row.InviteCode = "system"
	} else if err != nil {
		return agentmodel.Agent{}, "", err
	}
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).First(&agent, "id = ?", row.AgentID).Error; err != nil {
		return agentmodel.Agent{}, "", err
	}
	return agent, row.InviteCode, nil
}
