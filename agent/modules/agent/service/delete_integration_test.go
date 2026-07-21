package service

import (
	"context"
	"os"
	"testing"

	"github.com/juggleim/jugglemate-server/agent/migrations"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const deleteTestAppKey = "test_delete_appkey"

// TestDeleteAgentAgainstPostgres 验收删除 Agent 会清理全部关联并软删。
func TestDeleteAgentAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 AGENT_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接验收 PostgreSQL 失败: %v", err)
	}
	ctx := context.Background()
	// 000003 迁移把 deleted 加入 ck_agents_status；没有它下面的软删会被 CHECK 约束拒绝。
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	seedDeleteFixture(t, db)

	unbound := make([]string, 0, 2)
	service := New(db, nil, Config{}, nil, nil)
	service.SetUnbindInboxAgent(func(_ context.Context, appKey, inboxID string) error {
		unbound = append(unbound, appKey+":"+inboxID)
		// 真实实现会把 Bot 移出 Ticket 群后删除绑定行，这里只还原删行的部分。
		return db.Exec("DELETE FROM inbox_agent_bindings WHERE app_key=? AND inbox_id=?", appKey, inboxID).Error
	})
	actor := Actor{AppKey: deleteTestAppKey, OwnerID: "owner_del", IsAdmin: true}

	if err := service.DeleteAgent(ctx, actor, "agent_del_1"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	// 1) 已关联的 Inbox 必须被解绑（这是“agent 被关联要处理关联关系”的核心）。
	if len(unbound) != 1 || unbound[0] != deleteTestAppKey+":inbox_del_1" {
		t.Fatalf("未按预期解绑 Inbox: %v", unbound)
	}
	var bindingCount int64
	db.Table("inbox_agent_bindings").Where("agent_id=?", "agent_del_1").Count(&bindingCount)
	if bindingCount != 0 {
		t.Fatalf("inbox_agent_bindings 未清理，剩余 %d 行", bindingCount)
	}

	// 2) Agent 为软删：行还在（计费/会话历史可追溯），状态为 deleted。
	var status string
	if err := db.Table("agents").Where("id=?", "agent_del_1").Pluck("status", &status).Error; err != nil {
		t.Fatalf("查询 Agent 状态失败: %v", err)
	}
	if status != statusDeleted {
		t.Fatalf("期望软删为 %s，实际 %s", statusDeleted, status)
	}

	// 3) Bot 与 Bot-Agent 绑定一并停用，避免 Bot 继续连 IM 却路由不到 Agent。
	var botStatus, babStatus string
	db.Table("bots").Where("id=?", "bot_del_1").Pluck("status", &botStatus)
	db.Table("bot_agent_bindings").Where("agent_id=?", "agent_del_1").Pluck("status", &babStatus)
	if botStatus != "inactive" || babStatus != "inactive" {
		t.Fatalf("Bot 未停用: bot=%s binding=%s", botStatus, babStatus)
	}

	// 4) 能力挂载被卸载（软删不会触发外键级联，必须显式清理）。
	var knowledgeCount int64
	db.Table("agent_knowledge").Where("agent_id=?", "agent_del_1").Count(&knowledgeCount)
	if knowledgeCount != 0 {
		t.Fatalf("agent_knowledge 未卸载，剩余 %d 行", knowledgeCount)
	}

	// 5) 列表不再返回已删除 Agent。
	list, err := service.ListAgents(ctx, deleteTestAppKey, "owner_del", 1, 50)
	if err != nil {
		t.Fatalf("查询列表失败: %v", err)
	}
	for _, item := range list.Items {
		if item.AgentID == "agent_del_1" {
			t.Fatal("已删除的 Agent 仍出现在列表中")
		}
	}

	// 6) 重复删除返回 404 而不是再跑一遍解绑。
	if err := service.DeleteAgent(ctx, actor, "agent_del_1"); err == nil {
		t.Fatal("重复删除应当失败")
	}
	if len(unbound) != 1 {
		t.Fatalf("重复删除不应再次解绑: %v", unbound)
	}
}

// TestCreateAgentAfterSoftDeleteAgainstPostgres 验收「软删掉全部 Agent 后仍能重新创建」。
//
// TIPS: 软删是回归高发点 —— agent_first_recharge_grants 以 (app_key, owner_id) 为主键，
// 且只跟随物理删除级联清理。软删后 Agent 数量重新变成 0，创建流程会以为这是首个 Agent 并
// 再次插入预留记录，直接主键冲突、接口报 500。
func TestCreateAgentAfterSoftDeleteAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 AGENT_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接验收 PostgreSQL 失败: %v", err)
	}
	ctx := context.Background()
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	var model struct {
		ModelID string
	}
	if err := db.Table("llm_models").Select("model_id").
		Where("status = 'active' AND 'reasoning' = ANY(model_types)").First(&model).Error; err != nil {
		t.Skipf("本地没有可用的 active reasoning 模型，跳过: %v", err)
	}

	cleanup := func() {
		db.Exec("DELETE FROM agent_first_recharge_grants WHERE app_key=?", deleteTestAppKey)
		db.Exec("DELETE FROM agent_knowledge WHERE agent_id IN (SELECT id FROM agents WHERE app_key=?)", deleteTestAppKey)
		db.Exec("DELETE FROM agents WHERE app_key=?", deleteTestAppKey)
	}
	cleanup()
	t.Cleanup(cleanup)

	service := New(db, nil, Config{}, nil, nil)
	actor := Actor{AppKey: deleteTestAppKey, OwnerID: "owner_recreate", IsAdmin: true}
	name, agentType, prompt := "first agent", "assistant", "p"
	request := dto.CreateRequest{Name: &name, Type: &agentType, Prompt: &prompt, LLMModel: &model.ModelID}

	first, err := service.CreateAgent(ctx, actor, request)
	if err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	// 首个 Agent 应当预留赠送资格。
	var grants int64
	db.Table("agent_first_recharge_grants").Where("app_key=? AND owner_id=?", deleteTestAppKey, "owner_recreate").Count(&grants)
	if grants != 1 {
		t.Fatalf("首个 Agent 应预留赠送资格，实际 %d 行", grants)
	}

	if err := service.DeleteAgent(ctx, actor, first.ID); err != nil {
		t.Fatalf("软删失败: %v", err)
	}

	// 关键断言：删光之后 Agent 数量归零，重新创建不能因为预留记录主键冲突而失败。
	secondName := "second agent"
	request.Name = &secondName
	if _, err := service.CreateAgent(ctx, actor, request); err != nil {
		t.Fatalf("软删全部 Agent 后重新创建失败（预留记录主键冲突回归）: %v", err)
	}

	// 赠送资格不能被重复预留 —— 否则删了重建就能反复白拿首充。
	db.Table("agent_first_recharge_grants").Where("app_key=? AND owner_id=?", deleteTestAppKey, "owner_recreate").Count(&grants)
	if grants != 1 {
		t.Fatalf("赠送资格应保持 1 行，实际 %d 行", grants)
	}
}

func seedDeleteFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	cleanup := func() {
		db.Exec("DELETE FROM inbox_agent_bindings WHERE app_key=?", deleteTestAppKey)
		db.Exec("DELETE FROM bot_agent_bindings WHERE agent_id='agent_del_1'")
		db.Exec("DELETE FROM agent_knowledge WHERE agent_id='agent_del_1'")
		db.Exec("DELETE FROM bots WHERE app_key=?", deleteTestAppKey)
		db.Exec("DELETE FROM agents WHERE app_key=?", deleteTestAppKey)
		db.Exec("DELETE FROM inboxes WHERE app_key=?", deleteTestAppKey)
	}
	cleanup()
	t.Cleanup(cleanup)

	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO agents (id, owner_id, name, type, prompt, status, react_config, memory_config,
			total_invocations, success_count, fail_count, failure_rate, app_key, created_at, updated_at)
			VALUES ('agent_del_1','owner_del','del agent','assistant','p','active','{}'::jsonb,'{}'::jsonb,0,0,0,0,?,now(),now())`,
			[]any{deleteTestAppKey}},
		{`INSERT INTO bots (id, owner_id, invite_code, bot_user_id, bot_name, token, status, app_key, created_at, updated_at)
			VALUES ('bot_del_1','owner_del','inv_del_1','bot_user_del_1','del bot','tok_del_1','active',?,now(),now())`,
			[]any{deleteTestAppKey}},
		{`INSERT INTO bot_agent_bindings (id, bot_id, agent_id, status, created_at, updated_at)
			VALUES ('bab_del_1','bot_del_1','agent_del_1','active',now(),now())`, nil},
		{`INSERT INTO inboxes (inbox_id, channel_type, name, app_key) VALUES ('inbox_del_1','widget','del inbox',?)`,
			[]any{deleteTestAppKey}},
		{`INSERT INTO inbox_agent_bindings (id, app_key, inbox_id, agent_id, bot_id, status, created_at, updated_at)
			VALUES ('iab_del_1',?,'inbox_del_1','agent_del_1','bot_del_1','active',now(),now())`,
			[]any{deleteTestAppKey}},
		// TIPS: 必须真的挂一条知识，否则“卸载”断言在空表上恒真 —— 等于没测。
		{`INSERT INTO agent_knowledge (id, agent_id, knowledge_id, mounted_at)
			VALUES ('ak_del_1','agent_del_1','kb_del_1',now())`, nil},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("准备测试数据失败: %v\nSQL: %s", err, statement.sql)
		}
	}
}
