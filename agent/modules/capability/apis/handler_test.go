package apis

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"testing"
)

// TestRegisterRoutesIncludesSourceCompatibleCapabilityEndpoints 校验 Python 源服务的
// 17 个 Capability 路由在 Go 服务中完整保留。
func TestRegisterRoutesIncludesSourceCompatibleCapabilityEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	expected := []string{
		http.MethodPost + " /api/v1/capabilities/tools",
		http.MethodGet + " /api/v1/capabilities/tools/public",
		http.MethodGet + " /api/v1/capabilities/tools/mine",
		http.MethodGet + " /api/v1/capabilities/tools/:tool_id",
		http.MethodPut + " /api/v1/capabilities/tools/:tool_id",
		http.MethodDelete + " /api/v1/capabilities/tools/:tool_id",
		http.MethodPost + " /api/v1/capabilities/tools/:tool_id/test",
		http.MethodPost + " /api/v1/capabilities/skills",
		http.MethodGet + " /api/v1/capabilities/skills/public",
		http.MethodGet + " /api/v1/capabilities/skills/mine",
		http.MethodGet + " /api/v1/capabilities/skills/:skill_id",
		http.MethodPut + " /api/v1/capabilities/skills/:skill_id",
		http.MethodDelete + " /api/v1/capabilities/skills/:skill_id",
		http.MethodPost + " /api/v1/capabilities/skills/:skill_id/test",
		http.MethodGet + " /api/v1/capabilities/capabilities/public",
		http.MethodGet + " /api/v1/capabilities/capabilities/tools/mine",
		http.MethodGet + " /api/v1/capabilities/capabilities/skills/mine",
	}
	for _, route := range expected {
		if !routes[route] {
			t.Errorf("缺少源服务兼容路由: %s", route)
		}
	}
}

// TestRegisterRoutesKeepsInitialGoCompatibilityEndpoints 校验迁移初期已开放的 Go 路径
// 继续可用，避免已有客户端因源路径补齐而回归。
func TestRegisterRoutesKeepsInitialGoCompatibilityEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		http.MethodGet + " /api/v1/tools/public",
		http.MethodGet + " /api/v1/skills/public",
		http.MethodGet + " /api/v1/capabilities/public",
	} {
		if !routes[route] {
			t.Errorf("缺少 Go 兼容路由: %s", route)
		}
	}
}
