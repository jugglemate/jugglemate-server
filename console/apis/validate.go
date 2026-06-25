package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func Validate(ctx *gin.Context) {
	appkey := ctx.GetString(string(ctxs.CtxKey_AppKey))
	requesterId := ctx.GetString(string(ctxs.CtxKey_RequesterId))
	role := storageModels.UserRole(ctx.GetInt(string(ctxs.CtxKey_RoleType)))
	if appkey == "" || requesterId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		ctx.Abort()
		return
	}

	if role != storageModels.UserRoleAdmin {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		ctx.Abort()
		return
	}
}
