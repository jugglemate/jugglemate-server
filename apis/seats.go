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

// SeatsList 处理 GET /jmate/seats/list。
//
// 鉴权走外层 apis.Validate（仅校验已登录），不强制 admin 角色。
// 这让所有登录用户（admin 和客服）都能拿到联系人列表。
func SeatsList(ctx *gin.Context) {
	role, err := services.ParseRoleString(ctx.Query("role"))
	if err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	status, _, err := services.ParseSeatStatus(ctx.Query("status"))
	if err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	limit, offset, ok := parseSeatsPagination(ctx)
	if !ok {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	code, resp := services.ListSeats(
		ctxs.ToCtx(ctx),
		ctx.Query("keyword"),
		role,
		ctx.Query("inbox_id"),
		status,
		limit,
		offset,
	)
	if code != errs.IMErrorCode_SUCCESS {
		responses.ErrorHttpResp(ctx, code)
		return
	}
	responses.SuccessHttpResp(ctx, resp)
}

// parseSeatsPagination 把 page / pageSize 规整为 limit / offset，超过 100 夹紧到 100。
func parseSeatsPagination(ctx *gin.Context) (limit, offset int64, ok bool) {
	page := int64(1)
	if raw := strings.TrimSpace(ctx.Query("page")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, 0, false
		}
		page = parsed
	}
	limit = int64(20)
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
