package apis

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesIncludesAllSixteenLLMEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil, nil).RegisterRoutes(engine.Group("/api/v1"))

	count := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/admin/llm/") || strings.HasPrefix(route.Path, "/api/v1/llm/call") {
			count++
		}
	}
	if count != 16 {
		t.Fatalf("LLM 路由数量应为 16，实际为 %d: %#v", count, engine.Routes())
	}
}
