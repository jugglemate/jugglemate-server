// Package apis 提供 LLM 域的 Gin HTTP 接口。
package apis

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
)

// Handler 聚合 LLM 注册表和模型调用接口。
type Handler struct {
	registry *service.RegistryService
	calls    *service.CallService
}

// NewHandler 创建 LLM HTTP Handler。
func NewHandler(registry *service.RegistryService, calls *service.CallService) *Handler {
	return &Handler{registry: registry, calls: calls}
}

// RegisterRoutes 注册 LLM 域 16 个源兼容接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	providers := group.Group("/admin/llm/providers")
	providers.GET("", handler.listProviders)
	providers.POST("", handler.createProvider)
	providers.POST("/update", handler.updateProvider)
	providers.POST("/delete", handler.deleteProvider)
	providers.GET("/:provider_id", handler.getProvider)
	providers.POST("/:provider_id/set-default", handler.setDefaultProvider)
	providers.POST("/:provider_id/test", handler.testProvider)

	models := group.Group("/admin/llm/models")
	models.GET("", handler.listModels)
	models.POST("", handler.createModel)
	models.GET("/:model_pk", handler.getModel)
	models.PUT("/:model_pk", handler.updateModel)
	models.DELETE("/:model_pk", handler.deleteModel)

	defaults := group.Group("/admin/llm/defaults")
	defaults.GET("", handler.getDefaults)
	defaults.PUT("", handler.setDefaults)

	group.POST("/llm/call", handler.call)
	group.POST("/llm/call/stream", handler.stream)
}

// listProviders 获取 Provider 列表。
func (handler *Handler) listProviders(ctx *gin.Context) {
	items, err := handler.registry.ListProviders(ctx.Request.Context(), ctx.Query("protocol"), ctx.Query("status"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"items": items})
}

// getProvider 获取 Provider 详情。
func (handler *Handler) getProvider(ctx *gin.Context) {
	item, err := handler.registry.GetProvider(ctx.Request.Context(), ctx.Param("provider_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, item)
}

// createProvider 创建 Provider 及内联模型。
func (handler *Handler) createProvider(ctx *gin.Context) {
	var payload dto.ProviderCreate
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	item, err := handler.registry.CreateProvider(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Created(ctx, item)
}

// updateProvider 显式更新 Provider。
func (handler *Handler) updateProvider(ctx *gin.Context) {
	var payload dto.ProviderUpdate
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	item, err := handler.registry.UpdateProvider(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, item)
}

// deleteProvider 软删除 Provider 及其模型。
func (handler *Handler) deleteProvider(ctx *gin.Context) {
	var payload dto.ProviderDelete
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	if err := handler.registry.DeleteProvider(ctx.Request.Context(), payload.ProviderID); err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"success": true})
}

// setDefaultProvider 设置默认 Provider 并自动选择默认模型。
func (handler *Handler) setDefaultProvider(ctx *gin.Context) {
	result, err := handler.registry.SetDefaultProvider(ctx.Request.Context(), ctx.Param("provider_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// testProvider 测试 Provider 连接。
func (handler *Handler) testProvider(ctx *gin.Context) {
	result, err := handler.calls.TestProviderConnection(ctx.Request.Context(), ctx.Param("provider_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// listModels 获取模型列表。
func (handler *Handler) listModels(ctx *gin.Context) {
	capability := ctx.Query("capability")
	if capability != "" && capability != "reasoning" && capability != "summary" && capability != "embedding" {
		writeValidationError(ctx, errors.New("capability 仅支持 reasoning、summary、embedding"))
		return
	}
	items, err := handler.registry.ListModels(ctx.Request.Context(), ctx.Query("provider_id"), ctx.Query("status"), capability)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"items": items})
}

// getModel 获取模型详情。
func (handler *Handler) getModel(ctx *gin.Context) {
	item, err := handler.registry.GetModel(ctx.Request.Context(), ctx.Param("model_pk"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, item)
}

// createModel 创建模型。
func (handler *Handler) createModel(ctx *gin.Context) {
	var payload dto.ModelWrite
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	item, err := handler.registry.CreateModel(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Created(ctx, item)
}

// updateModel 更新模型。
func (handler *Handler) updateModel(ctx *gin.Context) {
	var payload dto.ModelUpdate
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	item, err := handler.registry.UpdateModel(ctx.Request.Context(), ctx.Param("model_pk"), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, item)
}

// deleteModel 检查引用后软删除模型。
func (handler *Handler) deleteModel(ctx *gin.Context) {
	if err := handler.registry.DeleteModel(ctx.Request.Context(), ctx.Param("model_pk")); err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"success": true})
}

// getDefaults 获取三类系统默认模型。
func (handler *Handler) getDefaults(ctx *gin.Context) {
	result, err := handler.registry.GetDefaults(ctx.Request.Context())
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// setDefaults 更新请求中显式给出的默认模型。
func (handler *Handler) setDefaults(ctx *gin.Context) {
	var payload map[string]*string
	decoder := json.NewDecoder(ctx.Request.Body)
	if err := decoder.Decode(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	for key := range payload {
		if key != "reasoning_model" && key != "summary_model" && key != "embedding_model" {
			writeValidationError(ctx, errors.New("请求包含未知字段: "+key))
			return
		}
	}
	result, err := handler.registry.SetDefaults(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// call 发起非流式模型调用。
func (handler *Handler) call(ctx *gin.Context) {
	var payload dto.CallRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	if err := validateCall(payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	result, err := handler.calls.Call(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// stream 发起不包裹统一信封的纯文本流式模型调用。
func (handler *Handler) stream(ctx *gin.Context) {
	var payload dto.CallRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	if err := validateCall(payload); err != nil {
		writeValidationError(ctx, err)
		return
	}
	chunks, err := handler.calls.Stream(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.Header("Content-Type", "text/plain; charset=utf-8")
	ctx.Status(http.StatusOK)
	for chunk := range chunks {
		if chunk.Err != nil {
			return
		}
		if chunk.Text == "" {
			continue
		}
		_, _ = ctx.Writer.Write([]byte(chunk.Text))
		ctx.Writer.Flush()
	}
}

func validateCall(payload dto.CallRequest) error {
	if strings.TrimSpace(payload.ModelID) == "" {
		return errors.New("model_id 不能为空")
	}
	if payload.MaxTokens != nil && *payload.MaxTokens < 1 {
		return errors.New("max_tokens 必须大于 0")
	}
	for _, message := range payload.Messages {
		if message.Role != "system" && message.Role != "user" && message.Role != "assistant" && message.Role != "tool" {
			return errors.New("消息 role 非法: " + message.Role)
		}
	}
	return nil
}

func writeValidationError(ctx *gin.Context, err error) {
	httpresponse.Failure(ctx, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, "请求参数校验失败: "+err.Error())
}

func writeError(ctx *gin.Context, err error) {
	var business *service.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Status, business.Message)
		return
	}
	httpresponse.Failure(ctx, http.StatusInternalServerError, http.StatusInternalServerError, "服务器内部错误")
}
