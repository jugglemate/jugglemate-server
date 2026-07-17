package services

import (
	"context"
	"os"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestBindInboxAgentAgainstPostgres 在显式提供 DSN 时验收 Inbox-Agent 绑定的真实 SQL。
//
// TIPS: 这些查询用了表别名 + 无主键的目标结构体，SQL 只有跑在真实 PostgreSQL 上才会
// 暴露问题（例如 First 按第一个字段拼出 `ORDER BY a.agent_id` 导致 42703）。
func TestBindInboxAgentAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 AGENT_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	dbcommons.UsePostgres(db)
	t.Cleanup(func() { dbcommons.UsePostgres(nil) })

	const appKey = "test_bind_appkey"
	seedInboxAgentFixture(t, db, appKey)

	detail, err := BindInboxAgent(context.Background(), appKey, "inbox_bind_1", "agent_bind_1")
	if err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	if detail == nil || detail.BotUserID != "bot_user_bind_1" {
		t.Fatalf("绑定结果不符合预期: %+v", detail)
	}

	// 重复绑定必须走 upsert 而不是唯一键冲突。
	if _, err := BindInboxAgent(context.Background(), appKey, "inbox_bind_1", "agent_bind_1"); err != nil {
		t.Fatalf("重复绑定失败: %v", err)
	}

	got, err := GetInboxAgent(context.Background(), appKey, "inbox_bind_1")
	if err != nil || got == nil || got.AgentID != "agent_bind_1" {
		t.Fatalf("查询绑定失败: detail=%+v err=%v", got, err)
	}

	// 没有 active Bot 的 Agent 不可绑定，且必须是可识别的 ErrAgentNotBindable。
	if _, err := BindInboxAgent(context.Background(), appKey, "inbox_bind_1", "agent_nobot"); err == nil {
		t.Fatal("期望绑定无 Bot 的 Agent 失败，实际成功")
	}
}

const agentInsertSQL = `INSERT INTO agents
	(id, owner_id, name, type, prompt, status, react_config, memory_config,
	 total_invocations, success_count, fail_count, failure_rate, app_key, created_at, updated_at)
	VALUES (?,?,?,'assistant','p','active','{}'::jsonb,'{}'::jsonb,0,0,0,0,?,now(),now())`

// seedInboxAgentFixture 准备一个 Inbox、一个带 active Bot 的 Agent 和一个无 Bot 的 Agent。
func seedInboxAgentFixture(t *testing.T, db *gorm.DB, appKey string) {
	t.Helper()
	cleanup := func() {
		db.Exec("DELETE FROM inbox_agent_bindings WHERE app_key=?", appKey)
		db.Exec("DELETE FROM bot_agent_bindings WHERE agent_id IN ('agent_bind_1','agent_nobot')")
		db.Exec("DELETE FROM bots WHERE app_key=?", appKey)
		db.Exec("DELETE FROM agents WHERE app_key=?", appKey)
		db.Exec("DELETE FROM inboxes WHERE app_key=?", appKey)
	}
	cleanup()
	t.Cleanup(cleanup)

	statements := []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO inboxes (inbox_id, channel_type, name, app_key) VALUES (?,?,?,?)",
			[]any{"inbox_bind_1", "widget", "bind fixture", appKey}},
		{agentInsertSQL, []any{"agent_bind_1", "owner_1", "bind agent", appKey}},
		{agentInsertSQL, []any{"agent_nobot", "owner_1", "no bot agent", appKey}},
		{"INSERT INTO bots (id, owner_id, invite_code, bot_user_id, bot_name, token, status, app_key, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,now(),now())",
			[]any{"bot_bind_1", "owner_1", "invite_bind_1", "bot_user_bind_1", "bind bot", "token_bind_1", "active", appKey}},
		{"INSERT INTO bot_agent_bindings (id, bot_id, agent_id, status, created_at, updated_at) VALUES (?,?,?,?,now(),now())",
			[]any{"bab_bind_1", "bot_bind_1", "agent_bind_1", "active"}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("准备测试数据失败: %v", err)
		}
	}
}
