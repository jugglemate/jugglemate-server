package apis

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesIncludesAllThirteenKnowledgeEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	count := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/knowledge") {
			count++
		}
	}
	if count != 13 {
		t.Fatalf("Knowledge 路由数量应为 13，实际为 %d", count)
	}
}
