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
)

type TicketAssignedNtfMsg struct {
	TicketId   string              `json:"ticket_id"`
	Operator   *apiModels.UserInfo `json:"operator"`
	Assignee   *apiModels.UserInfo `json:"assignee"`
	AssignType AssignType          `json:"assign_type"`
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
