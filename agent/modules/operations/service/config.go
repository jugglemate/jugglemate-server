package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type billingRule struct {
	ID                string `gorm:"size:64;primaryKey"`
	Status            string
	Version           int
	TokenUnitPrice    decimal.Decimal `gorm:"column:token_unit_price"`
	ToolPricing       map[string]any  `gorm:"column:tool_pricing;serializer:json"`
	MinRechargeAmount int             `gorm:"column:min_recharge_amount"`
	Precision         int
	ValidityYears     *int `gorm:"column:validity_years"`
	Reason            *string
	UpdatedBy         *string `gorm:"column:updated_by"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (billingRule) TableName() string { return "billing_rules" }

type modelCoefficient struct {
	ID           string `gorm:"size:64;primaryKey"`
	Status       string
	Version      int
	Coefficients map[string]any `gorm:"serializer:json"`
	UpdatedBy    *string        `gorm:"column:updated_by"`
	Reason       *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (modelCoefficient) TableName() string { return "model_coefficients" }

type policyRule struct {
	ID                 string `gorm:"size:64;primaryKey"`
	Status             string
	Version            int
	HighRiskOperations []string       `gorm:"column:high_risk_operations;serializer:json"`
	TransactionLimits  map[string]any `gorm:"column:transaction_limits;serializer:json"`
	RateLimits         map[string]any `gorm:"column:rate_limits;serializer:json"`
	ApprovalFlow       string         `gorm:"column:approval_flow"`
	UpdatedBy          *string        `gorm:"column:updated_by"`
	Reason             *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (policyRule) TableName() string { return "policy_rules" }

const (
	billingInitializationLock int64 = 0x42494C4C494E47
	modelInitializationLock   int64 = 0x4D4F44454C4346
	policyInitializationLock  int64 = 0x504F4C49435952
)

// GetBillingRule 获取当前生效计费规则，不存在时幂等创建默认版本。
func (service *Service) GetBillingRule(ctx context.Context) (dto.BillingRule, error) {
	entity, err := service.ensureBilling(ctx)
	if err != nil {
		return dto.BillingRule{}, err
	}
	return billingDTO(entity), nil
}

// UpdateBillingRule 通过乐观版本锁生成新的 active 计费规则和变更日志。
func (service *Service) UpdateBillingRule(ctx context.Context, admin AdminContext, request dto.BillingRuleUpdate) (dto.BillingRule, error) {
	if _, err := service.ensureBilling(ctx); err != nil {
		return dto.BillingRule{}, err
	}
	var result billingRule
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current billingRule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status='active'").Order("version DESC").First(&current).Error; err != nil {
			return err
		}
		if current.Version != request.Version {
			return operationError(409, "409_CONFLICT", "规则已被其他管理员修改，请刷新后重试")
		}
		if err := validateToolPricing(request.ToolPricing); err != nil {
			return err
		}
		if err := tx.Model(&current).Update("status", "draft").Error; err != nil {
			return err
		}
		adminID := admin.AdminID
		validity := request.ValidityYears
		result = billingRule{ID: uuid.NewString(), Status: "active", Version: current.Version + 1, TokenUnitPrice: decimal.NewFromFloat(request.TokenUnitPrice), ToolPricing: map[string]any{"low_risk": request.ToolPricing.LowRisk, "medium_risk": request.ToolPricing.MediumRisk, "high_risk": request.ToolPricing.HighRisk}, MinRechargeAmount: request.MinRechargeAmount, Precision: request.Precision, ValidityYears: &validity, Reason: request.Reason, UpdatedBy: &adminID}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return createConfigChange(tx, "billing", result.ID, adminID, request.Reason, current, result)
	})
	return billingDTO(result), err
}

// UpdateToolPricing 仅更新工具价格并保留其他计费参数。
func (service *Service) UpdateToolPricing(ctx context.Context, admin AdminContext, request dto.ToolPricingUpdate) (dto.BillingRule, error) {
	current, err := service.GetBillingRule(ctx)
	if err != nil {
		return dto.BillingRule{}, err
	}
	return service.UpdateBillingRule(ctx, admin, dto.BillingRuleUpdate{Version: request.Version, TokenUnitPrice: current.TokenUnitPrice, ToolPricing: request.ToolPricing, MinRechargeAmount: current.MinRechargeAmount, Precision: current.Precision, ValidityYears: current.ValidityYears, Reason: request.Reason})
}

// GetModelCoefficient 获取当前生效模型系数。
func (service *Service) GetModelCoefficient(ctx context.Context) (dto.ModelCoefficient, error) {
	entity, err := service.ensureModel(ctx)
	if err != nil {
		return dto.ModelCoefficient{}, err
	}
	return modelDTO(entity), nil
}

// UpdateModelCoefficient 生成新的模型系数版本和审计日志。
func (service *Service) UpdateModelCoefficient(ctx context.Context, admin AdminContext, request dto.ModelCoefficientUpdate) (dto.ModelCoefficient, error) {
	if len(request.Coefficients) == 0 {
		return dto.ModelCoefficient{}, operationError(400, "400_BAD_REQUEST", "coefficients 不能为空")
	}
	for name, value := range request.Coefficients {
		if name == "" || value <= 0 {
			return dto.ModelCoefficient{}, operationError(400, "400_BAD_REQUEST", "模型名称不能为空且系数必须大于 0")
		}
	}
	if _, err := service.ensureModel(ctx); err != nil {
		return dto.ModelCoefficient{}, err
	}
	var result modelCoefficient
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current modelCoefficient
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status='active'").Order("version DESC").First(&current).Error; err != nil {
			return err
		}
		if current.Version != request.Version {
			return operationError(409, "409_CONFLICT", "规则已被其他管理员修改，请刷新后重试")
		}
		if err := tx.Model(&current).Update("status", "draft").Error; err != nil {
			return err
		}
		values := map[string]any{}
		for key, value := range request.Coefficients {
			values[key] = value
		}
		adminID := admin.AdminID
		result = modelCoefficient{ID: uuid.NewString(), Status: "active", Version: current.Version + 1, Coefficients: values, UpdatedBy: &adminID, Reason: request.Reason}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return createConfigChange(tx, "model", result.ID, adminID, request.Reason, current, result)
	})
	return modelDTO(result), err
}

// GetPolicyRule 获取当前生效 Policy Guard 规则。
func (service *Service) GetPolicyRule(ctx context.Context) (dto.PolicyRule, error) {
	entity, err := service.ensurePolicy(ctx)
	if err != nil {
		return dto.PolicyRule{}, err
	}
	return policyDTO(entity), nil
}

// UpdatePolicyRule 生成新的 Policy Guard 版本和审计日志。
func (service *Service) UpdatePolicyRule(ctx context.Context, admin AdminContext, request dto.PolicyRuleUpdate) (dto.PolicyRule, error) {
	if len(request.HighRiskOperations) == 0 || request.TransactionLimits.Single <= 0 || request.TransactionLimits.Daily < request.TransactionLimits.Single || request.RateLimits.PerUserDaily <= 0 || request.RateLimits.AnomalyThreshold <= 0 {
		return dto.PolicyRule{}, operationError(400, "400_BAD_REQUEST", "Policy Guard 参数非法")
	}
	if request.ApprovalFlow != "user_confirm" && request.ApprovalFlow != "owner_approve" && request.ApprovalFlow != "platform_approve" {
		return dto.PolicyRule{}, operationError(400, "400_BAD_REQUEST", "approvalFlow 非法")
	}
	if _, err := service.ensurePolicy(ctx); err != nil {
		return dto.PolicyRule{}, err
	}
	var result policyRule
	err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current policyRule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status='active'").Order("version DESC").First(&current).Error; err != nil {
			return err
		}
		if current.Version != request.Version {
			return operationError(409, "409_CONFLICT", "规则已被其他管理员修改，请刷新后重试")
		}
		if err := tx.Model(&current).Update("status", "draft").Error; err != nil {
			return err
		}
		adminID := admin.AdminID
		result = policyRule{ID: uuid.NewString(), Status: "active", Version: current.Version + 1, HighRiskOperations: request.HighRiskOperations, TransactionLimits: map[string]any{"single": request.TransactionLimits.Single, "daily": request.TransactionLimits.Daily}, RateLimits: map[string]any{"per_user_daily": request.RateLimits.PerUserDaily, "anomaly_threshold": request.RateLimits.AnomalyThreshold}, ApprovalFlow: request.ApprovalFlow, UpdatedBy: &adminID, Reason: request.Reason}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return createConfigChange(tx, "policy", result.ID, adminID, request.Reason, current, result)
	})
	return policyDTO(result), err
}

func (service *Service) ensureBilling(ctx context.Context) (billingRule, error) {
	var entity billingRule
	err := service.db.WithContext(ctx).Where("status='active'").Order("version DESC").First(&entity).Error
	if err == nil {
		return entity, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return entity, err
	}
	// 简要描述：使用 PostgreSQL 事务级 advisory lock 串行化首次初始化，并在锁内二次查询，
	// 防止多个实例并发启动时各自创建一条 active 默认规则。
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", billingInitializationLock).Error; err != nil {
			return err
		}
		if err := tx.Where("status='active'").Order("version DESC").First(&entity).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		validity := 10
		entity = billingRule{ID: uuid.NewString(), Status: "active", Version: 1, TokenUnitPrice: decimal.NewFromInt(5000), ToolPricing: map[string]any{"low_risk": 1.0, "medium_risk": 3.0, "high_risk": 5.0}, MinRechargeAmount: 100, Precision: 5, ValidityYears: &validity}
		return tx.Create(&entity).Error
	})
	return entity, err
}
func (service *Service) ensureModel(ctx context.Context) (modelCoefficient, error) {
	var entity modelCoefficient
	err := service.db.WithContext(ctx).Where("status='active'").Order("version DESC").First(&entity).Error
	if err == nil {
		return entity, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return entity, err
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", modelInitializationLock).Error; err != nil {
			return err
		}
		if err := tx.Where("status='active'").Order("version DESC").First(&entity).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entity = modelCoefficient{ID: uuid.NewString(), Status: "active", Version: 1, Coefficients: map[string]any{"claude-3-5-sonnet": 1.0, "gpt-4-turbo": 1.2, "gpt-3.5-turbo": 0.4}}
		return tx.Create(&entity).Error
	})
	return entity, err
}
func (service *Service) ensurePolicy(ctx context.Context) (policyRule, error) {
	var entity policyRule
	err := service.db.WithContext(ctx).Where("status='active'").Order("version DESC").First(&entity).Error
	if err == nil {
		return entity, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return entity, err
	}
	err = service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", policyInitializationLock).Error; err != nil {
			return err
		}
		if err := tx.Where("status='active'").Order("version DESC").First(&entity).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entity = policyRule{ID: uuid.NewString(), Status: "active", Version: 1, HighRiskOperations: []string{"execute_swap"}, TransactionLimits: map[string]any{"single": 10000.0, "daily": 100000.0}, RateLimits: map[string]any{"per_user_daily": 100, "anomaly_threshold": 5}, ApprovalFlow: "user_confirm"}
		return tx.Create(&entity).Error
	})
	return entity, err
}
func billingDTO(value billingRule) dto.BillingRule {
	validity := 10
	if value.ValidityYears != nil {
		validity = *value.ValidityYears
	}
	return dto.BillingRule{Version: value.Version, Status: value.Status, TokenUnitPrice: value.TokenUnitPrice.InexactFloat64(), ToolPricing: dto.ToolPricing{LowRisk: number(value.ToolPricing, "low_risk"), MediumRisk: number(value.ToolPricing, "medium_risk"), HighRisk: number(value.ToolPricing, "high_risk")}, MinRechargeAmount: value.MinRechargeAmount, Precision: value.Precision, ValidityYears: validity, UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}
func modelDTO(value modelCoefficient) dto.ModelCoefficient {
	coefficients := map[string]float64{}
	for key, item := range value.Coefficients {
		coefficients[key] = number(map[string]any{"v": item}, "v")
	}
	return dto.ModelCoefficient{Version: value.Version, Status: value.Status, Coefficients: coefficients, UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}
func policyDTO(value policyRule) dto.PolicyRule {
	return dto.PolicyRule{Version: value.Version, Status: value.Status, HighRiskOperations: value.HighRiskOperations, TransactionLimits: dto.TransactionLimits{Single: number(value.TransactionLimits, "single"), Daily: number(value.TransactionLimits, "daily")}, RateLimits: dto.RateLimits{PerUserDaily: integer(value.RateLimits, "per_user_daily"), AnomalyThreshold: integer(value.RateLimits, "anomaly_threshold")}, ApprovalFlow: value.ApprovalFlow, UpdatedAt: value.UpdatedAt, UpdatedBy: value.UpdatedBy}
}
func createConfigChange(tx *gorm.DB, kind, id, admin string, reason *string, oldValue, newValue any) error {
	oldJSON, err := json.Marshal(oldValue)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(newValue)
	if err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO config_change_logs(id,config_type,config_id,old_value,new_value,changed_by,reason,created_at) VALUES (?,?,?,?::jsonb,?::jsonb,?,?,?)`, uuid.NewString(), kind, id, string(oldJSON), string(newJSON), defaultString(admin, "admin"), reason, time.Now().UTC()).Error
}
func validateToolPricing(value dto.ToolPricing) error {
	if value.LowRisk < 0 || value.MediumRisk < value.LowRisk || value.HighRisk < value.MediumRisk {
		return operationError(400, "400_BAD_REQUEST", "工具价格必须非负并按风险等级递增")
	}
	return nil
}
func number(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		result, _ := value.Float64()
		return result
	case string:
		result, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return result
	case decimal.Decimal:
		return value.InexactFloat64()
	default:
		return 0
	}
}
func integer(values map[string]any, key string) int { return int(number(values, key)) }
