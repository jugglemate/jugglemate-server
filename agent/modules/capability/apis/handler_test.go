package apis

import (
	"github.com/gin-gonic/gin"
	"strings"
	"testing"
)

func TestRegisterRoutesIncludesAllSeventeenCapabilityEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	count := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/tools") || strings.HasPrefix(route.Path, "/api/v1/skills") || strings.HasPrefix(route.Path, "/api/v1/capabilities") {
			count++
		}
	}
	if count != 17 {
		t.Fatalf("Capability 路由数量应为 17，实际为 %d", count)
	}
}
