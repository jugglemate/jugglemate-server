package services

import (
	"context"
	"log"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/tools"
)

var TicketAssignedNtfMsgType string = "jgm:ticketassign"
var TicketCsatNtfMsgType string = "jgm:csat"

type AssignType int

const (
	AssignType_Claim      AssignType = 0
	AssignType_AssignUser AssignType = 1
	AssignType_AssignTeam AssignType = 2
	// AssignType_HumanTakeover 表示由 Agent 转为人工接待，前端文案「人工接入」。
	//
	// TIPS: 此类通知没有确定的 operator/assignee —— 转人工由客户在群里触发，此刻还没有
	// 坐席接单，两个字段都为 null，前端只按 assign_type 渲染。
	AssignType_HumanTakeover AssignType = 3
	// AssignType_Reopen 表示关闭后的工单被客户新消息重新开启。
	AssignType_Reopen AssignType = 4
)

type TicketAssignedNtfMsg struct {
	TicketId   string              `json:"ticket_id"`
	Operator   *apiModels.UserInfo `json:"operator"`
	Assignee   *apiModels.UserInfo `json:"assignee"`
	AssignType AssignType          `json:"assign_type"`
}

// SendTicketReopenNtfMsg 向 Ticket 群广播「重启工单」通知。
// senderId 应使用触发重新开启的客户 IM 身份。
func SendTicketReopenNtfMsg(appkey, ticketId, senderId string) {
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		return
	}
	sdk.SendGroupMsg(juggleimsdk.Message{
		SenderId:   senderId,
		TargetIds:  []string{ticketId},
		MsgType:    TicketAssignedNtfMsgType,
		MsgContent: tools.ToJson(&TicketAssignedNtfMsg{TicketId: ticketId, AssignType: AssignType_Reopen}),
		IsStorage:  tools.BoolPtr(true),
		IsCount:    tools.BoolPtr(false),
	})
}

// SendTicketHumanTakeoverNtfMsg 向 Ticket 群广播「人工接入」通知。
//
// TIPS: 不能复用 SendTicketAssignedNtfMsg —— 那个函数从 ctx 取 appkey 和 requesterId，
// 而转人工由客户在群里触发、走的是 IM 入站链路，ctx 里没有控制台登录态，
// requesterId 会是空串导致消息发不出去。这里显式传入 appkey 与发送者。
//
// @param appkey 应用 AppKey
// @param ticketId 目标 Ticket 群 ID
// @param senderId 通知消息的发送者 IM 身份，通常传即将被移出群的 Agent Bot
func SendTicketHumanTakeoverNtfMsg(appkey, ticketId, senderId string) {
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		return
	}
	sdk.SendGroupMsg(juggleimsdk.Message{
		SenderId:   senderId,
		TargetIds:  []string{ticketId},
		MsgType:    TicketAssignedNtfMsgType,
		MsgContent: tools.ToJson(&TicketAssignedNtfMsg{TicketId: ticketId, AssignType: AssignType_HumanTakeover}),
		IsStorage:  tools.BoolPtr(true),
		IsCount:    tools.BoolPtr(true),
	})
}

// sendTicketCsatNtfMsgFn 是 SendTicketCsatNtfMsg 的可替换实现，供单测注入。
// 生产环境默认指向真实实现；test 中可替换为空操作以避免 imsdk.GetImSdk 依赖 DB。
var sendTicketCsatNtfMsgFn = SendTicketCsatNtfMsg

// SendTicketCsatNtfMsg 向 Ticket 群发送 jgm:csat 评价邀请卡片。
//
// 与 SendTicketAssignedNtfMsg 一致：走 IM REST API（imsdk.SendGroupMsg），
// 以指定 senderId（坐席）身份发送，IsStorage:true 由 IM Server 写入 ticket_messages。
// 由此 csat 消息不再依赖 Bot 长连接。
//
// @param appkey 应用 AppKey
// @param ticketId 目标 Ticket 群 ID
// @param senderId 发送者 IM 身份（应当是工单的 assignee 坐席 ID，不是 Bot）
// @param payload 评价邀请卡片内容
func SendTicketCsatNtfMsg(appkey, ticketId, senderId string, payload CsatInvitationPayload) {
	sdk := imsdk.GetImSdk(appkey)
	if sdk == nil {
		log.Printf("[CsatNtf] imsdk 未就绪 appkey=%s ticket=%s", appkey, ticketId)
		return
	}
	sdk.SendGroupMsg(juggleimsdk.Message{
		SenderId:   senderId,
		TargetIds:  []string{ticketId},
		MsgType:    TicketCsatNtfMsgType,
		MsgContent: tools.ToJson(payload),
		IsStorage:  tools.BoolPtr(true),
		IsCount:    tools.BoolPtr(true),
	})
}

func SendTicketAssignedNtfMsg(ctx context.Context, msg *TicketAssignedNtfMsg) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	requestId := ctxs.GetRequesterIdFromCtx(ctx)
	sdk := imsdk.GetImSdk(appkey)
	if sdk != nil {
		sdk.SendGroupMsg(juggleimsdk.Message{
			SenderId:   requestId,
			TargetIds:  []string{msg.TicketId},
			MsgType:    TicketAssignedNtfMsgType,
			MsgContent: tools.ToJson(msg),
			IsStorage:  tools.BoolPtr(true),
			IsCount:    tools.BoolPtr(true),
		})
	}
}
