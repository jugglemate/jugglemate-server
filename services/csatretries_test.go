package services

import (
	"context"
	"errors"
	"testing"

	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// mockCsatRetryTicket 重写 MarkCsatNotifiedOnce 与 QryTicketsNeedingCsatNotification
// 用于补发路径单测。把所有 ITicketStorage 调用路由到内部的 counters/ticket。
type mockCsatRetryTicket struct {
	ticket             *storageModels.Ticket
	pendingReturn      []storageModels.Ticket
	markErr            error
	markFirstWrite     bool
	markedOnce         bool
}

func (m *mockCsatRetryTicket) MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error) {
	if m.markErr != nil {
		return false, m.markErr
	}
	if m.markedOnce {
		return false, nil
	}
	m.markedOnce = true
	m.markFirstWrite = true
	return true, nil
}
func (m *mockCsatRetryTicket) QryTicketsNeedingCsatNotification(limit int64) ([]*storageModels.Ticket, error) {
	out := make([]*storageModels.Ticket, 0, len(m.pendingReturn))
	for i := range m.pendingReturn {
		t := m.pendingReturn[i]
		out = append(out, &t)
	}
	return out, nil
}

// 其余 ITicketStorage 方法在补发路径上不需要（runCsatRetryPass 不直接用），
// 这里只要保证编译能过即可；它实现的是 ITicketStorage 接口，所以必须齐全。
func (m *mockCsatRetryTicket) Create(item storageModels.Ticket) error { return nil }
func (m *mockCsatRetryTicket) Update(item storageModels.Ticket) error { return nil }
func (m *mockCsatRetryTicket) Upsert(item storageModels.Ticket) error { return nil }
func (m *mockCsatRetryTicket) Delete(appkey, ticketId string) error  { return nil }
func (m *mockCsatRetryTicket) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	if m.ticket != nil && m.ticket.AppKey == appkey && m.ticket.TicketId == ticketId {
		cp := *m.ticket
		return &cp, nil
	}
	return nil, nil
}
func (m *mockCsatRetryTicket) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}
func (m *mockCsatRetryTicket) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return nil
}
func (m *mockCsatRetryTicket) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (m *mockCsatRetryTicket) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	return nil
}
func (m *mockCsatRetryTicket) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (m *mockCsatRetryTicket) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	return false, nil
}

func TestRunCsatRetryPassHandlesEmptyPending(t *testing.T) {
	// 没有待补发工单时不应通知 IM，也不应报 panic。
	m := &mockCsatRetryTicket{} // pendingReturn 是 nil（空）
	oldSt := newTicketStorageForCsatNotify
	oldSender := csatIMSender
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	called := 0
	csatIMSender = func(ctx context.Context, appKey, botUserID, ticketId, msgType string, payload interface{}) error {
		called++
		return nil
	}
	defer func() {
		newTicketStorageForCsatNotify = oldSt
		csatIMSender = oldSender
	}()

	runCsatRetryPass(context.Background(), 1785397260207)

	if called != 0 {
		t.Fatalf("空候选列表不该调 IM，实际 %d", called)
	}
}

func TestRunCsatRetryPassSendsAndMarks(t *testing.T) {
	// NotifyCsatInvitation 内部会调 FindByTicketId + GetInboxAgent 找 bot_user_id。
	// 给 mock 装配一个 ticket，并通过 getInboxAgentFn 直接返回一个含 bot_user_id 的 detail。
	m := &mockCsatRetryTicket{
		ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_old",
			InboxId: "inbox_1", Status: storageModels.TicketStatusClosed,
		},
		pendingReturn: []storageModels.Ticket{
			{AppKey: "app_1", TicketId: "ticket_old", Status: storageModels.TicketStatusClosed, InboxId: "inbox_1"},
		},
	}
	oldSt := newTicketStorageForCsatNotify
	oldSender := csatIMSender
	oldGetInboxAgent := getInboxAgentFn
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	getInboxAgentFn = func(ctx context.Context, appKey, inboxID string) (*InboxAgentDetail, error) {
		return &InboxAgentDetail{BotUserID: "bot_xxx"}, nil
	}
	called := 0
	csatIMSender = func(ctx context.Context, appKey, botUserID, ticketId, msgType string, payload interface{}) error {
		called++
		return nil
	}
	defer func() {
		newTicketStorageForCsatNotify = oldSt
		csatIMSender = oldSender
		getInboxAgentFn = oldGetInboxAgent
	}()

	runCsatRetryPass(context.Background(), 1785397260207)

	if called != 1 {
		t.Fatalf("csatIMSender 应被调用 1 次，实际 %d", called)
	}
	if !m.markFirstWrite {
		t.Fatalf("MarkCsatNotifiedOnce 应被调用一次写入字段（FirstWrite=true）")
	}
}

func TestRunCsatRetryPassNotifyFailureDoesNotMark(t *testing.T) {
	m := &mockCsatRetryTicket{
		ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_old",
			InboxId: "inbox_1", Status: storageModels.TicketStatusClosed,
		},
		pendingReturn: []storageModels.Ticket{
			{AppKey: "app_1", TicketId: "ticket_old", Status: storageModels.TicketStatusClosed, InboxId: "inbox_1"},
		},
	}
	oldSt := newTicketStorageForCsatNotify
	oldSender := csatIMSender
	oldGetInboxAgent := getInboxAgentFn
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	getInboxAgentFn = func(ctx context.Context, appKey, inboxID string) (*InboxAgentDetail, error) {
		return &InboxAgentDetail{BotUserID: "bot_xxx"}, nil
	}
	csatIMSender = func(ctx context.Context, appKey, botUserID, ticketId, msgType string, payload interface{}) error {
		return errors.New("im down")
	}
	defer func() {
		newTicketStorageForCsatNotify = oldSt
		csatIMSender = oldSender
		getInboxAgentFn = oldGetInboxAgent
	}()

	runCsatRetryPass(context.Background(), 1785397260207)

	if m.markFirstWrite {
		t.Fatalf("Notify 失败时不应调用 MarkCsatNotifiedOnce（避免标错已通知）")
	}
}
