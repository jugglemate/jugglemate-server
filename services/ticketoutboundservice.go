package services

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/telegram"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newTicketStorageForOutbound   = storages.NewTicketStorage
	newInboxStorageForOutbound    = storages.NewInboxStorage
	newCustomerStorageForOutbound = storages.NewCustomerStorage
	newMessageStorageForOutbound = storages.NewTicketMessageStorage
	// updateLastUserMsgAtFn 是 last_user_msg_at 推进钩子，单测可替换。
	updateLastUserMsgAtFn = func(appkey, ticketId string, atMs int64) error {
		return newTicketStorageForOutbound().UpdateLastUserMsgAt(appkey, ticketId, atMs)
	}
	sendTelegramOutboundMessage = func(botToken, target, text string) error {
		return telegram.NewClient().SendMessage(botToken, target, text)
	}
)

type WebhookMessagePayload struct {
	Platform    string
	Sender      string
	Receiver    string
	ConverType  int
	MsgType     string
	MsgContent  string
	MentionInfo json.RawMessage
	MsgID       string
	MsgTime     int64
}

func ProcessWebhookMessage(appkey string, payload WebhookMessagePayload) errs.IMErrorCode {
	appkey = strings.TrimSpace(appkey)
	payload.Sender = strings.TrimSpace(payload.Sender)
	payload.Receiver = strings.TrimSpace(payload.Receiver)
	payload.MsgType = strings.TrimSpace(payload.MsgType)

	if payload.ConverType != ConversationType_Ticket {
		log.Printf("[WebhookMsgs] skip: unsupported conver_type=%d sender=%s receiver=%s msg_type=%s msg_id=%s",
			payload.ConverType, payload.Sender, payload.Receiver, payload.MsgType, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}
	if !strings.HasPrefix(payload.Receiver, TicketIDPrefix) {
		log.Printf("[WebhookMsgs] skip: receiver is not ticket group sender=%s receiver=%s msg_type=%s msg_id=%s",
			payload.Sender, payload.Receiver, payload.MsgType, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}
	if appkey == "" {
		log.Printf("[WebhookMsgs] skip: empty appkey sender=%s receiver=%s msg_type=%s msg_id=%s",
			payload.Sender, payload.Receiver, payload.MsgType, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	// 路由 1：jgm:csatreply 是客户在群内回复的评分自定义消息。
	// 走独立路径（不写入 ticket_messages "客户消息"流，不走 telegram 出站），
	// 由 RecordFromCustomMessage 落 ticket_ratings 并在 ticket_messages 留痕。
	if payload.MsgType == "jgm:csatreply" {
		return processCsatReplyCustomMessage(context.Background(), appkey, payload)
	}

	// 自动关闭 ticker 依赖 tickets.last_user_msg_at 推进；这里在 webhook 入站时把
	// ticket_messages 写库（UpsertByMsgId 幂等）+ 仅对 sender_role=user（坐席）推进
	// last_user_msg_at。客户消息不更新该字段，因为那是"坐席活跃"维度。
	// TIPS: 客户 / Agent Bot 不在 webhook 推送范围（Bot 用长连接），所以这条主要覆盖
	// Telegram 等 telegram 出站场景下"坐席在群内发文本"与"jgm:ticketassign"等业务消息。
	if payload.MsgTime > 0 {
		recordOutboundTicketMessage(appkey, payload)
	}

	ticket, err := newTicketStorageForOutbound().FindByTicketId(appkey, payload.Receiver)
	if err != nil {
		log.Printf("[WebhookMsgs] ticket lookup failed appkey=%s ticket_id=%s err=%v", appkey, payload.Receiver, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if ticket == nil {
		log.Printf("[WebhookMsgs] skip: ticket not found appkey=%s ticket_id=%s sender=%s msg_id=%s", appkey, payload.Receiver, payload.Sender, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}
	if strings.TrimSpace(ticket.SourceId) == payload.Sender {
		log.Printf("[WebhookMsgs] skip: sender is ticket source appkey=%s ticket_id=%s source_id=%s msg_id=%s", appkey, ticket.TicketId, ticket.SourceId, payload.MsgID)
		// TIPS: 这条分支就是"客户说话了"。客户消息由 IM 直连投递给群内 Bot，不走 webhook 出站，
		// 所以 Bot 不回复时这里是唯一能同时拿到 ticket 和时间点的位置，顺手把链路状态打出来。
		LogTicketAgentBotDiagnostics(context.Background(), appkey, ticket.InboxId, ticket.TicketId, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	ticketAppKey := strings.TrimSpace(ticket.AppKey)
	if ticketAppKey == "" {
		ticketAppKey = appkey
	}
	inbox, err := newInboxStorageForOutbound().FindByInboxId(ticketAppKey, ticket.InboxId)
	if err != nil {
		log.Printf("[WebhookMsgs] inbox lookup failed appkey=%s inbox_id=%s ticket_id=%s err=%v", ticketAppKey, ticket.InboxId, ticket.TicketId, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if inbox == nil {
		log.Printf("[WebhookMsgs] skip: inbox not found appkey=%s inbox_id=%s ticket_id=%s msg_id=%s", ticketAppKey, ticket.InboxId, ticket.TicketId, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	switch inbox.ChannelType {
	case string(ChannelType_Telegram):
		return forwardTicketGroupMessageToTelegram(ticketAppKey, ticket, inbox, payload)
	case string(ChannelType_Widget), string(ChannelType_JuggleIM):
		// TIPS: 客户本身就是 Ticket 群成员，IM 会直接投递群消息，无需二次出站转发。
		// 这两类渠道走到这里属于正常路径，不打日志——否则 Agent 和坐席的每一条回复
		// 都会刷一条 "unsupported channel_type"，既是噪音，也会掩盖真正未实现的渠道。
		return errs.IMErrorCode_SUCCESS
	default:
		log.Printf("[WebhookMsgs] skip: unsupported channel_type=%s appkey=%s inbox_id=%s ticket_id=%s msg_id=%s",
			inbox.ChannelType, ticketAppKey, inbox.InboxId, ticket.TicketId, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}
}

// processCsatReplyCustomMessage 路由 jgm:csatreply 入站消息到评分记录路径。
//
// 错误处理：除永久失败（工单不存在/字段不一致/超长）外，所有"瞬时失败"都返回
// SUCCESS 让 webhook 不重投（IM server 反复重投会让客户重复扣分；UNIQUE 防重是
// 兜底）。永久失败返回参数错误，前端视角等同于"消息被忽略"。
func processCsatReplyCustomMessage(ctx context.Context, appkey string, payload WebhookMessagePayload) errs.IMErrorCode {
	_, err := RecordFromCustomMessage(
		ctx,
		appkey,
		payload.Receiver,
		payload.MsgContent,
		payload.Sender,
		payload.MsgID,
		payload.MsgType,
		payload.MsgTime,
	)
	if err == nil {
		return errs.IMErrorCode_SUCCESS
	}
	switch {
	case errors.Is(err, ErrCsatAlreadyRated):
		// 已评过：静默
		return errs.IMErrorCode_SUCCESS
	case errors.Is(err, ErrCsatTicketNotFound),
		errors.Is(err, ErrCsatAppKeyMismatch),
		errors.Is(err, ErrCsatTicketIdMismatch),
		errors.Is(err, ErrCsatSenderNotCustomer),
		errors.Is(err, ErrCsatRatingOutOfRange),
		errors.Is(err, ErrCsatCommentTooLong):
		log.Printf("[CsatReply] 参数错误忽略 msg_id=%s err=%v", payload.MsgID, err)
		return errs.IMErrorCode_APP_ParamError
	default:
		log.Printf("[CsatReply] 内部错误 msg_id=%s err=%v", payload.MsgID, err)
		return errs.IMErrorCode_SUCCESS // 不让 IM 重投；防重靠 UNIQUE
	}
}

func forwardTicketGroupMessageToTelegram(appkey string, ticket *storageModels.Ticket, inbox *storageModels.Inbox, payload WebhookMessagePayload) errs.IMErrorCode {
	conf := ParseTelegramChannelConf(inbox.ChannelConf)
	if strings.TrimSpace(conf.BotToken) == "" {
		log.Printf("[WebhookMsgs] skip: telegram bot token missing appkey=%s inbox_id=%s ticket_id=%s msg_id=%s", appkey, inbox.InboxId, ticket.TicketId, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	customer, err := newCustomerStorageForOutbound().FindByCustomerId(appkey, ticket.CustomerId)
	if err != nil {
		log.Printf("[WebhookMsgs] customer lookup failed appkey=%s customer_id=%s ticket_id=%s err=%v", appkey, ticket.CustomerId, ticket.TicketId, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if customer == nil || strings.TrimSpace(customer.Identifier) == "" {
		log.Printf("[WebhookMsgs] skip: telegram target missing appkey=%s customer_id=%s ticket_id=%s msg_id=%s", appkey, ticket.CustomerId, ticket.TicketId, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	text := extractOutboundText(payload.MsgType, payload.MsgContent)
	if text == "" {
		log.Printf("[WebhookMsgs] skip: empty or unsupported outbound content appkey=%s ticket_id=%s msg_type=%s msg_id=%s", appkey, ticket.TicketId, payload.MsgType, payload.MsgID)
		return errs.IMErrorCode_SUCCESS
	}

	if err := sendTelegramOutboundMessage(strings.TrimSpace(conf.BotToken), strings.TrimSpace(customer.Identifier), text); err != nil {
		log.Printf("[WebhookMsgs] telegram send failed appkey=%s inbox_id=%s ticket_id=%s target=%s msg_id=%s err=%v",
			appkey, inbox.InboxId, ticket.TicketId, customer.Identifier, payload.MsgID, err)
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	log.Printf("[WebhookMsgs] telegram forwarded appkey=%s inbox_id=%s ticket_id=%s target=%s msg_id=%s",
		appkey, inbox.InboxId, ticket.TicketId, customer.Identifier, payload.MsgID)
	return errs.IMErrorCode_SUCCESS
}

// recordOutboundTicketMessage 把 webhook 收到的群消息写入 ticket_messages 留痕；
// 若 sender 是该工单的 assignee 则同步推进 last_user_msg_at。
//
// 字段语义：
//   - ticket_messages：所有 webhook 入站消息均留痕（UpsertByMsgId 幂等）；
//     sender_role 通过查 ticket 表判定（customer / user / bot）。
//   - last_user_msg_at：仅"该工单的 assignee 向本工单群发送"时推进，customer / bot 自身不推进；
//     用 GREATEST 防回退（DB 层）。
//
// TIPS: 该函数整体 best-effort —— 失败仅 slog，不阻断主流程；webhook 重投通过
// UpsertByMsgId 幂等，已写过的行不会重复增长。
//
// ticket 找不到时直接 return——没有 ticket 的工单不该在 ticket_messages 留 orphan 行，
// 否则后续 audit 查询会显示"幽灵消息"。这是与 webhook 主流程（"未关闭工单 → 静默丢"）
// 保持一致的策略。
func recordOutboundTicketMessage(appkey string, payload WebhookMessagePayload) {
	if payload.MsgID == "" || payload.MsgTime == 0 {
		return
	}
	ticket, err := newTicketStorageForOutbound().FindByTicketId(appkey, payload.Receiver)
	if err != nil || ticket == nil {
		// 工单不存在；不写 ticket_messages，避免"幽灵消息"。
		return
	}

	role := storageModels.TicketEventOperatorUser
	switch {
	case ticket.SourceId == payload.Sender:
		role = storageModels.TicketEventOperatorCustomer
	case ticket.AssigneeId == payload.Sender:
		role = storageModels.TicketEventOperatorUser
	default:
		// 兜底：bot 或未知身份都按 bot 处理，避免污染"坐席活跃"。
		role = storageModels.TicketEventOperatorBot
	}

	if err := newMessageStorageForOutbound().UpsertByMsgId(storageModels.TicketMessage{
		AppKey:      appkey,
		TicketId:    payload.Receiver,
		SenderId:    payload.Sender,
		SenderRole:  role,
		MsgId:       payload.MsgID,
		MsgType:     payload.MsgType,
		CreatedTime: payload.MsgTime,
	}); err != nil {
		log.Printf("[WebhookMsgRecorder] ticket_messages upsert 失败 msg_id=%s err=%v（继续）", payload.MsgID, err)
	}

	// 仅该工单 assignee 发消息时推进 last_user_msg_at
	if ticket.AssigneeId != "" && ticket.AssigneeId == payload.Sender {
		if err := updateLastUserMsgAtFn(appkey, payload.Receiver, payload.MsgTime); err != nil {
			log.Printf("[WebhookMsgRecorder] updateLastUserMsgAt 失败 msg_id=%s err=%v（继续）", payload.MsgID, err)
		}
	}
}

func extractOutboundText(msgType, msgContent string) string {
	msgType = strings.TrimSpace(msgType)
	if msgType != "" && msgType != "text" && msgType != "jg:text" {
		return ""
	}
	msgContent = strings.TrimSpace(msgContent)
	if msgContent == "" {
		return ""
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(msgContent), &raw); err == nil {
		contentValue, ok := raw["content"]
		if !ok {
			return ""
		}
		var text string
		if err := json.Unmarshal(contentValue, &text); err != nil {
			return ""
		}
		return strings.TrimSpace(text)
	}
	return msgContent
}
