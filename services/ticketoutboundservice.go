package services

import (
	"encoding/json"
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
	sendTelegramOutboundMessage   = func(botToken, target, text string) error {
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
