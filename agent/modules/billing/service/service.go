// Package service 实现 Billing 钱包、入账、计量和查询逻辑。
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/billing/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/billing/model"
	llmmodel "github.com/juggleim/jugglemate-server/agent/modules/llm/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Error 表示可返回调用方的 Billing 业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回 Billing 业务错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }
func billingError(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}

// Service 聚合 Billing 领域用例。
type Service struct {
	db            *gorm.DB
	quota         QuotaConfig
	quotaLocation *time.Location
}

// New 创建 Billing 服务。
func New(db *gorm.DB) *Service {
	location, _ := time.LoadLocation("Asia/Shanghai")
	return &Service{db: db, quota: QuotaConfig{FreeDailyEnabled: true, FreeDailyTokens: 500000, Timezone: "Asia/Shanghai"}, quotaLocation: location}
}

// GetWallet 查询钱包，不存在时以零余额幂等创建。
func (service *Service) GetWallet(ctx context.Context, ownerID string) (dto.WalletResponse, error) {
	var value dto.WalletResponse
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := ensureWallet(tx, ownerID)
		if err != nil {
			return err
		}
		value = walletResponse(wallet)
		return nil
	})
	return value, err
}

// SetWalletFrozen 冻结或解冻钱包，并通过行锁串行化管理员操作。
func (service *Service) SetWalletFrozen(ctx context.Context, ownerID string, frozen bool) (dto.WalletResponse, error) {
	var value dto.WalletResponse
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wallet model.Wallet
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).First(&wallet).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billingError(404, "404_WALLET_NOT_FOUND", "钱包不存在: "+ownerID)
		}
		if err != nil {
			return err
		}
		if frozen {
			wallet.Status = "frozen"
		} else {
			wallet.Status = "active"
		}
		wallet.Version++
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}
		value = walletResponse(wallet)
		return nil
	})
	return value, err
}

// CreateCompletedRecharge 创建充值订单并在同一事务中立即入账。
func (service *Service) CreateCompletedRecharge(ctx context.Context, ownerID string, amount decimal.Decimal) (dto.RechargeResponse, error) {
	if !amount.IsPositive() {
		return dto.RechargeResponse{}, billingError(422, "422_INVALID_AMOUNT", "充值金额必须大于 0")
	}
	var value dto.RechargeResponse
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := ensureWallet(tx, ownerID)
		if err != nil {
			return err
		}
		if wallet.Status == "frozen" {
			return billingError(403, "403_WALLET_FROZEN", "钱包已冻结")
		}
		order := model.RechargeOrder{ID: uuid.NewString(), OwnerID: ownerID, CreditAmount: amount, Status: "completed"}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if _, err := addCredits(tx, ownerID, amount); err != nil {
			return err
		}
		value = rechargeResponse(order)
		return nil
	})
	return value, err
}

// ListRecharges 分页查询 Owner 或平台全部充值订单。
func (service *Service) ListRecharges(ctx context.Context, ownerID, status string, start, end *time.Time, page, size int) (dto.RechargeListResponse, error) {
	page, size = pages(page, size)
	query := service.db.WithContext(ctx).Model(&model.RechargeOrder{})
	if ownerID != "" {
		query = query.Where("owner_id = ?", ownerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if end != nil {
		query = query.Where("created_at <= ?", *end)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.RechargeListResponse{}, err
	}
	var entities []model.RechargeOrder
	if err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&entities).Error; err != nil {
		return dto.RechargeListResponse{}, err
	}
	items := make([]dto.RechargeResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, rechargeResponse(entity))
	}
	return dto.RechargeListResponse{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// ConfirmRecharge 锁定 pending 订单、完成入账并记录管理员。
func (service *Service) ConfirmRecharge(ctx context.Context, orderID, adminID string) (dto.RechargeResponse, error) {
	return service.changeRecharge(ctx, orderID, adminID, true)
}

// CancelRecharge 锁定并取消 pending 订单。
func (service *Service) CancelRecharge(ctx context.Context, orderID, adminID string) (dto.RechargeResponse, error) {
	return service.changeRecharge(ctx, orderID, adminID, false)
}
func (service *Service) changeRecharge(ctx context.Context, orderID, adminID string, confirm bool) (dto.RechargeResponse, error) {
	var value dto.RechargeResponse
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.RechargeOrder
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return billingError(404, "404_ORDER_NOT_FOUND", "订单不存在: "+orderID)
		}
		if err != nil {
			return err
		}
		if order.Status != "pending" {
			return billingError(400, "400_INVALID_ORDER_STATUS", fmt.Sprintf("订单状态不正确: 当前 %s, 期望 pending", order.Status))
		}
		if confirm {
			order.Status = "completed"
			order.ConfirmedBy = &adminID
			if _, err := addCredits(tx, order.OwnerID, order.CreditAmount); err != nil {
				return err
			}
		} else {
			order.Status = "cancelled"
			order.CancelledBy = &adminID
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		value = rechargeResponse(order)
		return nil
	})
	return value, err
}

// CreateDeposit 创建 10 位数字外部入金订单。
func (service *Service) CreateDeposit(ctx context.Context, ownerID string, amount decimal.Decimal) (dto.DepositResponse, error) {
	if !amount.IsPositive() {
		return dto.DepositResponse{}, billingError(422, "422_INVALID_AMOUNT", "充值金额必须大于 0")
	}
	var wallet model.Wallet
	err := service.db.WithContext(ctx).Where("owner_id = ?", ownerID).First(&wallet).Error
	if err == nil && wallet.Status == "frozen" {
		return dto.DepositResponse{}, billingError(403, "403_WALLET_FROZEN", "钱包已冻结")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.DepositResponse{}, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		id, err := depositID()
		if err != nil {
			return dto.DepositResponse{}, err
		}
		order := model.DepositOrder{ID: id, OwnerID: ownerID, Amount: amount, Currency: "U", Status: "pending"}
		if err := service.db.WithContext(ctx).Create(&order).Error; err == nil {
			return depositResponse(order), nil
		}
	}
	return dto.DepositResponse{}, errors.New("生成唯一充值订单号失败")
}

// HandleDepositWebhook 幂等处理外部入金回调，成功时同事务增加钱包余额。
func (service *Service) HandleDepositWebhook(ctx context.Context, request dto.DepositWebhookRequest) dto.DepositWebhookResponse {
	result := dto.DepositWebhookResponse{Code: 0, Message: "处理成功"}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order model.DepositOrder
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", request.ID).First(&order).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = dto.DepositWebhookResponse{Code: -1, Message: "充值订单不存在: " + request.ID}
			return nil
		}
		if err != nil {
			return err
		}
		if order.Status != "pending" {
			result = dto.DepositWebhookResponse{Code: -1, Message: "充值订单已处理: " + order.Status}
			return nil
		}
		code := fmt.Sprint(request.Code)
		order.CallbackCode = &code
		order.CallbackMessage = &request.Message
		order.Remark = &request.Message
		if request.Code == 0 {
			order.Status = "success"
			if _, err := addCredits(tx, order.OwnerID, order.Amount); err != nil {
				return err
			}
		} else {
			order.Status = "fail"
		}
		return tx.Save(&order).Error
	})
	if err != nil {
		return dto.DepositWebhookResponse{Code: -1, Message: "处理失败"}
	}
	return result
}

// QueryTokenConsumption 按 owner/agent/provider/model 聚合 settled 消耗。
func (service *Service) QueryTokenConsumption(ctx context.Context, ownerID, dimension string, start, end time.Time) (dto.TokenConsumptionResponse, error) {
	if end.Sub(start) > 365*24*time.Hour {
		return dto.TokenConsumptionResponse{}, billingError(400, "400_DATE_RANGE_TOO_LARGE", "查询时间范围不能超过 365 天")
	}
	columns := map[string]string{"owner": "owner_id", "agent": "agent_id", "provider": "provider_id", "model": "model_id"}
	column, ok := columns[dimension]
	if !ok {
		return dto.TokenConsumptionResponse{}, billingError(422, "422_INVALID_DIMENSION", "聚合维度非法")
	}
	var items []dto.TokenConsumptionItem
	sql := fmt.Sprintf("SELECT %s AS dimension_value, COALESCE(SUM(input_tokens),0) input_tokens, COALESCE(SUM(output_tokens),0) output_tokens, COALESCE(SUM(total_tokens),0) total_tokens, COALESCE(SUM(credits_consumed),0) credits_consumed FROM consumption_records WHERE owner_id = ? AND event_date BETWEEN ? AND ? AND status = 'settled' GROUP BY %s", column, column)
	if err := service.db.WithContext(ctx).Raw(sql, ownerID, start, end).Scan(&items).Error; err != nil {
		return dto.TokenConsumptionResponse{}, err
	}
	return dto.TokenConsumptionResponse{Dimension: dimension, Items: items, StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02")}, nil
}

// QueryDailyConsumption 按日期聚合 settled 消耗并分页。
func (service *Service) QueryDailyConsumption(ctx context.Context, ownerID, agentID string, start, end time.Time, page, size int) (dto.DailyConsumptionResponse, error) {
	if end.Sub(start) > 365*24*time.Hour {
		return dto.DailyConsumptionResponse{}, billingError(400, "400_DATE_RANGE_TOO_LARGE", "查询时间范围不能超过 365 天")
	}
	page, size = pages(page, size)
	where := "owner_id = ? AND event_date BETWEEN ? AND ? AND status = 'settled'"
	args := []any{ownerID, start, end}
	if agentID != "" {
		where += " AND agent_id = ?"
		args = append(args, agentID)
	}
	var total int64
	if err := service.db.WithContext(ctx).Raw("SELECT COUNT(DISTINCT event_date) FROM consumption_records WHERE "+where, args...).Scan(&total).Error; err != nil {
		return dto.DailyConsumptionResponse{}, err
	}
	var items []dto.DailyConsumptionItem
	query := "SELECT event_date::text event_date, COALESCE(SUM(input_tokens),0) input_tokens, COALESCE(SUM(output_tokens),0) output_tokens, COALESCE(SUM(total_tokens),0) total_tokens, COUNT(DISTINCT session_id) session_count, COALESCE(SUM(credits_consumed),0) credits_consumed FROM consumption_records WHERE " + where + " GROUP BY event_date ORDER BY event_date DESC OFFSET ? LIMIT ?"
	args = append(args, (page-1)*size, size)
	if err := service.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error; err != nil {
		return dto.DailyConsumptionResponse{}, err
	}
	return dto.DailyConsumptionResponse{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// QuerySessionConsumption 查询指定日期的会话消耗明细。
func (service *Service) QuerySessionConsumption(ctx context.Context, ownerID, agentID string, eventDate time.Time) (dto.SessionConsumptionResponse, error) {
	where := "owner_id = ? AND event_date = ? AND status = 'settled'"
	args := []any{ownerID, eventDate}
	if agentID != "" {
		where += " AND agent_id = ?"
		args = append(args, agentID)
	}
	var items []dto.SessionConsumptionItem
	query := "SELECT session_id, agent_id, '' agent_name, model_id, COALESCE(SUM(input_tokens),0) input_tokens, COALESCE(SUM(output_tokens),0) output_tokens, COALESCE(SUM(total_tokens),0) total_tokens, COALESCE(SUM(credits_consumed),0) credits_consumed, COUNT(id) event_count FROM consumption_records WHERE " + where + " GROUP BY session_id,agent_id,model_id ORDER BY session_id"
	if err := service.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error; err != nil {
		return dto.SessionConsumptionResponse{}, err
	}
	return dto.SessionConsumptionResponse{Date: eventDate.Format("2006-01-02"), Items: items}, nil
}

// CalculateCredits 按 Token 数、模型系数和 Credit 单价计算四舍五入后的消耗。
func (service *Service) CalculateCredits(tokens int, coefficient, unitPrice decimal.Decimal, precision int) decimal.Decimal {
	if tokens <= 0 {
		return decimal.Zero
	}
	if !unitPrice.IsPositive() {
		unitPrice = decimal.NewFromInt(5000)
	}
	return decimal.NewFromInt(int64(tokens)).Mul(coefficient).Div(unitPrice).Round(int32(precision))
}

// PreCheck 校验钱包存在、未冻结且余额可覆盖预估 Credit。
func (service *Service) PreCheck(ctx context.Context, ownerID string, estimated decimal.Decimal) bool {
	var wallet model.Wallet
	if service.db.WithContext(ctx).Where("owner_id = ?", ownerID).First(&wallet).Error != nil {
		return false
	}
	return wallet.Status == "active" && !wallet.Balance.LessThan(estimated)
}

// EstimateCreditsForModel 按当前计费快照预估指定模型的 Credit 消耗。
func (service *Service) EstimateCreditsForModel(ctx context.Context, modelID string, estimatedTokens int) decimal.Decimal {
	unitPrice, precision, coefficient := service.billingSnapshot(ctx, modelID)
	return service.CalculateCredits(estimatedTokens, coefficient, unitPrice, precision)
}

// ChargeCompletedCall 使用当前生效计费规则结算一次成功 LLM 调用。
//
// 简要描述：计费规则和模型系数在扣费前读取为快照，随后由 Charge 在单一数据库事务中
// 同步写入消耗流水与钱包汇总；没有生效配置时沿用源服务默认值 5000 Token/Credit、系数 1。
func (service *Service) ChargeCompletedCall(ctx context.Context, ownerID, agentID, providerID, modelID, sessionID string, inputTokens, outputTokens int) (decimal.Decimal, error) {
	unitPrice, precision, coefficient := service.billingSnapshot(ctx, modelID)
	return service.Charge(ctx, ownerID, agentID, providerID, modelID, sessionID, "llm_call", inputTokens, outputTokens, coefficient, unitPrice, precision)
}

// ChargeEmbeddingCall 使用当前生效计费规则结算一次成功 Embedding 调用。
func (service *Service) ChargeEmbeddingCall(ctx context.Context, ownerID, knowledgeID, providerID, modelID string, totalTokens int) (decimal.Decimal, error) {
	unitPrice, precision, coefficient := service.billingSnapshot(ctx, modelID)
	return service.Charge(ctx, ownerID, knowledgeID, providerID, modelID, knowledgeID, "embedding", totalTokens, 0, coefficient, unitPrice, precision)
}

// ChargeEmbeddingCallWithFact 原子写入 Embedding Token 事实并完成 Credit 结算。
//
// 简要描述：llm_model_calls 是 Token 原始事实，consumption_records 与 credit_wallets
// 是 Credit 流水和汇总事实；三者必须在同一事务内提交，任一失败都会整体回滚。
func (service *Service) ChargeEmbeddingCallWithFact(ctx context.Context, ownerID, knowledgeID string, providerID uuid.UUID, modelID string, totalTokens int) (decimal.Decimal, error) {
	unitPrice, precision, coefficient := service.billingSnapshot(ctx, modelID)
	credits := service.CalculateCredits(totalTokens, coefficient, unitPrice, precision)
	if totalTokens <= 0 {
		return decimal.Zero, nil
	}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		zero := 0
		cost := decimal.Zero
		fact := llmmodel.ModelCall{ModelID: modelID, ProviderID: providerID, CallType: "embedding", RequestTime: time.Now().UTC(), Status: "success", InputTokens: &totalTokens, OutputTokens: &zero, TotalTokens: &totalTokens, Cost: &cost}
		if err := tx.Create(&fact).Error; err != nil {
			return err
		}
		return service.chargeTx(tx, ownerID, knowledgeID, providerID.String(), modelID, knowledgeID, "embedding", totalTokens, 0, coefficient, credits)
	})
	return credits, err
}

func (service *Service) billingSnapshot(ctx context.Context, modelID string) (decimal.Decimal, int, decimal.Decimal) {
	type billingRule struct {
		TokenUnitPrice decimal.Decimal `gorm:"column:token_unit_price"`
		Precision      int             `gorm:"column:precision"`
	}
	type coefficientRule struct {
		Coefficients map[string]any `gorm:"column:coefficients;serializer:json"`
	}
	unitPrice, precision, coefficient := decimal.NewFromInt(5000), 2, decimal.NewFromInt(1)
	var rule billingRule
	if service.db.WithContext(ctx).Table("billing_rules").Where("status = 'active'").Order("version DESC").First(&rule).Error == nil {
		if rule.TokenUnitPrice.IsPositive() {
			unitPrice = rule.TokenUnitPrice
		}
		precision = rule.Precision
	}
	var coefficients coefficientRule
	if service.db.WithContext(ctx).Table("model_coefficients").Where("status = 'active'").Order("version DESC").First(&coefficients).Error == nil {
		if raw, exists := coefficients.Coefficients[modelID]; exists {
			if parsed, err := decimal.NewFromString(fmt.Sprint(raw)); err == nil {
				coefficient = parsed
			}
		}
	}
	return unitPrice, precision, coefficient
}

// Charge 在同一事务内写 settled 流水并扣减钱包，防止并发超卖。
func (service *Service) Charge(ctx context.Context, ownerID, agentID, providerID, modelID, sessionID, eventType string, inputTokens, outputTokens int, coefficient, unitPrice decimal.Decimal, precision int) (decimal.Decimal, error) {
	total := inputTokens + outputTokens
	credits := service.CalculateCredits(total, coefficient, unitPrice, precision)
	if total <= 0 {
		return decimal.Zero, nil
	}
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return service.chargeTx(tx, ownerID, agentID, providerID, modelID, sessionID, eventType, inputTokens, outputTokens, coefficient, credits)
	})
	return credits, err
}

func (service *Service) chargeTx(tx *gorm.DB, ownerID, agentID, providerID, modelID, sessionID, eventType string, inputTokens, outputTokens int, coefficient, credits decimal.Decimal) error {
	total := inputTokens + outputTokens
	var wallet model.Wallet
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return billingError(404, "404_WALLET_NOT_FOUND", "钱包不存在: "+ownerID)
	}
	if err != nil {
		return err
	}
	if wallet.Status == "frozen" {
		return billingError(403, "403_WALLET_FROZEN", "钱包已冻结")
	}
	if wallet.Balance.LessThan(credits) {
		return billingError(403, "403_INSUFFICIENT_CREDITS", "余额不足")
	}
	record := model.ConsumptionRecord{ID: uuid.NewString(), OwnerID: ownerID, AgentID: agentID, ProviderID: providerID, ModelID: modelID, SessionID: sessionID, EventType: eventType, EventDate: time.Now().UTC(), InputTokens: inputTokens, OutputTokens: outputTokens, TotalTokens: total, ModelCoefficient: &coefficient, CreditsConsumed: credits, Status: "settled"}
	if err := tx.Create(&record).Error; err != nil {
		return err
	}
	wallet.Balance = wallet.Balance.Sub(credits)
	wallet.TotalConsumed = wallet.TotalConsumed.Add(credits)
	wallet.Version++
	return tx.Save(&wallet).Error
}

func ensureWallet(tx *gorm.DB, ownerID string) (model.Wallet, error) {
	seed := model.Wallet{ID: uuid.NewString(), OwnerID: ownerID, Balance: decimal.Zero, TotalRecharged: decimal.Zero, TotalConsumed: decimal.Zero, Status: "active", Version: 1}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}}, DoNothing: true}).Create(&seed).Error; err != nil {
		return model.Wallet{}, err
	}
	var wallet model.Wallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).First(&wallet).Error; err != nil {
		return model.Wallet{}, err
	}
	return wallet, nil
}
func addCredits(tx *gorm.DB, ownerID string, amount decimal.Decimal) (model.Wallet, error) {
	wallet, err := ensureWallet(tx, ownerID)
	if err != nil {
		return model.Wallet{}, err
	}
	wallet.Balance = wallet.Balance.Add(amount)
	wallet.TotalRecharged = wallet.TotalRecharged.Add(amount)
	wallet.Version++
	if err := tx.Save(&wallet).Error; err != nil {
		return model.Wallet{}, err
	}
	return wallet, nil
}
func walletResponse(value model.Wallet) dto.WalletResponse {
	return dto.WalletResponse{ID: value.ID, OwnerID: value.OwnerID, Balance: value.Balance.StringFixed(2) + "U", TotalRecharged: value.TotalRecharged.StringFixed(5), TotalConsumed: value.TotalConsumed.StringFixed(2) + "U", Status: value.Status, Version: value.Version, CreatedAt: format(value.CreatedAt), UpdatedAt: format(value.UpdatedAt)}
}
func rechargeResponse(value model.RechargeOrder) dto.RechargeResponse {
	var expires *string
	if value.ExpiresAt != nil {
		text := format(*value.ExpiresAt)
		expires = &text
	}
	return dto.RechargeResponse{ID: value.ID, OwnerID: value.OwnerID, CreditAmount: value.CreditAmount.StringFixed(5), ExpiresAt: expires, Status: value.Status, ConfirmedBy: value.ConfirmedBy, CancelledBy: value.CancelledBy, Reason: value.Reason, CreatedAt: format(value.CreatedAt), UpdatedAt: format(value.UpdatedAt)}
}
func depositResponse(value model.DepositOrder) dto.DepositResponse {
	return dto.DepositResponse{ID: value.ID, OwnerID: value.OwnerID, Amount: value.Amount.StringFixed(2), Currency: value.Currency, Status: value.Status, Remark: value.Remark, CreatedAt: format(value.CreatedAt), UpdatedAt: format(value.UpdatedAt)}
}
func pages(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func format(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func depositID() (string, error) {
	buffer := make([]byte, 10)
	for index := range buffer {
		number, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		buffer[index] = byte('0' + number.Int64())
	}
	return string(buffer), nil
}
