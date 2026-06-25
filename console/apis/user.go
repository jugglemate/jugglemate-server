package apis

import (
	"strconv"

	"github.com/gin-gonic/gin"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	consoleServices "github.com/juggleim/jugglemate-server/console/services"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
)

func QryUsers(ctx *gin.Context) {
	limit, offset, ok := parsePagination(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	req := consoleServices.ParseQryUsersReq(
		limit,
		offset,
		ctx.Query("keyword"),
		ctx.Query("role"),
		ctx.Query("sortField"),
		ctx.Query("sortOrder"),
	)
	code, resp := consoleServices.QryUsers(ctxs.ToCtx(ctx), req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func CreateUser(ctx *gin.Context) {
	var req consoleModels.CreateUserReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := consoleServices.CreateUser(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func UpdateUser(ctx *gin.Context) {
	userId := ctx.Param("user_id")
	if userId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	var req consoleModels.UpdateUserReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := consoleServices.UpdateUser(ctxs.ToCtx(ctx), userId, &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func DeleteUser(ctx *gin.Context) {
	userId := ctx.Param("user_id")
	if userId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code := consoleServices.DeleteUser(ctxs.ToCtx(ctx), userId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, nil)
}

func parsePagination(ctx *gin.Context) (limit, offset int64, ok bool) {
	limit = 20
	offset = 0
	if limitStr := ctx.Query("limit"); limitStr != "" {
		parsed, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, 0, false
		}
		limit = parsed
	}
	if offsetStr := ctx.Query("offset"); offsetStr != "" {
		parsed, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || parsed < 0 {
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}
