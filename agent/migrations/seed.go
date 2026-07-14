package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// SeedConfig Agent 系统初始化数据配置。
type SeedConfig struct {
	InnerAPIBaseURL string
}

// Seed 写入 Agent 平台必需的系统数据。
//
// 所有数据均使用确定性 ID 和 ON CONFLICT，允许部署过程安全重试。
func Seed(ctx context.Context, db *gorm.DB, cfg SeedConfig) error {
	if db == nil {
		return fmt.Errorf("Agent PostgreSQL 连接不能为空")
	}
	if cfg.InnerAPIBaseURL == "" {
		cfg.InnerAPIBaseURL = "https://t.JG.pro"
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := seedBuiltinAgent(tx); err != nil {
			return err
		}
		if err := seedSystemWallet(tx); err != nil {
			return err
		}
		if err := seedSystemTool(tx, cfg.InnerAPIBaseURL); err != nil {
			return err
		}
		return nil
	})
}

// seedBuiltinAgent 初始化未绑定 Bot 的系统兜底 Agent。
func seedBuiltinAgent(tx *gorm.DB) error {
	const statement = `
		INSERT INTO agents (
			id, owner_id, name, type, prompt, llm_model, llm_provider_id,
			status, react_config, memory_config,
			total_invocations, success_count, fail_count, failure_rate,
			activated_at, created_at, updated_at
		) VALUES (
			'Juggle_Agent', 'system', 'Juggle Agent', 'assistant', ?, NULL, NULL,
			'active', CAST(? AS JSONB), CAST(? AS JSONB),
			0, 0, 0, 0, now(), now(), now()
		)
		ON CONFLICT (id) DO NOTHING`
	reactConfig := `{"maxRounds":5,"targetRounds":3,"toolsPerRound":2,"confidenceThreshold":0.85}`
	memoryConfig := `{"shortTermEnabled":true,"shortTermWindowSize":5,"summaryEnabled":false,"summaryTriggerTokens":2000,"summaryMaxTokens":500,"summaryModel":"","longTermEnabled":false,"longTermExtractOnClose":true,"longTermInjectTopK":3,"longTermExpireDays":90}`
	if err := tx.Exec(statement, "你是 Juggle Agent，一个友好、专业的智能助手，请用简洁清晰的语言帮助用户解决问题。", reactConfig, memoryConfig).Error; err != nil {
		return fmt.Errorf("初始化系统 Agent 失败: %w", err)
	}
	return nil
}

// seedSystemWallet 初始化系统 Agent 的运营预算钱包。
func seedSystemWallet(tx *gorm.DB) error {
	const statement = `
		INSERT INTO credit_wallets (
			id, owner_id, balance, total_recharged, total_consumed,
			status, version, created_at, updated_at
		) VALUES (
			'wallet-system', 'system', 100000, 100000, 0,
			'active', 1, now(), now()
		)
		ON CONFLICT (owner_id) DO NOTHING`
	if err := tx.Exec(statement).Error; err != nil {
		return fmt.Errorf("初始化系统钱包失败: %w", err)
	}
	return nil
}

// seedSystemTool 初始化所有 Owner 可见的系统支付查询工具。
func seedSystemTool(tx *gorm.DB, baseURL string) error {
	connectionConfig, _ := json.Marshal(map[string]string{"base_url": baseURL})
	authSchema := `{"type":"object","required":["api_key","in","name"],"properties":{"in":{"enum":["header","query"],"type":"string"},"name":{"type":"string","minLength":1},"api_key":{"type":"string","minLength":1}},"additionalProperties":false}`
	const providerStatement = `
		INSERT INTO tool_providers (
			id, owner_id, provider_type, name, display_name,
			connection_config, auth_type, auth_schema, status, visibility
		) VALUES (
			'tp_system_jg_inner_payments', 'admin', 'custom', 'JG_inner_payments', 'JG 内部支付服务',
			CAST(? AS JSONB), 'api_key', CAST(? AS JSONB), 'active', 'system'
		)
		ON CONFLICT (id) DO NOTHING`
	if err := tx.Exec(providerStatement, string(connectionConfig), authSchema).Error; err != nil {
		return fmt.Errorf("初始化系统工具 Provider 失败: %w", err)
	}

	const definitionStatement = `
		INSERT INTO tool_definitions (
			id, provider_id, owner_id, tool_type, name, description,
			input_schema, output_schema, execution_config,
			risk_level, approval_required, timeout_seconds, status
		) VALUES (
			'td_system_jg_payment_history', 'tp_system_jg_inner_payments', 'admin', 'custom',
			'查询 JG 支付订单详情', ?, CAST(? AS JSONB), CAST(? AS JSONB), CAST(? AS JSONB),
			'low', false, 15, 'active'
		)
		ON CONFLICT (id) DO NOTHING`
	description := "查询 JG 支付订单详情，返回订单号、金额、币种、链上 tx_hash、状态等信息。需要 merchant_order_no（如 P306220724577370112）。"
	inputSchema := `{"type":"object","required":["merchant_order_no"],"properties":{"merchant_order_no":{"type":"string","description":"商户订单号，例如 P306220724577370112"}}}`
	outputSchema := `{"type":"object","properties":{"code":{"type":"integer"},"data":{"type":"object"},"success":{"type":"boolean"}}}`
	executionConfig := `{"path":"/JG/inner/payments/aeon/history/{merchant_order_no}","method":"GET","paramMapping":{"path":{"merchant_order_no":"merchant_order_no"}},"outputFields":["data.merchant_order_no","data.status","data.amount","data.currency","data.usd_amount","data.token_symbol","data.network","data.tx_hash","data.trade_time_cn"]}`
	if err := tx.Exec(definitionStatement, description, inputSchema, outputSchema, executionConfig).Error; err != nil {
		return fmt.Errorf("初始化系统工具 Definition 失败: %w", err)
	}
	return nil
}
