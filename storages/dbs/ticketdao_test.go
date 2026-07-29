package dbs

import (
	"testing"
	"time"

	"github.com/juggleim/jugglemate-server/storages/models"
)

func TestTicketDaoChannelTypeMapping(t *testing.T) {
	model := models.Ticket{
		TicketId:    "ticket_1",
		InboxId:     "inbox_1",
		ChannelType: "telegram",
		AppKey:      "app_1",
	}
	dao := newTicketDao(model)
	if dao.ChannelType != "telegram" {
		t.Fatalf("dao channel type = %q, want telegram", dao.ChannelType)
	}

	mapped := dao.toModel()
	if mapped.ChannelType != "telegram" {
		t.Fatalf("model channel type = %q, want telegram", mapped.ChannelType)
	}
}

func TestTicketDaoHumanTakenOverMapping(t *testing.T) {
	now := time.Now()
	dao := &TicketDao{
		TicketId:         "ticket_1",
		IsHumanTakenOver: true,
		HumanTakenOverAt: now,
		HumanTakenOverBy: "customer_1",
	}
	mapped := dao.toModel()
	if !mapped.IsHumanTakenOver {
		t.Fatalf("mapped is_human_taken_over = false, want true")
	}
	if mapped.HumanTakenOverBy != "customer_1" {
		t.Fatalf("mapped human_taken_over_by = %q, want customer_1", mapped.HumanTakenOverBy)
	}
	if mapped.HumanTakenOverAt != now.UnixMilli() {
		t.Fatalf("mapped human_taken_over_at = %d, want %d", mapped.HumanTakenOverAt, now.UnixMilli())
	}

	unset := &TicketDao{TicketId: "ticket_2"}
	mappedUnset := unset.toModel()
	if mappedUnset.IsHumanTakenOver || mappedUnset.HumanTakenOverAt != 0 || mappedUnset.HumanTakenOverBy != "" {
		t.Fatalf("unset mapped = %+v, want zero values", mappedUnset)
	}
}
