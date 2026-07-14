package routers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/apis"
	"github.com/juggleim/jugglemate-server/apis/handlers"
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

	group.POST("/aibots/add", apis.CreateAiBot)
	group.POST("/aibots/update", apis.UpdateAiBot)
	group.POST("/aibots/remove", apis.RemoveAiBot)
	group.GET("/aibots/mybots", apis.QryMyAiBots)
	group.POST("/aibots/:unique_name/materials/add", apis.AddAiMaterial)
	group.POST("/aibots/:unique_name/materials/upload", apis.UploadAiMaterial)
	group.POST("/aibots/avatar/upload", apis.UploadAvatar)
	group.GET("/aibots/:unique_name/materials", apis.QryAiMaterials)
	group.POST("/aibots/:unique_name/materials/:material_id/remove", apis.RemoveAiMaterial)
	group.POST("/aibots/:unique_name/sync", apis.SyncTwin)
	group.POST("/aibots/:unique_name/materials/:material_id/sync", apis.SyncMaterial)
	group.POST("/aibots/:unique_name/training", apis.StartTraining)
	group.GET("/aibots/:unique_name/jobs", apis.ListJobs)
	group.GET("/aibots/:unique_name/jobs/:job_id", apis.QueryJobStatus)
	group.POST("/aibots/jobs/batch_status", apis.BatchQueryJobStatus)
	group.GET("/aibots/:unique_name/versions", apis.ListVersions)
	group.POST("/aibots/:unique_name/versions/:version/activate", apis.ActivateVersion)
	group.GET("/aibots/:unique_name/versions/current", apis.GetCurrentVersion)
	group.GET("/aibots/:unique_name/evaluations", apis.ListEvaluations)
	// Session & Feedback
	group.POST("/aibots/sessions", handlers.CreateSession)
	group.GET("/aibots/sessions/lookup", handlers.LookupSession)
	group.GET("/aibots/sessions/:session_id", handlers.GetSession)
	group.POST("/aibots/sessions/:session_id/agent", handlers.UpdateSessionAgent)
	group.POST("/aibots/sessions/:session_id/takeover", handlers.TakeoverSession)
	group.POST("/aibots/sessions/:session_id/chat", handlers.SessionChat)
	group.POST("/aibots/sessions/:session_id/feedback", handlers.SubmitFeedback)
	group.GET("/aibots/:unique_name/sessions", handlers.ListAgentSessions)
	group.GET("/aibots/:unique_name/feedbacks", handlers.ListAgentFeedbacks)

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

// RouteMsgCallback 注册 IM 消息回调与节点转发入口。
func RouteMsgCallback(group *gin.RouterGroup) {
	group.POST("/msgcallback", apis.MsgCallback)
	group.POST("/forward", apis.MsgCallbackForward)
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
	group.GET("/inboxes/:inbox_id/members", consoleApis.QryInboxMembers)
	group.PUT("/inboxes/:inbox_id/members", consoleApis.ReplaceInboxMembers)
}
