// Package service 实现渐进式推理、上下文预算、知识增强与计费编排。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	knowledgedto "github.com/juggleim/jugglemate-server/agent/modules/knowledge/dto"
	knowledgeservice "github.com/juggleim/jugglemate-server/agent/modules/knowledge/service"
	llmdto "github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	llmservice "github.com/juggleim/jugglemate-server/agent/modules/llm/service"
	"github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	toolsservice "github.com/juggleim/jugglemate-server/agent/modules/tools/service"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"gorm.io/gorm"
)

// Request 表示一次推理调用的完整业务上下文。
type Request struct {
	AgentID              string
	OwnerID              string
	UserID               string
	Input                string
	ConversationID       string
	TraceID              string
	EnableHistoryContext bool
	Metadata             map[string]any
	Direct               bool
	ModelID              string
	MaxTokens            int
	Temperature          *float64
	Messages             []llmdto.Message
}

// Event 表示推理流中的一个可序列化事件。
type Event struct {
	Event           string         `json:"event"`
	TraceID         string         `json:"trace_id"`
	ConversationID  string         `json:"conversation_id,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	RoundNumber     int            `json:"round_number,omitempty"`
	ConfidenceScore float64        `json:"confidence_score,omitempty"`
	Payload         map[string]any `json:"payload,omitempty"`
}

// Result 表示推理完成后的业务结果。
type Result struct {
	ConversationID string
	Answer         string
	Path           string
	Status         string
	TraceID        string
	ErrorCode      string
	Confidence     float64
	InputTokens    int
	OutputTokens   int
}

// Error 表示可映射到 HTTP 契约的推理业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回推理业务错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }

// EventSink 接收按顺序生成的推理事件。
type EventSink func(Event) error

// Service 聚合推理所需的真实数据库、模型、知识、工具和计费依赖。
type Service struct {
	db        *gorm.DB
	llm       *llmservice.CallService
	knowledge *knowledgeservice.Service
	tools     *toolsservice.Service
	billing   *billingservice.Service
}

// New 创建渐进式推理服务。
func New(db *gorm.DB, llm *llmservice.CallService, knowledge *knowledgeservice.Service, tools *toolsservice.Service, billing *billingservice.Service) *Service {
	return &Service{db: db, llm: llm, knowledge: knowledge, tools: tools, billing: billing}
}

// Run 执行一次完整推理并持久化用户消息、助手消息和调用统计。
//
// 简要描述：主流程严格按“校验与预检 -> 会话续用 -> 上下文组装 -> 路径决策 ->
// 能力执行 -> 回复落库 -> 计费与摘要”的顺序推进，任何能力失败都会形成明确错误或降级结果。
func (service *Service) Run(ctx context.Context, request Request) (Result, error) {
	return service.run(ctx, request, nil)
}

// run 执行同步与流式共用的唯一事实链；sink 非空时模型增量会实时向上传递。
func (service *Service) run(ctx context.Context, request Request, sink EventSink) (Result, error) {
	if strings.TrimSpace(request.Input) == "" || request.AgentID == "" || request.UserID == "" {
		return Result{}, businessError(400, "400_INVALID_REQUEST", "agentId、userId 和 message 不能为空")
	}
	if request.TraceID == "" {
		request.TraceID = "trace-" + uuid.NewString()
	}
	if request.Metadata == nil {
		request.Metadata = map[string]any{}
	}
	agent, err := service.loadAgent(ctx, request)
	if err != nil {
		return Result{}, err
	}
	request.OwnerID = agent.OwnerID
	if request.ModelID == "" && agent.LLMModel != nil {
		request.ModelID = strings.TrimSpace(*agent.LLMModel)
	}
	if request.ModelID == "" {
		return Result{}, businessError(400, "400_DEFAULT_REASONING_MODEL_NOT_SET", "Agent 未配置可用推理模型")
	}
	if service.billing != nil {
		estimated := service.billing.EstimateCreditsForModel(ctx, request.ModelID, 1000)
		if !service.billing.PreCheck(ctx, request.OwnerID, estimated) {
			return Result{Path: "direct", Status: "failed", TraceID: request.TraceID, ErrorCode: "403_INSUFFICIENT_CREDITS", Answer: "余额不足，请充值后继续。"}, nil
		}
	}
	conversation, err := service.resolveConversation(ctx, request)
	if err != nil {
		return Result{}, err
	}
	request.ConversationID = conversation.ID
	userMessage := model.Message{ID: uuid.NewString(), ConversationID: conversation.ID, UserID: &request.UserID, Role: "user", Content: request.Input, Source: messageSource(request.Metadata), SelfTokenCount: estimateTokens(request.Input)}
	if value := metadataString(request.Metadata, "message_id"); value != "" {
		userMessage.MessageID = &value
	}
	if err := service.db.WithContext(ctx).Create(&userMessage).Error; err != nil {
		return Result{}, err
	}
	contextMessages, err := service.assembleContext(ctx, *agent, *conversation, request.EnableHistoryContext)
	if err != nil {
		return Result{}, err
	}
	knowledgeIDs, err := service.boundKnowledgeIDs(ctx, agent.ID)
	if err != nil {
		return Result{}, err
	}
	hasTools, err := service.hasTools(ctx, agent.ID)
	if err != nil {
		return Result{}, err
	}
	retrieval := []knowledgedto.SearchResult{}
	if len(knowledgeIDs) > 0 && service.knowledge != nil {
		retrieval, err = service.knowledge.SearchForReasoning(ctx, agent.OwnerID, knowledgeIDs, request.Input, 5, map[string]any{"agent_id": agent.ID, "conversation_id": conversation.ID, "trace_id": request.TraceID})
		if err != nil {
			logs.WithContext(ctx).WithField("module", "agent.reasoning").WithField("trace_id", request.TraceID).WithField("agent_id", agent.ID).WithField("conversation_id", conversation.ID).WithField("knowledge_count", len(knowledgeIDs)).Errorf("Agent 知识检索失败 error:%v", err)
			return Result{}, fmt.Errorf("知识检索失败: %w", err)
		}
	}
	path := choosePath(request.Direct, hasTools, retrieval)
	result := Result{ConversationID: conversation.ID, Path: path, Status: "completed", TraceID: request.TraceID, Confidence: confidence(retrieval, true, 1)}
	if path == "react" {
		result, err = service.runReAct(ctx, request, *agent, contextMessages, retrieval, result, sink)
	} else {
		result, err = service.runGeneration(ctx, request, *agent, contextMessages, retrieval, result, sink)
	}
	if err != nil {
		_ = service.bumpAgentStats(context.WithoutCancel(ctx), agent.ID, false)
		return Result{}, err
	}
	if strings.TrimSpace(result.Answer) == "" {
		return Result{}, businessError(502, "500_MODEL_EMPTY_RESPONSE", "模型未返回内容，请稍后重试")
	}
	assistant := model.Message{ID: uuid.NewString(), ConversationID: conversation.ID, Role: "assistant", Content: result.Answer, Source: "auto", SelfTokenCount: max(result.OutputTokens, estimateTokens(result.Answer))}
	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&assistant).Error; err != nil {
			return err
		}
		return tx.Model(&model.Conversation{}).Where("id = ?", conversation.ID).Updates(map[string]any{"updated_at": gorm.Expr("now()")}).Error
	}); err != nil {
		return Result{}, err
	}
	if service.billing != nil && result.InputTokens+result.OutputTokens > 0 {
		providerID := ""
		if agent.LLMProviderID != nil {
			providerID = *agent.LLMProviderID
		}
		if _, err := service.billing.ChargeCompletedCall(ctx, agent.OwnerID, agent.ID, providerID, request.ModelID, conversation.ID, result.InputTokens, result.OutputTokens); err != nil {
			return Result{}, err
		}
	}
	_ = service.bumpAgentStats(context.WithoutCancel(ctx), agent.ID, true)
	_ = service.maybeSummarize(context.WithoutCancel(ctx), *agent, *conversation)
	return result, nil
}

// Stream 执行推理并依次发送状态、Token 和完成事件。
//
// 简要描述：所有路径复用同一 run 业务事实链，模型上游产生的增量会立即转发；只有
// Policy 降级等没有模型流的路径才从最终正文补发，并保证 done 事件唯一且位于最后。
func (service *Service) Stream(ctx context.Context, request Request, sink EventSink) (Result, error) {
	if sink == nil {
		return Result{}, businessError(500, "500_STREAM_SINK_REQUIRED", "流式事件接收器不能为空")
	}
	if request.TraceID == "" {
		request.TraceID = "trace-" + uuid.NewString()
	}
	if err := sink(Event{Event: "started", TraceID: request.TraceID}); err != nil {
		return Result{}, err
	}
	emitted := 0
	streamSink := func(event Event) error {
		if event.Event == "token" {
			emitted++
		}
		return sink(event)
	}
	result, err := service.run(ctx, request, streamSink)
	if err != nil {
		_ = sink(Event{Event: "error", TraceID: request.TraceID, Payload: map[string]any{"code": errorCode(err), "message": err.Error()}})
		return Result{}, err
	}
	if emitted == 0 {
		for _, token := range splitTokens(result.Answer) {
			if err := sink(Event{Event: "token", TraceID: result.TraceID, ConversationID: result.ConversationID, Payload: map[string]any{"token": token}}); err != nil {
				return Result{}, err
			}
		}
	}
	if err := sink(Event{Event: "done", TraceID: result.TraceID, ConversationID: result.ConversationID, ConfidenceScore: result.Confidence, Payload: map[string]any{"answer": result.Answer, "path": result.Path, "status": result.Status, "error_code": nil}}); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (service *Service) loadAgent(ctx context.Context, request Request) (*agentmodel.Agent, error) {
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).Where("id = ?", request.AgentID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, businessError(404, "404_NOT_FOUND", "Agent 不存在")
		}
		return nil, err
	}
	// TIPS: 对话入口不再做 owner 归属校验 —— `/agents/active` 本就按 app_key 返回同应用下
	// 所有 Owner 的 active Agent（含系统内置 Agent），这里再按 owner_id 严格比对会让「列表里
	// 能选、点进去必 403」。可用性口径统一由 status 判断，多租户隔离仍由列表侧的 app_key 保证。
	if agent.Status == "archived" {
		return nil, businessError(403, "403_AGENT_ARCHIVED", "Agent 已归档")
	}
	if agent.Status == "error" {
		return nil, businessError(400, "400_RECOVERY_CHECK_FAILED", "Agent 处于错误状态")
	}
	return &agent, nil
}

func (service *Service) resolveConversation(ctx context.Context, request Request) (*model.Conversation, error) {
	var conversation model.Conversation
	query := service.db.WithContext(ctx)
	if request.ConversationID != "" {
		err := query.First(&conversation, "id = ?", request.ConversationID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, businessError(404, "404_CONVERSATION_NOT_FOUND", "会话不存在或已失效，请重建会话")
		}
		if err != nil {
			return nil, err
		}
		if conversation.AgentID != request.AgentID || conversation.UserID != request.UserID {
			return nil, businessError(409, "409_CONVERSATION_CONTEXT_MISMATCH", "会话上下文不一致")
		}
		return &conversation, nil
	}
	err := query.Where("agent_id = ? AND user_id = ? AND status = 'active'", request.AgentID, request.UserID).Order("updated_at DESC").First(&conversation).Error
	if err == nil {
		return &conversation, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	conversation = model.Conversation{ID: uuid.NewString(), AgentID: request.AgentID, UserID: request.UserID, Status: "active"}
	if value := metadataString(request.Metadata, "invite_code"); value != "" {
		conversation.InviteCode = &value
	}
	if value := metadataString(request.Metadata, "user_name"); value != "" {
		conversation.UserName = &value
	}
	if value := metadataString(request.Metadata, "pic"); value != "" {
		conversation.Pic = &value
	}
	if err := query.Create(&conversation).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (service *Service) assembleContext(ctx context.Context, agent agentmodel.Agent, conversation model.Conversation, enabled bool) ([]llmdto.Message, error) {
	messages := make([]llmdto.Message, 0, 24)
	if agent.Prompt != nil && strings.TrimSpace(*agent.Prompt) != "" {
		messages = append(messages, llmdto.Message{Role: "system", Content: *agent.Prompt})
	}
	if !enabled {
		return messages, nil
	}
	budget := 12000
	if conversation.ContextTokenBudget != nil && *conversation.ContextTokenBudget > 0 {
		budget = *conversation.ContextTokenBudget
	}
	if value := mapInt(agent.MemoryConfig, "shortTermWindowSize", 5); value > 0 {
		var history []model.Message
		if err := service.db.WithContext(ctx).Where("conversation_id = ?", conversation.ID).Order("created_at DESC").Limit(value*2 + 1).Find(&history).Error; err != nil {
			return nil, err
		}
		for left, right := 0, len(history)-1; left < right; left, right = left+1, right-1 {
			history[left], history[right] = history[right], history[left]
		}
		used := 0
		for _, item := range history {
			if item.Role == "user" && item.ID == history[len(history)-1].ID {
				continue
			}
			cost := max(item.SelfTokenCount, estimateTokens(item.Content))
			if used+cost > budget {
				continue
			}
			messages = append(messages, llmdto.Message{Role: item.Role, Content: item.Content})
			used += cost
		}
	}
	if mapBool(agent.MemoryConfig, "summaryEnabled", false) {
		var summary model.ConversationSummary
		if service.db.WithContext(ctx).Where("conversation_id = ? AND status = 'completed'", conversation.ID).Order("created_at DESC").First(&summary).Error == nil {
			messages = append(messages, llmdto.Message{Role: "system", Content: "历史会话摘要：" + summary.SummaryText})
		}
	}
	if mapBool(agent.MemoryConfig, "longTermEnabled", false) {
		limit := mapInt(agent.MemoryConfig, "longTermInjectTopK", 3)
		var memories []agentmodel.LongTermMemory
		if err := service.db.WithContext(ctx).Where("agent_id = ? AND user_id = ? AND status = 'active' AND (expires_at IS NULL OR expires_at > now())", agent.ID, conversation.UserID).Order("CASE importance WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, updated_at DESC").Limit(limit).Find(&memories).Error; err != nil {
			return nil, err
		}
		for _, memory := range memories {
			messages = append(messages, llmdto.Message{Role: "system", Content: "长期记忆：" + memory.Content})
		}
	}
	return messages, nil
}

func (service *Service) runGeneration(ctx context.Context, request Request, agent agentmodel.Agent, contextMessages []llmdto.Message, retrieval []knowledgedto.SearchResult, result Result, sink EventSink) (Result, error) {
	messages := append([]llmdto.Message{}, contextMessages...)
	if request.Direct && len(request.Messages) > 0 {
		messages = append(messages, request.Messages...)
	}
	if len(retrieval) > 0 && (result.Path == "two_step" || result.Path == "hybrid") {
		parts := make([]string, 0, len(retrieval))
		for index, item := range retrieval {
			parts = append(parts, fmt.Sprintf("[%d] %s", index+1, item.Snippet))
		}
		messages = append(messages, llmdto.Message{Role: "system", Content: "请优先依据以下知识回答；知识不足时明确说明，不要编造：\n" + strings.Join(parts, "\n")})
	}
	if !(request.Direct && len(request.Messages) > 0) {
		messages = append(messages, llmdto.Message{Role: "user", Content: request.Input})
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	content, usage, err := service.generate(ctx, request, agent, messages, maxTokens, sink)
	if err != nil {
		return Result{}, err
	}
	result.Answer = content
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens
	return result, nil
}

// generate 根据 sink 选择同步或真实上游流式模型调用，并统一返回聚合文本与用量。
func (service *Service) generate(ctx context.Context, request Request, agent agentmodel.Agent, messages []llmdto.Message, maxTokens int, sink EventSink) (string, llmdto.Usage, error) {
	payload := llmdto.CallRequest{ModelID: request.ModelID, Messages: messages, Temperature: request.Temperature, MaxTokens: &maxTokens, Metadata: map[string]any{"call_type": "reasoning", "agent_id": agent.ID, "conversation_id": request.ConversationID, "trace_id": request.TraceID}}
	if sink == nil {
		response, err := service.llm.Call(ctx, payload)
		return response.Content, response.Usage, err
	}
	// 简要描述：为本次上游流式调用创建独立取消域；下游断开或写入失败时，
	// 立即取消模型请求，避免生产者继续生成并阻塞在无人消费的通道上。
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	chunks, err := service.llm.Stream(streamCtx, payload)
	if err != nil {
		return "", llmdto.Usage{}, err
	}
	var content strings.Builder
	usage := llmdto.Usage{}
	for chunk := range chunks {
		if chunk.Err != nil {
			return "", usage, chunk.Err
		}
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
		if chunk.Text == "" {
			continue
		}
		content.WriteString(chunk.Text)
		if err := sink(Event{Event: "token", TraceID: request.TraceID, ConversationID: request.ConversationID, Payload: map[string]any{"token": chunk.Text}}); err != nil {
			return "", usage, err
		}
	}
	if usage.InputTokens == 0 {
		for _, message := range messages {
			usage.InputTokens += estimateTokens(message.Content)
		}
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = estimateTokens(content.String())
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return content.String(), usage, nil
}

func (service *Service) boundKnowledgeIDs(ctx context.Context, agentID string) ([]string, error) {
	var ids []string
	err := service.db.WithContext(ctx).Table("agent_knowledge").Where("agent_id = ?", agentID).Order("mounted_at").Pluck("knowledge_id", &ids).Error
	return ids, err
}

func (service *Service) hasTools(ctx context.Context, agentID string) (bool, error) {
	var count int64
	err := service.db.WithContext(ctx).Table("agent_tools at").Joins("JOIN tool_definitions td ON td.id=at.tool_id").Where("at.agent_id = ? AND at.enabled=true AND td.status='active' AND td.deleted_at IS NULL", agentID).Count(&count).Error
	return count > 0, err
}

func (service *Service) maybeSummarize(ctx context.Context, agent agentmodel.Agent, conversation model.Conversation) error {
	if !mapBool(agent.MemoryConfig, "summaryEnabled", false) {
		return nil
	}
	trigger := mapInt(agent.MemoryConfig, "summaryTriggerTokens", 2000)
	var total int
	if err := service.db.WithContext(ctx).Model(&model.Message{}).Where("conversation_id = ? AND is_summarized=false", conversation.ID).Select("COALESCE(SUM(self_token_count),0)").Scan(&total).Error; err != nil || total < trigger {
		return err
	}
	var messages []model.Message
	if err := service.db.WithContext(ctx).Where("conversation_id = ? AND is_summarized=false", conversation.ID).Order("created_at").Find(&messages).Error; err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	content := make([]string, 0, len(messages))
	ids := make([]string, 0, len(messages))
	for _, item := range messages {
		content = append(content, item.Role+": "+item.Content)
		ids = append(ids, item.ID)
	}
	modelID := requestModel(agent.MemoryConfig, agent.LLMModel)
	maxTokens := mapInt(agent.MemoryConfig, "summaryMaxTokens", 500)
	response, err := service.llm.Call(ctx, llmdto.CallRequest{ModelID: modelID, MaxTokens: &maxTokens, Messages: []llmdto.Message{{Role: "system", Content: "请压缩以下会话，保留用户事实、偏好、结论和未完成事项。"}, {Role: "user", Content: strings.Join(content, "\n")}}, Metadata: map[string]any{"call_type": "summary", "agent_id": agent.ID, "conversation_id": conversation.ID}})
	if err != nil {
		return err
	}
	summary := model.ConversationSummary{ID: uuid.NewString(), ConversationID: conversation.ID, AgentID: agent.ID, UserID: conversation.UserID, SummaryText: response.Content, CoveredMessageIDs: ids, CoveredMessageCount: len(ids), TokenCount: response.Usage.TotalTokens, Status: "completed"}
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&summary).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Message{}).Where("id IN ?", ids).Update("is_summarized", true).Error; err != nil {
			return err
		}
		return tx.Model(&model.Conversation{}).Where("id = ?", conversation.ID).Update("summary_id", summary.ID).Error
	})
}

func (service *Service) bumpAgentStats(ctx context.Context, agentID string, success bool) error {
	updates := map[string]any{"total_invocations": gorm.Expr("total_invocations + 1")}
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["fail_count"] = gorm.Expr("fail_count + 1")
	}
	updates["failure_rate"] = gorm.Expr("CASE WHEN total_invocations + 1 = 0 THEN 0 ELSE (fail_count + ?)::float / (total_invocations + 1) END", map[bool]int{true: 0, false: 1}[success])
	return service.db.WithContext(ctx).Model(&agentmodel.Agent{}).Where("id = ?", agentID).Updates(updates).Error
}

func choosePath(direct, hasTools bool, results []knowledgedto.SearchResult) string {
	if direct {
		return "direct_llm"
	}
	if hasTools {
		return "react"
	}
	if len(results) == 0 {
		return "direct"
	}
	if results[0].Score >= 0.55 {
		return "two_step"
	}
	if results[0].Score >= 0.15 {
		return "hybrid"
	}
	return "direct"
}

func confidence(results []knowledgedto.SearchResult, toolSuccess bool, round int) float64 {
	retrieval := 0.35
	if len(results) > 0 {
		retrieval = math.Max(0, math.Min(1, results[0].Score))
	}
	tool := 0.3
	if toolSuccess {
		tool = 1
	}
	value := retrieval*0.55 + tool*0.35 + math.Min(1, float64(round)/3)*0.1
	return math.Round(value*10000) / 10000
}

func businessError(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}
func estimateTokens(value string) int { return max(1, utf8.RuneCountInString(value)/4) }
func metadataString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}
func messageSource(values map[string]any) string {
	if source := metadataString(values, "source"); source == "human" || source == "webhook" {
		return source
	}
	return "auto"
}
func mapInt(values map[string]any, key string, fallback int) int {
	switch value := values[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	default:
		return fallback
	}
}
func mapBool(values map[string]any, key string, fallback bool) bool {
	value, ok := values[key].(bool)
	if ok {
		return value
	}
	return fallback
}
func requestModel(values map[string]any, fallback *string) string {
	if value := metadataString(values, "summaryModel"); value != "" {
		return value
	}
	if fallback != nil {
		return *fallback
	}
	return ""
}
func splitTokens(value string) []string {
	tokens := make([]string, 0, utf8.RuneCountInString(value))
	for _, token := range []rune(value) {
		tokens = append(tokens, string(token))
	}
	return tokens
}
func errorCode(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return "500_REASONING_ERROR"
}
func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
func marshal(value any) string { raw, _ := json.Marshal(value); return string(raw) }
func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
