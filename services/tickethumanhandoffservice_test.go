package services

import (
	"context"
	"errors"
	"testing"

	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

type mockTicketStorageForHandoff struct {
	markAppKey string
	markTicket string
	markBy     string
	markAt     int64
	err        error
}

func (s *mockTicketStorageForHandoff) Create(item storageModels.Ticket) error { return nil }
func (s *mockTicketStorageForHandoff) Update(item storageModels.Ticket) error { return nil }
func (s *mockTicketStorageForHandoff) Upsert(item storageModels.Ticket) error { return nil }
func (s *mockTicketStorageForHandoff) Delete(appkey, ticketId string) error   { return nil }
func (s *mockTicketStorageForHandoff) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}
func (s *mockTicketStorageForHandoff) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return nil
}
func (s *mockTicketStorageForHandoff) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (s *mockTicketStorageForHandoff) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	s.markAppKey = appkey
	s.markTicket = ticketId
	s.markBy = by
	s.markAt = atMs
	return s.err
}

func (s *mockTicketStorageForHandoff) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}

func (s *mockTicketStorageForHandoff) UpdateLastCustomerMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}

func (s *mockTicketStorageForHandoff) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	return false, nil
}
func (s *mockTicketStorageForHandoff) MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error) {
	return false, nil
}
func (s *mockTicketStorageForHandoff) QryTicketsNeedingCsatNotification(limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

type mockTicketEventStorageForHandoff struct {
	created    []storageModels.TicketEvent
	err        error
	listResult []*storageModels.TicketEvent
	lastQuery  struct {
		appkey  string
		ticket  string
		limit   int64
		offset  int64
	}
}

func (s *mockTicketEventStorageForHandoff) Create(event storageModels.TicketEvent) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, event)
	return nil
}
func (s *mockTicketEventStorageForHandoff) QryByTicket(appkey, ticketId string, limit, offset int64) ([]*storageModels.TicketEvent, error) {
	s.lastQuery.appkey = appkey
	s.lastQuery.ticket = ticketId
	s.lastQuery.limit = limit
	s.lastQuery.offset = offset
	return s.listResult, nil
}

func TestMarkHumanTakeoverPersistsFactAndEvent(t *testing.T) {
	ticketStore := &mockTicketStorageForHandoff{}
	eventStore := &mockTicketEventStorageForHandoff{}
	origTicket := newTicketStorageForHumanHandoff
	origEvent := newTicketEventStorageForHandoff
	newTicketStorageForHumanHandoff = func() storageModels.ITicketStorage { return ticketStore }
	newTicketEventStorageForHandoff = func() storageModels.ITicketEventStorage { return eventStore }
	defer func() {
		newTicketStorageForHumanHandoff = origTicket
		newTicketEventStorageForHandoff = origEvent
	}()

	if err := MarkHumanTakeover(context.Background(), "app_1", "ticket_1", "customer_1", "customer"); err != nil {
		t.Fatalf("MarkHumanTakeover = %v", err)
	}
	if ticketStore.markAppKey != "app_1" || ticketStore.markTicket != "ticket_1" || ticketStore.markBy != "customer_1" {
		t.Fatalf("mark args = %+v", ticketStore)
	}
	if ticketStore.markAt <= 0 {
		t.Fatalf("mark at = %d, want >0", ticketStore.markAt)
	}
	if len(eventStore.created) != 1 {
		t.Fatalf("event created = %d, want 1", len(eventStore.created))
	}
	created := eventStore.created[0]
	if created.EventType != storageModels.TicketEventTypeHumanTakeover {
		t.Fatalf("event type = %q, want human_takeover", created.EventType)
	}
	if created.OperatorID != "customer_1" || created.OperatorType != storageModels.TicketEventOperatorCustomer {
		t.Fatalf("event operator = %+v", created)
	}
	if created.CreatedTime != ticketStore.markAt {
		t.Fatalf("event time = %d, want mark time %d", created.CreatedTime, ticketStore.markAt)
	}
}

func TestMarkHumanTakeoverRejectsInvalidOperatorType(t *testing.T) {
	if err := MarkHumanTakeover(context.Background(), "app_1", "ticket_1", "customer_1", "robot"); err == nil {
		t.Fatalf("MarkHumanTakeover expected error for operatorType=robot")
	}
}

func TestMarkHumanTakeoverRejectsEmptyOperatorID(t *testing.T) {
	if err := MarkHumanTakeover(context.Background(), "app_1", "ticket_1", " ", "customer"); err == nil {
		t.Fatalf("MarkHumanTakeover expected error for empty operatorId")
	}
}

func TestMarkHumanTakeoverRejectsMissingAppKey(t *testing.T) {
	if err := MarkHumanTakeover(context.Background(), " ", "ticket_1", "customer_1", "customer"); err == nil {
		t.Fatalf("MarkHumanTakeover expected error for empty appKey")
	}
}

func TestMarkHumanTakeoverReturnsErrorWhenStorageFails(t *testing.T) {
	ticketStore := &mockTicketStorageForHandoff{err: errors.New("db down")}
	origTicket := newTicketStorageForHumanHandoff
	newTicketStorageForHumanHandoff = func() storageModels.ITicketStorage { return ticketStore }
	defer func() { newTicketStorageForHumanHandoff = origTicket }()

	if err := MarkHumanTakeover(context.Background(), "app_1", "ticket_1", "customer_1", "customer"); err == nil {
		t.Fatalf("MarkHumanTakeover expected error when storage fails")
	}
}

func TestMarkHumanTakeoverReturnsErrorWhenEventStorageFails(t *testing.T) {
	ticketStore := &mockTicketStorageForHandoff{}
	eventStore := &mockTicketEventStorageForHandoff{err: errors.New("event down")}
	origTicket := newTicketStorageForHumanHandoff
	origEvent := newTicketEventStorageForHandoff
	newTicketStorageForHumanHandoff = func() storageModels.ITicketStorage { return ticketStore }
	newTicketEventStorageForHandoff = func() storageModels.ITicketEventStorage { return eventStore }
	defer func() {
		newTicketStorageForHumanHandoff = origTicket
		newTicketEventStorageForHandoff = origEvent
	}()

	if err := MarkHumanTakeover(context.Background(), "app_1", "ticket_1", "customer_1", "customer"); err == nil {
		t.Fatalf("MarkHumanTakeover expected error when event storage fails")
	}
}

func TestQryTicketEventsNormalizesArgs(t *testing.T) {
	eventStore := &mockTicketEventStorageForHandoff{
		listResult: []*storageModels.TicketEvent{
			{ID: 1, AppKey: "app_1", TicketId: "ticket_1", EventType: storageModels.TicketEventTypeHumanTakeover},
		},
	}
	origEvent := newTicketEventStorageForHandoff
	newTicketEventStorageForHandoff = func() storageModels.ITicketEventStorage { return eventStore }
	defer func() { newTicketEventStorageForHandoff = origEvent }()

	items, err := QryTicketEvents(context.Background(), "app_1", "ticket_1", 0, -5)
	if err != nil {
		t.Fatalf("QryTicketEvents = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if eventStore.lastQuery.limit != 50 || eventStore.lastQuery.offset != 0 {
		t.Fatalf("last query = %+v", eventStore.lastQuery)
	}
}
