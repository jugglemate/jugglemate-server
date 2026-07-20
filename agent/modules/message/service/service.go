// Package service 实现对话、会话查询与应用直连业务。
package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	llmdto "github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/message/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	reasoningmodel "github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	"gorm.io/gorm"
)

// Error 表示可映射到 HTTP 契约的 Message 业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回 Message 业务错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }

// HumanHandoffFunc 把 Ticket 群从 Agent 接待切换为人工接待：拉入 Inbox 坐席、广播
// 「人工接入」通知并移出 Agent Bot。
//
// TIPS: 用函数字段注入而不是直接 import 客服域的 services 包 —— 工单群成员管理属于客服域
// 能力，Agent 平台模块不应该反向依赖它；装配在 main.go 完成。与 agentservice 的
// UnbindInboxAgentFunc 是同一套做法。
type HumanHandoffFunc func(ctx context.Context, appKey, ticketID, botUserID string) error

// Service 聚合 Message 域的真实 PostgreSQL、Reasoning、Billing 与 IM 依赖。
type Service struct {
	db           *gorm.DB
	reasoning    *reasoningservice.Service
	billing      *billingservice.Service
	connections  *imbot.Manager
	humanHandoff HumanHandoffFunc
}

// SetHumanHandoff 注入转人工的群成员切换实现。
func (service *Service) SetHumanHandoff(fn HumanHandoffFunc) {
	service.humanHandoff = fn
}

// New 创建 Message 业务服务。
func New(db *gorm.DB, reasoning *reasoningservice.Service, billing *billingservice.Service, connections *imbot.Manager) *Service {
	return &Service{db: db, reasoning: reasoning, billing: billing, connections: connections}
}

// Chat 执行标准推理对话。
func (service *Service) Chat(ctx context.Context, ownerID string, request dto.ChatRequest) (dto.ChatResponse, error) {
	enabled := true
	if request.EnableHistoryContext != nil {
		enabled = *request.EnableHistoryContext
	}
	result, err := service.reasoning.Run(ctx, reasoningservice.Request{AgentID: request.AgentID, OwnerID: ownerID, UserID: request.UserID, Input: request.Message, ConversationID: stringValue(request.ConversationID), TraceID: stringValue(request.TraceID), EnableHistoryContext: enabled, Metadata: request.Metadata})
	if err != nil {
		return dto.ChatResponse{}, err
	}
	return chatResponse(result), nil
}

// Stream 执行标准推理对话并输出事件。
func (service *Service) Stream(ctx context.Context, ownerID string, request dto.ChatRequest, sink reasoningservice.EventSink) error {
	enabled := true
	if request.EnableHistoryContext != nil {
		enabled = *request.EnableHistoryContext
	}
	_, err := service.reasoning.Stream(ctx, reasoningservice.Request{AgentID: request.AgentID, OwnerID: ownerID, UserID: request.UserID, Input: request.Message, ConversationID: stringValue(request.ConversationID), TraceID: stringValue(request.TraceID), EnableHistoryContext: enabled, Metadata: request.Metadata}, sink)
	return err
}

// AppChat 执行跳过知识与工具编排的应用直连对话，并执行用户日配额预检。
//
// 简要描述：按 Owner 与名称解析唯一 Agent，将请求 messages 原样传给模型；调用前用
// 最大输出量保守估算日配额，避免已知超额请求触发外部模型费用。
func (service *Service) AppChat(ctx context.Context, ownerID, userID string, request dto.AppChatRequest) (dto.AppChatResponse, error) {
	reasoningRequest, err := service.prepareAppRequest(ctx, ownerID, userID, request)
	if err != nil {
		return dto.AppChatResponse{}, err
	}
	result, err := service.reasoning.Run(ctx, reasoningRequest)
	if err != nil {
		return dto.AppChatResponse{}, err
	}
	return dto.AppChatResponse{Answer: result.Answer, Path: "direct_llm", Status: result.Status, TraceID: result.TraceID, ErrorCode: optional(result.ErrorCode)}, nil
}

// AppStream 执行应用直连真实流式对话，并复用非流式接口的模型解析与日配额预检。
func (service *Service) AppStream(ctx context.Context, ownerID, userID string, request dto.AppChatRequest, sink reasoningservice.EventSink) error {
	reasoningRequest, err := service.prepareAppRequest(ctx, ownerID, userID, request)
	if err != nil {
		return err
	}
	_, err = service.reasoning.Stream(ctx, reasoningRequest, sink)
	return err
}

// prepareAppRequest 校验应用直连消息、解析 Agent 并在调用外部模型前完成配额预检。
func (service *Service) prepareAppRequest(ctx context.Context, ownerID, userID string, request dto.AppChatRequest) (reasoningservice.Request, error) {
	var agent agentmodel.Agent
	query := service.db.WithContext(ctx).Where("name = ? AND status NOT IN ('archived','deleted')", strings.TrimSpace(request.AgentName))
	if ownerID != "system" {
		query = query.Where("owner_id = ?", ownerID)
	}
	if err := query.Order("updated_at DESC").First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return reasoningservice.Request{}, messageError(404, "404_AGENT_NOT_FOUND", "未找到指定名称的 Agent")
		}
		return reasoningservice.Request{}, err
	}
	if len(request.Messages) == 0 {
		return reasoningservice.Request{}, messageError(400, "400_MESSAGES_REQUIRED", "messages 不能为空")
	}
	contents := make([]string, 0, len(request.Messages))
	messages := make([]llmdto.Message, 0, len(request.Messages))
	input := ""
	for _, item := range request.Messages {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "system" && role != "user" && role != "assistant" && role != "tool" {
			return reasoningservice.Request{}, messageError(400, "400_INVALID_MESSAGE_ROLE", "messages.role 非法")
		}
		messages = append(messages, llmdto.Message{Role: role, Content: item.Content})
		contents = append(contents, item.Content)
		if role == "user" {
			input = item.Content
		}
	}
	if input == "" {
		input = request.Messages[len(request.Messages)-1].Content
	}
	maxTokens := request.MaxTokens
	if maxTokens == nil {
		value := 16384
		maxTokens = &value
	}
	estimated := service.billing.EstimateDirectTokens(contents, maxTokens)
	allowed, _, err := service.billing.PreCheckDirectQuota(ctx, agent.ID, userID, estimated, time.Now().UTC())
	if err != nil {
		return reasoningservice.Request{}, err
	}
	if !allowed {
		return reasoningservice.Request{}, messageError(403, "403_TOKEN_QUOTA_EXCEEDED", "今日 Token 额度不足，请明日再试或升级订阅")
	}
	return reasoningservice.Request{AgentID: agent.ID, OwnerID: agent.OwnerID, UserID: userID, Input: input, EnableHistoryContext: request.EnableHistoryContext, TraceID: stringValue(request.TraceID), Metadata: request.Metadata, Direct: true, ModelID: stringValue(agent.LLMModel), MaxTokens: *maxTokens, Temperature: request.Temperature, Messages: messages}, nil
}

// ListConversations 分页查询指定 Agent 最近 90 天会话。
func (service *Service) ListConversations(ctx context.Context, ownerID, agentID, status, userID string, dateFrom, dateTo *time.Time, page, pageSize int) (dto.ConversationListResponse, error) {
	if err := service.assertAgentReadable(ctx, ownerID, agentID); err != nil {
		return dto.ConversationListResponse{}, err
	}
	page, pageSize = pages(page, pageSize)
	where := "c.agent_id = ? AND c.created_at >= ?"
	args := []any{agentID, time.Now().UTC().AddDate(0, 0, -90)}
	if status != "" {
		where += " AND c.status = ?"
		args = append(args, status)
	}
	if userID != "" {
		where += " AND c.user_id = ?"
		args = append(args, userID)
	}
	if dateFrom != nil {
		where += " AND c.created_at >= ?"
		args = append(args, *dateFrom)
	}
	if dateTo != nil {
		where += " AND c.created_at <= ?"
		args = append(args, *dateTo)
	}
	var total int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM conversations c WHERE "+where, args...).Scan(&total).Error; err != nil {
		return dto.ConversationListResponse{}, err
	}
	query := `SELECT c.id,c.agent_id,c.user_id,c.user_name,c.pic,c.status,c.created_at,c.updated_at last_active_at,COALESCE(ms.message_count,0) message_count,COALESCE(ms.total_tokens,0) total_tokens FROM conversations c LEFT JOIN (SELECT conversation_id,COUNT(*) message_count,COALESCE(SUM(self_token_count),0) total_tokens FROM messages GROUP BY conversation_id) ms ON ms.conversation_id=c.id WHERE ` + where + ` ORDER BY c.updated_at DESC OFFSET ? LIMIT ?`
	listArgs := append(append([]any{}, args...), (page-1)*pageSize, pageSize)
	var items []dto.ConversationListItem
	if err := service.db.WithContext(ctx).Raw(query, listArgs...).Scan(&items).Error; err != nil {
		return dto.ConversationListResponse{}, err
	}
	return dto.ConversationListResponse{Items: items, Pagination: dto.PaginationMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: int(math.Ceil(float64(total) / float64(pageSize)))}}, nil
}

// GetConversation 查询会话详情和按时间升序排列的消息。
func (service *Service) GetConversation(ctx context.Context, ownerID, conversationID string) (dto.ConversationDetail, error) {
	var conversation reasoningmodel.Conversation
	if err := service.db.WithContext(ctx).First(&conversation, "id = ?", conversationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ConversationDetail{}, messageError(404, "404_CONVERSATION_NOT_FOUND", "会话不存在")
		}
		return dto.ConversationDetail{}, err
	}
	if err := service.assertAgentReadable(ctx, ownerID, conversation.AgentID); err != nil {
		return dto.ConversationDetail{}, err
	}
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).First(&agent, "id = ?", conversation.AgentID).Error; err != nil {
		return dto.ConversationDetail{}, err
	}
	var messages []reasoningmodel.Message
	if err := service.db.WithContext(ctx).Where("conversation_id = ?", conversation.ID).Order("created_at").Find(&messages).Error; err != nil {
		return dto.ConversationDetail{}, err
	}
	items := make([]dto.MessageItem, 0, len(messages))
	totalTokens := 0
	for _, item := range messages {
		items = append(items, dto.MessageItem{ID: item.ID, Role: item.Role, Content: item.Content, SelfTokenCount: item.SelfTokenCount, CreatedAt: item.CreatedAt})
		totalTokens += item.SelfTokenCount
	}
	last := conversation.UpdatedAt
	return dto.ConversationDetail{ID: conversation.ID, AgentID: conversation.AgentID, AgentName: agent.Name, UserID: conversation.UserID, UserName: conversation.UserName, Pic: conversation.Pic, Status: conversation.Status, CreatedAt: conversation.CreatedAt, LastActiveAt: &last, MessageCount: len(items), TotalTokens: totalTokens, Messages: items}, nil
}

// ListOwnerAgents 查询 Owner 的 Agent 及会话统计。
func (service *Service) ListOwnerAgents(ctx context.Context, ownerID string) ([]dto.OwnerAgentItem, error) {
	query := service.db.WithContext(ctx).Model(&agentmodel.Agent{}).Where("status NOT IN ('archived','deleted')")
	if ownerID != "system" {
		query = query.Where("owner_id = ? OR owner_id = 'system'", ownerID)
	}
	var agents []agentmodel.Agent
	if err := query.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, err
	}
	items := make([]dto.OwnerAgentItem, 0, len(agents))
	for _, agent := range agents {
		var stats dto.ConversationStats
		if err := service.db.WithContext(ctx).Raw(`SELECT COUNT(*) total_conversations,COUNT(*) FILTER (WHERE status='active') active_conversations,COUNT(*) FILTER (WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai') today_new FROM conversations WHERE agent_id=? AND created_at>=?`, agent.ID, time.Now().UTC().AddDate(0, 0, -90)).Scan(&stats).Error; err != nil {
			return nil, err
		}
		items = append(items, dto.OwnerAgentItem{ID: agent.ID, Name: agent.Name, Status: agent.Status, ConversationStats: stats})
	}
	return items, nil
}

// GetSummary 查询会话最新的已完成摘要。
func (service *Service) GetSummary(ctx context.Context, ownerID, conversationID string) (dto.ConversationSummary, error) {
	if _, err := service.GetConversation(ctx, ownerID, conversationID); err != nil {
		return dto.ConversationSummary{}, err
	}
	var summary reasoningmodel.ConversationSummary
	if err := service.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("created_at DESC").First(&summary).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.ConversationSummary{}, messageError(404, "404_SUMMARY_NOT_FOUND", "当前会话尚未生成摘要")
		}
		return dto.ConversationSummary{}, err
	}
	return dto.ConversationSummary{ID: summary.ID, ConversationID: summary.ConversationID, Status: summary.Status, SummaryText: summary.SummaryText, CoveredMessageCount: summary.CoveredMessageCount, TokenCount: summary.TokenCount}, nil
}

func (service *Service) assertAgentReadable(ctx context.Context, ownerID, agentID string) error {
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).First(&agent, "id = ?", agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return messageError(404, "404_AGENT_NOT_FOUND", "Agent 不存在")
		}
		return err
	}
	// TIPS: 与 reasoning.loadAgent 保持同一口径 —— 会话查询不再做 owner 归属校验，
	// 只保证 Agent 存在；否则同应用下换个账号查同一个 Agent 的会话就会 403。
	return nil
}

func chatResponse(result reasoningservice.Result) dto.ChatResponse {
	return dto.ChatResponse{ConversationID: result.ConversationID, Answer: result.Answer, Path: result.Path, Status: result.Status, TraceID: result.TraceID, ErrorCode: optional(result.ErrorCode)}
}
func messageError(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func pages(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func nowPointer(value time.Time) *time.Time { return &value }

// ErrorCode 返回 Message 或 Reasoning 错误的稳定业务码。
func ErrorCode(err error) string {
	var messageErr *Error
	if errors.As(err, &messageErr) {
		return messageErr.Code
	}
	var reasoningErr *reasoningservice.Error
	if errors.As(err, &reasoningErr) {
		return reasoningErr.Code
	}
	return "500_MESSAGE_ERROR"
}

// StatusCode 返回 Message 或 Reasoning 错误的语义 HTTP 状态。
func StatusCode(err error) int {
	var messageErr *Error
	if errors.As(err, &messageErr) {
		return messageErr.Status
	}
	var reasoningErr *reasoningservice.Error
	if errors.As(err, &reasoningErr) {
		return reasoningErr.Status
	}
	return 500
}
