package apis

import (
	"strings"

	"github.com/gin-gonic/gin"
	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/services"
)

// QryTicketRating 返回指定工单、客户、坐席组合的唯一一条评价。
//
// @route GET /jmate/tickets/:ticket_id/rating?customer_id=...
// @param ticket_id  path：工单 ID（必填）
// @param customer_id query：客户 ID（必填；按 UNIQUE (app_key, ticket_id, customer_id) 查询）
//
// 设计原因：评价写入全部走 jgm:csatreply IM 消息，不暴露 HTTP POST；
// GET 用于前端 UI 在客户打开评价卡之前判断"已评过"，决定隐藏/禁用按钮。
func QryTicketRating(ctx *gin.Context) {
	appKey := strings.TrimSpace(ctx.GetString(string(ctxs.CtxKey_AppKey)))
	if appKey == "" {
		appKey = strings.TrimSpace(ctxs.GetAppKeyFromCtx(ctx))
	}
	if appKey == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		return
	}
	ticketId := strings.TrimSpace(ctx.Param("ticket_id"))
	customerId := strings.TrimSpace(ctx.Query("customer_id"))
	if ticketId == "" || customerId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_ParamError)
		return
	}
	rating, err := services.GetTicketRating(appKey, ticketId, customerId)
	if err != nil {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		return
	}
	resp := &apiModels.QryTicketRatingResp{Rating: nil}
	if rating != nil {
		resp.Rating = &apiModels.TicketRatingInfo{
			TicketId:    rating.TicketId,
			CustomerId:  rating.CustomerId,
			AssigneeId:  rating.AssigneeId,
			Rating:      rating.Rating,
			Comment:     rating.Comment,
			CreatedTime: rating.CreatedTime,
		}
	}
	responses.SuccessHttpResp(ctx, resp)
}
