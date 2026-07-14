package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

// TestHTTPExecutorInjectsCredentialAndKeepsItOutOfSummary 校验鉴权注入与摘要脱敏。
func TestHTTPExecutorInjectsCredentialAndKeepsItOutOfSummary(t *testing.T) {
	executor := NewHTTPExecutor()
	executor.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/orders/42" || request.Header.Get("X-API-Key") != "top-secret" {
			t.Fatalf("请求路径或鉴权头不正确: path=%s key=%s", request.URL.Path, request.Header.Get("X-API-Key"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["amount"] != float64(9) {
			t.Fatalf("请求体不正确: %#v, err=%v", body, err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: request}, nil
	})}

	result := executor.Execute(context.Background(), ExecutionContext{
		TraceID:          "trace-1",
		ExecutionConfig:  map[string]any{"method": "POST", "path": "/orders/{id}"},
		ConnectionConfig: map[string]any{"base_url": "https://tool.example"},
		AuthType:         "api_key",
		Credential:       map[string]any{"api_key": "top-secret", "in": "header", "name": "X-API-Key"},
		Payload:          map[string]any{"id": 42, "amount": 9},
		TimeoutSeconds:   2,
	})
	if result.Status != "success" {
		t.Fatalf("执行应成功: %+v", result)
	}
	if result.RequestSummary == nil || strings.Contains(*result.RequestSummary, "top-secret") {
		t.Fatalf("请求摘要不应包含凭据: %v", result.RequestSummary)
	}
}
