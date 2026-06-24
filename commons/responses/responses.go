package responses

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
)

func ErrorHttpResp(ctx *gin.Context, code errs.IMErrorCode) {
	apiErr := errs.GetApiErrorByCode(code)
	ctx.JSON(apiErr.HttpCode, apiErr)
}

func SuccessHttpResp(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusOK, errs.SuccHttpResp{
		ApiErrorMsg: errs.ApiErrorMsg{
			Code: 0,
			Msg:  "success",
		},
		Data: data,
	})
}
