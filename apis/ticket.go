package apis

import (
	"strconv"

	"github.com/gin-gonic/gin"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

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
