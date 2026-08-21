package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newRegistryMock(t *testing.T) (*RegistryService, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 SQL mock 失败: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建 GORM 连接失败: %v", err)
	}
	return NewRegistryService(db, nil), mock
}

func expectConfigReads(mock sqlmock.Sqlmock, values map[string]interface{}) {
	for _, field := range []string{"reasoning_model", "summary_model", "embedding_model", "translation_model"} {
		key := defaultModelKeys[field]
		rows := sqlmock.NewRows([]string{"config_value"})
		if value, ok := values[key]; ok {
			rows.AddRow(value)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT config_value::text FROM system_config WHERE config_key = $1")).WithArgs(key).WillReturnRows(rows)
	}
}

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

func TestRegistryTranslationDefaultRead(t *testing.T) {
	service, mock := newRegistryMock(t)
	expectConfigReads(mock, map[string]interface{}{"default_translation_model": `"translate-v1"`})

	defaults, err := service.GetDefaults(context.Background())
	if err != nil {
		t.Fatalf("读取默认模型失败: %v", err)
	}
	if defaults.TranslationModel == nil || *defaults.TranslationModel != "translate-v1" {
		t.Fatalf("translation_model 读取错误: %#v", defaults.TranslationModel)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryTranslationDefaultSetWithoutCapability(t *testing.T) {
	service, mock := newRegistryMock(t)
	mock.ExpectBegin()
	modelRows := sqlmock.NewRows([]string{"id", "model_id", "status", "model_types"}).
		AddRow("00000000-0000-0000-0000-000000000001", "translate-v1", "active", "{}")
	mock.ExpectQuery(`SELECT \* FROM "llm_models" WHERE model_id = \$1 AND status <> \$2 ORDER BY "llm_models"\."id" LIMIT \$3`).
		WithArgs("translate-v1", "archived", 1).WillReturnRows(modelRows)
	mock.ExpectExec(`INSERT INTO system_config`).
		WithArgs("default_translation_model", `"translate-v1"`, "llm-default-model", "系统默认模型配置: translation_model").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectConfigReads(mock, map[string]interface{}{"default_translation_model": `"translate-v1"`})

	value := " translate-v1 "
	defaults, err := service.SetDefaults(context.Background(), map[string]*string{"translation_model": &value})
	if err != nil {
		t.Fatalf("无 translation capability 的 active 模型应可设为翻译默认: %v", err)
	}
	if defaults.TranslationModel == nil || *defaults.TranslationModel != "translate-v1" {
		t.Fatalf("设置结果错误: %#v", defaults.TranslationModel)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryTranslationDefaultClear(t *testing.T) {
	service, mock := newRegistryMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO system_config`).
		WithArgs("default_translation_model", "null", "llm-default-model", "系统默认模型配置: translation_model").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectConfigReads(mock, map[string]interface{}{"default_translation_model": "null"})

	defaults, err := service.SetDefaults(context.Background(), map[string]*string{"translation_model": nil})
	if err != nil {
		t.Fatalf("清除翻译默认失败: %v", err)
	}
	if defaults.TranslationModel != nil {
		t.Fatalf("清除后应为 nil: %#v", defaults.TranslationModel)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryTranslationDefaultRejectsInvalidModel(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		rows      *sqlmock.Rows
		wantQuery bool
		wantErr   string
	}{
		{name: "blank", value: "  ", wantErr: "默认模型不能为空"},
		{name: "missing", value: "missing", rows: sqlmock.NewRows([]string{"id", "model_id", "status"}), wantQuery: true, wantErr: "默认模型不存在"},
		{name: "inactive", value: "disabled", rows: sqlmock.NewRows([]string{"id", "model_id", "status"}).AddRow("00000000-0000-0000-0000-000000000002", "disabled", "disabled"), wantQuery: true, wantErr: "默认模型必须为 active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mock := newRegistryMock(t)
			mock.ExpectBegin()
			if tt.wantQuery {
				mock.ExpectQuery(`SELECT \* FROM "llm_models" WHERE model_id = \$1 AND status <> \$2 ORDER BY "llm_models"\."id" LIMIT \$3`).
					WithArgs(tt.value, "archived", 1).WillReturnRows(tt.rows)
			}
			mock.ExpectRollback()
			_, err := service.SetDefaults(context.Background(), map[string]*string{"translation_model": &tt.value})
			if err == nil || !regexp.MustCompile(tt.wantErr).MatchString(err.Error()) {
				t.Fatalf("期望错误包含 %q，实际 %v", tt.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
