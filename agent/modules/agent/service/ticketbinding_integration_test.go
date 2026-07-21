package service

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/migrations"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const ticketBindTestAppKey = "test_ticket_bind_appkey"

// TestBindTicketAgentAgainstPostgres 验收工单-Agent 绑定的覆盖语义与校验。
//
// TIPS: 重复绑定必须是覆盖而不是新增 —— 否则同一工单会堆积多行，
// ListActiveAgents 的 binded 标记将无法判定当前生效的是哪一个。
func TestBindTicketAgentAgainstPostgres(t *testing.T) {
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
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启测试事务失败: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	if err := tx.Exec("DELETE FROM ticket_agent_bindings WHERE app_key IN (?, ?)", ticketBindTestAppKey, ticketBindTestAppKey+"_other").Error; err != nil {
		t.Fatalf("清理测试绑定失败: %v", err)
	}
	if err := tx.Exec("DELETE FROM tickets WHERE app_key = ?", ticketBindTestAppKey).Error; err != nil {
		t.Fatalf("清理测试工单失败: %v", err)
	}
	if err := tx.Exec("DELETE FROM agents WHERE app_key = ?", ticketBindTestAppKey).Error; err != nil {
		t.Fatalf("清理测试 Agent 失败: %v", err)
	}
	service := &Service{db: tx}

	ticketID := "ticket_" + uuid.NewString()[:8]
	firstAgent := seedActiveAgent(t, tx, ticketBindTestAppKey)
	secondAgent := seedActiveAgent(t, tx, ticketBindTestAppKey)
	seedTicket(t, tx, ticketBindTestAppKey, ticketID)
	// TIPS: 在事务中强制准备历史系统 Agent，确保 active 列表不会再把它跨租户前置；
	// 回滚会恢复测试前状态，不修改已有数据。
	if err := tx.Exec(`INSERT INTO agents (id, app_key, owner_id, name, type, status, created_at, updated_at)
		VALUES ('Juggle_Agent', '', 'system', 'Juggle Agent', 'assistant', 'active', now(), now())
		ON CONFLICT (id) DO UPDATE SET app_key='', owner_id='system', status='active'`).Error; err != nil {
		t.Fatalf("准备历史系统 Agent 失败: %v", err)
	}
	list, err := service.ListActiveAgents(ctx, ticketBindTestAppKey, "", 1, 20)
	if err != nil {
		t.Fatalf("查询 active Agent 失败: %v", err)
	}
	if list.Total != 2 || len(list.Items) != 2 {
		t.Fatalf("active 列表只能返回当前应用的 2 个 Agent，实际 total=%d items=%d", list.Total, len(list.Items))
	}
	for _, item := range list.Items {
		if item.AgentID == "Juggle_Agent" {
			t.Fatal("active 列表不应注入历史系统 Juggle_Agent")
		}
	}

	if _, err := service.BindTicketAgent(ctx, ticketBindTestAppKey, ticketID, firstAgent); err != nil {
		t.Fatalf("首次绑定失败: %v", err)
	}
	bound, err := service.ticketBoundAgentID(ctx, ticketBindTestAppKey, ticketID)
	if err != nil || bound != firstAgent {
		t.Fatalf("首次绑定后应为 %s，实际 %s err=%v", firstAgent, bound, err)
	}

	// 换绑到另一个 Agent，应覆盖而不是新增一行。
	if _, err := service.BindTicketAgent(ctx, ticketBindTestAppKey, ticketID, secondAgent); err != nil {
		t.Fatalf("换绑失败: %v", err)
	}
	bound, err = service.ticketBoundAgentID(ctx, ticketBindTestAppKey, ticketID)
	if err != nil || bound != secondAgent {
		t.Fatalf("换绑后应为 %s，实际 %s err=%v", secondAgent, bound, err)
	}
	var rows int64
	if err := tx.Model(&model.TicketBinding{}).Where("app_key=? AND ticket_id=?", ticketBindTestAppKey, ticketID).Count(&rows).Error; err != nil {
		t.Fatalf("统计绑定行数失败: %v", err)
	}
	if rows != 1 {
		t.Fatalf("同一工单应只有一行绑定，实际 %d 行", rows)
	}

	// 未绑定的工单返回空串而不是报错。
	other, err := service.ticketBoundAgentID(ctx, ticketBindTestAppKey, "ticket_not_bound")
	if err != nil || other != "" {
		t.Fatalf("未绑定工单应返回空串，实际 %q err=%v", other, err)
	}

	if _, err := service.BindTicketAgent(ctx, ticketBindTestAppKey, "ticket_missing", firstAgent); err == nil {
		t.Fatal("绑定不存在的工单应失败")
	}
	if _, err := service.BindTicketAgent(ctx, ticketBindTestAppKey, ticketID, "agent_missing"); err == nil {
		t.Fatal("绑定不存在的 Agent 应失败")
	}

	otherAppKey := ticketBindTestAppKey + "_other"
	if err := tx.Create(&model.TicketBinding{ID: uuid.NewString(), AppKey: otherAppKey, TicketID: ticketID, AgentID: "other_agent"}).Error; err != nil {
		t.Fatalf("准备其他应用绑定失败: %v", err)
	}
	unbound, err := service.UnbindTicketAgent(ctx, ticketBindTestAppKey, ticketID)
	if err != nil {
		t.Fatalf("解绑失败: %v", err)
	}
	if unbound.SessionID != ticketID || unbound.AgentID != secondAgent || unbound.Binded {
		t.Fatalf("解绑响应异常: %+v", unbound)
	}
	bound, err = service.ticketBoundAgentID(ctx, ticketBindTestAppKey, ticketID)
	if err != nil || bound != "" {
		t.Fatalf("解绑后应无绑定，实际 %q err=%v", bound, err)
	}
	if err := tx.Model(&model.TicketBinding{}).Where("app_key=? AND ticket_id=?", otherAppKey, ticketID).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("解绑不应删除其他应用的同名工单绑定: rows=%d err=%v", rows, err)
	}
	// 重复解绑按幂等成功处理，原 Agent ID 为空。
	unbound, err = service.UnbindTicketAgent(ctx, ticketBindTestAppKey, ticketID)
	if err != nil || unbound.AgentID != "" || unbound.Binded {
		t.Fatalf("重复解绑应幂等成功: result=%+v err=%v", unbound, err)
	}
}

func seedActiveAgent(t *testing.T, db *gorm.DB, appKey string) string {
	t.Helper()
	agentID := uuid.NewString()
	err := db.Exec(`INSERT INTO agents (id, app_key, owner_id, name, type, status, created_at, updated_at)
		VALUES (?, ?, 'owner_ticket_bind', ?, 'assistant', 'active', now(), now())`,
		agentID, appKey, "绑定测试 "+agentID[:8]).Error
	if err != nil {
		t.Fatalf("准备 Agent 失败: %v", err)
	}
	return agentID
}

func seedTicket(t *testing.T, db *gorm.DB, appKey, ticketID string) {
	t.Helper()
	err := db.Exec(`INSERT INTO tickets (ticket_id, app_key, source_id, inbox_id, status, created_time, updated_time)
		VALUES (?, ?, 'customer_bind_test', 'inbox_bind_test', 0, now(), now())`, ticketID, appKey).Error
	if err != nil {
		t.Fatalf("准备 Ticket 失败: %v", err)
	}
}
