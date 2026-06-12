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
		fmt.Println("[MsgCallback] read request body err:", err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	ctx.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	var body msgCallbackBody
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		fmt.Println("[MsgCallback] unmarshal err:", err)
		responses.SuccessHttpResp(ctx, nil)
		return
	}
	if body.EventType != "message" {
		responses.SuccessHttpResp(ctx, nil)
		return
	}

	appkey := getAppKey(ctx)
	storage := storages.NewAgentStorage()
	sdk := imsdk.GetImSdk(appkey)
	client := agentclient.New(agentclient.Config{
		BaseURL:      strings.TrimRight(agentconfig.BaseURL(), "/"),
		Timeout:      agentconfig.Timeout(),
		Authorization: agentconfig.Authorization(),
	})
	// use detached context so chat calls survive beyond request timeout
	chatCtx := context.Background()
	for _, item := range body.Payload {
		if item.MsgType != "text" || item.MsgContent == "" || item.MsgID == "" {
			continue
		}
		twin, err := storage.FindTwinByAnyKey(appkey, item.Receiver)
		if err != nil || twin == nil {
			continue
		}
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
			continue
		}
		_ = storage.CreateMessage(storageModels.AgentMessage{
			AppKey:     appkey,
			UniqueName: twin.UniqueName,
			CustomerId: item.Sender,
			IMMsgId:    item.MsgID,
			Role:       "customer",
			Text:       item.MsgContent,
			Source:     "jim",
			Platform:   item.Platform,
			ConverType: item.ConverType,
			RawPayload: string(bodyBytes),
			MsgTime:    item.MsgTime,
		})

		resp, _, err := client.Chat(chatCtx, item.Sender, "jim", twin.UniqueName, agentclient.ChatRequest{Message: item.MsgContent})
		fallback := true
		reply := "抱歉，助手暂时无法回复，请稍后再试。"
		if err == nil && resp != nil && resp.Reply != "" {
			reply = resp.Reply
			fallback = resp.Fallback
		}
		agentMsgID := ""
		sessionID := ""
		if resp != nil {
			agentMsgID = resp.MessageID
			sessionID = resp.SessionID
		}
		msgRecord := storageModels.AgentMessage{
			AppKey:         appkey,
			UniqueName:     twin.UniqueName,
			CustomerId:     item.Sender,
			IMMsgId:        item.MsgID,
			AgentMessageId: agentMsgID,
			SessionId:      sessionID,
			Role:           "twin",
			Text:           reply,
			Fallback:       fallback,
			Source:         "agent",
			Platform:       item.Platform,
			ConverType:     item.ConverType,
			RawPayload:     string(bodyBytes),
			MsgTime:        item.MsgTime,
		}
		if err := storage.CreateMessage(msgRecord); err != nil {
			log.Printf("CreateMessage(twin) failed: %v", err)
		}

		if sdk != nil {
			sendResp, _, sendErr := sdk.SendPrivateMsg(juggleimsdk.Message{
				SenderId:   twin.BotId,
				TargetIds:  []string{item.Sender},
				MsgType:    "jg:text",
				MsgContent: fmt.Sprintf(`{"content":%q}`, reply),
				IsStorage:  boolPtr(true),
				IsCount:    boolPtr(true),
			})
			if sendErr != nil {
				log.Printf("SendPrivateMsg failed: resp=%v, err=%v", sendResp, sendErr)
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
