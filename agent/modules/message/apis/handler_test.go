package apis

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandlerRegistersAllMessageRoutes 校验 Message 域 14 个源路由无遗漏。
func TestHandlerRegistersAllMessageRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/v1")
	NewHandler(nil).RegisterRoutes(group)
	expected := map[string]string{
		"/api/v1/chat/healthz":                           http.MethodGet,
		"/api/v1/chat/completion":                        http.MethodPost,
		"/api/v1/chat/stream":                            http.MethodPost,
		"/api/v1/chat/app/stream":                        http.MethodPost,
		"/api/v1/chat/app/completion":                    http.MethodPost,
		"/api/v1/bot/human-interventions/enter":          http.MethodPost,
		"/api/v1/bot/human-interventions/send-message":   http.MethodPost,
		"/api/v1/bot/human-interventions/exit":           http.MethodPost,
		"/api/v1/bot/human-interventions/status":         http.MethodGet,
		"/api/v1/bot/human-interventions/poll-messages":  http.MethodGet,
		"/api/v1/conversations/:conversation_id/summary": http.MethodGet,
		"/api/v1/owner/agents/:agent_id/conversations":   http.MethodGet,
		"/api/v1/owner/conversations/:conversation_id":   http.MethodGet,
		"/api/v1/owner/agents":                           http.MethodGet,
	}
	actual := map[string]string{}
	for _, route := range engine.Routes() {
		actual[route.Path] = route.Method
	}
	for path, method := range expected {
		if actual[path] != method {
			t.Errorf("路由未正确注册 %s %s，实际 %s", method, path, actual[path])
		}
	}
	if len(actual) != len(expected) {
		t.Fatalf("Message 路由数量应为 %d，实际 %d: %+v", len(expected), len(actual), actual)
	}
}
