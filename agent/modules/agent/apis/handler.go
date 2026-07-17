// Package apis 提供 Agent Profile、生命周期、能力挂载和长期记忆的 Gin 接口。
package apis

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	agentservice "github.com/juggleim/jugglemate-server/agent/modules/agent/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 暴露 Agent 非 Bot 相关的 17 个源兼容接口。
type Handler struct{ service *agentservice.Service }

// NewHandler 创建 Agent HTTP Handler。
func NewHandler(service *agentservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册 Agent Profile、生命周期、能力挂载和长期记忆接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	routes := group.Group("/agents")
	routes.POST("", handler.createWithBot)
	routes.POST("/create", handler.createWithBot)
	routes.POST("/with-bot", handler.createWithBot)
	routes.POST("/bind-bot", handler.bindBotLegacy)
	routes.GET("", handler.list)
	routes.POST("/update", handler.update)
	routes.POST("/delete", handler.delete)
	routes.POST("/:agent_id/activate", handler.activate)
	routes.POST("/:agent_id/pause", handler.pause)
	routes.POST("/:agent_id/archive", handler.archive)
	routes.POST("/:agent_id/bots/bind", handler.bindBot)
	routes.POST("/:agent_id/skills", handler.mountSkill)
	routes.DELETE("/:agent_id/skills/:skill_id", handler.unmountSkill)
	routes.POST("/:agent_id/tools", handler.mountTool)
	routes.DELETE("/:agent_id/tools/:tool_id", handler.unmountTool)
	routes.POST("/:agent_id/knowledge", handler.mountKnowledge)
	routes.DELETE("/:agent_id/knowledge/:knowledge_id", handler.unmountKnowledge)
	routes.GET("/:agent_id/users/:user_id/long-term-memory", handler.listLongTermMemory)
	routes.DELETE("/:agent_id/users/:user_id/long-term-memory/:memory_id", handler.deleteLongTermMemory)
	routes.GET("/:agent_id", handler.get)
}

// create 创建不绑定 Bot 的 Agent。
func (handler *Handler) create(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.CreateRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if err := validateCreateRequest(request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.CreateAgent(ctx.Request.Context(), actor, request)
	respond(ctx, result, err)
}

// createWithBot 创建 Agent、注册 Bot 并建立绑定。
func (handler *Handler) createWithBot(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.CreateWithBotRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if request.BotName == nil {
		value := strings.TrimSpace(request.Name) + " Bot"
		request.BotName = &value
	}
	if strings.TrimSpace(*request.BotName) == "" || len([]rune(*request.BotName)) > 128 || strings.TrimSpace(request.Name) == "" || len([]rune(request.Name)) > 128 || strings.TrimSpace(request.Type) == "" || len([]rune(request.Type)) > 32 || strings.TrimSpace(request.Prompt) == "" || len([]rune(request.Prompt)) > 2000 {
		validation(ctx, errors.New("with-bot 请求参数非法"))
		return
	}
	if err := validateCapabilityIDs(request.SkillIDs, request.ToolIDs, request.KnowledgeIDs); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.CreateAgentWithBot(ctx.Request.Context(), actor, request)
	respond(ctx, result, err)
}

// bindBot 将已有 Bot 绑定到路径指定 Agent。
func (handler *Handler) bindBot(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.BindBotRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if err := validateBotBinding(request.BotUserID, request.Token, request.BotName); err != nil {
		validation(ctx, err)
		return
	}
	createBot := true
	if request.CreateBot != nil {
		createBot = *request.CreateBot
	}
	result, err := handler.service.BindExistingBot(ctx.Request.Context(), actor, ctx.Param("agent_id"), request.BotUserID, request.Token, request.BotName, createBot)
	respond(ctx, result, err)
}

// bindBotLegacy 处理旧脚本使用的 Bot 绑定入口。
func (handler *Handler) bindBotLegacy(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.BindBotLegacyRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if strings.TrimSpace(request.AgentID) == "" || len(request.AgentID) > 64 {
		validation(ctx, errors.New("agent_id 参数非法"))
		return
	}
	if err := validateBotBinding(request.BotUserID, request.Token, request.Name); err != nil {
		validation(ctx, err)
		return
	}
	if strings.TrimSpace(ctx.GetHeader("X-Invite-Code")) == "" && strings.TrimSpace(request.InviteCode) != "" {
		actor.OwnerID = strings.TrimSpace(request.InviteCode)
	}
	createBot := true
	if request.CreateBot != nil {
		createBot = *request.CreateBot
	}
	result, err := handler.service.BindExistingBot(ctx.Request.Context(), actor, request.AgentID, request.BotUserID, request.Token, request.Name, createBot)
	respond(ctx, result, err)
}

// list 分页查询当前 Owner 的 Agent。
func (handler *Handler) list(ctx *gin.Context) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		unauthorized(ctx)
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page < 1 || size < 1 || size > 100 {
		validation(ctx, errors.New("page 或 pageSize 超出范围"))
		return
	}
	result, err := handler.service.ListAgents(ctx.Request.Context(), principal.AppKey, principal.ID, page, size)
	respond(ctx, result, err)
}

// get 获取 Agent 详情。
func (handler *Handler) get(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	result, err := handler.service.GetAgent(ctx.Request.Context(), actor, ctx.Param("agent_id"))
	respond(ctx, result, err)
}

// update 更新 Agent 配置和目标能力集合。
func (handler *Handler) update(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.UpdateRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if err := validateUpdateRequest(request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UpdateAgent(ctx.Request.Context(), actor, request)
	respond(ctx, result, err)
}

// delete 删除 draft Agent。
func (handler *Handler) delete(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.DeleteRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if strings.TrimSpace(request.AgentID) == "" || len(request.AgentID) > 64 {
		validation(ctx, errors.New("agentId 参数非法"))
		return
	}
	err := handler.service.DeleteAgent(ctx.Request.Context(), actor, request.AgentID)
	respond(ctx, gin.H{"status": "deleted", "agentId": request.AgentID}, err)
}

// activate 启动 Agent。
func (handler *Handler) activate(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	result, err := handler.service.ActivateAgent(ctx.Request.Context(), actor, ctx.Param("agent_id"))
	respond(ctx, result, err)
}

// pause 暂停 Agent。
func (handler *Handler) pause(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	result, err := handler.service.PauseAgent(ctx.Request.Context(), actor, ctx.Param("agent_id"))
	respond(ctx, result, err)
}

// archive 归档 Agent，并在有运行会话时返回 202 语义状态。
func (handler *Handler) archive(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	var request dto.ArchiveRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return
	}
	if strings.TrimSpace(request.ConfirmName) == "" || len([]rune(request.ConfirmName)) > 128 {
		validation(ctx, errors.New("confirmName 参数非法"))
		return
	}
	result, err := handler.service.ArchiveAgent(ctx.Request.Context(), actor, ctx.Param("agent_id"), request.ConfirmName)
	if err != nil {
		writeError(ctx, err)
		return
	}
	if result.ArchiveInProgress {
		httpresponse.Accepted(ctx, result)
		return
	}
	httpresponse.Success(ctx, result)
}

// mountSkill 挂载 Skill。
func (handler *Handler) mountSkill(ctx *gin.Context) {
	actor, request, ok := mountRequest(ctx)
	if !ok {
		return
	}
	result, err := handler.service.MountSkill(ctx.Request.Context(), actor, ctx.Param("agent_id"), request.CapabilityID)
	respond(ctx, result, err)
}

// unmountSkill 卸载 Skill。
func (handler *Handler) unmountSkill(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	id := ctx.Param("skill_id")
	err := handler.service.UnmountSkill(ctx.Request.Context(), actor, ctx.Param("agent_id"), id)
	respond(ctx, gin.H{"status": "unmounted", "capabilityId": id}, err)
}

// mountTool 挂载 Tool。
func (handler *Handler) mountTool(ctx *gin.Context) {
	actor, request, ok := mountRequest(ctx)
	if !ok {
		return
	}
	result, err := handler.service.MountTool(ctx.Request.Context(), actor, ctx.Param("agent_id"), request.CapabilityID)
	respond(ctx, result, err)
}

// unmountTool 卸载 Tool。
func (handler *Handler) unmountTool(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	id := ctx.Param("tool_id")
	err := handler.service.UnmountTool(ctx.Request.Context(), actor, ctx.Param("agent_id"), id)
	respond(ctx, gin.H{"status": "unmounted", "capabilityId": id}, err)
}

// mountKnowledge 挂载 ready 知识库。
func (handler *Handler) mountKnowledge(ctx *gin.Context) {
	actor, request, ok := mountRequest(ctx)
	if !ok {
		return
	}
	result, err := handler.service.MountKnowledge(ctx.Request.Context(), actor, ctx.Param("agent_id"), request.CapabilityID)
	respond(ctx, result, err)
}

// unmountKnowledge 卸载知识库。
func (handler *Handler) unmountKnowledge(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	id := ctx.Param("knowledge_id")
	err := handler.service.UnmountKnowledge(ctx.Request.Context(), actor, ctx.Param("agent_id"), id)
	respond(ctx, gin.H{"status": "unmounted", "capabilityId": id}, err)
}

// listLongTermMemory 查询用户长期记忆。
func (handler *Handler) listLongTermMemory(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	topK, err := strconv.Atoi(ctx.DefaultQuery("topK", "50"))
	if err != nil || topK < 1 || topK > 1000 {
		validation(ctx, errors.New("topK 超出范围"))
		return
	}
	items, err := handler.service.ListLongTermMemories(ctx.Request.Context(), actor, ctx.Param("agent_id"), ctx.Param("user_id"), topK)
	respond(ctx, gin.H{"items": items}, err)
}

// deleteLongTermMemory 删除用户长期记忆。
func (handler *Handler) deleteLongTermMemory(ctx *gin.Context) {
	actor, ok := requestActor(ctx)
	if !ok {
		return
	}
	id := ctx.Param("memory_id")
	err := handler.service.DeleteLongTermMemory(ctx.Request.Context(), actor, ctx.Param("agent_id"), ctx.Param("user_id"), id)
	respond(ctx, gin.H{"status": "deleted", "memoryId": id}, err)
}

func mountRequest(ctx *gin.Context) (agentservice.Actor, dto.CapabilityMountRequest, bool) {
	actor, ok := requestActor(ctx)
	if !ok {
		return agentservice.Actor{}, dto.CapabilityMountRequest{}, false
	}
	var request dto.CapabilityMountRequest
	if err := bindStrict(ctx, &request); err != nil {
		validation(ctx, err)
		return agentservice.Actor{}, dto.CapabilityMountRequest{}, false
	}
	if strings.TrimSpace(request.CapabilityID) == "" || len(request.CapabilityID) > 64 {
		validation(ctx, errors.New("capabilityId 参数非法"))
		return agentservice.Actor{}, dto.CapabilityMountRequest{}, false
	}
	return actor, request, true
}

func requestActor(ctx *gin.Context) (agentservice.Actor, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		unauthorized(ctx)
		return agentservice.Actor{}, false
	}
	return agentservice.Actor{OwnerID: principal.ID, AppKey: principal.AppKey, IsAdmin: principal.Role == identity.RoleAdmin}, true
}

func bindStrict(ctx *gin.Context, target any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求体只能包含一个 JSON 对象")
	}
	return nil
}

func validateCreateRequest(request dto.CreateRequest) error {
	if err := validateOptionalText(request.Name, "name", 128, false); err != nil {
		return err
	}
	if err := validateOptionalText(request.Type, "type", 32, false); err != nil {
		return err
	}
	if err := validateOptionalText(request.Prompt, "prompt", 2000, false); err != nil {
		return err
	}
	if err := validateOptionalText(request.LLMModel, "llmModel", 100, false); err != nil {
		return err
	}
	if err := validateOptionalText(request.LLMProviderID, "llmProviderId", 64, false); err != nil {
		return err
	}
	return validateCapabilityIDs(request.SkillIDs, request.ToolIDs, request.KnowledgeIDs)
}

func validateUpdateRequest(request dto.UpdateRequest) error {
	if strings.TrimSpace(request.AgentID) == "" || len(request.AgentID) > 64 {
		return errors.New("agentId 参数非法")
	}
	for _, item := range []struct {
		value *string
		name  string
		max   int
	}{{request.Name, "name", 128}, {request.Type, "type", 32}, {request.Prompt, "prompt", 2000}, {request.LLMModel, "llmModel", 100}, {request.LLMProviderID, "llmProviderId", 64}} {
		if err := validateOptionalText(item.value, item.name, item.max, false); err != nil {
			return err
		}
	}
	if request.ReactConfig != nil {
		patch := request.ReactConfig
		if (patch.MaxRounds != nil && (*patch.MaxRounds < 3 || *patch.MaxRounds > 10)) || (patch.TargetRounds != nil && (*patch.TargetRounds < 2 || *patch.TargetRounds > 8)) || (patch.ToolsPerRound != nil && (*patch.ToolsPerRound < 1 || *patch.ToolsPerRound > 5)) || (patch.ConfidenceThreshold != nil && (*patch.ConfidenceThreshold < 0.5 || *patch.ConfidenceThreshold > 1)) {
			return errors.New("reactConfig 参数非法")
		}
	}
	if request.MemoryConfig != nil {
		patch := request.MemoryConfig
		if (patch.ShortTermWindowSize != nil && (*patch.ShortTermWindowSize < 1 || *patch.ShortTermWindowSize > 20)) || (patch.SummaryTriggerTokens != nil && *patch.SummaryTriggerTokens < 100) || (patch.SummaryMaxTokens != nil && *patch.SummaryMaxTokens < 50) || (patch.LongTermInjectTopK != nil && (*patch.LongTermInjectTopK < 1 || *patch.LongTermInjectTopK > 10)) || (patch.LongTermExpireDays != nil && *patch.LongTermExpireDays < 1) {
			return errors.New("memoryConfig 参数非法")
		}
	}
	if request.SkillIDs != nil || request.ToolIDs != nil || request.KnowledgeIDs != nil {
		var skills, tools, knowledge []string
		if request.SkillIDs != nil {
			skills = *request.SkillIDs
		}
		if request.ToolIDs != nil {
			tools = *request.ToolIDs
		}
		if request.KnowledgeIDs != nil {
			knowledge = *request.KnowledgeIDs
		}
		return validateCapabilityIDs(skills, tools, knowledge)
	}
	return nil
}

func validateOptionalText(value *string, field string, max int, allowEmpty bool) error {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if (!allowEmpty && trimmed == "") || len([]rune(trimmed)) > max {
		return errors.New(field + " 参数非法")
	}
	return nil
}

func validateCapabilityIDs(groups ...[]string) error {
	for _, group := range groups {
		for _, id := range group {
			if strings.TrimSpace(id) == "" || len(id) > 64 {
				return errors.New("能力 ID 参数非法")
			}
		}
	}
	return nil
}

func validateBotBinding(userID, token string, name *string) error {
	if strings.TrimSpace(userID) == "" || len(userID) > 64 || strings.TrimSpace(token) == "" || len(token) > 4096 {
		return errors.New("Bot user id 或 token 参数非法")
	}
	if name != nil && (strings.TrimSpace(*name) == "" || len([]rune(*name)) > 128) {
		return errors.New("Bot 名称参数非法")
	}
	return nil
}

func respond(ctx *gin.Context, data any, err error) {
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, data)
}
func writeError(ctx *gin.Context, err error) {
	var business *agentservice.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Code, business.Message)
		return
	}
	httpresponse.Failure(ctx, http.StatusInternalServerError, 500, "服务器内部错误")
}
func validation(ctx *gin.Context, err error) {
	message := "请求参数校验失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message += ": " + err.Error()
	}
	httpresponse.Failure(ctx, 422, 422, message)
}
func unauthorized(ctx *gin.Context) {
	httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未登录")
}
