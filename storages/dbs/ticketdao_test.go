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

// TestTicketDaoAutoCloseMapping 验证 last_user_msg_at / closed_at 字段映射。
func TestTicketDaoAutoCloseMapping(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-10 * time.Minute)
	t2 := now

	dao := &TicketDao{
		TicketId:         "ticket_ac",
		LastUserMsgAt:    &t1,
		ClosedAt:         &t2,
	}
	mapped := dao.toModel()
	if mapped.LastUserMsgAt != t1.UnixMilli() {
		t.Fatalf("mapped last_user_msg_at = %d, want %d", mapped.LastUserMsgAt, t1.UnixMilli())
	}
	if mapped.ClosedAt != t2.UnixMilli() {
		t.Fatalf("mapped closed_at = %d, want %d", mapped.ClosedAt, t2.UnixMilli())
	}

	unset := &TicketDao{TicketId: "ticket_ac_empty"}
	mappedUnset := unset.toModel()
	if mappedUnset.LastUserMsgAt != 0 || mappedUnset.ClosedAt != 0 {
		t.Fatalf("unset mapped = %+v, want zero values", mappedUnset)
	}
}
