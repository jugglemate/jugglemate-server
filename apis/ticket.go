package apis

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

func TransferTicket(ctx *gin.Context) {
	ticketId := strings.TrimSpace(ctx.Param("ticket_id"))
	var req apiModels.TransferTicketReq
	if ticketId == "" || ctx.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.AssigneeId) == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	req.AssigneeId = strings.TrimSpace(req.AssigneeId)
	code, resp := services.TransferTicket(ctxs.ToCtx(ctx), ticketId, &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func ClaimTicket(ctx *gin.Context) {
	ticketId := ctx.Param("ticket_id")
	if ticketId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.ClaimTicket(ctxs.ToCtx(ctx), ticketId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func QryTickets(ctx *gin.Context) {
	req, ok := parseQryTicketsReq(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.QryTickets(ctxs.ToCtx(ctx), req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func QryTicketInboxMembers(ctx *gin.Context) {
	req, ok := parseQryTicketInboxMembersReq(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.QryTicketInboxMembers(ctxs.ToCtx(ctx), req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func parseQryTicketInboxMembersReq(ctx *gin.Context) (*apiModels.QryTicketInboxMembersReq, bool) {
	ticketId := strings.TrimSpace(ctx.Query("ticket_id"))
	if ticketId == "" {
		ticketId = strings.TrimSpace(ctx.Query("ticketId"))
	}
	if ticketId == "" {
		return nil, false
	}
	req := &apiModels.QryTicketInboxMembersReq{TicketId: ticketId, Limit: 50}
	if startIdStr := ctx.Query("start_id"); startIdStr != "" {
		startId, err := strconv.ParseInt(startIdStr, 10, 64)
		if err != nil || startId < 0 {
			return nil, false
		}
		req.StartId = startId
	}
	if limitStr := ctx.Query("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 || limit > 100 {
			return nil, false
		}
		req.Limit = limit
	}
	return req, true
}

func parseQryTicketsReq(ctx *gin.Context) (*apiModels.QryTicketsReq, bool) {
	req := &apiModels.QryTicketsReq{
		Limit:  20,
		Offset: 0,
	}
	if statusStr := ctx.Query("status"); statusStr != "" {
		status, err := strconv.Atoi(statusStr)
		if err != nil || status < 0 || status > 2 {
			return nil, false
		}
		req.Status = &status
	}
	if handoffStr := ctx.Query("is_human_taken_over"); handoffStr != "" {
		value, err := strconv.ParseBool(handoffStr)
		if err != nil {
			return nil, false
		}
		req.IsHumanTakenOver = &value
	}
	if limitStr := ctx.Query("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 {
			return nil, false
		}
		req.Limit = limit
	}
	if offsetStr := ctx.Query("offset"); offsetStr != "" {
		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			return nil, false
		}
		req.Offset = offset
	}

	return req, true
}
