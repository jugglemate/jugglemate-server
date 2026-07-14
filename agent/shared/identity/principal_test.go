package identity

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
)

func TestFromJMateGinBuildsOwnerPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set(string(ctxs.CtxKey_RequesterId), "owner-1")
	ctx.Set(string(ctxs.CtxKey_AppKey), "app-1")

	principal, err := FromJMateGin(ctx)
	if err != nil {
		t.Fatalf("转换身份失败: %v", err)
	}
	if principal.ID != "owner-1" || principal.Role != RoleOwner || principal.AppKey != "app-1" {
		t.Fatalf("身份内容不符合预期: %+v", principal)
	}
}
