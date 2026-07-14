// Package apis 提供 Tools v2 的 Gin HTTP 接口。
package apis

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	toolsservice "github.com/juggleim/jugglemate-server/agent/modules/tools/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 暴露 Tools v2 管理、执行、决策和观测接口。
type Handler struct{ service *toolsservice.Service }

// NewHandler 创建 Tools v2 Handler。
func NewHandler(service *toolsservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册 Tools v2 全部 29 个接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	providers := group.Group("/tool-providers")
	providers.POST("", handler.createProvider)
	providers.GET("", handler.listProviders)
	providers.GET("/:provider_id", handler.getProvider)
	providers.PATCH("/:provider_id", handler.updateProvider)
	providers.DELETE("/:provider_id", handler.deleteProvider)
	providers.POST("/:provider_id/transition", handler.transitionProvider)

	definitions := group.Group("/tool-definitions")
	definitions.POST("/plugin", handler.createPluginDefinition)
	definitions.POST("/custom/import", handler.importOpenAPI)
	definitions.POST("/custom", handler.createCustomDefinition)
	definitions.GET("", handler.listDefinitions)
	definitions.GET("/:definition_id", handler.getDefinition)
	definitions.PATCH("/:definition_id", handler.updateDefinition)
	definitions.DELETE("/:definition_id", handler.deleteDefinition)
	definitions.POST("/:definition_id/test", handler.testDefinition)
	definitions.POST("/:definition_id/activate", handler.activateDefinition)

	credentials := group.Group("/tool-credentials")
	credentials.POST("", handler.createCredential)
	credentials.GET("", handler.listCredentials)
	credentials.GET("/:credential_id", handler.getCredential)
	credentials.POST("/:credential_id/rotate", handler.rotateCredential)
	credentials.POST("/:credential_id/revoke", handler.revokeCredential)

	agents := group.Group("/agents")
	agents.POST("/:agent_id/tool-bindings", handler.mountTool)
	agents.PATCH("/:agent_id/tool-bindings/:tool_definition_id", handler.updateBinding)
	agents.DELETE("/:agent_id/tool-bindings/:tool_definition_id", handler.unmountTool)
	agents.GET("/:agent_id/tool-bindings", handler.listBindings)

	execution := group.Group("/internal/v1/tool-execution")
	execution.POST("/execute", handler.execute)
	execution.POST("/orchestrate", handler.orchestrate)

	group.GET("/tool-call-records", handler.listCallRecords)
	group.GET("/tool-decision-records", handler.listDecisionRecords)
	group.GET("/tool-decision-records/:record_id", handler.getDecisionRecord)
}

func (handler *Handler) createProvider(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.ProviderCreateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.CreateProvider(ctx, ownerID, command)
	respond(ctx, value, err)
}
func (handler *Handler) listProviders(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	value, err := handler.service.ListProviders(ctx, ownerID, ctx.Query("q"), ctx.Query("status"), ctx.Query("providerType"), page, size)
	respond(ctx, value, err)
}
func (handler *Handler) getProvider(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.GetProvider(ctx, ownerID, ctx.Param("provider_id"))
	respond(ctx, value, err)
}
func (handler *Handler) updateProvider(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.ProviderUpdateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.UpdateProvider(ctx, ownerID, ctx.Param("provider_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) deleteProvider(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	id := ctx.Param("provider_id")
	err := handler.service.DeleteProvider(ctx, ownerID, id)
	respond(ctx, gin.H{"message": "Provider 已删除", "providerId": id}, err)
}
func (handler *Handler) transitionProvider(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.TransitionProvider(ctx, ownerID, ctx.Param("provider_id"), ctx.Query("target_status"))
	respond(ctx, value, err)
}

func (handler *Handler) createPluginDefinition(ctx *gin.Context) {
	handler.createDefinition(ctx, "plugin")
}
func (handler *Handler) createCustomDefinition(ctx *gin.Context) {
	handler.createDefinition(ctx, "custom")
}
func (handler *Handler) createDefinition(ctx *gin.Context, toolType string) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.DefinitionCreateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.CreateDefinition(ctx, ownerID, toolType, command)
	respond(ctx, value, err)
}
func (handler *Handler) importOpenAPI(ctx *gin.Context) {
	var command dto.OpenAPIImportRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.ImportOpenAPI(ctx, command)
	respond(ctx, value, err)
}
func (handler *Handler) listDefinitions(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	value, err := handler.service.ListDefinitions(ctx, ownerID, ctx.Query("providerId"), ctx.Query("toolType"), ctx.Query("status"), ctx.Query("riskLevel"), ctx.Query("q"), page, size)
	respond(ctx, value, err)
}
func (handler *Handler) getDefinition(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.GetDefinition(ctx, ownerID, ctx.Param("definition_id"))
	respond(ctx, value, err)
}
func (handler *Handler) updateDefinition(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.DefinitionUpdateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.UpdateDefinition(ctx, ownerID, ctx.Param("definition_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) deleteDefinition(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	id := ctx.Param("definition_id")
	err := handler.service.DeleteDefinition(ctx, ownerID, id)
	respond(ctx, gin.H{"message": "工具定义已删除", "definitionId": id}, err)
}
func (handler *Handler) testDefinition(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.DefinitionTestRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.TestDefinition(ctx, ownerID, ctx.Param("definition_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) activateDefinition(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.ActivateDefinition(ctx, ownerID, ctx.Param("definition_id"))
	respond(ctx, value, err)
}

func (handler *Handler) createCredential(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.CredentialCreateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.CreateCredential(ctx, ownerID, command)
	respond(ctx, value, err)
}
func (handler *Handler) listCredentials(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	value, err := handler.service.ListCredentials(ctx, ownerID, ctx.Query("providerId"), ctx.Query("status"), page, size)
	respond(ctx, value, err)
}
func (handler *Handler) getCredential(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.GetCredential(ctx, ownerID, ctx.Param("credential_id"))
	respond(ctx, value, err)
}
func (handler *Handler) rotateCredential(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	var command dto.CredentialRotateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.RotateCredential(ctx, ownerID, ctx.Param("credential_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) revokeCredential(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.RevokeCredential(ctx, ownerID, ctx.Param("credential_id"))
	respond(ctx, value, err)
}

func (handler *Handler) mountTool(ctx *gin.Context) {
	var command dto.BindingCreateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.MountTool(ctx, ctx.Param("agent_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) updateBinding(ctx *gin.Context) {
	var command dto.BindingUpdateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.UpdateBinding(ctx, ctx.Param("agent_id"), ctx.Param("tool_definition_id"), command)
	respond(ctx, value, err)
}
func (handler *Handler) unmountTool(ctx *gin.Context) {
	agentID, id := ctx.Param("agent_id"), ctx.Param("tool_definition_id")
	err := handler.service.UnmountTool(ctx, agentID, id)
	respond(ctx, gin.H{"message": "工具已卸载", "agentId": agentID, "toolDefinitionId": id}, err)
}
func (handler *Handler) listBindings(ctx *gin.Context) {
	value, err := handler.service.ListBindings(ctx, ctx.Param("agent_id"))
	respond(ctx, value, err)
}

func (handler *Handler) execute(ctx *gin.Context) {
	var command dto.ExecuteRequest
	if !bind(ctx, &command) {
		return
	}
	httpresponse.Success(ctx, handler.service.Execute(ctx, command))
}
func (handler *Handler) orchestrate(ctx *gin.Context) {
	var command dto.OrchestrateRequest
	if !bind(ctx, &command) {
		return
	}
	value, err := handler.service.Orchestrate(ctx, command)
	respond(ctx, value, err)
}
func (handler *Handler) listCallRecords(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	value, err := handler.service.ListCallRecords(ctx, ownerID, ctx.Query("toolDefinitionId"), ctx.Query("status"), ctx.Query("traceId"), page, size)
	respond(ctx, value, err)
}
func (handler *Handler) listDecisionRecords(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	value, err := handler.service.ListDecisionRecords(ctx, ownerID, ctx.Query("agentId"), ctx.Query("action"), ctx.Query("traceId"), page, size)
	respond(ctx, value, err)
}
func (handler *Handler) getDecisionRecord(ctx *gin.Context) {
	value, err := handler.service.GetDecisionRecord(ctx, ctx.Param("record_id"))
	respond(ctx, value, err)
}

func owner(ctx *gin.Context) (string, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, http.StatusUnauthorized, "401_UNAUTHORIZED", "未登录")
		return "", false
	}
	return principal.ID, true
}
func pages(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	return page, size
}
func bind(ctx *gin.Context, target any) bool {
	if ctx.ShouldBindJSON(target) != nil {
		httpresponse.Failure(ctx, http.StatusUnprocessableEntity, 422, "请求参数校验失败")
		return false
	}
	return true
}
func respond(ctx *gin.Context, value any, err error) {
	if err == nil {
		httpresponse.Success(ctx, value)
		return
	}
	var business *toolsservice.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Code, business.Message)
		return
	}
	httpresponse.Failure(ctx, http.StatusInternalServerError, 500, "服务器内部错误")
}
