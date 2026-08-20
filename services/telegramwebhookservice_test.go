package services

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestProcessTelegramWebhookSupportedTextCreatesTicketAndSendsGroupMessage(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)

	body := []byte(`{"update_id":1,"message":{"message_id":77,"from":{"id":123456,"first_name":"Ada","last_name":"Lovelace","username":"ada"},"chat":{"id":123456,"type":"private"},"text":"hello"}}`)
	code := ProcessTelegramWebhook("inbox_tg", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created == nil || env.customers.created.Identifier != "123456" || env.customers.created.Nickname != "Ada Lovelace" {
		t.Fatalf("customer = %+v", env.customers.created)
	}
	if env.rels.created == nil || !strings.HasPrefix(env.rels.created.SourceId, CustomerSourceIDPrefix) {
		t.Fatalf("rel = %+v", env.rels.created)
	}
	if env.tickets.created == nil || env.tickets.created.InboxId != "inbox_tg" || env.tickets.created.SourceId != env.rels.created.SourceId {
		t.Fatalf("ticket = %+v", env.tickets.created)
	}
	if env.tickets.created.ChannelType != string(ChannelType_Telegram) {
		t.Fatalf("ticket channel type = %q, want telegram", env.tickets.created.ChannelType)
	}
	if env.globalTagAppKey != "app_1" || env.globalTagTicketId != env.tickets.created.TicketId {
		t.Fatalf("global tag sync app=%q ticket=%q", env.globalTagAppKey, env.globalTagTicketId)
	}
	if !strings.HasPrefix(env.tickets.created.TicketId, TicketIDPrefix) || env.createdGroupId != env.tickets.created.TicketId {
		t.Fatalf("ticket id = %q group id = %q, want matching ticket-prefixed ids", env.tickets.created.TicketId, env.createdGroupId)
	}
	// TIPS: 建群成员只有客户本人 —— 坐席（桩数据里的 agent_1）改为转人工时才入群，
	// 该 Inbox 也没有绑定 Agent Bot。
	if !reflect.DeepEqual(env.createdGroupMembers, []string{env.rels.created.SourceId}) {
		t.Fatalf("group members = %+v，期望仅有客户", env.createdGroupMembers)
	}
	if env.sentMsg.SenderId != env.rels.created.SourceId || env.sentMsg.TargetId != env.tickets.created.TicketId {
		t.Fatalf("sent msg = %+v", env.sentMsg)
	}
	if env.sentMsg.MsgContent != `{"content":"hello"}` {
		t.Fatalf("msg content = %s", env.sentMsg.MsgContent)
	}
	if env.sentMsg.MsgId == nil || *env.sentMsg.MsgId != "telegram:inbox_tg:77" {
		t.Fatalf("msg id = %v", env.sentMsg.MsgId)
	}
}

func TestProcessTelegramWebhookCaptionUsesCaptionText(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)

	body := []byte(`{"message":{"message_id":88,"from":{"id":123456,"username":"ada"},"chat":{"id":123456,"type":"private"},"caption":"photo caption"}}`)
	code := ProcessTelegramWebhook("inbox_tg", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.sentMsg.MsgContent != `{"content":"photo caption"}` {
		t.Fatalf("msg content = %s", env.sentMsg.MsgContent)
	}
}

func TestProcessTelegramWebhookIgnoresNonPrivateChat(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)

	body := []byte(`{"message":{"message_id":77,"from":{"id":123456},"chat":{"id":-1,"type":"group"},"text":"hello"}}`)
	code := ProcessTelegramWebhook("inbox_tg", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created != nil || env.tickets.created != nil || env.sentMsg.TargetId != "" {
		t.Fatalf("unexpected side effects customer=%+v ticket=%+v msg=%+v", env.customers.created, env.tickets.created, env.sentMsg)
	}
}

func TestProcessTelegramWebhookIgnoresUnsupportedPayload(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)

	body := []byte(`{"callback_query":{"id":"cb_1","data":"x"}}`)
	code := ProcessTelegramWebhook("inbox_tg", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created != nil || env.tickets.created != nil || env.sentMsg.TargetId != "" {
		t.Fatalf("unexpected side effects customer=%+v ticket=%+v msg=%+v", env.customers.created, env.tickets.created, env.sentMsg)
	}
}

func TestProcessTelegramWebhookRejectsUnknownAndNonTelegramInbox(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)

	body := []byte(`{"message":{"message_id":77,"from":{"id":123456},"chat":{"id":123456,"type":"private"},"text":"hello"}}`)
	code := ProcessTelegramWebhook("missing", body)
	if code != errs.IMErrorCode_APP_CHANNEL_NOT_EXIST {
		t.Fatalf("missing inbox code = %d", code)
	}
	env.inboxes.byAny["widget_inbox"] = []*storageModels.Inbox{{InboxId: "widget_inbox", ChannelType: string(ChannelType_Widget), AppKey: "app_1"}}
	code = ProcessTelegramWebhook("widget_inbox", body)
	if code != errs.IMErrorCode_APP_CHANNEL_NOT_EXIST {
		t.Fatalf("widget inbox code = %d", code)
	}
}

func TestProcessTelegramWebhookReusesCustomerAndTicket(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)
	env.customers.byIdentifier["123456"] = &storageModels.Customer{CustomerId: "c_existing", Nickname: "Existing", Identifier: "123456", AppKey: "app_1"}
	env.rels.byCustomerInbox["c_existing|inbox_tg"] = &storageModels.CustomerInboxRel{CustomerId: "c_existing", InboxId: "inbox_tg", SourceId: "customer_existing", AppKey: "app_1"}
	env.tickets.bySource["customer_existing"] = &storageModels.Ticket{TicketId: "ticket_existing", SourceId: "customer_existing", InboxId: "inbox_tg", Status: storageModels.TicketStatusClosed, AppKey: "app_1"}

	body := []byte(`{"message":{"message_id":77,"from":{"id":123456,"username":"ada"},"chat":{"id":123456,"type":"private"},"text":"hello"}}`)
	code := ProcessTelegramWebhook("inbox_tg", body)
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.customers.created != nil || env.rels.created != nil || env.tickets.created != nil || env.createdGroupId != "" {
		t.Fatalf("created duplicate customer=%+v rel=%+v ticket=%+v group=%q", env.customers.created, env.rels.created, env.tickets.created, env.createdGroupId)
	}
	if env.sentMsg.TargetId != "ticket_existing" {
		t.Fatalf("target = %q, want ticket_existing", env.sentMsg.TargetId)
	}
	if env.tickets.updatedTicketId != "ticket_existing" || env.tickets.updatedStatus != storageModels.TicketStatusReOpen || env.tickets.updateStatusCalls != 1 {
		t.Fatalf("status update ticket=%q status=%d calls=%d", env.tickets.updatedTicketId, env.tickets.updatedStatus, env.tickets.updateStatusCalls)
	}
	if env.tickets.bySource["customer_existing"].Status != storageModels.TicketStatusReOpen {
		t.Fatalf("ticket status = %d, want reopened", env.tickets.bySource["customer_existing"].Status)
	}
	if env.globalTagAppKey != "app_1" || env.globalTagTicketId != "ticket_existing" {
		t.Fatalf("global tag sync app=%q ticket=%q", env.globalTagAppKey, env.globalTagTicketId)
	}
}

func TestProcessTelegramWebhookDoesNotUpdateActiveTicketStatus(t *testing.T) {
	env := newTelegramWebhookTestEnv(t)
	env.customers.byIdentifier["123456"] = &storageModels.Customer{CustomerId: "c_existing", Nickname: "Existing", Identifier: "123456", AppKey: "app_1"}
	env.rels.byCustomerInbox["c_existing|inbox_tg"] = &storageModels.CustomerInboxRel{CustomerId: "c_existing", InboxId: "inbox_tg", SourceId: "customer_existing", AppKey: "app_1"}
	env.tickets.bySource["customer_existing"] = &storageModels.Ticket{TicketId: "ticket_existing", SourceId: "customer_existing", InboxId: "inbox_tg", Status: storageModels.TicketStatusProcessing, AppKey: "app_1"}

	body := []byte(`{"message":{"message_id":77,"from":{"id":123456,"username":"ada"},"chat":{"id":123456,"type":"private"},"text":"hello"}}`)
	if code := ProcessTelegramWebhook("inbox_tg", body); code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.tickets.updateStatusCalls != 0 {
		t.Fatalf("status update calls = %d, want 0", env.tickets.updateStatusCalls)
	}
	if env.globalTagTicketId != "" {
		t.Fatalf("unexpected global tag sync for ticket %q", env.globalTagTicketId)
	}
}

type telegramWebhookTestEnv struct {
	inboxes   *telegramFakeInboxStorage
	customers *telegramFakeCustomerStorage
	rels      *telegramFakeRelStorage
	tickets   *telegramFakeTicketStorage

	createdGroupId      string
	createdGroupMembers []string
	sentMsg             juggleimsdk.Message
	globalTagAppKey     string
	globalTagTicketId   string
}

func newTelegramWebhookTestEnv(t *testing.T) *telegramWebhookTestEnv {
	t.Helper()
	env := &telegramWebhookTestEnv{
		inboxes: &telegramFakeInboxStorage{
			byId: map[string]*storageModels.Inbox{
				"inbox_tg": {InboxId: "inbox_tg", ChannelType: string(ChannelType_Telegram), ChannelConf: `{"bot_name":"bot","bot_token":"secret"}`, AppKey: "app_1"},
			},
			byAny: map[string][]*storageModels.Inbox{
				"inbox_tg": {
					{InboxId: "inbox_tg", ChannelType: string(ChannelType_Telegram), ChannelConf: `{"bot_name":"bot","bot_token":"secret"}`, AppKey: "app_1"},
				},
			},
		},
		customers: &telegramFakeCustomerStorage{byIdentifier: map[string]*storageModels.Customer{}},
		rels:      &telegramFakeRelStorage{byCustomerInbox: map[string]*storageModels.CustomerInboxRel{}},
		tickets:   &telegramFakeTicketStorage{bySource: map[string]*storageModels.Ticket{}},
	}

	oldInboxWebhook := newInboxStorageForTelegramWebhook
	oldInbox := newInboxStorageForCustomer
	oldCustomer := newCustomerStorageForCustomer
	oldRel := newCustomerInboxRelStorageForCustomer
	oldTicket := newTicketStorageForCustomer
	oldMember := newInboxMemberStorageForCustomer
	oldUser := newUserStorageForCustomer
	oldGetCustomerSDK := getImSdkForCustomer
	oldGetTelegramSDK := getImSdkForTelegramWebhook
	oldRegister := registerIMUserForCustomer
	oldRegisterCustomer := registerCustomerIMUserForCustomer
	oldCreateGroup := createGroupForCustomer
	oldResolveBot := resolveInboxAgentBotForCustomer
	oldSyncGlobalTags := syncTicketGlobalConversationTagsForCustomer
	oldSendGroup := sendTelegramGroupMsg

	newInboxStorageForTelegramWebhook = func() storageModels.IInboxStorage { return env.inboxes }
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
	getImSdkForTelegramWebhook = func(appkey string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	registerIMUserForCustomer = func(_ *juggleimsdk.JuggleIMSdk, _, _, _ string) errs.IMErrorCode { return errs.IMErrorCode_SUCCESS }
	registerCustomerIMUserForCustomer = func(_ *juggleimsdk.JuggleIMSdk, _, _, _ string) (errs.IMErrorCode, string) {
		return errs.IMErrorCode_SUCCESS, "im_token"
	}
	createGroupForCustomer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		env.createdGroupId = req.GroupId
		env.createdGroupMembers = append([]string{}, req.MemberIds...)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	resolveInboxAgentBotForCustomer = func(context.Context, string, string) (string, error) { return "", nil }
	syncTicketGlobalConversationTagsForCustomer = func(appkey, ticketId string) errs.IMErrorCode {
		env.globalTagAppKey = appkey
		env.globalTagTicketId = ticketId
		return errs.IMErrorCode_SUCCESS
	}
	sendTelegramGroupMsg = func(_ *juggleimsdk.JuggleIMSdk, msg juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		env.sentMsg = msg
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	t.Cleanup(func() {
		newInboxStorageForTelegramWebhook = oldInboxWebhook
		newInboxStorageForCustomer = oldInbox
		newCustomerStorageForCustomer = oldCustomer
		newCustomerInboxRelStorageForCustomer = oldRel
		newTicketStorageForCustomer = oldTicket
		newInboxMemberStorageForCustomer = oldMember
		newUserStorageForCustomer = oldUser
		getImSdkForCustomer = oldGetCustomerSDK
		getImSdkForTelegramWebhook = oldGetTelegramSDK
		registerIMUserForCustomer = oldRegister
		registerCustomerIMUserForCustomer = oldRegisterCustomer
		createGroupForCustomer = oldCreateGroup
		resolveInboxAgentBotForCustomer = oldResolveBot
		syncTicketGlobalConversationTagsForCustomer = oldSyncGlobalTags
		sendTelegramGroupMsg = oldSendGroup
	})
	return env
}

type telegramFakeInboxStorage struct {
	byId  map[string]*storageModels.Inbox
	byAny map[string][]*storageModels.Inbox
}

func (s *telegramFakeInboxStorage) Create(item storageModels.Inbox) error { return nil }
func (s *telegramFakeInboxStorage) Update(item storageModels.Inbox) error { return nil }
func (s *telegramFakeInboxStorage) Upsert(item storageModels.Inbox) error { return nil }
func (s *telegramFakeInboxStorage) Delete(appkey, inboxId string) error   { return nil }
func (s *telegramFakeInboxStorage) FindByInboxId(appkey, inboxId string) (*storageModels.Inbox, error) {
	return s.byId[inboxId], nil
}
func (s *telegramFakeInboxStorage) FindByInboxIdAny(inboxId string) ([]*storageModels.Inbox, error) {
	return s.byAny[inboxId], nil
}
func (s *telegramFakeInboxStorage) QryByApp(appkey, channelType string, limit, offset int64) (*storageModels.InboxListResult, error) {
	return &storageModels.InboxListResult{}, nil
}
func (s *telegramFakeInboxStorage) QryByChannelType(appkey, channelType string, startId, limit int64) ([]*storageModels.Inbox, error) {
	return nil, nil
}

type telegramFakeCustomerStorage struct {
	byIdentifier map[string]*storageModels.Customer
	byId         map[string]*storageModels.Customer
	created      *storageModels.Customer
}

func (s *telegramFakeCustomerStorage) Create(item storageModels.Customer) error {
	s.created = &item
	if s.byId != nil {
		s.byId[item.CustomerId] = &item
	}
	s.byIdentifier[item.Identifier] = &item
	return nil
}
func (s *telegramFakeCustomerStorage) Update(item storageModels.Customer) error { return nil }
func (s *telegramFakeCustomerStorage) Upsert(item storageModels.Customer) error { return nil }
func (s *telegramFakeCustomerStorage) Delete(appkey, customerId string) error   { return nil }
func (s *telegramFakeCustomerStorage) FindByCustomerId(appkey, customerId string) (*storageModels.Customer, error) {
	if s.byId == nil {
		return nil, nil
	}
	return s.byId[customerId], nil
}
func (s *telegramFakeCustomerStorage) FindByIdentifier(appkey, identifier string) (*storageModels.Customer, error) {
	return s.byIdentifier[identifier], nil
}

type telegramFakeRelStorage struct {
	byCustomerInbox map[string]*storageModels.CustomerInboxRel
	created         *storageModels.CustomerInboxRel
}

func (s *telegramFakeRelStorage) Create(item storageModels.CustomerInboxRel) error {
	s.created = &item
	s.byCustomerInbox[item.CustomerId+"|"+item.InboxId] = &item
	return nil
}
func (s *telegramFakeRelStorage) Update(item storageModels.CustomerInboxRel) error { return nil }
func (s *telegramFakeRelStorage) Upsert(item storageModels.CustomerInboxRel) error { return nil }
func (s *telegramFakeRelStorage) Delete(appkey, customerId, inboxId, sourceId string) error {
	return nil
}
func (s *telegramFakeRelStorage) Find(appkey, customerId, inboxId, sourceId string) (*storageModels.CustomerInboxRel, error) {
	return nil, nil
}
func (s *telegramFakeRelStorage) FindByCustomerInbox(appkey, customerId, inboxId string) (*storageModels.CustomerInboxRel, error) {
	return s.byCustomerInbox[customerId+"|"+inboxId], nil
}
func (s *telegramFakeRelStorage) QryByCustomer(appkey, customerId string, startId, limit int64) ([]*storageModels.CustomerInboxRel, error) {
	return nil, nil
}
func (s *telegramFakeRelStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.CustomerInboxRel, error) {
	return nil, nil
}

type telegramFakeTicketStorage struct {
	bySource          map[string]*storageModels.Ticket
	byTicket          map[string]*storageModels.Ticket
	created           *storageModels.Ticket
	updatedTicketId   string
	updatedStatus     storageModels.TicketStatus
	updateStatusCalls int
}

func (s *telegramFakeTicketStorage) Create(item storageModels.Ticket) error {
	s.created = &item
	s.bySource[item.SourceId] = &item
	if s.byTicket != nil {
		s.byTicket[item.TicketId] = &item
	}
	return nil
}
func (s *telegramFakeTicketStorage) Update(item storageModels.Ticket) error { return nil }
func (s *telegramFakeTicketStorage) Upsert(item storageModels.Ticket) error { return nil }
func (s *telegramFakeTicketStorage) Delete(appkey, ticketId string) error   { return nil }
func (s *telegramFakeTicketStorage) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	if s.byTicket == nil {
		return nil, nil
	}
	return s.byTicket[ticketId], nil
}
func (s *telegramFakeTicketStorage) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return s.bySource[sourceId], nil
}
func (s *telegramFakeTicketStorage) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	s.updatedTicketId = ticketId
	s.updatedStatus = status
	s.updateStatusCalls++
	return nil
}
func (s *telegramFakeTicketStorage) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return nil
}
func (s *telegramFakeTicketStorage) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *telegramFakeTicketStorage) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	return nil
}
func (s *telegramFakeTicketStorage) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (s *telegramFakeTicketStorage) UpdateLastCustomerMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (s *telegramFakeTicketStorage) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	return false, nil
}
func (s *telegramFakeTicketStorage) MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error) {
	return false, nil
}
func (s *telegramFakeTicketStorage) QryTicketsNeedingCsatNotification(limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *telegramFakeTicketStorage) String() string {
	return fmt.Sprintf("created=%+v bySource=%+v", s.created, s.bySource)
}
