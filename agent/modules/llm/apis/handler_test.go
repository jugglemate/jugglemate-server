package apis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/service"
)

type translationServiceStub struct {
	request dto.TranslateRequest
	result  dto.TranslateResponse
	err     error
	calls   int
}

func (stub *translationServiceStub) Translate(_ context.Context, request dto.TranslateRequest) (dto.TranslateResponse, error) {
	stub.calls++
	stub.request = request
	return stub.result, stub.err
}

func newTranslationEngine(translator translationService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil, nil, translator).RegisterRoutes(engine.Group("/api/v1"))
	return engine
}

func performTranslationRequest(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/llm/translate", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)
	return recorder
}

func responseEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var envelope map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, recorder.Body.String())
	}
	return envelope
}

func TestRegisterRoutesIncludesTranslationEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewHandler(nil, nil).RegisterRoutes(engine.Group("/api/v1"))

	count := 0
	foundTranslation := false
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/admin/llm/") || strings.HasPrefix(route.Path, "/api/v1/llm/") {
			count++
		}
		if route.Method == http.MethodPost && route.Path == "/api/v1/llm/translate" {
			foundTranslation = true
		}
	}
	if count != 17 || !foundTranslation {
		t.Fatalf("LLM 路由应含 17 个接口及翻译 POST，count=%d found=%v routes=%#v", count, foundTranslation, engine.Routes())
	}
}

func TestTranslateHandlerAcceptsValidRequest(t *testing.T) {
	translator := &translationServiceStub{result: dto.TranslateResponse{
		Content: "你好", ModelID: "translate-v1", Usage: dto.Usage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4}, Cost: 0.01, ResponseTimeMS: 25,
	}}
	recorder := performTranslationRequest(newTranslationEngine(translator), `{"source":"hello","target_language":"中文","source_language":"English","glossary":{"hello":"你好"}}`)
	envelope := responseEnvelope(t, recorder)
	if recorder.Code != http.StatusOK || envelope["code"] != float64(0) || translator.calls != 1 {
		t.Fatalf("有效请求应成功: status=%d body=%s calls=%d", recorder.Code, recorder.Body.String(), translator.calls)
	}
	if translator.request.Source != "hello" || translator.request.TargetLanguage != "中文" || translator.request.SourceLanguage == nil || *translator.request.SourceLanguage != "English" || translator.request.Glossary["hello"] != "你好" {
		t.Fatalf("Handler 请求映射错误: %#v", translator.request)
	}
	data := envelope["data"].(map[string]interface{})
	if data["content"] != "你好" || data["model_id"] != "translate-v1" || data["response_time_ms"] != float64(25) {
		t.Fatalf("Handler 响应映射错误: %#v", data)
	}
}

func TestTranslateHandlerRejectsInvalidJSONContracts(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "required source", body: `{"target_language":"中文"}`},
		{name: "required target language", body: `{"source":"hello"}`},
		{name: "unknown field", body: `{"source":"hello","target_language":"中文","target_languge":"法语"}`},
		{name: "model id", body: `{"source":"hello","target_language":"中文","model_id":"caller-model"}`},
		{name: "multiple json values", body: `{"source":"hello","target_language":"中文"} {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator := &translationServiceStub{}
			recorder := performTranslationRequest(newTranslationEngine(translator), tt.body)
			envelope := responseEnvelope(t, recorder)
			if envelope["code"] != float64(http.StatusUnprocessableEntity) || recorder.Header().Get("X-Original-Status") != "422" {
				t.Fatalf("非法请求应返回校验错误: body=%s", recorder.Body.String())
			}
			if translator.calls != 0 {
				t.Fatal("非法请求不得调用翻译服务")
			}
		})
	}
}

func TestTranslateHandlerMapsServiceError(t *testing.T) {
	translator := &translationServiceStub{err: &service.Error{Status: http.StatusUnprocessableEntity, Message: "未配置默认翻译模型，请先在管理后台设置"}}
	recorder := performTranslationRequest(newTranslationEngine(translator), `{"source":"hello","target_language":"中文"}`)
	envelope := responseEnvelope(t, recorder)
	if envelope["code"] != float64(http.StatusUnprocessableEntity) || envelope["msg"] != "未配置默认翻译模型，请先在管理后台设置" || translator.calls != 1 {
		t.Fatalf("业务错误映射错误: body=%s calls=%d", recorder.Body.String(), translator.calls)
	}
}
