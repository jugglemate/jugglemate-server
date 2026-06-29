package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	newInboxStorageForTelegramWebhook = storages.NewInboxStorage
	getImSdkForTelegramWebhook        = getImSdkForCustomer
	sendTelegramGroupMsg              = func(sdk *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return sdk.SendGroupMsg(msg)
	}
)

type TelegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	MessageID int64         `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      *TelegramChat `json:"chat"`
	Text      string        `json:"text"`
	Caption   string        `json:"caption"`
}

type TelegramUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

type TelegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func ProcessTelegramWebhook(inboxId string, body []byte) errs.IMErrorCode {
	inboxId = strings.TrimSpace(inboxId)
	if inboxId == "" {
		return errs.IMErrorCode_APP_ParamError
	}
	inbox, code := resolveTelegramWebhookInbox(inboxId)
	if code != errs.IMErrorCode_SUCCESS {
		return code
	}
	conf := ParseTelegramChannelConf(inbox.ChannelConf)
	if strings.TrimSpace(conf.BotToken) == "" {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		log.Printf("[TelegramWebhook] invalid payload inbox=%s err=%v", inboxId, err)
		return errs.IMErrorCode_SUCCESS
	}
	msg := update.Message
	if msg == nil || msg.From == nil || msg.Chat == nil {
		log.Printf("[TelegramWebhook] unsupported update inbox=%s update_id=%d", inboxId, update.UpdateID)
		return errs.IMErrorCode_SUCCESS
	}
	if msg.Chat.Type != "private" {
		log.Printf("[TelegramWebhook] ignore non-private chat inbox=%s chat_type=%s update_id=%d", inboxId, msg.Chat.Type, update.UpdateID)
		return errs.IMErrorCode_SUCCESS
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = strings.TrimSpace(msg.Caption)
	}
	if text == "" {
		log.Printf("[TelegramWebhook] unsupported message body inbox=%s message_id=%d", inboxId, msg.MessageID)
		return errs.IMErrorCode_SUCCESS
	}

	identity := fmt.Sprint(msg.From.ID)
	nickname := telegramDisplayName(msg.From)
	resultCode, result := startCustomerTicket(customerTicketStartReq{
		AppKey:      inbox.AppKey,
		InboxId:     inbox.InboxId,
		Identifier:  identity,
		Nickname:    nickname,
		ChannelType: ChannelType_Telegram,
		GenerateSourceId: func() string {
			return CustomerSourceIDPrefix + tools.GenerateUUIDShort22()
		},
	})
	if resultCode != errs.IMErrorCode_SUCCESS {
		return resultCode
	}

	sdk := getImSdkForTelegramWebhook(inbox.AppKey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED
	}
	imMsgId := telegramIMMessageID(inbox.InboxId, msg.MessageID)
	sendCode, _, err := sendTelegramGroupMsg(sdk, juggleimsdk.Message{
		SenderId:   result.Rel.SourceId,
		TargetId:   result.Ticket.TicketId,
		MsgType:    "jg:text",
		MsgContent: fmt.Sprintf(`{"content":%q}`, text),
		MsgId:      &imMsgId,
		IsStorage:  boolPtrForTelegram(true),
		IsCount:    boolPtrForTelegram(true),
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if sendCode != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(sendCode)
	}
	return errs.IMErrorCode_SUCCESS
}

func resolveTelegramWebhookInbox(inboxId string) (*storageModels.Inbox, errs.IMErrorCode) {
	inboxes, err := newInboxStorageForTelegramWebhook().FindByInboxIdAny(inboxId)
	if err != nil {
		return nil, errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if len(inboxes) != 1 || inboxes[0] == nil {
		log.Printf("[TelegramWebhook] inbox lookup failed inbox=%s matches=%d", inboxId, len(inboxes))
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	inbox := inboxes[0]
	if inbox.ChannelType != string(ChannelType_Telegram) {
		return nil, errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}
	return inbox, errs.IMErrorCode_SUCCESS
}

func telegramDisplayName(user *TelegramUser) string {
	if user == nil {
		return ""
	}
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
	if name != "" {
		return name
	}
	if strings.TrimSpace(user.Username) != "" {
		return strings.TrimSpace(user.Username)
	}
	return fmt.Sprint(user.ID)
}

func telegramIMMessageID(inboxId string, messageId int64) string {
	if messageId <= 0 {
		return ""
	}
	return fmt.Sprintf("telegram:%s:%d", inboxId, messageId)
}

func boolPtrForTelegram(v bool) *bool {
	return &v
}
