package bootstrap

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/configures"
)

// TestModuleStartAndStopAgainstInfrastructure 在显式配置时验收完整模块启动、依赖读取和关闭。
func TestModuleStartAndStopAgainstInfrastructure(t *testing.T) {
	dsn := os.Getenv("AGENT_TEST_POSTGRES_DSN")
	redisAddress := os.Getenv("AGENT_TEST_REDIS_ADDRESS")
	if dsn == "" || redisAddress == "" {
		t.Skip("未设置 Agent 基础设施集成测试环境")
	}
	workerEnabled := false
	imEnabled := false
	activationCheck := false
	freeDaily := true
	freeTokens := 500000
	cfg := configures.AgentConfig{Enabled: true}
	cfg.Postgres = configures.AgentPostgresConfig{DSN: dsn, MaxIdleConns: 2, MaxOpenConns: 4, ConnMaxLifetimeSeconds: 60}
	cfg.Redis = configures.AgentRedisConfig{Address: redisAddress, DialTimeoutSeconds: 3}
	cfg.Security = configures.AgentSecurityConfig{SecretKey: "acceptance-secret", SecretSalt: "acceptance-salt"}
	cfg.Knowledge = configures.AgentKnowledgeConfig{WorkerEnabled: workerEnabled, VectorDimension: 1536, VectorBatchSize: 32, MaxFileSizeMB: 10, VectorizeStream: "acceptance-vectorize", ConsumerGroup: "acceptance", ConsumerName: "acceptance", TaskTimeoutSeconds: 30, MaxRetries: 1, ObjectStorageRoot: t.TempDir()}
	cfg.Tools = configures.AgentToolsConfig{InnerAPIBaseURL: "https://acceptance.invalid"}
	cfg.Billing = configures.AgentBillingConfig{DirectFreeDailyEnabled: &freeDaily, DirectFreeDailyTokens: &freeTokens, DirectQuotaTimezone: "Asia/Shanghai"}
	cfg.Profile = configures.AgentProfileConfig{FirstAgentRechargeAmount: "0", ActivationPointsCheckEnabled: &activationCheck}
	cfg.IM = configures.AgentIMConfig{Enabled: &imEnabled}
	module := New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := module.Start(ctx); err != nil {
		t.Fatalf("启动 Agent 模块失败: %v", err)
	}
	if _, err := module.DB(); err != nil {
		t.Fatalf("启动后 PostgreSQL 不可用: %v", err)
	}
	if client, err := module.Redis(); err != nil || client.Ping(ctx).Err() != nil {
		t.Fatalf("启动后 Redis 不可用: client=%v err=%v", client, err)
	}
	if _, err := module.Reasoning(); err != nil {
		t.Fatalf("启动后 Reasoning 不可用: %v", err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	module.RegisterNativeRoutes(engine)
	// 145 - 5（移除的 /bot/human-interventions/*）+ 2（新增 sessions bind/unbind）= 142
	if count := len(engine.Routes()); count != 142 {
		t.Fatalf("源兼容路由应为 142 条，实际为 %d", count)
	}
	if err := module.Stop(ctx); err != nil {
		t.Fatalf("关闭 Agent 模块失败: %v", err)
	}
	if _, err := module.DB(); err == nil {
		t.Fatal("关闭后不应继续暴露 PostgreSQL")
	}
}
