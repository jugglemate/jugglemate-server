// Package model 定义 Billing 域持久化实体。
package model

import (
	"github.com/shopspring/decimal"
	"time"
)

// Wallet 表示 Owner 的 Credit 钱包。
type Wallet struct {
	ID             string          `gorm:"size:64;primaryKey"`
	OwnerID        string          `gorm:"column:owner_id;size:64;not null;uniqueIndex"`
	Balance        decimal.Decimal `gorm:"type:numeric(12,5);not null"`
	TotalRecharged decimal.Decimal `gorm:"column:total_recharged;type:numeric(12,5);not null"`
	TotalConsumed  decimal.Decimal `gorm:"column:total_consumed;type:numeric(12,5);not null"`
	Status         string          `gorm:"size:16;not null"`
	Version        int             `gorm:"not null"`
	CreatedAt      time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回钱包表名。
func (Wallet) TableName() string { return "credit_wallets" }

// RechargeOrder 表示平台内直充或待管理员确认的充值订单。
type RechargeOrder struct {
	ID           string          `gorm:"size:64;primaryKey"`
	OwnerID      string          `gorm:"column:owner_id;size:64;not null"`
	CreditAmount decimal.Decimal `gorm:"column:credit_amount;type:numeric(12,5);not null"`
	ExpiresAt    *time.Time      `gorm:"column:expires_at"`
	Status       string          `gorm:"size:16;not null"`
	ConfirmedBy  *string         `gorm:"column:confirmed_by;size:64"`
	CancelledBy  *string         `gorm:"column:cancelled_by;size:64"`
	Reason       *string         `gorm:"size:500"`
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回充值订单表名。
func (RechargeOrder) TableName() string { return "recharge_orders" }

// DepositOrder 表示外部充值系统处理的入金订单。
type DepositOrder struct {
	ID              string          `gorm:"size:10;primaryKey"`
	OwnerID         string          `gorm:"column:owner_id;size:64;not null"`
	Amount          decimal.Decimal `gorm:"type:numeric(12,2);not null"`
	Currency        string          `gorm:"size:8;not null"`
	Status          string          `gorm:"size:16;not null"`
	Remark          *string         `gorm:"size:500"`
	CallbackCode    *string         `gorm:"column:callback_code;size:16"`
	CallbackMessage *string         `gorm:"column:callback_message;size:500"`
	CreatedAt       time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回外部充值订单表名。
func (DepositOrder) TableName() string { return "deposit_orders" }

// ConsumptionRecord 表示已按计费快照换算的 Token/Credit 流水。
type ConsumptionRecord struct {
	ID               string           `gorm:"size:64;primaryKey"`
	OwnerID          string           `gorm:"column:owner_id;size:64;not null"`
	AgentID          string           `gorm:"column:agent_id;size:64;not null"`
	ProviderID       string           `gorm:"column:provider_id;size:64;not null"`
	ModelID          string           `gorm:"column:model_id;size:64;not null"`
	SessionID        string           `gorm:"column:session_id;size:64;not null"`
	EventType        string           `gorm:"column:event_type;size:20;not null"`
	EventDate        time.Time        `gorm:"column:event_date;type:date;not null"`
	InputTokens      int              `gorm:"column:input_tokens;not null"`
	OutputTokens     int              `gorm:"column:output_tokens;not null"`
	TotalTokens      int              `gorm:"column:total_tokens;not null"`
	ModelCoefficient *decimal.Decimal `gorm:"column:model_coefficient;type:numeric(10,5)"`
	CreditsConsumed  decimal.Decimal  `gorm:"column:credits_consumed;type:numeric(12,5);not null"`
	Status           string           `gorm:"size:16;not null"`
	CreatedAt        time.Time        `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回消耗流水表名。
func (ConsumptionRecord) TableName() string { return "consumption_records" }

// Subscription 表示用户当前生效或历史订阅。
type Subscription struct {
	ID               string    `gorm:"size:64;primaryKey"`
	UserID           string    `gorm:"column:user_id;size:64;not null"`
	PlanType         string    `gorm:"column:plan_type;size:16;not null"`
	Status           string    `gorm:"size:16;not null"`
	DailyQuotaTokens int       `gorm:"column:daily_quota_tokens;not null"`
	PeriodStartAt    time.Time `gorm:"column:period_start_at;not null"`
	PeriodEndAt      time.Time `gorm:"column:period_end_at;not null"`
	ReplacedBy       *string   `gorm:"column:replaced_by;size:64"`
	CreatedBy        string    `gorm:"column:created_by;size:64;not null"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 返回用户订阅表名。
func (Subscription) TableName() string { return "user_subscriptions" }

// TokenQuota 表示用户某日某类型的有效或已失效 Token 发放记录。
type TokenQuota struct {
	ID                string    `gorm:"size:64;primaryKey"`
	UserID            string    `gorm:"column:user_id;size:64;not null"`
	QuotaDate         time.Time `gorm:"column:quota_date;type:date;not null"`
	QuotaType         string    `gorm:"column:quota_type;size:32;not null"`
	GrantTokens       int       `gorm:"column:grant_tokens;not null"`
	GrantSourceStatus string    `gorm:"column:grant_source_status;size:32;not null"`
	Status            string    `gorm:"size:16;not null"`
	Reason            *string   `gorm:"size:128"`
	IssuedAt          time.Time `gorm:"column:issued_at;autoCreateTime"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 返回用户日 Token 配额表名。
func (TokenQuota) TableName() string { return "user_token_quotas" }
