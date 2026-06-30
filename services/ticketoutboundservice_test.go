package services

import (
	"errors"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestProcessWebhookMessageForwardsTelegramTicketGroupMessage(t *testing.T) {
	env := newOutboundTestEnv(t)

	code := ProcessWebhookMessage("app_1", WebhookMessagePayload{
		Sender:     "agent_1",
		Receiver:   "ticket_1",
		ConverType: ConversationType_Ticket,
		MsgType:    "jg:text",
		MsgContent: `{"content":"hello telegram"}`,
		MsgID:      "msg_1",
	})

	if code != errs.IMErrorCode_SUCCESS {
		t.Fatalf("code = %d", code)
	}
	if env.sent.token != "secret" || env.sent.target != "998877" || env.sent.text != "hello telegram" {
		t.Fatalf("sent = %+v", env.sent)
	}
}

func TestProcessWebhookMessageSkipsIneligibleTicketMessages(t *testing.T) {
	cases := []struct {
		name    string
		payload WebhookMessagePayload
	}{
		{
			name: "private message",
			payload: WebhookMessagePayload{
				Sender: "agent_1", Receiver: "ticket_1", ConverType: 1, MsgType: "jg:text", MsgContent: `{"content":"hello"}`,
			},
		},
		{
			name: "non-ticket group",
			payload: WebhookMessagePayload{
				Sender: "agent_1", Receiver: "group_1", ConverType: ConversationType_Ticket, MsgType: "jg:text", MsgContent: `{"content":"hello"}`,
			},
		},
		{
			name: "visitor source loop",
			payload: WebhookMessagePayload{
				Sender: "customer_source", Receiver: "ticket_1", ConverType: ConversationType_Ticket, MsgType: "jg:text", MsgContent: `{"content":"hello"}`,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			env := newOutboundTestEnv(t)
			code := ProcessWebhookMessage("app_1", tt.payload)
			if code != errs.IMErrorCode_SUCCESS {
				t.Fatalf("code = %d", code)
			}
			if env.sent.called {
				t.Fatalf("send called for skipped payload: %+v", env.sent)
			}
		})
	}
}

func TestProcessWebhookMessageSkipsMissingTicketInboxUnsupportedAndTelegramConfig(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*outboundTestEnv)
	}{
		{
			name: "missing ticket",
			setup: func(env *outboundTestEnv) {
				env.tickets.byTicket = map[string]*storageModels.Ticket{}
			},
		},
		{
			name: "missing inbox",
			setup: func(env *outboundTestEnv) {
				env.inboxes.byId = map[string]*storageModels.Inbox{}
			},
		},
		{
			name: "unsupported channel",
			setup: func(env *outboundTestEnv) {
				env.inboxes.byId["inbox_tg"].ChannelType = string(ChannelType_Widget)
			},
		},
		{
			name: "missing bot token",
			setup: func(env *outboundTestEnv) {
				env.inboxes.byId["inbox_tg"].ChannelConf = `{"bot_name":"bot"}`
			},
		},
		{
			name: "missing customer",
			setup: func(env *outboundTestEnv) {
				env.customers.byId = map[string]*storageModels.Customer{}
			},
		},
		{
			name: "missing customer identifier",
			setup: func(env *outboundTestEnv) {
				env.customers.byId["customer_1"].Identifier = ""
			},
		},
		{
			name: "empty text",
			setup: func(env *outboundTestEnv) {
				env.payload.MsgContent = `{"content":"   "}`
			},
		},
		{
			name: "unsupported msg type",
			setup: func(env *outboundTestEnv) {
				env.payload.MsgType = "jg:image"
				env.payload.MsgContent = `{"url":"https://example.com/a.png"}`
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			env := newOutboundTestEnv(t)
			tt.setup(env)
			code := ProcessWebhookMessage("app_1", env.payload)
			if code != errs.IMErrorCode_SUCCESS {
				t.Fatalf("code = %d", code)
			}
			if env.sent.called {
				t.Fatalf("send called for no-op payload: %+v", env.sent)
			}
		})
	}
}

func TestProcessWebhookMessageReturnsErrorOnTelegramSendFailure(t *testing.T) {
	env := newOutboundTestEnv(t)
	env.sendErr = errors.New("telegram down")

	code := ProcessWebhookMessage("app_1", env.payload)

	if code != errs.IMErrorCode_APP_INTERNAL_TIMEOUT {
		t.Fatalf("code = %d, want %d", code, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
	}
	if !env.sent.called {
		t.Fatal("send was not called")
	}
}

func TestProcessWebhookMessageSkippedPayloadDoesNotPreventLaterEligiblePayload(t *testing.T) {
	env := newOutboundTestEnv(t)
	for _, payload := range []WebhookMessagePayload{
		{Sender: "agent_1", Receiver: "not_ticket", ConverType: ConversationType_Ticket, MsgType: "jg:text", MsgContent: `{"content":"skip"}`},
		env.payload,
	} {
		if code := ProcessWebhookMessage("app_1", payload); code != errs.IMErrorCode_SUCCESS {
			t.Fatalf("code = %d", code)
		}
	}
	if !env.sent.called || env.sent.text != "hello telegram" {
		t.Fatalf("eligible payload not sent: %+v", env.sent)
	}
}

type outboundTestEnv struct {
	inboxes   *telegramFakeInboxStorage
	customers *telegramFakeCustomerStorage
	tickets   *telegramFakeTicketStorage
	payload   WebhookMessagePayload
	sendErr   error
	sent      struct {
		called bool
		token  string
		target string
		text   string
	}
}

func newOutboundTestEnv(t *testing.T) *outboundTestEnv {
	t.Helper()
	env := &outboundTestEnv{
		inboxes: &telegramFakeInboxStorage{byId: map[string]*storageModels.Inbox{
			"inbox_tg": {InboxId: "inbox_tg", ChannelType: string(ChannelType_Telegram), ChannelConf: `{"bot_name":"bot","bot_token":"secret"}`, AppKey: "app_1"},
		}},
		customers: &telegramFakeCustomerStorage{byId: map[string]*storageModels.Customer{
			"customer_1": {CustomerId: "customer_1", Identifier: "998877", AppKey: "app_1"},
		}, byIdentifier: map[string]*storageModels.Customer{}},
		tickets: &telegramFakeTicketStorage{byTicket: map[string]*storageModels.Ticket{
			"ticket_1": {TicketId: "ticket_1", SourceId: "customer_source", CustomerId: "customer_1", InboxId: "inbox_tg", AppKey: "app_1"},
		}, bySource: map[string]*storageModels.Ticket{}},
		payload: WebhookMessagePayload{
			Sender:     "agent_1",
			Receiver:   "ticket_1",
			ConverType: ConversationType_Ticket,
			MsgType:    "jg:text",
			MsgContent: `{"content":"hello telegram"}`,
			MsgID:      "msg_1",
		},
	}
	oldInbox := newInboxStorageForOutbound
	oldCustomer := newCustomerStorageForOutbound
	oldTicket := newTicketStorageForOutbound
	oldSend := sendTelegramOutboundMessage
	newInboxStorageForOutbound = func() storageModels.IInboxStorage { return env.inboxes }
	newCustomerStorageForOutbound = func() storageModels.ICustomerStorage { return env.customers }
	newTicketStorageForOutbound = func() storageModels.ITicketStorage { return env.tickets }
	sendTelegramOutboundMessage = func(botToken, target, text string) error {
		env.sent.called = true
		env.sent.token = botToken
		env.sent.target = target
		env.sent.text = text
		return env.sendErr
	}
	t.Cleanup(func() {
		newInboxStorageForOutbound = oldInbox
		newCustomerStorageForOutbound = oldCustomer
		newTicketStorageForOutbound = oldTicket
		sendTelegramOutboundMessage = oldSend
	})
	return env
}
