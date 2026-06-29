package apis

import (
	"io"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

var processTelegramWebhookForAPI = services.ProcessTelegramWebhook

func TelegramWebhook(ctx *gin.Context) {
	inboxId := ctx.Param("inbox_id")
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Printf("[TelegramWebhook] read body failed inbox=%s err=%v", inboxId, err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	code := processTelegramWebhookForAPI(inboxId, body)
	if code != errs.IMErrorCode_SUCCESS {
		log.Printf("[TelegramWebhook] process failed inbox=%s code=%d", inboxId, code)
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, nil)
}
