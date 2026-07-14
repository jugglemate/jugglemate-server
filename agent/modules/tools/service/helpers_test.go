package service

import (
	"testing"

	"github.com/juggleim/jugglemate-server/agent/modules/tools/model"
)

// TestValidatePayloadMatchesSupportedSchemaSubset 校验 Tools v2 的基础 Schema 规则。
func TestValidatePayloadMatchesSupportedSchemaSubset(t *testing.T) {
	schema := map[string]any{"type": "object", "required": []string{"name", "count"}, "properties": map[string]any{"name": map[string]any{"type": "string"}, "count": map[string]any{"type": "integer"}}}
	if err := validatePayload(schema, map[string]any{"name": "demo", "count": float64(2)}); err != nil {
		t.Fatalf("合法载荷不应失败: %v", err)
	}
	if err := validatePayload(schema, map[string]any{"name": "demo", "count": 2.5}); err == nil {
		t.Fatal("非整数载荷应校验失败")
	}
}

// TestScoreCandidateUsesIntentAndRequiredFields 校验候选评分同时考虑意图和必填参数。
func TestScoreCandidateUsesIntentAndRequiredFields(t *testing.T) {
	description := "query order status"
	definition := model.Definition{ID: "tool-1", Name: "query_order", Description: &description, InputSchema: map[string]any{"type": "object", "required": []string{"orderId"}, "properties": map[string]any{"orderId": map[string]any{"type": "string"}}}}
	result := scoreCandidate("query order", map[string]any{"orderId": "42"}, definition)
	if result.Score < 0.55 || len(result.Unmatched) != 0 {
		t.Fatalf("候选应达到调用阈值且不存在缺失参数: %+v", result)
	}
}
