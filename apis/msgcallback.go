package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglechat-server-ai/commons/agentclient"
	"github.com/juggleim/jugglechat-server-ai/commons/agentconfig"
	"github.com/juggleim/jugglechat-server-ai/commons/imsdk"
	"github.com/juggleim/jugglechat-server-ai/commons/responses"
	"github.com/juggleim/jugglechat-server-ai/services"
	"github.com/juggleim/jugglechat-server-ai/storages"
	storageModels "github.com/juggleim/jugglechat-server-ai/storages/models"
)

type msgCallbackBody struct {
	EventType string           `json:"event_type"`
	Timestamp int64            `json:"timestamp"`
	Payload   []msgCallbackMsg `json:"payload"`
}

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

func MsgCallback(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Printf("[MsgCallback] read request body err: %v", err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	ctx.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	var body msgCallbackBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		log.Printf("[MsgCallback] unmarshal err: %v, raw=%s", err, string(bodyBytes))
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	if body.EventType != "message" {
		log.Printf("[MsgCallback] non-message event ignored: event_type=%s, payload_count=%d", body.EventType, len(body.Payload))
		responses.SuccessHttpResp(ctx, nil)
		return
	}

	appkey := getAppKey(ctx)
	log.Printf("[MsgCallback] received: appkey=%s, event_type=%s, payload_count=%d", appkey, body.EventType, len(body.Payload))

	storage := storages.NewAgentStorage()
	sdk := imsdk.GetImSdk(appkey)
	client := agentclient.New(agentclient.Config{
		BaseURL:      strings.TrimRight(agentconfig.BaseURL(), "/"),
		Timeout:      agentconfig.Timeout(),
		Authorization: agentconfig.Authorization(),
	})
	// use detached context so chat calls survive beyond request timeout
	chatCtx := context.Background()
	for i, item := range body.Payload {
		log.Printf("[MsgCallback] [%d/%d] msg_type=%s, sender=%s, receiver=%s, conver_type=%d, msg_id=%s, content_len=%d",
			i+1, len(body.Payload), item.MsgType, item.Sender, item.Receiver, item.ConverType, item.MsgID, len(item.MsgContent))

		if item.MsgType != "text" {
			log.Printf("[MsgCallback] [%d] skip: msg_type=%s not text", i+1, item.MsgType)
			continue
		}
		if item.MsgContent == "" || item.MsgID == "" {
			log.Printf("[MsgCallback] [%d] skip: empty content or msg_id", i+1)
			continue
		}
		twin, err := storage.FindTwinByAnyKey(appkey, item.Receiver)
		if err != nil || twin == nil {
			log.Printf("[MsgCallback] [%d] skip: FindTwinByAnyKey(receiver=%s) twin not found, err=%v", i+1, item.Receiver, err)
			continue
		}
		log.Printf("[MsgCallback] [%d] found twin: unique_name=%s, bot_id=%s", i+1, twin.UniqueName, twin.BotId)

		// 幂等：检查是否已处理过（customer 消息存在即跳过）
		existMsgs, _ := storage.QryMessagesByIM(appkey, item.MsgID)
		alreadyDone := false
		for _, msg := range existMsgs {
			if msg.Role == "customer" {
				alreadyDone = true
				break
			}
		}
		if alreadyDone {
			log.Printf("[MsgCallback] [%d] skip: idempotent (customer msg exists for msg_id=%s)", i+1, item.MsgID)
			continue
		}

		// 会话管理：按 IM 会话维度（receiver 作为会话 ID）查找或创建会话
		sess, sessErr := services.MsgCallbackService.FindOrCreateSession(appkey, twin.UniqueName, item.Sender, item.Platform, item.Receiver)
		if sessErr != nil {
			log.Printf("[MsgCallback] [%d] FindOrCreateSession failed: unique_name=%s, sender=%s, platform_conv_id=%s, err=%v",
				i+1, twin.UniqueName, item.Sender, item.Receiver, sessErr)
		}
		if sess != nil {
			log.Printf("[MsgCallback] [%d] session: session_id=%s, auto_mode=%d, status=%d", i+1, sess.SessionId, sess.AutoMode, sess.Status)
			_, saveErr := services.MsgCallbackService.SaveMessage(appkey, twin.UniqueName, item.Sender, sess.SessionId, item.MsgID, "customer", item.MsgContent, item.Platform)
			if saveErr != nil {
				log.Printf("[MsgCallback] [%d] SaveMessage(customer) failed: %v", i+1, saveErr)
			}
		} else {
			log.Printf("[MsgCallback] [%d] session is nil, proceeding without session", i+1)
		}

		log.Printf("[MsgCallback] [%d] calling agent Chat: sender=%s, unique_name=%s, text=%q",
			i+1, item.Sender, twin.UniqueName, item.MsgContent)
		resp, _, chatErr := client.Chat(chatCtx, item.Sender, "jim", twin.UniqueName, agentclient.ChatRequest{Message: item.MsgContent})
		fallback := true
		reply := "抱歉，助手暂时无法回复，请稍后再试。"
		if chatErr != nil {
			log.Printf("[MsgCallback] [%d] agent Chat error: %v", i+1, chatErr)
		} else if resp == nil {
			log.Printf("[MsgCallback] [%d] agent Chat returned nil response", i+1)
		} else {
			log.Printf("[MsgCallback] [%d] agent Chat success: reply_len=%d, fallback=%v", i+1, len(resp.Reply), resp.Fallback)
			reply = resp.Reply
			fallback = resp.Fallback
		}
		agentMsgID := ""
		sessionID := ""
		if resp != nil {
			agentMsgID = resp.MessageID
			sessionID = resp.SessionID
		}
		if sess != nil {
			sessionID = sess.SessionId
		}

		// takeover 判断：auto_mode=1 → 直接发IM；auto_mode=0 → 存pending建议不发送
		shouldSend := true
		suggestionStatus := ""
		if sess == nil || services.MsgCallbackService.ShouldAutoReply(sess) {
			// auto mode: 直接回复
			log.Printf("[MsgCallback] [%d] auto mode: sending reply via IM", i+1)
		} else {
			// manual mode: 存为pending，不发IM
			shouldSend = false
			suggestionStatus = "pending"
			log.Printf("[MsgCallback] [%d] manual mode: storing as pending suggestion", i+1)
		}

		msgRecord := storageModels.AgentMessage{
			AppKey:            appkey,
			UniqueName:        twin.UniqueName,
			CustomerId:        item.Sender,
			IMMsgId:           item.MsgID,
			AgentMessageId:    agentMsgID,
			SessionId:         sessionID,
			Role:              "twin",
			Text:              reply,
			Fallback:          fallback,
			Source:            "agent",
			Platform:          item.Platform,
			ConverType:        item.ConverType,
			RawPayload:        string(bodyBytes),
			SuggestionStatus:  suggestionStatus,
			MsgTime:           item.MsgTime,
		}
		if err := storage.CreateMessage(msgRecord); err != nil {
			log.Printf("[MsgCallback] [%d] CreateMessage(twin) failed: %v", i+1, err)
		} else {
			log.Printf("[MsgCallback] [%d] twin message saved: session_id=%s, status=%s", i+1, sessionID, suggestionStatus)
		}

		if shouldSend && sdk != nil {
			sendResp, _, sendErr := sdk.SendPrivateMsg(juggleimsdk.Message{
				SenderId:   twin.BotId,
				TargetIds:  []string{item.Sender},
				MsgType:    "jg:text",
				MsgContent: fmt.Sprintf(`{"content":%q}`, reply),
				IsStorage:  boolPtr(true),
				IsCount:    boolPtr(true),
			})
			if sendErr != nil {
				log.Printf("[MsgCallback] [%d] SendPrivateMsg failed: resp=%v, err=%v", i+1, sendResp, sendErr)
			} else {
				log.Printf("[MsgCallback] [%d] reply sent via IM: sender=%s -> target=%s", i+1, twin.BotId, item.Sender)
			}
		}
	}

	responses.SuccessHttpResp(ctx, nil)
}

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

func boolPtr(v bool) *bool {
	return &v
}
