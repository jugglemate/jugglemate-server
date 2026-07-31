package services

import (
	"context"
	"testing"

	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// mockCsatRetryTicket 重写 MarkCsatNotifiedOnce 与 QryTicketsNeedingCsatNotification
// 用于补发路径单测。把所有 ITicketStorage 调用路由到内部的 counters/ticket。
type mockCsatRetryTicket struct {
	ticket        *storageModels.Ticket
	pendingReturn []storageModels.Ticket
	markErr       error
	markFirstWrite bool
	markedOnce    bool
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

// 其余 ITicketStorage 方法在补发路径上不需要，仅保证编译通过。
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
func (m *mockCsatRetryTicket) UpdateLastCustomerMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (m *mockCsatRetryTicket) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	return false, nil
}

func TestRunCsatRetryPassHandlesEmptyPending(t *testing.T) {
	// 没有待补发工单时不应通知 IM，也不应报 panic。
	m := &mockCsatRetryTicket{} // pendingReturn 是 nil（空）
	oldSt := newTicketStorageForCsatNotify
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	defer func() { newTicketStorageForCsatNotify = oldSt }()

	runCsatRetryPass(context.Background(), 1785397260207)
	// 无 pending 工单则 MarkCsatNotifiedOnce 不会被调用，测试通过即表示无 panic。
}

func TestRunCsatRetryPassSendsAndMarks(t *testing.T) {
	// NotifyCsatInvitation 内部查 ticket.FindByTicketId 获取 AssigneeId；
	// 因此 mock 的 ticket 必须有 AssigneeId。
	m := &mockCsatRetryTicket{
		ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_old",
			InboxId: "inbox_1", AssigneeId: "agent_123",
			Status: storageModels.TicketStatusClosed,
		},
		pendingReturn: []storageModels.Ticket{
			{AppKey: "app_1", TicketId: "ticket_old", Status: storageModels.TicketStatusClosed, InboxId: "inbox_1", AssigneeId: "agent_123"},
		},
	}
	oldSt := newTicketStorageForCsatNotify
	oldSender := sendTicketCsatNtfMsgFn
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	called := false
	sendTicketCsatNtfMsgFn = func(appkey, ticketId, senderId string, payload CsatInvitationPayload) {
		called = true
	}
	defer func() {
		newTicketStorageForCsatNotify = oldSt
		sendTicketCsatNtfMsgFn = oldSender
	}()

	runCsatRetryPass(context.Background(), 1785397260207)

	if !called {
		t.Fatalf("SendTicketCsatNtfMsg 应被调用 1 次")
	}
	if !m.markFirstWrite {
		t.Fatalf("MarkCsatNotifiedOnce 应被调用一次写入字段（FirstWrite=true）")
	}
}

func TestRunCsatRetryPassNotifyFailureDoesNotMark(t *testing.T) {
	// NotifyCsatInvitation 失败场景：工单缺失 AssigneeId，返回错误，
	// 此时不应调用 MarkCsatNotifiedOnce。
	m := &mockCsatRetryTicket{
		ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_old",
			InboxId: "inbox_1", AssigneeId: "", // 无坐席
			Status: storageModels.TicketStatusClosed,
		},
		pendingReturn: []storageModels.Ticket{
			{AppKey: "app_1", TicketId: "ticket_old", Status: storageModels.TicketStatusClosed, InboxId: "inbox_1", AssigneeId: ""},
		},
	}
	oldSt := newTicketStorageForCsatNotify
	newTicketStorageForCsatNotify = func() storageModels.ITicketStorage { return m }
	defer func() { newTicketStorageForCsatNotify = oldSt }()

	runCsatRetryPass(context.Background(), 1785397260207)

	if m.markFirstWrite {
		t.Fatalf("Notify 失败时不应调用 MarkCsatNotifiedOnce（避免标错已通知）")
	}
}
