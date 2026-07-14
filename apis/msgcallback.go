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
	"github.com/juggleim/jugglemate-server/commons/agentclient"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
	"github.com/juggleim/jugglemate-server/storages"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
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

// MsgCallback 接收 IM 服务直接推送的消息事件并进入 Agent 自动回复链路。
func MsgCallback(ctx *gin.Context) {
	handleMsgCallback(ctx, "im")
}

// MsgCallbackForward 接收节点转发的消息事件并复用同一处理链路。
func MsgCallbackForward(ctx *gin.Context) {
	handleMsgCallback(ctx, "node")
}

func handleMsgCallback(ctx *gin.Context, source string) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Printf("[MsgCallback:%s] read request body err: %v", source, err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	ctx.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	var body msgCallbackBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		log.Printf("[MsgCallback:%s] unmarshal err: %v, raw=%s", source, err, string(bodyBytes))
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	if body.EventType != "message" {
		log.Printf("[MsgCallback:%s] non-message event ignored: event_type=%s, payload_count=%d", source, body.EventType, len(body.Payload))
		responses.SuccessHttpResp(ctx, nil)
		return
	}

	appkey := getAppKey(ctx)
	log.Printf("[MsgCallback:%s] received: appkey=%s, event_type=%s, payload_count=%d", source, appkey, body.EventType, len(body.Payload))

	storage := storages.NewAgentStorage()
	sdk := imsdk.GetImSdk(appkey)
	client := agentclient.New(agentclient.Config{})
	// use detached context so chat calls survive beyond request timeout
	chatCtx := context.Background()
	for i, item := range body.Payload {
		log.Printf("[MsgCallback:%s] [%d/%d] msg_type=%s, sender=%s, receiver=%s, conver_type=%d, msg_id=%s, content_len=%d",
			source,
			i+1, len(body.Payload), item.MsgType, item.Sender, item.Receiver, item.ConverType, item.MsgID, len(item.MsgContent))

		if item.MsgType != "text" {
			log.Printf("[MsgCallback:%s] [%d] skip: msg_type=%s not text", source, i+1, item.MsgType)
			continue
		}
		if item.MsgContent == "" || item.MsgID == "" {
			log.Printf("[MsgCallback:%s] [%d] skip: empty content or msg_id", source, i+1)
			continue
		}

		// 按 session_id 规则构造：会话ID + "_" + 会话类型
		sessionId := item.Sender + "_" + fmt.Sprint(item.ConverType)
		sess, err := storage.FindSession(appkey, sessionId)
		if err != nil || sess == nil || sess.UniqueName == "" {
			log.Printf("[MsgCallback:%s] [%d] skip: session not found or no agent bound, session_id=%s, err=%v", source, i+1, sessionId, err)
			continue
		}
		log.Printf("[MsgCallback:%s] [%d] session found: session_id=%s, unique_name=%s, auto_mode=%d", source, i+1, sess.SessionId, sess.UniqueName, sess.AutoMode)

		// 验证 twin 存在
		twin, err := storage.FindTwin(appkey, sess.UniqueName)
		if err != nil || twin == nil {
			log.Printf("[MsgCallback:%s] [%d] skip: FindTwin(unique_name=%s) not found, err=%v", source, i+1, sess.UniqueName, err)
			continue
		}
		log.Printf("[MsgCallback:%s] [%d] twin found: unique_name=%s, bot_id=%s", source, i+1, twin.UniqueName, twin.BotId)

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
			log.Printf("[MsgCallback:%s] [%d] skip: idempotent (customer msg exists for msg_id=%s)", source, i+1, item.MsgID)
			continue
		}

		// 保存用户消息
		sessionID := sess.SessionId
		autoMode := sess.AutoMode
		_, saveErr := services.MsgCallbackService.SaveMessage(appkey, sess.UniqueName, item.Sender, sessionID, item.MsgID, "customer", item.MsgContent, item.Platform)
		if saveErr != nil {
			log.Printf("[MsgCallback:%s] [%d] SaveMessage(customer) failed: %v", source, i+1, saveErr)
		}

		// auto_mode 路由：0=人工（不调大模型），1=协助（调大模型+通知运营），2=自动（调大模型+发用户）
		if autoMode == 0 {
			log.Printf("[MsgCallback:%s] [%d] skip: auto_mode=0 (manual), no LLM call", source, i+1)
			continue
		}

		log.Printf("[MsgCallback:%s] [%d] calling agent Chat: sender=%s, unique_name=%s, text=%q",
			source, i+1, item.Sender, twin.UniqueName, item.MsgContent)
		resp, _, chatErr := client.Chat(chatCtx, item.Sender, "jim", twin.UniqueName, agentclient.ChatRequest{Message: item.MsgContent})
		fallback := true
		reply := "抱歉，助手暂时无法回复，请稍后再试。"
		if chatErr != nil {
			log.Printf("[MsgCallback:%s] [%d] agent Chat error: %v", source, i+1, chatErr)
		} else if resp == nil {
			log.Printf("[MsgCallback:%s] [%d] agent Chat returned nil response", source, i+1)
		} else {
			log.Printf("[MsgCallback:%s] [%d] agent Chat success: reply=%q, fallback=%v, message_id=%s", source, i+1, resp.Reply, resp.Fallback, resp.MessageID)
			if resp.Reply != "" {
				reply = resp.Reply
			}
			fallback = resp.Fallback
		}
		agentMsgID := ""
		if resp != nil {
			agentMsgID = resp.MessageID
		}

		// pending 标识：assist=协助模式建议，auto=自动模式已发送
		isAuto := autoMode == 2
		pendingSource := "assist"
		if isAuto {
			pendingSource = "auto"
		}

		msgRecord := storageModels.AgentMessage{
			AppKey:           appkey,
			UniqueName:       twin.UniqueName,
			CustomerId:       item.Sender,
			IMMsgId:          item.MsgID,
			AgentMessageId:   agentMsgID,
			SessionId:        sessionID,
			Role:             "twin",
			Text:             reply,
			Fallback:         fallback,
			Source:           "agent",
			Platform:         item.Platform,
			ConverType:       item.ConverType,
			RawPayload:       string(bodyBytes),
			SuggestionStatus: "pending",
			PendingSource:    pendingSource,
			MsgTime:          item.MsgTime,
		}
		if err := storage.CreateMessage(msgRecord); err != nil {
			log.Printf("[MsgCallback:%s] [%d] CreateMessage(twin) failed: %v", source, i+1, err)
		} else {
			log.Printf("[MsgCallback:%s] [%d] twin message saved: session_id=%s, pending_source=%s", source, i+1, sessionID, pendingSource)
		}

		if sdk != nil {
			if isAuto {
				// 自动模式：以客服身份回复用户
				log.Printf("[MsgCallback:%s] [%d] auto mode: sending reply to customer", source, i+1)
				sendResp, _, sendErr := sdk.SendPrivateMsg(juggleimsdk.Message{
					SenderId:   item.Receiver,
					TargetIds:  []string{item.Sender},
					MsgType:    "jg:text",
					MsgContent: fmt.Sprintf(`{"content":%q}`, reply),
					IsStorage:  boolPtr(true),
					IsCount:    boolPtr(true),
				})
				if sendErr != nil {
					log.Printf("[MsgCallback:%s] [%d] SendPrivateMsg(customer) failed: resp=%v, err=%v", source, i+1, sendResp, sendErr)
				} else {
					log.Printf("[MsgCallback:%s] [%d] reply sent via IM: sender=%s -> target=%s", source, i+1, item.Receiver, item.Sender)
				}
			} else {
				// 协助模式：以 agentMsgBot 身份通知运营，消息体嵌入 session 标记
				log.Printf("[MsgCallback:%s] [%d] assist mode: sending IM notification to operator via agentMsgBot", source, i+1)
				notifyContent := fmt.Sprintf("[session:%s|agent:%s] %s", sessionID, twin.UniqueName, reply)
				sendResp, _, sendErr := sdk.SendPrivateMsg(juggleimsdk.Message{
					SenderId:   "agentMsgBot",
					TargetIds:  []string{item.Receiver},
					MsgType:    "jg:text",
					MsgContent: fmt.Sprintf(`{"content":%q}`, notifyContent),
					IsStorage:  boolPtr(true),
					IsCount:    boolPtr(true),
				})
				if sendErr != nil {
					log.Printf("[MsgCallback:%s] [%d] SendPrivateMsg(operator) failed: resp=%v, err=%v", source, i+1, sendResp, sendErr)
				} else {
					log.Printf("[MsgCallback:%s] [%d] notification sent via IM: sender=agentMsgBot -> target=%s", source, i+1, item.Receiver)
				}
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
