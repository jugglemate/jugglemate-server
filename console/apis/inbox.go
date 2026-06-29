package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
	consoleServices "github.com/juggleim/jugglemate-server/console/services"
)

var createJuggleIMInboxForAPI = consoleServices.CreateJuggleIMInbox

func QryInboxes(ctx *gin.Context) {
	limit, offset, ok := parsePagination(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := consoleServices.QryInboxes(ctxs.ToCtx(ctx), limit, offset)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func CreateTelegramInbox(ctx *gin.Context) {
	var req consoleModels.CreateTelegramInboxReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := consoleServices.CreateTelegramInbox(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func CreateJuggleIMInbox(ctx *gin.Context) {
	var req consoleModels.CreateJuggleIMInboxReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := createJuggleIMInboxForAPI(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func CreateWidgetInbox(ctx *gin.Context) {
	var req consoleModels.CreateWidgetInboxReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := consoleServices.CreateWidgetInbox(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func QryInboxMembers(ctx *gin.Context) {
	inboxId := ctx.Param("inbox_id")
	code, resp := consoleServices.QryInboxMembers(ctxs.ToCtx(ctx), inboxId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func ReplaceInboxMembers(ctx *gin.Context) {
	inboxId := ctx.Param("inbox_id")
	var req consoleModels.ReplaceInboxMembersReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code := consoleServices.ReplaceInboxMembers(ctxs.ToCtx(ctx), inboxId, &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, nil)
}
