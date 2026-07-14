// Package apis 提供 Operations 管理后台的 Gin HTTP 接口。
package apis

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
	operationsservice "github.com/juggleim/jugglemate-server/agent/modules/operations/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 暴露 Operations 域全部 17 个源兼容接口。
type Handler struct{ service *operationsservice.Service }

// NewHandler 创建 Operations HTTP Handler。
func NewHandler(service *operationsservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册平台统计、Owner、Agent 管控、全局配置和会话审计接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/admin/stats/platform", handler.platformStats)
	group.GET("/admin/owners", handler.listOwners)
	group.GET("/admin/owners/:owner_id", handler.getOwner)
	group.GET("/admin/agents/console-list", handler.listConsoleAgents)
	group.GET("/admin/agents", handler.listAgents)
	group.GET("/admin/agents/:agent_id", handler.getAgent)
	group.POST("/admin/agents/:agent_id/pause", handler.pauseAgent)
	group.POST("/admin/agents/:agent_id/resume", handler.resumeAgent)
	group.GET("/admin/billing/rules", handler.getBillingRule)
	group.PUT("/admin/billing/rules", handler.updateBillingRule)
	group.PUT("/admin/billing/tool-pricing", handler.updateToolPricing)
	group.GET("/admin/models/coefficients", handler.getModelCoefficients)
	group.PUT("/admin/models/coefficients", handler.updateModelCoefficients)
	group.GET("/admin/policy/rules", handler.getPolicyRule)
	group.PUT("/admin/policy/rules", handler.updatePolicyRule)
	group.GET("/admin/audit/conversations", handler.listAuditConversations)
	group.GET("/admin/audit/conversations/:conversation_id", handler.getAuditConversation)
}

// platformStats 查询平台统计，优先读取已生成的按日快照。
func (handler *Handler) platformStats(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	from, ok := optionalDate(ctx, "date_from")
	if !ok {
		return
	}
	to, ok := optionalDate(ctx, "date_to")
	if !ok {
		return
	}
	result, err := handler.service.PlatformStats(ctx.Request.Context(), from, to)
	respond(ctx, result, err)
}

// listOwners 分页查询平台 Owner 聚合列表。
func (handler *Handler) listOwners(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	page, size := pagination(ctx, "pageSize")
	result, err := handler.service.ListOwners(ctx.Request.Context(), ctx.Query("keyword"), page, size)
	respond(ctx, result, err)
}

// getOwner 查询单个 Owner 的 Agent、知识库与收入聚合详情。
func (handler *Handler) getOwner(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	result, err := handler.service.GetOwner(ctx.Request.Context(), ctx.Param("owner_id"))
	respond(ctx, result, err)
}

// listConsoleAgents 查询被 X-Invite-Code 隔离的运营首屏 Agent 列表。
func (handler *Handler) listConsoleAgents(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	ownerID, ok := ownerScope(ctx, "")
	if !ok {
		return
	}
	page, size := pagination(ctx, "pageSize")
	result, err := handler.service.ListConsoleAgents(ctx.Request.Context(), ownerID, ctx.Query("keyword"), ctx.Query("status"), page, size)
	respond(ctx, result, err)
}

// listAgents 查询被 X-Invite-Code 隔离的管理员 Agent 列表。
func (handler *Handler) listAgents(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	ownerID, ok := ownerScope(ctx, ctx.Query("ownerId"))
	if !ok {
		return
	}
	page, size := pagination(ctx, "pageSize")
	result, err := handler.service.ListAgents(ctx.Request.Context(), ownerID, ctx.Query("status"), page, size)
	respond(ctx, result, err)
}

// getAgent 查询管理员视角 Agent 详情。
func (handler *Handler) getAgent(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	result, err := handler.service.GetAgent(ctx.Request.Context(), ctx.Param("agent_id"))
	respond(ctx, result, err)
}

// pauseAgent 校验版本和原因后暂停 Agent。
func (handler *Handler) pauseAgent(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.PauseAgentRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.PauseAgent(ctx.Request.Context(), admin, ctx.Param("agent_id"), request)
	respond(ctx, result, err)
}

// resumeAgent 校验乐观锁版本后恢复 Agent。
func (handler *Handler) resumeAgent(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.ResumeAgentRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.ResumeAgent(ctx.Request.Context(), admin, ctx.Param("agent_id"), request)
	respond(ctx, result, err)
}

// getBillingRule 获取当前生效的计费规则。
func (handler *Handler) getBillingRule(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	result, err := handler.service.GetBillingRule(ctx.Request.Context())
	respond(ctx, result, err)
}

// updateBillingRule 创建新版本计费规则并保留旧版本和变更记录。
func (handler *Handler) updateBillingRule(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.BillingRuleUpdate
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UpdateBillingRule(ctx.Request.Context(), admin, request)
	respond(ctx, result, err)
}

// updateToolPricing 独立更新工具风险价格并生成完整计费规则版本。
func (handler *Handler) updateToolPricing(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.ToolPricingUpdate
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UpdateToolPricing(ctx.Request.Context(), admin, request)
	respond(ctx, result, err)
}

// getModelCoefficients 获取当前模型计费系数。
func (handler *Handler) getModelCoefficients(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	result, err := handler.service.GetModelCoefficient(ctx.Request.Context())
	respond(ctx, result, err)
}

// updateModelCoefficients 更新模型计费系数并生成新版本。
func (handler *Handler) updateModelCoefficients(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.ModelCoefficientUpdate
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UpdateModelCoefficient(ctx.Request.Context(), admin, request)
	respond(ctx, result, err)
}

// getPolicyRule 获取当前 Policy Guard 规则。
func (handler *Handler) getPolicyRule(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	result, err := handler.service.GetPolicyRule(ctx.Request.Context())
	respond(ctx, result, err)
}

// updatePolicyRule 更新 Policy Guard 规则并生成新版本。
func (handler *Handler) updatePolicyRule(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	var request dto.PolicyRuleUpdate
	if err := ctx.ShouldBindJSON(&request); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UpdatePolicyRule(ctx.Request.Context(), admin, request)
	respond(ctx, result, err)
}

// listAuditConversations 分页查询审计会话列表。
func (handler *Handler) listAuditConversations(ctx *gin.Context) {
	if _, ok := handler.admin(ctx); !ok {
		return
	}
	page, size := pagination(ctx, "page_size")
	from, ok := optionalDatePointer(ctx, "date_from")
	if !ok {
		return
	}
	to, ok := optionalDatePointer(ctx, "date_to")
	if !ok {
		return
	}
	result, err := handler.service.ListAuditConversations(ctx.Request.Context(), page, size, from, to, ctx.Query("agent_id"), ctx.Query("user_id"))
	respond(ctx, result, err)
}

// getAuditConversation 查询只读审计详情，并记录管理员查看动作。
func (handler *Handler) getAuditConversation(ctx *gin.Context) {
	admin, ok := handler.admin(ctx)
	if !ok {
		return
	}
	result, err := handler.service.GetAuditConversation(ctx.Request.Context(), admin, ctx.Param("conversation_id"))
	respond(ctx, result, err)
}

// admin 从可信登录态和审计请求头构建管理员上下文。
func (handler *Handler) admin(ctx *gin.Context) (operationsservice.AdminContext, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未认证的管理员调用")
		return operationsservice.AdminContext{}, false
	}
	adminID := principal.ID
	for _, header := range []string{"X-Admin-Id", "X-Invite-Code", "X-User-Id"} {
		if value := strings.TrimSpace(ctx.GetHeader(header)); value != "" {
			adminID = value
			break
		}
	}
	return operationsservice.AdminContext{AdminID: adminID, IPAddress: ctx.ClientIP(), UserAgent: ctx.Request.UserAgent()}, true
}

// ownerScope 强制列表只能读取 X-Invite-Code 对应 Owner 的数据。
func ownerScope(ctx *gin.Context, query string) (string, bool) {
	ownerID := strings.TrimSpace(ctx.GetHeader("X-Invite-Code"))
	if ownerID == "" {
		httpresponse.Failure(ctx, 400, "400_BAD_REQUEST", "请求缺少必填 Header: X-Invite-Code")
		return "", false
	}
	if strings.TrimSpace(query) != "" && strings.TrimSpace(query) != ownerID {
		httpresponse.Failure(ctx, 400, "400_BAD_REQUEST", "ownerId 必须与 X-Invite-Code 一致")
		return "", false
	}
	return ownerID, true
}

// pagination 读取分页参数并将非法边界交给 Service 统一归一化。
func pagination(ctx *gin.Context, sizeName string) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery(sizeName, "20"))
	return page, size
}

// optionalDate 解析可选的 YYYY-MM-DD 日期参数。
func optionalDate(ctx *gin.Context, name string) (time.Time, bool) {
	value := strings.TrimSpace(ctx.Query(name))
	if value == "" {
		return time.Time{}, true
	}
	result, err := time.Parse("2006-01-02", value)
	if err != nil {
		validation(ctx, errors.New(name+" 必须为 YYYY-MM-DD 格式"))
		return time.Time{}, false
	}
	return result, true
}

// optionalDatePointer 解析可选日期并保留“未提供”语义。
func optionalDatePointer(ctx *gin.Context, name string) (*time.Time, bool) {
	if strings.TrimSpace(ctx.Query(name)) == "" {
		return nil, true
	}
	result, ok := optionalDate(ctx, name)
	if !ok {
		return nil, false
	}
	return &result, true
}

// respond 写入统一 Agent API 响应信封。
func respond(ctx *gin.Context, result any, err error) {
	if err != nil {
		failure(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// failure 将 Operations 业务错误转换为兼容错误码。
func failure(ctx *gin.Context, err error) {
	message := err.Error()
	var operationErr *operationsservice.Error
	if errors.As(err, &operationErr) {
		message = operationErr.Message
	}
	httpresponse.Failure(ctx, operationsservice.StatusCode(err), operationsservice.ErrorCode(err), message)
}

// validation 返回统一请求参数校验错误。
func validation(ctx *gin.Context, err error) {
	httpresponse.Failure(ctx, 422, "422_VALIDATION_ERROR", err.Error())
}
