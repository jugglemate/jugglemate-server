package imbot

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
)

// TestRegisterBotSignsRequestAndParsesIdentity 校验 Bot 注册签名、响应解析与消息接收设置。
//
// TIPS: only_mentioned 必须落到 false —— 线上曾因它默认为真，导致新建 Bot 只在被 @ 时
// 才收到群消息，客户正常发言时 Bot 静默不回，且 IM 侧无任何错误日志。
func TestRegisterBotSignsRequestAndParsesIdentity(t *testing.T) {
	secret := "secret"
	seen := map[string]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		nonce, timestamp := request.Header.Get("nonce"), request.Header.Get("timestamp")
		digest := sha1.Sum([]byte(secret + nonce + timestamp)) //nolint:gosec // 校验固定协议签名。
		if request.Header.Get("appkey") != "app" || request.Header.Get("signature") != hex.EncodeToString(digest[:]) {
			t.Errorf("%s 签名头不符合预期: %+v", request.URL.Path, request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("%s 请求体不是 JSON: %v", request.URL.Path, err)
		}
		seen[request.URL.Path] = body
		_ = json.NewEncoder(response).Encode(map[string]any{"code": 0, "msg": "success", "data": map[string]any{"user_id": "bot-user", "token": "connect-token"}})
	}))
	defer server.Close()
	client := NewRegisterClient(server.URL, false, func(context.Context, string) (string, error) { return secret, nil })
	result, err := client.RegisterBot(context.Background(), "app", "bot-id", "测试 Bot")
	if err != nil {
		t.Fatalf("Bot 注册失败: %v", err)
	}
	if result.UserID != "bot-user" || result.Token != "connect-token" {
		t.Fatalf("Bot 注册结果不符合预期: %+v", result)
	}
	register, ok := seen["/apigateway/bots/register"]
	if !ok {
		t.Fatal("未调用 Bot 注册接口")
	}
	if register["nickname"] != "测试 Bot" {
		t.Fatalf("注册请求体不符合预期: %v", register)
	}
	assertOnlyMentionedFalse(t, "/apigateway/bots/register", register)
	update, ok := seen["/apigateway/bots/update"]
	if !ok {
		t.Fatal("注册后未显式写入 bot_settings")
	}
	if update["bot_id"] != "bot-id" {
		t.Fatalf("设置请求体缺少正确的 bot_id: %v", update)
	}
	assertOnlyMentionedFalse(t, "/apigateway/bots/update", update)
}

// assertOnlyMentionedFalse 断言请求体里的 bot_settings.only_mentioned 显式为 false。
func assertOnlyMentionedFalse(t *testing.T, path string, body map[string]any) {
	t.Helper()
	settings, ok := body["bot_settings"].(map[string]any)
	if !ok {
		t.Fatalf("%s 请求体缺少 bot_settings: %v", path, body)
	}
	if settings["only_mentioned"] != false {
		t.Fatalf("%s 的 only_mentioned 必须为 false，实际 %v", path, settings["only_mentioned"])
	}
}

// TestRegisterBotFailsWhenSettingsRejected 校验设置写入失败时整体注册失败，不留下哑 Bot。
func TestRegisterBotFailsWhenSettingsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/apigateway/bots/update" {
			_ = json.NewEncoder(response).Encode(map[string]any{"code": 10102, "msg": "bot not found"})
			return
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"code": 0, "msg": "success", "data": map[string]any{"user_id": "bot-user", "token": "connect-token"}})
	}))
	defer server.Close()
	client := NewRegisterClient(server.URL, false, func(context.Context, string) (string, error) { return "secret", nil })
	if _, err := client.RegisterBot(context.Background(), "app", "bot-id", "测试 Bot"); err == nil {
		t.Fatal("bot_settings 写入被拒绝时注册应失败")
	}
}

// TestDisabledManagerSkipsConnection 校验本地关闭 IM 时不建立网络连接。
func TestDisabledManagerSkipsConnection(t *testing.T) {
	disabled := false
	manager := NewManager(configures.AgentIMConfig{Enabled: &disabled})
	userID, err := manager.EnsureBot(context.Background(), "app", "", "expected-user")
	if err != nil || userID != "expected-user" {
		t.Fatalf("关闭 IM 后应直接返回期望用户: userID=%q err=%v", userID, err)
	}
}

// TestValidateWSAddress 覆盖线上踩过的带路径 wsAddress 配置。
func TestValidateWSAddress(t *testing.T) {
	cases := []struct {
		address string
		ok      bool
	}{
		{"ws://127.0.0.1:9002", true},
		{"wss://ws.juggleim.com", true},
		{"wss://ws.juggleim.com/im", false},
		{"ws.juggleim.com", false},
		{"", false},
		{"wss://", false},
	}
	for _, item := range cases {
		if problem := validateWSAddress(item.address); (problem == "") != item.ok {
			t.Fatalf("wsAddress=%q 期望合法=%v，实际 problem=%q", item.address, item.ok, problem)
		}
	}
}
