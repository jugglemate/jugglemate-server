// Package apis 提供 Chat、Conversation、人工介入与 Owner 查询接口。
package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/message/dto"
	messageservice "github.com/juggleim/jugglemate-server/agent/modules/message/service"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 暴露 Message 域 14 个源兼容接口。
type Handler struct{ service *messageservice.Service }

// NewHandler 创建 Message HTTP Handler。
func NewHandler(service *messageservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册 Chat、会话、Owner 查询和人工介入路由。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	chat := group.Group("/chat")
	chat.GET("/healthz", handler.health)
	chat.POST("/completion", handler.chat)
	chat.POST("/stream", handler.stream)
	chat.POST("/app/stream", handler.appStream)
	chat.POST("/app/completion", handler.appCompletion)
	human := group.Group("/bot/human-interventions")
	human.POST("/enter", handler.enterHuman)
	human.POST("/send-message", handler.sendHuman)
	human.POST("/exit", handler.exitHuman)
	human.GET("/status", handler.humanStatus)
	human.GET("/poll-messages", handler.pollHuman)
	group.GET("/conversations/:conversation_id/summary", handler.summary)
	group.GET("/owner/agents/:agent_id/conversations", handler.listConversations)
	group.GET("/owner/conversations/:conversation_id", handler.conversation)
	group.GET("/owner/agents", handler.ownerAgents)
}

// health 返回对话模块健康状态，不访问外部模型。
func (handler *Handler) health(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) }

// chat 处理非流式标准推理对话。
func (handler *Handler) chat(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	var request dto.ChatRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.Chat(ctx.Request.Context(), principal.ID, request)
	respond(ctx, result, err)
}

// stream 以 SSE 输出标准推理事件，并以 `[DONE]` 结束。
func (handler *Handler) stream(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	var request dto.ChatRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	prepareSSE(ctx)
	ctx.Stream(func(writer io.Writer) bool {
		err := handler.service.Stream(ctx.Request.Context(), principal.ID, request, func(event reasoningservice.Event) error {
			raw, marshalErr := json.Marshal(event)
			if marshalErr != nil {
				return marshalErr
			}
			_, writeErr := fmt.Fprintf(writer, "data: %s\n\n", raw)
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			return writeErr
		})
		if err != nil {
			event := reasoningservice.Event{Event: "error", Payload: map[string]any{"code": errorCode(err), "message": err.Error()}}
			raw, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(writer, "data: %s\n\n", raw)
		}
		_, _ = fmt.Fprint(writer, "data: [DONE]\n\n")
		return false
	})
}

// appCompletion 处理应用直连非流式请求。
func (handler *Handler) appCompletion(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	userID := strings.TrimSpace(ctx.GetHeader("X-User-Id"))
	if userID == "" {
		failure(ctx, messageserviceError(400, "400_USER_ID_REQUIRED_FOR_APP_CHAT", "缺少用户标识，请在 Header 中提供 X-User-Id"))
		return
	}
	var request dto.AppChatRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.AppChat(ctx.Request.Context(), principal.ID, userID, request)
	respond(ctx, result, err)
}

// appStream 以 SSE 输出应用直连结果。
func (handler *Handler) appStream(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	userID := strings.TrimSpace(ctx.GetHeader("X-User-Id"))
	if userID == "" {
		failure(ctx, messageserviceError(400, "400_USER_ID_REQUIRED_FOR_APP_CHAT", "缺少用户标识，请在 Header 中提供 X-User-Id"))
		return
	}
	var request dto.AppChatRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	prepareSSE(ctx)
	err := handler.service.AppStream(ctx.Request.Context(), principal.ID, userID, request, func(event reasoningservice.Event) error {
		writeEvent(ctx, event)
		return nil
	})
	if err != nil {
		writeEvent(ctx, reasoningservice.Event{Event: "error", Payload: map[string]any{"code": errorCode(err), "message": err.Error()}})
	}
	ctx.Writer.WriteString("data: [DONE]\n\n")
}

// listConversations 查询 Agent 会话列表。
func (handler *Handler) listConversations(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "10"))
	from, err := parseOptionalTime(ctx.Query("date_from"))
	if err != nil {
		validation(ctx, err)
		return
	}
	to, err := parseOptionalTime(ctx.Query("date_to"))
	if err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.ListConversations(ctx.Request.Context(), principal.ID, ctx.Param("agent_id"), ctx.Query("status"), ctx.Query("user_id"), from, to, page, size)
	respond(ctx, result, err)
}

// conversation 查询会话详情。
func (handler *Handler) conversation(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	result, err := handler.service.GetConversation(ctx.Request.Context(), principal.ID, ctx.Param("conversation_id"))
	respond(ctx, result, err)
}

// ownerAgents 查询 Owner 的 Agent 与会话统计。
func (handler *Handler) ownerAgents(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	items, err := handler.service.ListOwnerAgents(ctx.Request.Context(), principal.ID)
	respond(ctx, gin.H{"items": items}, err)
}

// summary 查询会话最新摘要。
func (handler *Handler) summary(ctx *gin.Context) {
	principal, ok := handler.principal(ctx)
	if !ok {
		return
	}
	result, err := handler.service.GetSummary(ctx.Request.Context(), principal.ID, ctx.Param("conversation_id"))
	respond(ctx, result, err)
}

// enterHuman 开启人工介入。
func (handler *Handler) enterHuman(ctx *gin.Context) {
	operator, ok := operatorID(ctx)
	if !ok {
		return
	}
	var request dto.HumanEnterRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.EnterHuman(ctx.Request.Context(), operator, request)
	respond(ctx, result, err)
}

// sendHuman 在人工态发送消息。
func (handler *Handler) sendHuman(ctx *gin.Context) {
	operator, ok := operatorID(ctx)
	if !ok {
		return
	}
	var request dto.HumanSendRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.SendHumanMessage(ctx.Request.Context(), operator, request)
	respond(ctx, result, err)
}

// exitHuman 退出人工介入。
func (handler *Handler) exitHuman(ctx *gin.Context) {
	operator, ok := operatorID(ctx)
	if !ok {
		return
	}
	var request dto.HumanExitRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.ExitHuman(ctx.Request.Context(), operator, request)
	respond(ctx, result, err)
}

// humanStatus 查询人工介入状态。
func (handler *Handler) humanStatus(ctx *gin.Context) {
	result, err := handler.service.HumanStatus(ctx.Request.Context(), ctx.Query("agent_id"), ctx.Query("user_id"))
	respond(ctx, result, err)
}

// pollHuman 轮询人工态增量用户消息。
func (handler *Handler) pollHuman(ctx *gin.Context) {
	since, err := time.Parse(time.RFC3339, ctx.Query("since_ts"))
	if err != nil {
		validation(ctx, errors.New("since_ts 格式错误，请使用 ISO8601 UTC"))
		return
	}
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	result, err := handler.service.PollHumanMessages(ctx.Request.Context(), ctx.Query("agent_id"), ctx.Query("user_id"), since, limit)
	respond(ctx, result, err)
}

func (handler *Handler) principal(ctx *gin.Context) (identity.Principal, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未认证的 Agent 调用")
		return identity.Principal{}, false
	}
	return principal, true
}
func operatorID(ctx *gin.Context) (string, bool) {
	for _, header := range []string{"X-Admin-Id", "X-User-Id", "X-Invite-Code"} {
		if value := strings.TrimSpace(ctx.GetHeader(header)); value != "" {
			return value, true
		}
	}
	httpresponse.Failure(ctx, 400, "400_INVALID_REQUEST", "缺少操作人身份，请提供 X-Admin-Id / X-User-Id / X-Invite-Code")
	return "", false
}
func respond(ctx *gin.Context, result any, err error) {
	if err != nil {
		failure(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}
func failure(ctx *gin.Context, err error) {
	status, code := messageservice.StatusCode(err), messageservice.ErrorCode(err)
	httpresponse.Failure(ctx, status, code, err.Error())
}
func validation(ctx *gin.Context, err error) {
	httpresponse.Failure(ctx, 422, "422_VALIDATION_ERROR", err.Error())
}
func parseOptionalTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return &parsed, err
}
func prepareSSE(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
}
func writeEvent(ctx *gin.Context, event reasoningservice.Event) {
	raw, _ := json.Marshal(event)
	_, _ = fmt.Fprintf(ctx.Writer, "data: %s\n\n", raw)
	ctx.Writer.Flush()
}
func messageserviceError(status int, code, message string) error {
	return &messageservice.Error{Status: status, Code: code, Message: message}
}
func errorCode(err error) string { return messageservice.ErrorCode(err) }
