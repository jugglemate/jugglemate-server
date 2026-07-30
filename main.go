package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/imbot-sdk-go/imbotclients/pbdefines/pbobjs"
	agentbootstrap "github.com/juggleim/jugglemate-server/agent/bootstrap"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"github.com/juggleim/jugglemate-server/console"
	"github.com/juggleim/jugglemate-server/routers"
	"github.com/juggleim/jugglemate-server/services"
)

const (
	csatInvitationMsgType = "jgm:csat"
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
	// TIPS: 持久化「工单转人工」事实与事件流水与上一行同源；解耦注入避免 Agent 平台模块
	// 直接依赖客服域 services 包。
	agentModule.SetMarkHumanTakeover(services.MarkHumanTakeover)

	// TIPS: 自动关闭工单后发评价邀请卡（jgm:csat）。需要 IM Bot 长连接，但 services
	// 不持有 IM SDK 直接引用，所以通过 services.SetCsatIMSender 注入一个桥接闭包。
	botConn, botErr := agentModule.BotConnections()
	if botErr != nil {
		logs.Error("Get agent bot connections failed.", botErr)
		os.Exit(1)
	}
	services.SetCsatIMSender(func(ctx context.Context, appKey, botUserID, ticketId string, msgType string, payload interface{}) error {
		// channelType 固定用群消息（telegram / widget / juggleim 都对应 group）。
		_, err := botConn.SendCustomMessage(ctx, appKey, botUserID, ticketId,
			pbobjs.ChannelType_Group, msgType, payload)
		return err
	})
	// TIPS: 启动后台 ticker，按 last_user_msg_at 阈值关闭 + 发评价邀请卡。仅在
	// agent 模块已 enabled（已启动）时跑。
	go services.StartAutoCloseTicker(context.Background(), services.ConfigFromAppConfig(configures.Config.Agent))

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
