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
	if len(items) != 1 {
		t.Fatalf("当前应只有一份最终态基线，实际为 %d", len(items))
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
