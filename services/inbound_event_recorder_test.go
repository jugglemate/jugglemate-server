package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// TestClassifySenderRoleCases 锁死角色判定规则。
func TestClassifySenderRoleCases(t *testing.T) {
	tk := &storageModels.Ticket{
		AppKey:     "app_1",
		TicketId:   "ticket_1",
		SourceId:   "customer_1",
		AssigneeId: "u_seat_1",
	}
	cases := []struct {
		name       string
		ticket     *storageModels.Ticket
		sender     string
		wantRole   storageModels.TicketEventOperator
	}{
		{"customer matches source", tk, "customer_1", storageModels.TicketEventOperatorCustomer},
		{"assignee matches user", tk, "u_seat_1", storageModels.TicketEventOperatorUser},
		{"bot is none of above", tk, "bot-xxx", storageModels.TicketEventOperatorBot},
		{"empty sender falls back to bot", tk, "", storageModels.TicketEventOperatorBot},
		{"nil ticket is system", nil, "anything", storageModels.TicketEventOperatorSystem},
		{"whitespace sender falls back to bot", tk, "  ", storageModels.TicketEventOperatorBot},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifySenderRole(c.ticket, c.sender); got != c.wantRole {
				t.Fatalf("ClassifySenderRole(%v, %q) = %q, want %q",
					c.ticket, c.sender, got, c.wantRole)
			}
		})
	}
}

// mockFailingMessageStorage 总是在 UpsertByMsgId 返错；
// 用于验证 UnreliableOp 包装不会让 caller panic。
type mockFailingMessageStorage struct {
	upsertErr error
}

func (m *mockFailingMessageStorage) UpsertByMsgId(_ storageModels.TicketMessage) error {
	return m.upsertErr
}
func (m *mockFailingMessageStorage) QryLastByTicket(_, _ string) (*storageModels.TicketMessage, error) {
	return nil, nil
}

// TestInboundEventRecorder_BestEffortOnUpsertFailureUpsert 失败时 UnreliableOp
// 带错误返回，调用方调 Discard() 显式吞掉。
func TestInboundEventRecorder_BestEffortOnUpsertFailureUpsert(t *testing.T) {
	rec := NewInboundEventRecorder(
		&fakeTicketStorage{ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_1",
			SourceId: "customer_1", AssigneeId: "u_seat_1",
		}},
		&mockFailingMessageStorage{upsertErr: errors.New("db down")},
		func(_, _ string, _ int64) error { return nil },
	)

	op := rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "u_seat_1", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "msg_x", MsgTime: 1785000000000,
	})
	if op.Err() == nil {
		t.Fatalf("expected op.Err() non-nil on upsert failure")
	}
	if !strings.Contains(op.Err().Error(), "db down") {
		t.Fatalf("expected err to wrap db down, got %v", op.Err())
	}
	// Discard 不应 panic
	op.Discard()
}

// TestInboundEventRecorder_NoAdvanceOnAssigneeNoMatch 验证 last_user_msg_at
// 仅在 sender==AssigneeId 时推进；其他全部跳过。
func TestInboundEventRecorder_NoAdvanceOnAssigneeNoMatch(t *testing.T) {
	advanced := 0
	rec := NewInboundEventRecorder(
		&fakeTicketStorage{ticket: &storageModels.Ticket{
			AppKey: "app_1", TicketId: "ticket_1",
			SourceId: "customer_1", AssigneeId: "u_seat_1",
		}},
		&mockMessageStorage{},
		func(_, _ string, _ int64) error {
			advanced++
			return nil
		},
	)
	// 客户消息：客户发，sender == source 但 != assignee
	rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "customer_1", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "m1", MsgTime: 1785000000000,
	}).Discard()
	// Bot 消息
	rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "bot-xxx", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "m2", MsgTime: 1785000000001,
	}).Discard()
	if advanced != 0 {
		t.Fatalf("last_user_msg_at 应不被推进，实际 %d 次", advanced)
	}
	// 坐席消息：应推进
	rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "u_seat_1", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "m3", MsgTime: 1785000000002,
	}).Discard()
	if advanced != 1 {
		t.Fatalf("last_user_msg_at 应被推进 1 次，实际 %d", advanced)
	}
}

// TestInboundEventRecorder_TicketNotFound 验证 ticket==nil 早 return（不写幽灵）。
func TestInboundEventRecorder_TicketNotFound(t *testing.T) {
	msg := &mockMessageStorage{}
	rec := NewInboundEventRecorder(
		&fakeTicketStorage{ticket: nil}, // FindByTicketId 返 nil
		msg,
		func(_, _ string, _ int64) error { return errors.New("should not advance") },
	)
	op := rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "u_seat_1", Receiver: "ticket_missing",
		MsgType: "jg:text", MsgID: "m1", MsgTime: 1785000000000,
	})
	if op.Err() != nil {
		t.Fatalf("ticket==nil 不应有 err：%v", op.Err())
	}
	if msg.last.MsgId != "" {
		t.Fatalf("ticket==nil 不应留痕，实际写入了：%+v", msg.last)
	}
}

// TestInboundEventRecorder_EmptyMsgIDSkips 验证 msg_id 或 time 为 0 时跳过。
func TestInboundEventRecorder_EmptyMsgIDSkips(t *testing.T) {
	msg := &mockMessageStorage{}
	rec := NewInboundEventRecorder(
		&fakeTicketStorage{ticket: nil},
		msg,
		func(_, _ string, _ int64) error { return errors.New("should not advance") },
	)
	rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "u_seat_1", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "", MsgTime: 1785000000000,
	}).Discard()
	if msg.last.MsgId != "" {
		t.Fatalf("msg_id 为空时不应留痕")
	}
	rec.RecordOnce(context.Background(), "app_1", WebhookMessagePayload{
		Sender: "u_seat_1", Receiver: "ticket_1",
		MsgType: "jg:text", MsgID: "m1", MsgTime: 0,
	}).Discard()
	if msg.last.MsgId != "" {
		t.Fatalf("msg_time=0 时不应留痕")
	}
}

// TestGetInboundEventRecorderDefault 验证懒加载 default recorder 行为。
func TestGetInboundEventRecorderDefault(t *testing.T) {
	// 用 SetInboundEventRecorderForTest 注入一个 sentinel；如果之后被默认 lazy 创建覆盖，
	// GetInboundEventRecorder() 会换成新对象。
	old := &InboundEventRecorder{} // 零值足够
	SetInboundEventRecorderForTest(old)
	defer SetInboundEventRecorderForTest(nil)

	if GetInboundEventRecorder() != old {
		t.Fatalf("GetInboundEventRecorder 应该返回已注入的 sentinel，没返回默认实例")
	}

	// 重置后 Get 应当返回一个新的（懒加载）实例 —— 不应复用已 nil 化的 holder
	SetInboundEventRecorderForTest(nil)
	got := GetInboundEventRecorder()
	if got == nil {
		t.Fatalf("GetInboundEventRecorder 懒加载失败")
	}
}
