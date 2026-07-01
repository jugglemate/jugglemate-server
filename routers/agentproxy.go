package routers

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
)

const (
	// agentAdminApiPrefix agent-server 管理 API 的固定前缀。
	agentAdminApiPrefix = "/api/v1"

	// headerInternalAuth Go 代理注入的可信内部共享密钥头。
	headerInternalAuth = "X-Internal-Auth"
	// 注入给下游的 owner/身份头（均取登录用户 userId）。
	headerUserId     = "X-User-ID"
	headerAdminId    = "X-Admin-Id"
	headerInviteCode = "X-Invite-Code"
)

// RouteAgentAdminProxy 注册 /agentapi/* 反向代理，转发到 agent-server 管理 API。
// 该路由挂在已启用 apis.Validate 的分组下，故仅登录用户可访问。
func RouteAgentAdminProxy(group *gin.RouterGroup) {
	group.Any("/agentapi/*proxyPath", agentAdminProxyHandler)
}

// agentAdminProxyHandler 校验登录态后，剥离客户端身份头并注入可信内部头，
// 将 /jmate/agentapi/<rest> 转发到 <agentAdmin.baseURL>/api/v1/<rest>。
func agentAdminProxyHandler(ctx *gin.Context) {
	cfg := configures.Config.AgentAdmin
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		log.Printf("[agentAdminProxy] agentAdmin.baseURL not configured; refusing to proxy")
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		return
	}
	target, err := url.Parse(baseURL)
	if err != nil || target.Host == "" {
		log.Printf("[agentAdminProxy] invalid agentAdmin.baseURL=%q err=%v", baseURL, err)
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_INTERNAL_TIMEOUT)
		return
	}

	// Validate 中间件已将登录用户写入上下文。
	// 仅用户 token 会写入 RequesterId；若以 API Key(Bearer) 通过鉴权则为空，
	// 此时不应注入空身份/owner 转发，直接按未登录拒绝。
	userId := strings.TrimSpace(ctx.GetString(string(ctxs.CtxKey_RequesterId)))
	if userId == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
		return
	}
	// 通配段形如 "/agents"、"/tool-providers"。
	proxyPath := ctx.Param("proxyPath")

	proxy := &httputil.ReverseProxy{
		Director: agentAdminDirector(target, proxyPath, userId, cfg.InternalSecret),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("[agentAdminProxy] upstream error: %v", e)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(ctx.Writer, ctx.Request)
}

// agentAdminDirector 构造反向代理的 Director：重写目标地址，剥离客户端身份头，
// 注入可信内部密钥与登录用户身份。抽出为独立函数便于单测。
func agentAdminDirector(target *url.URL, proxyPath, userId, secret string) func(*http.Request) {
	return func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.URL.Path = strings.TrimRight(target.Path, "/") + agentAdminApiPrefix + proxyPath
		// RawQuery 原样保留。

		// 剥离客户端可能伪造的鉴权/身份头。
		req.Header.Del("Authorization")
		req.Header.Del("appkey")
		req.Header.Del(headerInternalAuth)
		req.Header.Del(headerUserId)
		req.Header.Del(headerAdminId)
		req.Header.Del(headerInviteCode)

		// 注入可信内部密钥与登录用户身份（owner 范围）。
		req.Header.Set(headerInternalAuth, secret)
		req.Header.Set(headerUserId, userId)
		req.Header.Set(headerAdminId, userId)
		req.Header.Set(headerInviteCode, userId)
	}
}
