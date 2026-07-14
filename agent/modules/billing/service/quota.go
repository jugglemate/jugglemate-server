package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/billing/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuotaConfig 定义直连用户每日免费配额及日期边界。
type QuotaConfig struct {
	FreeDailyEnabled bool
	FreeDailyTokens  int
	Timezone         string
}

// DirectQuotaSnapshot 表示直连用户当日配额、消耗和分桶余额。
type DirectQuotaSnapshot struct {
	QuotaDate                     time.Time
	GrantedTokens                 int
	UsedTokens                    int
	RemainingTokens               int
	FreeGrantedTokens             int
	SubscriptionGrantedTokens     int
	UsedFromFreeTokens            int
	UsedFromSubscriptionTokens    int
	FreeRemainingTokens           int
	SubscriptionRemainingTokens   int
	FreeGrantSourceStatus         *string
	SubscriptionGrantSourceStatus *string
}

// ConfigureQuota 设置直连用户日 Token 配额规则。
func (service *Service) ConfigureQuota(config QuotaConfig) error {
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return err
	}
	if config.FreeDailyTokens < 0 {
		config.FreeDailyTokens = 0
	}
	service.quota = config
	service.quotaLocation = location
	return nil
}

// EstimateDirectTokens 根据输入字符数与输出上限保守估算 Token。
func (service *Service) EstimateDirectTokens(contents []string, maxTokens *int) int {
	characters := 0
	for _, content := range contents {
		characters += len([]rune(content))
	}
	input := characters / 4
	if input < 1 {
		input = 1
	}
	output := 1024
	if maxTokens != nil && *maxTokens > 0 {
		output = *maxTokens
	}
	if output > 4096 {
		output = 4096
	}
	return input + output
}

// PreCheckDirectQuota 在直连 LLM 调用前补齐日发放并校验预估 Token。
func (service *Service) PreCheckDirectQuota(ctx context.Context, agentID, userID string, estimatedTokens int, now time.Time) (bool, DirectQuotaSnapshot, error) {
	snapshot, err := service.BuildDirectQuotaSnapshot(ctx, agentID, userID, now)
	if err != nil {
		return false, DirectQuotaSnapshot{}, err
	}
	return estimatedTokens <= 0 || snapshot.RemainingTokens >= estimatedTokens, snapshot, nil
}

// BuildDirectQuotaSnapshot 构建用户指定本地日的配额与成功调用消耗快照。
//
// 简要描述：配额发放与订阅变更在事务中完成；免费额度优先抵扣，订阅额度随后抵扣。
// 已用量只统计当前 agent+user 对应会话中的成功 LLM 调用，保持与源服务口径一致。
func (service *Service) BuildDirectQuotaSnapshot(ctx context.Context, agentID, userID string, now time.Time) (DirectQuotaSnapshot, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	location := service.quotaLocation
	if location == nil {
		location, _ = time.LoadLocation("Asia/Shanghai")
	}
	local := now.In(location)
	quotaDate := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	if err := service.ensureDailyGrants(ctx, userID, quotaDate, now); err != nil {
		return DirectQuotaSnapshot{}, err
	}
	var grants []model.TokenQuota
	if err := service.db.WithContext(ctx).Where("user_id = ? AND quota_date = ? AND status = 'issued'", userID, quotaDate).Find(&grants).Error; err != nil {
		return DirectQuotaSnapshot{}, err
	}
	free, subscription := 0, 0
	for _, grant := range grants {
		if grant.QuotaType == "free_daily" {
			free += grant.GrantTokens
		}
		if grant.QuotaType == "subscription_daily" {
			subscription += grant.GrantTokens
		}
	}
	dayStartLocal := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	dayStart, dayEnd := dayStartLocal.UTC(), dayStartLocal.AddDate(0, 0, 1).UTC()
	var used int
	query := `SELECT COALESCE(SUM(c.total_tokens),0) FROM llm_model_calls c JOIN conversations v ON c.conversation_id::text = v.id WHERE v.agent_id = ? AND v.user_id = ? AND c.status = 'success' AND c.request_time >= ? AND c.request_time < ?`
	if err := service.db.WithContext(ctx).Raw(query, agentID, userID, dayStart, dayEnd).Scan(&used).Error; err != nil {
		return DirectQuotaSnapshot{}, err
	}
	if used < 0 {
		used = 0
	}
	granted := free + subscription
	remaining := maximum(granted-used, 0)
	usedFree := minimum(used, free)
	usedSubscription := maximum(used-free, 0)
	snapshot := DirectQuotaSnapshot{QuotaDate: quotaDate, GrantedTokens: granted, UsedTokens: used, RemainingTokens: remaining, FreeGrantedTokens: free, SubscriptionGrantedTokens: subscription, UsedFromFreeTokens: usedFree, UsedFromSubscriptionTokens: usedSubscription, FreeRemainingTokens: maximum(free-used, 0), SubscriptionRemainingTokens: maximum(subscription-usedSubscription, 0)}
	if free > 0 {
		value := "gifted_free"
		snapshot.FreeGrantSourceStatus = &value
	}
	if subscription > 0 {
		value := "paid_subscription"
		snapshot.SubscriptionGrantSourceStatus = &value
	}
	return snapshot, nil
}

func (service *Service) ensureDailyGrants(ctx context.Context, userID string, quotaDate, now time.Time) error {
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var grants []model.TokenQuota
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND quota_date = ? AND status = 'issued'", userID, quotaDate).Find(&grants).Error; err != nil {
			return err
		}
		byType := map[string]model.TokenQuota{}
		for _, grant := range grants {
			byType[grant.QuotaType] = grant
		}
		free, hasFree := byType["free_daily"]
		if service.quota.FreeDailyEnabled && !hasFree {
			if err := createIssuedQuota(tx, userID, quotaDate, "free_daily", service.quota.FreeDailyTokens, "gifted_free", "auto_issue_daily_free_quota"); err != nil {
				return err
			}
		} else if !service.quota.FreeDailyEnabled && hasFree {
			if err := invalidateQuota(tx, &free, "daily_free_quota_disabled"); err != nil {
				return err
			}
		}
		var subscription model.Subscription
		err := tx.Where("user_id = ? AND status = 'active' AND period_start_at <= ? AND period_end_at > ?", userID, now, now).Order("period_end_at DESC").First(&subscription).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		grant, hasSubscription := byType["subscription_daily"]
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if hasSubscription {
				return invalidateQuota(tx, &grant, "subscription_not_active")
			}
			return nil
		}
		if hasSubscription && grant.GrantTokens == subscription.DailyQuotaTokens {
			return nil
		}
		if hasSubscription {
			if err := invalidateQuota(tx, &grant, "subscription_changed"); err != nil {
				return err
			}
		}
		return createIssuedQuota(tx, userID, quotaDate, "subscription_daily", subscription.DailyQuotaTokens, "paid_subscription", "auto_issue_subscription_"+subscription.PlanType)
	})
}

func createIssuedQuota(tx *gorm.DB, userID string, date time.Time, quotaType string, tokens int, source, reason string) error {
	entity := model.TokenQuota{ID: uuid.NewString(), UserID: userID, QuotaDate: date, QuotaType: quotaType, GrantTokens: maximum(tokens, 0), GrantSourceStatus: source, Status: "issued", Reason: &reason}
	// 简要描述：多实例并发补发时由部分唯一索引兜底，冲突请求视为幂等成功。
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&entity).Error
}
func invalidateQuota(tx *gorm.DB, entity *model.TokenQuota, reason string) error {
	entity.Status = "invalidated"
	entity.Reason = &reason
	return tx.Save(entity).Error
}
func minimum(left, right int) int {
	if left < right {
		return left
	}
	return right
}
func maximum(left, right int) int {
	if left > right {
		return left
	}
	return right
}
