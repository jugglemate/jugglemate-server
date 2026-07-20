package apis

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/agent/dto"
)

// TestRegisterRoutesIncludesAllAgentEndpoints 校验 Agent 21 个接口没有遗漏。
func TestRegisterRoutesIncludesAllAgentEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil).RegisterRoutes(engine.Group("/api/v1"))
	routes := engine.Routes()
	if len(routes) != 21 {
		t.Fatalf("Agent 路由数量应为 21，实际为 %d: %+v", len(routes), routes)
	}
	// TIPS: /agents/active 是静态段，必须能与 /agents/:agent_id 共存而不被参数路由吞掉。
	found := false
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/api/v1/agents/active" {
			found = true
		}
	}
	if !found {
		t.Fatal("缺少 GET /api/v1/agents/active 路由")
	}
}

// TestCreateValidationDistinguishesMissingAndInvalid 校验缺失字段交给业务层、显式空值由契约层拒绝。
func TestCreateValidationDistinguishesMissingAndInvalid(t *testing.T) {
	if err := validateCreateRequest(dto.CreateRequest{}); err != nil {
		t.Fatalf("缺失字段应由服务层返回 400，而非契约层 422: %v", err)
	}
	empty := ""
	if err := validateCreateRequest(dto.CreateRequest{Name: &empty}); err == nil {
		t.Fatal("显式空 name 应由契约层拒绝")
	}
}

// TestUpdateValidationRejectsOutOfRangePatch 校验嵌套配置边界在入参层生效。
func TestUpdateValidationRejectsOutOfRangePatch(t *testing.T) {
	maxRounds := 11
	request := dto.UpdateRequest{AgentID: "agent-1", ReactConfig: &dto.ReactConfigPatch{MaxRounds: &maxRounds}}
	if err := validateUpdateRequest(request); err == nil {
		t.Fatal("超范围 maxRounds 应由契约层拒绝")
	}
}
