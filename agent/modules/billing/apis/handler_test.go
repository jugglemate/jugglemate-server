package apis

import (
	"github.com/gin-gonic/gin"
	"strings"
	"testing"
)

// TestRegisterRoutesIncludesAllThirteenBillingEndpoints 校验 Billing 路由无遗漏。
func TestRegisterRoutesIncludesAllThirteenBillingEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(nil)
	group := engine.Group("/api/v1")
	handler.RegisterWebhookRoutes(group)
	handler.RegisterRoutes(group)
	count := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/billing") || strings.HasPrefix(route.Path, "/api/v1/admin/billing") || strings.HasPrefix(route.Path, "/api/v1/webhook") {
			count++
		}
	}
	if count != 13 {
		t.Fatalf("Billing 路由数量应为 13，实际为 %d: %#v", count, engine.Routes())
	}
}
