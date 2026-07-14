// Package identity 定义 Agent 模块统一的调用身份。
package identity

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
)

type contextKey string

const principalKey contextKey = "agent-principal"

const ginPrincipalKey = "agent-principal"

// Role 调用方角色。
type Role string

const (
	// RoleOwner 表示 Agent 资产所有者。
	RoleOwner Role = "owner"
	// RoleAdmin 表示平台管理员。
	RoleAdmin Role = "admin"
	// RoleUser 表示使用 Agent 的终端用户。
	RoleUser Role = "user"
)

// Principal 表示通过鉴权的 Agent API 调用身份。
type Principal struct {
	ID     string
	Role   Role
	AppKey string
}

// ErrUnauthenticated 表示请求中没有可信身份。
var ErrUnauthenticated = errors.New("未认证的 Agent 调用")

// FromJMateGin 将当前项目登录上下文转换为 Agent Owner 身份。
func FromJMateGin(ctx *gin.Context) (Principal, error) {
	id := ctx.GetString(string(ctxs.CtxKey_RequesterId))
	if id == "" {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{
		ID:     id,
		Role:   RoleOwner,
		AppKey: ctx.GetString(string(ctxs.CtxKey_AppKey)),
	}, nil
}

// SetGinPrincipal 将统一身份写入 Gin 和标准请求 context。
func SetGinPrincipal(ctx *gin.Context, principal Principal) {
	ctx.Set(ginPrincipalKey, principal)
	ctx.Request = ctx.Request.WithContext(WithPrincipal(ctx.Request.Context(), principal))
}

// FromGin 从 Gin 请求读取统一身份。
func FromGin(ctx *gin.Context) (Principal, error) {
	if value, exists := ctx.Get(ginPrincipalKey); exists {
		if principal, ok := value.(Principal); ok && principal.ID != "" {
			return principal, nil
		}
	}
	return FromContext(ctx.Request.Context())
}

// WithPrincipal 将调用身份写入标准 context。
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// FromContext 从标准 context 读取调用身份。
func FromContext(ctx context.Context) (Principal, error) {
	principal, ok := ctx.Value(principalKey).(Principal)
	if !ok || principal.ID == "" {
		return Principal{}, ErrUnauthenticated
	}
	return principal, nil
}
