package services

import (
	"context"
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
			{ID: 8, TicketId: "pending", ChannelType: "telegram", Status: storageModels.TicketStatusPending},
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
	if resp.Items[0].ChannelType != "telegram" {
		t.Fatalf("channel type = %q, want telegram", resp.Items[0].ChannelType)
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

func TestQryTicketInboxMembersUsesTicketInboxAndPaginates(t *testing.T) {
	userStorage := &mockUserStorage{users: map[string]*storageModels.User{
		"admin": {UserId: "admin", Role: storageModels.UserRoleAdmin},
		"u_1":   {UserId: "u_1", LoginAccount: "agent1@example.com", Avator: "a1.png", Email: "agent1@example.com"},
		"u_2":   {UserId: "u_2", LoginAccount: "agent2@example.com", Avator: "a2.png", Email: "agent2@example.com"},
	}}
	ticketStorage := &mockTicketStorage{findTicket: &storageModels.Ticket{TicketId: "t_1", InboxId: "inbox_1"}}
	memberStorage := &mockInboxMemberStorage{members: []*storageModels.InboxMember{
		{ID: 10, InboxId: "inbox_1", MemberId: "u_1"},
		{ID: 9, InboxId: "inbox_1", MemberId: "u_2"},
		{ID: 8, InboxId: "inbox_1", MemberId: "u_3"},
	}}
	restore := mockTicketStorages(userStorage, &mockCustomerStorage{}, ticketStorage)
	oldMemberStorage := newInboxMemberStorageForTicket
	newInboxMemberStorageForTicket = func() storageModels.IInboxMemberStorage { return memberStorage }
	defer func() {
		restore()
		newInboxMemberStorageForTicket = oldMemberStorage
	}()

	code, resp := QryTicketInboxMembers(ticketTestContext("app_1", "admin"), &apiModels.QryTicketInboxMembersReq{
		TicketId: "t_1",
		StartId:  20,
		Limit:    2,
	})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("QryTicketInboxMembers code = %d, want success", code)
	}
	if memberStorage.appkey != "app_1" || memberStorage.inboxId != "inbox_1" || memberStorage.startId != 20 || memberStorage.limit != 3 {
		t.Fatalf("member query args = appkey:%q inbox:%q start:%d limit:%d", memberStorage.appkey, memberStorage.inboxId, memberStorage.startId, memberStorage.limit)
	}
	if len(resp.Items) != 2 || resp.Items[0].UserId != "u_1" || resp.Items[1].UserId != "u_2" {
		t.Fatalf("items = %+v", resp.Items)
	}
	if resp.NextStartId != 9 {
		t.Fatalf("next_start_id = %d, want 9", resp.NextStartId)
	}
}

func TestQryTicketInboxMembersRejectsMissingTicket(t *testing.T) {
	restore := mockTicketStorages(
		&mockUserStorage{user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService}},
		&mockCustomerStorage{},
		&mockTicketStorage{},
	)
	defer restore()

	code, _ := QryTicketInboxMembers(ticketTestContext("app_1", "u_1"), &apiModels.QryTicketInboxMembersReq{TicketId: "missing", Limit: 50})
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("QryTicketInboxMembers code = %d, want param error", code)
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
	var tagCount int
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Nickname: "Agent", Avator: "agent.png", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{
		customers: map[string]*storageModels.Customer{
			"c_1": {CustomerId: "c_1", Nickname: "Customer", Avator: "customer.png"},
		},
	}, ticketStorage, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		tagCount++
		assertTicketTagReq(t, req)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	})
	defer restore()

	var notifyCount int
	var globalTagSyncCount int
	syncTicketGlobalConversationTagsForClaim = func(appkey, ticketId string) errs.IMErrorCode {
		globalTagSyncCount++
		if appkey != "app_1" || ticketId != "t_1" {
			t.Fatalf("global tag sync app=%q ticket=%q", appkey, ticketId)
		}
		return errs.IMErrorCode_SUCCESS
	}
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
	if tagCount != 1 {
		t.Fatalf("tagCount = %d, want only my_ticket tag", tagCount)
	}
	if globalTagSyncCount != 1 {
		t.Fatalf("globalTagSyncCount = %d, want 1", globalTagSyncCount)
	}
}

func TestClaimTicketRejectsNonPending(t *testing.T) {
	ticketStorage := &mockTicketStorage{}
	restore := mockTicketClaimDeps(&mockUserStorage{
		user: &storageModels.User{UserId: "u_1", Role: storageModels.UserRoleCustomerService},
	}, &mockCustomerStorage{}, ticketStorage, nil)
	defer restore()

	code, _ := ClaimTicket(ticketTestContext("app_1", "u_1"), "t_1")
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("ClaimTicket code = %d, want param error", code)
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
	}, &mockCustomerStorage{}, ticketStorage, func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
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

func TestTransferTicketSuccess(t *testing.T) {
	operator := &storageModels.User{UserId: "u_1", Nickname: "Operator", Avator: "operator.png", Role: storageModels.UserRoleCustomerService}
	assignee := &storageModels.User{UserId: "u_2", Nickname: "Assignee", Avator: "assignee.png", Role: storageModels.UserRoleCustomerService}
	ticketStorage := &mockTicketStorage{
		findTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
		transferTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			AssigneeId: "u_2",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketStorages(&mockUserStorage{users: map[string]*storageModels.User{
		"u_1": operator,
		"u_2": assignee,
	}}, &mockCustomerStorage{}, ticketStorage)
	origSend := sendTicketAssignedNtfMsgForTransfer
	origGetSDK := getImSdkForTicketTransfer
	origTag := tagConversForTransfer
	origUnTag := unTagConversForTransfer
	defer func() {
		restore()
		sendTicketAssignedNtfMsgForTransfer = origSend
		getImSdkForTicketTransfer = origGetSDK
		tagConversForTransfer = origTag
		unTagConversForTransfer = origUnTag
	}()

	var notified *TicketAssignedNtfMsg
	var taggedUsers, untaggedUsers []string
	var globalTagSyncCount int
	syncTicketGlobalConversationTagsForTransfer = func(appkey, ticketId string) errs.IMErrorCode {
		globalTagSyncCount++
		if appkey != "app_1" || ticketId != "t_1" {
			t.Fatalf("global tag sync app=%q ticket=%q", appkey, ticketId)
		}
		return errs.IMErrorCode_SUCCESS
	}
	getImSdkForTicketTransfer = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	tagConversForTransfer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		taggedUsers = append(taggedUsers, req.UserId)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	unTagConversForTransfer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		assertTicketTagReq(t, req)
		untaggedUsers = append(untaggedUsers, req.UserId)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	sendTicketAssignedNtfMsgForTransfer = func(_ context.Context, msg *TicketAssignedNtfMsg) {
		notified = msg
	}
	code, resp := TransferTicket(ticketTestContext("app_1", "u_1"), "t_1", &apiModels.TransferTicketReq{AssigneeId: "u_2"})
	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("TransferTicket code = %d, want success", code)
	}
	if ticketStorage.transferAppkey != "app_1" || ticketStorage.transferTicketId != "t_1" || ticketStorage.transferOldId != "u_1" || ticketStorage.transferNewId != "u_2" {
		t.Fatalf("transfer args = appkey:%q ticket:%q old:%q new:%q", ticketStorage.transferAppkey, ticketStorage.transferTicketId, ticketStorage.transferOldId, ticketStorage.transferNewId)
	}
	if resp == nil || resp.Ticket == nil || resp.Ticket.AssigneeId != "u_2" || resp.Ticket.Assignee == nil || resp.Ticket.Assignee.Id != "u_2" {
		t.Fatalf("transfer resp = %+v", resp)
	}
	if notified == nil || notified.TicketId != "t_1" || notified.AssignType != AssignType_AssignUser || notified.Operator == nil || notified.Operator.Id != "u_1" || notified.Assignee == nil || notified.Assignee.Id != "u_2" {
		t.Fatalf("notification = %+v", notified)
	}
	if len(untaggedUsers) != 1 || untaggedUsers[0] != "u_1" {
		t.Fatalf("untagged users = %+v, want u_1", untaggedUsers)
	}
	if len(taggedUsers) != 1 || taggedUsers[0] != "u_2" {
		t.Fatalf("tagged users = %+v, want u_2", taggedUsers)
	}
	if globalTagSyncCount != 1 {
		t.Fatalf("globalTagSyncCount = %d, want 1", globalTagSyncCount)
	}
}

func TestTransferTicketRollsBackWhenNewAssigneeTagFails(t *testing.T) {
	ticketStorage := &mockTicketStorage{
		findTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			AssigneeId: "u_1",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
		transferTicket: &storageModels.Ticket{
			TicketId:   "t_1",
			AssigneeId: "u_2",
			Status:     storageModels.TicketStatusProcessing,
			AppKey:     "app_1",
		},
	}
	restore := mockTicketStorages(&mockUserStorage{users: map[string]*storageModels.User{
		"u_1": {UserId: "u_1", Role: storageModels.UserRoleCustomerService},
		"u_2": {UserId: "u_2", Role: storageModels.UserRoleCustomerService},
	}}, &mockCustomerStorage{}, ticketStorage)
	origGetSDK := getImSdkForTicketTransfer
	origTag := tagConversForTransfer
	origUnTag := unTagConversForTransfer
	origSend := sendTicketAssignedNtfMsgForTransfer
	defer func() {
		restore()
		getImSdkForTicketTransfer = origGetSDK
		tagConversForTransfer = origTag
		unTagConversForTransfer = origUnTag
		sendTicketAssignedNtfMsgForTransfer = origSend
	}()

	var taggedUsers, untaggedUsers []string
	var notified bool
	getImSdkForTicketTransfer = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	unTagConversForTransfer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		untaggedUsers = append(untaggedUsers, req.UserId)
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	tagConversForTransfer = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error) {
		taggedUsers = append(taggedUsers, req.UserId)
		if req.UserId == "u_2" {
			return juggleimsdk.ApiCode(errs.IMErrorCode_APP_INTERNAL_TIMEOUT), "", nil
		}
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	sendTicketAssignedNtfMsgForTransfer = func(context.Context, *TicketAssignedNtfMsg) { notified = true }

	code, _ := TransferTicket(ticketTestContext("app_1", "u_1"), "t_1", &apiModels.TransferTicketReq{AssigneeId: "u_2"})
	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("TransferTicket code = %d, want internal timeout", code)
	}
	if ticketStorage.transferOldId != "u_2" || ticketStorage.transferNewId != "u_1" {
		t.Fatalf("rollback transfer old=%q new=%q, want u_2 to u_1", ticketStorage.transferOldId, ticketStorage.transferNewId)
	}
	if len(untaggedUsers) != 1 || untaggedUsers[0] != "u_1" {
		t.Fatalf("untagged users = %+v, want u_1", untaggedUsers)
	}
	if len(taggedUsers) != 2 || taggedUsers[0] != "u_2" || taggedUsers[1] != "u_1" {
		t.Fatalf("tagged users = %+v, want failed u_2 then restored u_1", taggedUsers)
	}
	if notified {
		t.Fatal("notification sent for rolled-back transfer")
	}
}

func TestTransferTicketRejectsNonOwner(t *testing.T) {
	ticketStorage := &mockTicketStorage{findTicket: &storageModels.Ticket{
		TicketId:   "t_1",
		AssigneeId: "u_3",
		Status:     storageModels.TicketStatusProcessing,
		AppKey:     "app_1",
	}}
	restore := mockTicketStorages(&mockUserStorage{users: map[string]*storageModels.User{
		"u_1": {UserId: "u_1", Role: storageModels.UserRoleCustomerService},
		"u_2": {UserId: "u_2", Role: storageModels.UserRoleCustomerService},
	}}, &mockCustomerStorage{}, ticketStorage)
	defer restore()

	code, _ := TransferTicket(ticketTestContext("app_1", "u_1"), "t_1", &apiModels.TransferTicketReq{AssigneeId: "u_2"})
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("TransferTicket code = %d, want not login", code)
	}
	if ticketStorage.transferTicketId != "" {
		t.Fatalf("unexpected transfer of ticket %q", ticketStorage.transferTicketId)
	}
}

func TestTransferTicketRejectsUnknownAssignee(t *testing.T) {
	restore := mockTicketStorages(&mockUserStorage{users: map[string]*storageModels.User{
		"u_1": {UserId: "u_1", Role: storageModels.UserRoleAdmin},
	}}, &mockCustomerStorage{}, &mockTicketStorage{})
	defer restore()

	code, _ := TransferTicket(ticketTestContext("app_1", "u_1"), "t_1", &apiModels.TransferTicketReq{AssigneeId: "missing"})
	if code != errs.IMErrorCode_APP_USER_NOT_EXIST {
		t.Fatalf("TransferTicket code = %d, want user not exist", code)
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
	origSyncClaimTags := syncTicketGlobalConversationTagsForClaim
	origSyncTransferTags := syncTicketGlobalConversationTagsForTransfer
	newUserStorageForTicket = func() storageModels.IUserStorage {
		return userStorage
	}
	newCustomerStorageForTicket = func() storageModels.ICustomerStorage {
		return customerStorage
	}
	newTicketStorageForQuery = func() storageModels.ITicketStorage {
		return ticketStorage
	}
	syncTicketGlobalConversationTagsForClaim = func(string, string) errs.IMErrorCode {
		return errs.IMErrorCode_SUCCESS
	}
	syncTicketGlobalConversationTagsForTransfer = func(string, string) errs.IMErrorCode {
		return errs.IMErrorCode_SUCCESS
	}
	return func() {
		newUserStorageForTicket = origUserStorage
		newCustomerStorageForTicket = origCustomerStorage
		newTicketStorageForQuery = origTicketStorage
		syncTicketGlobalConversationTagsForClaim = origSyncClaimTags
		syncTicketGlobalConversationTagsForTransfer = origSyncTransferTags
	}
}

func mockTicketClaimDeps(
	userStorage *mockUserStorage,
	customerStorage *mockCustomerStorage,
	ticketStorage *mockTicketStorage,
	tagConvers func(*juggleimsdk.JuggleIMSdk, juggleimsdk.TagConversReq) (juggleimsdk.ApiCode, string, error),
) func() {
	restoreStorages := mockTicketStorages(userStorage, customerStorage, ticketStorage)
	origGetImSdk := getImSdkForTicketClaim
	origTagConvers := tagConversForClaim
	origSendTicketAssignedNtfMsg := sendTicketAssignedNtfMsgForClaim
	getImSdkForTicketClaim = func(appkey string) *juggleimsdk.JuggleIMSdk {
		return &juggleimsdk.JuggleIMSdk{}
	}
	if tagConvers != nil {
		tagConversForClaim = tagConvers
	}
	sendTicketAssignedNtfMsgForClaim = func(context.Context, *TicketAssignedNtfMsg) {}
	return func() {
		restoreStorages()
		getImSdkForTicketClaim = origGetImSdk
		tagConversForClaim = origTagConvers
		sendTicketAssignedNtfMsgForClaim = origSendTicketAssignedNtfMsg
	}
}

func assertTicketTagReq(t *testing.T, req juggleimsdk.TagConversReq) {
	t.Helper()
	if req.UserId == "" {
		t.Fatalf("TagConvers req missing user_id: %+v", req)
	}
	if req.Tag != ticketConversationTagMyTicket {
		t.Fatalf("TagConvers tag = %q, want my_ticket", req.Tag)
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
	user  *storageModels.User
	users map[string]*storageModels.User
	err   error
	// updatedImTokens 记录 UpdateImToken 的入参，供"补注册后回写 token"这类断言使用。
	updatedImTokens map[string]string
	// deleted 记录 Delete 的入参，供注册失败回滚的断言使用。
	deleted []string
}

func (s *mockUserStorage) Create(item storageModels.User) error {
	return nil
}

func (s *mockUserStorage) FindByAccount(appkey, account string) (*storageModels.User, error) {
	return nil, nil
}

func (s *mockUserStorage) FindByUserId(appkey, userId string) (*storageModels.User, error) {
	if s.users != nil {
		return s.users[userId], s.err
	}
	return s.user, s.err
}

func (s *mockUserStorage) FindByAccountWithAppkey(account, appkey string) (*storageModels.User, error) {
	return nil, nil
}

func (s *mockUserStorage) UpdateImToken(appkey, userId, imToken string) error {
	if s.updatedImTokens == nil {
		s.updatedImTokens = map[string]string{}
	}
	s.updatedImTokens[userId] = imToken
	return nil
}

func (s *mockUserStorage) QryByApp(appkey string, filter storageModels.UserListFilter, limit, offset int64) (*storageModels.UserListResult, error) {
	return &storageModels.UserListResult{}, nil
}

func (s *mockUserStorage) UpdateUser(appkey, userId string, updates storageModels.UserUpdate) error {
	return nil
}

func (s *mockUserStorage) Delete(appkey, userId string) error {
	s.deleted = append(s.deleted, userId)
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

type mockTicketStorage struct {
	called           string
	appkey           string
	assigneeId       string
	status           *storageModels.TicketStatus
	isHumanTakenOver *bool
	limit            int64
	offset           int64
	tickets          []*storageModels.Ticket
	err              error
	customerId       string
	startId          int64
	customerLimit    int64

	claimAppkey      string
	claimTicketId    string
	claimAssigneeId  string
	claimTicket      *storageModels.Ticket
	revertAppkey     string
	revertTicketId   string
	revertAssigneeId string
	findTicket       *storageModels.Ticket
	findAppkey       string
	findTicketId     string
	transferAppkey   string
	transferTicketId string
	transferOldId    string
	transferNewId    string
	transferTicket   *storageModels.Ticket

	markAppkey   string
	markTicketId string
	markBy       string
	markAt       int64
	markErr      error
}

type mockInboxMemberStorage struct {
	appkey  string
	inboxId string
	startId int64
	limit   int64
	members []*storageModels.InboxMember
	err     error
}

func (s *mockInboxMemberStorage) Create(item storageModels.InboxMember) error   { return nil }
func (s *mockInboxMemberStorage) Upsert(item storageModels.InboxMember) error   { return nil }
func (s *mockInboxMemberStorage) Delete(appkey, inboxId, memberId string) error { return nil }
func (s *mockInboxMemberStorage) Find(appkey, inboxId, memberId string) (*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *mockInboxMemberStorage) QryByInbox(appkey, inboxId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	s.appkey = appkey
	s.inboxId = inboxId
	s.startId = startId
	s.limit = limit
	return s.members, s.err
}
func (s *mockInboxMemberStorage) QryByMember(appkey, memberId string, startId, limit int64) ([]*storageModels.InboxMember, error) {
	return nil, nil
}
func (s *mockInboxMemberStorage) CountByInboxes(appkey string, inboxIds []string) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (s *mockInboxMemberStorage) ReplaceByInbox(appkey, inboxId string, memberIds []string) error {
	return nil
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
	s.findAppkey = appkey
	s.findTicketId = ticketId
	return s.findTicket, s.err
}

func (s *mockTicketStorage) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	s.called = "all"
	s.appkey = appkey
	s.status = status
	s.limit = limit
	s.offset = offset
	s.isHumanTakenOver = isHumanTakenOver
	return s.tickets, s.err
}

func (s *mockTicketStorage) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	s.called = "visible"
	s.appkey = appkey
	s.assigneeId = assigneeId
	s.status = status
	s.limit = limit
	s.offset = offset
	s.isHumanTakenOver = isHumanTakenOver
	return s.tickets, s.err
}

func (s *mockTicketStorage) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	s.appkey = appkey
	s.customerId = customerId
	s.startId = startId
	s.customerLimit = limit
	s.isHumanTakenOver = isHumanTakenOver
	return s.tickets, s.err
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

func (s *mockTicketStorage) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	s.transferAppkey = appkey
	s.transferTicketId = ticketId
	s.transferOldId = oldAssigneeId
	s.transferNewId = newAssigneeId
	return s.transferTicket, s.err
}

func (s *mockTicketStorage) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	s.markAppkey = appkey
	s.markTicketId = ticketId
	s.markBy = by
	s.markAt = atMs
	return s.markErr
}
