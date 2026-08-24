package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
)

func TestOpenAICallProtocol(t *testing.T) {
	reasoningEffort := "none"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("OpenAI 请求不符合协议: path=%s auth=%s", request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "gpt-test" {
			t.Fatalf("模型名未使用 API 配置: %#v", payload)
		}
		if payload["reasoning_effort"] != "none" {
			t.Fatalf("OpenAI-compatible 请求未转发 reasoning_effort: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
	}))
	defer server.Close()

	result, err := NewHTTPGateway().Call(context.Background(), Runtime{Protocol: "openai", APIKey: "key", BaseURL: server.URL, APIModelName: "gpt-test", TimeoutSeconds: 5}, dto.CallRequest{Messages: []dto.Message{{Role: "user", Content: "你好"}}, ReasoningEffort: &reasoningEffort})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "ok" || result.Usage.TotalTokens != 5 {
		t.Fatalf("OpenAI 响应归一错误: %+v", result)
	}
}

func TestAnthropicSystemMessageAndResponseProtocol(t *testing.T) {
	reasoningEffort := "none"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" || request.Header.Get("x-api-key") != "key" {
			t.Fatalf("Anthropic 请求不符合协议")
		}
		var payload map[string]interface{}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if payload["system"] != "系统提示" {
			t.Fatalf("system 消息未提升到顶层: %#v", payload)
		}
		if _, exists := payload["reasoning_effort"]; exists {
			t.Fatalf("Anthropic 原生请求不应发送 OpenAI reasoning_effort: %#v", payload)
		}
		_, _ = writer.Write([]byte(`{"content":[{"type":"text","text":"done"}],"usage":{"input_tokens":4,"output_tokens":6},"stop_reason":"end_turn"}`))
	}))
	defer server.Close()

	result, err := NewHTTPGateway().Call(context.Background(), Runtime{Protocol: "anthropic", APIKey: "key", BaseURL: server.URL, APIModelName: "claude-test", TimeoutSeconds: 5}, dto.CallRequest{Messages: []dto.Message{{Role: "system", Content: "系统提示"}, {Role: "user", Content: "你好"}}, ReasoningEffort: &reasoningEffort})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "done" || result.Usage.TotalTokens != 10 {
		t.Fatalf("Anthropic 响应归一错误: %+v", result)
	}
}

func TestOpenAIEmbeddingProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			t.Fatalf("嵌入路径错误: %s", request.URL.Path)
		}
		_, _ = writer.Write([]byte(`{"data":[{"index":1,"embedding":[3,4]},{"index":0,"embedding":[1,2]}],"usage":{"prompt_tokens":7,"total_tokens":7}}`))
	}))
	defer server.Close()

	result, err := NewHTTPGateway().Embed(context.Background(), Runtime{Protocol: "openai-compatible", APIKey: "key", BaseURL: server.URL, APIModelName: "embed", TimeoutSeconds: 5}, []string{"a", "b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Embeddings) != 2 || result.Embeddings[0][0] != 1 || result.Usage.TotalTokens != 7 {
		t.Fatalf("嵌入响应排序或 usage 错误: %+v", result)
	}
}
