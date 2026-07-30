package dbs

import (
	"testing"

	"github.com/juggleim/jugglemate-server/storages/models"
)

func TestTicketMessageDaoMapping(t *testing.T) {
	event := models.TicketMessage{
		AppKey:      "app_1",
		TicketId:    "ticket_1",
		SenderId:    "customer_1",
		SenderRole:  models.TicketEventOperatorCustomer,
		MsgId:       "msg_xxx",
		MsgType:     "jgm:csatreply",
		CreatedTime: 1785000000000,
	}
	dao := newTicketMessageDao(event)
	if dao.SenderRole != "customer" {
		t.Fatalf("dao sender_role = %q, want customer", dao.SenderRole)
	}
	if dao.MsgType != "jgm:csatreply" {
		t.Fatalf("dao msg_type = %q, want jgm:csatreply", dao.MsgType)
	}

	mapped := dao.toModel()
	if mapped.SenderRole != models.TicketEventOperatorCustomer {
		t.Fatalf("mapped sender_role = %q, want customer", mapped.SenderRole)
	}
	if mapped.MsgType != "jgm:csatreply" {
		t.Fatalf("mapped msg_type = %q, want jgm:csatreply", mapped.MsgType)
	}
	if mapped.CreatedTime != 1785000000000 {
		t.Fatalf("mapped created_time = %d, want 1785000000000", mapped.CreatedTime)
	}

	var nilDao *TicketMessageDao
	if got := nilDao.toModel(); got != nil {
		t.Fatalf("nil dao toModel = %+v, want nil", got)
	}
}

func TestTicketRatingDaoMapping(t *testing.T) {
	rating := models.TicketRating{
		AppKey:      "app_1",
		TicketId:    "ticket_1",
		CustomerId:  "customer_1",
		AssigneeId:  "u_seat_1",
		Rating:      5,
		Comment:     "非常好",
		Source:      models.TicketRatingSourceCardWithComment,
		CreatedTime: 1785000000000,
	}
	dao := newTicketRatingDao(rating)
	if dao.Source != "card_with_comment" {
		t.Fatalf("dao source = %q, want card_with_comment", dao.Source)
	}

	mapped := dao.toModel()
	if mapped.Source != models.TicketRatingSourceCardWithComment {
		t.Fatalf("mapped source = %q, want card_with_comment", mapped.Source)
	}
	if mapped.Rating != 5 || mapped.Comment != "非常好" {
		t.Fatalf("mapped rating/comment mismatch: %+v", mapped)
	}
	if mapped.CreatedTime != 1785000000000 {
		t.Fatalf("mapped created_time = %d, want 1785000000000", mapped.CreatedTime)
	}

	if !models.IsValidTicketRatingSource("card_button") {
		t.Fatalf("card_button should be valid source")
	}
	if !models.IsValidTicketRatingSource("card_with_comment") {
		t.Fatalf("card_with_comment should be valid source")
	}
	if models.IsValidTicketRatingSource("unknown") {
		t.Fatalf("unknown should NOT be valid source")
	}

	var nilDao *TicketRatingDao
	if got := nilDao.toModel(); got != nil {
		t.Fatalf("nil dao toModel = %+v, want nil", got)
	}
}
