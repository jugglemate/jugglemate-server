// Package auth 实现 Agent 原生 API 与当前控制台的统一身份接入。
package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/shared/httpresponse"
	"github.com/juggleim/jugglemate-server/agent/shared/identity"
	"github.com/juggleim/jugglemate-server/commons/configures"
)

const (
	internalSecretHeader   = "X-Internal-Auth"
	internalIdentityHeader = "X-User-ID"
)

// Service 负责把可信内部代理或当前 JMate 登录态转换为统一 Principal。
type Service struct {
	cfg configures.AgentAuthConfig
}

// NewService 创建认证服务。
func NewService(cfg configures.AgentAuthConfig) *Service {
	return &Service{cfg: cfg}
}

// NativeMiddleware 校验 `/api/v1` 的可信内部调用身份。
//
// 简要描述：与源服务当前生产行为一致，只信任共享密钥与身份头；旧账号密码登录和
// Bearer JWT 均不再作为原生入口，避免出现两套可绕过上游权限的登录体系。
func (service *Service) NativeMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !service.cfg.Enabled {
			ctx.Next()
			return
		}
		expected := strings.TrimSpace(service.cfg.InternalSecret)
		provided := strings.TrimSpace(ctx.GetHeader(internalSecretHeader))
		userID := strings.TrimSpace(ctx.GetHeader(internalIdentityHeader))
		if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 || userID == "" {
			httpresponse.Failure(ctx, http.StatusUnauthorized, "401_UNAUTHORIZED", "未登录或登录已过期，请重新登录")
			ctx.Abort()
			return
		}
		identity.SetGinPrincipal(ctx, identity.Principal{ID: userID, Role: identity.RoleOwner})
		ctx.Request.Header.Set("X-Invite-Code", userID)
		ctx.Next()
	}
}

// ConsoleMiddleware 将当前项目已经校验的登录态转换为统一 Principal。
func (service *Service) ConsoleMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal, err := identity.FromJMateGin(ctx)
		if err != nil {
			httpresponse.Failure(ctx, http.StatusUnauthorized, "401_UNAUTHORIZED", "未登录或登录已过期，请重新登录")
			ctx.Abort()
			return
		}
		identity.SetGinPrincipal(ctx, principal)
		ctx.Request.Header.Set("X-Invite-Code", principal.ID)
		ctx.Next()
	}
}

// RegisterRoutes 注册源服务兼容的认证接口。
func (service *Service) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/auth/login", service.loginDisabled)
	group.GET("/auth/me", service.me)
}

// loginDisabled 返回旧账号密码登录已停用提示。
func (service *Service) loginDisabled(ctx *gin.Context) {
	httpresponse.Failure(ctx, http.StatusGone, "410_LOGIN_DISABLED", "登录已停用：请通过 jugglemate-server 控制台访问")
}

// me 返回当前可信调用身份。
func (service *Service) me(ctx *gin.Context) {
	principal, err := identity.FromGin(ctx)
	if err != nil {
		httpresponse.Failure(ctx, http.StatusUnauthorized, "401_UNAUTHORIZED", "未登录")
		return
	}
	httpresponse.Success(ctx, gin.H{"username": principal.ID})
}
