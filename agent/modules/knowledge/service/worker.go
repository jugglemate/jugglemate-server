package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/model"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/repository"
	llmservice "github.com/juggleim/jugglemate-server/agent/modules/llm/service"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"github.com/ledongthuc/pdf"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/html"
)

// Worker 消费 Redis Stream 并执行真实文档向量化。
type Worker struct {
	repo    *repository.Repository
	redis   *redis.Client
	llm     *llmservice.CallService
	billing *billingservice.Service
	cfg     configures.AgentKnowledgeConfig
	cancel  context.CancelFunc
	wait    sync.WaitGroup
}

// NewWorker 创建 Knowledge 向量化 Worker。
func NewWorker(repo *repository.Repository, redisClient *redis.Client, llm *llmservice.CallService, billing *billingservice.Service, cfg configures.AgentKnowledgeConfig) *Worker {
	return &Worker{repo: repo, redis: redisClient, llm: llm, billing: billing, cfg: cfg}
}

// Start 创建消费者组并启动消费循环。
func (worker *Worker) Start(ctx context.Context) error {
	if worker == nil || !worker.cfg.WorkerEnabled {
		return nil
	}
	err := worker.redis.XGroupCreateMkStream(ctx, worker.cfg.VectorizeStream, worker.cfg.ConsumerGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("创建 Knowledge Redis 消费组失败: %w", err)
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	worker.cancel = cancel
	worker.wait.Add(1)
	go func() { defer worker.wait.Done(); worker.run(workerCtx) }()
	return nil
}

// Stop 停止消费循环并等待当前任务退出。
func (worker *Worker) Stop(ctx context.Context) error {
	if worker == nil || worker.cancel == nil {
		return nil
	}
	worker.cancel()
	done := make(chan struct{})
	go func() { worker.wait.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run 持续消费新任务；Redis 暂时不可用时退避重试。
func (worker *Worker) run(ctx context.Context) {
	for ctx.Err() == nil {
		streams, err := worker.redis.XReadGroup(ctx, &redis.XReadGroupArgs{Group: worker.cfg.ConsumerGroup, Consumer: worker.cfg.ConsumerName, Streams: []string{worker.cfg.VectorizeStream, ">"}, Count: 1, Block: time.Second}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				worker.handle(ctx, message)
			}
		}
	}
}

// handle 执行任务并保证成功/最终失败后 ACK，暂态失败则重新入队。
//
// 简要描述：数据库状态先进入 processing，向量批量生成成功后一次事务替换；任何中途
// 失败都不会破坏旧向量。失败在最大次数内重新入队，超限后落 failed 状态并 ACK。
func (worker *Worker) handle(ctx context.Context, message redis.XMessage) {
	knowledgeID := valueString(message.Values["knowledge_id"])
	taskID := valueString(message.Values["task_id"])
	retry, _ := strconv.Atoi(valueString(message.Values["retry_count"]))
	if knowledgeID == "" || taskID == "" {
		_, _ = worker.redis.XAck(ctx, worker.cfg.VectorizeStream, worker.cfg.ConsumerGroup, message.ID).Result()
		return
	}
	logEntry := logs.WithContext(ctx).WithField("module", "agent.knowledge.worker").WithField("knowledge_id", knowledgeID).WithField("task_id", taskID)
	err := worker.process(ctx, knowledgeID, taskID)
	if err != nil {
		logEntry.WithField("retry", retry).Errorf("知识库向量化任务失败 error:%v", err)
		retry++
		if retry <= worker.cfg.MaxRetries {
			_ = worker.repo.FailTask(context.WithoutCancel(ctx), taskID, "VECTORIZE_RETRY", err.Error(), retry)
			_ = worker.redis.XAdd(context.WithoutCancel(ctx), &redis.XAddArgs{Stream: worker.cfg.VectorizeStream, Values: map[string]any{"knowledge_id": knowledgeID, "task_id": taskID, "retry_count": retry, "enqueued_at": time.Now().UTC().Format(time.RFC3339Nano)}}).Err()
		} else {
			_ = worker.repo.FailTask(context.WithoutCancel(ctx), taskID, "VECTORIZE_FAILED", err.Error(), retry)
		}
	} else {
		logEntry.Infof("知识库向量化任务完成")
	}
	_, _ = worker.redis.XAck(context.WithoutCancel(ctx), worker.cfg.VectorizeStream, worker.cfg.ConsumerGroup, message.ID).Result()
}

func (worker *Worker) process(ctx context.Context, knowledgeID, taskID string) error {
	entity, err := worker.repo.Find(ctx, knowledgeID)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := worker.repo.MarkTask(ctx, taskID, map[string]any{"status": "processing", "stage": "extracting", "progress": 10, "started_at": now}, map[string]any{"status": "processing"}); err != nil {
		return err
	}
	path, file, cleanup, err := worker.source(ctx, entity)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	text, err := extractText(path)
	if err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("文档未提取到有效文本")
	}
	chunks := splitText(text, entity.ChunkSize, entity.OverlapWindow)
	if len(chunks) == 0 {
		return errors.New("文档分块结果为空")
	}
	if err := worker.repo.MarkTask(ctx, taskID, map[string]any{"stage": "embedding", "progress": 35}, map[string]any{"status": "processing"}); err != nil {
		return err
	}
	vectors := make([]model.Vector, 0, len(chunks))
	totalTokens := 0
	resolvedModelID := ""
	var resolvedProviderID uuid.UUID
	batchSize := worker.cfg.VectorBatchSize
	if batchSize <= 0 {
		batchSize = 32
	}
	for start := 0; start < len(chunks); start += batchSize {
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		call, err := worker.llm.EmbedForSettlement(ctx, resolvedModelID, chunks[start:end])
		if err != nil {
			return err
		}
		totalTokens += call.Usage.TotalTokens
		resolvedModelID = call.ModelID
		resolvedProviderID = call.ProviderID
		if len(call.Embeddings) > 0 && len(call.Embeddings[0]) != worker.cfg.VectorDimension {
			logs.WithContext(ctx).
				WithField("module", "agent.knowledge.worker").
				WithField("knowledge_id", knowledgeID).
				WithField("task_id", taskID).
				WithField("model_id", call.ModelID).
				WithField("embedding_dimension", len(call.Embeddings[0])).
				WithField("vector_dimension", worker.cfg.VectorDimension).
				WithField("batch_start", start).
				WithField("batch_size", len(call.Embeddings)).
				Warnf("Embedding 原始维度与知识库存储维度不一致，已归一化")
		}
		for index, embedding := range call.Embeddings {
			normalized := normalizeVector(embedding, worker.cfg.VectorDimension)
			chunkIndex := start + index
			chunkID := uuid.NewString()
			vectors = append(vectors, model.Vector{ID: uuid.NewString(), KnowledgeID: knowledgeID, ChunkID: &chunkID, DocumentIndex: 0, ChunkIndex: chunkIndex, ContentChunk: chunks[chunkIndex], Embedding: normalized, SourcePosition: map[string]any{"file_id": file.ID, "chunk_index": chunkIndex}})
		}
		progress := 35 + float64(end)*55/float64(len(chunks))
		_ = worker.repo.MarkTask(ctx, taskID, map[string]any{"stage": "embedding", "progress": progress}, map[string]any{"status": "processing"})
	}
	if worker.billing != nil && totalTokens > 0 {
		if _, err := worker.billing.ChargeEmbeddingCallWithFact(ctx, entity.OwnerID, knowledgeID, resolvedProviderID, resolvedModelID, totalTokens); err != nil {
			return fmt.Errorf("结算 Embedding 消耗失败: %w", err)
		}
	}
	return worker.repo.ReplaceVectors(ctx, taskID, vectors, &text)
}

func (worker *Worker) source(ctx context.Context, entity *model.Knowledge) (string, *model.File, func(), error) {
	if entity.SourceType == "file" {
		if entity.FileID == nil {
			return "", nil, nil, errors.New("知识库未绑定文件")
		}
		file, err := worker.repo.FindFile(ctx, *entity.FileID)
		if err != nil {
			return "", nil, nil, err
		}
		if strings.HasPrefix(file.StoredPath, "pending://") {
			return "", nil, nil, errors.New("仅收到文件元数据，尚未上传真实内容")
		}
		return file.StoredPath, file, nil, nil
	}
	if entity.SourceURL == nil {
		return "", nil, nil, errors.New("知识库未配置来源 URL")
	}
	if err := validateRemoteURL(ctx, *entity.SourceURL); err != nil {
		return "", nil, nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, *entity.SourceURL, nil)
	if err != nil {
		return "", nil, nil, err
	}
	client := safeHTTPClient(time.Duration(worker.cfg.TaskTimeoutSeconds) * time.Second)
	response, err := client.Do(request)
	if err != nil {
		return "", nil, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", nil, nil, fmt.Errorf("下载文档失败: HTTP %d", response.StatusCode)
	}
	limit := int64(worker.cfg.MaxFileSizeMB) * 1024 * 1024
	content, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", nil, nil, err
	}
	if int64(len(content)) > limit {
		return "", nil, nil, errors.New("文档大小超过限制")
	}
	ext := strings.ToLower(filepath.Ext(response.Request.URL.Path))
	if ext == "" {
		mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
		ext = extensionForMIME(mediaType)
	}
	if !allowedExtension(ext) {
		return "", nil, nil, errors.New("文档格式不支持")
	}
	root, _ := filepath.Abs(worker.cfg.ObjectStorageRoot)
	directory := filepath.Join(root, "documents")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", nil, nil, err
	}
	fileID := uuid.NewString()
	path := filepath.Join(directory, fileID+ext)
	if err := os.WriteFile(path, content, 0o640); err != nil {
		return "", nil, nil, err
	}
	hashBytes := sha256.Sum256(content)
	hash := hex.EncodeToString(hashBytes[:])
	file := &model.File{ID: fileID, KnowledgeID: entity.ID, OriginalName: filepath.Base(response.Request.URL.Path), StoredPath: path, FileSize: int64(len(content)), MIMEType: response.Header.Get("Content-Type"), ContentHash: &hash}
	if file.OriginalName == "." || file.OriginalName == "/" || file.OriginalName == "" {
		file.OriginalName = fileID + ext
	}
	if err := worker.repo.CreateFile(ctx, file, false); err != nil {
		_ = os.Remove(path)
		return "", nil, nil, err
	}
	return path, file, nil, nil
}

func extractText(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		file, reader, err := pdf.Open(path)
		if err != nil {
			return "", err
		}
		defer file.Close()
		plain, err := reader.GetPlainText()
		if err != nil {
			return "", err
		}
		raw, err := io.ReadAll(plain)
		return string(raw), err
	case ".html", ".htm":
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		document, err := html.Parse(bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		var builder strings.Builder
		var walk func(*html.Node)
		walk = func(node *html.Node) {
			if node.Type == html.TextNode {
				text := strings.TrimSpace(node.Data)
				if text != "" {
					builder.WriteString(text)
					builder.WriteByte('\n')
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(document)
		return builder.String(), nil
	default:
		raw, err := os.ReadFile(path)
		return string(raw), err
	}
}

func splitText(text string, chunkSize, overlap int) []string {
	runes := []rune(text)
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}
	result := []string{}
	step := chunkSize - overlap
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			result = append(result, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return result
}
func extensionForMIME(value string) string {
	switch value {
	case "application/pdf":
		return ".pdf"
	case "text/html":
		return ".html"
	case "text/markdown":
		return ".md"
	case "text/plain":
		return ".txt"
	}
	return ""
}
func valueString(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case []byte:
		return string(item)
	default:
		return fmt.Sprint(item)
	}
}
