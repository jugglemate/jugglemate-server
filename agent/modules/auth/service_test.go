package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
	"github.com/juggleim/jugglemate-server/commons/configures"
)

func TestNativeMiddlewareAcceptsOnlyTrustedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	service := NewService(configures.AgentAuthConfig{Enabled: true, InternalSecret: "shared"})
	engine.Use(service.NativeMiddleware())
	engine.GET("/protected", func(ctx *gin.Context) {
		principal, err := identity.FromGin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ctx.String(http.StatusOK, principal.ID)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set(internalSecretHeader, "shared")
	request.Header.Set(internalIdentityHeader, "owner-1")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "owner-1" {
		t.Fatalf("可信身份未通过: code=%d body=%s", recorder.Code, recorder.Body.String())
	}

	denied := httptest.NewRecorder()
	engine.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if denied.Code != http.StatusOK || denied.Header().Get("X-Original-Status") != "401" {
		t.Fatalf("未认证请求应返回信封 401: code=%d headers=%v", denied.Code, denied.Header())
	}
}
