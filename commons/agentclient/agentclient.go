package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/juggleim/jugglemate-server/commons/tools"
)

// Config 保留旧客户端构造参数；Twin 公开方法已改用进程内 Backend。
type Config struct {
	BaseURL       string
	Timeout       time.Duration
	OwnerID       string
	Authorization string // Bearer token
	TwinsToken    string // /twins 接口专用 token
}

// Client 为当前业务层提供兼容的 Twin 调用门面。
type Client struct {
	cfg     Config
	backend Backend
}

// ErrBackendUnavailable 表示 Go Agent 模块尚未安装进程内 Twin 实现。
var ErrBackendUnavailable = errors.New("Go Agent Twin 兼容服务尚未启动")

// Backend 定义旧 Twin 调用方迁移到 Go Agent 的进程内兼容契约。
//
// 简要描述：业务层保留原有方法签名，Client 在 Go Agent 启动后优先调用本接口，
// 不再经过 Python HTTP；未安装 Backend 仅用于独立客户端兼容测试。
type Backend interface {
	// CreateTwin 创建一个 Owner 下的 Twin 映射。
	CreateTwin(context.Context, string, map[string]any) (*Twin, int, error)
	// ListTwins 查询 Owner 的 Twin 列表。
	ListTwins(context.Context, string, string) ([]Twin, int, error)
	// GetTwin 查询单个 Twin。
	GetTwin(context.Context, string, string) (*Twin, int, error)
	// UpdateTwin 更新 Twin 展示和提示词配置。
	UpdateTwin(context.Context, string, string, map[string]any) (*Twin, int, error)
	// DeleteTwin 删除 Twin 兼容映射并归档 Agent。
	DeleteTwin(context.Context, string, string) (int, error)
	// AddMaterial 写入结构化训练材料。
	AddMaterial(context.Context, string, string, map[string]any) (*Material, int, error)
	// AddMaterialFile 写入文件训练材料。
	AddMaterialFile(context.Context, string, string, string, []byte, string, map[string]string) (*Material, int, error)
	// ListMaterials 查询材料列表。
	ListMaterials(context.Context, string, string, string) ([]Material, int, error)
	// DeleteMaterial 删除一份材料。
	DeleteMaterial(context.Context, string, string, string) (int, error)
	// StartTraining 启动训练兼容任务。
	StartTraining(context.Context, string, string, map[string]any) (*Job, int, error)
	// StartEvaluation 启动评估兼容任务。
	StartEvaluation(context.Context, string, string, map[string]any) (*Job, int, error)
	// GetEvaluation 查询评估结果。
	GetEvaluation(context.Context, string, string, string) (*Evaluation, int, error)
	// ListEvaluations 查询 Twin 的评估结果列表。
	ListEvaluations(context.Context, string, string, string) ([]Evaluation, int, error)
	// ListJobs 查询 Twin 任务。
	ListJobs(context.Context, string, string, string) ([]Job, int, error)
	// GetJob 查询单个任务。
	GetJob(context.Context, string, string) (*Job, int, error)
	// CancelJob 取消未完成任务。
	CancelJob(context.Context, string, string) (*Job, int, error)
	// ListVersions 查询训练版本。
	ListVersions(context.Context, string, string) ([]Version, int, error)
	// ActivateVersion 激活指定版本。
	ActivateVersion(context.Context, string, string, string) (*Twin, int, error)
	// GetCurrentVersion 查询当前版本。
	GetCurrentVersion(context.Context, string, string) (*Version, int, error)
	// Chat 通过 Twin 映射调用 Go Reasoning。
	Chat(context.Context, string, string, string, ChatRequest) (*ChatResponse, int, error)
}

var backendRegistry struct {
	sync.RWMutex
	value Backend
}

// SetBackend 安装 Go Agent 进程内 Twin 兼容实现；传入 nil 可在模块关闭时解除安装。
func SetBackend(backend Backend) {
	backendRegistry.Lock()
	backendRegistry.value = backend
	backendRegistry.Unlock()
}

// currentBackend 返回当前安装的进程内实现。
func currentBackend() Backend {
	backendRegistry.RLock()
	defer backendRegistry.RUnlock()
	return backendRegistry.value
}

// ErrorResponse 表示旧 HTTP 服务错误结构，仅保留给历史响应解析。
type ErrorResponse struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

// Twin 表示旧业务层使用的智能分身视图。
type Twin struct {
	UniqueName     string `json:"unique_name"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url"`
	Greeting       string `json:"greeting"`
	OwnerID        string `json:"owner_id"`
	Status         string `json:"status"`
	ActiveVersion  string `json:"active_version"`
	TrainingMode   string `json:"training_mode"`
	MaterialsCount int    `json:"materials_count"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// TwinPage 表示旧 Twin 游标分页结构。
type TwinPage struct {
	Data       []Twin `json:"data"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// Material 表示一份 Twin 训练材料。
type Material struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

// MaterialPage 表示旧材料游标分页结构。
type MaterialPage struct {
	Data       []Material `json:"data"`
	NextCursor string     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

// Job 表示训练或评估兼容任务。
type Job struct {
	JobID      string         `json:"job_id"`
	Type       string         `json:"type"`
	Twin       string         `json:"twin"`
	Status     string         `json:"status"`
	Progress   *int           `json:"progress"`
	Result     map[string]any `json:"result"`
	Error      map[string]any `json:"error"`
	CreatedAt  string         `json:"created_at"`
	StartedAt  string         `json:"started_at"`
	FinishedAt string         `json:"finished_at"`
}

// JobPage 表示旧任务游标分页结构。
type JobPage struct {
	Data       []Job  `json:"data"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

// Version 表示 Twin 训练版本。
type Version struct {
	Version        string `json:"version"`
	Mode           string `json:"mode"`
	Active         bool   `json:"active"`
	TrainingJobID  string `json:"training_job_id"`
	MaterialsCount int    `json:"materials_count"`
	CreatedAt      string `json:"created_at"`
}

// VersionPage 表示旧版本游标分页结构。
type VersionPage struct {
	Data       []Version `json:"data"`
	NextCursor string    `json:"next_cursor"`
	HasMore    bool      `json:"has_more"`
}

// Evaluation 表示 Twin 版本评估结果。
type Evaluation struct {
	EvaluationID string  `json:"evaluation_id"`
	Twin         string  `json:"twin"`
	Version      string  `json:"version"`
	CreatedAt    string  `json:"created_at"`
	OverallScore float64 `json:"overall_score"`
	Dimensions   []struct {
		Name    string  `json:"name"`
		Score   float64 `json:"score"`
		Comment string  `json:"comment"`
	} `json:"dimensions"`
	SummaryMD string `json:"summary_md"`
}

// EvaluationPage 表示旧评估游标分页结构。
type EvaluationPage struct {
	Data       []Evaluation `json:"data"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

// ChatRequest 表示 Twin 对话请求。
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse 表示 Twin 对话兼容响应。
type ChatResponse struct {
	MessageID string `json:"message_id"`
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	Fallback  bool   `json:"fallback"`
	CreatedAt string `json:"created_at"`
}

// New 创建优先使用当前进程内 Backend 的兼容 Client。
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{cfg: cfg, backend: currentBackend()}
}

// CreateTwin 创建 Twin。
func (c *Client) CreateTwin(ctx context.Context, ownerID string, req map[string]any) (*Twin, int, error) {
	if c.backend != nil {
		return c.backend.CreateTwin(ctx, ownerID, req)
	}
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPost, "/twins", ownerID, req, &out)
	return &out, code, err
}

// ListTwins 查询 Owner 的 Twin 列表。
func (c *Client) ListTwins(ctx context.Context, ownerID string, query string) ([]Twin, int, error) {
	if c.backend != nil {
		return c.backend.ListTwins(ctx, ownerID, query)
	}
	var out TwinPage
	code, err := c.requestJSON(ctx, http.MethodGet, "/twins"+query, ownerID, nil, &out)
	return out.Data, code, err
}

// GetTwin 查询单个 Twin。
func (c *Client) GetTwin(ctx context.Context, ownerID, uniqueName string) (*Twin, int, error) {
	if c.backend != nil {
		return c.backend.GetTwin(ctx, ownerID, uniqueName)
	}
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s", uniqueName), ownerID, nil, &out)
	return &out, code, err
}

// UpdateTwin 更新 Twin。
func (c *Client) UpdateTwin(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Twin, int, error) {
	if c.backend != nil {
		return c.backend.UpdateTwin(ctx, ownerID, uniqueName, req)
	}
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPatch, fmt.Sprintf("/twins/%s", uniqueName), ownerID, req, &out)
	return &out, code, err
}

// DeleteTwin 删除 Twin。
func (c *Client) DeleteTwin(ctx context.Context, ownerID, uniqueName string) (int, error) {
	if c.backend != nil {
		return c.backend.DeleteTwin(ctx, ownerID, uniqueName)
	}
	return c.requestNoBody(ctx, http.MethodDelete, fmt.Sprintf("/twins/%s", uniqueName), ownerID)
}

// AddMaterial 写入结构化材料。
func (c *Client) AddMaterial(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Material, int, error) {
	return c.AddMaterialJSON(ctx, ownerID, uniqueName, req)
}

// AddMaterialJSON 写入 JSON 材料。
func (c *Client) AddMaterialJSON(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Material, int, error) {
	if c.backend != nil {
		return c.backend.AddMaterial(ctx, ownerID, uniqueName, req)
	}
	var out Material
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/materials", uniqueName), ownerID, req, &out)
	return &out, code, err
}

// AddMaterialFile 写入文件材料。
func (c *Client) AddMaterialFile(ctx context.Context, ownerID, uniqueName, fileName string, file io.Reader, title string, extraFields map[string]string) (*Material, int, error) {
	var out Material
	if c.backend != nil {
		content, err := io.ReadAll(file)
		if err != nil {
			return nil, 0, err
		}
		return c.backend.AddMaterialFile(ctx, ownerID, uniqueName, fileName, content, title, extraFields)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if title != "" {
		_ = writer.WriteField("title", title)
	}
	if extraFields != nil {
		for k, v := range extraFields {
			_ = writer.WriteField(k, v)
		}
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, 0, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, 0, err
	}
	if err := writer.Close(); err != nil {
		return nil, 0, err
	}

	headers := map[string]string{
		"X-Owner-Id":   ownerID,
		"Content-Type": writer.FormDataContentType(),
	}
	code, err := c.requestBytesWithHeaders(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/materials", uniqueName), headers, buf.Bytes(), &out)
	return &out, code, err
}

// ListMaterials 查询材料列表。
func (c *Client) ListMaterials(ctx context.Context, ownerID, uniqueName, query string) ([]Material, int, error) {
	if c.backend != nil {
		return c.backend.ListMaterials(ctx, ownerID, uniqueName, query)
	}
	var out MaterialPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/materials%s", uniqueName, query), ownerID, nil, &out)
	return out.Data, code, err
}

// DeleteMaterial 删除材料。
func (c *Client) DeleteMaterial(ctx context.Context, ownerID, uniqueName, materialID string) (int, error) {
	if c.backend != nil {
		return c.backend.DeleteMaterial(ctx, ownerID, uniqueName, materialID)
	}
	return c.requestNoBody(ctx, http.MethodDelete, fmt.Sprintf("/twins/%s/materials/%s", uniqueName, materialID), ownerID)
}

// StartTraining 启动训练任务。
func (c *Client) StartTraining(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Job, int, error) {
	if c.backend != nil {
		return c.backend.StartTraining(ctx, ownerID, uniqueName, req)
	}
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/training", uniqueName), ownerID, req, &out)
	return &out, code, err
}

// StartEvaluation 启动评估任务。
func (c *Client) StartEvaluation(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Job, int, error) {
	if c.backend != nil {
		return c.backend.StartEvaluation(ctx, ownerID, uniqueName, req)
	}
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/evaluations", uniqueName), ownerID, req, &out)
	return &out, code, err
}

// GetEvaluation 查询评估结果。
func (c *Client) GetEvaluation(ctx context.Context, ownerID, uniqueName, evaluationID string) (*Evaluation, int, error) {
	if c.backend != nil {
		return c.backend.GetEvaluation(ctx, ownerID, uniqueName, evaluationID)
	}
	var out Evaluation
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/evaluations/%s", uniqueName, evaluationID), ownerID, nil, &out)
	return &out, code, err
}

// ListEvaluations 查询评估结果列表。
func (c *Client) ListEvaluations(ctx context.Context, ownerID, uniqueName, query string) ([]Evaluation, int, error) {
	if c.backend != nil {
		return c.backend.ListEvaluations(ctx, ownerID, uniqueName, query)
	}
	var out EvaluationPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/evaluations%s", uniqueName, query), ownerID, nil, &out)
	return out.Data, code, err
}

// ListJobs 查询任务列表。
func (c *Client) ListJobs(ctx context.Context, ownerID, uniqueName, query string) ([]Job, int, error) {
	if c.backend != nil {
		return c.backend.ListJobs(ctx, ownerID, uniqueName, query)
	}
	var out JobPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/jobs%s", uniqueName, query), ownerID, nil, &out)
	return out.Data, code, err
}

// GetJob 查询单个任务。
func (c *Client) GetJob(ctx context.Context, ownerID, jobID string) (*Job, int, error) {
	if c.backend != nil {
		return c.backend.GetJob(ctx, ownerID, jobID)
	}
	var out Job
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/jobs/%s", jobID), ownerID, nil, &out)
	if err != nil {
		log.Printf("[agentclient] GET /jobs/%s response error: code=%d err=%v", jobID, code, err)
	} else {
		log.Printf("[agentclient] GET /jobs/%s response: %s", jobID, tools.ToJson(out))
	}
	return &out, code, err
}

// CancelJob 取消任务。
func (c *Client) CancelJob(ctx context.Context, ownerID, jobID string) (*Job, int, error) {
	if c.backend != nil {
		return c.backend.CancelJob(ctx, ownerID, jobID)
	}
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/jobs/%s/cancel", jobID), ownerID, nil, &out)
	return &out, code, err
}

// ListVersions 查询版本列表。
func (c *Client) ListVersions(ctx context.Context, ownerID, uniqueName string) ([]Version, int, error) {
	if c.backend != nil {
		return c.backend.ListVersions(ctx, ownerID, uniqueName)
	}
	var out VersionPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/versions", uniqueName), ownerID, nil, &out)
	return out.Data, code, err
}

// ActivateVersion 激活指定版本。
func (c *Client) ActivateVersion(ctx context.Context, ownerID, uniqueName, version string) (*Twin, int, error) {
	if c.backend != nil {
		return c.backend.ActivateVersion(ctx, ownerID, uniqueName, version)
	}
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/versions/%s/activate", uniqueName, version), ownerID, nil, &out)
	return &out, code, err
}

// GetCurrentVersion 查询当前版本。
func (c *Client) GetCurrentVersion(ctx context.Context, ownerID, uniqueName string) (*Version, int, error) {
	if c.backend != nil {
		return c.backend.GetCurrentVersion(ctx, ownerID, uniqueName)
	}
	var out Version
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/versions/current", uniqueName), ownerID, nil, &out)
	return &out, code, err
}

// GetTwinCurrentVersion 是 GetCurrentVersion 的旧名称兼容方法。
func (c *Client) GetTwinCurrentVersion(ctx context.Context, ownerID, uniqueName string) (*Version, int, error) {
	return c.GetCurrentVersion(ctx, ownerID, uniqueName)
}

// Chat 通过 Twin 映射执行对话。
func (c *Client) Chat(ctx context.Context, customerID, customerSource, uniqueName string, req ChatRequest) (*ChatResponse, int, error) {
	if c.backend != nil {
		return c.backend.Chat(ctx, customerID, customerSource, uniqueName, req)
	}
	var out ChatResponse
	headers := map[string]string{
		"X-Customer-Id": customerID,
	}
	if customerSource != "" {
		headers["X-Customer-Source"] = customerSource
	}
	code, err := c.requestJSONWithHeaders(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/chat", uniqueName), headers, req, &out)
	if err != nil {
		log.Printf("[agentclient] Chat error: unique_name=%s code=%d err=%v", uniqueName, code, err)
	} else if out.Reply == "" {
		log.Printf("[agentclient] Chat returned empty reply: unique_name=%s code=%d message_id=%s session_id=%s fallback=%v created_at=%s",
			uniqueName, code, out.MessageID, out.SessionID, out.Fallback, out.CreatedAt)
	} else {
		log.Printf("[agentclient] chat response parsed: code=%d message_id=%s session_id=%s reply=%q fallback=%v created_at=%s",
			code, out.MessageID, out.SessionID, out.Reply, out.Fallback, out.CreatedAt)
	}
	return &out, code, err
}

func (c *Client) requestNoBody(ctx context.Context, method, path, ownerID string) (int, error) {
	return c.requestJSONWithHeaders(ctx, method, path, map[string]string{
		"X-Owner-Id": ownerID,
	}, nil, nil)
}

func (c *Client) requestJSON(ctx context.Context, method, path, ownerID string, req any, out any) (int, error) {
	return c.requestJSONWithHeaders(ctx, method, path, map[string]string{
		"X-Owner-Id": ownerID,
	}, req, out)
}

func (c *Client) requestJSONWithHeaders(ctx context.Context, method, path string, headers map[string]string, req any, out any) (int, error) {
	if strings.HasPrefix(path, "/twins/") || path == "/twins" || strings.HasPrefix(path, "/jobs/") || path == "/jobs" {
		return http.StatusServiceUnavailable, ErrBackendUnavailable
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	if (strings.HasPrefix(path, "/twins/") || path == "/twins") ||
		(strings.HasPrefix(path, "/jobs/") || path == "/jobs") {
		token := c.cfg.TwinsToken
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		} else if c.cfg.Authorization != "" {
			headers["Authorization"] = "Bearer " + c.cfg.Authorization
		}
	} else if c.cfg.Authorization != "" {
		headers["Authorization"] = "Bearer " + c.cfg.Authorization
	}
	log.Printf("[agentclient]   %s", headers)
	body := ""
	if req != nil {
		body = tools.ToJson(req)
		headers["Content-Type"] = "application/json"
	}
	fullURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	log.Printf("[agentclient] %s %s", method, fullURL)
	for k, v := range headers {
		log.Printf("[agentclient]   %s: %s", k, v)
	}
	if body != "" {
		log.Printf("[agentclient]   body: %s", body)
	}
	log.Printf("[agentclient] %s %s timeout=%v", method, fullURL, c.cfg.Timeout)
	respBody, code, err := tools.HttpDoBytesWithTimeout(method, fullURL, headers, body, c.cfg.Timeout)
	if err != nil {
		log.Printf("[agentclient] HTTP request failed: %s %s err=%v", method, fullURL, err)
		return code, err
	}
	if code < 200 || code >= 300 {
		log.Printf("[agentclient] non-200 HTTP status: %s %s code=%d body=%s", method, fullURL, code, string(respBody))
	}
	if len(respBody) == 0 {
		log.Printf("[agentclient] empty response body: %s %s code=%d", method, fullURL, code)
	}
	if strings.Contains(path, "/chat") || strings.Contains(path, "/jobs") {
		log.Printf("[agentclient] %s %s raw response: code=%d body=%s", method, path, code, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			log.Printf("[agentclient] json unmarshal failed: %s %s err=%v body=%s", method, fullURL, err, string(respBody))
			return code, err
		}
	}
	return code, nil
}

func (c *Client) requestBytesWithHeaders(ctx context.Context, method, path string, headers map[string]string, body []byte, out any) (int, error) {
	if strings.HasPrefix(path, "/twins/") || path == "/twins" || strings.HasPrefix(path, "/jobs/") || path == "/jobs" {
		return http.StatusServiceUnavailable, ErrBackendUnavailable
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	if (strings.HasPrefix(path, "/twins/") || path == "/twins") ||
		(strings.HasPrefix(path, "/jobs/") || path == "/jobs") {
		if c.cfg.TwinsToken != "" {
			headers["Authorization"] = "Bearer " + c.cfg.TwinsToken
		} else if c.cfg.Authorization != "" {
			headers["Authorization"] = "Bearer " + c.cfg.Authorization
		}
	} else if c.cfg.Authorization != "" {
		headers["Authorization"] = "Bearer " + c.cfg.Authorization
	}

	fullURL := strings.TrimRight(c.cfg.BaseURL, "/") + path
	log.Printf("[agentclient] %s %s timeout=%v", method, fullURL, c.cfg.Timeout)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: c.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// BaseURL 返回历史 HTTP 地址，仅用于兼容配置检查。
func (c *Client) BaseURL() string {
	return c.cfg.BaseURL
}

// Timeout 返回历史客户端超时配置。
func (c *Client) Timeout() time.Duration {
	return c.cfg.Timeout
}

// OwnerID 返回历史客户端默认 Owner 配置。
func (c *Client) OwnerID() string {
	return c.cfg.OwnerID
}

// ToErrorResponse 解析旧服务错误响应。
func ToErrorResponse(bs []byte) (*ErrorResponse, error) {
	var out ErrorResponse
	if len(bs) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(bs, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
