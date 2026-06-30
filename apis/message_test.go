package apis

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/services"
)

func TestWebhookMsgsReceivesMessageSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhooks/message", WebhookMsgs)

	body := []byte(`{
	  "event_type": "message",
	  "timestamp": 1713456000000,
	  "payload": [{
	    "platform": "iOS",
	    "sender": "userid1",
	    "receiver": "userid2",
	    "conver_type": 1,
	    "msg_type": "text",
	    "msg_content": "Hello, world!",
	    "mention_info": {
	      "mention_type": "mention_all",
	      "target_user_ids": ["userid1", "userid2"]
	    },
	    "msg_id": "123",
	    "msg_time": 1718980832156
	  }]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWebhookMsgsDelegatesParsedPayloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhooks/message", WebhookMsgs)

	var calls []services.WebhookMessagePayload
	var appkeys []string
	oldProcess := processWebhookMsgForAPI
	processWebhookMsgForAPI = func(appkey string, payload services.WebhookMessagePayload) errs.IMErrorCode {
		appkeys = append(appkeys, appkey)
		calls = append(calls, payload)
		return errs.IMErrorCode_SUCCESS
	}
	t.Cleanup(func() { processWebhookMsgForAPI = oldProcess })

	body := []byte(`{
	  "event_type": "message",
	  "timestamp": 1713456000000,
	  "payload": [
	    {"platform":"iOS","sender":"agent_1","receiver":"ticket_1","conver_type":2,"msg_type":"jg:text","msg_content":"{\"content\":\"one\"}","msg_id":"msg_1","msg_time":1718980832156},
	    {"platform":"Web","sender":"agent_2","receiver":"ticket_2","conver_type":2,"msg_type":"text","msg_content":"two","msg_id":"msg_2","msg_time":1718980832157}
	  ]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("appkey", "app_1")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	for _, appkey := range appkeys {
		if appkey != "app_1" {
			t.Fatalf("appkey = %q, want app_1", appkey)
		}
	}
	if calls[0].Sender != "agent_1" || calls[0].Receiver != "ticket_1" || calls[0].ConverType != 2 || calls[0].MsgType != "jg:text" || calls[0].MsgID != "msg_1" {
		t.Fatalf("first call = %+v", calls[0])
	}
	if calls[1].Sender != "agent_2" || calls[1].Receiver != "ticket_2" || calls[1].MsgContent != "two" {
		t.Fatalf("second call = %+v", calls[1])
	}
}

func TestWebhookMsgsAcknowledgesMalformedBodyWithoutDelegating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhooks/message", WebhookMsgs)

	called := false
	oldProcess := processWebhookMsgForAPI
	processWebhookMsgForAPI = func(appkey string, payload services.WebhookMessagePayload) errs.IMErrorCode {
		called = true
		return errs.IMErrorCode_SUCCESS
	}
	t.Cleanup(func() { processWebhookMsgForAPI = oldProcess })

	req := httptest.NewRequest(http.MethodPost, "/webhooks/message", bytes.NewReader([]byte(`{bad json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if called {
		t.Fatal("handler delegated malformed body")
	}
}
