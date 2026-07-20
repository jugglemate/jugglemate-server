package services

import (
	"context"
	"os"
	"strings"
	"testing"

	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
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
	groupCalls := stubInboxAgentIMSdk(t)

	detail, err := BindInboxAgent(context.Background(), appKey, "inbox_bind_1", "agent_bind_1")
	if err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	if detail == nil || detail.BotUserID != "bot_user_bind_1" {
		t.Fatalf("绑定结果不符合预期: %+v", detail)
	}
	// TIPS: fixture 里有一张未关闭 Ticket，必须真的走到建群同步。之前 fixture 没有 Ticket，
	// syncOpenTicketAgentBot 直接 len==0 提前返回，导致整表扫描的类型错配（created_time
	// TIMESTAMPTZ → int64）在测试里永远不会暴露，最终在生产上炸成 17006。
	if len(*groupCalls) != 1 || (*groupCalls)[0] != "add:ticket_open_1:bot_user_bind_1" {
		t.Fatalf("未按预期同步未关闭 Ticket 群: %v", *groupCalls)
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

// TestRebindInboxAgentAgainstPostgres 验收换绑：新 Bot 必须先进群，旧 Bot 再被移出。
func TestRebindInboxAgentAgainstPostgres(t *testing.T) {
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
	seedSecondAgentFixture(t, db, appKey)
	calls := stubInboxAgentIMSdk(t)
	ctx := context.Background()

	if _, err := BindInboxAgent(ctx, appKey, "inbox_bind_1", "agent_bind_1"); err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	*calls = (*calls)[:0]

	// 换绑到另一个 Agent。
	detail, err := BindInboxAgent(ctx, appKey, "inbox_bind_1", "agent_bind_2")
	if err != nil {
		t.Fatalf("换绑失败: %v", err)
	}
	if detail == nil || detail.BotUserID != "bot_user_bind_2" {
		t.Fatalf("换绑结果不符: %+v", detail)
	}

	// 关键：必须先加新 Bot 再移除旧 Bot —— 反过来会让群在中途完全没有可用 Bot。
	want := []string{"add:ticket_open_1:bot_user_bind_2", "del:ticket_open_1:bot_user_bind_1"}
	if len(*calls) != 2 || (*calls)[0] != want[0] || (*calls)[1] != want[1] {
		t.Fatalf("换绑的群成员操作不符合预期:\n实际 %v\n期望 %v", *calls, want)
	}

	// 绑定记录必须指向新 Agent 与新 Bot。
	var row struct {
		AgentID string
		BotID   string
	}
	if err := db.Table("inbox_agent_bindings").Select("agent_id,bot_id").
		Where("app_key=? AND inbox_id=?", appKey, "inbox_bind_1").Take(&row).Error; err != nil {
		t.Fatalf("查询绑定失败: %v", err)
	}
	if row.AgentID != "agent_bind_2" || row.BotID != "bot_bind_2" {
		t.Fatalf("绑定未切换: %+v", row)
	}

	// 重复绑定同一个 Agent 时，旧 Bot 与新 Bot 是同一个，不能把它误删出群。
	*calls = (*calls)[:0]
	if _, err := BindInboxAgent(ctx, appKey, "inbox_bind_1", "agent_bind_2"); err != nil {
		t.Fatalf("重复绑定失败: %v", err)
	}
	for _, call := range *calls {
		if strings.HasPrefix(call, "del:") {
			t.Fatalf("重复绑定同一 Agent 不应移除其 Bot: %v", *calls)
		}
	}
}

// seedSecondAgentFixture 追加第二个带 active Bot 的 Agent，用于换绑。
func seedSecondAgentFixture(t *testing.T, db *gorm.DB, appKey string) {
	t.Helper()
	cleanup := func() {
		db.Exec("DELETE FROM bot_agent_bindings WHERE agent_id='agent_bind_2'")
		db.Exec("DELETE FROM bots WHERE id='bot_bind_2'")
		db.Exec("DELETE FROM agents WHERE id='agent_bind_2'")
	}
	cleanup()
	t.Cleanup(cleanup)
	statements := []struct {
		sql  string
		args []any
	}{
		{agentInsertSQL, []any{"agent_bind_2", "owner_1", "second agent", appKey}},
		{"INSERT INTO bots (id, owner_id, invite_code, bot_user_id, bot_name, token, status, app_key, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,now(),now())",
			[]any{"bot_bind_2", "owner_1", "invite_bind_2", "bot_user_bind_2", "second bot", "token_bind_2", "active", appKey}},
		{"INSERT INTO bot_agent_bindings (id, bot_id, agent_id, status, created_at, updated_at) VALUES (?,?,?,?,now(),now())",
			[]any{"bab_bind_2", "bot_bind_2", "agent_bind_2", "active"}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("准备第二个 Agent 失败: %v", err)
		}
	}
}

// stubInboxAgentIMSdk 挡掉真实 IM 调用，并记录建群同步动作。
func stubInboxAgentIMSdk(t *testing.T) *[]string {
	t.Helper()
	calls := make([]string, 0, 4)
	origSdk, origAdd, origDel := getImSdkForInboxAgent, addInboxAgentToGroup, removeInboxAgentFromGroup
	getImSdkForInboxAgent = func(string) *juggleimsdk.JuggleIMSdk { return &juggleimsdk.JuggleIMSdk{} }
	addInboxAgentToGroup = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		calls = append(calls, "add:"+req.GroupId+":"+strings.Join(req.MemberIds, ","))
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	removeInboxAgentFromGroup = func(_ *juggleimsdk.JuggleIMSdk, req juggleimsdk.GroupMembersReq) (juggleimsdk.ApiCode, string, error) {
		calls = append(calls, "del:"+req.GroupId+":"+strings.Join(req.MemberIds, ","))
		return juggleimsdk.ApiCode(errs.IMErrorCode_SUCCESS), "", nil
	}
	t.Cleanup(func() {
		getImSdkForInboxAgent, addInboxAgentToGroup, removeInboxAgentFromGroup = origSdk, origAdd, origDel
	})
	return &calls
}

const agentInsertSQL = `INSERT INTO agents
	(id, owner_id, name, type, prompt, status, react_config, memory_config,
	 total_invocations, success_count, fail_count, failure_rate, app_key, created_at, updated_at)
	VALUES (?,?,?,'assistant','p','active','{}'::jsonb,'{}'::jsonb,0,0,0,0,?,now(),now())`

// seedInboxAgentFixture 准备一个 Inbox、一个带 active Bot 的 Agent 和一个无 Bot 的 Agent。
func seedInboxAgentFixture(t *testing.T, db *gorm.DB, appKey string) {
	t.Helper()
	// TIPS: 清理必须覆盖 fixture 插入的每一张表，否则唯一键（如 uq_tickets_app_ticket）
	// 会让第二次运行卡在准备数据阶段，测试失败原因看起来就与被测代码无关。
	cleanup := func() {
		db.Exec("DELETE FROM inbox_agent_bindings WHERE app_key=?", appKey)
		db.Exec("DELETE FROM bot_agent_bindings WHERE agent_id IN ('agent_bind_1','agent_nobot')")
		db.Exec("DELETE FROM tickets WHERE app_key=?", appKey)
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
		// TIPS: 必须有一张未关闭 Ticket，否则同步逻辑会提前返回，测不到整表扫描的类型错配。
		// created_time 走 Postgres 的 TIMESTAMPTZ 默认值，正是模型里 int64 扫不出来的那列。
		{"INSERT INTO tickets (ticket_id, source_id, customer_id, inbox_id, channel_type, status, app_key) VALUES (?,?,?,?,?,?,?)",
			[]any{"ticket_open_1", "src_1", "cust_1", "inbox_bind_1", "widget", int(storageModels.TicketStatusProcessing), appKey}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("准备测试数据失败: %v", err)
		}
	}
}
