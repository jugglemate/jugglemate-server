// Package adapter 提供 Tools v2 对外部 HTTP 工具的执行适配。
package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ExecutionContext 封装一次工具 HTTP 调用所需的全部运行时信息。
type ExecutionContext struct {
	TraceID          string
	ToolType         string
	ExecutionConfig  map[string]any
	ConnectionConfig map[string]any
	AuthType         string
	Credential       map[string]any
	Payload          map[string]any
	TimeoutSeconds   int
}

// ExecutionResult 表示外部工具执行结果。
type ExecutionResult struct {
	Status          string
	Output          any
	ErrorCode       *string
	ErrorMessage    *string
	DurationMS      int
	RequestSummary  *string
	ResponseSummary *string
}

type cachedToken struct {
	value     string
	expiresAt time.Time
}

// HTTPExecutor 执行 plugin/custom 的 HTTP 模式，并负责运行时鉴权注入。
type HTTPExecutor struct {
	client *http.Client
	mu     sync.Mutex
	tokens map[string]cachedToken
}

// NewHTTPExecutor 创建 HTTP 工具执行器。
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{client: &http.Client{}, tokens: map[string]cachedToken{}}
}

// Execute 执行一次真实 HTTP 工具调用。
//
// 简要描述：调用方只传递已解密的结构化凭据，本方法在内存中把它注入 header/query；
// 请求摘要在注入前生成且永不包含鉴权信息，从而避免凭据进入接口响应或日志。
func (executor *HTTPExecutor) Execute(ctx context.Context, input ExecutionContext) ExecutionResult {
	started := time.Now()
	method := strings.ToUpper(stringValue(input.ExecutionConfig["method"]))
	if method == "" {
		method = http.MethodPost
	}
	path := stringValue(input.ExecutionConfig["path"])
	if path == "" {
		path = stringValue(input.ExecutionConfig["endpoint"])
	}
	baseURL := stringValue(input.ConnectionConfig["base_url"])
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		baseURL = ""
	}
	if path == "" || (baseURL == "" && !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://")) {
		return failed(started, "500_TOOL_RUNTIME_ERROR", "HTTP 执行地址为空", nil, nil)
	}

	headers, authQuery, err := executor.auth(ctx, input.AuthType, input.Credential)
	if err != nil {
		return failed(started, "401_TOOL_CREDENTIAL_REVOKED", err.Error(), nil, nil)
	}
	plan, err := buildPlan(method, joinURL(baseURL, path), input.Payload, input.ExecutionConfig, headers, authQuery)
	if err != nil {
		return failed(started, "500_TOOL_RUNTIME_ERROR", err.Error(), nil, nil)
	}
	plan.headers.Set("X-Trace-Id", input.TraceID)
	summaryQuery := cloneValues(plan.query)
	for key := range authQuery {
		summaryQuery.Del(key)
	}
	requestSummary := summarize(map[string]any{"method": method, "url": plan.url, "query": summaryQuery, "body": plan.body})

	var body io.Reader
	if plan.body != nil {
		encoded, marshalErr := json.Marshal(plan.body)
		if marshalErr != nil {
			return failed(started, "500_TOOL_RUNTIME_ERROR", "请求体序列化失败", requestSummary, nil)
		}
		body = bytes.NewReader(encoded)
		plan.headers.Set("Content-Type", "application/json")
	}
	requestCtx := ctx
	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 30
	}
	requestCtx, cancel := context.WithTimeout(requestCtx, time.Duration(input.TimeoutSeconds)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, plan.urlWithQuery(), body)
	if err != nil {
		return failed(started, "500_TOOL_RUNTIME_ERROR", err.Error(), requestSummary, nil)
	}
	req.Header = plan.headers
	response, err := executor.client.Do(req)
	if err != nil {
		if requestCtx.Err() == context.DeadlineExceeded {
			return failedWithStatus(started, "timeout", "504_TOOL_TIMEOUT", "工具调用超时", requestSummary, nil)
		}
		return failed(started, "500_TOOL_RUNTIME_ERROR", fmt.Sprintf("HTTP 调用异常: %v", err), requestSummary, nil)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return failed(started, "500_TOOL_RUNTIME_ERROR", "读取上游响应失败", requestSummary, nil)
	}
	var output any
	if len(content) > 0 && json.Unmarshal(content, &output) != nil {
		output = string(content)
	}
	responseSummary := summarize(output)
	if response.StatusCode >= 400 {
		return failed(started, "502_TOOL_UPSTREAM_ERROR", fmt.Sprintf("上游错误: %d", response.StatusCode), requestSummary, responseSummary)
	}
	return ExecutionResult{Status: "success", Output: output, DurationMS: int(time.Since(started).Milliseconds()), RequestSummary: requestSummary, ResponseSummary: responseSummary}
}

type requestPlan struct {
	url     string
	headers http.Header
	query   url.Values
	body    any
}

func (plan requestPlan) urlWithQuery() string {
	if len(plan.query) == 0 {
		return plan.url
	}
	separator := "?"
	if strings.Contains(plan.url, "?") {
		separator = "&"
	}
	return plan.url + separator + plan.query.Encode()
}

var pathParameterPattern = regexp.MustCompile(`\{([^{}]+)\}`)

func buildPlan(method, endpoint string, payload, execution map[string]any, headers http.Header, query url.Values) (requestPlan, error) {
	used := map[string]bool{}
	body := any(nil)
	mapping, _ := execution["paramMapping"].(map[string]any)
	if mapping == nil {
		mapping, _ = execution["param_mapping"].(map[string]any)
	}
	if mapping != nil {
		for placeholder, source := range mapValue(mapping["path"]) {
			key := stringValue(source)
			if value, ok := payload[key]; ok {
				endpoint = strings.ReplaceAll(endpoint, "{"+placeholder+"}", fmt.Sprint(value))
				used[key] = true
			}
		}
		for name, source := range mapValue(mapping["query"]) {
			key := stringValue(source)
			if value, ok := payload[key]; ok {
				query.Set(name, fmt.Sprint(value))
				used[key] = true
			}
		}
		for name, source := range mapValue(mapping["header"]) {
			key := stringValue(source)
			if value, ok := payload[key]; ok {
				headers.Set(name, fmt.Sprint(value))
				used[key] = true
			}
		}
		switch bodyMapping := mapping["body"].(type) {
		case string:
			body = payload[bodyMapping]
			used[bodyMapping] = true
		case map[string]any:
			mapped := map[string]any{}
			for name, source := range bodyMapping {
				key := stringValue(source)
				mapped[name] = payload[key]
				used[key] = true
			}
			body = mapped
		}
	} else {
		for _, match := range pathParameterPattern.FindAllStringSubmatch(endpoint, -1) {
			key := match[1]
			if value, ok := payload[key]; ok {
				endpoint = strings.ReplaceAll(endpoint, match[0], fmt.Sprint(value))
				used[key] = true
			}
		}
		remaining := map[string]any{}
		for key, value := range payload {
			if !used[key] {
				remaining[key] = value
			}
		}
		if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead || method == http.MethodOptions {
			for key, value := range remaining {
				if value != nil {
					query.Set(key, fmt.Sprint(value))
				}
			}
		} else if len(remaining) > 0 {
			body = remaining
		}
	}
	if strings.Contains(endpoint, "{") {
		return requestPlan{}, fmt.Errorf("路径参数未完整提供")
	}
	return requestPlan{url: endpoint, headers: headers, query: query, body: body}, nil
}

func (executor *HTTPExecutor) auth(ctx context.Context, authType string, credential map[string]any) (http.Header, url.Values, error) {
	headers := http.Header{}
	query := url.Values{}
	switch authType {
	case "", "none":
		return headers, query, nil
	case "api_key":
		key, name, position := stringValue(credential["api_key"]), stringValue(credential["name"]), stringValue(credential["in"])
		if key == "" || name == "" {
			return nil, nil, fmt.Errorf("api_key 凭据缺少 api_key/name 字段")
		}
		if position == "query" {
			query.Set(name, key)
		} else if position == "" || position == "header" {
			headers.Set(name, key)
		} else {
			return nil, nil, fmt.Errorf("api_key.in 不支持: %s", position)
		}
	case "bearer":
		token, prefix := stringValue(credential["token"]), stringValue(credential["prefix"])
		if prefix == "" {
			prefix = "Bearer"
		}
		if token == "" {
			return nil, nil, fmt.Errorf("bearer 凭据缺少 token 字段")
		}
		headers.Set("Authorization", prefix+" "+token)
	case "basic":
		username, password := stringValue(credential["username"]), stringValue(credential["password"])
		if username == "" || password == "" {
			return nil, nil, fmt.Errorf("basic 凭据缺少 username/password 字段")
		}
		headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	case "oauth2_client_credentials":
		token, err := executor.oauthToken(ctx, credential)
		if err != nil {
			return nil, nil, err
		}
		headers.Set("Authorization", "Bearer "+token)
	default:
		return nil, nil, fmt.Errorf("不支持的鉴权类型: %s", authType)
	}
	return headers, query, nil
}

func (executor *HTTPExecutor) oauthToken(ctx context.Context, credential map[string]any) (string, error) {
	tokenURL, clientID, secret := stringValue(credential["token_url"]), stringValue(credential["client_id"]), stringValue(credential["client_secret"])
	scope, audience := stringValue(credential["scope"]), stringValue(credential["audience"])
	if tokenURL == "" || clientID == "" || secret == "" {
		return "", fmt.Errorf("oauth2 凭据缺少必填字段")
	}
	cacheKey := strings.Join([]string{tokenURL, clientID, scope, audience}, "|")
	executor.mu.Lock()
	cached, ok := executor.tokens[cacheKey]
	executor.mu.Unlock()
	if ok && cached.expiresAt.After(time.Now().Add(5*time.Second)) {
		return cached.value, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {clientID}, "client_secret": {secret}}
	if scope != "" {
		form.Set("scope", scope)
	}
	if audience != "" {
		form.Set("audience", audience)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := executor.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token 获取失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("oauth2 token 获取失败: HTTP %d", response.StatusCode)
	}
	var body map[string]any
	if json.NewDecoder(response.Body).Decode(&body) != nil {
		return "", fmt.Errorf("oauth2 token 响应格式非法")
	}
	token := stringValue(body["access_token"])
	if token == "" {
		return "", fmt.Errorf("oauth2 响应缺少 access_token")
	}
	expires := 600.0
	if value, ok := body["expires_in"].(float64); ok {
		expires = value
	}
	executor.mu.Lock()
	executor.tokens[cacheKey] = cachedToken{value: token, expiresAt: time.Now().Add(time.Duration(expires) * time.Second)}
	executor.mu.Unlock()
	return token, nil
}

func failed(started time.Time, code, message string, request, response *string) ExecutionResult {
	return failedWithStatus(started, "failed", code, message, request, response)
}
func failedWithStatus(started time.Time, status, code, message string, request, response *string) ExecutionResult {
	return ExecutionResult{Status: status, ErrorCode: &code, ErrorMessage: &message, DurationMS: int(time.Since(started).Milliseconds()), RequestSummary: request, ResponseSummary: response}
}
func summarize(value any) *string {
	if value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		encoded, _ := json.Marshal(value)
		text = string(encoded)
	}
	if len(text) > 500 {
		text = text[:500] + "...(truncated)"
	}
	return &text
}
func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}
func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}
func cloneValues(source url.Values) url.Values {
	result := make(url.Values, len(source))
	for key, values := range source {
		result[key] = append([]string(nil), values...)
	}
	return result
}
func joinURL(baseURL, path string) string {
	if baseURL == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}
