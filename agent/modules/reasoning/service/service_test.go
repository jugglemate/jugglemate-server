package service

import "testing"

// TestChoosePath 校验四条推理路径的确定性短路规则。
func TestChoosePath(t *testing.T) {
	if path := choosePath(true, true, nil); path != "direct_llm" {
		t.Fatalf("直连请求应跳过推理，实际 %s", path)
	}
	if path := choosePath(false, true, nil); path != "react" {
		t.Fatalf("存在工具时应强制 ReAct，实际 %s", path)
	}
	if path := choosePath(false, false, nil); path != "direct" {
		t.Fatalf("无能力时应走 Direct，实际 %s", path)
	}
}

// TestParseExtractedMemories 校验长期记忆过滤、默认重要度和去重。
func TestParseExtractedMemories(t *testing.T) {
	items := parseExtractedMemories("前缀\n```json\n[{\"type\":\"fact\",\"content\":\"用户在上海\",\"importance\":\"bad\"},{\"type\":\"fact\",\"content\":\"用户在上海\",\"importance\":\"high\"},{\"type\":\"invalid\",\"content\":\"丢弃\",\"importance\":\"low\"}]\n```")
	if len(items) != 1 {
		t.Fatalf("长期记忆应过滤并去重，实际 %+v", items)
	}
	if items[0].Importance != "medium" {
		t.Fatalf("非法重要度应降级为 medium，实际 %s", items[0].Importance)
	}
}

// TestSplitTokensPreservesUnicode 校验流式切分不会破坏中文字符。
func TestSplitTokensPreservesUnicode(t *testing.T) {
	tokens := splitTokens("你好A")
	if len(tokens) != 3 || tokens[0] != "你" || tokens[2] != "A" {
		t.Fatalf("Unicode token 切分错误: %v", tokens)
	}
}
