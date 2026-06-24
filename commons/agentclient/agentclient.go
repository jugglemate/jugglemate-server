package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/commons/tools"
)

type Config struct {
	BaseURL       string
	Timeout       time.Duration
	OwnerID       string
	Authorization string // Bearer token
	TwinsToken    string // /twins 接口专用 token
}

type Client struct {
	cfg Config
}

type ErrorResponse struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

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

type TwinPage struct {
	Data       []Twin `json:"data"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type Material struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	SizeBytes int64  `json:"size_bytes"`
	CreatedAt string `json:"created_at"`
}

type MaterialPage struct {
	Data       []Material `json:"data"`
	NextCursor string     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}

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

type JobPage struct {
	Data       []Job  `json:"data"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type Version struct {
	Version        string `json:"version"`
	Mode           string `json:"mode"`
	Active         bool   `json:"active"`
	TrainingJobID  string `json:"training_job_id"`
	MaterialsCount int    `json:"materials_count"`
	CreatedAt      string `json:"created_at"`
}

type VersionPage struct {
	Data       []Version `json:"data"`
	NextCursor string    `json:"next_cursor"`
	HasMore    bool      `json:"has_more"`
}

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

type EvaluationPage struct {
	Data       []Evaluation `json:"data"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	MessageID string `json:"message_id"`
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	Fallback  bool   `json:"fallback"`
	CreatedAt string `json:"created_at"`
}

func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{cfg: cfg}
}

func (c *Client) CreateTwin(ctx context.Context, ownerID string, req map[string]any) (*Twin, int, error) {
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPost, "/twins", ownerID, req, &out)
	return &out, code, err
}

func (c *Client) ListTwins(ctx context.Context, ownerID string, query string) ([]Twin, int, error) {
	var out TwinPage
	code, err := c.requestJSON(ctx, http.MethodGet, "/twins"+query, ownerID, nil, &out)
	return out.Data, code, err
}

func (c *Client) GetTwin(ctx context.Context, ownerID, uniqueName string) (*Twin, int, error) {
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s", uniqueName), ownerID, nil, &out)
	return &out, code, err
}

func (c *Client) UpdateTwin(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Twin, int, error) {
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPatch, fmt.Sprintf("/twins/%s", uniqueName), ownerID, req, &out)
	return &out, code, err
}

func (c *Client) DeleteTwin(ctx context.Context, ownerID, uniqueName string) (int, error) {
	return c.requestNoBody(ctx, http.MethodDelete, fmt.Sprintf("/twins/%s", uniqueName), ownerID)
}

func (c *Client) AddMaterial(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Material, int, error) {
	return c.AddMaterialJSON(ctx, ownerID, uniqueName, req)
}

func (c *Client) AddMaterialJSON(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Material, int, error) {
	var out Material
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/materials", uniqueName), ownerID, req, &out)
	return &out, code, err
}

func (c *Client) AddMaterialFile(ctx context.Context, ownerID, uniqueName, fileName string, file io.Reader, title string, extraFields map[string]string) (*Material, int, error) {
	var out Material

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

func (c *Client) ListMaterials(ctx context.Context, ownerID, uniqueName, query string) ([]Material, int, error) {
	var out MaterialPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/materials%s", uniqueName, query), ownerID, nil, &out)
	return out.Data, code, err
}

func (c *Client) DeleteMaterial(ctx context.Context, ownerID, uniqueName, materialID string) (int, error) {
	return c.requestNoBody(ctx, http.MethodDelete, fmt.Sprintf("/twins/%s/materials/%s", uniqueName, materialID), ownerID)
}

func (c *Client) StartTraining(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Job, int, error) {
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/training", uniqueName), ownerID, req, &out)
	return &out, code, err
}

func (c *Client) StartEvaluation(ctx context.Context, ownerID, uniqueName string, req map[string]any) (*Job, int, error) {
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/evaluations", uniqueName), ownerID, req, &out)
	return &out, code, err
}

func (c *Client) GetEvaluation(ctx context.Context, ownerID, uniqueName, evaluationID string) (*Evaluation, int, error) {
	var out Evaluation
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/evaluations/%s", uniqueName, evaluationID), ownerID, nil, &out)
	return &out, code, err
}

func (c *Client) ListJobs(ctx context.Context, ownerID, uniqueName, query string) ([]Job, int, error) {
	var out JobPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/jobs%s", uniqueName, query), ownerID, nil, &out)
	return out.Data, code, err
}

func (c *Client) GetJob(ctx context.Context, ownerID, jobID string) (*Job, int, error) {
	var out Job
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/jobs/%s", jobID), ownerID, nil, &out)
	if err != nil {
		log.Printf("[agentclient] GET /jobs/%s response error: code=%d err=%v", jobID, code, err)
	} else {
		log.Printf("[agentclient] GET /jobs/%s response: %s", jobID, tools.ToJson(out))
	}
	return &out, code, err
}

func (c *Client) CancelJob(ctx context.Context, ownerID, jobID string) (*Job, int, error) {
	var out Job
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/jobs/%s/cancel", jobID), ownerID, nil, &out)
	return &out, code, err
}

func (c *Client) ListVersions(ctx context.Context, ownerID, uniqueName string) ([]Version, int, error) {
	var out VersionPage
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/versions", uniqueName), ownerID, nil, &out)
	return out.Data, code, err
}

func (c *Client) ActivateVersion(ctx context.Context, ownerID, uniqueName, version string) (*Twin, int, error) {
	var out Twin
	code, err := c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/twins/%s/versions/%s/activate", uniqueName, version), ownerID, nil, &out)
	return &out, code, err
}

func (c *Client) GetCurrentVersion(ctx context.Context, ownerID, uniqueName string) (*Version, int, error) {
	var out Version
	code, err := c.requestJSON(ctx, http.MethodGet, fmt.Sprintf("/twins/%s/versions/current", uniqueName), ownerID, nil, &out)
	return &out, code, err
}

func (c *Client) GetTwinCurrentVersion(ctx context.Context, ownerID, uniqueName string) (*Version, int, error) {
	return c.GetCurrentVersion(ctx, ownerID, uniqueName)
}

func (c *Client) Chat(ctx context.Context, customerID, customerSource, uniqueName string, req ChatRequest) (*ChatResponse, int, error) {
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

func (c *Client) BaseURL() string {
	return c.cfg.BaseURL
}

func (c *Client) Timeout() time.Duration {
	return c.cfg.Timeout
}

func (c *Client) OwnerID() string {
	return c.cfg.OwnerID
}

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
