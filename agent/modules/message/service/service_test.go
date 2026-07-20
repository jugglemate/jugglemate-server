package service

import (
	"errors"
	"testing"

	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
)

// TestMessageErrorMapping 校验 Message 与 Reasoning 错误映射保持稳定。
func TestMessageErrorMapping(t *testing.T) {
	err := messageError(409, "409_CONFLICT", "冲突")
	if StatusCode(err) != 409 || ErrorCode(err) != "409_CONFLICT" {
		t.Fatalf("Message 错误映射错误: %d %s", StatusCode(err), ErrorCode(err))
	}
	reasoningErr := &reasoningservice.Error{Status: 403, Code: "403_AGENT_ARCHIVED", Message: "已归档"}
	if StatusCode(reasoningErr) != 403 || ErrorCode(reasoningErr) != "403_AGENT_ARCHIVED" {
		t.Fatalf("Reasoning 错误映射错误")
	}
	if ErrorCode(errors.New("unexpected")) != "500_MESSAGE_ERROR" {
		t.Fatal("未知错误应使用稳定兜底码")
	}
}

