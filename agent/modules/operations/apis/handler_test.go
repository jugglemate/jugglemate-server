package apis

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandlerRegistersAllOperationsRoutes 校验 Operations 域 17 个源路由无遗漏。
func TestHandlerRegistersAllOperationsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	expected := map[string]string{
		"/api/v1/admin/stats/platform":                       http.MethodGet,
		"/api/v1/admin/owners":                               http.MethodGet,
		"/api/v1/admin/owners/:owner_id":                     http.MethodGet,
		"/api/v1/admin/agents/console-list":                  http.MethodGet,
		"/api/v1/admin/agents":                               http.MethodGet,
		"/api/v1/admin/agents/:agent_id":                     http.MethodGet,
		"/api/v1/admin/agents/:agent_id/pause":               http.MethodPost,
		"/api/v1/admin/agents/:agent_id/resume":              http.MethodPost,
		"/api/v1/admin/billing/rules":                        http.MethodGet,
		"/api/v1/admin/billing/tool-pricing":                 http.MethodPut,
		"/api/v1/admin/models/coefficients":                  http.MethodGet,
		"/api/v1/admin/policy/rules":                         http.MethodGet,
		"/api/v1/admin/audit/conversations":                  http.MethodGet,
		"/api/v1/admin/audit/conversations/:conversation_id": http.MethodGet,
	}
	// 同一路径不同 Method 需要使用组合键，避免 PUT 覆盖 GET。
	actual := map[string]bool{}
	for _, route := range engine.Routes() {
		actual[route.Method+" "+route.Path] = true
	}
	for path, method := range expected {
		if !actual[method+" "+path] {
			t.Errorf("路由未正确注册 %s %s", method, path)
		}
	}
	for _, route := range []string{
		http.MethodPut + " /api/v1/admin/billing/rules",
		http.MethodPut + " /api/v1/admin/models/coefficients",
		http.MethodPut + " /api/v1/admin/policy/rules",
	} {
		if !actual[route] {
			t.Errorf("路由未正确注册 %s", route)
		}
	}
	if len(actual) != 17 {
		t.Fatalf("Operations 路由数量应为 17，实际 %d: %+v", len(actual), actual)
	}
}
