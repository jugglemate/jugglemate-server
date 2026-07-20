// Package imbot 实现 IM Bot 注册与 WebSocket 长连接管理。
package imbot

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Error 表示 IM Server 注册或连接错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回 IM Bot 错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }

// RegisteredBot 表示 IM Server 注册返回的 Bot 身份。
type RegisteredBot struct {
	BotID  string
	UserID string
	Token  string
}

// CredentialResolver 按 AppKey 返回 IM AppSecret。
type CredentialResolver func(ctx context.Context, appKey string) (string, error)

// RegisterClient 调用 IM Server API 创建 Bot。
type RegisterClient struct {
	apiBaseURL string
	resolve    CredentialResolver
	client     *http.Client
}

// NewRegisterClient 创建带 TLS、超时和动态应用凭证约束的 IM Bot 注册客户端。
func NewRegisterClient(apiBaseURL string, insecure bool, resolve CredentialResolver) *RegisterClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecure} //nolint:gosec // 仅由显式测试配置启用。
	return &RegisterClient{apiBaseURL: strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"), resolve: resolve, client: &http.Client{Timeout: 15 * time.Second, Transport: transport}}
}

// RegisterBot 使用 SHA1 签名头调用 `/apigateway/bots/register` 并校验完整响应。
func (client *RegisterClient) RegisterBot(ctx context.Context, appKey, botID, nickname string) (RegisteredBot, error) {
	if client.apiBaseURL == "" {
		return RegisteredBot{}, &Error{Status: 500, Code: "500_IMBOT_SERVER_API_NOT_CONFIGURED", Message: "IM Server API 地址未配置"}
	}
	if strings.TrimSpace(appKey) == "" || client.resolve == nil {
		return RegisteredBot{}, &Error{Status: 500, Code: "500_IMBOT_CREDENTIALS_NOT_CONFIGURED", Message: "IM AppKey/AppSecret 未配置"}
	}
	appSecret, err := client.resolve(ctx, appKey)
	if err != nil || strings.TrimSpace(appSecret) == "" {
		return RegisteredBot{}, &Error{Status: 500, Code: "500_IMBOT_CREDENTIALS_NOT_CONFIGURED", Message: "无法解析当前应用的 IM 凭证"}
	}
	var payload struct {
		UserID string `json:"user_id"`
		Token  string `json:"token"`
	}
	body := map[string]any{"bot_id": botID, "nickname": nickname, "ext_fields": map[string]any{}, "bot_settings": defaultBotSettings()}
	if err := client.callSigned(ctx, appKey, appSecret, "/apigateway/bots/register", body, &payload, "REGISTER"); err != nil {
		return RegisteredBot{}, err
	}
	if strings.TrimSpace(payload.UserID) == "" || strings.TrimSpace(payload.Token) == "" {
		return RegisteredBot{}, &Error{Status: 502, Code: "502_IMBOT_REGISTER_INCOMPLETE", Message: "Bot 注册响应缺少 user_id 或 token"}
	}
	// TIPS: 注册接口是否读取 bot_settings 未在 IM 侧文档化，这里再显式 update 一次兜底。
	// 该设置必须为 false，否则 Bot 只在被 @ 时才收到群消息 —— 客户在 Ticket 群里正常发言
	// 不会 @ 任何人，Bot 就会静默不回，且 IM 侧不产生任何错误日志，极难排查。
	if err := client.applyBotSettings(ctx, appKey, appSecret, botID); err != nil {
		return RegisteredBot{}, err
	}
	return RegisteredBot{BotID: botID, UserID: payload.UserID, Token: payload.Token}, nil
}

// defaultBotSettings 返回 Ticket 群客服场景下 Bot 必须具备的消息接收设置。
//
// @return only_mentioned 为 false，表示群内任意消息都投递给 Bot，而非仅在被 @ 时
func defaultBotSettings() map[string]any {
	return map[string]any{"only_mentioned": false}
}

// applyBotSettings 调用 `/apigateway/bots/update` 显式写入 Bot 的消息接收设置。
func (client *RegisterClient) applyBotSettings(ctx context.Context, appKey, appSecret, botID string) error {
	body := map[string]any{"bot_id": botID, "bot_settings": defaultBotSettings()}
	return client.callSigned(ctx, appKey, appSecret, "/apigateway/bots/update", body, nil, "SETTINGS")
}

// callSigned 以 IM Server 的 SHA1 签名协议发起请求，并把响应 data 解析到 out（out 可为 nil）。
//
// @param stage 出错时嵌进错误码的阶段标识，便于区分是注册还是设置环节失败
func (client *RegisterClient) callSigned(ctx context.Context, appKey, appSecret, path string, body any, out any, stage string) error {
	raw, _ := json.Marshal(body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.apiBaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	nonceValue, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return err
	}
	nonce := nonceValue.String()
	timestamp := fmt.Sprint(time.Now().UnixMilli())
	digest := sha1.Sum([]byte(appSecret + nonce + timestamp)) //nolint:gosec // IM Server 固定签名协议。
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("appkey", appKey)
	request.Header.Set("nonce", nonce)
	request.Header.Set("timestamp", timestamp)
	request.Header.Set("signature", hex.EncodeToString(digest[:]))
	response, err := client.client.Do(request)
	if err != nil {
		return &Error{Status: 502, Code: "502_IMBOT_" + stage + "_REQUEST_FAILED", Message: "Bot " + path + " 请求失败: " + err.Error()}
	}
	defer response.Body.Close()
	envelope := struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return &Error{Status: 502, Code: "502_IMBOT_" + stage + "_BAD_RESPONSE", Message: fmt.Sprintf("Bot %s 响应非 JSON: HTTP %d", path, response.StatusCode)}
	}
	if envelope.Code != 0 {
		return &Error{Status: 502, Code: "502_IMBOT_" + stage + "_REJECTED", Message: fmt.Sprintf("Bot %s 被拒绝: code=%d %s", path, envelope.Code, envelope.Msg)}
	}
	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return &Error{Status: 502, Code: "502_IMBOT_" + stage + "_BAD_RESPONSE", Message: "Bot " + path + " 响应 data 解析失败"}
		}
	}
	return nil
}
