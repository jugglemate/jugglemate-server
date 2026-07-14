package service

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestCalculateCreditsUsesHalfUpPrecision 校验 Credit 换算精度。
func TestCalculateCreditsUsesHalfUpPrecision(t *testing.T) {
	service := New(nil)
	credits := service.CalculateCredits(125, decimal.NewFromFloat(1.5), decimal.NewFromInt(5000), 2)
	if !credits.Equal(decimal.NewFromFloat(0.04)) {
		t.Fatalf("Credit 换算结果应为 0.04，实际为 %s", credits)
	}
}

// TestDepositIDIsTenDigits 校验外部充值订单号格式。
func TestDepositIDIsTenDigits(t *testing.T) {
	value, err := depositID()
	if err != nil || len(value) != 10 {
		t.Fatalf("充值订单号生成失败: value=%q err=%v", value, err)
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			t.Fatalf("充值订单号必须为纯数字: %q", value)
		}
	}
}

// TestEstimateDirectTokensCapsOutput 校验直连配额估算会限制异常大的输出上限。
func TestEstimateDirectTokensCapsOutput(t *testing.T) {
	service := New(nil)
	maxTokens := 100000
	if value := service.EstimateDirectTokens([]string{"12345678"}, &maxTokens); value != 4098 {
		t.Fatalf("Token 估算应为 4098，实际为 %d", value)
	}
}

// TestConfigureQuotaRejectsUnknownTimezone 校验非法时区会阻止模块启动。
func TestConfigureQuotaRejectsUnknownTimezone(t *testing.T) {
	service := New(nil)
	if err := service.ConfigureQuota(QuotaConfig{Timezone: "Mars/Base"}); err == nil {
		t.Fatal("非法时区应返回错误")
	}
}
