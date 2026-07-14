package service

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/juggleim/jugglemate-server/agent/migrations"
	"github.com/juggleim/jugglemate-server/agent/modules/operations/dto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestConfigInitializationAgainstPostgres 验收多实例并发初始化只产生一个 active 配置。
func TestConfigInitializationAgainstPostgres(t *testing.T) {
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
		t.Fatalf("执行最终态迁移失败: %v", err)
	}
	for _, table := range []string{"config_change_logs", "billing_rules", "model_coefficients", "policy_rules"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("清理配置验收表 %s 失败: %v", table, err)
		}
	}
	service := New(db, nil)

	// 简要描述：并发模拟多个服务实例第一次读取空库，advisory lock 应让三类默认配置各只创建一条。
	const workers = 12
	errorsByWorker := make(chan error, workers*3)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := service.GetBillingRule(ctx); err != nil {
				errorsByWorker <- err
			}
			if _, err := service.GetModelCoefficient(ctx); err != nil {
				errorsByWorker <- err
			}
			if _, err := service.GetPolicyRule(ctx); err != nil {
				errorsByWorker <- err
			}
		}()
	}
	group.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		t.Errorf("并发初始化失败: %v", err)
	}
	for _, table := range []string{"billing_rules", "model_coefficients", "policy_rules"} {
		var count int64
		if err := db.Table(table).Where("status = 'active'").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s 应仅有一个 active 版本: count=%d err=%v", table, count, err)
		}
	}

	reason := "PostgreSQL 集成验收"
	updated, err := service.UpdateBillingRule(ctx, AdminContext{AdminID: "acceptance-admin"}, dto.BillingRuleUpdate{Version: 1, TokenUnitPrice: 6000, ToolPricing: dto.ToolPricing{LowRisk: 1, MediumRisk: 3, HighRisk: 6}, MinRechargeAmount: 100, Precision: 5, ValidityYears: 10, Reason: &reason})
	if err != nil || updated.Version != 2 {
		t.Fatalf("配置版本更新失败: result=%+v err=%v", updated, err)
	}
	var changes int64
	if err := db.Table("config_change_logs").Where("config_type = 'billing' AND changed_by = 'acceptance-admin'").Count(&changes).Error; err != nil || changes != 1 {
		t.Fatalf("配置变更日志异常: count=%d err=%v", changes, err)
	}
}
