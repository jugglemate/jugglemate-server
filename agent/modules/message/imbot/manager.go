package imbot

import (
	"context"
	"encoding/json"
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
	stop    chan struct{}
	// stopOnce 保证 Stop 可重入：模块关闭与测试清理都可能调用它。
	stopOnce sync.Once
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
	return &Manager{config: config, enabled: enabled, clients: map[string]*imbotclients.ImBotClient{}, stop: make(chan struct{})}
}

// Start 连接数据库中全部 active Bot；单个连接失败不阻止服务启动。
func (manager *Manager) Start(ctx context.Context, db *gorm.DB) error {
	if !manager.enabled {
		slog.WarnContext(ctx, "[IMBot] 模块已禁用，不会建立任何 Bot 连接，Agent 无法收到 Ticket 群消息", "config_enabled", false)
		return nil
	}
	slog.InfoContext(ctx, "[IMBot] 开始建立 Bot 常驻连接", "ws_address", manager.config.WSAddress)
	if problem := validateWSAddress(manager.config.WSAddress); problem != "" {
		slog.ErrorContext(ctx, "[IMBot] wsAddress 配置非法，所有 Bot 连接都将失败", "ws_address", manager.config.WSAddress, "problem", problem)
	}
	var bots []struct {
		AppKey    string `gorm:"column:app_key"`
		BotUserID string `gorm:"column:bot_user_id"`
		Token     string `gorm:"column:token"`
	}
	if err := db.WithContext(ctx).Table("bots").Select("app_key, bot_user_id, token").Where("status = 'active'").Find(&bots).Error; err != nil {
		slog.ErrorContext(ctx, "[IMBot] 查询 active Bot 失败", "error", err)
		return err
	}
	slog.InfoContext(ctx, "[IMBot] active Bot 查询完成", "total", len(bots))
	if len(bots) == 0 {
		slog.WarnContext(ctx, "[IMBot] bots 表没有 status='active' 的记录，Agent 不会收到任何 Ticket 群消息")
	}
	connected, skipped, failed := 0, 0, 0
	for _, bot := range bots {
		if bot.Token == "" {
			skipped++
			slog.WarnContext(ctx, "[IMBot] 跳过：Bot token 为空，无法建立连接", "app_key", bot.AppKey, "bot_user_id", bot.BotUserID)
			continue
		}
		if _, err := manager.EnsureBot(ctx, bot.AppKey, bot.Token, bot.BotUserID); err != nil {
			failed++
			slog.ErrorContext(ctx, "[IMBot] 业务 IM Bot 连接失败", "app_key", bot.AppKey, "bot_user_id", bot.BotUserID, "error", err)
			continue
		}
		connected++
		slog.InfoContext(ctx, "[IMBot] Bot 连接成功", "app_key", bot.AppKey, "bot_user_id", bot.BotUserID)
	}
	slog.InfoContext(ctx, "[IMBot] Bot 连接建立完毕", "connected", connected, "skipped_no_token", skipped, "failed", failed)
	go manager.watchConnections()
	return nil
}

// ConnectionState 返回指定 Bot 长连接的当前状态，用于入站链路诊断。
//
// TIPS: SDK 内部有自动重连，连接掉线后 GetState() 会在 disconnected/connecting 之间跳变，
// 但 Manager 的 clients 里仍然留着这个客户端对象。因此"有 client"不等于"能收消息"，
// 排查 Bot 不回消息时必须看状态而不是看有没有注册。
func (manager *Manager) ConnectionState(appKey, botUserID string) string {
	if manager == nil {
		return "no_manager"
	}
	if !manager.enabled {
		return "disabled"
	}
	manager.mu.RLock()
	client := manager.clients[clientKey(appKey, botUserID)]
	manager.mu.RUnlock()
	if client == nil {
		return "no_client"
	}
	return describeState(client.GetState())
}

func describeState(state utils.ConnectState) string {
	switch state {
	case utils.State_connected:
		return "connected"
	case utils.State_Connecting:
		return "connecting"
	case utils.State_Disconnect:
		return "disconnected"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

// watchConnections 周期性输出全部 Bot 长连接状态，便于事后对照消息时间点排查掉线。
func (manager *Manager) watchConnections() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-manager.stop:
			return
		case <-ticker.C:
			manager.mu.RLock()
			states := make(map[string]string, len(manager.clients))
			for key, client := range manager.clients {
				states[key] = describeState(client.GetState())
			}
			manager.mu.RUnlock()
			connected := 0
			details := make([]string, 0, len(states))
			for key, state := range states {
				if state == "connected" {
					connected++
				}
				details = append(details, key+"="+state)
			}
			slog.Info("[IMBot] 长连接巡检", "total", len(states), "connected", connected, "details", strings.Join(details, ","))
		}
	}
}

// EnsureBot 确保指定 Bot 已连接，并返回握手确认的用户 ID。
func (manager *Manager) EnsureBot(ctx context.Context, appKey, token, expectedUserID string) (string, error) {
	if !manager.enabled {
		slog.WarnContext(ctx, "[IMBot] EnsureBot 跳过：模块已禁用", "app_key", appKey, "bot_user_id", expectedUserID)
		return expectedUserID, nil
	}
	if token == "" || appKey == "" || manager.config.WSAddress == "" {
		slog.ErrorContext(ctx, "[IMBot] EnsureBot 前置校验失败", "app_key", appKey, "bot_user_id", expectedUserID,
			"has_token", token != "", "ws_address", manager.config.WSAddress)
		return "", &Error{Status: 500, Code: "500_IMBOT_CREDENTIALS_NOT_CONFIGURED", Message: "IM Bot WebSocket 配置或 token 缺失"}
	}
	manager.mu.RLock()
	existing := manager.clients[clientKey(appKey, expectedUserID)]
	manager.mu.RUnlock()
	if existing != nil && existing.GetState() == utils.State_connected {
		slog.InfoContext(ctx, "[IMBot] EnsureBot 复用已有连接", "app_key", appKey, "bot_user_id", expectedUserID)
		return expectedUserID, nil
	}
	slog.InfoContext(ctx, "[IMBot] 正在连接 Bot", "app_key", appKey, "bot_user_id", expectedUserID, "ws_address", manager.config.WSAddress)
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
		slog.InfoContext(ctx, "[IMBot] EnsureBot 连接完成", "app_key", appKey, "bot_user_id", response.userID,
			"state", describeState(client.GetState()), "replaced_old_client", old != nil && old != client)
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

// SendCustomMessage 发送任意 msg_type 的自定义消息。
//
// TIPS: 用来发业务自定义协议（如 jgm:csat 邀请卡 / jgm:ticketassign 通知），
// IM server 端按 msg_type 路由分发。msg_content 必须是可以被 JSON 序列化的对象。
func (manager *Manager) SendCustomMessage(ctx context.Context, appKey, botUserID, targetID string, channelType pbobjs.ChannelType, msgType string, msgContent interface{}) (string, error) {
	client, err := manager.client(appKey, botUserID)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(msgContent)
	if err != nil {
		return "", err
	}
	up := &pbobjs.UpMsg{
		MsgType:    msgType,
		MsgContent: raw,
		Flags:      0,
		ClientUid:  fmt.Sprintf("agent-%s-%d", msgType, time.Now().UnixNano()),
	}
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
			return "", fmt.Errorf("发送自定义 IM 消息失败 msg_type=%s code=%d", msgType, response.code)
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
	if message == nil {
		slog.Warn("[IMBot] 收到空消息", "bot_user_id", listener.botUserID)
		return
	}
	targetLog, channelLog := "", pbobjs.ChannelType_Private
	if message.Conversation != nil {
		targetLog, channelLog = message.Conversation.ConversationId, message.Conversation.ConversationType
	}
	slog.Info("[IMBot] 收到消息", "app_key", listener.appKey, "bot_user_id", listener.botUserID,
		"sender_id", message.SenderId, "msg_id", message.MsgId, "target_id", targetLog, "channel_type", int(channelLog))
	if message.SenderId == "" || message.SenderId == listener.botUserID {
		slog.Info("[IMBot] 跳过：发送者为空或是 Bot 自己", "bot_user_id", listener.botUserID, "sender_id", message.SenderId, "msg_id", message.MsgId)
		return
	}
	textContent, ok := message.MsgContent.(*messages.TextMessage)
	if !ok {
		slog.Info("[IMBot] 跳过：非文本消息", "bot_user_id", listener.botUserID, "msg_id", message.MsgId,
			"content_type", fmt.Sprintf("%T", message.MsgContent))
		return
	}
	if strings.TrimSpace(textContent.Content) == "" {
		slog.Info("[IMBot] 跳过：文本内容为空", "bot_user_id", listener.botUserID, "msg_id", message.MsgId)
		return
	}
	listener.manager.mu.RLock()
	handler := listener.manager.handler
	listener.manager.mu.RUnlock()
	if handler == nil {
		slog.Error("[IMBot] 跳过：入站处理器未注册", "bot_user_id", listener.botUserID, "msg_id", message.MsgId)
	}
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

// validateWSAddress 校验 wsAddress 是否符合 SDK 要求；合法返回空串，否则返回问题描述。
//
// TIPS: imbot SDK 的 wsURL() 把 scheme 之后的整段当作 URL.Host 并硬编码 Path="/imbot"。
// 因此 wsAddress 只能是 host[:port]，一旦带路径（如 wss://ws.juggleim.com/im），那个 "/"
// 会被转义成 %2F，最终报 `invalid URL escape "%2F"`，且错误只出现在 SDK 自己的输出里，
// 从业务日志完全看不出来。这里在启动时就把问题点破。
func validateWSAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return "为空"
	}
	rest := ""
	switch {
	case strings.HasPrefix(address, "wss://"):
		rest = address[6:]
	case strings.HasPrefix(address, "ws://"):
		rest = address[5:]
	default:
		return "缺少 ws:// 或 wss:// 前缀"
	}
	if rest == "" {
		return "缺少主机名"
	}
	if strings.ContainsAny(rest, "/?#") {
		return "只能填 host[:port]，不能带路径或查询参数（SDK 会固定拼接 /imbot，路径中的 / 会被转义成 %2F 导致连接失败）"
	}
	return ""
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

// DisconnectBot 断开单个 Bot 的常驻连接，未连接时安全返回。
//
// TIPS: 删除 Agent 时调用。Bot 连接是进程启动时按 bots.status='active' 建立的，只把库里
// 状态置为 inactive 并不会让已建立的连接消失 —— 那个 Bot 会继续在群里收消息（虽然已经路由
// 不到任何 Agent），直到下次重启才消失。
func (manager *Manager) DisconnectBot(appKey, botUserID string) {
	if manager == nil || !manager.enabled {
		return
	}
	key := clientKey(appKey, botUserID)
	manager.mu.Lock()
	client := manager.clients[key]
	delete(manager.clients, key)
	manager.mu.Unlock()
	if client != nil {
		client.Disconnect()
	}
}

// Stop 主动断开全部 Bot，SDK 不再自动重连。
func (manager *Manager) Stop() {
	manager.stopOnce.Do(func() { close(manager.stop) })
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
