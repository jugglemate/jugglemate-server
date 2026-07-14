package twins

import (
	"testing"
	"time"
)

// TestCompatibilityDTOConversions 校验 Twin 兼容响应保持旧 snake_case DTO 所需字段。
func TestCompatibilityDTOConversions(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	result := toTwin(mapping{OwnerID: "owner-1", UniqueName: "bot-1", DisplayName: "助手", Status: "trained", ActiveVersion: "v1", MaterialsCount: 2, CreatedAt: now, UpdatedAt: now})
	if result.OwnerID != "owner-1" || result.UniqueName != "bot-1" || result.ActiveVersion != "v1" || result.MaterialsCount != 2 {
		t.Fatalf("Twin 转换结果异常: %+v", result)
	}
}

// TestFirstNonEmpty 校验兼容字段回退会忽略空白值。
func TestFirstNonEmpty(t *testing.T) {
	if value := firstNonEmpty("  ", " greeting ", "fallback"); value != "greeting" {
		t.Fatalf("字段回退结果异常: %q", value)
	}
}
