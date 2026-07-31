package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/juggleim/imbot-sdk-go/imbotclients/pbdefines/pbobjs"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	reasoningmodel "github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	"gorm.io/gorm"
)

// fallbackReply 是推理失败或结果为空时回给客户的兜底文案。
const fallbackReply = "系统繁忙，请稍后重试"

// HandleInbound 处理 IM Bot 入站文本并完成人工分流或 Agent 自动回复。
//
// 简要描述：数据库 message_id 是跨重连幂等依据；仅当前 Inbox 绑定 Bot 收到的 Ticket
// 群客户消息可以触发 Agent。命中人工 TTL 时只落用户消息和通知，不触发任何 LLM 调用。
func (service *Service) HandleInbound(ctx context.Context, inbound imbot.InboundMessage) {
	slog.InfoContext(ctx, "[Inbound] 进入入站处理", "app_key", inbound.AppKey, "bot_user_id", inbound.BotUserID,
		"sender_id", inbound.SenderID, "target_id", inbound.TargetID, "channel_type", int(inbound.ChannelType), "msg_id", inbound.MessageID)
	if inbound.ChannelType != pbobjs.ChannelType_Group || strings.TrimSpace(inbound.TargetID) == "" {
		slog.InfoContext(ctx, "[Inbound] 跳过：非群会话或 target 为空", "channel_type", int(inbound.ChannelType), "target_id", inbound.TargetID, "msg_id", inbound.MessageID)
		return
	}
	// 双重保险：人工接管后不走大模型推理。
	// OnMessageReceive 处已有拦截，此处防止未来任何调用链路绕过去。
	if service.isTicketHumanTakenOver(ctx, inbound.AppKey, inbound.TargetID) {
		slog.InfoContext(ctx, "[Inbound] 跳过：工单已人工接管（双重保险）", "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID)
		return
	}
	if inbound.MessageID != "" {
		var count int64
		if service.db.WithContext(ctx).Model(&reasoningmodel.Message{}).Where("message_id = ?", inbound.MessageID).Count(&count).Error == nil && count > 0 {
			slog.InfoContext(ctx, "[Inbound] 跳过：消息已处理（幂等）", "msg_id", inbound.MessageID)
			return
		}
	}
	agent, inviteCode, err := service.resolveInboundAgent(ctx, inbound)
	if err != nil {
		slog.ErrorContext(ctx, "[Inbound] IM 入站路由失败", "app_key", inbound.AppKey, "bot_user_id", inbound.BotUserID,
			"ticket_id", inbound.TargetID, "sender_id", inbound.SenderID, "error", err)
		return
	}
	slog.InfoContext(ctx, "[Inbound] 路由命中 Agent", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID)
	// TIPS: 转人工后非 Widget 渠道 Bot 已被移出群，IM 不再投递消息；
	// Widget 渠道 Bot 保留在群内但 OnMessageReceive 处已拦截，此处作为双重保险。
	if MatchHandoffKeyword(inbound.Text) {
		slog.InfoContext(ctx, "[Inbound] 客户触发转人工", "agent_id", agent.ID, "ticket_id", inbound.TargetID,
			"sender_id", inbound.SenderID, "msg_id", inbound.MessageID)
		// TIPS: 先持久化「工单转人工」事实 —— 哪怕 IM 后续失败，事件流水与 is_human_taken_over
		// 标记也已经留下审计痕迹；失败时记录错误日志但不阻塞后续 IM 副作用（与原逻辑保持
		// 一致：IM 失败仍继续走 Agent 兜底）。
		if service.markHumanTakeover != nil {
			if markErr := service.markHumanTakeover(ctx, inbound.AppKey, inbound.TargetID, inbound.SenderID, "customer"); markErr != nil {
				slog.ErrorContext(ctx, "[Inbound] 写入转人工事实失败", "agent_id", agent.ID, "ticket_id", inbound.TargetID,
					"sender_id", inbound.SenderID, "msg_id", inbound.MessageID, "error", markErr)
			}
		}
		if handoffErr := service.handoffToHuman(ctx, inbound.AppKey, inbound.TargetID, inbound.BotUserID); handoffErr != nil {
			slog.ErrorContext(ctx, "[Inbound] 转人工失败", "agent_id", agent.ID, "ticket_id", inbound.TargetID,
				"bot_user_id", inbound.BotUserID, "msg_id", inbound.MessageID, "error", handoffErr)
			// TIPS: 切换失败时 Bot 仍在群里，继续走下面的 Agent 回复兜底，
			// 至少保证客户有人应答，而不是石沉大海。
		} else {
			slog.InfoContext(ctx, "[Inbound] 转人工完成，坐席已入群、Bot 已退群", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID)
			return
		}
	}
	request := reasoningservice.Request{AgentID: agent.ID, OwnerID: agent.OwnerID, UserID: inbound.SenderID, Input: inbound.Text, EnableHistoryContext: true, Metadata: map[string]any{"source": "ticket_group", "invite_code": inviteCode, "app_key": inbound.AppKey, "bot_user_id": inbound.BotUserID, "message_id": inbound.MessageID, "ticket_id": inbound.TargetID}}
	// TIPS: Ticket 群回复走非流式 Run + jg:text 整条发送，不用 jg:streamtext。
	// 流式方案需要「先发一条 is_finished=false 的消息，再用 ModifyMsg 反复原地改写」，
	// 而 ModifyMsg 不产生新消息事件，客户端必须额外监听消息更新才能拿到后续内容；
	// 中途出错还会留下一条永远标记不了完成的半截消息。整条发送对客户端只有一条普通
	// 文本消息，语义简单且无残留态。Run 与 Stream 共用同一条推理链，结果完全一致。
	result, err := service.reasoning.Run(ctx, request)
	if err != nil {
		slog.ErrorContext(ctx, "[Inbound] IM 入站推理失败", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID, "error", err)
		if _, sendErr := service.connections.SendText(context.WithoutCancel(ctx), inbound.AppKey, inbound.BotUserID, inbound.TargetID, inbound.ChannelType, fallbackReply); sendErr != nil {
			slog.ErrorContext(ctx, "[Inbound] 兜底文案发送失败", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "error", sendErr)
		}
		return
	}
	answer := strings.TrimSpace(result.Answer)
	if answer == "" {
		slog.WarnContext(ctx, "[Inbound] 推理返回空回答，改发兜底文案", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID)
		answer = fallbackReply
	}
	messageID, err := service.connections.SendText(context.WithoutCancel(ctx), inbound.AppKey, inbound.BotUserID, inbound.TargetID, inbound.ChannelType, answer)
	if err != nil {
		slog.ErrorContext(ctx, "[Inbound] IM 回复发送失败", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "msg_id", inbound.MessageID, "error", err)
		return
	}
	slog.InfoContext(ctx, "[Inbound] 回复已发送", "agent_id", agent.ID, "ticket_id", inbound.TargetID, "in_msg_id", inbound.MessageID, "out_msg_id", messageID, "answer_len", len([]rune(answer)))
}

// logInboundRouteMiss 在入站路由 JOIN 未命中时逐条定位断点，避免只看到一个笼统的 404。
//
// TIPS: resolveInboundAgent 把 App、Ticket 状态、客户来源、Inbox 绑定、Bot 绑定五个条件压在
// 一条 JOIN 里，任何一环缺失都只报 404。这里把每一环单独查一遍，直接指出是哪一环断的。
func (service *Service) logInboundRouteMiss(ctx context.Context, inbound imbot.InboundMessage) {
	var ticket struct {
		AppKey   string
		InboxID  string
		SourceID string
		Status   int
	}
	if err := service.db.WithContext(ctx).Table("tickets").
		Select("app_key,inbox_id,source_id,status").
		Where("ticket_id=?", inbound.TargetID).Take(&ticket).Error; err != nil {
		slog.WarnContext(ctx, "[Inbound] 路由未命中：tickets 表查不到该 Ticket", "ticket_id", inbound.TargetID, "error", err)
		return
	}
	slog.WarnContext(ctx, "[Inbound] 路由未命中：Ticket 现状",
		"ticket_id", inbound.TargetID,
		"ticket_app_key", ticket.AppKey, "inbound_app_key", inbound.AppKey, "app_key_match", ticket.AppKey == inbound.AppKey,
		"ticket_source_id", ticket.SourceID, "inbound_sender_id", inbound.SenderID, "sender_is_source", ticket.SourceID == inbound.SenderID,
		"ticket_status", ticket.Status, "is_closed", ticket.Status == 2,
		"inbox_id", ticket.InboxID)

	var binding struct {
		AgentID string
		BotID   string
		Status  string
	}
	if err := service.db.WithContext(ctx).Table("inbox_agent_bindings").
		Select("agent_id,bot_id,status").
		Where("app_key=? AND inbox_id=?", ticket.AppKey, ticket.InboxID).Take(&binding).Error; err != nil {
		slog.WarnContext(ctx, "[Inbound] 路由未命中：该 Inbox 没有 Agent 绑定记录", "inbox_id", ticket.InboxID, "error", err)
		return
	}
	slog.WarnContext(ctx, "[Inbound] 路由未命中：Inbox 绑定现状",
		"inbox_id", ticket.InboxID, "agent_id", binding.AgentID, "bot_id", binding.BotID, "binding_status", binding.Status)

	var bot struct {
		BotUserID string
		Status    string
	}
	if err := service.db.WithContext(ctx).Table("bots").
		Select("bot_user_id,status").
		Where("id=?", binding.BotID).Take(&bot).Error; err != nil {
		slog.WarnContext(ctx, "[Inbound] 路由未命中：bots 表查不到绑定的 Bot", "bot_id", binding.BotID, "error", err)
		return
	}
	slog.WarnContext(ctx, "[Inbound] 路由未命中：Bot 现状",
		"bot_id", binding.BotID, "db_bot_user_id", bot.BotUserID, "inbound_bot_user_id", inbound.BotUserID,
		"bot_user_id_match", bot.BotUserID == inbound.BotUserID, "bot_status", bot.Status)

	var botAgentCount int64
	service.db.WithContext(ctx).Table("bot_agent_bindings").
		Where("bot_id=? AND agent_id=? AND status='active'", binding.BotID, binding.AgentID).Count(&botAgentCount)
	slog.WarnContext(ctx, "[Inbound] 路由未命中：Bot-Agent 绑定现状",
		"bot_id", binding.BotID, "agent_id", binding.AgentID, "active_bindings", botAgentCount)
}

// isTicketHumanTakenOver 检查工单是否已进入人工接管状态。
// 用于"Bot 不退群"场景的双重保险：HandleInbound 入口处直接拦截，防止 LLM 被调用。
//
// 重要：DB 查询失败时宁可误拦（返回 true）也不放任消息穿透到 LLM ——
// 一次 DB 抖动不应导致大量不必要的 LLM 调用。
func (service *Service) isTicketHumanTakenOver(ctx context.Context, appKey, ticketID string) bool {
	var ticket struct{ IsHumanTakenOver bool }
	err := service.db.WithContext(ctx).Table("tickets").
		Select("is_human_taken_over").
		Where("app_key=? AND ticket_id=?", appKey, ticketID).
		Take(&ticket).Error
	if err != nil {
		slog.WarnContext(ctx, "[Inbound] isTicketHumanTakenOver DB 查询失败，保守拦截",
			"ticket_id", ticketID, "error", err)
		return true // 保守策略：DB 异常时宁可误拦，不走 LLM
	}
	return ticket.IsHumanTakenOver
}

func (service *Service) resolveInboundAgent(ctx context.Context, inbound imbot.InboundMessage) (agentmodel.Agent, string, error) {
	var row struct {
		AgentID    string
		InviteCode string
	}
	// TIPS: 同时校验 App、当前 Inbox 绑定、Bot-Agent 绑定、未关闭 Ticket 和客户来源，
	// 防止群内坐席消息或历史 Bot 在换绑后继续触发推理。
	//
	// TIPS: 这里必须用 Take 而非 First。First 会自动追加 ORDER BY 主键，而目标是匿名结构体、
	// 没有主键，GORM 会退化成按第一个字段排序并拼上主表别名，生成 `ORDER BY t.agent_id` ——
	// 主表是 tickets，没有 agent_id 列，直接报 42703。条件已唯一定位一行，本就不需要排序。
	// 同样的坑见 services/inboxagentservice.go 的 BindInboxAgent。
	err := service.db.WithContext(ctx).Table("tickets t").
		Select("iab.agent_id,b.invite_code").
		Joins("JOIN inbox_agent_bindings iab ON iab.app_key=t.app_key AND iab.inbox_id=t.inbox_id AND iab.status='active'").
		Joins("JOIN bots b ON b.id=iab.bot_id AND b.app_key=t.app_key AND b.status='active'").
		Joins("JOIN bot_agent_bindings bab ON bab.bot_id=b.id AND bab.agent_id=iab.agent_id AND bab.status='active'").
		Where("t.app_key=? AND t.ticket_id=? AND t.source_id=? AND t.status<>? AND b.bot_user_id=?", inbound.AppKey, inbound.TargetID, inbound.SenderID, 2, inbound.BotUserID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		service.logInboundRouteMiss(ctx, inbound)
		return agentmodel.Agent{}, "", messageError(404, "404_TICKET_AGENT_BINDING_NOT_FOUND", "当前 Ticket 没有可用 Agent 绑定")
	}
	if err != nil {
		return agentmodel.Agent{}, "", err
	}
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).First(&agent, "id = ? AND app_key = ? AND status = 'active'", row.AgentID, inbound.AppKey).Error; err != nil {
		return agentmodel.Agent{}, "", err
	}
	return agent, row.InviteCode, nil
}
