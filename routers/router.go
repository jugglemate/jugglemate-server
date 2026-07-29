package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/apis"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	consoleApis "github.com/juggleim/jugglemate-server/console/apis"
)

// AgentRouter 定义 Go Agent 模块向当前控制台注册路由所需的最小接口。
type AgentRouter interface {
	// Enabled 返回 Go Agent 模块是否启用。
	Enabled() bool
	// RegisterConsoleRoutes 注册 `/jmate/agentapi` 下的本地接口。
	RegisterConsoleRoutes(group *gin.RouterGroup)
}

// Route 注册当前服务的全部 HTTP 路由。
func Route(eng *gin.Engine, prefix string, agentRouters ...AgentRouter) {
	eng.Use(corsHandler())

	publicGroup := eng.Group("/" + prefix)
	publicGroup.POST("/webhooks/telegram/:inbox_id", apis.TelegramWebhook)
	publicGroup.POST("/webhooks/juggleim/:inbox_id", apis.JuggleIMWebhook)
	publicGroup.POST("/webhooks/message", apis.WebhookMsgs)

	group := eng.Group("/" + prefix)
	group.Use(apis.Validate)

	group.POST("/user/login", apis.Login)
	group.POST("/user/register", apis.Register)

	group.POST("/customers/start", apis.StartWebCustom)
	group.GET("/customers/info", apis.QryCustomerInfo)
	group.GET("/customers/tickets", apis.QryCustomerTickets)
	group.GET("/tickets/inboxmembers/list", apis.QryTicketInboxMembers)

	group.GET("/tickets/list", apis.QryTickets)
	group.POST("/tickets/:ticket_id/claim", apis.ClaimTicket)
	group.POST("/tickets/:ticket_id/transfer", apis.TransferTicket)
	group.GET("/tickets/:ticket_id/events", apis.QryTicketEvents)

	// 联系人方向坐席目录：登录即可调用，不限 admin。
	group.GET("/seats/list", apis.SeatsList)

	RouteConsole(group.Group("/console"))

	// 简要描述：Agent 控制台只允许进入本进程 Go 模块；模块配置错误或未启用时返回
	// 明确不可用错误，禁止再次回退已下线的 Python agent-server。
	if len(agentRouters) > 0 && agentRouters[0] != nil && agentRouters[0].Enabled() {
		agentRouters[0].RegisterConsoleRoutes(group.Group("/agentapi"))
	} else {
		group.Any("/agentapi/*proxyPath", func(ctx *gin.Context) {
			responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		})
	}
}

func corsHandler() gin.HandlerFunc {
	return func(context *gin.Context) {
		method := context.Request.Method
		context.Writer.Header().Add("Access-Control-Allow-Origin", "*")
		context.Writer.Header().Add("Access-Control-Allow-Headers", "*")
		context.Writer.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PATCH, PUT")
		context.Writer.Header().Add("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		context.Writer.Header().Add("Access-Control-Allow-Credentials", "true")

		if method == "OPTIONS" {
			context.AbortWithStatus(http.StatusNoContent)
		}
		context.Next()
	}
}

// RouteConsole 注册原有 JMate 管理控制台接口。
func RouteConsole(group *gin.RouterGroup) {
	group.Use(consoleApis.Validate)

	group.GET("/users", consoleApis.QryUsers)
	group.POST("/users", consoleApis.CreateUser)
	group.PUT("/users/:user_id", consoleApis.UpdateUser)
	group.DELETE("/users/:user_id", consoleApis.DeleteUser)

	group.GET("/inboxes", consoleApis.QryInboxes)
	group.POST("/inboxes/telegram", consoleApis.CreateTelegramInbox)
	group.POST("/inboxes/juggleim", consoleApis.CreateJuggleIMInbox)
	group.POST("/inboxes/widget", consoleApis.CreateWidgetInbox)
	group.GET("/inboxes/:inbox_id", consoleApis.QryInbox)
	group.PUT("/inboxes/:inbox_id", consoleApis.UpdateInbox)
	group.DELETE("/inboxes/:inbox_id", consoleApis.DeleteInbox)
	group.GET("/inboxes/:inbox_id/members", consoleApis.QryInboxMembers)
	group.PUT("/inboxes/:inbox_id/members", consoleApis.ReplaceInboxMembers)
	group.GET("/inboxes/:inbox_id/agent", consoleApis.QryInboxAgent)
	group.PUT("/inboxes/:inbox_id/agent", consoleApis.BindInboxAgent)
	group.DELETE("/inboxes/:inbox_id/agent", consoleApis.UnbindInboxAgent)

	// 客服数据统计（详见 docs/console-stats-api.md）。
	group.GET("/stats/overview", apis.ConsoleStatsOverview)
	group.GET("/stats/agents", apis.ConsoleStatsAgents)
	group.GET("/stats/customers", apis.ConsoleStatsCustomers)
}
