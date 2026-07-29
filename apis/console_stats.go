package apis

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

// ConsoleStatsOverview 处理 GET /jmate/console/stats/overview。
//
// 鉴权由 RouteConsole 已挂的 consoleApis.Validate 接管（admin 角色校验），
// 这里只负责参数解析与响应分发。
func ConsoleStatsOverview(ctx *gin.Context) {
	code, resp := services.GetStatsOverview(
		ctxs.ToCtx(ctx),
		ctx.Query("from"),
		ctx.Query("to"),
		ctx.Query("granularity"),
	)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

// ConsoleStatsAgents 处理 GET /jmate/console/stats/agents。
//
// 分页用 page/pageSize（上限 100）；默认 page=1 pageSize=20。
func ConsoleStatsAgents(ctx *gin.Context) {
	limit, offset, ok := parseStatsPagination(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	includeAdmin := true
	if raw := strings.TrimSpace(ctx.Query("include_admin")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
			return
		}
		includeAdmin = parsed
	}
	code, resp := services.GetStatsAgents(
		ctxs.ToCtx(ctx),
		ctx.Query("from"),
		ctx.Query("to"),
		includeAdmin,
		limit,
		offset,
	)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

// ConsoleStatsCustomers 处理 GET /jmate/console/stats/customers。
func ConsoleStatsCustomers(ctx *gin.Context) {
	limit, offset, ok := parseStatsPagination(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.GetStatsCustomers(
		ctxs.ToCtx(ctx),
		ctx.Query("from"),
		ctx.Query("to"),
		ctx.Query("keyword"),
		ctx.Query("channel_type"),
		limit,
		offset,
	)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

// parseStatsPagination 把 page/pageSize 规整为 limit/offset。
//
// 上限 100，超过则夹紧到 100，避免误传 pageSize=10000 拉爆数据库。
func parseStatsPagination(ctx *gin.Context) (limit, offset int64, ok bool) {
	page := int64(1)
	limit = int64(20)
	if raw := strings.TrimSpace(ctx.Query("page")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, 0, false
		}
		page = parsed
	}
	if raw := strings.TrimSpace(ctx.Query("pageSize")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, 0, false
		}
		limit = parsed
	} else if raw := strings.TrimSpace(ctx.Query("page_size")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, 0, false
		}
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}
	offset = (page - 1) * limit
	return limit, offset, true
}
