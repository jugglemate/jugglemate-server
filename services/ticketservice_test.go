package services

import (
	"context"
	"testing"

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
	called     string
	appkey     string
	assigneeId string
	status     *storageModels.TicketStatus
	limit      int64
	offset     int64
	tickets    []*storageModels.Ticket
	err        error
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

func (s *mockTicketStorage) QryByChannel(appkey, channelId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func (s *mockTicketStorage) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}
