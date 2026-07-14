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

	"github.com/juggleim/jugglemate-server/commons/configures"
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

// RegisterClient 调用 IM Server API 创建 Bot。
type RegisterClient struct {
	config configures.AgentIMConfig
	client *http.Client
}

// NewRegisterClient 创建带 TLS 与超时约束的 IM Bot 注册客户端。
func NewRegisterClient(config configures.AgentIMConfig) *RegisterClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.ServerAPIInsecure} //nolint:gosec // 仅由显式测试配置启用。
	return &RegisterClient{config: config, client: &http.Client{Timeout: 15 * time.Second, Transport: transport}}
}

// RegisterBot 使用 SHA1 签名头调用 `/bots/register` 并校验完整响应。
func (client *RegisterClient) RegisterBot(ctx context.Context, botID, nickname string) (RegisteredBot, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(client.config.ServerAPIURL), "/")
	if baseURL == "" {
		return RegisteredBot{}, &Error{Status: 500, Code: "500_IMBOT_SERVER_API_NOT_CONFIGURED", Message: "IM Server API 地址未配置"}
	}
	if strings.TrimSpace(client.config.AppKey) == "" || strings.TrimSpace(client.config.AppSecret) == "" {
		return RegisteredBot{}, &Error{Status: 500, Code: "500_IMBOT_CREDENTIALS_NOT_CONFIGURED", Message: "IM AppKey/AppSecret 未配置"}
	}
	body, _ := json.Marshal(map[string]any{"bot_id": botID, "nickname": nickname, "ext_fields": map[string]any{}})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/bots/register", bytes.NewReader(body))
	if err != nil {
		return RegisteredBot{}, err
	}
	nonceValue, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return RegisteredBot{}, err
	}
	nonce := nonceValue.String()
	timestamp := fmt.Sprint(time.Now().UnixMilli())
	digest := sha1.Sum([]byte(client.config.AppSecret + nonce + timestamp)) //nolint:gosec // IM Server 固定签名协议。
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("appkey", client.config.AppKey)
	request.Header.Set("nonce", nonce)
	request.Header.Set("timestamp", timestamp)
	request.Header.Set("signature", hex.EncodeToString(digest[:]))
	response, err := client.client.Do(request)
	if err != nil {
		return RegisteredBot{}, &Error{Status: 502, Code: "502_IMBOT_REGISTER_REQUEST_FAILED", Message: "Bot 注册请求失败: " + err.Error()}
	}
	defer response.Body.Close()
	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			UserID string `json:"user_id"`
			Token  string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return RegisteredBot{}, &Error{Status: 502, Code: "502_IMBOT_REGISTER_BAD_RESPONSE", Message: fmt.Sprintf("Bot 注册响应非 JSON: HTTP %d", response.StatusCode)}
	}
	if payload.Code != 0 {
		return RegisteredBot{}, &Error{Status: 502, Code: "502_IMBOT_REGISTER_REJECTED", Message: "Bot 注册被拒绝: " + payload.Msg}
	}
	if strings.TrimSpace(payload.Data.UserID) == "" || strings.TrimSpace(payload.Data.Token) == "" {
		return RegisteredBot{}, &Error{Status: 502, Code: "502_IMBOT_REGISTER_INCOMPLETE", Message: "Bot 注册响应缺少 user_id 或 token"}
	}
	return RegisteredBot{BotID: botID, UserID: payload.Data.UserID, Token: payload.Data.Token}, nil
}
