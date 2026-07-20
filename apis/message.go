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

// msgCallbackBody 是 IM 推送的消息事件信封。
//
// @param event_type 事件类型，仅处理 "message"
// @param timestamp  IM 侧事件时间戳（毫秒）
// @param payload    本次推送的消息列表
type msgCallbackBody struct {
	EventType string           `json:"event_type"`
	Timestamp int64            `json:"timestamp"`
	Payload   []msgCallbackMsg `json:"payload"`
}

// msgCallbackMsg 是单条消息事件。
//
// @param platform     消息来源平台，如 Web、Bot
// @param sender       发送者 ID
// @param receiver     接收会话 ID，Ticket 群为 ticket_ 前缀
// @param conver_type  会话类型，2 表示 Ticket 群
// @param msg_type     消息类型，如 jg:text
// @param msg_content  消息体 JSON 字符串
// @param mention_info @ 信息原文
// @param msg_id       IM 消息 ID，用于幂等
// @param msg_time     消息时间戳（毫秒）
type msgCallbackMsg struct {
	Platform    string          `json:"platform"`
	Sender      string          `json:"sender"`
	Receiver    string          `json:"receiver"`
	ConverType  int             `json:"conver_type"`
	MsgType     string          `json:"msg_type"`
	MsgContent  string          `json:"msg_content"`
	MentionInfo json.RawMessage `json:"mention_info"`
	MsgID       string          `json:"msg_id"`
	MsgTime     int64           `json:"msg_time"`
}

// getAppKey 依次从上下文、Header、Query 中解析 AppKey。
func getAppKey(ctx *gin.Context) string {
	if v := ctx.GetString("appkey"); v != "" {
		return v
	}
	if v := ctx.GetHeader("appkey"); v != "" {
		return v
	}
	if v := ctx.Query("appkey"); v != "" {
		return v
	}
	return ctx.Query("app_key")
}

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
