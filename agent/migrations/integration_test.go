package migrations

import (
	"context"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestApplyAndSeedAgainstPostgres 在显式提供 DSN 时验收真实 PostgreSQL 迁移、Seed 与幂等重跑。
func TestApplyAndSeedAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("未设置 AGENT_TEST_POSTGRES_DSN")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接验收 PostgreSQL 失败: %v", err)
	}
	ctx := context.Background()
	if err := Apply(ctx, db); err != nil {
		t.Fatalf("执行最终态迁移失败: %v", err)
	}
	if err := Seed(ctx, db, SeedConfig{InnerAPIBaseURL: "https://acceptance.invalid"}); err != nil {
		t.Fatalf("执行系统 Seed 失败: %v", err)
	}
	if err := Apply(ctx, db); err != nil {
		t.Fatalf("幂等重跑迁移失败: %v", err)
	}
	for _, table := range []string{"agents", "knowledge_vectors", "credit_wallets", "twin_compat_mappings", "twin_compat_evaluations"} {
		var exists bool
		if err := db.Raw("SELECT to_regclass(?) IS NOT NULL", "public."+table).Scan(&exists).Error; err != nil || !exists {
			t.Fatalf("迁移后缺少表 %s: exists=%v err=%v", table, exists, err)
		}
	}
	var agents, tools int64
	if err := db.Table("agents").Where("id = 'Juggle_Agent'").Count(&agents).Error; err != nil || agents != 1 {
		t.Fatalf("系统 Agent Seed 异常: count=%d err=%v", agents, err)
	}
	if err := db.Table("tool_definitions").Where("id = 'td_system_jg_payment_history'").Count(&tools).Error; err != nil || tools != 1 {
		t.Fatalf("系统 Tool Seed 异常: count=%d err=%v", tools, err)
	}
}
