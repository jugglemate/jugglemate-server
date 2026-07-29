package dbs

import (
	"testing"

	"github.com/juggleim/jugglemate-server/storages/models"
)

func TestTicketEventDaoEventTypeMapping(t *testing.T) {
	event := models.TicketEvent{
		AppKey:       "app_1",
		TicketId:     "ticket_1",
		EventType:    models.TicketEventTypeHumanTakeover,
		OperatorID:   "customer_1",
		OperatorType: models.TicketEventOperatorCustomer,
		Payload:      `{"keyword":"转人工"}`,
	}
	dao := newTicketEventDao(event)
	if dao.EventType != "human_takeover" {
		t.Fatalf("dao event_type = %q, want human_takeover", dao.EventType)
	}
	if dao.OperatorType != "customer" {
		t.Fatalf("dao operator_type = %q, want customer", dao.OperatorType)
	}
	if dao.Payload != `{"keyword":"转人工"}` {
		t.Fatalf("dao payload = %q, want raw json", dao.Payload)
	}

	mapped := dao.toModel()
	if mapped.EventType != models.TicketEventTypeHumanTakeover {
		t.Fatalf("mapped event_type = %q, want human_takeover", mapped.EventType)
	}
	if mapped.OperatorType != models.TicketEventOperatorCustomer {
		t.Fatalf("mapped operator_type = %q, want customer", mapped.OperatorType)
	}
	if mapped.Payload != `{"keyword":"转人工"}` {
		t.Fatalf("mapped payload = %q, want raw json", mapped.Payload)
	}
}

func TestTicketEventDaoNilPointerSafe(t *testing.T) {
	var nilDao *TicketEventDao
	if got := nilDao.toModel(); got != nil {
		t.Fatalf("nil dao toModel = %+v, want nil", got)
	}
}
