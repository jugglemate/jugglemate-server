package dbs

import (
	"testing"

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
