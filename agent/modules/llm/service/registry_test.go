package service

import (
	"testing"

	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
)

func TestValidateModelWriteCapabilityAndDimension(t *testing.T) {
	payload := dto.ModelWrite{ModelID: "chat", DisplayName: "Chat", APIModelName: "chat", Status: "draft", ContextLength: 8192, MaxOutput: 1024}
	if err := validateModelWrite(&payload); err == nil {
		t.Fatal("未声明能力的模型应校验失败")
	}
	payload.SupportsReasoning = true
	if err := validateModelWrite(&payload); err != nil {
		t.Fatalf("合法推理模型不应失败: %v", err)
	}
	payload.SupportsEmbedding = true
	if err := validateModelWrite(&payload); err == nil {
		t.Fatal("普通嵌入模型缺少维度应校验失败")
	}
}

func TestValidateConfigurableEmbeddingDimension(t *testing.T) {
	payload := dto.ModelWrite{ModelID: "qwen-text-embedding-v3", DisplayName: "Qwen", APIModelName: "qwen", Status: "active", SupportsEmbedding: true, ContextLength: 8192, MaxOutput: 1}
	if err := validateModelWrite(&payload); err != nil {
		t.Fatalf("可配置维度模型应允许使用默认维度: %v", err)
	}
	bad := 1000
	payload.EmbeddingDimension = &bad
	if err := validateModelWrite(&payload); err == nil {
		t.Fatal("不受支持的可配置维度应校验失败")
	}
}

func TestStatusTransitionsMatchSourceRules(t *testing.T) {
	if !providerTransitions["active"]["disabled"] || providerTransitions["active"]["archived"] {
		t.Fatal("Provider 状态机规则错误")
	}
	if !modelTransitions["draft"]["disabled"] || modelTransitions["active"]["archived"] {
		t.Fatal("Model 状态机规则错误")
	}
}
