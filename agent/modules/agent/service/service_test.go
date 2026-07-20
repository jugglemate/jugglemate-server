package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
)

// TestAgentStateTransitions 校验 Agent 生命周期白名单。
func TestAgentStateTransitions(t *testing.T) {
	allowed := [][2]string{{"draft", "active"}, {"active", "paused"}, {"active", "error"}, {"paused", "active"}, {"paused", "archived"}, {"error", "active"}}
	for _, transition := range allowed {
		if err := assertTransition(transition[0], transition[1]); err != nil {
			t.Fatalf("合法状态流转被拒绝 %s -> %s: %v", transition[0], transition[1], err)
		}
	}
	var business *Error
	if err := assertTransition("draft", "archived"); !errors.As(err, &business) || business.Code != "409_INVALID_STATUS_TRANSITION" {
		t.Fatalf("非法状态流转应返回统一业务错误: %v", err)
	}
}

// TestAgentConfigPatchMerge 校验 ReAct 与记忆配置只覆盖显式字段。
func TestAgentConfigPatchMerge(t *testing.T) {
	react := defaultReactConfig()
	maxRounds := 8
	applyReactPatch(&react, &dto.ReactConfigPatch{MaxRounds: &maxRounds})
	if react.MaxRounds != 8 || react.TargetRounds != 3 || react.ToolsPerRound != 2 {
		t.Fatalf("ReAct 补丁合并结果不符合预期: %+v", react)
	}
	memory := defaultMemoryConfig("summary-model")
	enabled := true
	applyMemoryPatch(&memory, &dto.MemoryConfigPatch{LongTermEnabled: &enabled})
	if !memory.LongTermEnabled || memory.SummaryModel != "summary-model" || memory.ShortTermWindowSize != 5 {
		t.Fatalf("记忆补丁合并结果不符合预期: %+v", memory)
	}
}

// TestCapabilityDifferencesPreservesTargetOrder 校验能力同步先移除旧项并按目标顺序新增。
func TestCapabilityDifferencesPreservesTargetOrder(t *testing.T) {
	current := []dto.CapabilityItem{{ID: "b"}, {ID: "a"}, {ID: "old"}}
	remove, add := differences(current, []string{"b", "new-2", "new-1", "new-2"})
	if len(remove) != 2 || remove[0] != "a" || remove[1] != "old" {
		t.Fatalf("待移除能力集合不符合预期: %v", remove)
	}
	if len(add) != 2 || add[0] != "new-2" || add[1] != "new-1" {
		t.Fatalf("待新增能力顺序不符合预期: %v", add)
	}
}

// TestAgentConfigValidation 校验跨字段 ReAct 约束和记忆边界。
func TestAgentConfigValidation(t *testing.T) {
	react := defaultReactConfig()
	react.MaxRounds = 3
	react.TargetRounds = 4
	if err := validateReact(react); err == nil {
		t.Fatal("targetRounds 大于 maxRounds 时应拒绝")
	}
	memory := defaultMemoryConfig("")
	memory.LongTermInjectTopK = 11
	if err := validateMemory(memory); err == nil {
		t.Fatal("长期记忆注入条数超过上限时应拒绝")
	}
}

// TestNewBotUserIDFormat 校验 Bot 身份复用统一 ID 生成规则且互不重复。
func TestNewBotUserIDFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := newBotUserID()
		if !strings.HasPrefix(id, BotUserIDPrefix) {
			t.Fatalf("缺少前缀: %s", id)
		}
		// tools.GenerateUUIDShort22 固定产出 22 个字符。
		if got := len(id) - len(BotUserIDPrefix); got != 22 {
			t.Fatalf("随机段长度应为 22，实际 %d: %s", got, id)
		}
		// bots.bot_user_id 是 VARCHAR(64)。
		if len(id) > 64 {
			t.Fatalf("超出 bot_user_id 列长度: %s", id)
		}
		if seen[id] {
			t.Fatalf("生成了重复 ID: %s", id)
		}
		seen[id] = true
	}
}
