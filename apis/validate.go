package apis

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
	"github.com/juggleim/jugglemate-server/commons/responses"
	"github.com/juggleim/jugglemate-server/commons/tools"
	"github.com/juggleim/jugglemate-server/services"
)

const (
	Header_RequestId     string = "request-id"
	Header_AppKey        string = "appkey"
	Header_Authorization string = "Authorization"
	Header_Version       string = "version"
)

// SkipAuthPaths paths that don't require authentication
var SkipAuthPaths = []string{
	"/jim/user/register",
	"/jim/user/login",
}

func Validate(ctx *gin.Context) {
	session := tools.GenerateUUIDShort11()
	ctx.Header(Header_RequestId, session)
	ctx.Set(string(ctxs.CtxKey_Session), session)

	// check appkey
	appkey := ctx.Request.Header.Get(Header_AppKey)
	if appkey == "" {
		responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_APPKEY_REQUIRED)
		ctx.Abort()
		return
	}
	ctx.Set(string(ctxs.CtxKey_AppKey), appkey)

	version := ctx.Request.Header.Get(Header_Version)
	if version != "" {
		ctx.Set(string(ctxs.CtxKey_Version), version)
	}

	urlPath := ctx.Request.URL.Path

	// check if path requires auth
	requiresAuth := true
	for _, path := range SkipAuthPaths {
		if strings.HasPrefix(urlPath, path) {
			requiresAuth = false
			break
		}
	}

	if requiresAuth {
		tokenStr := ctx.Request.Header.Get(Header_Authorization)
		if tokenStr == "" {
			responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
			ctx.Abort()
			return
		}

		secureKey := GetSecureKey(appkey)
		if secureKey == "" {
			responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_EXISTED)
			ctx.Abort()
			return
		}

		if strings.HasPrefix(tokenStr, "Bearer ") {
			tokenStr = tokenStr[7:]
			if !services.CheckApiKey(tokenStr, appkey, secureKey) {
				responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
				ctx.Abort()
				return
			}
		} else {
			authToken, err := services.ParseTokenString(tokenStr)
			if err != nil {
				responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
				ctx.Abort()
				return
			}
			token, err := services.ParseToken(authToken, []byte(secureKey))
			if err != nil {
				responses.ErrorHttpResp(ctx, errs.IMErrorCode_APP_NOT_LOGIN)
				ctx.Abort()
				return
			}
			ctx.Set(string(ctxs.CtxKey_RequesterId), token.UserId)
			ctx.Set(string(ctxs.CtxKey_UserToken), tokenStr)
		}
	}
}

// GetSecureKey returns the secure key for an appkey
// In production, this should read from database or config
var GetSecureKey = func(appkey string) string {
	return ""
}

// RegisterSecureKeyProvider registers a function to provide secure key for an appkey
func RegisterSecureKeyProvider(provider func(appkey string) string) {
	GetSecureKey = provider
}
