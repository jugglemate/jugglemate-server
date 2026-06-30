package apis

import (
	"encoding/json"
	"io"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

var processWebhookMsgForAPI = services.ProcessWebhookMessage

func WebhookMsgs(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Printf("[WebhookMsgs] read request body err: %v", err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}

	var body msgCallbackBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		log.Printf("[WebhookMsgs] unmarshal err: %v, raw=%s", err, string(bodyBytes))
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	if body.EventType != "" && body.EventType != "message" {
		log.Printf("[WebhookMsgs] non-message event ignored: event_type=%s payload_count=%d", body.EventType, len(body.Payload))
		responses.SuccessHttpResp(ctx, nil)
		return
	}

	appkey := getAppKey(ctx)
	log.Printf("[WebhookMsgs] received: appkey=%s event_type=%s timestamp=%d payload_count=%d", appkey, body.EventType, body.Timestamp, len(body.Payload))
	for i, item := range body.Payload {
		log.Printf("[WebhookMsgs] [%d/%d] platform=%s sender=%s receiver=%s conver_type=%d msg_type=%s msg_id=%s msg_time=%d",
			i+1,
			len(body.Payload),
			item.Platform,
			item.Sender,
			item.Receiver,
			item.ConverType,
			item.MsgType,
			item.MsgID,
			item.MsgTime,
		)
		code := processWebhookMsgForAPI(appkey, services.WebhookMessagePayload{
			Platform:    item.Platform,
			Sender:      item.Sender,
			Receiver:    item.Receiver,
			ConverType:  item.ConverType,
			MsgType:     item.MsgType,
			MsgContent:  item.MsgContent,
			MentionInfo: item.MentionInfo,
			MsgID:       item.MsgID,
			MsgTime:     item.MsgTime,
		})
		if code != errs.IMErrorCode_SUCCESS {
			log.Printf("[WebhookMsgs] [%d/%d] payload processing failed code=%d sender=%s receiver=%s conver_type=%d msg_type=%s msg_id=%s",
				i+1, len(body.Payload), code, item.Sender, item.Receiver, item.ConverType, item.MsgType, item.MsgID)
		}
	}
	responses.SuccessHttpResp(ctx, nil)
}
