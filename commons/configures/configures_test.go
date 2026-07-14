package configures

import "testing"

func TestApplyDefaultsForAgentModule(t *testing.T) {
	conf := AppConfig{}
	applyDefaults(&conf)

	if conf.Port != 8050 {
		t.Fatalf("默认端口不符合预期: %d", conf.Port)
	}
	if conf.Agent.Postgres.MaxOpenConns != 50 || conf.Agent.Postgres.MaxIdleConns != 10 {
		t.Fatalf("PostgreSQL 连接池默认值不符合预期: %+v", conf.Agent.Postgres)
	}
	if conf.Agent.Knowledge.VectorDimension != 1536 || conf.Agent.Knowledge.VectorizeStream == "" {
		t.Fatalf("知识库默认值不符合预期: %+v", conf.Agent.Knowledge)
	}
	if conf.Agent.Tools.InnerAPIBaseURL == "" {
		t.Fatal("系统工具默认地址不能为空")
	}
	if conf.Agent.Billing.DirectFreeDailyEnabled == nil || !*conf.Agent.Billing.DirectFreeDailyEnabled {
		t.Fatal("直连每日免费配额默认应启用")
	}
	if conf.Agent.Billing.DirectFreeDailyTokens == nil || *conf.Agent.Billing.DirectFreeDailyTokens != 500000 || conf.Agent.Billing.DirectQuotaTimezone != "Asia/Shanghai" {
		t.Fatalf("Billing 配额默认值不符合预期: %+v", conf.Agent.Billing)
	}
	if conf.Agent.Profile.FirstAgentRechargeAmount != "10000" || conf.Agent.Profile.ActivationPointsCheckEnabled == nil || !*conf.Agent.Profile.ActivationPointsCheckEnabled {
		t.Fatalf("Agent Profile 默认值不符合预期: %+v", conf.Agent.Profile)
	}
	if conf.Agent.IM.Enabled == nil || !*conf.Agent.IM.Enabled || conf.Agent.IM.WSAddress != "wss://127.0.0.1" || conf.Agent.IM.DefaultBotName != "JG Agent Bot" {
		t.Fatalf("Agent IM 默认值不符合预期: %+v", conf.Agent.IM)
	}
}

// TestApplyDefaultsPreservesZeroFreeQuota 校验显式零额度不会被默认值覆盖。
func TestApplyDefaultsPreservesZeroFreeQuota(t *testing.T) {
	zero := 0
	conf := AppConfig{Agent: AgentConfig{Billing: AgentBillingConfig{DirectFreeDailyTokens: &zero}}}
	applyDefaults(&conf)
	if conf.Agent.Billing.DirectFreeDailyTokens == nil || *conf.Agent.Billing.DirectFreeDailyTokens != 0 {
		t.Fatal("显式配置的零免费额度不应被默认值覆盖")
	}
}

// TestApplyDefaultsPreservesDisabledFreeQuota 校验显式关闭免费配额不会被默认值覆盖。
func TestApplyDefaultsPreservesDisabledFreeQuota(t *testing.T) {
	disabled := false
	conf := AppConfig{Agent: AgentConfig{Billing: AgentBillingConfig{DirectFreeDailyEnabled: &disabled}}}
	applyDefaults(&conf)
	if conf.Agent.Billing.DirectFreeDailyEnabled == nil || *conf.Agent.Billing.DirectFreeDailyEnabled {
		t.Fatal("显式关闭的直连每日免费配额不应被重新启用")
	}
}
