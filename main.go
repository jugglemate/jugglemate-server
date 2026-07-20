package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	agentbootstrap "github.com/juggleim/jugglemate-server/agent/bootstrap"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"github.com/juggleim/jugglemate-server/console"
	"github.com/juggleim/jugglemate-server/routers"
	"github.com/juggleim/jugglemate-server/services"
)

func main() {
	// init configure
	if err := configures.InitConfigures(); err != nil {
		fmt.Println("Init Configures failed", err)
		return
	}
	//init log
	logs.InitLogs()
	// 初始化统一 PostgreSQL 与 Go Agent 平台。
	agentModule := agentbootstrap.New(configures.Config.Agent)
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := agentModule.Start(startupCtx); err != nil {
		startupCancel()
		logs.Error("Init Agent module failed.", err)
		return
	}
	startupCancel()
	postgres, err := agentModule.DB()
	if err != nil {
		logs.Error("Get shared PostgreSQL failed.", err)
		return
	}
	dbcommons.UsePostgres(postgres)
	// TIPS: 把客服域的 Inbox 解绑能力注入 Agent 平台，供删除 Agent 时清理关联的 Inbox 与
	// Ticket 群里的 Bot。必须放在 UsePostgres 之后 —— 解绑实现依赖共享的 PostgreSQL 连接。
	agentModule.SetUnbindInboxAgent(services.UnbindInboxAgent)
	// TIPS: 转人工时需要把 Inbox 坐席拉进 Ticket 群并移出 Agent Bot，同样属于客服域能力，
	// 与上面一样用注入方式装配，同样依赖已就绪的 PostgreSQL。
	agentModule.SetHumanHandoff(services.SwitchTicketToHuman)

	httpServer := gin.Default()
	agentModule.RegisterNativeRoutes(httpServer)
	routers.Route(httpServer, "jmate", agentModule)
	console.LoadConsoleWeb(httpServer)

	// Serve uploaded static files (avatars, etc.) publicly
	httpServer.Static("/static", "./data")

	go httpServer.Run(fmt.Sprintf(":%d", configures.Config.Port))

	closeChan := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigChan
		signal.Stop(sigChan)
		close(closeChan)
	}()

	<-closeChan
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := agentModule.Stop(shutdownCtx); err != nil {
		logs.Error("Stop Agent module failed.", err)
	}
}
