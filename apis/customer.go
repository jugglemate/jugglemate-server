package apis

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

func QryCustomerTickets(ctx *gin.Context) {
	req, ok := parseQryCustomerTicketsReq(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.QryCustomerTickets(ctxs.ToCtx(ctx), req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func parseQryCustomerTicketsReq(ctx *gin.Context) (*models.QryCustomerTicketsReq, bool) {
	customerId := strings.TrimSpace(ctx.Query("customer_id"))
	if customerId == "" {
		customerId = strings.TrimSpace(ctx.Query("customerId"))
	}
	if customerId == "" {
		return nil, false
	}
	req := &models.QryCustomerTicketsReq{CustomerId: customerId, Limit: 20}
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

func QryCustomerInfo(ctx *gin.Context) {
	customerId := strings.TrimSpace(ctx.Query("customer_id"))
	if customerId == "" {
		customerId = strings.TrimSpace(ctx.Query("customerId"))
	}
	if customerId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.QryCustomerInfo(ctxs.ToCtx(ctx), customerId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func StartWebCustom(ctx *gin.Context) {
	var req models.StartCustomReq
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Identifier == "" || strings.TrimSpace(req.InboxId) == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.StartWebCustom(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}
