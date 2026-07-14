package service

import (
	"testing"

	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
)

// TestValidatePauseRequest 校验平台暂停原因白名单和“其他”说明长度。
func TestValidatePauseRequest(t *testing.T) {
	if err := validatePauseRequest(dto.PauseAgentRequest{Reason: "违规操作"}); err != nil {
		t.Fatalf("标准原因不应失败: %v", err)
	}
	short := "太短"
	if err := validatePauseRequest(dto.PauseAgentRequest{Reason: "其他", Description: &short}); err == nil {
		t.Fatal("其他原因的过短说明应被拒绝")
	}
	detail := "这是超过十个中文字符的完整暂停原因说明"
	if err := validatePauseRequest(dto.PauseAgentRequest{Reason: "其他", Description: &detail}); err != nil {
		t.Fatalf("合法其他原因不应失败: %v", err)
	}
}

// TestValidateToolPricing 校验工具价格必须按风险等级非递减。
func TestValidateToolPricing(t *testing.T) {
	if err := validateToolPricing(dto.ToolPricing{LowRisk: 1, MediumRisk: 3, HighRisk: 5}); err != nil {
		t.Fatalf("合法价格不应失败: %v", err)
	}
	if err := validateToolPricing(dto.ToolPricing{LowRisk: 3, MediumRisk: 2, HighRisk: 5}); err == nil {
		t.Fatal("风险价格倒挂应被拒绝")
	}
}
