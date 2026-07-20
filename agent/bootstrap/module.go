// Package bootstrap 负责 Agent 模块的装配、启动和关闭。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/migrations"
	agentapis "github.com/juggleim/jugglemate-server/agent/modules/agent/apis"
	agentservice "github.com/juggleim/jugglemate-server/agent/modules/agent/service"
	authmodule "github.com/juggleim/jugglemate-server/agent/modules/auth"
	billingapis "github.com/juggleim/jugglemate-server/agent/modules/billing/apis"
	billingservice "github.com/juggleim/jugglemate-server/agent/modules/billing/service"
	capabilityapis "github.com/juggleim/jugglemate-server/agent/modules/capability/apis"
	capabilityservice "github.com/juggleim/jugglemate-server/agent/modules/capability/service"
	knowledgeapis "github.com/juggleim/jugglemate-server/agent/modules/knowledge/apis"
	knowledgerepository "github.com/juggleim/jugglemate-server/agent/modules/knowledge/repository"
	knowledgeservice "github.com/juggleim/jugglemate-server/agent/modules/knowledge/service"
	llmadapter "github.com/juggleim/jugglemate-server/agent/modules/llm/adapter"
	llmapis "github.com/juggleim/jugglemate-server/agent/modules/llm/apis"
	llmservice "github.com/juggleim/jugglemate-server/agent/modules/llm/service"
	messageapis "github.com/juggleim/jugglemate-server/agent/modules/message/apis"
	messageimbot "github.com/juggleim/jugglemate-server/agent/modules/message/imbot"
	messageservice "github.com/juggleim/jugglemate-server/agent/modules/message/service"
	operationsapis "github.com/juggleim/jugglemate-server/agent/modules/operations/apis"
	operationsservice "github.com/juggleim/jugglemate-server/agent/modules/operations/service"
	reasoningservice "github.com/juggleim/jugglemate-server/agent/modules/reasoning/service"
	toolsadapter "github.com/juggleim/jugglemate-server/agent/modules/tools/adapter"
	toolsapis "github.com/juggleim/jugglemate-server/agent/modules/tools/apis"
	toolsservice "github.com/juggleim/jugglemate-server/agent/modules/tools/service"
	"github.com/juggleim/jugglemate-server/agent/shared/database"
	"github.com/juggleim/jugglemate-server/agent/shared/redisclient"
	sharedsecurity "github.com/juggleim/jugglemate-server/agent/shared/security"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/services"
	"github.com/juggleim/jugglemate-server/storages"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Module 表示与客服/工单域共享 PostgreSQL 的 Go Agent 平台模块。
type Module struct {
	cfg configures.AgentConfig

	mu              sync.RWMutex
	started         bool
	db              *gorm.DB
	redis           *redis.Client
	auth            *authmodule.Service
	llm             *llmapis.Handler
	knowledge       *knowledgeapis.Handler
	knowledgeWorker *knowledgeservice.Worker
	capability      *capabilityapis.Handler
	tools           *toolsapis.Handler
	billing         *billingapis.Handler
	agent           *agentapis.Handler
	agentService    *agentservice.Service
	reasoning       *reasoningservice.Service
	message         *messageapis.Handler
	messageService  *messageservice.Service
	operations      *operationsapis.Handler
	botConnections  *messageimbot.Manager
}

// SetUnbindInboxAgent 注入 Inbox 解绑实现，供删除 Agent 时清理 Inbox 关联与 Ticket 群 Bot。
//
// TIPS: 由 main.go 在 Start 之后装配。移出 Ticket 群属于客服域能力，Agent 平台模块不直接
// 依赖它，避免两个域互相 import。
func (module *Module) SetUnbindInboxAgent(fn agentservice.UnbindInboxAgentFunc) {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.agentService != nil {
		module.agentService.SetUnbindInboxAgent(fn)
	}
}

// SetHumanHandoff 注入转人工的 Ticket 群成员切换实现。
//
// TIPS: 由 main.go 在 Start 之后装配，原因同 SetUnbindInboxAgent —— 拉坐席进群、移出 Bot
// 都是客服域能力，Agent 平台模块不直接依赖 services 包。
func (module *Module) SetHumanHandoff(fn messageservice.HumanHandoffFunc) {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.messageService != nil {
		module.messageService.SetHumanHandoff(fn)
	}
}

// New 创建尚未启动的 Agent 模块。
func New(cfg configures.AgentConfig) *Module {
	return &Module{cfg: cfg}
}

// Enabled 返回 Agent Go 模块是否启用。
func (module *Module) Enabled() bool {
	return module != nil && module.cfg.Enabled
}

// Start 连接真实基础设施、执行最终态迁移并写入系统 seed。
//
// 简要描述：启动过程遵循 PostgreSQL -> migration/seed -> Redis 的顺序；任一步失败
// 都会关闭已经打开的资源，避免主服务携带半初始化的 Agent 模块继续运行。
func (module *Module) Start(ctx context.Context) error {
	if module == nil || !module.cfg.Enabled {
		return nil
	}
	module.mu.Lock()
	defer module.mu.Unlock()
	if module.started {
		return nil
	}
	if module.cfg.Knowledge.VectorDimension != knowledgeservice.StorageVectorDimension {
		return fmt.Errorf("Agent 知识向量维度配置必须与最终态数据库一致: configured=%d required=%d", module.cfg.Knowledge.VectorDimension, knowledgeservice.StorageVectorDimension)
	}

	db, err := database.Open(ctx, module.cfg.Postgres)
	if err != nil {
		return err
	}
	if err := migrations.Apply(ctx, db); err != nil {
		_ = database.Close(db)
		return fmt.Errorf("初始化 Agent 数据库结构失败: %w", err)
	}
	if err := migrations.Seed(ctx, db, migrations.SeedConfig{InnerAPIBaseURL: module.cfg.Tools.InnerAPIBaseURL}); err != nil {
		_ = database.Close(db)
		return fmt.Errorf("初始化 Agent 系统数据失败: %w", err)
	}
	redisClient, err := redisclient.Open(ctx, module.cfg.Redis)
	if err != nil {
		_ = database.Close(db)
		return err
	}
	encryption, err := sharedsecurity.NewEncryptionService(module.cfg.Security.SecretKey, module.cfg.Security.SecretSalt)
	if err != nil {
		_ = redisclient.Close(redisClient)
		_ = database.Close(db)
		return fmt.Errorf("初始化 Agent 加密服务失败: %w", err)
	}
	registryService := llmservice.NewRegistryService(db, encryption)
	callService := llmservice.NewCallService(db, encryption, llmadapter.NewHTTPGateway())
	billingService := billingservice.New(db)
	freeDailyEnabled := true
	if module.cfg.Billing.DirectFreeDailyEnabled != nil {
		freeDailyEnabled = *module.cfg.Billing.DirectFreeDailyEnabled
	}
	freeDailyTokens := 500000
	if module.cfg.Billing.DirectFreeDailyTokens != nil {
		freeDailyTokens = *module.cfg.Billing.DirectFreeDailyTokens
	}
	if err := billingService.ConfigureQuota(billingservice.QuotaConfig{FreeDailyEnabled: freeDailyEnabled, FreeDailyTokens: freeDailyTokens, Timezone: module.cfg.Billing.DirectQuotaTimezone}); err != nil {
		_ = redisclient.Close(redisClient)
		_ = database.Close(db)
		return fmt.Errorf("初始化 Billing 配额时区失败: %w", err)
	}
	firstAgentRecharge, err := decimal.NewFromString(module.cfg.Profile.FirstAgentRechargeAmount)
	if err != nil || firstAgentRecharge.IsNegative() {
		_ = redisclient.Close(redisClient)
		_ = database.Close(db)
		return fmt.Errorf("初始化 Agent 首次赠送金额失败: 配置值 %q 非法", module.cfg.Profile.FirstAgentRechargeAmount)
	}
	activationPointsCheck := true
	if module.cfg.Profile.ActivationPointsCheckEnabled != nil {
		activationPointsCheck = *module.cfg.Profile.ActivationPointsCheckEnabled
	}
	botConnections := messageimbot.NewManager(module.cfg.IM)
	if err := botConnections.Start(ctx, db); err != nil {
		botConnections.Stop()
		_ = redisclient.Close(redisClient)
		_ = database.Close(db)
		return fmt.Errorf("启动 IM Bot 连接管理器失败: %w", err)
	}
	registerClient := messageimbot.NewRegisterClient(configures.Config.ImApiDomain, module.cfg.IM.ServerAPIInsecure, func(_ context.Context, appKey string) (string, error) {
		app, err := storages.NewAppInfoStorage().FindByAppkey(appKey)
		if err != nil {
			return "", err
		}
		if app == nil || app.AppSecret == "" {
			return "", fmt.Errorf("IM 应用不存在或凭证为空: %s", appKey)
		}
		return app.AppSecret, nil
	})
	agentService := agentservice.New(db, billingService, agentservice.Config{FirstAgentRechargeAmount: firstAgentRecharge, ActivationPointsCheckEnabled: activationPointsCheck}, registerClient, botConnections)
	knowledgeRepository := knowledgerepository.New(db)
	knowledgeService := knowledgeservice.New(knowledgeRepository, redisClient, callService, module.cfg.Knowledge)
	knowledgeWorker := knowledgeservice.NewWorker(knowledgeRepository, redisClient, callService, billingService, module.cfg.Knowledge)
	if err := knowledgeWorker.Start(ctx); err != nil {
		botConnections.Stop()
		_ = redisclient.Close(redisClient)
		_ = database.Close(db)
		return fmt.Errorf("启动 Knowledge 向量化 Worker 失败: %w", err)
	}

	module.agentService = agentService
	module.db = db
	module.redis = redisClient
	module.auth = authmodule.NewService(module.cfg.Auth)
	module.llm = llmapis.NewHandler(registryService, callService)
	module.knowledge = knowledgeapis.NewHandler(knowledgeService)
	module.knowledgeWorker = knowledgeWorker
	module.capability = capabilityapis.NewHandler(capabilityservice.New(db))
	toolsService := toolsservice.New(db, encryption, toolsadapter.NewHTTPExecutor())
	module.tools = toolsapis.NewHandler(toolsService)
	module.billing = billingapis.NewHandler(billingService)
	module.agent = agentapis.NewHandler(agentService)
	module.reasoning = reasoningservice.New(db, callService, knowledgeService, toolsService, billingService)
	messageService := messageservice.New(db, module.reasoning, billingService, botConnections)
	botConnections.SetInboundHandler(messageService.HandleInbound)
	// TIPS: 让主工程的 webhook 诊断日志能查到 Bot 长连接状态，避免 services 反向依赖 Agent 模块。
	services.SetBotConnectionStateProbe(botConnections.ConnectionState)
	module.message = messageapis.NewHandler(messageService)
	module.messageService = messageService
	module.operations = operationsapis.NewHandler(operationsservice.New(db, agentService))
	module.botConnections = botConnections
	module.started = true
	return nil
}

// Stop 停止 Agent 后台能力并关闭基础设施连接。
func (module *Module) Stop(ctx context.Context) error {
	if module == nil {
		return nil
	}
	module.mu.Lock()
	defer module.mu.Unlock()
	if !module.started {
		return nil
	}
	module.botConnections.Stop()
	err := errors.Join(module.knowledgeWorker.Stop(ctx), redisclient.Close(module.redis), database.Close(module.db))
	module.redis = nil
	module.db = nil
	module.auth = nil
	module.llm = nil
	module.knowledge = nil
	module.knowledgeWorker = nil
	module.capability = nil
	module.tools = nil
	module.billing = nil
	module.agent = nil
	module.reasoning = nil
	module.message = nil
	module.operations = nil
	module.botConnections = nil
	module.started = false
	return err
}

// Reasoning 返回已启动模块的进程内推理服务，供 Message 与兼容门面复用。
func (module *Module) Reasoning() (*reasoningservice.Service, error) {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if !module.started || module.reasoning == nil {
		return nil, fmt.Errorf("Agent 推理模块尚未启动")
	}
	return module.reasoning, nil
}

// DB 返回已启动模块的 Agent PostgreSQL 连接。
func (module *Module) DB() (*gorm.DB, error) {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if !module.started || module.db == nil {
		return nil, fmt.Errorf("Agent 模块尚未启动")
	}
	return module.db, nil
}

// Redis 返回已启动模块的 Agent Redis 客户端。
func (module *Module) Redis() (*redis.Client, error) {
	module.mu.RLock()
	defer module.mu.RUnlock()
	if !module.started || module.redis == nil {
		return nil, fmt.Errorf("Agent 模块尚未启动")
	}
	return module.redis, nil
}

// RegisterNativeRoutes 注册与源服务兼容的 `/api/v1` 路由。
func (module *Module) RegisterNativeRoutes(engine *gin.Engine) {
	if !module.Enabled() {
		return
	}
	group := engine.Group("/api/v1")
	module.registerHealthRoutes(group)
	module.billing.RegisterWebhookRoutes(group)
	group.Use(module.auth.NativeMiddleware())
	module.auth.RegisterRoutes(group)
	module.llm.RegisterRoutes(group)
	module.knowledge.RegisterRoutes(group)
	module.capability.RegisterRoutes(group)
	module.tools.RegisterRoutes(group)
	module.billing.RegisterRoutes(group)
	module.agent.RegisterRoutes(group)
	module.message.RegisterRoutes(group)
	module.operations.RegisterRoutes(group)
}

// RegisterConsoleRoutes 注册当前控制台使用的 `/jmate/agentapi` 本地路由。
func (module *Module) RegisterConsoleRoutes(group *gin.RouterGroup) {
	if !module.Enabled() {
		return
	}
	module.registerHealthRoutes(group)
	group.Use(module.auth.ConsoleMiddleware())
	module.auth.RegisterRoutes(group)
	module.llm.RegisterRoutes(group)
	module.knowledge.RegisterRoutes(group)
	module.capability.RegisterRoutes(group)
	module.tools.RegisterRoutes(group)
	module.billing.RegisterRoutes(group)
	module.agent.RegisterRoutes(group)
	module.message.RegisterRoutes(group)
	module.operations.RegisterRoutes(group)
}

// registerHealthRoutes 注册不经过统一信封的健康检查接口。
func (module *Module) registerHealthRoutes(group *gin.RouterGroup) {
	group.GET("/health", module.health)
	group.GET("/health/ready", module.ready)
	group.GET("/health/live", module.live)
}

// health 检查 Agent PostgreSQL 与 Redis 状态。
func (module *Module) health(ctx *gin.Context) {
	type component struct {
		Status string `json:"status"`
		Type   string `json:"type,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	result := struct {
		Status     string               `json:"status"`
		Components map[string]component `json:"components"`
	}{Status: "healthy", Components: map[string]component{}}

	checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
	defer cancel()
	db, dbErr := module.DB()
	if dbErr == nil {
		dbErr = db.WithContext(checkCtx).Exec("SELECT 1").Error
	}
	if dbErr != nil {
		result.Status = "unhealthy"
		result.Components["database"] = component{Status: "unhealthy", Error: dbErr.Error()}
	} else {
		result.Components["database"] = component{Status: "healthy", Type: "postgresql"}
	}

	redisClient, redisErr := module.Redis()
	if redisErr == nil {
		redisErr = redisClient.Ping(checkCtx).Err()
	}
	if redisErr != nil {
		result.Status = "unhealthy"
		result.Components["redis"] = component{Status: "unhealthy", Error: redisErr.Error()}
	} else {
		result.Components["redis"] = component{Status: "healthy"}
	}
	ctx.JSON(http.StatusOK, result)
}

// ready 检查 Agent 模块是否可以接收请求。
func (module *Module) ready(ctx *gin.Context) {
	db, err := module.DB()
	if err == nil {
		checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
		defer cancel()
		err = db.WithContext(checkCtx).Exec("SELECT 1").Error
	}
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "not_ready", "error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
}

// live 检查 Go 进程中的 Agent 模块是否存活。
func (module *Module) live(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "alive"})
}
