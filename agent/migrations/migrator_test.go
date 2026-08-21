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
	if len(items) != 9 {
		t.Fatalf("当前应包含 9 份迁移（含翻译模型调用类型约束），实际为 %d", len(items))
	}
	translation := migrationByName(t, items, "000008_allow_translation_model_calls.sql")
	if translation.Version != "000008" ||
		!strings.Contains(translation.SQL, "DROP CONSTRAINT IF EXISTS ck_llm_model_calls_call_type") ||
		!strings.Contains(translation.SQL, "ADD CONSTRAINT ck_llm_model_calls_call_type") ||
		!strings.Contains(translation.SQL, "CHECK (call_type IN") ||
		!strings.Contains(translation.SQL, "'translation'") {
		t.Fatalf("翻译调用类型迁移不完整: version=%s sql=%s", translation.Version, translation.SQL)
	}
	csatNotified := migrationByName(t, items, "000007_ticket_csat_notified_at.sql")
	if csatNotified.Version != "000007" ||
		!strings.Contains(csatNotified.SQL, "ALTER TABLE tickets") ||
		!strings.Contains(csatNotified.SQL, "csat_notified_at") {
		t.Fatalf("csat_notified_at 重发支持迁移不完整: version=%s", csatNotified.Version)
	}
	if items[5].Version != "000006" ||
		!strings.Contains(items[5].SQL, "ALTER TABLE tickets") ||
		!strings.Contains(items[5].SQL, "last_user_msg_at") ||
		!strings.Contains(items[5].SQL, "CREATE TABLE IF NOT EXISTS ticket_messages") ||
		!strings.Contains(items[5].SQL, "CREATE TABLE IF NOT EXISTS ticket_ratings") {
		t.Fatalf("工单自动关闭与评价迁移不完整: version=%s", items[5].Version)
	}
	if items[4].Version != "000005" ||
		!strings.Contains(items[4].SQL, "ALTER TABLE tickets") ||
		!strings.Contains(items[4].SQL, "is_human_taken_over") ||
		!strings.Contains(items[4].SQL, "CREATE TABLE IF NOT EXISTS ticket_events") {
		t.Fatalf("工单转人工标记迁移不完整: version=%s", items[4].Version)
	}
	if items[3].Version != "000004" ||
		!strings.Contains(items[3].SQL, "CREATE TABLE IF NOT EXISTS ticket_agent_bindings") ||
		!strings.Contains(items[3].SQL, "uq_tab_app_ticket") {
		t.Fatalf("工单-Agent 绑定迁移不完整: version=%s", items[3].Version)
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

func migrationByName(t *testing.T, items []Migration, name string) Migration {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("迁移列表缺少 %s", name)
	return Migration{}
}

func TestMigrationVersionRejectsInvalidName(t *testing.T) {
	if _, err := migrationVersion("baseline.sql"); err == nil {
		t.Fatal("非数字版本文件名应返回错误")
	}
}
