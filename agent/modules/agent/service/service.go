// Package service 实现 Agent Profile、生命周期、能力挂载和长期记忆用例。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	"github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	systemOwnerID  = "system"
	builtinAgentID = "Juggle_Agent"
	// statusDeleted 软删除状态：Agent 行保留（计费与会话历史仍可追溯），但对所有查询不可见。
	statusDeleted = "deleted"
	// statusActive 已激活状态：只有该状态的 Agent 能被绑定到 Inbox 并对外提供服务。
	statusActive = "active"
)

var allowedTransitions = map[string]map[string]bool{
	"draft":    {"active": true},
	"active":   {"paused": true, "error": true},
	"paused":   {"active": true, "archived": true},
	"error":    {"active": true},
	"archived": {},
}

// Config 定义 Agent 创建赠送和生命周期校验规则。
type Config struct {
	FirstAgentRechargeAmount     decimal.Decimal
	ActivationPointsCheckEnabled bool
}

// Actor 表示 Agent 用例调用方身份。
type Actor struct {
	OwnerID string
	AppKey  string
	IsAdmin bool
}

// Error 表示可映射到 HTTP 契约的 Agent 业务错误。
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error 返回 Agent 业务错误文本。
func (err *Error) Error() string { return err.Code + ": " + err.Message }

// Service 聚合 Agent 领域用例。
// UnbindInboxAgentFunc 解除单个 Inbox 的 Agent 绑定，并把 Bot 移出该 Inbox 下所有未关闭
// Ticket 群。
//
// TIPS: 用函数字段注入而不是直接 import 客服域的 services 包 —— 移出 Ticket 群属于客服域
// 能力，Agent 平台模块不应该反向依赖它；装配在 main.go 完成，未注入时删除会直接失败而不是
// 静默留下僵尸 Bot。
type UnbindInboxAgentFunc func(ctx context.Context, appKey, inboxID string) error

type Service struct {
	db               *gorm.DB
	billing          *billingservice.Service
	config           Config
	register         *imbot.RegisterClient
	connections      *imbot.Manager
	unbindInboxAgent UnbindInboxAgentFunc
}

// New 创建使用真实 PostgreSQL 与 Billing 的 Agent 服务。
func New(db *gorm.DB, billing *billingservice.Service, config Config, register *imbot.RegisterClient, connections *imbot.Manager) *Service {
	if config.FirstAgentRechargeAmount.IsNegative() {
		config.FirstAgentRechargeAmount = decimal.Zero
	}
	return &Service{db: db, billing: billing, config: config, register: register, connections: connections}
}

// SetUnbindInboxAgent 注入 Inbox 解绑实现，供删除 Agent 时清理关联使用。
func (service *Service) SetUnbindInboxAgent(fn UnbindInboxAgentFunc) {
	service.unbindInboxAgent = fn
}

func businessError(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}

// findAgent 按 ID 查询未删除的 Agent。
//
// TIPS: 这里统一排除软删除状态，让详情、更新、生命周期、能力挂载等所有入口默认 fail-closed；
// 逐个入口去判断 deleted 迟早会漏掉一处，从而对已删除的 Agent 继续操作。
func (service *Service) findAgent(ctx context.Context, agentID string) (*model.Agent, error) {
	var entity model.Agent
	err := service.db.WithContext(ctx).Where("id = ? AND status <> ?", agentID, statusDeleted).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, businessError(404, "404_NOT_FOUND", "Agent 不存在")
	}
	return &entity, err
}

func assertPermission(actor Actor, entity *model.Agent, allowBuiltin bool) error {
	if allowBuiltin && entity.ID == builtinAgentID && entity.OwnerID == systemOwnerID {
		return nil
	}
	if strings.TrimSpace(actor.AppKey) == "" || actor.AppKey != entity.AppKey {
		return businessError(403, "403_APP_KEY_MISMATCH", "Agent 不属于当前应用")
	}
	if actor.IsAdmin || actor.OwnerID == entity.OwnerID {
		return nil
	}
	return businessError(403, "403_FORBIDDEN", "无权限访问该 Agent")
}

func assertTransition(current, next string) error {
	if allowedTransitions[current][next] {
		return nil
	}
	return businessError(409, "409_INVALID_STATUS_TRANSITION", fmt.Sprintf("非法状态流转: %s -> %s", current, next))
}

func defaultReactConfig() dto.ReactConfig {
	return dto.ReactConfig{MaxRounds: 5, TargetRounds: 3, ToolsPerRound: 2, ConfidenceThreshold: 0.85}
}

func defaultMemoryConfig(summaryModel string) dto.MemoryConfig {
	return dto.MemoryConfig{ShortTermEnabled: true, ShortTermWindowSize: 5, SummaryEnabled: false, SummaryTriggerTokens: 2000, SummaryMaxTokens: 500, SummaryModel: summaryModel, LongTermEnabled: false, LongTermExtractOnClose: true, LongTermInjectTopK: 3, LongTermExpireDays: 90}
}

func reactMap(value dto.ReactConfig) map[string]any {
	return map[string]any{"maxRounds": value.MaxRounds, "targetRounds": value.TargetRounds, "toolsPerRound": value.ToolsPerRound, "confidenceThreshold": value.ConfidenceThreshold}
}

func memoryMap(value dto.MemoryConfig) map[string]any {
	return map[string]any{"shortTermEnabled": value.ShortTermEnabled, "shortTermWindowSize": value.ShortTermWindowSize, "summaryEnabled": value.SummaryEnabled, "summaryTriggerTokens": value.SummaryTriggerTokens, "summaryMaxTokens": value.SummaryMaxTokens, "summaryModel": value.SummaryModel, "longTermEnabled": value.LongTermEnabled, "longTermExtractOnClose": value.LongTermExtractOnClose, "longTermInjectTopK": value.LongTermInjectTopK, "longTermExpireDays": value.LongTermExpireDays}
}

func reactFromMap(value map[string]any) dto.ReactConfig {
	defaults := defaultReactConfig()
	defaults.MaxRounds = integer(value, "maxRounds", defaults.MaxRounds)
	defaults.TargetRounds = integer(value, "targetRounds", defaults.TargetRounds)
	defaults.ToolsPerRound = integer(value, "toolsPerRound", defaults.ToolsPerRound)
	defaults.ConfidenceThreshold = number(value, "confidenceThreshold", defaults.ConfidenceThreshold)
	return defaults
}

func memoryFromMap(value map[string]any) dto.MemoryConfig {
	defaults := defaultMemoryConfig("")
	defaults.ShortTermEnabled = boolean(value, "shortTermEnabled", defaults.ShortTermEnabled)
	defaults.ShortTermWindowSize = integer(value, "shortTermWindowSize", defaults.ShortTermWindowSize)
	defaults.SummaryEnabled = boolean(value, "summaryEnabled", defaults.SummaryEnabled)
	defaults.SummaryTriggerTokens = integer(value, "summaryTriggerTokens", defaults.SummaryTriggerTokens)
	defaults.SummaryMaxTokens = integer(value, "summaryMaxTokens", defaults.SummaryMaxTokens)
	defaults.SummaryModel = text(value, "summaryModel", defaults.SummaryModel)
	defaults.LongTermEnabled = boolean(value, "longTermEnabled", defaults.LongTermEnabled)
	defaults.LongTermExtractOnClose = boolean(value, "longTermExtractOnClose", defaults.LongTermExtractOnClose)
	defaults.LongTermInjectTopK = integer(value, "longTermInjectTopK", defaults.LongTermInjectTopK)
	defaults.LongTermExpireDays = integer(value, "longTermExpireDays", defaults.LongTermExpireDays)
	return defaults
}

func integer(values map[string]any, key string, fallback int) int {
	switch value := values[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case int64:
		return int(value)
	default:
		return fallback
	}
}
func number(values map[string]any, key string, fallback float64) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return fallback
	}
}
func boolean(values map[string]any, key string, fallback bool) bool {
	value, ok := values[key].(bool)
	if !ok {
		return fallback
	}
	return value
}
func text(values map[string]any, key, fallback string) string {
	value, ok := values[key].(string)
	if !ok {
		return fallback
	}
	return value
}

func normalizeRequired(value *string, field string, max int) (string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "", businessError(400, "400_MISSING_REQUIRED_FIELD", "请求缺少必填字段: "+field)
	}
	result := strings.TrimSpace(*value)
	if len([]rune(result)) > max {
		return "", businessError(400, "400_INVALID_CONFIG", field+" 长度超过上限")
	}
	return result, nil
}
