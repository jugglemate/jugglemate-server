package imbot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/juggleim/imbot-sdk-go/imbotclients"
	"github.com/juggleim/imbot-sdk-go/imbotclients/pbdefines/pbobjs"
	sdkmodels "github.com/juggleim/imbot-sdk-go/models"
	"github.com/juggleim/imbot-sdk-go/models/messages"
	"github.com/juggleim/imbot-sdk-go/utils"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"gorm.io/gorm"
)

// Manager 为每个 active Bot 维护一个支持自动重连的 SDK 客户端。
type Manager struct {
	config  configures.AgentIMConfig
	enabled bool
	mu      sync.RWMutex
	clients map[string]*imbotclients.ImBotClient
	handler func(context.Context, InboundMessage)
}

// InboundMessage 表示从 SDK 收到并完成基础归一化的消息。
type InboundMessage struct {
	AppKey      string
	BotUserID   string
	SenderID    string
	MessageID   string
	Text        string
	TargetID    string
	ChannelType pbobjs.ChannelType
}

// NewManager 创建 IM Bot 连接管理器。
func NewManager(config configures.AgentIMConfig) *Manager {
	enabled := true
	if config.Enabled != nil {
		enabled = *config.Enabled
	}
	return &Manager{config: config, enabled: enabled, clients: map[string]*imbotclients.ImBotClient{}}
}

// Start 连接数据库中全部 active Bot；单个连接失败不阻止服务启动。
func (manager *Manager) Start(ctx context.Context, db *gorm.DB) error {
	if !manager.enabled {
		return nil
	}
	var bots []struct {
		AppKey    string `gorm:"column:app_key"`
		BotUserID string `gorm:"column:bot_user_id"`
		Token     string `gorm:"column:token"`
	}
	if err := db.WithContext(ctx).Table("bots").Select("app_key, bot_user_id, token").Where("status = 'active'").Find(&bots).Error; err != nil {
		return err
	}
	for _, bot := range bots {
		if bot.Token == "" {
			continue
		}
		if _, err := manager.EnsureBot(ctx, bot.AppKey, bot.Token, bot.BotUserID); err != nil {
			slog.ErrorContext(ctx, "业务 IM Bot 连接失败", "app_key", bot.AppKey, "bot_user_id", bot.BotUserID, "error", err)
		}
	}
	return nil
}

// EnsureBot 确保指定 Bot 已连接，并返回握手确认的用户 ID。
func (manager *Manager) EnsureBot(ctx context.Context, appKey, token, expectedUserID string) (string, error) {
	if !manager.enabled {
		return expectedUserID, nil
	}
	if token == "" || appKey == "" || manager.config.WSAddress == "" {
		return "", &Error{Status: 500, Code: "500_IMBOT_CREDENTIALS_NOT_CONFIGURED", Message: "IM Bot WebSocket 配置或 token 缺失"}
	}
	manager.mu.RLock()
	existing := manager.clients[clientKey(appKey, expectedUserID)]
	manager.mu.RUnlock()
	if existing != nil && existing.GetState() == utils.State_connected {
		return expectedUserID, nil
	}
	client := imbotclients.NewImBotClient(manager.config.WSAddress, appKey)
	type connectResult struct {
		userID string
		err    error
	}
	result := make(chan connectResult, 1)
	go func() {
		code, ack := client.Connect(token)
		if code != utils.ClientErrorCode_Success || ack == nil {
			result <- connectResult{err: fmt.Errorf("Bot 连接失败 code=%d", code)}
			return
		}
		result <- connectResult{userID: ack.GetUserId()}
	}()
	select {
	case <-ctx.Done():
		go func() { response := <-result; _ = response; client.Disconnect() }()
		return "", ctx.Err()
	case response := <-result:
		if response.err != nil {
			return "", response.err
		}
		if expectedUserID != "" && expectedUserID != response.userID {
			client.Disconnect()
			return "", fmt.Errorf("Bot 连接身份不匹配 expected=%s actual=%s", expectedUserID, response.userID)
		}
		manager.mu.Lock()
		key := clientKey(appKey, response.userID)
		old := manager.clients[key]
		client.AddMessageListener(&messageListener{manager: manager, appKey: appKey, botUserID: response.userID})
		manager.clients[key] = client
		manager.mu.Unlock()
		if old != nil && old != client {
			old.Disconnect()
		}
		return response.userID, nil
	}
}

// SetInboundHandler 设置所有 Bot 连接共享的入站消息处理器。
func (manager *Manager) SetInboundHandler(handler func(context.Context, InboundMessage)) {
	manager.mu.Lock()
	manager.handler = handler
	manager.mu.Unlock()
}

// SendText 使用指定应用的 Bot 向目标会话发送普通文本消息，并返回平台消息 ID。
func (manager *Manager) SendText(ctx context.Context, appKey, botUserID, targetID string, channelType pbobjs.ChannelType, text string) (string, error) {
	client, err := manager.client(appKey, botUserID)
	if err != nil {
		return "", err
	}
	content := messages.NewTextMessage(text)
	raw, err := content.Encode()
	if err != nil {
		return "", err
	}
	up := &pbobjs.UpMsg{MsgType: content.GetContentType(), MsgContent: raw, Flags: int32(content.GetFlags()), ClientUid: "agent-" + fmt.Sprint(time.Now().UnixNano()), SearchText: content.GetSearchContent()}
	type result struct {
		code utils.ClientErrorCode
		ack  *pbobjs.PublishAckMsgBody
	}
	done := make(chan result, 1)
	go func() {
		code, ack := client.SendMessage(&sdkmodels.Conversation{ConversationId: targetID, ConversationType: channelType}, up)
		done <- result{code: code, ack: ack}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case response := <-done:
		if response.code != utils.ClientErrorCode_Success || response.ack == nil {
			return "", fmt.Errorf("发送 IM 消息失败 code=%d", response.code)
		}
		return response.ack.GetMsgId(), nil
	}
}

// SendStreamText 首发一条可编辑的流式文本消息。
func (manager *Manager) SendStreamText(ctx context.Context, appKey, botUserID, targetID string, channelType pbobjs.ChannelType, text string, sequence int, finished bool) (string, error) {
	client, err := manager.client(appKey, botUserID)
	if err != nil {
		return "", err
	}
	content := messages.NewStreamTextMessage()
	content.Content, content.Seq, content.IsFinished = text, sequence, finished
	raw, err := content.Encode()
	if err != nil {
		return "", err
	}
	up := &pbobjs.UpMsg{MsgType: content.GetContentType(), MsgContent: raw, Flags: int32(content.GetFlags()), ClientUid: "agent-stream-" + fmt.Sprint(time.Now().UnixNano()), SearchText: content.GetSearchContent()}
	type result struct {
		code utils.ClientErrorCode
		ack  *pbobjs.PublishAckMsgBody
	}
	done := make(chan result, 1)
	go func() {
		code, ack := client.SendMessage(&sdkmodels.Conversation{ConversationId: targetID, ConversationType: channelType}, up)
		done <- result{code: code, ack: ack}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case response := <-done:
		if response.code != utils.ClientErrorCode_Success || response.ack == nil {
			return "", fmt.Errorf("发送 IM 流式消息失败 code=%d", response.code)
		}
		return response.ack.GetMsgId(), nil
	}
}

// ModifyStreamText 更新已发送的流式文本并可将其标记为完成。
func (manager *Manager) ModifyStreamText(ctx context.Context, appKey, botUserID, targetID string, channelType pbobjs.ChannelType, messageID, text string, sequence int, finished bool) error {
	client, err := manager.client(appKey, botUserID)
	if err != nil {
		return err
	}
	content := messages.NewStreamTextMessage()
	content.Content, content.Seq, content.IsFinished = text, sequence, finished
	raw, err := content.Encode()
	if err != nil {
		return err
	}
	request := &pbobjs.ModifyMsgReq{TargetId: targetID, ChannelType: channelType, MsgId: messageID, MsgType: content.GetContentType(), MsgContent: raw}
	done := make(chan utils.ClientErrorCode, 1)
	go func() { code, _ := client.ModifyMsg(request); done <- code }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case code := <-done:
		if code != utils.ClientErrorCode_Success {
			return fmt.Errorf("编辑 IM 流式消息失败 code=%d", code)
		}
		return nil
	}
}

func (manager *Manager) client(appKey, botUserID string) (*imbotclients.ImBotClient, error) {
	manager.mu.RLock()
	client := manager.clients[clientKey(appKey, botUserID)]
	manager.mu.RUnlock()
	if client == nil {
		return nil, &Error{Status: 503, Code: "503_NO_BOT_CONNECTION", Message: "无可用 Bot 连接"}
	}
	return client, nil
}

type messageListener struct {
	manager   *Manager
	appKey    string
	botUserID string
}

// OnMessageReceive 接收 IM SDK 消息并投递到 Go Message 入站处理器。
func (listener *messageListener) OnMessageReceive(message *sdkmodels.Message) {
	if message == nil || message.SenderId == "" || message.SenderId == listener.botUserID {
		return
	}
	textContent, ok := message.MsgContent.(*messages.TextMessage)
	if !ok || strings.TrimSpace(textContent.Content) == "" {
		return
	}
	listener.manager.mu.RLock()
	handler := listener.manager.handler
	listener.manager.mu.RUnlock()
	if handler != nil {
		targetID := ""
		channel := pbobjs.ChannelType_Private
		if message.Conversation != nil {
			targetID = message.Conversation.ConversationId
			channel = message.Conversation.ConversationType
		}
		go handler(context.Background(), InboundMessage{AppKey: listener.appKey, BotUserID: listener.botUserID, SenderID: message.SenderId, MessageID: message.MsgId, Text: textContent.Content, TargetID: targetID, ChannelType: channel})
	}
}

func clientKey(appKey, botUserID string) string {
	return strings.TrimSpace(appKey) + ":" + strings.TrimSpace(botUserID)
}

// OnMessageRecall 接收消息撤回事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageRecall(*sdkmodels.Message) {}

// OnMessageUpdate 接收消息更新事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageUpdate(*sdkmodels.Message) {}

// OnMessageDelete 接收消息删除事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageDelete(*sdkmodels.Conversation, []string) {}

// OnMessageClear 接收会话清理事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageClear(*sdkmodels.Conversation, int64, string) {}

// OnMessageReactionAdd 接收表情添加事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageReactionAdd(*sdkmodels.Conversation, *sdkmodels.MessageReaction) {}

// OnMessageReactionRemove 接收表情移除事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageReactionRemove(*sdkmodels.Conversation, *sdkmodels.MessageReaction) {
}

// OnMessageSetTop 接收消息置顶事件；当前 Agent 入站链路无需二次处理。
func (*messageListener) OnMessageSetTop(*sdkmodels.Message, string, bool) {}

// Stop 主动断开全部 Bot，SDK 不再自动重连。
func (manager *Manager) Stop() {
	manager.mu.Lock()
	clients := make([]*imbotclients.ImBotClient, 0, len(manager.clients))
	for _, client := range manager.clients {
		clients = append(clients, client)
	}
	manager.clients = map[string]*imbotclients.ImBotClient{}
	manager.mu.Unlock()
	for _, client := range clients {
		client.Disconnect()
	}
}
