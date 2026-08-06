package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestProcessWhatsAppWebhookCreatesTicketAndSendsTextToGroup(t *testing.T) {
	env := newWhatsAppWebhookTestEnv(t)
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"changes":[{"field":"messages","value":{"metadata":{"phone_number_id":"phone_1"},"contacts":[{"wa_id":"15551234567","profile":{"name":"Ada Lovelace"}}],"messages":[{"from":"15551234567","id":"wamid.1","timestamp":"1700000000","type":"text","text":{"body":"hello"}}]}}]}]}`)

	code := ProcessWhatsAppWebhook("inbox_wa", body, whatsappTestSignature(body, "app-secret"))
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created == nil || env.customers.created.Identifier != "15551234567" || env.customers.created.Nickname != "Ada Lovelace" {
		t.Fatalf("customer = %+v", env.customers.created)
	}
	if env.tickets.created == nil || env.tickets.created.ChannelType != string(ChannelType_WhatsApp) {
		t.Fatalf("ticket = %+v", env.tickets.created)
	}
	if env.sentMsg.MsgContent != `{"content":"hello"}` {
		t.Fatalf("msg content = %s", env.sentMsg.MsgContent)
	}
	if env.sentMsg.MsgId == nil || *env.sentMsg.MsgId != "whatsapp:inbox_wa:wamid.1" {
		t.Fatalf("msg id = %v", env.sentMsg.MsgId)
	}
}

func TestProcessWhatsAppWebhookRejectsInvalidSignature(t *testing.T) {
	env := newWhatsAppWebhookTestEnv(t)
	body := []byte(`{"object":"whatsapp_business_account"}`)

	code := ProcessWhatsAppWebhook("inbox_wa", body, "sha256=bad")
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created != nil || env.sentMsg.TargetId != "" {
		t.Fatalf("unexpected side effects customer=%+v msg=%+v", env.customers.created, env.sentMsg)
	}
}

func TestWhatsAppWebhookChallengeUsesInboxVerifyToken(t *testing.T) {
	newWhatsAppWebhookTestEnv(t)
	challenge, ok := WhatsAppWebhookChallenge("inbox_wa", "subscribe", "verify-token", "challenge-1")
	if !ok || challenge != "challenge-1" {
		t.Fatalf("challenge=%q ok=%v", challenge, ok)
	}
	if _, ok := WhatsAppWebhookChallenge("inbox_wa", "subscribe", "wrong", "challenge-1"); ok {
		t.Fatal("challenge accepted with wrong token")
	}
}

func TestWhatsAppMessageContentKeepsMediaCaptionAndInteractiveSelection(t *testing.T) {
	if got := whatsappMessageContent(WhatsAppMessage{
		Type:  "image",
		Image: &WhatsAppMedia{Caption: "product photo"},
	}); got != "product photo" {
		t.Fatalf("image content = %q", got)
	}
	if got := whatsappMessageContent(WhatsAppMessage{
		Type: "interactive",
		Interactive: &WhatsAppInteractive{
			ButtonReply: &struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{Title: "Choose"},
		},
	}); got != "Choose" {
		t.Fatalf("interactive content = %q", got)
	}
}

type whatsappWebhookTestEnv struct {
	inboxes   *telegramFakeInboxStorage
	customers *telegramFakeCustomerStorage
	rels      *telegramFakeRelStorage
	tickets   *telegramFakeTicketStorage
	sentMsg   juggleimsdk.Message
}

func newWhatsAppWebhookTestEnv(t *testing.T) *whatsappWebhookTestEnv {
	t.Helper()
	env := &whatsappWebhookTestEnv{
		inboxes: &telegramFakeInboxStorage{
			byId: map[string]*storageModels.Inbox{
				"inbox_wa": {
					InboxId:     "inbox_wa",
					ChannelType: string(ChannelType_WhatsApp),
					ChannelConf: `{"access_token":"token","phone_number_id":"phone_1","webhook_verify_token":"verify-token","app_secret":"app-secret"}`,
					AppKey:      "app_1",
				},
			},
			byAny: map[string][]*storageModels.Inbox{
				"inbox_wa": {{
					InboxId:     "inbox_wa",
					ChannelType: string(ChannelType_WhatsApp),
					ChannelConf: `{"access_token":"token","phone_number_id":"phone_1","webhook_verify_token":"verify-token","app_secret":"app-secret"}`,
					AppKey:      "app_1",
				}},
			},
		},
		customers: &telegramFakeCustomerStorage{byIdentifier: map[string]*storageModels.Customer{}},
		rels:      &telegramFakeRelStorage{byCustomerInbox: map[string]*storageModels.CustomerInboxRel{}},
		tickets:   &telegramFakeTicketStorage{bySource: map[string]*storageModels.Ticket{}},
	}

	oldInboxWebhook := newInboxStorageForWhatsAppWebhook
	oldInbox := newInboxStorageForCustomer
	oldCustomer := newCustomerStorageForCustomer
	oldRel := newCustomerInboxRelStorageForCustomer
	oldTicket := newTicketStorageForCustomer
	oldMember := newInboxMemberStorageForCustomer
	oldUser := newUserStorageForCustomer
	oldGetCustomerSDK := getImSdkForCustomer
	oldGetWebhookSDK := getImSdkForWhatsAppWebhook
	oldRegisterCustomer := registerCustomerIMUserForCustomer
	oldCreateGroup := createGroupForCustomer
	oldResolveBot := resolveInboxAgentBotForCustomer
	oldSyncGlobalTags := syncTicketGlobalConversationTagsForCustomer
	oldSendGroup := sendWhatsAppGroupMsg

	newInboxStorageForWhatsAppWebhook = func() storageModels.IInboxStorage { return env.inboxes }
	newInboxStorageForCustomer = func() storageModels.IInboxStorage { return env.inboxes }
	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage { return env.customers }
	newCustomerInboxRelStorageForCustomer = func() storageModels.ICustomerInboxRelStorage { return env.rels }
	newTicketStorageForCustomer = func() storageModels.ITicketStorage { return env.tickets }
	newInboxMemberStorageForCustomer = func() storageModels.IInboxMemberStorage {
		return &customerInboxMemberStorage{}
	}
	newUserStorageForCustomer = func() storageModels.IUserStorage {
		return &customerUserStorage{users: map[string]*storageModels.User{}}
	}
	getImSdkForCustomer = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	getImSdkForWhatsAppWebhook = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	registerCustomerIMUserForCustomer = func(_ *juggleimsdk.JuggleIMSdk, _, _, _ string) (errs.IMErrorCode, string) {
		return errs.IMErrorCode_SUCCESS, "im-token"
	}
	createGroupForCustomer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), req.GroupId, nil
	}
	resolveInboxAgentBotForCustomer = func(context.Context, string, string) (string, error) { return "", nil }
	syncTicketGlobalConversationTagsForCustomer = func(string, string) errs.IMErrorCode {
		return errs.IMErrorCode_SUCCESS
	}
	sendWhatsAppGroupMsg = func(_ *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		env.sentMsg = msg
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	t.Cleanup(func() {
		newInboxStorageForWhatsAppWebhook = oldInboxWebhook
		newInboxStorageForCustomer = oldInbox
		newCustomerStorageForCustomer = oldCustomer
		newCustomerInboxRelStorageForCustomer = oldRel
		newTicketStorageForCustomer = oldTicket
		newInboxMemberStorageForCustomer = oldMember
		newUserStorageForCustomer = oldUser
		getImSdkForCustomer = oldGetCustomerSDK
		getImSdkForWhatsAppWebhook = oldGetWebhookSDK
		registerCustomerIMUserForCustomer = oldRegisterCustomer
		createGroupForCustomer = oldCreateGroup
		resolveInboxAgentBotForCustomer = oldResolveBot
		syncTicketGlobalConversationTagsForCustomer = oldSyncGlobalTags
		sendWhatsAppGroupMsg = oldSendGroup
	})
	return env
}

func whatsappTestSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWhatsAppMessageContentTrimsText(t *testing.T) {
	message := WhatsAppMessage{Type: "text", Text: &struct {
		Body string `json:"body"`
	}{Body: "  hello  "}}
	if got := strings.TrimSpace(whatsappMessageContent(message)); got != "hello" {
		t.Fatalf("content = %q", got)
	}
}
