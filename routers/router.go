package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/apis"
	"github.com/juggleim/jugglemate-server/apis/handlers"
)

func Route(group *gin.RouterGroup) {
	group.POST("/user/login", apis.Login)
	group.POST("/user/register", apis.Register)

	group.POST("/customers/start", apis.StartWebCustom)

	group.GET("/tickets/list", apis.QryTickets)
	group.POST("/tickets/:ticket_id/claim", apis.ClaimTicket)

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
}

func RouteMsgCallback(group *gin.RouterGroup) {
	group.POST("/msgcallback", apis.MsgCallback)
	group.POST("/forward", apis.MsgCallbackForward)
}
