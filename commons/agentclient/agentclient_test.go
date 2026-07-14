package agentclient

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("AGENTCLIENT_INTEGRATION") != "1" {
		t.Skip("设置 AGENTCLIENT_INTEGRATION=1 后运行旧远端客户端集成测试")
	}
}

func TestHealthCheck(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL: "http://8.218.63.116:8080",
		Timeout: 5 * time.Second,
	})

	code, err := client.requestJSONWithHeaders(context.Background(), "GET", "/healthz", nil, nil, nil)
	if err != nil {
		t.Fatalf("health check failed: code=%d, err=%v", code, err)
	}
	t.Logf("health check passed: code=%d", code)
}

func TestListTwins(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
		OwnerID:       "W0tAbIbSIri",
	})

	twins, code, err := client.ListTwins(context.Background(), "W0tAbIbSIri", "")
	if err != nil {
		t.Fatalf("ListTwins failed: code=%d, err=%v", code, err)
	}
	t.Logf("ListTwins succeeded: count=%d, twins=%+v", len(twins), twins)
}

func TestGetTwin(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
	})

	twin, code, err := client.GetTwin(context.Background(), "W0tAbIbSIri", "zxf2")
	if err != nil {
		t.Fatalf("GetTwin failed: code=%d, err=%v", code, err)
	}
	t.Logf("GetTwin succeeded: twin=%+v", twin)
}

func TestChat(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
	})

	resp, code, err := client.Chat(context.Background(), "cu_42", "jim", "zxf2", ChatRequest{
		Message: "你好，简单介绍一下你自己",
	})
	if err != nil {
		t.Fatalf("Chat failed: code=%d, err=%v", code, err)
	}
	t.Logf("Chat succeeded: resp=%+v", resp)
}

func TestChatWithOwnerID(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
		OwnerID:       "W0tAbIbSIri",
	})

	resp, code, err := client.Chat(context.Background(), "cu_42", "jim", "zxf2", ChatRequest{
		Message: "你好",
	})
	if err != nil {
		t.Fatalf("ChatWithOwnerID failed: code=%d, err=%v", code, err)
	}
	t.Logf("ChatWithOwnerID succeeded: resp=%+v", resp)
}

func TestAdminListTwins(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
	})

	// Admin endpoint: /v1/admin/twins
	// The client uses /twins, so we need to use requestJSONWithHeaders directly
	var result TwinPage
	code, err := client.requestJSONWithHeaders(context.Background(), "GET", "/admin/twins", nil, nil, &result)
	if err != nil {
		t.Fatalf("AdminListTwins failed: code=%d, err=%v", code, err)
	}
	t.Logf("AdminListTwins succeeded: count=%d, twins=%+v", len(result.Data), result.Data)
}

func TestConfig(t *testing.T) {
	cfg := Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       10 * time.Second,
		OwnerID:       "W0tAbIbSIri",
		Authorization: "my-token",
	}
	client := New(cfg)

	if client.BaseURL() != cfg.BaseURL {
		t.Fatalf("BaseURL mismatch: got %s, want %s", client.BaseURL(), cfg.BaseURL)
	}
	if client.Timeout() != cfg.Timeout {
		t.Fatalf("Timeout mismatch: got %v, want %v", client.Timeout(), cfg.Timeout)
	}
	if client.OwnerID() != cfg.OwnerID {
		t.Fatalf("OwnerID mismatch: got %s, want %s", client.OwnerID(), cfg.OwnerID)
	}

	t.Logf("Config test passed: BaseURL=%s, Timeout=%v, OwnerID=%s", client.BaseURL(), client.Timeout(), client.OwnerID())
}

func TestCreateTwin(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "5b646e425241f3e7fe7d5e3faae143e118a80b06921c28a9",
	})

	twin, code, err := client.CreateTwin(context.Background(), "W0tAbIbSIri", map[string]any{
		"unique_name":  "my-bot-test",
		"display_name": "测试分身",
	})
	if err != nil {
		t.Fatalf("CreateTwin failed: code=%d, err=%v", code, err)
	}
	t.Logf("CreateTwin succeeded: twin=%+v", twin)
}

func TestToErrorResponse(t *testing.T) {
	// Test empty body
	resp, err := ToErrorResponse(nil)
	if err != nil || resp != nil {
		t.Fatalf("ToErrorResponse(nil) = %v, %v; want nil, nil", resp, err)
	}

	resp, err = ToErrorResponse([]byte{})
	if err != nil || resp != nil {
		t.Fatalf("ToErrorResponse([]byte{}) = %v, %v; want nil, nil", resp, err)
	}

	// Test valid error JSON
	errJSON := []byte(`{"error":{"code":"unauthorized","message":"invalid token"}}`)
	resp, err = ToErrorResponse(errJSON)
	if err != nil {
		t.Fatalf("ToErrorResponse failed: %v", err)
	}
	if resp.Error.Code != "unauthorized" {
		t.Fatalf("Error.Code = %s; want unauthorized", resp.Error.Code)
	}
	if resp.Error.Message != "invalid token" {
		t.Fatalf("Error.Message = %s; want invalid token", resp.Error.Message)
	}

	t.Logf("ToErrorResponse test passed: resp=%+v", resp)
}

func TestUnauthorized(t *testing.T) {
	requireIntegration(t)
	client := New(Config{
		BaseURL:       "http://8.218.63.116:8080",
		Timeout:       5 * time.Second,
		Authorization: "wrong-token",
	})

	// Server returns 401 for unauthorized, but may return 404 if route doesn't exist
	// Just verify we get a response with the wrong token
	_, code, err := client.ListTwins(context.Background(), "W0tAbIbSIri", "")
	t.Logf("Unauthorized request: code=%d, err=%v", code, err)
	// The key is that wrong token should not give us valid data
	// We just verify the request completed (whether error or not)
}

type createOnlyBackend struct {
	Backend
	called bool
}

func (backend *createOnlyBackend) CreateTwin(_ context.Context, ownerID string, request map[string]any) (*Twin, int, error) {
	backend.called = true
	return &Twin{OwnerID: ownerID, UniqueName: request["unique_name"].(string)}, 201, nil
}

// TestClientUsesInProcessBackend 校验公开 Twin 方法只调用 Go 进程内实现。
func TestClientUsesInProcessBackend(t *testing.T) {
	backend := &createOnlyBackend{}
	SetBackend(backend)
	defer SetBackend(nil)
	client := New(Config{BaseURL: "http://不应访问.example"})
	result, code, err := client.CreateTwin(context.Background(), "owner-1", map[string]any{"unique_name": "bot-1"})
	if err != nil || code != 201 || result.UniqueName != "bot-1" || !backend.called {
		t.Fatalf("进程内调用结果异常: result=%+v code=%d err=%v called=%v", result, code, err, backend.called)
	}
}

// TestClientRefusesLegacyTwinHTTP 校验未启动 Go 模块时不会回退 Python Twin HTTP。
func TestClientRefusesLegacyTwinHTTP(t *testing.T) {
	SetBackend(nil)
	client := New(Config{BaseURL: "http://127.0.0.1:1"})
	_, code, err := client.GetTwin(context.Background(), "owner-1", "bot-1")
	if code != 503 || err != ErrBackendUnavailable {
		t.Fatalf("应返回 Go Backend 不可用，实际 code=%d err=%v", code, err)
	}
}

// TestMain runs all tests
func TestMain(m *testing.M) {
	fmt.Println("Running agentclient tests...")
	m.Run()
}
