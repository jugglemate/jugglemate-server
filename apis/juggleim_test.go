package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
)

func TestJuggleIMWebhookPassesInboxAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldProcess := processJuggleIMWebhookForAPI
	defer func() { processJuggleIMWebhookForAPI = oldProcess }()

	var gotInbox string
	var gotBody string
	processJuggleIMWebhookForAPI = func(inboxId string, body []byte) errs.IMErrorCode {
		gotInbox = inboxId
		gotBody = string(body)
		return errs.IMErrorCode_SUCCESS
	}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "inbox_id", Value: "inbox_juggle"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/webhooks/juggleim/inbox_juggle", strings.NewReader(`{"sender":"u_1"}`))

	JuggleIMWebhook(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if gotInbox != "inbox_juggle" || gotBody != `{"sender":"u_1"}` {
		t.Fatalf("got inbox=%q body=%q", gotInbox, gotBody)
	}
}

func TestJuggleIMWebhookReturnsErrorForServiceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldProcess := processJuggleIMWebhookForAPI
	defer func() { processJuggleIMWebhookForAPI = oldProcess }()

	processJuggleIMWebhookForAPI = func(inboxId string, body []byte) errs.IMErrorCode {
		return errs.IMErrorCode_APP_CHANNEL_NOT_EXIST
	}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Params = gin.Params{{Key: "inbox_id", Value: "missing"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/webhooks/juggleim/missing", strings.NewReader(`{}`))

	JuggleIMWebhook(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want API envelope success status", w.Code)
	}
	if !strings.Contains(w.Body.String(), "17014") {
		t.Fatalf("body = %s, want channel error code", w.Body.String())
	}
}
