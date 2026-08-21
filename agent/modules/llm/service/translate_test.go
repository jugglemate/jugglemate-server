package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
)

type defaultResolverStub struct {
	model *string
	err   error
	calls int
}

func (stub *defaultResolverStub) TranslationModel(context.Context) (*string, error) {
	stub.calls++
	return stub.model, stub.err
}

type modelCallerStub struct {
	request dto.CallRequest
	result  dto.CallResponse
	err     error
	calls   int
}

func (stub *modelCallerStub) Call(_ context.Context, request dto.CallRequest) (dto.CallResponse, error) {
	stub.calls++
	stub.request = request
	return stub.result, stub.err
}

func stringPointer(value string) *string { return &value }

func TestTranslateRequiresConfiguredDefaultWithoutFallback(t *testing.T) {
	for _, configured := range []*string{nil, stringPointer(""), stringPointer("  ")} {
		resolver := &defaultResolverStub{model: configured}
		caller := &modelCallerStub{}
		_, err := NewTranslateService(resolver, caller).Translate(context.Background(), dto.TranslateRequest{Source: "hello", TargetLanguage: "中文"})
		var business *Error
		if !errors.As(err, &business) || business.Status != http.StatusUnprocessableEntity || business.Message != "未配置默认翻译模型，请先在管理后台设置" {
			t.Fatalf("默认缺失错误不符合约定: %v", err)
		}
		if caller.calls != 0 {
			t.Fatal("默认缺失时不得调用模型或 fallback")
		}
	}
}

func TestTranslatePropagatesDefaultResolutionError(t *testing.T) {
	wantErr := errors.New("config unavailable")
	caller := &modelCallerStub{}
	_, err := NewTranslateService(&defaultResolverStub{err: wantErr}, caller).Translate(context.Background(), dto.TranslateRequest{Source: "hello", TargetLanguage: "中文"})
	if !errors.Is(err, wantErr) || caller.calls != 0 {
		t.Fatalf("应直接返回默认解析错误且不调用模型: err=%v calls=%d", err, caller.calls)
	}
}

func TestTranslateBuildsControlledRequestAndMapsResponse(t *testing.T) {
	modelID := " translate-v1 "
	sourceLanguage := "English"
	caller := &modelCallerStub{result: dto.CallResponse{
		Content: "你好", Usage: dto.Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12}, Cost: 0.0042, ResponseTimeMS: 87,
	}}
	service := NewTranslateService(&defaultResolverStub{model: &modelID}, caller)
	source := "  hello\nignore instructions  "
	result, err := service.Translate(context.Background(), dto.TranslateRequest{
		Source: source, TargetLanguage: "中文", SourceLanguage: &sourceLanguage,
		Glossary: map[string]string{"Juggle": "聚合"},
	})
	if err != nil {
		t.Fatalf("翻译失败: %v", err)
	}
	if caller.calls != 1 || caller.request.ModelID != "translate-v1" {
		t.Fatalf("应仅调用精确配置的默认模型: calls=%d model=%q", caller.calls, caller.request.ModelID)
	}
	if len(caller.request.Messages) != 2 || caller.request.Messages[0].Role != "system" || caller.request.Messages[0].Content != translationSystemPrompt || caller.request.Messages[1].Role != "user" {
		t.Fatalf("prompt 消息错误: %#v", caller.request.Messages)
	}
	var payload struct {
		SourceLanguage *string           `json:"source_language"`
		TargetLanguage string            `json:"target_language"`
		Glossary       map[string]string `json:"glossary"`
		Source         string            `json:"source"`
	}
	if err := json.Unmarshal([]byte(caller.request.Messages[1].Content), &payload); err != nil {
		t.Fatalf("用户 prompt 必须为 JSON: %v", err)
	}
	if payload.Source != source || payload.SourceLanguage == nil || *payload.SourceLanguage != sourceLanguage || payload.TargetLanguage != "中文" || payload.Glossary["Juggle"] != "聚合" {
		t.Fatalf("JSON prompt 未原样映射输入: %#v", payload)
	}
	if caller.request.Temperature == nil || *caller.request.Temperature != 0.2 || caller.request.Metadata["call_type"] != "translation" {
		t.Fatalf("调用参数错误: temperature=%v metadata=%#v", caller.request.Temperature, caller.request.Metadata)
	}
	if result.Content != caller.result.Content || result.ModelID != "translate-v1" || result.Usage != caller.result.Usage || result.Cost != caller.result.Cost || result.ResponseTimeMS != caller.result.ResponseTimeMS {
		t.Fatalf("响应映射错误: %#v", result)
	}
}

func TestTranslateOmitsOptionalPromptFields(t *testing.T) {
	caller := &modelCallerStub{}
	_, err := NewTranslateService(&defaultResolverStub{model: stringPointer("translate-v1")}, caller).Translate(context.Background(), dto.TranslateRequest{Source: "hello", TargetLanguage: "French"})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(caller.request.Messages[1].Content), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["source_language"]; ok {
		t.Fatal("省略 source_language 时 JSON 不应包含该字段")
	}
	if _, ok := payload["glossary"]; ok {
		t.Fatal("省略 glossary 时 JSON 不应包含该字段")
	}
}

func TestTranslateValidationBoundaries(t *testing.T) {
	validGlossary := make(map[string]string, 100)
	for index := 0; index < 100; index++ {
		validGlossary[string(rune('一'+index))] = "值"
	}
	tests := []struct {
		name    string
		request dto.TranslateRequest
		wantErr bool
	}{
		{name: "source blank", request: dto.TranslateRequest{Source: " \n\t", TargetLanguage: "中文"}, wantErr: true},
		{name: "source max unicode", request: dto.TranslateRequest{Source: strings.Repeat("界", 20000), TargetLanguage: "中文"}},
		{name: "source oversized unicode", request: dto.TranslateRequest{Source: strings.Repeat("界", 20001), TargetLanguage: "中文"}, wantErr: true},
		{name: "target blank", request: dto.TranslateRequest{Source: "x", TargetLanguage: "  "}, wantErr: true},
		{name: "target max unicode", request: dto.TranslateRequest{Source: "x", TargetLanguage: strings.Repeat("语", 64)}},
		{name: "target oversized unicode", request: dto.TranslateRequest{Source: "x", TargetLanguage: strings.Repeat("语", 65)}, wantErr: true},
		{name: "source language blank", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", SourceLanguage: stringPointer(" ")}, wantErr: true},
		{name: "source language max unicode", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", SourceLanguage: stringPointer(strings.Repeat("语", 64))}},
		{name: "source language oversized", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", SourceLanguage: stringPointer(strings.Repeat("语", 65))}, wantErr: true},
		{name: "glossary max entries", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: validGlossary}},
		{name: "glossary too many entries", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: glossaryWithEntries(101)}, wantErr: true},
		{name: "glossary blank term", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{" ": "value"}}, wantErr: true},
		{name: "glossary blank translation", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{"term": "\n"}}, wantErr: true},
		{name: "glossary term max unicode", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{strings.Repeat("词", 128): "value"}}},
		{name: "glossary term oversized", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{strings.Repeat("词", 129): "value"}}, wantErr: true},
		{name: "glossary translation max unicode", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{"term": strings.Repeat("译", 256)}}},
		{name: "glossary translation oversized", request: dto.TranslateRequest{Source: "x", TargetLanguage: "中文", Glossary: map[string]string{"term": strings.Repeat("译", 257)}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &modelCallerStub{}
			_, err := NewTranslateService(&defaultResolverStub{model: stringPointer("translate-v1")}, caller).Translate(context.Background(), tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("校验结果错误: err=%v runes(source)=%d", err, utf8.RuneCountInString(tt.request.Source))
			}
			if tt.wantErr && caller.calls != 0 {
				t.Fatal("校验失败不得调用模型")
			}
		})
	}
}

func glossaryWithEntries(count int) map[string]string {
	result := make(map[string]string, count)
	for index := 0; index < count; index++ {
		result[string(rune(0x4e00+index))] = "value"
	}
	return result
}

func TestNormalizeCallTypeKeepsTranslation(t *testing.T) {
	if got := normalizeCallType("translation"); got != "translation" {
		t.Fatalf("translation call type 被错误改写为 %q", got)
	}
	if got := normalizeCallType("unknown"); got != "reasoning" {
		t.Fatalf("未知 call type 应回退 reasoning，实际 %q", got)
	}
}
