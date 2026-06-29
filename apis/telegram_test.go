package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
)

func TestTelegramWebhookPassesInboxAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldProcess := processTelegramWebhookForAPI
	defer func() { processTelegramWebhookForAPI = oldProcess }()

	var gotInbox string
	var gotBody string
	processTelegramWebhookForAPI = func(inboxId string, body []byte) errs.IMErrorCode {
		gotInbox = inboxId
		gotBody = string(body)
		return errs.IMErrorCode_SUCCESS
	}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "inbox_id", Value: "inbox_tg"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/webhooks/telegram/inbox_tg", strings.NewReader(`{"ok":true}`))

	TelegramWebhook(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if gotInbox != "inbox_tg" || gotBody != `{"ok":true}` {
		t.Fatalf("got inbox=%q body=%q", gotInbox, gotBody)
	}
}

func TestTelegramWebhookReturnsErrorForServiceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldProcess := processTelegramWebhookForAPI
	defer func() { processTelegramWebhookForAPI = oldProcess }()

	processTelegramWebhookForAPI = func(inboxId string, body []byte) errs.IMErrorCode {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "inbox_id", Value: "missing"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/webhooks/telegram/missing", strings.NewReader(`{}`))

	TelegramWebhook(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want API envelope success status", w.Code)
	}
	if !strings.Contains(w.Body.String(), "17014") {
		t.Fatalf("body = %s, want channel error code", w.Body.String())
	}
}
