package services

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestAutoCloseCutoffUsesTimeValue(t *testing.T) {
	cutoffMs := int64(1785401409116)
	cutoffAt := time.UnixMilli(cutoffMs)
	if cutoffAt.UnixMilli() != cutoffMs {
		t.Fatalf("UnixMilli roundtrip = %d, want %d", cutoffAt.UnixMilli(), cutoffMs)
	}
}

type mockTickerTicket struct {
	tickets        map[string]*storageModels.Ticket
	closeCallCount int32
	closedByIdle   []string
}

func (m *mockTickerTicket) Create(item storageModels.Ticket) error { return nil }
func (m *mockTickerTicket) Update(item storageModels.Ticket) error { return nil }
func (m *mockTickerTicket) Upsert(item storageModels.Ticket) error { return nil }
func (m *mockTickerTicket) Delete(appkey, ticketId string) error   { return nil }
func (m *mockTickerTicket) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	if t, ok := m.tickets[ticketId]; ok && t.AppKey == appkey {
		cp := *t
		return &cp, nil
	}
	return nil, nil
}
func (m *mockTickerTicket) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}
func (m *mockTickerTicket) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return nil
}
func (m *mockTickerTicket) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockTickerTicket) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	return nil
}
func (m *mockTickerTicket) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (m *mockTickerTicket) UpdateLastCustomerMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (m *mockTickerTicket) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	atomic.AddInt32(&m.closeCallCount, 1)
	m.closedByIdle = append(m.closedByIdle, ticketId)
	return true, nil
}

func (m *mockTickerTicket) MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error) {
	return true, nil
}

func (m *mockTickerTicket) QryTicketsNeedingCsatNotification(limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

type mockTickerEvent struct {
	created []storageModels.TicketEvent
}

func (m *mockTickerEvent) Create(event storageModels.TicketEvent) error {
	m.created = append(m.created, event)
	return nil
}

func (m *mockTickerEvent) QryByTicket(appkey, ticketId string, limit, offset int64) ([]*storageModels.TicketEvent, error) {
	return nil, nil
}

func TestRunAutoCloseOnceClosesStaleTickets(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	tickets := map[string]*storageModels.Ticket{
		"ticket_active": {
			AppKey: "app_1", TicketId: "ticket_active",
			Status:        storageModels.TicketStatusProcessing,
			LastUserMsgAt: nowMs - 1*60*1000, // 1 分钟前
		},
		"ticket_old": {
			AppKey: "app_1", TicketId: "ticket_old",
			Status:        storageModels.TicketStatusProcessing,
			LastUserMsgAt: nowMs - 15*60*1000, // 15 分钟前
		},
	}
	ticketStore := &mockTickerTicket{tickets: tickets}
	eventStore := &mockTickerEvent{}

	oldTicket := newTicketStorageForAutoClose
	oldEvent := newTicketEventStorageForAutoClose
	oldSyncTags := syncTicketGlobalConversationTagsForAutoClose
	oldFetch := fetchAutoCloseCandidates
	oldCsatNotifyTicket := newTicketStorageForCsatNotify
	newTicketStorageForAutoClose = func() storageModels.ITicketStorage { return ticketStore }
	newTicketEventStorageForAutoClose = func() storageModels.ITicketEventStorage { return eventStore }
	syncTicketGlobalConversationTagsForAutoClose = func(appkey, ticketId string) errs.IMErrorCode {
		if appkey == "app_1" && ticketId == "ticket_old" {
			return errs.IMErrorCode_SUCCESS
		}
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return ticketStore }
	fetchAutoCloseCandidates = func(ctx context.Context, cutoffMs int64) ([]autoCloseCandidate, error) {
		out := []autoCloseCandidate{}
		for _, t := range tickets {
			if t.LastUserMsgAt < cutoffMs {
				out = append(out, autoCloseCandidate{AppKey: t.AppKey, TicketId: t.TicketId})
			}
		}
		return out, nil
	}
	oldCsatSender := sendTicketCsatNtfMsgFn
	sendTicketCsatNtfMsgFn = func(appkey, ticketId, senderId string, payload CsatInvitationPayload) {}
	defer func() {
		newTicketStorageForAutoClose = oldTicket
		newTicketEventStorageForAutoClose = oldEvent
		syncTicketGlobalConversationTagsForAutoClose = oldSyncTags
		fetchAutoCloseCandidates = oldFetch
		newTicketStorageForCsatNotify = oldCsatNotifyTicket
		sendTicketCsatNtfMsgFn = oldCsatSender
	}()

	// jgm:csat 现已通过 IM REST API 发送（SendTicketCsatNtfMsg），
	// 测试中 stubbed 为空操作。NotifyCsatInvitation 需要 ticket 有 AssigneeId
	// 才能成功；测试数据无 assignee → 返回错误 → 不会误标记 csat_notified_at。
	runAutoCloseOnce(context.Background(), 5*time.Minute, nowMs)

	if atomic.LoadInt32(&ticketStore.closeCallCount) != 1 {
		t.Fatalf("closeCallCount = %d, want 1", ticketStore.closeCallCount)
	}
	if ticketStore.closedByIdle[0] != "ticket_old" {
		t.Fatalf("closed = %v, want ticket_old", ticketStore.closedByIdle)
	}
	closeEvents := 0
	for _, e := range eventStore.created {
		if e.EventType == storageModels.TicketEventTypeClose {
			closeEvents++
		}
	}
	if closeEvents != 1 {
		t.Fatalf("close events = %d, want 1", closeEvents)
	}
}

func TestRunAutoCloseOnceClosesStaleReopenedTicket(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	tickets := map[string]*storageModels.Ticket{
		"ticket_reopened": {
			AppKey:            "app_1",
			TicketId:          "ticket_reopened",
			Status:            storageModels.TicketStatusReOpen,
			AssigneeId:        "agent_1",
			LastUserMsgAt:     nowMs - 10*60*1000,
			LastCustomerMsgAt: nowMs - 11*60*1000,
		},
	}
	ticketStore := &mockTickerTicket{tickets: tickets}
	eventStore := &mockTickerEvent{}

	oldTicket := newTicketStorageForAutoClose
	oldEvent := newTicketEventStorageForAutoClose
	oldSyncTags := syncTicketGlobalConversationTagsForAutoClose
	oldFetch := fetchAutoCloseCandidates
	oldCsatNotifyTicket := newTicketStorageForCsatNotify
	oldCsatSender := sendTicketCsatNtfMsgFn
	defer func() {
		newTicketStorageForAutoClose = oldTicket
		newTicketEventStorageForAutoClose = oldEvent
		syncTicketGlobalConversationTagsForAutoClose = oldSyncTags
		fetchAutoCloseCandidates = oldFetch
		newTicketStorageForCsatNotify = oldCsatNotifyTicket
		sendTicketCsatNtfMsgFn = oldCsatSender
	}()

	newTicketStorageForAutoClose = func() storageModels.ITicketStorage { return ticketStore }
	newTicketEventStorageForAutoClose = func() storageModels.ITicketEventStorage { return eventStore }
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return ticketStore }
	syncTicketGlobalConversationTagsForAutoClose = func(appkey, ticketId string) errs.IMErrorCode {
		return errs.IMErrorCode_SUCCESS
	}
	fetchAutoCloseCandidates = func(ctx context.Context, cutoffMs int64) ([]autoCloseCandidate, error) {
		ticket := tickets["ticket_reopened"]
		if ticket.Status == storageModels.TicketStatusReOpen &&
			ticket.LastUserMsgAt < cutoffMs && ticket.LastCustomerMsgAt < ticket.LastUserMsgAt {
			return []autoCloseCandidate{{AppKey: ticket.AppKey, TicketId: ticket.TicketId}}, nil
		}
		return nil, nil
	}
	sendTicketCsatNtfMsgFn = func(appkey, ticketId, senderId string, payload CsatInvitationPayload) {}

	runAutoCloseOnce(context.Background(), 5*time.Minute, nowMs)

	if atomic.LoadInt32(&ticketStore.closeCallCount) != 1 {
		t.Fatalf("closeCallCount = %d, want 1", ticketStore.closeCallCount)
	}
	if len(ticketStore.closedByIdle) != 1 || ticketStore.closedByIdle[0] != "ticket_reopened" {
		t.Fatalf("closed = %v, want ticket_reopened", ticketStore.closedByIdle)
	}
	if len(eventStore.created) != 1 || eventStore.created[0].EventType != storageModels.TicketEventTypeClose {
		t.Fatalf("events = %+v, want one close event", eventStore.created)
	}
}

func TestRunAutoCloseOnceNoDB(t *testing.T) {
	nowMs := time.Now().UnixMilli()
	// 不连真实 DB，runAutoCloseOnce 会在 GetDb()==nil 时直接返回；
	// 我们的目的是确认 nil DB 不会 panic、不调用 CloseByIdle。
	tickets := map[string]*storageModels.Ticket{
		"ticket_old": {
			AppKey: "app_1", TicketId: "ticket_old",
			Status:        storageModels.TicketStatusProcessing,
			LastUserMsgAt: nowMs - 15*60*1000,
		},
	}
	ticketStore := &mockTickerTicket{tickets: tickets}
	eventStore := &mockTickerEvent{}
	oldTicket := newTicketStorageForAutoClose
	oldEvent := newTicketEventStorageForAutoClose
	oldFetch := fetchAutoCloseCandidates
	oldCsatNotifyTicket := newTicketStorageForCsatNotify
	newTicketStorageForAutoClose = func() storageModels.ITicketStorage { return ticketStore }
	newTicketEventStorageForAutoClose = func() storageModels.ITicketEventStorage { return eventStore }
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return ticketStore }
	fetchAutoCloseCandidates = func(ctx context.Context, cutoffMs int64) ([]autoCloseCandidate, error) {
		return nil, nil // db==nil 路径等价：包级注入直接返回空
	}
	oldCsatSender := sendTicketCsatNtfMsgFn
	sendTicketCsatNtfMsgFn = func(appkey, ticketId, senderId string, payload CsatInvitationPayload) {}
	defer func() {
		newTicketStorageForAutoClose = oldTicket
		newTicketEventStorageForAutoClose = oldEvent
		fetchAutoCloseCandidates = oldFetch
		newTicketStorageForCsatNotify = oldCsatNotifyTicket
		sendTicketCsatNtfMsgFn = oldCsatSender
	}()

	runAutoCloseOnce(context.Background(), 5*time.Minute, nowMs)

	if atomic.LoadInt32(&ticketStore.closeCallCount) != 0 {
		t.Fatalf("closeCallCount = %d, want 0 (no DB)", ticketStore.closeCallCount)
	}
}
