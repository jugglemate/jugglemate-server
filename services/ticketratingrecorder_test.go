package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/juggleim/jugglemate-server/storages"
	"github.com/juggleim/jugglemate-server/storages/dbs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

type mockRatingStorage struct {
	createErr error
	created   []storageModels.TicketRating
}

func (m *mockRatingStorage) Create(item storageModels.TicketRating) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, item)
	return nil
}

func (m *mockRatingStorage) FindByTicketAndCustomer(appkey, ticketId, customerId string) (*storageModels.TicketRating, error) {
	for i := range m.created {
		if m.created[i].AppKey == appkey && m.created[i].TicketId == ticketId && m.created[i].CustomerId == customerId {
			r := m.created[i]
			return &r, nil
		}
	}
	return nil, nil
}

func (m *mockRatingStorage) QryCsatByAssigneeAndTime(appkey string, startMs, endMs int64) (map[string]storageModels.CsatAggregate, error) {
	out := map[string]storageModels.CsatAggregate{}
	for i := range m.created {
		r := m.created[i]
		if r.AppKey != appkey || r.AssigneeId == "" {
			continue
		}
		if r.CreatedTime < startMs || r.CreatedTime >= endMs {
			continue
		}
		if _, ok := out[r.AssigneeId]; !ok {
			out[r.AssigneeId] = storageModels.CsatAggregate{AssigneeID: r.AssigneeId}
		}
		agg := out[r.AssigneeId]
		avg := *agg.CsatAvg
		count := agg.RatingCount
		newAvg := (avg*float64(count) + float64(r.Rating)) / float64(count+1)
		agg.CsatAvg = &newAvg
		agg.RatingCount = count + 1
		out[r.AssigneeId] = agg
	}
	return out, nil
}

type mockMessageStorage struct {
	last storageModels.TicketMessage
}

func (m *mockMessageStorage) UpsertByMsgId(item storageModels.TicketMessage) error {
	m.last = item
	return nil
}

func (m *mockMessageStorage) QryLastByTicket(appkey, ticketId string) (*storageModels.TicketMessage, error) {
	if m.last.AppKey == appkey && m.last.TicketId == ticketId {
		r := m.last
		return &r, nil
	}
	return nil, nil
}

func setupRecordMocks(t *testing.T) (*mockRatingStorage, *mockMessageStorage, *storageModels.Ticket) {
	t.Helper()
	rs := &mockRatingStorage{}
	ms := &mockMessageStorage{}
	origRating := newTicketRatingStorageForRating
	origMsg := newTicketMessageStorageForRating
	origTicket := newTicketStorageForRating
	newTicketRatingStorageForRating = func() storageModels.ITicketRatingStorage { return rs }
	newTicketMessageStorageForRating = func() storageModels.ITicketMessageStorage { return ms }
	ticket := &storageModels.Ticket{
		AppKey:      "app_1",
		TicketId:    "ticket_1",
		SourceId:    "customer_1",
		AssigneeId:  "u_seat_1",
	}
	newTicketStorageForRating = func() storageModels.ITicketStorage { return &fakeTicketStorage{ticket: ticket} }
	t.Cleanup(func() {
		newTicketRatingStorageForRating = origRating
		newTicketMessageStorageForRating = origMsg
		newTicketStorageForRating = origTicket
		_ = storages.NewTicketStorage
	})
	return rs, ms, ticket
}

// 极简 mock 不实现 ITicketStorage 全部方法；通过 NewTicketStorage 的同样接口，但只接管 FindByTicketId
type fakeTicketStorage struct {
	ticket *storageModels.Ticket
}

func (f *fakeTicketStorage) Create(item storageModels.Ticket) error { return nil }
func (f *fakeTicketStorage) Update(item storageModels.Ticket) error { return nil }
func (f *fakeTicketStorage) Upsert(item storageModels.Ticket) error { return nil }
func (f *fakeTicketStorage) Delete(appkey, ticketId string) error   { return nil }
func (f *fakeTicketStorage) FindByTicketId(appkey, ticketId string) (*storageModels.Ticket, error) {
	if f.ticket != nil && f.ticket.AppKey == appkey && f.ticket.TicketId == ticketId {
		r := *f.ticket
		return &r, nil
	}
	return nil, nil
}
func (f *fakeTicketStorage) FindBySource(appkey, sourceId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryAll(appkey string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryVisible(appkey, assigneeId string, status *storageModels.TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) UpdateStatus(appkey, ticketId string, status storageModels.TicketStatus) error {
	return nil
}
func (f *fakeTicketStorage) ClaimIfPending(appkey, ticketId, assigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error {
	return nil
}
func (f *fakeTicketStorage) TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*storageModels.Ticket, error) {
	return nil, nil
}
func (f *fakeTicketStorage) MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error {
	return nil
}
func (f *fakeTicketStorage) UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error {
	return nil
}
func (f *fakeTicketStorage) CloseByIdle(appkey, ticketId string, idleMs, atMs int64) (bool, error) {
	return false, nil
}
func (f *fakeTicketStorage) MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error) {
	return false, nil
}
func (f *fakeTicketStorage) QryTicketsNeedingCsatNotification(limit int64) ([]*storageModels.Ticket, error) {
	return nil, nil
}

func makeReplyPayloadJSON(t *testing.T, appKey, ticketId string, rating int, comment string) string {
	t.Helper()
	p := CsatReplyPayload{Kind: "csatreply", Version: 1, AppKey: appKey, TicketID: ticketId, Rating: rating, Comment: comment, ClientTs: 1785000000000}
	b, _ := json.Marshal(p)
	return string(b)
}

func TestRecordFromCustomMessageOK(t *testing.T) {
	rs, ms, _ := setupRecordMocks(t)
	// comment 为空 → source = card_button；显式断言这条路径
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 5, "")
	out, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_xyz", "jgm:csatreply", 1785000000000)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if out == nil || out.Rating != 5 {
		t.Fatalf("rating=%v, want 5", out)
	}
	if len(rs.created) != 1 || rs.created[0].Source != storageModels.TicketRatingSourceCardButton {
		t.Fatalf("rating storage wrong: %+v", rs.created)
	}
	if ms.last.MsgId != "msg_xyz" || ms.last.SenderRole != storageModels.TicketEventOperatorCustomer {
		t.Fatalf("message record wrong: %+v", ms.last)
	}
}

func TestRecordFromCustomMessageWithComment(t *testing.T) {
	rs, _, _ := setupRecordMocks(t)
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 3, "客服回复慢")
	if _, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_abc", "jgm:csatreply", 1785000000000); err != nil {
		t.Fatalf("err = %v", err)
	}
	if rs.created[0].Source != storageModels.TicketRatingSourceCardWithComment {
		t.Fatalf("source = %q, want card_with_comment", rs.created[0].Source)
	}
}

func TestRecordFromCustomMessageAppKeyMismatch(t *testing.T) {
	setupRecordMocks(t)
	content := makeReplyPayloadJSON(t, "OTHER", "ticket_1", 5, "")
	if _, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_1", "jgm:csatreply", 1785000000000); !errors.Is(err, ErrCsatAppKeyMismatch) {
		t.Fatalf("err = %v, want ErrCsatAppKeyMismatch", err)
	}
}

func TestRecordFromCustomMessageSenderNotCustomer(t *testing.T) {
	setupRecordMocks(t)
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 5, "")
	if _, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "NOT_CUSTOMER", "msg_1", "jgm:csatreply", 1785000000000); !errors.Is(err, ErrCsatSenderNotCustomer) {
		t.Fatalf("err = %v, want ErrCsatSenderNotCustomer", err)
	}
}

func TestRecordFromCustomMessageDuplicate(t *testing.T) {
	rs, _, _ := setupRecordMocks(t)
	// 让 storage 抛 ErrTicketRatingAlreadyExists 的等价错误（包级），
	// ticketratingdao.Create 会做 errors.Is 翻译。
	rs.createErr = dbs.ErrTicketRatingAlreadyExists
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 5, "")
	_, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_1", "jgm:csatreply", 1785000000000)
	if !errors.Is(err, ErrCsatAlreadyRated) {
		t.Fatalf("err = %v, want ErrCsatAlreadyRated", err)
	}
}

func TestRecordFromCustomMessageCommentTruncated(t *testing.T) {
	rs, _, _ := setupRecordMocks(t)
	long := strings.Repeat("x", 600)
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 3, long)
	if _, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_1", "jgm:csatreply", 1785000000000); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(rs.created[0].Comment) != 500 {
		t.Fatalf("comment len = %d, want 500", len(rs.created[0].Comment))
	}
}

func TestGetTicketRatingReturnsStored(t *testing.T) {
	_, _, _ = setupRecordMocks(t)
	content := makeReplyPayloadJSON(t, "app_1", "ticket_1", 5, "")
	if _, err := RecordFromCustomMessage(context.Background(), "app_1", "ticket_1",
		content, "customer_1", "msg_1", "jgm:csatreply", 1785000000000); err != nil {
		t.Fatalf("err = %v", err)
	}
	out, err := GetTicketRating("app_1", "ticket_1", "customer_1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out == nil || out.Rating != 5 {
		t.Fatalf("rating = %+v, want rating=5", out)
	}
}

func TestGetTicketRatingEmpty(t *testing.T) {
	_, _, _ = setupRecordMocks(t)
	out, err := GetTicketRating("app_1", "missing", "customer_1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != nil {
		t.Fatalf("rating = %+v, want nil", out)
	}
}
