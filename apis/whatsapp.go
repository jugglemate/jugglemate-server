package apis

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/services"
)

func WhatsAppWebhookVerify(ctx *gin.Context) {
	challenge, ok := services.WhatsAppWebhookChallenge(
		ctx.Param("inbox_id"),
		ctx.Query("hub.mode"),
		ctx.Query("hub.verify_token"),
		ctx.Query("hub.challenge"),
	)
	if !ok {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	ctx.String(http.StatusOK, challenge)
}

func WhatsAppWebhook(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	inboxID := ctx.Param("inbox_id")
	code := services.ProcessWhatsAppWebhook(inboxID, body, ctx.GetHeader("X-Hub-Signature-256"))
	if code == errs.IMErrorCode_APP_NOT_LOGIN {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	if code != errs.IMErrorCode_SUCCESS {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Status(http.StatusOK)
}
