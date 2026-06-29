package services

import (
	"encoding/json"
	"log"
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newInboxStorageForJuggleIMWebhook = storages.NewInboxStorage
	getImSdkForJuggleIMWebhook        = getImSdkForCustomer
	sendJuggleIMGroupMsg              = func(sdk *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return sdk.SendGroupMsg(msg)
	}
)

type CustomChatMsgReq struct {
	AppKey      string       `json:"app_key"`
	Sender      string       `json:"sender"`
	Receiver    string       `json:"receiver"`
	ConverType  int          `json:"conver_type"`
	MsgType     string       `json:"msg_type"`
	MsgContent  string       `json:"msg_content"`
	MsgId       string       `json:"msg_id"`
	MsgTime     int64        `json:"msg_time"`
	MentionInfo *MentionInfo `json:"mention_info,omitempty"`
}

type MentionInfo struct {
	MentionedType int      `json:"mentioned_type,omitempty"`
	UserIds       []string `json:"user_ids,omitempty"`
}

func ProcessJuggleIMWebhook(inboxId string, body []byte) errs.IMErrorCode {
	inboxId = strings.TrimSpace(inboxId)
	if inboxId == "" {
		return errs.IMErrorCode_APP_ParamError
	}
	inbox, code := resolveJuggleIMWebhookInbox(inboxId)
	if code != errs.IMErrorCode_SUCCESS {
		return code
	}
	conf := ParseJuggleIMChannelConf(inbox.ChannelConf)
	if strings.TrimSpace(conf.BotToken) == "" {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}

	var req CustomChatMsgReq
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[JuggleIMWebhook] invalid payload inbox=%s err=%v", inboxId, err)
		return errs.IMErrorCode_SUCCESS
	}
	identity := strings.TrimSpace(req.Sender)
	if identity == "" {
		log.Printf("[JuggleIMWebhook] missing sender inbox=%s msg_id=%s", inboxId, req.MsgId)
		return errs.IMErrorCode_SUCCESS
	}
	if strings.TrimSpace(req.MsgContent) == "" {
		log.Printf("[JuggleIMWebhook] empty content inbox=%s sender=%s msg_id=%s", inboxId, identity, req.MsgId)
		return errs.IMErrorCode_SUCCESS
	}
	msgType := strings.TrimSpace(req.MsgType)
	if msgType == "" {
		msgType = "jg:text"
	}

	resultCode, result := startCustomerTicket(customerTicketStartReq{
		AppKey:      inbox.AppKey,
		InboxId:     inbox.InboxId,
		Identifier:  identity,
		Nickname:    identity,
		ChannelType: ChannelType_JuggleIM,
		GenerateSourceId: func() string {
			return CustomerSourceIDPrefix + tools.GenerateUUIDShort22()
		},
	})
	if resultCode != errs.IMErrorCode_SUCCESS {
		return resultCode
	}

	sdk := getImSdkForJuggleIMWebhook(inbox.AppKey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED
	}
	msgId := strings.TrimSpace(req.MsgId)
	msg := juggleimsdk.Message{
		SenderId:   result.Rel.SourceId,
		TargetId:   result.Ticket.TicketId,
		MsgType:    msgType,
		MsgContent: req.MsgContent,
		IsStorage:  boolPtrForJuggleIM(true),
		IsCount:    boolPtrForJuggleIM(true),
	}
	if msgId != "" {
		msg.MsgId = &msgId
	}
	sendCode, _, err := sendJuggleIMGroupMsg(sdk, msg)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if sendCode != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(sendCode)
	}
	return errs.IMErrorCode_SUCCESS
}

func resolveJuggleIMWebhookInbox(inboxId string) (*storageModels.Inbox, errs.IMErrorCode) {
	inboxes, err := newInboxStorageForJuggleIMWebhook().FindByInboxIdAny(inboxId)
	if err != nil {
		return nil, errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if len(inboxes) != 1 || inboxes[0] == nil {
		log.Printf("[JuggleIMWebhook] inbox lookup failed inbox=%s matches=%d", inboxId, len(inboxes))
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	inbox := inboxes[0]
	if inbox.ChannelType != string(ChannelType_JuggleIM) {
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return inbox, errs.IMErrorCode_SUCCESS
}

func boolPtrForJuggleIM(v bool) *bool {
	return &v
}
