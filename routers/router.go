package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglechat-server-ai/apis"
)

func Route(group *gin.RouterGroup) {
	group.POST("/user/login", apis.Login)
	group.POST("/user/register", apis.Register)
	group.POST("/aibots/add", apis.CreateAiBot)
	group.POST("/aibots/update", apis.UpdateAiBot)
	group.POST("/aibots/remove", apis.RemoveAiBot)
	group.GET("/aibots/mybots", apis.QryMyAiBots)
	group.POST("/aibots/:unique_name/materials/add", apis.AddAiMaterial)
	group.GET("/aibots/:unique_name/materials", apis.QryAiMaterials)
	group.POST("/aibots/:unique_name/materials/:material_id/remove", apis.RemoveAiMaterial)
	group.POST("/aibots/:unique_name/training", apis.StartTraining)
	group.GET("/aibots/:unique_name/jobs", apis.ListJobs)
	group.GET("/aibots/:unique_name/versions", apis.ListVersions)
	group.POST("/aibots/:unique_name/versions/:version/activate", apis.ActivateVersion)
	group.GET("/aibots/:unique_name/versions/current", apis.GetCurrentVersion)
	group.GET("/aibots/:unique_name/evaluations", apis.ListEvaluations)
}

func RouteMsgCallback(group *gin.RouterGroup) {
	group.POST("/msgcallback", apis.MsgCallback)
}
