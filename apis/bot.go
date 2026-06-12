package apis

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglechat-server-ai/apis/models"
	"github.com/juggleim/jugglechat-server-ai/commons/ctxs"
	"github.com/juggleim/jugglechat-server-ai/commons/errs"
	"github.com/juggleim/jugglechat-server-ai/commons/responses"
	"github.com/juggleim/jugglechat-server-ai/services"
)

func CreateAiBot(ctx *gin.Context) {
	var req models.AiBotInfo
	if err := ctx.ShouldBindJSON(&req); err != nil || (req.Nickname == "" && req.DisplayName == "") {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, botInfo := services.CreateAiBot(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, botInfo)
}

func UpdateAiBot(ctx *gin.Context) {
	var req models.UpdateAiBotReq
	if err := ctx.ShouldBindJSON(&req); err != nil || req.BotId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, botInfo := services.UpdateAiBot(ctxs.ToCtx(ctx), &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, botInfo)
}

func RemoveAiBot(ctx *gin.Context) {
	var req struct {
		BotId string `json:"bot_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.BotId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code := services.RemoveAiBot(ctxs.ToCtx(ctx), req.BotId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, nil)
}

func QryMyAiBots(ctx *gin.Context) {
	count := int64(20)
	if val := ctx.Query("count"); val != "" {
		if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
			count = intVal
		}
	}
	offset := ctx.Query("offset")
	code, bots := services.QryMyAiBots(ctxs.ToCtx(ctx), count, offset)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, bots)
}

func AddAiMaterial(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	if uniqueName == "" {
		uniqueName = ctx.Query("unique_name")
	}
	var req models.AiMaterialInfo
	if err := ctx.ShouldBindJSON(&req); err != nil || uniqueName == "" || (req.Type == "" && req.Source == "") {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, material := services.AddAiMaterial(ctxs.ToCtx(ctx), uniqueName, &req)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, material)
}

func QryAiMaterials(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	if uniqueName == "" {
		uniqueName = ctx.Query("unique_name")
	}
	count := int64(20)
	if val := ctx.Query("count"); val != "" {
		if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
			count = intVal
		}
	}
	offset := ctx.Query("offset")
	code, materials := services.QryAiMaterials(ctxs.ToCtx(ctx), uniqueName, count, offset)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, materials)
}

func StartTraining(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	var req struct {
		Mode string `json:"mode"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || uniqueName == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, twin := services.StartTraining(ctxs.ToCtx(ctx), uniqueName, req.Mode)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, twin)
}

func ListJobs(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	code, data := services.ListJobs(ctxs.ToCtx(ctx), uniqueName, ctx.Query("status"), ctx.Query("type"), parseLimit(ctx.Query("limit")), ctx.Query("cursor"))
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, data)
}

func ListVersions(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	code, data := services.ListVersions(ctxs.ToCtx(ctx), uniqueName, parseLimit(ctx.Query("limit")), ctx.Query("cursor"))
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, data)
}

func ActivateVersion(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	version := ctx.Param("version")
	code, data := services.ActivateVersion(ctxs.ToCtx(ctx), uniqueName, version)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, data)
}

func GetCurrentVersion(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	code, data := services.GetCurrentVersion(ctxs.ToCtx(ctx), uniqueName)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, data)
}

func ListEvaluations(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	code, data := services.ListEvaluations(ctxs.ToCtx(ctx), uniqueName, parseLimit(ctx.Query("limit")), ctx.Query("cursor"))
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, data)
}

func parseLimit(val string) int64 {
	count := int64(20)
	if val != "" {
		if intVal, err := strconv.ParseInt(val, 10, 64); err == nil {
			count = intVal
		}
	}
	return count
}

func RemoveAiMaterial(ctx *gin.Context) {
	uniqueName := ctx.Param("unique_name")
	materialId := ctx.Param("material_id")
	if uniqueName == "" || materialId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code := services.RemoveAiMaterial(ctxs.ToCtx(ctx), uniqueName, materialId)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, nil)
}
