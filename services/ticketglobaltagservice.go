package services

import (
	"net/http"
	"strings"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// globalConverTagsRequest 定义 IM Server 全局会话标签接口的请求参数。
//
// 简要描述：当前公开版 imserver-sdk-go 尚未封装该接口，因此复用 SDK 已有的
// HttpCall 完成统一签名、请求发送和错误码解析，避免依赖本机源码 replace。
type globalConverTagsRequest struct {
	ConverID         string    `json:"conver_id"`
	ChannelType      int       `json:"channel_type"`
	SubChannel       string    `json:"sub_channel"`
	GlobalConverTags *[]string `json:"global_conver_tags"`
}

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
	setGlobalConverTagsForTicket  = func(sdk *juggleimsdk.JuggleIMSdk, req globalConverTagsRequest) (juggleimsdk.ApiCode, string, error) {
		return sdk.HttpCall(http.MethodPost, "/apigateway/convers/globaltags/set", req, nil)
	}
)

// SyncTicketGlobalConversationTags 重新读取工单，并使用最新持久化状态覆盖群会话全局标签。
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
	code, _, err := setGlobalConverTagsForTicket(sdk, globalConverTagsRequest{
		ConverID:         ticket.TicketId,
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

// ticketGlobalConversationTags 根据分配状态、工单状态和渠道生成完整标签集合。
//
// 简要描述：调用端使用覆盖语义写入 IM Server，因此这里必须一次返回三类标签，
// 不能只返回发生变化的标签，否则会丢失其他维度的现有状态。
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
