package apis

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRegisterRoutesIncludesAllTwentyNineToolsV2Endpoints 校验迁移接口没有遗漏。
func TestRegisterRoutesIncludesAllTwentyNineToolsV2Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	count := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/tool-") || strings.Contains(route.Path, "/tool-bindings") || strings.HasPrefix(route.Path, "/api/v1/internal/v1/tool-execution") {
			count++
		}
	}
	if count != 29 {
		t.Fatalf("Tools v2 路由数量应为 29，实际为 %d: %#v", count, engine.Routes())
	}
}
