package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
)

// 覆盖三种参数解析失败路径，不需要起 service。

func newStatsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/jmate/console")
	g.GET("/stats/overview", ConsoleStatsOverview)
	g.GET("/stats/agents", ConsoleStatsAgents)
	g.GET("/stats/customers", ConsoleStatsCustomers)
	return r
}

// 喂一个含 appkey 的 ctx 进入 service 前置校验
func injectAppKey(req *http.Request) *http.Request {
	req.Header.Set("appkey", "fake_app_for_unit")
	// 强制带上 Gin ctx 的鉴权变量（虽然这里 service 只看 appkey）
	return req
}

func TestConsoleStatsOverviewOk(t *testing.T) {
	// service 走到 SQL 之前会因为某个上层 fail，但我们只验证 router/route 与 200/500 形状。
	// 这里只测绑定是否成功（即路由注册无误）。
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/overview", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// 我们期望 service 不会 panic；返回 500 或 200 都正常，状态码不强制。
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected code: %d body=%s", w.Code, w.Body.String())
	}
}

func TestConsoleStatsAgentsBadIncludeAdmin(t *testing.T) {
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/agents?include_admin=notbool", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with err code body, got %d", w.Code)
	}
	// 错误码 17005 应在 body 里
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestConsoleStatsAgentsBadPage(t *testing.T) {
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/agents?page=0", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestConsoleStatsAgentsBadPageSize(t *testing.T) {
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/agents?pageSize=abc", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestConsoleStatsAgentsClampPageSize(t *testing.T) {
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/agents?pageSize=10000", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// pageSize 应被夹到 100，service 后续会走 SQL —— 不验证 SQL，只验证不是 17005
	if strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("pageSize=10000 should be clamped to 100, not 17005; body=%s", w.Body.String())
	}
}

func TestConsoleStatsCustomersBadPage(t *testing.T) {
	r := newStatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/console/stats/customers?page=-1", nil)
	injectAppKey(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

// sanity guard
var _ = ctxs.CtxKey_AppKey
