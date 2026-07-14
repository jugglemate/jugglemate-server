// Package httpresponse 实现 Agent API 的统一响应契约。
package httpresponse

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const originalStatusHeader = "X-Original-Status"

// Envelope Agent JSON API 的统一响应信封。
type Envelope struct {
	Code interface{} `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// ItemsData 数组接口的统一数据结构。
type ItemsData struct {
	Items interface{} `json:"items"`
}

// Success 返回成功对象。
func Success(ctx *gin.Context, data interface{}) {
	ctx.Header(originalStatusHeader, "200")
	ctx.JSON(http.StatusOK, Envelope{Code: 0, Msg: "success", Data: data})
}

// Created 返回创建成功对象，并保留源接口的 201 语义状态。
func Created(ctx *gin.Context, data interface{}) {
	ctx.Header(originalStatusHeader, "201")
	ctx.JSON(http.StatusOK, Envelope{Code: 0, Msg: "success", Data: data})
}

// Accepted 返回已接受处理对象，并保留源接口的 202 语义状态。
func Accepted(ctx *gin.Context, data interface{}) {
	ctx.Header(originalStatusHeader, "202")
	ctx.JSON(http.StatusOK, Envelope{Code: 0, Msg: "success", Data: data})
}

// SuccessItems 返回成功数组，并按源服务契约包裹为 data.items。
func SuccessItems(ctx *gin.Context, items interface{}) {
	Success(ctx, ItemsData{Items: items})
}

// Failure 返回业务失败。
//
// 简要描述：为兼容源服务，对外 HTTP 状态固定为 200，真实语义状态同时写入
// code 和 X-Original-Status；调用方必须以 code 判断业务结果。
func Failure(ctx *gin.Context, semanticStatus int, code interface{}, message string) {
	if semanticStatus <= 0 {
		semanticStatus = http.StatusInternalServerError
	}
	if code == nil || code == 0 || code == "" {
		code = semanticStatus
	}
	ctx.Header(originalStatusHeader, statusCodeString(semanticStatus))
	ctx.JSON(http.StatusOK, Envelope{Code: code, Msg: message, Data: nil})
}

// statusCodeString 将状态码转换为响应头使用的十进制字符串。
func statusCodeString(status int) string {
	const digits = "0123456789"
	if status == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for status > 0 {
		i--
		buf[i] = digits[status%10]
		status /= 10
	}
	return string(buf[i:])
}
