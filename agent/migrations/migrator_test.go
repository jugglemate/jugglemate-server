package migrations

import (
	"strings"
	"testing"
)

func TestListContainsCompleteBaseline(t *testing.T) {
	items, err := List()
	if err != nil {
		t.Fatalf("读取迁移失败: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("当前应包含 Agent 基线、客服域收敛和 Agent 软删除迁移，实际为 %d", len(items))
	}
	if items[1].Version != "000002" ||
		!strings.Contains(items[1].SQL, "CREATE TABLE IF NOT EXISTS inbox_agent_bindings") ||
		!strings.Contains(items[1].SQL, "CREATE TABLE IF NOT EXISTS agent_first_recharge_grants") {
		t.Fatalf("客服域收敛迁移不完整: version=%s", items[1].Version)
	}
	// 软删除依赖 ck_agents_status 放开 deleted，否则 DeleteAgent 会被 CHECK 约束拒绝。
	if items[2].Version != "000003" || !strings.Contains(items[2].SQL, "'deleted'") ||
		!strings.Contains(items[2].SQL, "ck_agents_status") {
		t.Fatalf("Agent 软删除迁移不完整: version=%s", items[2].Version)
	}
	if items[0].Version != "000001" {
		t.Fatalf("基线版本不符合预期: %s", items[0].Version)
	}
	if count := strings.Count(items[0].SQL, "CREATE TABLE IF NOT EXISTS"); count != 48 {
		t.Fatalf("基线应创建 43 张源业务表和 5 张 Twin 兼容表，实际为 %d", count)
	}
	for _, table := range []string{"agents", "llm_providers", "knowledge_vectors", "react_sessions", "tool_definitions", "credit_wallets", "twin_compat_mappings", "twin_compat_jobs"} {
		if !strings.Contains(items[0].SQL, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("基线缺少核心表 %s", table)
		}
	}
}

func TestMigrationVersionRejectsInvalidName(t *testing.T) {
	if _, err := migrationVersion("baseline.sql"); err == nil {
		t.Fatal("非数字版本文件名应返回错误")
	}
}
