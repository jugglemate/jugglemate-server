// Package service 实现 Knowledge 状态机、文件接入、任务入队与检索编排。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/model"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/repository"
	llmservice "github.com/juggleim/jugglemate-server/agent/modules/llm/service"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"github.com/redis/go-redis/v9"
)

var transitions = map[string]map[string]bool{
	"draft": {"draft": true, "pending": true, "archived": true}, "pending": {"pending": true, "processing": true, "failed": true, "archived": true},
	"processing": {"processing": true, "ready": true, "failed": true, "archived": true}, "ready": {"ready": true, "pending": true, "archived": true},
	"failed": {"failed": true, "pending": true, "archived": true}, "archived": {"archived": true},
}

// Error 表示可映射 HTTP 状态与业务码的 Knowledge 异常。
type Error struct {
	Status  int
	Code    interface{}
	Message string
}

// Error 返回异常消息。
func (err *Error) Error() string { return err.Message }

// Service 管理知识库聚合与异步向量化任务。
type Service struct {
	repo       *repository.Repository
	redis      *redis.Client
	llm        *llmservice.CallService
	cfg        configures.AgentKnowledgeConfig
	httpClient *http.Client
}

// New 创建 Knowledge 业务服务。
func New(repo *repository.Repository, redisClient *redis.Client, llm *llmservice.CallService, cfg configures.AgentKnowledgeConfig) *Service {
	return &Service{repo: repo, redis: redisClient, llm: llm, cfg: cfg, httpClient: safeHTTPClient(time.Duration(cfg.TaskTimeoutSeconds) * time.Second)}
}

// Create 创建知识库，并在来源完备时按请求触发向量化。
func (service *Service) Create(ctx context.Context, ownerID string, payload dto.Create) (*model.Knowledge, error) {
	if payload.Type == "" {
		payload.Type = "offline_document"
	}
	if payload.SourceType == "" {
		payload.SourceType = "url"
	}
	if !validType(payload.Type) || (payload.SourceType != "url" && payload.SourceType != "file") {
		return nil, badRequest("400_INVALID_SOURCE_TYPE", "知识库类型或来源类型非法")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return nil, badRequest(400, "知识库名称不能为空")
	}
	if payload.SourceType == "url" && payload.SourceURL != nil {
		normalized := strings.TrimSpace(*payload.SourceURL)
		if normalized == "" {
			return nil, badRequest("400_SOURCE_URL_REQUIRED", "sourceUrl 不能为空")
		}
		payload.SourceURL = &normalized
	}
	entity := &model.Knowledge{ID: uuid.NewString(), OwnerID: ownerID, Name: strings.TrimSpace(payload.Name), Type: payload.Type, SourceType: payload.SourceType, SourceURL: payload.SourceURL, Status: "draft", ChunkSize: 500, OverlapWindow: 50}
	if err := service.repo.Create(ctx, entity, payload.AgentID); err != nil {
		if repository.IsNotFound(err) {
			return nil, notFound("关联 Agent 不存在")
		}
		return nil, err
	}
	trigger := payload.TriggerVectorize == nil || *payload.TriggerVectorize
	if trigger && ((entity.SourceType == "url" && entity.SourceURL != nil) || (entity.SourceType == "file" && entity.FileID != nil)) {
		_, _ = service.Trigger(ctx, ownerID, entity.ID)
	}
	return service.repo.Find(ctx, entity.ID)
}

// List 查询 Owner 的知识库分页列表。
func (service *Service) List(ctx context.Context, ownerID, status, agentID string, page, pageSize int) ([]model.Knowledge, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return service.repo.ListOwner(ctx, ownerID, status, agentID, page, pageSize)
}

// Get 获取 Owner 可访问的知识库。
func (service *Service) Get(ctx context.Context, ownerID, knowledgeID string) (*model.Knowledge, error) {
	entity, err := service.repo.Find(ctx, knowledgeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, notFound("知识库不存在: " + knowledgeID)
		}
		return nil, err
	}
	if entity.OwnerID != ownerID {
		return nil, forbidden("无权访问该知识库")
	}
	return entity, nil
}

// Update 更新知识库基础信息和分块配置，并可立即重新向量化。
func (service *Service) Update(ctx context.Context, ownerID string, payload dto.Update) (*model.Knowledge, *model.Task, error) {
	entity, err := service.Get(ctx, ownerID, payload.KnowledgeID)
	if err != nil {
		return nil, nil, err
	}
	values := map[string]any{}
	if payload.Name != nil {
		name := strings.TrimSpace(*payload.Name)
		if name == "" {
			return nil, nil, badRequest("400_INVALID_NAME", "知识库名称不能为空")
		}
		values["name"] = name
	}
	if payload.SourceURL != nil {
		if entity.SourceType != "url" {
			return nil, nil, badRequest("400_INVALID_SOURCE_TYPE", "仅 URL 类型知识库支持更新 sourceUrl")
		}
		value := strings.TrimSpace(*payload.SourceURL)
		if value == "" {
			return nil, nil, badRequest("400_SOURCE_URL_REQUIRED", "sourceUrl 不能为空")
		}
		values["source_url"] = value
	}
	if payload.ChunkSize != nil {
		if *payload.ChunkSize < 200 || *payload.ChunkSize > 1000 {
			return nil, nil, badRequest(422, "chunkSize 必须为 200-1000")
		}
		values["chunk_size"] = *payload.ChunkSize
	}
	if payload.OverlapWindow != nil {
		if *payload.OverlapWindow < 0 || *payload.OverlapWindow > 200 {
			return nil, nil, badRequest(422, "overlapWindow 必须为 0-200")
		}
		values["overlap_window"] = *payload.OverlapWindow
	}
	if len(values) > 0 {
		if err := service.repo.Update(ctx, entity.ID, values); err != nil {
			return nil, nil, err
		}
	}
	var task *model.Task
	if payload.TriggerRevectorize {
		task, err = service.Trigger(ctx, ownerID, entity.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	updated, err := service.repo.Find(ctx, entity.ID)
	return updated, task, err
}

// Delete 将知识库归档。
func (service *Service) Delete(ctx context.Context, ownerID, knowledgeID string) error {
	entity, err := service.Get(ctx, ownerID, knowledgeID)
	if err != nil {
		return err
	}
	if !transitions[entity.Status]["archived"] {
		return conflict("知识库当前状态不可归档")
	}
	return service.repo.Update(ctx, knowledgeID, map[string]any{"status": "archived", "current_task_id": nil, "deleted_at": time.Now()})
}

// Trigger 校验来源和嵌入模型后，创建真实 Redis Stream 任务。
func (service *Service) Trigger(ctx context.Context, ownerID, knowledgeID string) (*model.Task, error) {
	entity, err := service.Get(ctx, ownerID, knowledgeID)
	if err != nil {
		return nil, err
	}
	if !transitions[entity.Status]["pending"] {
		return nil, conflict("知识库当前状态不可触发向量化")
	}
	if entity.SourceType == "url" {
		if entity.SourceURL == nil {
			return nil, badRequest("400_SOURCE_URL_REQUIRED", "当前知识库缺少可向量化的文档 URL")
		}
		if err := validateRemoteURL(ctx, *entity.SourceURL); err != nil {
			return nil, err
		}
		if err := service.probeRemote(ctx, *entity.SourceURL); err != nil {
			return nil, err
		}
	} else if entity.FileID == nil {
		return nil, badRequest("400_FILE_NOT_UPLOADED", "当前知识库缺少可向量化的文件，请先上传文件")
	}
	available, err := service.llm.HasEmbeddingModel(ctx)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, badRequest("400_EMBEDDING_MODEL_NOT_FOUND", "未找到可用的 Embedding 模型")
	}
	task, err := service.repo.StartTask(ctx, knowledgeID)
	if err != nil {
		return nil, err
	}
	if err := service.redis.XAdd(ctx, &redis.XAddArgs{Stream: service.cfg.VectorizeStream, Values: map[string]any{"knowledge_id": knowledgeID, "task_id": task.ID, "enqueued_at": time.Now().UTC().Format(time.RFC3339Nano)}}).Err(); err != nil {
		_ = service.repo.FailTask(ctx, task.ID, "QUEUE_UNAVAILABLE", err.Error(), 0)
		return nil, fmt.Errorf("知识向量化任务入队失败: %w", err)
	}
	return task, nil
}

// UploadMetadata 兼容仅提交文件元数据的旧接口。
func (service *Service) UploadMetadata(ctx context.Context, ownerID, knowledgeID string, payload dto.FileMetadataUpload) (dto.FileProgress, error) {
	if _, err := service.Get(ctx, ownerID, knowledgeID); err != nil {
		return dto.FileProgress{}, err
	}
	if payload.TotalBytes > int64(service.cfg.MaxFileSizeMB)*1024*1024 {
		return dto.FileProgress{}, &Error{Status: http.StatusRequestEntityTooLarge, Code: "413_FILE_TOO_LARGE", Message: "文件大小超过限制"}
	}
	file := &model.File{ID: uuid.NewString(), KnowledgeID: knowledgeID, OriginalName: filepath.Base(payload.FileName), StoredPath: "pending://" + uuid.NewString(), FileSize: payload.TotalBytes, MIMEType: payload.MIMEType, ContentHash: payload.ContentHash}
	if err := service.repo.CreateFile(ctx, file, true); err != nil {
		return dto.FileProgress{}, err
	}
	return dto.FileProgress{FileID: file.ID, Status: "metadata_received", UploadedBytes: 0, TotalBytes: payload.TotalBytes}, nil
}

// UploadMultipart 保存真实文件并按需触发向量化。
func (service *Service) UploadMultipart(ctx context.Context, ownerID, knowledgeID string, header *multipart.FileHeader, trigger bool) (dto.FileProgress, error) {
	if _, err := service.Get(ctx, ownerID, knowledgeID); err != nil {
		return dto.FileProgress{}, err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtension(ext) {
		return dto.FileProgress{}, badRequest("400_INVALID_FORMAT", "文件格式不支持，仅支持 PDF、Markdown、HTML、TXT")
	}
	if header.Size > int64(service.cfg.MaxFileSizeMB)*1024*1024 {
		return dto.FileProgress{}, &Error{Status: http.StatusRequestEntityTooLarge, Code: "413_FILE_TOO_LARGE", Message: "文件大小超过限制"}
	}
	source, err := header.Open()
	if err != nil {
		return dto.FileProgress{}, err
	}
	defer source.Close()
	root, err := filepath.Abs(service.cfg.ObjectStorageRoot)
	if err != nil {
		return dto.FileProgress{}, err
	}
	directory := filepath.Join(root, "documents")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return dto.FileProgress{}, err
	}
	fileID := uuid.NewString()
	path := filepath.Join(directory, fileID+ext)
	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return dto.FileProgress{}, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(destination, hasher), io.LimitReader(source, int64(service.cfg.MaxFileSizeMB)*1024*1024+1))
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return dto.FileProgress{}, copyErr
	}
	if closeErr != nil {
		return dto.FileProgress{}, closeErr
	}
	if written > int64(service.cfg.MaxFileSizeMB)*1024*1024 {
		_ = os.Remove(path)
		return dto.FileProgress{}, &Error{Status: 413, Code: "413_FILE_TOO_LARGE", Message: "文件大小超过限制"}
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	file := &model.File{ID: fileID, KnowledgeID: knowledgeID, OriginalName: filepath.Base(header.Filename), StoredPath: path, FileSize: written, MIMEType: header.Header.Get("Content-Type"), ContentHash: &hash}
	if file.MIMEType == "" {
		file.MIMEType = "application/octet-stream"
	}
	if err := service.repo.CreateFile(ctx, file, true); err != nil {
		_ = os.Remove(path)
		return dto.FileProgress{}, err
	}
	if trigger {
		if _, err := service.Trigger(ctx, ownerID, knowledgeID); err != nil {
			return dto.FileProgress{}, err
		}
	}
	return dto.FileProgress{FileID: fileID, Status: "uploaded", UploadedBytes: written, TotalBytes: written}, nil
}

// FileProgress 查询文件接收进度。
func (service *Service) FileProgress(ctx context.Context, ownerID, knowledgeID, fileID string) (dto.FileProgress, error) {
	if _, err := service.Get(ctx, ownerID, knowledgeID); err != nil {
		return dto.FileProgress{}, err
	}
	file, err := service.repo.FindFile(ctx, fileID)
	if err != nil || file.KnowledgeID != knowledgeID {
		return dto.FileProgress{}, notFound("文件不存在: " + fileID)
	}
	status, uploaded := "uploaded", file.FileSize
	if strings.HasPrefix(file.StoredPath, "pending://") {
		status, uploaded = "metadata_received", 0
	}
	return dto.FileProgress{FileID: file.ID, Status: status, UploadedBytes: uploaded, TotalBytes: file.FileSize}, nil
}

// Progress 查询最近任务进度。
func (service *Service) Progress(ctx context.Context, ownerID, knowledgeID string) (dto.TaskProgress, error) {
	entity, err := service.Get(ctx, ownerID, knowledgeID)
	if err != nil {
		return dto.TaskProgress{}, err
	}
	task, taskErr := service.repo.FindTask(ctx, knowledgeID, entity.CurrentTaskID)
	if taskErr != nil && !repository.IsNotFound(taskErr) {
		return dto.TaskProgress{}, taskErr
	}
	if task == nil {
		stage, progress := "queued", 0.0
		if entity.Status == "ready" || entity.Status == "archived" {
			stage, progress = entity.Status, 100
		}
		return dto.TaskProgress{Status: entity.Status, Stage: stage, Progress: progress, ErrorCode: entity.LastErrorCode, ErrorMessage: entity.LastErrorMessage}, nil
	}
	return dto.TaskProgress{TaskID: &task.ID, Status: entity.Status, Stage: task.Stage, Progress: task.Progress, ErrorCode: task.ErrorCode, ErrorMessage: task.ErrorMessage, RetryCount: task.RetryCount}, nil
}

// Chunks 返回真实知识分块。
func (service *Service) Chunks(ctx context.Context, ownerID, knowledgeID string) ([]dto.Chunk, error) {
	if _, err := service.Get(ctx, ownerID, knowledgeID); err != nil {
		return nil, err
	}
	items, err := service.repo.ListChunks(ctx, knowledgeID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.Chunk, 0, len(items))
	for _, item := range items {
		result = append(result, dto.Chunk{ChunkIndex: item.ChunkIndex, Content: item.ContentChunk})
	}
	return result, nil
}

// Search 执行 Owner 隔离的全文检索。
func (service *Service) Search(ctx context.Context, ownerID, query string, topK int) ([]dto.SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	rows, err := service.repo.SearchKeyword(ctx, ownerID, query, topK)
	if err != nil {
		return nil, err
	}
	result := make([]dto.SearchResult, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.SearchResult{KnowledgeID: row.KnowledgeID, Snippet: row.ContentChunk, Score: row.Score})
	}
	return result, nil
}

// SearchForReasoning 对 Agent 已挂载知识库执行向量与全文融合检索。
//
// 简要描述：向量与关键词结果按知识分块去重，命中两路的分块获得融合加权；当嵌入
// 模型不可用时保留全文检索结果，使推理可以明确降级而不是整体失败。
func (service *Service) SearchForReasoning(ctx context.Context, ownerID string, knowledgeIDs []string, query string, topK int, metadata map[string]any) ([]dto.SearchResult, error) {
	if len(knowledgeIDs) == 0 || strings.TrimSpace(query) == "" {
		return []dto.SearchResult{}, nil
	}
	if topK < 1 {
		topK = 5
	}
	if topK > 20 {
		topK = 20
	}
	logEntry := logs.WithContext(ctx).
		WithField("module", "agent.knowledge").
		WithField("trace_id", mapString(metadata, "trace_id")).
		WithField("owner_id", ownerID).
		WithField("knowledge_count", len(knowledgeIDs))
	keywordRows, err := service.repo.SearchKeywordScoped(ctx, ownerID, knowledgeIDs, query, topK)
	if err != nil {
		logEntry.Errorf("知识库关键词检索失败 error:%v", err)
		return nil, err
	}
	vectorRows := []repository.SearchRow{}
	if service.llm != nil {
		embeddings, _, modelID, embedErr := service.llm.EmbedWithMetadata(ctx, "", []string{query}, metadata)
		logEntry.WithField("model_id", modelID)
		if embedErr == nil && len(embeddings) > 0 {
			rawDimension := len(embeddings[0])
			logEntry.WithField("embedding_dimension", rawDimension).WithField("vector_dimension", service.cfg.VectorDimension)
			if rawDimension == 0 {
				logEntry.Warnf("Embedding 模型返回空向量，知识检索降级为关键词")
			} else {
				if rawDimension != service.cfg.VectorDimension {
					logEntry.Warnf("查询向量维度与知识库存储维度不一致，已归一化")
				}
				normalized := normalizeVector(embeddings[0], service.cfg.VectorDimension).Slice()
				vectorRows, err = service.repo.SearchVectorScoped(ctx, ownerID, knowledgeIDs, normalized, topK)
			}
			if err != nil {
				logEntry.Errorf("知识库向量检索失败 error:%v", err)
				return nil, err
			}
		} else if embedErr != nil {
			logEntry.Warnf("生成查询向量失败，知识检索降级为关键词 error:%v", embedErr)
		} else {
			logEntry.Warnf("Embedding 模型未返回查询向量，知识检索降级为关键词")
		}
	}
	type merged struct {
		knowledgeID, snippet string
		score                float64
	}
	items := map[string]merged{}
	for _, row := range keywordRows {
		key := row.KnowledgeID + "\x00" + row.ContentChunk
		items[key] = merged{row.KnowledgeID, row.ContentChunk, row.Score * 0.4}
	}
	for _, row := range vectorRows {
		key := row.KnowledgeID + "\x00" + row.ContentChunk
		item := items[key]
		item.knowledgeID, item.snippet = row.KnowledgeID, row.ContentChunk
		item.score += row.Score * 0.6
		items[key] = item
	}
	result := make([]dto.SearchResult, 0, len(items))
	for _, item := range items {
		result = append(result, dto.SearchResult{KnowledgeID: item.knowledgeID, Snippet: item.snippet, Score: item.score})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	if len(result) > topK {
		result = result[:topK]
	}
	logEntry.WithField("keyword_hits", len(keywordRows)).WithField("vector_hits", len(vectorRows)).WithField("result_count", len(result)).Infof("知识库融合检索完成")
	return result, nil
}

func mapString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func validType(value string) bool {
	return value == "offline_document" || value == "web_crawler" || value == "api_sync"
}
func allowedExtension(value string) bool {
	switch value {
	case ".pdf", ".md", ".markdown", ".html", ".htm", ".txt":
		return true
	}
	return false
}
func validateRemoteURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return badRequest("400_INVALID_URL", "文档 URL 非法")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil {
		return badRequest("404_DOC_NOT_FOUND", "文档 URL 无法解析")
	}
	for _, address := range addresses {
		if address.IP.IsLoopback() || address.IP.IsPrivate() || address.IP.IsLinkLocalUnicast() || address.IP.IsUnspecified() {
			return forbidden("文档 URL 不允许访问内网地址")
		}
	}
	return nil
}

func (service *Service) probeRemote(ctx context.Context, raw string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return badRequest("400_INVALID_URL", "文档 URL 非法")
	}
	response, err := service.httpClient.Do(request)
	if err != nil {
		return badRequest("404_DOC_NOT_FOUND", "文档 URL 无法访问")
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return badRequest("404_DOC_NOT_FOUND", fmt.Sprintf("文档 URL 返回 HTTP %d", response.StatusCode))
	}
	if response.ContentLength > int64(service.cfg.MaxFileSizeMB)*1024*1024 {
		return &Error{Status: http.StatusRequestEntityTooLarge, Code: "413_FILE_TOO_LARGE", Message: "文档大小超过限制"}
	}
	ext := strings.ToLower(filepath.Ext(response.Request.URL.Path))
	if ext == "" {
		mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		ext = extensionForMIME(mediaType)
	}
	if !allowedExtension(ext) {
		return badRequest("400_INVALID_FORMAT", "文档格式不支持，仅支持 PDF、Markdown、HTML、TXT")
	}
	return nil
}

// safeHTTPClient 在实际建连阶段再次解析并拒绝内网地址，避免 DNS rebinding 绕过 URL 校验。
func safeHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, item := range addresses {
			if !item.IP.IsLoopback() && !item.IP.IsPrivate() && !item.IP.IsLinkLocalUnicast() && !item.IP.IsUnspecified() {
				return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
			}
		}
		return nil, fmt.Errorf("目标地址不允许访问")
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("重定向次数过多")
		}
		return validateRemoteURL(request.Context(), request.URL.String())
	}}
}
func badRequest(code interface{}, message string) error {
	return &Error{Status: http.StatusBadRequest, Code: code, Message: message}
}
func forbidden(message string) error {
	return &Error{Status: http.StatusForbidden, Code: 403, Message: message}
}
func notFound(message string) error {
	return &Error{Status: http.StatusNotFound, Code: 404, Message: message}
}
func conflict(message string) error {
	return &Error{Status: http.StatusConflict, Code: 409, Message: message}
}
