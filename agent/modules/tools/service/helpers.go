package service

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/model"
)

var tokenPattern = regexp.MustCompile(`[A-Za-z0-9_]+`)

var allowedTransitions = map[string]map[string]bool{
	"draft":      {"testing": true, "deleted": true},
	"testing":    {"testing": true, "active": true, "draft": true, "deleted": true},
	"active":     {"deprecated": true, "deleted": true},
	"deprecated": {"testing": true, "active": true, "deleted": true},
	"deleted":    {},
}

var defaultAuthSchemas = map[string]map[string]any{
	"none": {"type": "object", "additionalProperties": false, "properties": map[string]any{}},
	"api_key": {"type": "object", "additionalProperties": false, "required": []string{"api_key", "in", "name"}, "properties": map[string]any{
		"api_key": map[string]any{"type": "string", "minLength": 1}, "in": map[string]any{"type": "string", "enum": []string{"header", "query"}}, "name": map[string]any{"type": "string", "minLength": 1},
	}},
	"bearer": {"type": "object", "additionalProperties": false, "required": []string{"token"}, "properties": map[string]any{
		"token": map[string]any{"type": "string", "minLength": 1}, "prefix": map[string]any{"type": "string", "enum": []string{"Bearer"}, "default": "Bearer"},
	}},
	"basic": {"type": "object", "additionalProperties": false, "required": []string{"username", "password"}, "properties": map[string]any{
		"username": map[string]any{"type": "string", "minLength": 1}, "password": map[string]any{"type": "string", "minLength": 1},
	}},
	"oauth2_client_credentials": {"type": "object", "additionalProperties": false, "required": []string{"token_url", "client_id", "client_secret"}, "properties": map[string]any{
		"token_url": map[string]any{"type": "string", "format": "uri"}, "client_id": map[string]any{"type": "string", "minLength": 1}, "client_secret": map[string]any{"type": "string", "minLength": 1}, "scope": map[string]any{"type": "string"}, "audience": map[string]any{"type": "string"},
	}},
}

func pageValues(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func providerResponse(entity model.Provider) dto.ProviderResponse {
	return dto.ProviderResponse{ID: entity.ID, OwnerID: entity.OwnerID, Name: entity.Name, DisplayName: entity.DisplayName, ProviderType: entity.ProviderType, AuthType: entity.AuthType, AuthSchema: nonNilMap(entity.AuthSchema), ConnectionConfig: nonNilMap(entity.ConnectionConfig), Status: entity.Status, Visibility: entity.Visibility, CreatedAt: formatTime(entity.CreatedAt), UpdatedAt: formatTime(entity.UpdatedAt)}
}

func definitionResponse(entity model.Definition) dto.DefinitionResponse {
	return dto.DefinitionResponse{ID: entity.ID, ProviderID: entity.ProviderID, OwnerID: entity.OwnerID, Name: entity.Name, Description: entity.Description, ToolType: entity.ToolType, InputSchema: nonNilMap(entity.InputSchema), OutputSchema: nonNilMap(entity.OutputSchema), ExecutionConfig: nonNilMap(entity.ExecutionConfig), RiskLevel: entity.RiskLevel, ApprovalRequired: entity.ApprovalRequired, TimeoutSeconds: entity.TimeoutSeconds, Status: entity.Status, CreatedAt: formatTime(entity.CreatedAt), UpdatedAt: formatTime(entity.UpdatedAt)}
}

func credentialResponse(entity model.Credential) dto.CredentialResponse {
	var rotated *string
	if entity.RotatedAt != nil {
		value := formatTime(*entity.RotatedAt)
		rotated = &value
	}
	return dto.CredentialResponse{ID: entity.ID, OwnerID: entity.OwnerID, ProviderID: entity.ProviderID, Name: entity.Name, Status: entity.Status, RotatedAt: rotated, CreatedAt: formatTime(entity.CreatedAt), UpdatedAt: formatTime(entity.UpdatedAt)}
}

func bindingResponse(entity model.Binding) dto.BindingResponse {
	return dto.BindingResponse{ID: entity.ID, AgentID: entity.AgentID, ToolDefinitionID: entity.ToolID, Enabled: entity.Enabled, RuntimeOverrides: nonNilMap(entity.RuntimeOverrides), CredentialID: entity.CredentialID, PolicyGuardEnabled: entity.PolicyGuardEnabled, ApprovalRequired: entity.ApprovalRequired, MountedAt: formatTime(entity.MountedAt), UpdatedAt: formatTime(entity.UpdatedAt)}
}

func callRecordResponse(entity model.CallRecord) dto.CallRecordResponse {
	var billed *float64
	if entity.BilledCredit != nil {
		value, _ := entity.BilledCredit.Float64()
		billed = &value
	}
	return dto.CallRecordResponse{ID: entity.ID, TraceID: entity.TraceID, ConversationID: entity.ConversationID, AgentID: entity.AgentID, OwnerID: entity.OwnerID, ToolDefinitionID: entity.ToolDefinitionID, Status: entity.Status, InputSummary: entity.InputSummary, OutputSummary: entity.OutputSummary, ErrorCode: entity.ErrorCode, ErrorMessage: entity.ErrorMessage, DurationMS: entity.DurationMS, BilledCredit: billed, IdempotencyKey: entity.IdempotencyKey, CreatedAt: formatTime(entity.CreatedAt)}
}

func decisionRecordResponse(entity model.DecisionRecord) dto.DecisionRecordResponse {
	return dto.DecisionRecordResponse{ID: entity.ID, TraceID: entity.TraceID, ConversationID: entity.ConversationID, AgentID: entity.AgentID, OwnerID: entity.OwnerID, IntentSummary: entity.IntentSummary, CandidateTools: entity.CandidateTools, SelectedTools: entity.SelectedTools, DecisionAction: entity.DecisionAction, TerminationReason: entity.TerminationReason, CreatedAt: formatTime(entity.CreatedAt)}
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func validateSchemaShape(schema map[string]any) bool {
	if schema == nil || schema["type"] != "object" {
		return false
	}
	if properties, ok := schema["properties"]; ok {
		if _, valid := properties.(map[string]any); !valid {
			return false
		}
	}
	if required, ok := schema["required"]; ok {
		switch required.(type) {
		case []any, []string:
		default:
			return false
		}
	}
	return true
}

// validatePayload 对源服务支持的 JSON Schema 子集执行必填项与基础类型校验。
//
// 简要描述：这里有意不声称支持完整 JSON Schema；执行前只校验源实现已承诺的
// object 根结构、required 和六种基础类型，确保 Go/Python 迁移前后结果一致。
func validatePayload(schema map[string]any, payload map[string]any) error {
	for _, key := range stringSlice(schema["required"]) {
		if _, exists := payload[key]; !exists {
			return fmt.Errorf("缺少必填字段: %s", key)
		}
	}
	properties, _ := schema["properties"].(map[string]any)
	for key, raw := range properties {
		value, exists := payload[key]
		if !exists {
			continue
		}
		sub, _ := raw.(map[string]any)
		typeName, _ := sub["type"].(string)
		if !matchesType(typeName, value) {
			return fmt.Errorf("字段 %s 类型错误: 期望 %s", key, typeName)
		}
		if enum := anySlice(sub["enum"]); len(enum) > 0 && !containsJSON(enum, value) {
			return fmt.Errorf("字段 %s 不在允许范围", key)
		}
	}
	return nil
}

func matchesType(typeName string, value any) bool {
	switch typeName {
	case "", "null":
		return true
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "integer":
		switch number := value.(type) {
		case float64:
			return math.Trunc(number) == number
		case float32:
			return math.Trunc(float64(number)) == float64(number)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	case "number":
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	default:
		return true
	}
}

func containsJSON(values []any, target any) bool {
	encodedTarget, _ := json.Marshal(target)
	for _, value := range values {
		encoded, _ := json.Marshal(value)
		if string(encoded) == string(encodedTarget) {
			return true
		}
	}
	return false
}

func anySlice(value any) []any {
	switch list := value.(type) {
	case []any:
		return list
	case []string:
		result := make([]any, len(list))
		for index, item := range list {
			result[index] = item
		}
		return result
	default:
		return nil
	}
}

func stringSlice(value any) []string {
	switch list := value.(type) {
	case []string:
		return list
	case []any:
		result := make([]string, 0, len(list))
		for _, item := range list {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func evaluateRisk(method, path, name string, description *string) (string, bool) {
	haystack := strings.ToLower(strings.Join([]string{name, path, deref(description)}, " "))
	for _, keyword := range []string{"rm_rf", "force_delete", "purge_all"} {
		if strings.Contains(haystack, keyword) {
			return "critical", true
		}
	}
	for _, keyword := range []string{"transfer", "swap", "sign", "trade", "transaction", "withdraw", "delete", "drop", "purge", "revoke", "approve"} {
		if strings.Contains(haystack, keyword) {
			return "high", true
		}
	}
	switch strings.ToUpper(method) {
	case "DELETE":
		return "high", true
	case "POST", "PUT", "PATCH":
		return "medium", false
	default:
		return "low", false
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func pointer(value string) *string { return &value }

func tokenize(value string) []string {
	set := map[string]bool{}
	for _, token := range tokenPattern.FindAllString(strings.ToLower(value), -1) {
		set[token] = true
	}
	result := make([]string, 0, len(set))
	for token := range set {
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func summarize(value any) *string {
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		if len(text) > 500 {
			text = text[:500] + "...(truncated)"
		}
		return &text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		text := fmt.Sprint(value)
		return &text
	}
	text := string(encoded)
	if len(text) > 500 {
		text = text[:500] + "...(truncated)"
	}
	return &text
}
