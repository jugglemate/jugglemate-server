// Package apis 提供旧版 Capability 的 Gin 接口。
package apis

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/capability/dto"
	capabilityservice "github.com/juggleim/jugglemate-server/agent/modules/capability/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
)

// Handler 暴露 Tool、Skill 和混合发现接口。
type Handler struct{ service *capabilityservice.Service }

// NewHandler 创建 Capability Handler。
func NewHandler(service *capabilityservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册 Capability 域全部 17 个接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	tools := group.Group("/tools")
	tools.POST("", handler.createTool)
	tools.GET("/public", handler.publicTools)
	tools.GET("/mine", handler.myTools)
	tools.GET("/:tool_id", handler.getTool)
	tools.PUT("/:tool_id", handler.updateTool)
	tools.DELETE("/:tool_id", handler.deleteTool)
	tools.POST("/:tool_id/test", handler.testTool)
	skills := group.Group("/skills")
	skills.POST("", handler.createSkill)
	skills.GET("/public", handler.publicSkills)
	skills.GET("/mine", handler.mySkills)
	skills.GET("/:skill_id", handler.getSkill)
	skills.PUT("/:skill_id", handler.updateSkill)
	skills.DELETE("/:skill_id", handler.deleteSkill)
	skills.POST("/:skill_id/test", handler.testSkill)
	capabilities := group.Group("/capabilities")
	capabilities.GET("/public", handler.publicCapabilities)
	capabilities.GET("/tools/mine", handler.myTools)
	capabilities.GET("/skills/mine", handler.mySkills)
}

// createTool 创建 Tool。
func (handler *Handler) createTool(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.ToolWrite
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.CreateTool(ctx, owner, p)
	respond(ctx, item, err)
}

// getTool 获取 Tool。
func (handler *Handler) getTool(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	item, err := handler.service.GetTool(ctx, owner, ctx.Param("tool_id"))
	respond(ctx, item, err)
}

// updateTool 更新 Tool。
func (handler *Handler) updateTool(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.ToolUpdate
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.UpdateTool(ctx, owner, ctx.Param("tool_id"), p)
	respond(ctx, item, err)
}

// deleteTool 删除 Tool。
func (handler *Handler) deleteTool(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	id := ctx.Param("tool_id")
	err := handler.service.DeleteTool(ctx, owner, id)
	respond(ctx, gin.H{"message": "Tool 已删除", "toolId": id}, err)
}

// testTool 测试 Tool。
func (handler *Handler) testTool(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.TestRequest
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.TestTool(ctx, owner, ctx.Param("tool_id"), p.Payload)
	respond(ctx, item, err)
}

// publicTools 查询当前 Owner 可用 Tool。
func (handler *Handler) publicTools(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	items, total, err := handler.service.ListTools(ctx, owner, true, ctx.Query("q"), ctx.Query("riskLevel"), page, size)
	respond(ctx, gin.H{"items": items, "total": total}, err)
}

// myTools 查询我的 Tool。
func (handler *Handler) myTools(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	items, total, err := handler.service.ListTools(ctx, owner, false, "", "", page, size)
	respond(ctx, gin.H{"items": items, "total": total}, err)
}

// createSkill 创建 Skill。
func (handler *Handler) createSkill(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.SkillWrite
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.CreateSkill(ctx, owner, p)
	respond(ctx, item, err)
}

// getSkill 获取 Skill。
func (handler *Handler) getSkill(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	item, err := handler.service.GetSkill(ctx, owner, ctx.Param("skill_id"))
	respond(ctx, item, err)
}

// updateSkill 更新 Skill。
func (handler *Handler) updateSkill(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.SkillUpdate
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.UpdateSkill(ctx, owner, ctx.Param("skill_id"), p)
	respond(ctx, item, err)
}

// deleteSkill 删除 Skill。
func (handler *Handler) deleteSkill(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	id := ctx.Param("skill_id")
	err := handler.service.DeleteSkill(ctx, owner, id)
	respond(ctx, gin.H{"message": "Skill 已删除", "skillId": id}, err)
}

// testSkill 按顺序测试 Skill 所含 Tool。
func (handler *Handler) testSkill(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	var p dto.TestRequest
	if ctx.ShouldBindJSON(&p) != nil {
		validation(ctx)
		return
	}
	item, err := handler.service.TestSkill(ctx, owner, ctx.Param("skill_id"), p.Payload)
	respond(ctx, item, err)
}

// publicSkills 查询公共 Skill。
func (handler *Handler) publicSkills(ctx *gin.Context) {
	_, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	items, total, err := handler.service.ListSkills(ctx, "", true, ctx.Query("q"), ctx.Query("category"), ctx.Query("riskLevel"), page, size)
	respond(ctx, gin.H{"items": items, "total": total}, err)
}

// mySkills 查询我的 Skill。
func (handler *Handler) mySkills(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	items, total, err := handler.service.ListSkills(ctx, owner, false, "", "", "", page, size)
	respond(ctx, gin.H{"items": items, "total": total}, err)
}

// publicCapabilities 混合查询公共 Tool 与 Skill。
func (handler *Handler) publicCapabilities(ctx *gin.Context) {
	owner, ok := owner(ctx)
	if !ok {
		return
	}
	page, size := pages(ctx)
	kind := ctx.Query("type")
	items := []dto.DiscoveryItem{}
	var total int64
	if kind == "" || kind == "tool" {
		tools, count, err := handler.service.ListTools(ctx, owner, true, ctx.Query("q"), ctx.Query("riskLevel"), page, size)
		if err != nil {
			writeError(ctx, err)
			return
		}
		total += count
		for _, item := range tools {
			items = append(items, dto.DiscoveryItem{ID: item.ID, Name: item.Name, Type: "tool", Status: item.Status, Visibility: item.Visibility, RiskLevel: item.RiskLevel, CallCount: item.CallCount, UpdatedAt: item.UpdatedAt})
		}
	}
	if kind == "" || kind == "skill" {
		skills, count, err := handler.service.ListSkills(ctx, "", true, ctx.Query("q"), ctx.Query("category"), ctx.Query("riskLevel"), page, size)
		if err != nil {
			writeError(ctx, err)
			return
		}
		total += count
		for _, item := range skills {
			items = append(items, dto.DiscoveryItem{ID: item.ID, Name: item.Name, Type: "skill", Status: item.Status, Visibility: item.Visibility, RiskLevel: item.RiskLevel, Category: item.Category, CallCount: item.CallCount, UpdatedAt: item.UpdatedAt})
		}
	}
	httpresponse.Success(ctx, gin.H{"items": items, "total": total})
}

func owner(ctx *gin.Context) (string, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未登录")
		return "", false
	}
	return principal.ID, true
}
func pages(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	return page, size
}
func validation(ctx *gin.Context) { httpresponse.Failure(ctx, 422, 422, "请求参数校验失败") }
func respond(ctx *gin.Context, data any, err error) {
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpresponse.Success(ctx, data)
}
func writeError(ctx *gin.Context, err error) {
	var business *capabilityservice.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Code, business.Message)
		return
	}
	httpresponse.Failure(ctx, http.StatusInternalServerError, 500, "服务器内部错误")
}
