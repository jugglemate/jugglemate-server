package imbot

import (
	"context"
	"testing"

	"github.com/juggleim/imbot-sdk-go/imbotclients/pbdefines/pbobjs"
	sdkmodels "github.com/juggleim/imbot-sdk-go/models"
	"github.com/juggleim/imbot-sdk-go/models/messages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestMessageListenerAdvancesLastUserMsgAtForAssigneeGroupMessage(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdate := updateLastUserMsgAtForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastUserMsgAtForIMBot = oldUpdate
	}()

	var updatedAt int64
	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		return &storageModels.Ticket{AppKey: appKey, TicketId: ticketID, AssigneeId: "u_seat_1"}, nil
	}
	updateLastUserMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error {
		updatedAt = atMs
		return nil
	}

	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: &Manager{}}
	msg := &sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Group},
		SenderId:     "u_seat_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello"),
	}

	listener.advanceTicketLastUserMsgAt(msg)

	if updatedAt != 1785300000000 {
		t.Fatalf("updatedAt = %d, want 1785300000000", updatedAt)
	}
}

func TestMessageListenerSkipsCustomerGroupMessage(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdate := updateLastUserMsgAtForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastUserMsgAtForIMBot = oldUpdate
	}()

	called := false
	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		return &storageModels.Ticket{AppKey: appKey, TicketId: ticketID, AssigneeId: "u_seat_1"}, nil
	}
	updateLastUserMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error {
		called = true
		return nil
	}

	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: &Manager{}}
	msg := &sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Group},
		SenderId:     "customer_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello"),
	}

	listener.advanceTicketLastUserMsgAt(msg)

	if called {
		t.Fatalf("customer message should not advance last_user_msg_at")
	}
}

func TestMessageListenerSkipsNonGroupMessage(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdate := updateLastUserMsgAtForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastUserMsgAtForIMBot = oldUpdate
	}()

	called := false
	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		called = true
		return nil, nil
	}
	updateLastUserMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error {
		called = true
		return nil
	}

	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: &Manager{}}
	msg := &sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Private},
		SenderId:     "u_seat_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello"),
	}

	listener.advanceTicketLastUserMsgAt(msg)

	if called {
		t.Fatalf("non-group message should be ignored")
	}
}

var _ = context.Background
