// Package apis 提供 Billing 的 Gin HTTP 接口。
package apis

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/billing/dto"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
	"github.com/shopspring/decimal"
)

// Handler 暴露 Billing 钱包、充值、入金和消耗查询接口。
type Handler struct{ service *billingservice.Service }

// NewHandler 创建 Billing Handler。
func NewHandler(service *billingservice.Service) *Handler { return &Handler{service: service} }

// RegisterRoutes 注册需要可信身份的 12 个 Billing 接口。
func (handler *Handler) RegisterRoutes(group *gin.RouterGroup) {
	billing := group.Group("/billing")
	billing.GET("/wallet", handler.wallet)
	billing.POST("/recharge", handler.recharge)
	billing.GET("/recharges", handler.ownerRecharges)
	billing.GET("/token-consumption", handler.tokenConsumption)
	billing.GET("/consumption/daily", handler.dailyConsumption)
	billing.GET("/consumption/daily/:event_date/sessions", handler.sessionConsumption)
	billing.POST("/deposit", handler.deposit)
	admin := group.Group("/admin/billing")
	admin.GET("/recharges", handler.adminRecharges)
	admin.POST("/recharges/:order_id/confirm", handler.confirmRecharge)
	admin.POST("/recharges/:order_id/cancel", handler.cancelRecharge)
	admin.POST("/wallets/:owner_id/freeze", handler.freezeWallet)
	admin.POST("/wallets/:owner_id/unfreeze", handler.unfreezeWallet)
}

// RegisterWebhookRoutes 注册免登录且不使用统一信封的外部充值回调。
func (handler *Handler) RegisterWebhookRoutes(group *gin.RouterGroup) {
	group.POST("/webhook/deposit", handler.depositWebhook)
}

func (handler *Handler) wallet(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	value, err := handler.service.GetWallet(ctx, ownerID)
	respond(ctx, value, err)
}
func (handler *Handler) recharge(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	amount, ok := amount(ctx)
	if !ok {
		return
	}
	value, err := handler.service.CreateCompletedRecharge(ctx, ownerID, amount)
	respond(ctx, value, err)
}
func (handler *Handler) ownerRecharges(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	handler.recharges(ctx, ownerID)
}
func (handler *Handler) adminRecharges(ctx *gin.Context) {
	handler.recharges(ctx, ctx.Query("ownerId"))
}
func (handler *Handler) recharges(ctx *gin.Context, ownerID string) {
	start, ok := optionalDateTime(ctx, "startDate")
	if !ok {
		return
	}
	end, ok := optionalDateTime(ctx, "endDate")
	if !ok {
		return
	}
	page, size := pageValues(ctx)
	value, err := handler.service.ListRecharges(ctx, ownerID, ctx.Query("status"), start, end, page, size)
	respond(ctx, value, err)
}
func (handler *Handler) confirmRecharge(ctx *gin.Context) {
	value, err := handler.service.ConfirmRecharge(ctx, ctx.Param("order_id"), adminID(ctx))
	respond(ctx, value, err)
}
func (handler *Handler) cancelRecharge(ctx *gin.Context) {
	value, err := handler.service.CancelRecharge(ctx, ctx.Param("order_id"), adminID(ctx))
	respond(ctx, value, err)
}
func (handler *Handler) freezeWallet(ctx *gin.Context) {
	value, err := handler.service.SetWalletFrozen(ctx, ctx.Param("owner_id"), true)
	respond(ctx, value, err)
}
func (handler *Handler) unfreezeWallet(ctx *gin.Context) {
	value, err := handler.service.SetWalletFrozen(ctx, ctx.Param("owner_id"), false)
	respond(ctx, value, err)
}
func (handler *Handler) deposit(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	amount, ok := amount(ctx)
	if !ok {
		return
	}
	value, err := handler.service.CreateDeposit(ctx, ownerID, amount)
	respond(ctx, value, err)
}
func (handler *Handler) depositWebhook(ctx *gin.Context) {
	var request dto.DepositWebhookRequest
	if ctx.ShouldBindJSON(&request) != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"detail": "请求参数校验失败"})
		return
	}
	ctx.JSON(http.StatusOK, handler.service.HandleDepositWebhook(ctx, request))
}
func (handler *Handler) tokenConsumption(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	start, ok := requiredDate(ctx, "startDate")
	if !ok {
		return
	}
	end, ok := requiredDate(ctx, "endDate")
	if !ok {
		return
	}
	value, err := handler.service.QueryTokenConsumption(ctx, ownerID, ctx.Query("dimension"), start, end)
	respond(ctx, value, err)
}
func (handler *Handler) dailyConsumption(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	start, ok := requiredDate(ctx, "startDate")
	if !ok {
		return
	}
	end, ok := requiredDate(ctx, "endDate")
	if !ok {
		return
	}
	page, size := pageValues(ctx)
	value, err := handler.service.QueryDailyConsumption(ctx, ownerID, ctx.Query("agentId"), start, end, page, size)
	respond(ctx, value, err)
}
func (handler *Handler) sessionConsumption(ctx *gin.Context) {
	ownerID, ok := owner(ctx)
	if !ok {
		return
	}
	eventDate, err := time.Parse("2006-01-02", ctx.Param("event_date"))
	if err != nil {
		validation(ctx)
		return
	}
	value, queryErr := handler.service.QuerySessionConsumption(ctx, ownerID, ctx.Query("agentId"), eventDate)
	respond(ctx, value, queryErr)
}

func owner(ctx *gin.Context) (string, bool) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, 401, "401_UNAUTHORIZED", "未登录")
		return "", false
	}
	return principal.ID, true
}
func adminID(ctx *gin.Context) string {
	if value := ctx.GetHeader("X-Admin-Id"); value != "" {
		return value
	}
	principal, err := identity.FromGin(ctx)
	if err == nil {
		return principal.ID
	}
	return "admin"
}
func amount(ctx *gin.Context) (decimal.Decimal, bool) {
	var request dto.AmountRequest
	if ctx.ShouldBindJSON(&request) != nil {
		validation(ctx)
		return decimal.Zero, false
	}
	value, err := decimal.NewFromString(fmt.Sprint(request.CreditAmount))
	if err != nil || !value.IsPositive() {
		validation(ctx)
		return decimal.Zero, false
	}
	return value, true
}
func requiredDate(ctx *gin.Context, name string) (time.Time, bool) {
	value, err := time.Parse("2006-01-02", ctx.Query(name))
	if err != nil {
		validation(ctx)
		return time.Time{}, false
	}
	return value, true
}
func optionalDateTime(ctx *gin.Context, name string) (*time.Time, bool) {
	raw := ctx.Query(name)
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		if parsed, dateErr := time.Parse("2006-01-02", raw); dateErr == nil {
			value = parsed
		} else {
			validation(ctx)
			return nil, false
		}
	}
	return &value, true
}
func pageValues(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	return page, size
}
func validation(ctx *gin.Context) { httpresponse.Failure(ctx, 422, 422, "请求参数校验失败") }
func respond(ctx *gin.Context, value any, err error) {
	if err == nil {
		httpresponse.Success(ctx, value)
		return
	}
	var business *billingservice.Error
	if errors.As(err, &business) {
		httpresponse.Failure(ctx, business.Status, business.Code, business.Message)
		return
	}
	httpresponse.Failure(ctx, 500, 500, "服务器内部错误")
}
