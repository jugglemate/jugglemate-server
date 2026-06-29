package apis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
	consoleModels "github.com/juggleim/jugglemate-server/console/apis/models"
)

func TestCreateJuggleIMInboxHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldCreate := createJuggleIMInboxForAPI
	defer func() { createJuggleIMInboxForAPI = oldCreate }()

	var gotReq *consoleModels.CreateJuggleIMInboxReq
	createJuggleIMInboxForAPI = func(_ context.Context, req *consoleModels.CreateJuggleIMInboxReq) (errs.IMErrorCode, *consoleModels.InboxItem) {
		gotReq = req
		return errs.IMErrorCode_SUCCESS, &consoleModels.InboxItem{
			ID:          "inbox_juggle",
			Name:        req.Name,
			ChannelType: "juggleim",
			ChannelConf: consoleModels.JuggleIMConfigItem{BotName: req.BotName},
		}
	}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/console/inboxes/juggleim", strings.NewReader(`{"name":"JuggleIM","bot_name":"bot","bot_token":"secret"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CreateJuggleIMInbox(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if gotReq == nil || gotReq.Name != "JuggleIM" || gotReq.BotName != "bot" || gotReq.BotToken != "secret" {
		t.Fatalf("req = %+v", gotReq)
	}
	if !strings.Contains(w.Body.String(), `"channel_type":"juggleim"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestCreateJuggleIMInboxHandlerRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/jmate/console/inboxes/juggleim", strings.NewReader(`{`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CreateJuggleIMInbox(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "17004") {
		t.Fatalf("body = %s", w.Body.String())
	}
}
