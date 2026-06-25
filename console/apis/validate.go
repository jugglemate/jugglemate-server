package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func Validate(ctx *gin.Context) {
	appkey := ctx.GetString(string(ctxs.CtxKey_AppKey))
	requesterId := ctx.GetString(string(ctxs.CtxKey_RequesterId))
	if appkey == "" || requesterId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		ctx.Abort()
		return
	}

	user, err := storages.NewUserStorage().FindByUserId(appkey, requesterId)
	if err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		ctx.Abort()
		return
	}
	if user == nil || user.Role != storageModels.UserRoleAdmin {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		ctx.Abort()
		return
	}
}
