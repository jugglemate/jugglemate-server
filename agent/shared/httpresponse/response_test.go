package httpresponse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFailureKeepsHTTP200AndExposesSemanticStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	Failure(ctx, http.StatusNotFound, 404, "资源不存在")

	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态应为 200，实际为 %d", recorder.Code)
	}
	if got := recorder.Header().Get(originalStatusHeader); got != "404" {
		t.Fatalf("X-Original-Status 应为 404，实际为 %q", got)
	}
	var envelope Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if envelope.Code != float64(404) || envelope.Msg != "资源不存在" || envelope.Data != nil {
		t.Fatalf("失败信封不符合预期: %+v", envelope)
	}
}

func TestFailurePreservesStringBusinessCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	Failure(ctx, http.StatusUnauthorized, "401_UNAUTHORIZED", "未登录")

	var envelope Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if envelope.Code != "401_UNAUTHORIZED" {
		t.Fatalf("字符串业务码未保留: %#v", envelope.Code)
	}
}

func TestSuccessItemsWrapsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	SuccessItems(ctx, []string{"a", "b"})

	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 应为对象: %#v", body["data"])
	}
	items, ok := data["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items 包裹不符合预期: %#v", data["items"])
	}
}
