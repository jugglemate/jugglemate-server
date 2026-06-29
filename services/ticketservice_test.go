package services

import (
	"context"
	"errors"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestQryTicketsAdminUsesAllTicketsQuery(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		tickets: []*storageModels.Ticket{
			{ID: 3, TicketId: "t3", Status: storageModels.TicketStatusProcessing},
			{ID: 2, TicketId: "t2", Status: storageModels.TicketStatusPending},
		},
	}
	restore := mockTicketStorages(&mockUserStorage{
		user: &storageModels.User{UserId: "admin", Role: storageModels.UserRoleAdmin},
	}, &mockCustomerStorage{}, ticketStorage)
	defer restore()

	status := 1
	code, resp := QryTickets(ticketTestContext("app_1", "admin"), &apiModels.QryTicketsReq{
		Status: &status,
		Limit:  10,
		Offset: 20,
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("QryTickets code = %d, want success", code)
	}
	if ticketStorage.called != "all" {
		t.Fatalf("called query = %q, want all", ticketStorage.called)
	}
	if ticketStorage.appkey != "app_1" || ticketStorage.limit != 10 || ticketStorage.offset != 20 {
		t.Fatalf("query args = appkey:%q limit:%d offset:%d", ticketStorage.appkey, ticketStorage.limit, ticketStorage.offset)
	}
	if ticketStorage.status == nil || *ticketStorage.status != storageModels.TicketStatusProcessing {
		t.Fatalf("status = %v, want processing", ticketStorage.status)
	}
	if len(resp.Items) != 2 || resp.Items[0].TicketId != "t3" || resp.Items[1].TicketId != "t2" {
		t.Fatalf("items = %+v, want returned ticket order", resp.Items)
	}
}

func TestQryTicketsCustomerServiceUsesVisibleQuery(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		tickets: []*storageModels.Ticket{
			{ID: 8, TicketId: "pending", Status: storageModels.TicketStatusPending},
			{ID: 7, TicketId: "assigned", CustomerId: "c_1", AssigneeId: "u_1", Status: storageModels.TicketStatusClosed},
		},
	}
	restore := mockTicketStorages(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Nickname: "Agent", Avator: "agent.png", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{
		customers: map[string]*storageModels.Customer{
			"c_1": {CustomerId: "c_1", Nickname: "Customer", Avator: "customer.png"},
		},
	}, ticketStorage)
	defer restore()

	code, resp := QryTickets(ticketTestContext("app_1", "u_1"), &apiModels.QryTicketsReq{
		Limit:  20,
		Offset: 0,
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("QryTickets code = %d, want success", code)
	}
	if ticketStorage.called != "visible" {
		t.Fatalf("called query = %q, want visible", ticketStorage.called)
	}
	if ticketStorage.assigneeId != "u_1" {
		t.Fatalf("assigneeId = %q, want u_1", ticketStorage.assigneeId)
	}
	if ticketStorage.status != nil {
		t.Fatalf("status = %v, want nil", ticketStorage.status)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(resp.Items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[1].Customer == nil || resp.Items[1].Customer.Id != "c_1" || resp.Items[1].Customer.Nickname != "Customer" || resp.Items[1].Customer.Avatar != "customer.png" {
		t.Fatalf("customer = %+v, want c_1 Customer customer.png", resp.Items[1].Customer)
	}
	if resp.Items[1].Assignee == nil || resp.Items[1].Assignee.Id != "u_1" || resp.Items[1].Assignee.Nickname != "Agent" || resp.Items[1].Assignee.Avatar != "agent.png" {
		t.Fatalf("assignee = %+v, want u_1 Agent agent.png", resp.Items[1].Assignee)
	}
}

func TestQryTicketsRejectsInvalidStatus(t *testing.T) {
	restore := mockTicketStorages(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleAdmin},
	}, &mockCustomerStorage{}, &mockTicketStorage{})
	defer restore()

	status := 9
	code, _ := QryTickets(ticketTestContext("app_1", "u_1"), &apiModels.QryTicketsReq{
		Status: &status,
		Limit:  20,
	})
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("QryTickets code = %d, want param error", code)
	}
}

func TestQryTicketsMissingRequester(t *testing.T) {
	code, _ := QryTickets(ticketTestContext("app_1", ""), &apiModels.QryTicketsReq{Limit: 20})
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("QryTickets code = %d, want not login", code)
	}
}

func TestClaimTicketSuccess(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		claimTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			SourceId:   "customer_1",
			CustomerId: "c_1",
			InboxId:    "inbox_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Nickname: "Agent", Avator: "agent.png", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{
		customers: map[string]*storageModels.Customer{
			"c_1": {CustomerId: "c_1", Nickname: "Customer", Avator: "customer.png"},
		},
	}, ticketStorage, &mockInboxMemberStorage{
		members: []*storageModels.InboxMember{
			{ID: 2, InboxId: "inbox_1", MemberId: "u_1"},
			{ID: 1, InboxId: "inbox_1", MemberId: "u_2"},
		},
	}, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	})
	defer restore()

	var notifyCount int
	sendTicketAssignedNtfMsgForClaim = func(_ context.Context, msg *TicketAssignedNtfMsg) {
		notifyCount++
		if msg.TicketId != "t_1" || msg.AssignType != AssignType_Claim {
			t.Fatalf("notify msg = %+v", msg)
		}
	}

	code, resp := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("ClaimTicket code = %d, want success", code)
	}
	if ticketStorage.claimAppkey != "app_1" || ticketStorage.claimTicketId != "t_1" || ticketStorage.claimAssigneeId != "u_1" {
		t.Fatalf("claim args = appkey:%q ticketId:%q assigneeId:%q", ticketStorage.claimAppkey, ticketStorage.claimTicketId, ticketStorage.claimAssigneeId)
	}
	if resp.Ticket == nil || resp.Ticket.TicketId != "t_1" || resp.Ticket.Status != int(storageModels.TicketStatusProcessing) {
		t.Fatalf("ticket = %+v, want processing t_1", resp.Ticket)
	}
	if resp.Ticket.InboxId != "inbox_1" {
		t.Fatalf("ticket inbox = %q, want inbox_1", resp.Ticket.InboxId)
	}
	if notifyCount != 1 {
		t.Fatalf("notifyCount = %d, want 1", notifyCount)
	}
}

func TestClaimTicketRejectsNonPending(t *testing.T) {
	ticketStorage := &mockTicketStorage{}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, &mockInboxMemberStorage{}, nil)
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("ClaimTicket code = %d, want param error", code)
	}
}

func TestClaimTicketRevertsWhenInboxMemberQueryFails(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		claimTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			InboxId:    "inbox_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, &mockInboxMemberStorage{err: errors.New("query failed")}, nil)
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("ClaimTicket code = %d, want internal timeout", code)
	}
	if ticketStorage.revertAppkey != "app_1" || ticketStorage.revertTicketId != "t_1" || ticketStorage.revertAssigneeId != "u_1" {
		t.Fatalf("revert args = appkey:%q ticketId:%q assigneeId:%q", ticketStorage.revertAppkey, ticketStorage.revertTicketId, ticketStorage.revertAssigneeId)
	}
}

func TestClaimTicketRevertsWhenAssignedTagFails(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		claimTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			InboxId:    "inbox_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, &mockInboxMemberStorage{
		members: []*storageModels.InboxMember{{ID: 1, InboxId: "inbox_1", MemberId: "u_2"}},
	}, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		if req.Tag == ticketConversationTagAssigned {
			return juggleimsdk.ApiCode(errs.IMErrorCode_APP_INTERNAL_TIMEOUT), "", nil
		}
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	})
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("ClaimTicket code = %d, want internal timeout", code)
	}
	if ticketStorage.revertAppkey != "app_1" || ticketStorage.revertTicketId != "t_1" || ticketStorage.revertAssigneeId != "u_1" {
		t.Fatalf("revert args = appkey:%q ticketId:%q assigneeId:%q", ticketStorage.revertAppkey, ticketStorage.revertTicketId, ticketStorage.revertAssigneeId)
	}
}

func TestClaimTicketRevertsWhenMyTicketTagFails(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		claimTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			InboxId:    "inbox_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, &mockInboxMemberStorage{
		members: []*storageModels.InboxMember{{ID: 1, InboxId: "inbox_1", MemberId: "u_2"}},
	}, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		if req.Tag == ticketConversationTagMyTicket {
			return juggleimsdk.ApiCode(errs.IMErrorCode_APP_INTERNAL_TIMEOUT), "", nil
		}
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	})
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("ClaimTicket code = %d, want internal timeout", code)
	}
	if ticketStorage.revertAppkey != "app_1" || ticketStorage.revertTicketId != "t_1" || ticketStorage.revertAssigneeId != "u_1" {
		t.Fatalf("revert args = appkey:%q ticketId:%q assigneeId:%q", ticketStorage.revertAppkey, ticketStorage.revertTicketId, ticketStorage.revertAssigneeId)
	}
}

func TestClaimTicketDoesNotTagSourceUserAssigned(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		claimTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			SourceId:   "customer_1",
			InboxId:    "inbox_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	var assignedUsers []string
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, &mockInboxMemberStorage{
		members: []*storageModels.InboxMember{{ID: 1, InboxId: "inbox_1", MemberId: "u_2"}},
	}, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		if req.Tag == ticketConversationTagAssigned {
			assignedUsers = append(assignedUsers, req.UserId)
		}
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	})
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("ClaimTicket code = %d, want success", code)
	}
	if len(assignedUsers) != 1 || assignedUsers[0] != "u_2" {
		t.Fatalf("assignedUsers = %+v, want only u_2", assignedUsers)
	}
}

func ticketTestContext(appkey, requesterId string) context.Context {
	ctx := context.Background()
	if appkey != "" {
		ctx = context.WithValue(ctx, ctxs.CtxKey_AppKey, appkey)
	}
	if requesterId != "" {
		ctx = context.WithValue(ctx, ctxs.CtxKey_RequesterId, requesterId)
	}
	return ctx
}

func mockTicketStorages(userStorage *mockUserStorage, customerStorage *mockCustomerStorage, ticketStorage *mockTicketStorage) func() {
	origUserStorage := newUserStorageForTicket
	origCustomerStorage := newCustomerStorageForTicket
	origTicketStorage := newTicketStorageForQuery
	newUserStorageForTicket = func() storageModels.IUserStorage {
		return userStorage
	}
	newCustomerStorageForTicket = func() storageModels.ICustomerStorage {
		return customerStorage
	}
	newTicketStorageForQuery = func() storageModels.ITicketStorage {
		return ticketStorage
	}
	return func() {
		newUserStorageForTicket = origUserStorage
		newCustomerStorageForTicket = origCustomerStorage
		newTicketStorageForQuery = origTicketStorage
	}
}

func mockTicketClaimDeps(
	userStorage *mockUserStorage,
	customerStorage *mockCustomerStorage,
	ticketStorage *mockTicketStorage,
	inboxMemberStorage *mockInboxMemberStorage,
	tagConvers func(*juggleimsdk.JuggleIMSdk, juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error),
) func() {
	restoreStorages := mockTicketStorages(userStorage, customerStorage, ticketStorage)
	origGetImSdk := getImSdkForTicketClaim
	origInboxMemberStorage := newInboxMemberStorageForTicket
	origTagConvers := tagConversForClaim
	origSendTicketAssignedNtfMsg := sendTicketAssignedNtfMsgForClaim
	getImSdkForTicketClaim = func(appkey string) *juggleimsdk.JuggleIMSdk {
		return &juggleimsdk.JuggleIMSdk{}
	}
	if inboxMemberStorage != nil {
		newInboxMemberStorageForTicket = func() storageModels.IInboxMemberStorage {
			return inboxMemberStorage
		}
	}
	if tagConvers != nil {
		tagConversForClaim = tagConvers
	}
	sendTicketAssignedNtfMsgForClaim = func(context.Context, *TicketAssignedNtfMsg) {}
	return func() {
		restoreStorages()
		getImSdkForTicketClaim = origGetImSdk
		newInboxMemberStorageForTicket = origInboxMemberStorage
		tagConversForClaim = origTagConvers
		sendTicketAssignedNtfMsgForClaim = origSendTicketAssignedNtfMsg
	}
}

func assertTicketTagReq(t *testing.T, req juggleimsdk.TagConversReq) {
	t.Helper()
	if req.UserId == "" {
		t.Fatalf("TagConvers req missing user_id: %+v", req)
	}
	if req.Tag != ticketConversationTagAssigned && req.Tag != ticketConversationTagMyTicket {
		t.Fatalf("TagConvers tag = %q, want assigned or my_ticket", req.Tag)
	}
	if len(req.Convers) != 1 || req.Convers[0] == nil {
		t.Fatalf("TagConvers convers = %+v, want one conversation", req.Convers)
	}
	if req.Convers[0].TargetId != "t_1" {
		t.Fatalf("TagConvers target = %q, want t_1", req.Convers[0].TargetId)
	}
	if req.Convers[0].ChannelType != int(juggleimsdk.ChannelType_Group) {
		t.Fatalf("TagConvers channel_type = %d, want group", req.Convers[0].ChannelType)
	}
}

type mockUserStorage struct {
	user *storageModels.User
	err  error
}

func (s *mockUserStorage) Create(item storageModels.User) error {
	return nil
}

func (s *mockUserStorage) FindByAccount(appkey, account string) (*storageModels.User, error) {
	return nil, nil
}

func (s *mockUserStorage) FindByUserId(appkey, userId string) (*storageModels.User, error) {
	return s.user, s.err
}

func (s *mockUserStorage) FindByAccountWithAppkey(account, appkey string) (*storageModels.User, error) {
	return nil, nil
}

func (s *mockUserStorage) UpdateImToken(appkey, userId, imToken string) error {
	return nil
}

func (s *mockUserStorage) QryByApp(appkey string, filter storageModels.UserListFilter, limit, offset int64) (*storageModels.UserListResult, error) {
	return &storageModels.UserListResult{}, nil
}

func (s *mockUserStorage) UpdateUser(appkey, userId string, updates storageModels.UserUpdate) error {
	return nil
}

func (s *mockUserStorage) Delete(appkey, userId string) error {
	return nil
}

type mockCustomerStorage struct {
	customers map[string]*storageModels.Customer
	err       error
}

func (s *mockCustomerStorage) Create(item storageModels.Customer) error {
	return nil
}

func (s *mockCustomerStorage) Update(item storageModels.Customer) error {
	return nil
}

func (s *mockCustomerStorage) Upsert(item storageModels.Customer) error {
	return nil
}

func (s *mockCustomerStorage) Delete(appkey, customerId string) error {
	return nil
}

func (s *mockCustomerStorage) FindByCustomerId(appkey, customerId string) (*storageModels.Customer, error) {
	if s.customers == nil {
		return nil, s.err
	}
	return s.customers[customerId], s.err
}

func (s *mockCustomerStorage) FindByIdentifier(appkey, identifier string) (*storageModels.Customer, error) {
	return nil, nil
}

type mockInboxMemberStorage struct {
	members []*storageModels.InboxMember
	err     error

	qryAppkey  string
	qryInboxId string
	qryStartId int64
	qryLimit   int64
}

func (s *mockInboxMemberStorage) Create(item storageModels.InboxMember) error {
	return nil
}

func (s *mockInboxMemberStorage) Upsert(item storageModels.InboxMember) error {
	return nil
}

func (s *mockInboxMemberStorage) Delete(appkey, inboxId, memberId string) error {
	return nil
}

func (s *mockInboxMemberStorage) Find(appkey, inboxId, memberId string) (*storageModels.InboxMember, error) {
	return nil, nil
}

func (s *mockInboxMemberStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	s.qryAppkey = appkey
	s.qryInboxId = inboxId
	s.qryStartId = startId
	s.qryLimit = limit
	return s.members, s.err
}

func (s *mockInboxMemberStorage) QryByMember(appkey, memberId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}

func (s *mockInboxMemberStorage) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	return nil, nil
}

func (s *mockInboxMemberStorage) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	return nil
}

type mockTicketStorage struct {
	called     string
	appkey     string
	assigneeId string
	status     *storageModels.TicketStatus
	limit      int64
	offset     int64
	tickets    []*storageModels.Ticket
	err        error

	claimAppkey      string
	claimTicketId    string
	claimAssigneeId  string
	claimTicket      *storageModels.Ticket
	revertAppkey     string
	revertTicketId   string
	revertAssigneeId string
}

func (s *mockTicketStorage) Create(item storageModels.Ticket) error {
	return nil
}

func (s *mockTicketStorage) Update(item storageModels.Ticket) error {
	return nil
}

func (s *mockTicketStorage) Upsert(item storageModels.Ticket) error {
	return nil
}

func (s *mockTicketStorage) Delete(appkey, ticketId string) error {
	return nil
}

func (s *mockTicketStorage) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryAll(appkey string, status *storageModels.TicketStatus, limit, offset int64) ([]*storageModels.Ticket, error) {
	s.called = "all"
	s.appkey = appkey
	s.status = status
	s.limit = limit
	s.offset = offset
	return s.tickets, s.err
}

func (s *mockTicketStorage) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, limit, offset int64) ([]*storageModels.Ticket, error) {
	s.called = "visible"
	s.appkey = appkey
	s.assigneeId = assigneeId
	s.status = status
	s.limit = limit
	s.offset = offset
	return s.tickets, s.err
}

func (s *mockTicketStorage) QryByCustomer(appkey, customerId string, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}

func (s *mockTicketStorage) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	s.claimAppkey = appkey
	s.claimTicketId = ticketId
	s.claimAssigneeId = assigneeId
	return s.claimTicket, s.err
}

func (s *mockTicketStorage) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	s.revertAppkey = appkey
	s.revertTicketId = ticketId
	s.revertAssigneeId = assigneeId
	return s.err
}
