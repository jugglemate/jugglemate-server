package services

import (
	"context"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/tools"
)

var TicketAssignedNtfMsgType string = "jgm:ticketassign"

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
)

type TicketAssignedNtfMsg struct {
	TicketId   string              `json:"ticket_id"`
	Operator   *apiModels.UserInfo `json:"operator"`
	Assignee   *apiModels.UserInfo `json:"assignee"`
	AssignType AssignType          `json:"assign_type"`
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
		IsCount:    tools.BoolPtr(false),
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
			IsCount:    tools.BoolPtr(false),
		})
	}
}
