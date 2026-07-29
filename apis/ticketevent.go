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

func QryTicketEvents(ctx *gin.Context) {
	ticketId := strings.TrimSpace(ctx.Param("ticket_id"))
	if ticketId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	req, ok := parseQryTicketEventsReq(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	appKey := ctxs.GetAppKeyFromCtx(ctx)
	if appKey == "" || strings.TrimSpace(ticketId) == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		return
	}
	events, err := services.QryTicketEvents(ctxs.ToCtx(ctx), appKey, ticketId, req.Limit, req.Offset)
	if err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		return
	}
	resp := &apiModels.QryTicketEventsResp{
		Items: make([]*apiModels.TicketEventInfo, 0, len(events)),
	}
	for _, item := range events {
		if item == nil {
			continue
		}
		resp.Items = append(resp.Items, &apiModels.TicketEventInfo{
			ID:           item.ID,
			TicketId:     item.TicketId,
			EventType:    string(item.EventType),
			OperatorID:   item.OperatorID,
			OperatorType: string(item.OperatorType),
			Payload:      item.Payload,
			CreatedTime:  item.CreatedTime,
		})
	}
	// TIPS: 倒序拉取时 record id 单调递减 —— 取队尾 id 作为下一页游标。
	if len(events) > 0 {
		last := events[len(events)-1]
		if last != nil {
			resp.NextStartId = last.ID
		}
	}
	responses.SuccessHttpResp(ctx, resp)
}

func parseQryTicketEventsReq(ctx *gin.Context) (*apiModels.QryTicketEventsReq, bool) {
	req := &apiModels.QryTicketEventsReq{Limit: 50}
	if limitStr := ctx.Query("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 || limit > 100 {
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
