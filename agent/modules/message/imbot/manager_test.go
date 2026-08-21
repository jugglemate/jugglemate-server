package imbot

import (
	"context"
	"errors"
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

func TestMessageListenerReopensClosedTicketForCustomerGroupMessage(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdateCustomer := updateLastCustomerMsgAtForIMBot
	oldReopen := reopenTicketForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastCustomerMsgAtForIMBot = oldUpdateCustomer
		reopenTicketForIMBot = oldReopen
	}()

	var updatedCustomerAt int64
	var reopenedTicketID string
	var syncedTicketID string
	var notifiedTicketID string
	var notifiedSenderID string
	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		return &storageModels.Ticket{
			AppKey: appKey, TicketId: ticketID, SourceId: "customer_1",
			Status: storageModels.TicketStatusClosed,
		}, nil
	}
	updateLastCustomerMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error {
		updatedCustomerAt = atMs
		return nil
	}
	reopenTicketForIMBot = func(appKey, ticketID string) error {
		reopenedTicketID = ticketID
		return nil
	}

	manager := &Manager{}
	manager.SetTicketTagSync(func(appKey, ticketID string) error {
		syncedTicketID = ticketID
		return nil
	})
	manager.SetTicketReopenNotifier(func(appKey, ticketID, senderID string) {
		notifiedTicketID = ticketID
		notifiedSenderID = senderID
	})
	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: manager}
	msg := &sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Group},
		SenderId:     "customer_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello again"),
	}

	listener.advanceTicketLastUserMsgAt(msg)

	if updatedCustomerAt != msg.MsgTime {
		t.Fatalf("updatedCustomerAt = %d, want %d", updatedCustomerAt, msg.MsgTime)
	}
	if reopenedTicketID != "ticket_1" {
		t.Fatalf("reopenedTicketID = %q, want ticket_1", reopenedTicketID)
	}
	if syncedTicketID != "ticket_1" {
		t.Fatalf("syncedTicketID = %q, want ticket_1", syncedTicketID)
	}
	if notifiedTicketID != "ticket_1" || notifiedSenderID != "customer_1" {
		t.Fatalf("reopen notification ticket=%q sender=%q", notifiedTicketID, notifiedSenderID)
	}
}

func TestMessageListenerDoesNotReopenActiveTicketForCustomerMessage(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdateCustomer := updateLastCustomerMsgAtForIMBot
	oldReopen := reopenTicketForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastCustomerMsgAtForIMBot = oldUpdateCustomer
		reopenTicketForIMBot = oldReopen
	}()

	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		return &storageModels.Ticket{AppKey: appKey, TicketId: ticketID, SourceId: "customer_1", Status: storageModels.TicketStatusProcessing}, nil
	}
	updateLastCustomerMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error { return nil }
	reopened := false
	reopenTicketForIMBot = func(appKey, ticketID string) error {
		reopened = true
		return nil
	}

	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: &Manager{}}
	listener.advanceTicketLastUserMsgAt(&sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Group},
		SenderId:     "customer_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello"),
	})

	if reopened {
		t.Fatal("active ticket should not be reopened")
	}
}

func TestMessageListenerDoesNotSyncTagsWhenReopenFails(t *testing.T) {
	oldFind := findTicketForIMBot
	oldUpdateCustomer := updateLastCustomerMsgAtForIMBot
	oldReopen := reopenTicketForIMBot
	defer func() {
		findTicketForIMBot = oldFind
		updateLastCustomerMsgAtForIMBot = oldUpdateCustomer
		reopenTicketForIMBot = oldReopen
	}()

	findTicketForIMBot = func(appKey, ticketID string) (*storageModels.Ticket, error) {
		return &storageModels.Ticket{AppKey: appKey, TicketId: ticketID, SourceId: "customer_1", Status: storageModels.TicketStatusClosed}, nil
	}
	updateLastCustomerMsgAtForIMBot = func(appKey, ticketID string, atMs int64) error { return nil }
	reopenTicketForIMBot = func(appKey, ticketID string) error { return errors.New("update failed") }

	synced := false
	manager := &Manager{}
	manager.SetTicketTagSync(func(appKey, ticketID string) error {
		synced = true
		return nil
	})
	listener := &messageListener{appKey: "app_1", botUserID: "bot_1", manager: manager}
	listener.advanceTicketLastUserMsgAt(&sdkmodels.Message{
		Conversation: &sdkmodels.Conversation{ConversationId: "ticket_1", ConversationType: pbobjs.ChannelType_Group},
		SenderId:     "customer_1",
		MsgId:        "msg_1",
		MsgTime:      1785300000000,
		MsgContent:   messages.NewTextMessage("hello"),
	})

	if synced {
		t.Fatal("tags should not be synced when reopening fails")
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
