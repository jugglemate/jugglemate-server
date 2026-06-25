package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/logs"
	"github.com/juggleim/jugglemate-server/console"
	"github.com/juggleim/jugglemate-server/routers"
)

func main() {
	// init configure
	if err := configures.InitConfigures(); err != nil {
		fmt.Println("Init Configures failed", err)
		return
	}
	//init log
	logs.InitLogs()
	// // init IMSDK provider
	// imsdk.RegisterAppInfoProvider(func(appkey string) (string, string, bool) {
	// 	// For now, use configured default appkey/secret
	// 	// In production, this could read from database
	// 	if appkey == configures.Config.AppKey {
	// 		return configures.Config.AppSecret, configures.Config.ImApiDomain, true
	// 	}
	// 	return "", "", false
	// })

	// // init Validate secure key provider
	// apis.RegisterSecureKeyProvider(func(appkey string) string {
	// 	if appkey == configures.Config.AppKey {
	// 		return configures.Config.AppSecret
	// 	}
	// 	return ""
	// })

	// init mysql
	if err := dbcommons.InitMysql(); err != nil {
		logs.Error("Init Mysql failed.", err)
		return
	}
	// upgrade db
	dbcommons.Upgrade()

	httpServer := gin.Default()
	routers.Route(httpServer, "jmate")
	console.LoadConsoleWeb(httpServer)

	// Serve uploaded static files (avatars, etc.) publicly
	httpServer.Static("/static", "./data")

	msgCallbackGrp := httpServer.Group("/botmsgs")
	routers.RouteMsgCallback(msgCallbackGrp)

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
}
