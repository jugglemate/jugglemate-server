// Package apis 提供 Knowledge 域的 Gin HTTP 接口。
package apis

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/knowledge/model"
	knowledgeservice "github.com/juggleim/jugglemate-server/agent/modules/knowledge/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 提供 Knowledge 的 13 个源兼容接口。
type Handler struct{ service *knowledgeservice.Service }

// NewHandler 创建 Knowledge HTTP Handler。
func NewHandler(service *knowledgeservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册 Knowledge 域全部接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	routes := group.Group("/knowledge")
	routes.POST("", handler.create)
	routes.GET("", handler.list)
	routes.POST("/update", handler.update)
	routes.POST("/search", handler.search)
	routes.POST("/delete", handler.delete)
	routes.GET("/:knowledge_id", handler.get)
	routes.POST("/:knowledge_id/vectorize", handler.vectorize)
	routes.POST("/:knowledge_id/revectorize", handler.vectorize)
	routes.POST("/:knowledge_id/files", handler.uploadMetadata)
	routes.POST("/:knowledge_id/upload", handler.uploadMultipart)
	routes.GET("/:knowledge_id/files/:file_id/progress", handler.fileProgress)
	routes.GET("/:knowledge_id/progress", handler.progress)
	routes.GET("/:knowledge_id/chunks", handler.chunks)
}

// create 创建知识库草稿。
func (handler *Handler) create(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	var payload dto.Create
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		validation(ctx, err)
		return
	}
	entity, err := handler.service.Create(ctx.Request.Context(), owner, payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"id": entity.ID, "status": entity.Status})
}

// list 分页查询当前 Owner 的知识库。
func (handler *Handler) list(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	items, total, err := handler.service.List(ctx.Request.Context(), owner, ctx.Query("status"), ctx.Query("agentId"), page, pageSize)
	if err != nil {
		writeError(ctx, err)
		return
	}
	reads := make([]dto.Read, 0, len(items))
	for index := range items {
		reads = append(reads, toRead(&items[index]))
	}
	httpresponse.Success(ctx, gin.H{"items": reads, "total": total, "page": page, "pageSize": pageSize})
}

// get 获取知识库详情。
func (handler *Handler) get(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	entity, err := handler.service.Get(ctx.Request.Context(), owner, ctx.Param("knowledge_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, toRead(entity))
}

// vectorize 创建向量化任务。
func (handler *Handler) vectorize(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	task, err := handler.service.Trigger(ctx.Request.Context(), owner, ctx.Param("knowledge_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"status": "pending", "taskId": task.ID})
}

// update 更新知识库和向量化配置。
func (handler *Handler) update(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	var payload dto.Update
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		validation(ctx, err)
		return
	}
	entity, task, err := handler.service.Update(ctx.Request.Context(), owner, payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	response := gin.H{"config": dto.Config{ChunkSize: entity.ChunkSize, OverlapWindow: entity.OverlapWindow}, "status": entity.Status}
	if task != nil {
		response["taskId"] = task.ID
	}
	httpresponse.Success(ctx, response)
}

// uploadMetadata 接收旧版文件元数据请求。
func (handler *Handler) uploadMetadata(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	var payload dto.FileMetadataUpload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		validation(ctx, err)
		return
	}
	result, err := handler.service.UploadMetadata(ctx.Request.Context(), owner, ctx.Param("knowledge_id"), payload)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// uploadMultipart 接收并持久化真实文件内容。
func (handler *Handler) uploadMultipart(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		validation(ctx, err)
		return
	}
	trigger := strings.ToLower(ctx.DefaultQuery("triggerVectorize", "true")) != "false"
	result, err := handler.service.UploadMultipart(ctx.Request.Context(), owner, ctx.Param("knowledge_id"), header, trigger)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"status": result.Status, "knowledgeId": ctx.Param("knowledge_id"), "fileName": header.Filename, "fileSize": result.TotalBytes})
}

// fileProgress 查询文件接收进度。
func (handler *Handler) fileProgress(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	result, err := handler.service.FileProgress(ctx.Request.Context(), owner, ctx.Param("knowledge_id"), ctx.Param("file_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// progress 查询知识库构建任务进度。
func (handler *Handler) progress(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	result, err := handler.service.Progress(ctx.Request.Context(), owner, ctx.Param("knowledge_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, result)
}

// chunks 返回真实持久化知识分块。
func (handler *Handler) chunks(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	items, err := handler.service.Chunks(ctx.Request.Context(), owner, ctx.Param("knowledge_id"))
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"items": items})
}

// search 在当前 Owner 范围内检索知识分块。
func (handler *Handler) search(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	var payload dto.SearchRequest
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		validation(ctx, err)
		return
	}
	items, err := handler.service.Search(ctx.Request.Context(), owner, payload.Query, payload.TopK)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"items": items})
}

// delete 归档知识库。
func (handler *Handler) delete(ctx *gin.Context) {
	owner, ok := ownerID(ctx)
	if !ok {
		return
	}
	var payload dto.Delete
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		validation(ctx, err)
		return
	}
	if err := handler.service.Delete(ctx.Request.Context(), owner, payload.KnowledgeID); err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, gin.H{"status": "archived"})
}

func ownerID(ctx *gin.Context) (string, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未登录")
		return "", false
	}
	return principal.ID, true
}
func toRead(entity *model.Knowledge) dto.Read {
	return dto.Read{ID: entity.ID, Name: entity.Name, Type: entity.Type, SourceType: entity.SourceType, SourceURL: entity.SourceURL, Status: entity.Status, Config: dto.Config{ChunkSize: entity.ChunkSize, OverlapWindow: entity.OverlapWindow}, Stats: dto.Stats{DocumentCount: entity.DocumentCount, VectorCount: entity.VectorCount, HitRate: entity.HitRate, LastCalculatedAt: entity.LastCalculatedAt}, OwnerID: entity.OwnerID, WarningLowHitRate: entity.WarningLowHitRate, LastErrorCode: entity.LastErrorCode, LastErrorMessage: entity.LastErrorMessage, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
func validation(ctx *gin.Context, err error) {
	httpresponse.Failure(ctx, 422, 422, "请求参数校验失败: "+err.Error())
}
func writeError(ctx *gin.Context, err error) {
	var business *knowledgeservice.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Code, business.Message)
		return
	}
	httpresponse.Failure(ctx, http.StatusInternalServerError, 500, "服务器内部错误")
}
