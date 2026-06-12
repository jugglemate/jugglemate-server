package apis

import (
	"encoding/json"
	"testing"
)

func TestMsgCallbackBodyParse(t *testing.T) {
	raw := []byte(`{
	  "event_type": "message",
	  "timestamp": 1713456000000,
	  "payload": [{
	    "platform": "iOS",
	    "sender": "userid1",
	    "receiver": "userid2",
	    "conver_type": 1,
	    "msg_type": "text",
	    "msg_content": "Hello, world!",
	    "msg_id": "123",
	    "msg_time": 1718980832156
	  }]
	}`)
	var body msgCallbackBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.EventType != "message" || len(body.Payload) != 1 {
		t.Fatalf("unexpected body: %+v", body)
	}
	msg := body.Payload[0]
	if msg.Sender != "userid1" || msg.Receiver != "userid2" || msg.MsgContent != "Hello, world!" {
		t.Fatalf("unexpected msg: %+v", msg)
	}
}
