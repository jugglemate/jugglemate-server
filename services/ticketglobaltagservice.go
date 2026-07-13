package services

import (
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

const (
	ticketGlobalTagAssigned         = "assigned"
	ticketGlobalTagUnassigned       = "unassigned"
	ticketGlobalTagStatusPending    = "ticket_status_pending"
	ticketGlobalTagStatusProcessing = "ticket_status_processing"
	ticketGlobalTagStatusClosed     = "ticket_status_closed"
	ticketGlobalTagStatusReopen     = "ticket_status_reopen"
	ticketGlobalTagChannelWidget    = "channel_widget"
	ticketGlobalTagChannelTelegram  = "channel_telegram"
	ticketGlobalTagChannelJuggleIM  = "channel_juggleim"
)

var (
	newTicketStorageForGlobalTags = storages.NewTicketStorage
	getImSdkForTicketGlobalTags   = imsdk.GetImSdk
	setGlobalConverTagsForTicket  = func(sdk *juggleimsdk.JuggleIMSdk, req juggleimsdk.GlobalConverTagsReq) (juggleimsdk.ApiCode, string, error) {
		return sdk.SetGlobalConverTags(req)
	}
)

// SyncTicketGlobalConversationTags reloads the ticket and replaces the global
// tags on its group conversation with the ticket's current persisted state.
func SyncTicketGlobalConversationTags(appkey, ticketId string) errs.IMErrorCode {
	appkey = strings.TrimSpace(appkey)
	ticketId = strings.TrimSpace(ticketId)
	if appkey == "" || ticketId == "" {
		return errs.IMErrorCode_APP_ParamError
	}

	ticket, err := newTicketStorageForGlobalTags().FindByTicketId(appkey, ticketId)
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if ticket == nil {
		return errs.IMErrorCode_APP_ParamError
	}
	tags, ok := ticketGlobalConversationTags(ticket)
	if !ok {
		return errs.IMErrorCode_APP_ParamError
	}

	sdk := getImSdkForTicketGlobalTags(appkey)
	if sdk == nil {
		return errs.IMErrorCode_APP_NOT_EXISTED
	}
	code, _, err := setGlobalConverTagsForTicket(sdk, juggleimsdk.GlobalConverTagsReq{
		ConverId:         ticket.TicketId,
		ChannelType:      int(juggleimsdk.ChannelType_Group),
		GlobalConverTags: &tags,
	})
	if err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT
	}
	if code != juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS) {
		return errs.IMErrorCode(code)
	}
	return errs.IMErrorCode_SUCCESS
}

func ticketGlobalConversationTags(ticket *storageModels.Ticket) ([]string, bool) {
	if ticket == nil {
		return nil, false
	}

	assignmentTag := ticketGlobalTagUnassigned
	if strings.TrimSpace(ticket.AssigneeId) != "" {
		assignmentTag = ticketGlobalTagAssigned
	}

	var statusTag string
	switch ticket.Status {
	case storageModels.TicketStatusPending:
		statusTag = ticketGlobalTagStatusPending
	case storageModels.TicketStatusProcessing:
		statusTag = ticketGlobalTagStatusProcessing
	case storageModels.TicketStatusClosed:
		statusTag = ticketGlobalTagStatusClosed
	case storageModels.TicketStatusReOpen:
		statusTag = ticketGlobalTagStatusReopen
	default:
		return nil, false
	}

	var channelTag string
	switch strings.TrimSpace(ticket.ChannelType) {
	case string(ChannelType_Widget):
		channelTag = ticketGlobalTagChannelWidget
	case string(ChannelType_Telegram):
		channelTag = ticketGlobalTagChannelTelegram
	case string(ChannelType_JuggleIM):
		channelTag = ticketGlobalTagChannelJuggleIM
	default:
		return nil, false
	}

	return []string{assignmentTag, statusTag, channelTag}, true
}
