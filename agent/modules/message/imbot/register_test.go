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

// TestRegisterBotSignsRequestAndParsesIdentity 校验 Bot 注册签名与响应解析。
func TestRegisterBotSignsRequestAndParsesIdentity(t *testing.T) {
	secret := "secret"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		nonce, timestamp := request.Header.Get("nonce"), request.Header.Get("timestamp")
		digest := sha1.Sum([]byte(secret + nonce + timestamp)) //nolint:gosec // 校验固定协议签名。
		if request.Header.Get("appkey") != "app" || request.Header.Get("signature") != hex.EncodeToString(digest[:]) {
			t.Fatalf("Bot 注册签名头不符合预期: %+v", request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["nickname"] != "测试 Bot" {
			t.Fatalf("Bot 注册请求体不符合预期: body=%v err=%v", body, err)
		}
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
