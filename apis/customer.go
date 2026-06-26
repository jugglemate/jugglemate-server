package apis

import (
	"github.com/gin-gonic/gin"
	"strings"

	"github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

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
