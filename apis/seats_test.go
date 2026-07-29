package apis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 仅覆盖 route 注册 + 参数解析失败路径；SQL 验证靠冒烟测试。

func newSeatsRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/jmate")
	g.GET("/seats/list", SeatsList)
	return r
}

func TestSeatsListBadRole(t *testing.T) {
	r := newSeatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/seats/list?role=root", nil)
	req.Header.Set("appkey", "fake_app")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestSeatsListBadStatus(t *testing.T) {
	r := newSeatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/seats/list?status=9", nil)
	req.Header.Set("appkey", "fake_app")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestSeatsListBadPage(t *testing.T) {
	r := newSeatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/seats/list?page=0", nil)
	req.Header.Set("appkey", "fake_app")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestSeatsListBadPageSize(t *testing.T) {
	r := newSeatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/seats/list?pageSize=abc", nil)
	req.Header.Set("appkey", "fake_app")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("expected 17005 in body, got %s", w.Body.String())
	}
}

func TestSeatsListClampPageSize(t *testing.T) {
	// pageSize=10000 应被夹到 100，service 走到 ctx/appkey 检查前已经把 limit 缩好
	r := newSeatsRouter()
	req := httptest.NewRequest(http.MethodGet, "/jmate/seats/list?pageSize=10000", nil)
	req.Header.Set("appkey", "fake_app")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), "17005") {
		t.Fatalf("pageSize=10000 should be clamped, body=%s", w.Body.String())
	}
}
