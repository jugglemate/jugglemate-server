// Package adapter 实现 OpenAI、OpenAI-compatible 与 Anthropic 协议适配。
package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
)

// Runtime 表示一次模型调用所需的已解密运行配置。
type Runtime struct {
	Protocol       string
	APIKey         string
	BaseURL        string
	TimeoutSeconds int
	ProxyURL       string
	APIModelName   string
	CustomHeaders  map[string]string
}

// Result 表示协议无关的模型调用结果。
type Result struct {
	Content      string
	Usage        dto.Usage
	FinishReason *string
}

// EmbeddingResult 表示批量文本嵌入结果。
type EmbeddingResult struct {
	Embeddings [][]float32
	Usage      dto.Usage
}

// StreamChunk 表示一个流式文本块或终止错误。
type StreamChunk struct {
	Text  string
	Usage *dto.Usage
	Err   error
}

// ConnectionResult 表示 Provider 连接测试结果。
type ConnectionResult struct {
	Success       bool    `json:"success"`
	ErrorCategory *string `json:"error_category"`
	ErrorCode     *string `json:"error_code"`
	Message       *string `json:"message"`
	LatencyMS     *int    `json:"latency_ms"`
}

// HTTPGateway 调用 Provider 的标准 HTTP 接口。
type HTTPGateway struct{}

// NewHTTPGateway 创建 HTTP 模型网关。
func NewHTTPGateway() *HTTPGateway { return &HTTPGateway{} }

// Embed 调用 OpenAI 或 OpenAI-compatible 嵌入接口。
func (gateway *HTTPGateway) Embed(ctx context.Context, runtime Runtime, texts []string, dimension *int) (EmbeddingResult, error) {
	if runtime.Protocol == "anthropic" {
		return EmbeddingResult{}, fmt.Errorf("Anthropic Provider 不支持嵌入接口")
	}
	payload := map[string]interface{}{"model": runtime.APIModelName, "input": texts}
	if dimension != nil {
		payload["dimensions"] = *dimension
	}
	response, _, err := gateway.do(ctx, runtime, http.MethodPost, "/v1/embeddings", payload)
	if err != nil {
		return EmbeddingResult{}, err
	}
	defer response.Body.Close()
	if err := requireSuccess(response); err != nil {
		return EmbeddingResult{}, err
	}
	var body struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return EmbeddingResult{}, fmt.Errorf("解析嵌入响应失败: %w", err)
	}
	sort.Slice(body.Data, func(i, j int) bool { return body.Data[i].Index < body.Data[j].Index })
	result := EmbeddingResult{Embeddings: make([][]float32, 0, len(body.Data)), Usage: dto.Usage{InputTokens: body.Usage.PromptTokens, TotalTokens: body.Usage.TotalTokens}}
	for _, item := range body.Data {
		result.Embeddings = append(result.Embeddings, item.Embedding)
	}
	if len(result.Embeddings) != len(texts) {
		return EmbeddingResult{}, fmt.Errorf("嵌入响应数量不匹配: want=%d got=%d", len(texts), len(result.Embeddings))
	}
	return result, nil
}

// Call 执行一次非流式模型调用。
func (gateway *HTTPGateway) Call(ctx context.Context, runtime Runtime, request dto.CallRequest) (Result, error) {
	payload := buildPayload(runtime, request, false)
	response, elapsed, err := gateway.do(ctx, runtime, http.MethodPost, callPath(runtime.Protocol), payload)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if err := requireSuccess(response); err != nil {
		return Result{}, err
	}
	var body map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return Result{}, fmt.Errorf("解析模型响应失败: %w", err)
	}
	result := parseResult(runtime.Protocol, body)
	_ = elapsed
	return result, nil
}

// Stream 发起流式模型调用，建立上游连接失败时同步返回错误。
//
// 简要描述：先等待上游返回成功状态再把 channel 交给 HTTP handler；这样 401、超时等
// 错误仍能按统一信封返回，只有连接成功后才切换到不可回退的纯文本流响应。
func (gateway *HTTPGateway) Stream(ctx context.Context, runtime Runtime, request dto.CallRequest) (<-chan StreamChunk, error) {
	payload := buildPayload(runtime, request, true)
	response, _, err := gateway.do(ctx, runtime, http.MethodPost, callPath(runtime.Protocol), payload)
	if err != nil {
		return nil, err
	}
	if err := requireSuccess(response); err != nil {
		response.Body.Close()
		return nil, err
	}
	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			raw := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if raw == "[DONE]" {
				break
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &event); err != nil {
				chunks <- StreamChunk{Err: fmt.Errorf("解析模型流事件失败: %w", err)}
				return
			}
			text, usage := parseStreamEvent(runtime.Protocol, event)
			if usage != nil {
				chunks <- StreamChunk{Usage: usage}
			}
			if text != "" {
				chunks <- StreamChunk{Text: text}
			}
		}
		if err := scanner.Err(); err != nil {
			chunks <- StreamChunk{Err: fmt.Errorf("读取模型流失败: %w", err)}
		}
	}()
	return chunks, nil
}

// TestConnection 调用 Provider 模型列表端点验证网络与凭据。
func (gateway *HTTPGateway) TestConnection(ctx context.Context, runtime Runtime) ConnectionResult {
	response, elapsed, err := gateway.do(ctx, runtime, http.MethodGet, "/v1/models", nil)
	latency := int(elapsed.Milliseconds())
	if err != nil {
		category := "network"
		message := err.Error()
		if timeoutError, ok := err.(net.Error); ok && timeoutError.Timeout() {
			category = "timeout"
			message = "连接超时"
		}
		return ConnectionResult{ErrorCategory: &category, Message: &message, LatencyMS: &latency}
	}
	defer response.Body.Close()
	if response.StatusCode < 300 {
		return ConnectionResult{Success: true, LatencyMS: &latency}
	}
	code := fmt.Sprintf("%d", response.StatusCode)
	category, message := "config", "配置错误"
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		category, message = "auth", "认证失败"
	}
	return ConnectionResult{ErrorCategory: &category, ErrorCode: &code, Message: &message, LatencyMS: &latency}
}

func (gateway *HTTPGateway) do(ctx context.Context, runtime Runtime, method string, path string, payload interface{}) (*http.Response, time.Duration, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, normalizeBaseURL(runtime)+path, body)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	if runtime.Protocol == "anthropic" {
		request.Header.Set("x-api-key", runtime.APIKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	} else {
		request.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	}
	for key, value := range runtime.CustomHeaders {
		request.Header.Set(key, value)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if strings.TrimSpace(runtime.ProxyURL) != "" {
		proxyURL, err := url.Parse(runtime.ProxyURL)
		if err != nil {
			return nil, 0, fmt.Errorf("代理地址非法: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	timeout := time.Duration(runtime.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	started := time.Now()
	response, err := client.Do(request)
	return response, time.Since(started), err
}

func normalizeBaseURL(runtime Runtime) string {
	baseURL := strings.TrimRight(strings.TrimSpace(runtime.BaseURL), "/")
	if baseURL == "" {
		if runtime.Protocol == "anthropic" {
			baseURL = "https://api.anthropic.com"
		} else {
			baseURL = "https://api.openai.com"
		}
	}
	if runtime.Protocol != "anthropic" && strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL
}

func callPath(protocol string) string {
	if protocol == "anthropic" {
		return "/v1/messages"
	}
	return "/v1/chat/completions"
}

func buildPayload(runtime Runtime, request dto.CallRequest, stream bool) map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(request.Messages))
	var system []string
	for _, message := range request.Messages {
		if runtime.Protocol == "anthropic" && message.Role == "system" {
			if message.Content != "" {
				system = append(system, message.Content)
			}
			continue
		}
		messages = append(messages, map[string]interface{}{"role": message.Role, "content": message.Content})
	}
	payload := map[string]interface{}{"model": runtime.APIModelName, "messages": messages}
	if runtime.Protocol == "anthropic" {
		if len(system) > 0 {
			payload["system"] = strings.Join(system, "\n\n")
		}
		if request.MaxTokens == nil {
			payload["max_tokens"] = 1024
		}
	}
	if len(request.Tools) > 0 {
		payload["tools"] = normalizeTools(runtime.Protocol, request.Tools)
	}
	if request.Temperature != nil {
		payload["temperature"] = *request.Temperature
	}
	if request.MaxTokens != nil {
		payload["max_tokens"] = *request.MaxTokens
	}
	if stream {
		payload["stream"] = true
		if runtime.Protocol != "anthropic" {
			payload["stream_options"] = map[string]bool{"include_usage": true}
		}
	}
	return payload
}

func normalizeTools(protocol string, tools []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if protocol == "anthropic" {
			if function, ok := tool["function"].(map[string]interface{}); ok {
				result = append(result, map[string]interface{}{"name": function["name"], "description": function["description"], "input_schema": function["parameters"]})
				continue
			}
		} else if schema, ok := tool["input_schema"]; ok {
			result = append(result, map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": tool["name"], "description": tool["description"], "parameters": schema}})
			continue
		}
		result = append(result, tool)
	}
	return result
}

func requireSuccess(response *http.Response) error {
	if response.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	return fmt.Errorf("上游模型接口返回 %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
}

func parseResult(protocol string, body map[string]interface{}) Result {
	if protocol == "anthropic" {
		content := ""
		if blocks, ok := body["content"].([]interface{}); ok {
			for _, raw := range blocks {
				if block, ok := raw.(map[string]interface{}); ok && block["type"] == "text" {
					content += stringValue(block["text"])
				}
			}
		}
		usageMap, _ := body["usage"].(map[string]interface{})
		usage := dto.Usage{InputTokens: intValue(usageMap["input_tokens"]), OutputTokens: intValue(usageMap["output_tokens"])}
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		finish := nullableString(body["stop_reason"])
		return Result{Content: content, Usage: usage, FinishReason: finish}
	}
	choices, _ := body["choices"].([]interface{})
	var choice map[string]interface{}
	if len(choices) > 0 {
		choice, _ = choices[0].(map[string]interface{})
	}
	message, _ := choice["message"].(map[string]interface{})
	usageMap, _ := body["usage"].(map[string]interface{})
	return Result{Content: extractText(message["content"]), Usage: dto.Usage{InputTokens: intValue(usageMap["prompt_tokens"]), OutputTokens: intValue(usageMap["completion_tokens"]), TotalTokens: intValue(usageMap["total_tokens"])}, FinishReason: nullableString(choice["finish_reason"])}
}

func parseStreamEvent(protocol string, event map[string]interface{}) (string, *dto.Usage) {
	if protocol == "anthropic" {
		eventType := stringValue(event["type"])
		if eventType == "content_block_delta" {
			delta, _ := event["delta"].(map[string]interface{})
			return stringValue(delta["text"]), nil
		}
		if eventType == "message_start" {
			message, _ := event["message"].(map[string]interface{})
			raw, _ := message["usage"].(map[string]interface{})
			usage := dto.Usage{InputTokens: intValue(raw["input_tokens"])}
			return "", &usage
		}
		if eventType == "message_delta" {
			raw, _ := event["usage"].(map[string]interface{})
			usage := dto.Usage{OutputTokens: intValue(raw["output_tokens"])}
			return "", &usage
		}
		return "", nil
	}
	if raw, ok := event["usage"].(map[string]interface{}); ok {
		usage := dto.Usage{InputTokens: intValue(raw["prompt_tokens"]), OutputTokens: intValue(raw["completion_tokens"]), TotalTokens: intValue(raw["total_tokens"])}
		return "", &usage
	}
	choices, _ := event["choices"].([]interface{})
	if len(choices) == 0 {
		return "", nil
	}
	choice, _ := choices[0].(map[string]interface{})
	delta, _ := choice["delta"].(map[string]interface{})
	return extractText(delta["content"]), nil
}

func extractText(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	blocks, _ := value.([]interface{})
	result := ""
	for _, raw := range blocks {
		if block, ok := raw.(map[string]interface{}); ok {
			result += stringValue(block["text"])
		}
	}
	return result
}
func stringValue(value interface{}) string { text, _ := value.(string); return text }
func nullableString(value interface{}) *string {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}
func intValue(value interface{}) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case json.Number:
		result, _ := number.Int64()
		return int(result)
	default:
		return 0
	}
}
