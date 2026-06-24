package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

func Login(ctx *gin.Context) {
	var req models.LoginReq
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Account == "" || req.Password == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := services.Login(ctxs.ToCtx(ctx), req.Account, req.Password)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

func Register(ctx *gin.Context) {
	var req models.RegisterReq
	if err := ctx.ShouldBindJSON(&req); err != nil || req.Account == "" || req.Password == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_REQ_BODY_ILLEGAL)
		return
	}
	code, resp := services.Register(ctxs.ToCtx(ctx), req.Account, req.Password)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}
