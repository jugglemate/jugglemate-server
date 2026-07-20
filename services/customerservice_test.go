package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestQryCustomerInfo(t *testing.T) {
	origCustomerStorage := newCustomerStorageForCustomer
	origTicketStorage := newTicketStorageForCustomer
	defer func() {
		newCustomerStorageForCustomer = origCustomerStorage
		newTicketStorageForCustomer = origTicketStorage
	}()
	ticketStorage := &mockTicketStorage{findTicket: &storageModels.Ticket{
		TicketId:   "ticket_1",
		CustomerId: "customer_1",
		SourceId:   "source_1",
	}}
	newTicketStorageForCustomer = func() storageModels.ITicketStorage { return ticketStorage }
	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage {
		return &mockCustomerStorage{customers: map[string]*storageModels.Customer{
			"customer_1": {CustomerId: "customer_1", Nickname: "Alice", Avator: "alice.png"},
		}}
	}
	ctx := context.WithValue(context.Background(), ctxs.CtxKey_AppKey, "app_1")
	ctx = context.WithValue(ctx, ctxs.CtxKey_RequesterId, "agent_1")

	code, info := QryCustomerInfo(ctx, "ticket_1")
	if code != errs.IMErrorCode_SUCCESS || info == nil || info.CustomerId != "customer_1" || info.SourceId != "source_1" || info.Nickname != "Alice" || info.Avatar != "alice.png" {
		t.Fatalf("code=%d info=%+v", code, info)
	}
	if ticketStorage.findAppkey != "app_1" || ticketStorage.findTicketId != "ticket_1" {
		t.Fatalf("ticket query appkey=%q ticket_id=%q", ticketStorage.findAppkey, ticketStorage.findTicketId)
	}
}

func TestQryCustomerInfoNotFoundAndStorageError(t *testing.T) {
	origCustomerStorage := newCustomerStorageForCustomer
	origTicketStorage := newTicketStorageForCustomer
	defer func() {
		newCustomerStorageForCustomer = origCustomerStorage
		newTicketStorageForCustomer = origTicketStorage
	}()
	ctx := context.WithValue(context.Background(), ctxs.CtxKey_AppKey, "app_1")
	ctx = context.WithValue(ctx, ctxs.CtxKey_RequesterId, "agent_1")

	newTicketStorageForCustomer = func() storageModels.ITicketStorage {
		return &mockTicketStorage{}
	}
	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage {
		return &mockCustomerStorage{}
	}
	if code, _ := QryCustomerInfo(ctx, "missing"); code != errs.IMErrorCode_APP_USER_NOT_EXIST {
		t.Fatalf("ticket not found code=%d", code)
	}

	newTicketStorageForCustomer = func() storageModels.ITicketStorage {
		return &mockTicketStorage{err: errors.New("query failed")}
	}
	if code, _ := QryCustomerInfo(ctx, "ticket_1"); code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("ticket storage error code=%d", code)
	}

	newTicketStorageForCustomer = func() storageModels.ITicketStorage {
		return &mockTicketStorage{findTicket: &storageModels.Ticket{TicketId: "ticket_1", CustomerId: "customer_1"}}
	}
	if code, _ := QryCustomerInfo(ctx, "ticket_1"); code != errs.IMErrorCode_APP_USER_NOT_EXIST {
		t.Fatalf("customer not found code=%d", code)
	}

	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage {
		return &mockCustomerStorage{err: errors.New("query failed")}
	}
	if code, _ := QryCustomerInfo(ctx, "ticket_1"); code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("customer storage error code=%d", code)
	}
}

func TestQryCustomerTicketsUsesCursorPagination(t *testing.T) {
	customerStorage := &mockCustomerStorage{customers: map[string]*storageModels.Customer{
		"customer_1": {CustomerId: "customer_1", Nickname: "Alice"},
	}}
	ticketStorage := &mockTicketStorage{tickets: []*storageModels.Ticket{
		{ID: 30, TicketId: "ticket_30", CustomerId: "customer_1", Status: storageModels.TicketStatusClosed},
		{ID: 20, TicketId: "ticket_20", CustomerId: "customer_1", Status: storageModels.TicketStatusProcessing},
		{ID: 10, TicketId: "ticket_10", CustomerId: "customer_1", Status: storageModels.TicketStatusPending},
	}}
	restoreTicketDeps := mockTicketStorages(&mockUserStorage{}, customerStorage, ticketStorage)
	origCustomerTicketStorage := newTicketStorageForCustomer
	origCustomerStorage := newCustomerStorageForCustomer
	newTicketStorageForCustomer = func() storageModels.ITicketStorage { return ticketStorage }
	newCustomerStorageForCustomer = func() storageModels.ICustomerStorage { return customerStorage }
	defer func() {
		restoreTicketDeps()
		newTicketStorageForCustomer = origCustomerTicketStorage
		newCustomerStorageForCustomer = origCustomerStorage
	}()

	ctx := context.WithValue(context.Background(), ctxs.CtxKey_AppKey, "app_1")
	ctx = context.WithValue(ctx, ctxs.CtxKey_RequesterId, "agent_1")
	code, resp := QryCustomerTickets(ctx, &apiModels.QryCustomerTicketsReq{
		CustomerId: "customer_1",
		StartId:    40,
		Limit:      2,
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code=%d", code)
	}
	if ticketStorage.customerId != "customer_1" || ticketStorage.startId != 40 || ticketStorage.customerLimit != 3 {
		t.Fatalf("query customer=%q start=%d limit=%d", ticketStorage.customerId, ticketStorage.startId, ticketStorage.customerLimit)
	}
	if resp == nil || len(resp.Items) != 2 || resp.Items[0].TicketId != "ticket_30" || resp.Items[1].TicketId != "ticket_20" || resp.NextStartId != 20 {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestValidateWidgetInbox(t *testing.T) {
	tests := []struct {
		name  string
		inbox *storageModels.Inbox
		want  errs.IMErrorCode
	}{
		{
			name:  "missing inbox",
			inbox: nil,
			want:  errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "telegram inbox rejected",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_1",
				ChannelType: string(ChannelType_Telegram),
			},
			want: errs.IMErrorCode_APP_CHANNEL_NOT_EXIST,
		},
		{
			name: "widget inbox accepted",
			inbox: &storageModels.Inbox{
				InboxId:     "inbox_widget_1",
				ChannelType: string(ChannelType_Widget),
			},
			want: errs.IMErrorCode_SUCCESS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateWidgetInbox(tt.inbox); got != tt.want {
				t.Fatalf("validateWidgetInbox() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildTicketGroupMemberIds(t *testing.T) {
	got := buildTicketGroupMemberIds("customer_1", []string{"u_1", "u_2", "u_1", "", "customer_1"})
	want := []string{"customer_1", "u_1", "u_2"}
	if len(got) != len(want) {
		t.Fatalf("member ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member ids = %v, want %v", got, want)
		}
	}
}

func TestGenerateTicketIdUsesTicketPrefix(t *testing.T) {
	ticketId := generateTicketId()
	if !strings.HasPrefix(ticketId, TicketIDPrefix) {
		t.Fatalf("ticket id = %q, want prefix %q", ticketId, TicketIDPrefix)
	}
	if len(ticketId) <= len(TicketIDPrefix) {
		t.Fatalf("ticket id = %q, want random suffix", ticketId)
	}
}

// TestPrepareTicketGroupMemberIdsExcludesSeats 校验建群成员只有客户与 Agent Bot。
//
// TIPS: 坐席改为转人工时才入群（SwitchTicketToHuman）。这里显式断言"不注册任何坐席"，
// 防止有人为了修别的问题把注册循环加回来 —— 那会让客户与 Agent 的对话默认暴露给全部坐席。
func TestPrepareTicketGroupMemberIdsExcludesSeats(t *testing.T) {
	registered := map[string]struct{}{}
	oldRegister := registerIMUserForCustomer
	oldResolveBot := resolveInboxAgentBotForCustomer
	registerIMUserForCustomer = func(sdk *juggleimsdk.JuggleIMSdk, userId, nickname, portrait string) errs.IMErrorCode {
		registered[userId] = struct{}{}
		return errs.IMErrorCode_SUCCESS
	}
	resolveInboxAgentBotForCustomer = func(context.Context, string, string) (string, error) { return "bot_1", nil }
	t.Cleanup(func() {
		registerIMUserForCustomer = oldRegister
		resolveInboxAgentBotForCustomer = oldResolveBot
	})

	code, memberIds := prepareTicketGroupMemberIds("app_1", "inbox_1", "customer_1")
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if len(memberIds) != 2 || memberIds[0] != "customer_1" || memberIds[1] != "bot_1" {
		t.Fatalf("member ids = %v，期望仅有客户与 Bot", memberIds)
	}
	if len(registered) != 0 {
		t.Fatalf("建群不应注册任何坐席，实际注册了 %v", registered)
	}
}

// TestPrepareTicketGroupMemberIdsWithoutAgentBot 校验 Inbox 未绑定 Agent 时只有客户自己。
func TestPrepareTicketGroupMemberIdsWithoutAgentBot(t *testing.T) {
	oldResolveBot := resolveInboxAgentBotForCustomer
	resolveInboxAgentBotForCustomer = func(context.Context, string, string) (string, error) { return "", nil }
	t.Cleanup(func() { resolveInboxAgentBotForCustomer = oldResolveBot })

	code, memberIds := prepareTicketGroupMemberIds("app_1", "inbox_1", "customer_1")
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if len(memberIds) != 1 || memberIds[0] != "customer_1" {
		t.Fatalf("member ids = %v，期望仅有客户", memberIds)
	}
}

type customerInboxMemberStorage struct {
	members []*storageModels.InboxMember
}

func (s *customerInboxMemberStorage) Create(item storageModels.InboxMember) error { return nil }
func (s *customerInboxMemberStorage) Upsert(item storageModels.InboxMember) error { return nil }
func (s *customerInboxMemberStorage) Delete(appkey, inboxId, memberId string) error {
	return nil
}
func (s *customerInboxMemberStorage) Find(appkey, inboxId, memberId string) (*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return s.members, nil
}
func (s *customerInboxMemberStorage) QryByMember(appkey, memberId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	return nil, nil
}
func (s *customerInboxMemberStorage) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	return nil
}

type customerUserStorage struct {
	users map[string]*storageModels.User
}

func (s *customerUserStorage) Create(item storageModels.User) error { return nil }
func (s *customerUserStorage) FindByAccount(appkey, account string) (*storageModels.User, error) {
	return nil, nil
}
func (s *customerUserStorage) FindByUserId(appkey, userId string) (*storageModels.User, error) {
	if s.users == nil {
		return nil, nil
	}
	return s.users[userId], nil
}
func (s *customerUserStorage) FindByAccountWithAppkey(account, appkey string) (*storageModels.User, error) {
	return nil, nil
}
func (s *customerUserStorage) UpdateImToken(appkey, userId, imToken string) error { return nil }
func (s *customerUserStorage) QryByApp(appkey string, filter storageModels.UserListFilter, limit, offset int64) (*storageModels.UserListResult, error) {
	return nil, nil
}
func (s *customerUserStorage) UpdateUser(appkey, userId string, updates storageModels.UserUpdate) error {
	return nil
}
func (s *customerUserStorage) Delete(appkey, userId string) error { return nil }
