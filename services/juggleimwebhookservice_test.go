package services

import (
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestProcessJuggleIMWebhookSupportedCallbackCreatesTicketAndSendsGroupMessage(t *testing.T) {
	env := newJuggleIMWebhookTestEnv(t)

	body := []byte(`{"app_key":"ignored","sender":"visitor_1","receiver":"bot_1","conver_type":1,"msg_type":"jg:text","msg_content":"{\"content\":\"hello\"}","msg_id":"msg_1","msg_time":123}`)
	code := ProcessJuggleIMWebhook("inbox_juggle", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created == nil || env.customers.created.Identifier != "visitor_1" || env.customers.created.Nickname != "visitor_1" {
		t.Fatalf("customer = %+v", env.customers.created)
	}
	if env.rels.created == nil || !strings.HasPrefix(env.rels.created.SourceId, CustomerSourceIDPrefix) || strings.Contains(env.rels.created.SourceId, "visitor_1") {
		t.Fatalf("rel = %+v", env.rels.created)
	}
	if env.tickets.created == nil || env.tickets.created.InboxId != "inbox_juggle" || env.tickets.created.SourceId != env.rels.created.SourceId {
		t.Fatalf("ticket = %+v", env.tickets.created)
	}
	if env.tickets.created.ChannelType != string(ChannelType_JuggleIM) {
		t.Fatalf("ticket channel type = %q, want juggleim", env.tickets.created.ChannelType)
	}
	if !strings.HasPrefix(env.tickets.created.TicketId, TicketIDPrefix) || env.createdGroupId != env.tickets.created.TicketId {
		t.Fatalf("ticket id = %q group id = %q, want matching ticket-prefixed ids", env.tickets.created.TicketId, env.createdGroupId)
	}
	if env.sentMsg.SenderId != env.rels.created.SourceId || env.sentMsg.TargetId != env.tickets.created.TicketId {
		t.Fatalf("sent msg = %+v", env.sentMsg)
	}
	if env.sentMsg.MsgType != "jg:text" || env.sentMsg.MsgContent != `{"content":"hello"}` {
		t.Fatalf("sent msg = %+v", env.sentMsg)
	}
	if env.sentMsg.MsgId == nil || *env.sentMsg.MsgId != "msg_1" {
		t.Fatalf("msg id = %v", env.sentMsg.MsgId)
	}
}

func TestProcessJuggleIMWebhookIgnoresInvalidPayloadMissingSenderAndEmptyContent(t *testing.T) {
	env := newJuggleIMWebhookTestEnv(t)

	for _, body := range [][]byte{
		[]byte(`{`),
		[]byte(`{"sender":"","msg_content":"hello"}`),
		[]byte(`{"sender":"visitor_1","msg_content":""}`),
	} {
		code := ProcessJuggleIMWebhook("inbox_juggle", body)
		if code != errs.IMErrorCode_SUCCESS {
			t.Fatalf("code = %d for body %s", code, string(body))
		}
	}
	if env.customers.created != nil || env.tickets.created != nil || env.sentMsg.TargetId != "" {
		t.Fatalf("unexpected side effects customer=%+v ticket=%+v msg=%+v", env.customers.created, env.tickets.created, env.sentMsg)
	}
}

func TestProcessJuggleIMWebhookRejectsUnknownAndNonJuggleIMInbox(t *testing.T) {
	env := newJuggleIMWebhookTestEnv(t)

	body := []byte(`{"sender":"visitor_1","msg_type":"jg:text","msg_content":"hello","msg_id":"msg_1"}`)
	code := ProcessJuggleIMWebhook("missing", body)
	if code != errs.IMErrorCode_APP_CHANNEL_NOT_EXIST {
		t.Fatalf("missing inbox code = %d", code)
	}
	env.inboxes.byAny["telegram_inbox"] = []*storageModels.Inbox{{InboxId: "telegram_inbox", ChannelType: string(ChannelType_Telegram), AppKey: "app_1"}}
	code = ProcessJuggleIMWebhook("telegram_inbox", body)
	if code != errs.IMErrorCode_APP_CHANNEL_NOT_EXIST {
		t.Fatalf("telegram inbox code = %d", code)
	}
}

func TestProcessJuggleIMWebhookReusesCustomerAndTicket(t *testing.T) {
	env := newJuggleIMWebhookTestEnv(t)
	env.customers.byIdentifier["visitor_1"] = &storageModels.Customer{CustomerId: "c_existing", Nickname: "Existing", Identifier: "visitor_1", AppKey: "app_1"}
	env.rels.byCustomerInbox["c_existing|inbox_juggle"] = &storageModels.CustomerInboxRel{CustomerId: "c_existing", InboxId: "inbox_juggle", SourceId: "customer_existing", AppKey: "app_1"}
	env.tickets.bySource["customer_existing"] = &storageModels.Ticket{TicketId: "ticket_existing", SourceId: "customer_existing", InboxId: "inbox_juggle", AppKey: "app_1"}

	body := []byte(`{"sender":"visitor_1","msg_type":"jg:custom","msg_content":"{\"x\":1}","msg_id":"msg_2"}`)
	code := ProcessJuggleIMWebhook("inbox_juggle", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created != nil || env.rels.created != nil || env.tickets.created != nil || env.createdGroupId != "" {
		t.Fatalf("created duplicate customer=%+v rel=%+v ticket=%+v group=%q", env.customers.created, env.rels.created, env.tickets.created, env.createdGroupId)
	}
	if env.sentMsg.TargetId != "ticket_existing" || env.sentMsg.MsgType != "jg:custom" {
		t.Fatalf("sent msg = %+v", env.sentMsg)
	}
}

type juggleIMWebhookTestEnv struct {
	inboxes   *telegramFakeInboxStorage
	customers *telegramFakeCustomerStorage
	rels      *telegramFakeRelStorage
	tickets   *telegramFakeTicketStorage

	createdGroupId string
	sentMsg        juggleimsdk.Message
}

func newJuggleIMWebhookTestEnv(t *testing.T) *juggleIMWebhookTestEnv {
	t.Helper()
	env := &juggleIMWebhookTestEnv{
		inboxes: &telegramFakeInboxStorage{
			byId: map[string]*storageModels.Inbox{
				"inbox_juggle": {InboxId: "inbox_juggle", ChannelType: string(ChannelType_JuggleIM), ChannelConf: `{"bot_name":"bot","bot_token":"secret"}`, AppKey: "app_1"},
			},
			byAny: map[string][]*storageModels.Inbox{
				"inbox_juggle": {
					{InboxId: "inbox_juggle", ChannelType: string(ChannelType_JuggleIM), ChannelConf: `{"bot_name":"bot","bot_token":"secret"}`, AppKey: "app_1"},
				},
			},
		},
		customers: &telegramFakeCustomerStorage{byIdentifier: map[string]*storageModels.Customer{}},
		rels:      &telegramFakeRelStorage{byCustomerInbox: map[string]*storageModels.CustomerInboxRel{}},
		tickets:   &telegramFakeTicketStorage{bySource: map[string]*storageModels.Ticket{}},
	}

	oldInboxWebhook := newInboxStorageForJuggleIMWebhook
	oldInbox := newInboxStorageForCustomer
	oldCustomer := newCustomerStorageForCustomer
	oldRel := newCustomerInboxRelStorageForCustomer
	oldTicket := newTicketStorageForCustomer
	oldMember := newInboxMemberStorageForCustomer
	oldUser := newUserStorageForCustomer
	oldGetCustomerSDK := getImSdkForCustomer
	oldGetWebhookSDK := getImSdkForJuggleIMWebhook
	oldRegister := registerIMUserForCustomer
	oldRegisterCustomer := registerCustomerIMUserForCustomer
	oldCreateGroup := createGroupForCustomer
	oldSyncGlobalTags := syncTicketGlobalConversationTagsForCustomer
	oldSendGroup := sendJuggleIMGroupMsg

	newInboxStorageForJuggleIMWebhook = func() storageModels.IInboxStorage { return env.inboxes }
	newInboxStorageForCustomer = func() storageModels.IInboxStorage { return env.inboxes }
	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage { return env.customers }
	newCustomerInboxRelStorageForCustomer = func() storageModels.ICustomerInboxRelStorage { return env.rels }
	newTicketStorageForCustomer = func() storageModels.ITicketStorage { return env.tickets }
	newInboxMemberStorageForCustomer = func() storageModels.IInboxMemberStorage {
		return &customerInboxMemberStorage{members: []*storageModels.InboxMember{{MemberId: "agent_1"}}}
	}
	newUserStorageForCustomer = func() storageModels.IUserStorage {
		return &customerUserStorage{users: map[string]*storageModels.User{"agent_1": {UserId: "agent_1", Nickname: "Agent"}}}
	}
	getImSdkForCustomer = func(appkey string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	getImSdkForJuggleIMWebhook = func(appkey string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	registerIMUserForCustomer = func(_ *juggleimsdk.JuggleIMSdk, _, _, _ string) errs.IMErrorCode { return errs.IMErrorCode_SUCCESS }
	registerCustomerIMUserForCustomer = func(_ *juggleimsdk.JuggleIMSdk, _, _, _ string) (errs.IMErrorCode, string) {
		return errs.IMErrorCode_SUCCESS, "im_token"
	}
	createGroupForCustomer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		env.createdGroupId = req.GroupId
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	syncTicketGlobalConversationTagsForCustomer = func(string, string) errs.IMErrorCode {
		return errs.IMErrorCode_SUCCESS
	}
	sendJuggleIMGroupMsg = func(_ *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		env.sentMsg = msg
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	t.Cleanup(func() {
		newInboxStorageForJuggleIMWebhook = oldInboxWebhook
		newInboxStorageForCustomer = oldInbox
		newCustomerStorageForCustomer = oldCustomer
		newCustomerInboxRelStorageForCustomer = oldRel
		newTicketStorageForCustomer = oldTicket
		newInboxMemberStorageForCustomer = oldMember
		newUserStorageForCustomer = oldUser
		getImSdkForCustomer = oldGetCustomerSDK
		getImSdkForJuggleIMWebhook = oldGetWebhookSDK
		registerIMUserForCustomer = oldRegister
		registerCustomerIMUserForCustomer = oldRegisterCustomer
		createGroupForCustomer = oldCreateGroup
		syncTicketGlobalConversationTagsForCustomer = oldSyncGlobalTags
		sendJuggleIMGroupMsg = oldSendGroup
	})
	return env
}
