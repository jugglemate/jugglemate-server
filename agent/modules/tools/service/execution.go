package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/adapter"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/tools/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type executionContext struct {
	definition model.Definition
	provider   model.Provider
	binding    *model.Binding
	credential map[string]any
	timeout    int
}

// TestDefinition 校验输入并真实调用工具，记录供激活使用的最近测试结果。
func (service *Service) TestDefinition(ctx context.Context, ownerID, definitionID string, command dto.DefinitionTestRequest) (dto.DefinitionTestResponse, error) {
	definition, err := service.visibleDefinition(ctx, ownerID, definitionID)
	if err != nil {
		return dto.DefinitionTestResponse{}, err
	}
	if validationErr := validatePayload(definition.InputSchema, command.Payload); validationErr != nil {
		code, message := "400_INVALID_SCHEMA", validationErr.Error()
		return dto.DefinitionTestResponse{Success: false, Status: "failed", ErrorCode: &code, ErrorMessage: &message, DurationMS: 0, TraceID: ""}, nil
	}
	if definition.Status == "draft" {
		definition.Status = "testing"
		if err := service.db.WithContext(ctx).Save(&definition).Error; err != nil {
			return dto.DefinitionTestResponse{}, err
		}
	}
	traceID := "trace-" + uuid.NewString()
	request := dto.ExecuteRequest{TraceID: traceID, TurnID: "test-" + traceID, AgentID: "__test__", OwnerID: ownerID, ToolDefinitionID: definitionID, Input: command.Payload}
	result := service.execute(ctx, request, command.CredentialID, true)
	service.testMu.Lock()
	service.lastTests[definitionID] = testSnapshot{success: result.Status == "success", at: time.Now().UTC()}
	service.testMu.Unlock()
	return dto.DefinitionTestResponse{Success: result.Status == "success", Status: result.Status, ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, DurationMS: result.DurationMS, TraceID: result.TraceID, OutputSummary: summarize(result.Output)}, nil
}

// ActivateDefinition 激活最近一次真实测试成功的工具定义。
func (service *Service) ActivateDefinition(ctx context.Context, ownerID, definitionID string) (dto.DefinitionResponse, error) {
	definition, err := service.visibleDefinition(ctx, ownerID, definitionID)
	if err != nil {
		return dto.DefinitionResponse{}, err
	}
	service.testMu.RLock()
	snapshot, ok := service.lastTests[definitionID]
	service.testMu.RUnlock()
	if !ok || !snapshot.success {
		return dto.DefinitionResponse{}, businessError("409_TOOL_NOT_TESTED")
	}
	if !allowedTransitions[definition.Status]["active"] {
		return dto.DefinitionResponse{}, businessError("409_INVALID_STATE_TRANSITION")
	}
	definition.Status = "active"
	if err := service.db.WithContext(ctx).Save(&definition).Error; err != nil {
		return dto.DefinitionResponse{}, err
	}
	return definitionResponse(definition), nil
}

// Execute 执行已挂载且已激活的工具调用。
func (service *Service) Execute(ctx context.Context, request dto.ExecuteRequest) dto.ExecuteResponse {
	return service.execute(ctx, request, nil, false)
}

// execute 是测试与生产工具调用共享的执行外观。
//
// 简要描述：先完成定义、Provider、挂载和凭据的原子化上下文装配，再做审批与幂等判断，
// 最后才向上游发请求；所有进入实际决策后的生产调用都会写入事实表，测试调用不会污染计费记录。
func (service *Service) execute(ctx context.Context, request dto.ExecuteRequest, forcedCredentialID *string, allowInactive bool) dto.ExecuteResponse {
	started := time.Now()
	if request.TraceID == "" {
		request.TraceID = "trace-" + uuid.NewString()
	}
	execution, err := service.buildExecutionContext(ctx, request, forcedCredentialID, allowInactive)
	if err != nil {
		return responseFromError(request.TraceID, started, err)
	}
	if execution.definition.ApprovalRequired && (execution.binding == nil || !execution.binding.ApprovalRequired) && request.AgentID != "__test__" {
		result := adapter.ExecutionResult{Status: "blocked", ErrorCode: pointer("403_TOOL_APPROVAL_REQUIRED"), ErrorMessage: pointer("该操作需要确认后执行"), DurationMS: int(time.Since(started).Milliseconds())}
		service.recordCall(ctx, request, result)
		return executionResponse(request.TraceID, result)
	}
	if request.IdempotencyKey != nil {
		var existing model.CallRecord
		err := service.db.WithContext(ctx).Where("owner_id = ? AND agent_id = ? AND tool_definition_id = ? AND idempotency_key = ?", request.OwnerID, request.AgentID, request.ToolDefinitionID, *request.IdempotencyKey).Order("created_at DESC").First(&existing).Error
		if err == nil {
			return dto.ExecuteResponse{Status: existing.Status, ErrorCode: existing.ErrorCode, ErrorMessage: existing.ErrorMessage, DurationMS: existing.DurationMS, TraceID: existing.TraceID}
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return responseFromError(request.TraceID, started, err)
		}
	}
	result := service.executor.Execute(ctx, adapter.ExecutionContext{TraceID: request.TraceID, ToolType: execution.definition.ToolType, ExecutionConfig: execution.definition.ExecutionConfig, ConnectionConfig: execution.provider.ConnectionConfig, AuthType: execution.provider.AuthType, Credential: execution.credential, Payload: request.Input, TimeoutSeconds: execution.timeout})
	if request.AgentID != "__test__" {
		service.recordCall(ctx, request, result)
	}
	return executionResponse(request.TraceID, result)
}

func (service *Service) buildExecutionContext(ctx context.Context, request dto.ExecuteRequest, forcedCredentialID *string, allowInactive bool) (executionContext, error) {
	definition, err := service.definitionByID(ctx, request.ToolDefinitionID)
	if err != nil {
		return executionContext{}, err
	}
	if !allowInactive && definition.Status != "active" {
		return executionContext{}, businessError("409_CAPABILITY_NOT_ACTIVE")
	}
	provider, err := service.providerByID(ctx, definition.ProviderID)
	if err != nil {
		return executionContext{}, err
	}
	var binding *model.Binding
	if request.AgentID != "__test__" {
		var entity model.Binding
		err := service.db.WithContext(ctx).Where("agent_id = ? AND tool_id = ?", request.AgentID, request.ToolDefinitionID).First(&entity).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && !allowInactive {
			return executionContext{}, businessError("404_BINDING_NOT_FOUND")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return executionContext{}, err
		}
		if entity.ID != "" {
			binding = &entity
			if !entity.Enabled && !allowInactive {
				return executionContext{}, businessError("409_CAPABILITY_NOT_ACTIVE")
			}
		}
	}
	credentialPayload := map[string]any(nil)
	if provider.AuthType != "none" {
		credentialID := forcedCredentialID
		if credentialID == nil && binding != nil {
			credentialID = binding.CredentialID
		}
		var credential model.Credential
		if credentialID != nil {
			err = service.db.WithContext(ctx).Where("id = ?", *credentialID).First(&credential).Error
		} else {
			err = service.db.WithContext(ctx).Where("owner_id = ? AND provider_id = ? AND status = 'active'", request.OwnerID, provider.ID).Order("updated_at DESC").First(&credential).Error
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return executionContext{}, businessError("404_CREDENTIAL_NOT_FOUND")
		}
		if err != nil {
			return executionContext{}, err
		}
		if credential.Status != "active" {
			return executionContext{}, businessError("401_TOOL_CREDENTIAL_REVOKED")
		}
		credentialPayload, err = service.decryptCredential(credential.CredentialPayloadEncrypted)
		if err != nil {
			return executionContext{}, err
		}
	}
	timeout := definition.TimeoutSeconds
	if binding != nil {
		if value, ok := numericInt(binding.RuntimeOverrides["timeoutSeconds"]); ok {
			timeout = value
		} else if value, ok := numericInt(binding.RuntimeOverrides["timeout"]); ok {
			timeout = value
		}
	}
	return executionContext{definition: definition, provider: provider, binding: binding, credential: credentialPayload, timeout: timeout}, nil
}

func (service *Service) recordCall(ctx context.Context, request dto.ExecuteRequest, result adapter.ExecutionResult) {
	var billed *decimal.Decimal
	if result.Status == "success" {
		zero := decimal.Zero
		billed = &zero
	}
	record := model.CallRecord{ID: "toolcall-" + uuid.NewString(), TraceID: request.TraceID, ConversationID: request.ConversationID, AgentID: request.AgentID, OwnerID: request.OwnerID, ToolDefinitionID: request.ToolDefinitionID, Status: result.Status, InputSummary: result.RequestSummary, OutputSummary: result.ResponseSummary, ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, DurationMS: result.DurationMS, BilledCredit: billed, IdempotencyKey: request.IdempotencyKey}
	_ = service.db.WithContext(ctx).Create(&record).Error
}

// Orchestrate 对 Agent 的 active 挂载工具进行候选评分、决策和最多一次实际调用。
func (service *Service) Orchestrate(ctx context.Context, request dto.OrchestrateRequest) (dto.OrchestrateResponse, error) {
	maxCalls := 3
	if request.MaxCalls != nil {
		maxCalls = *request.MaxCalls
	}
	if maxCalls > 10 {
		return dto.OrchestrateResponse{}, businessError("429_TOOL_CALL_BUDGET_EXCEEDED")
	}
	if maxCalls <= 0 {
		return dto.OrchestrateResponse{Terminated: "completed", Steps: []dto.OrchestrateStep{}}, nil
	}
	decision, err := service.decide(ctx, request)
	if err != nil {
		return dto.OrchestrateResponse{}, err
	}
	step := dto.OrchestrateStep{Decision: decision}
	result := dto.OrchestrateResponse{Terminated: "completed", Steps: []dto.OrchestrateStep{step}}
	if decision.Action != "call" || len(decision.SelectedTools) == 0 {
		return result, nil
	}
	execution := service.Execute(ctx, dto.ExecuteRequest{TraceID: request.TraceID, TurnID: request.TurnID, AgentID: request.AgentID, OwnerID: request.OwnerID, ToolDefinitionID: decision.SelectedTools[0], ConversationID: request.ConversationID, Input: request.Payload, IdempotencyKey: request.IdempotencyKey})
	result.Steps[0].Execution = &dto.OrchestrateExecutionResponse{Status: execution.Status, Output: execution.Output, ErrorCode: execution.ErrorCode, ErrorMessage: execution.ErrorMessage, DurationMS: execution.DurationMS, TraceID: execution.TraceID}
	if execution.Status != "success" {
		result.Terminated = "interrupted"
	} else if maxCalls <= 1 {
		result.Terminated = "budget_exhausted"
	}
	return result, nil
}

type candidateSnapshot struct {
	binding    model.Binding
	definition model.Definition
}

func (service *Service) decide(ctx context.Context, request dto.OrchestrateRequest) (dto.DecisionOutcome, error) {
	var snapshots []candidateSnapshot
	var bindings []model.Binding
	if err := service.db.WithContext(ctx).Where("agent_id = ? AND enabled = true", request.AgentID).Find(&bindings).Error; err != nil {
		return dto.DecisionOutcome{}, err
	}
	for _, binding := range bindings {
		var definition model.Definition
		err := service.db.WithContext(ctx).Where("id = ? AND status = 'active' AND deleted_at IS NULL", binding.ToolID).First(&definition).Error
		if err == nil {
			snapshots = append(snapshots, candidateSnapshot{binding: binding, definition: definition})
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return dto.DecisionOutcome{}, err
		}
	}
	if len(snapshots) == 0 {
		outcome := dto.DecisionOutcome{Action: "no_call", SelectedTools: []string{}, Candidates: []dto.DecisionCandidate{}, TerminationReason: pointer("no_active_tool")}
		return outcome, service.persistDecision(ctx, request, outcome)
	}
	candidates := make([]dto.DecisionCandidate, 0, len(snapshots))
	byID := map[string]candidateSnapshot{}
	for _, snapshot := range snapshots {
		candidate := scoreCandidate(request.Intent, request.Payload, snapshot.definition)
		candidates = append(candidates, candidate)
		byID[snapshot.definition.ID] = snapshot
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	outcome := dto.DecisionOutcome{Action: "call", SelectedTools: []string{candidates[0].ToolDefinitionID}, Candidates: candidates}
	if candidates[0].Score < 0.55 {
		outcome.Action = "no_call"
		outcome.SelectedTools = []string{}
		outcome.TerminationReason = pointer("low_confidence")
	} else if len(candidates[0].Unmatched) > 0 {
		outcome.Action = "ask_clarification"
		outcome.SelectedTools = []string{}
		outcome.TerminationReason = pointer("missing_required:" + strings.Join(candidates[0].Unmatched, ","))
	} else if len(candidates) >= 2 && candidates[0].Score-candidates[1].Score < 0.05 {
		outcome.Action = "ask_clarification"
		outcome.SelectedTools = []string{}
		outcome.TerminationReason = pointer("candidate_conflict")
	} else {
		top := byID[candidates[0].ToolDefinitionID]
		if top.definition.ApprovalRequired && !top.binding.ApprovalRequired {
			outcome.Action = "blocked"
			outcome.TerminationReason = pointer("approval_required")
		}
	}
	return outcome, service.persistDecision(ctx, request, outcome)
}

func scoreCandidate(intent string, payload map[string]any, definition model.Definition) dto.DecisionCandidate {
	intentTokens, targetTokens := tokenize(intent), tokenize(definition.Name+" "+deref(definition.Description))
	intentSet := make(map[string]bool, len(intentTokens))
	for _, token := range intentTokens {
		intentSet[token] = true
	}
	intersection := []string{}
	for _, token := range targetTokens {
		if intentSet[token] {
			intersection = append(intersection, token)
		}
	}
	intentScore := 0.0
	if len(targetTokens) > 0 {
		intentScore = float64(len(intersection)) / float64(len(targetTokens))
		if intentScore > 1 {
			intentScore = 1
		}
	}
	required := stringSlice(definition.InputSchema["required"])
	properties, _ := definition.InputSchema["properties"].(map[string]any)
	matched, unmatched := []string{}, []string{}
	schemaScore := 1.0
	if len(required) > 0 {
		for _, key := range required {
			if _, ok := payload[key]; ok {
				matched = append(matched, key)
			} else {
				unmatched = append(unmatched, key)
			}
		}
		schemaScore = float64(len(matched)) / float64(len(required))
	} else if len(properties) > 0 {
		available := 0
		for key := range properties {
			if _, ok := payload[key]; ok {
				available++
			}
		}
		schemaScore = float64(available) / float64(len(properties))
	}
	if intentScore > 0 {
		for _, token := range intersection {
			matched = append(matched, "intent:"+token)
		}
	}
	score := round4(intentScore*0.5 + schemaScore*0.4 + 0.1)
	return dto.DecisionCandidate{ToolDefinitionID: definition.ID, Name: definition.Name, Score: score, IntentScore: round4(intentScore), SchemaScore: round4(schemaScore), SuccessRateScore: 1, Matched: matched, Unmatched: unmatched}
}

func (service *Service) persistDecision(ctx context.Context, request dto.OrchestrateRequest, outcome dto.DecisionOutcome) error {
	candidates := make([]map[string]any, 0, len(outcome.Candidates))
	for _, item := range outcome.Candidates {
		candidates = append(candidates, map[string]any{"toolDefinitionId": item.ToolDefinitionID, "name": item.Name, "score": item.Score, "intentScore": item.IntentScore, "schemaScore": item.SchemaScore, "successRateScore": item.SuccessRateScore, "matched": item.Matched, "unmatched": item.Unmatched})
	}
	record := model.DecisionRecord{ID: "decision-" + uuid.NewString(), TraceID: request.TraceID, ConversationID: request.ConversationID, AgentID: request.AgentID, OwnerID: request.OwnerID, IntentSummary: truncateRunes(request.Intent, 1000), CandidateTools: candidates, SelectedTools: outcome.SelectedTools, DecisionAction: outcome.Action, TerminationReason: outcome.TerminationReason}
	return service.db.WithContext(ctx).Create(&record).Error
}

// ListCallRecords 分页查询当前 Owner 的工具调用事实。
func (service *Service) ListCallRecords(ctx context.Context, ownerID, definitionID, status, traceID string, page, pageSize int) (dto.CallRecordListResponse, error) {
	page, pageSize = pageValues(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.CallRecord{}).Where("owner_id = ?", ownerID)
	if definitionID != "" {
		query = query.Where("tool_definition_id = ?", definitionID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if traceID != "" {
		query = query.Where("trace_id = ?", traceID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.CallRecordListResponse{}, err
	}
	var entities []model.CallRecord
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.CallRecordListResponse{}, err
	}
	items := make([]dto.CallRecordResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, callRecordResponse(entity))
	}
	return dto.CallRecordListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListDecisionRecords 分页查询当前 Owner 的工具决策记录。
func (service *Service) ListDecisionRecords(ctx context.Context, ownerID, agentID, action, traceID string, page, pageSize int) (dto.DecisionRecordListResponse, error) {
	page, pageSize = pageValues(page, pageSize)
	query := service.db.WithContext(ctx).Model(&model.DecisionRecord{}).Where("owner_id = ?", ownerID)
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if action != "" {
		query = query.Where("decision_action = ?", action)
	}
	if traceID != "" {
		query = query.Where("trace_id = ?", traceID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return dto.DecisionRecordListResponse{}, err
	}
	var entities []model.DecisionRecord
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entities).Error; err != nil {
		return dto.DecisionRecordListResponse{}, err
	}
	items := make([]dto.DecisionRecordResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, decisionRecordResponse(entity))
	}
	return dto.DecisionRecordListResponse{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetDecisionRecord 按 ID 查询单条工具决策记录。
func (service *Service) GetDecisionRecord(ctx context.Context, recordID string) (dto.DecisionRecordResponse, error) {
	var entity model.DecisionRecord
	err := service.db.WithContext(ctx).Where("id = ?", recordID).First(&entity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.DecisionRecordResponse{}, businessError("404_NOT_FOUND")
	}
	if err != nil {
		return dto.DecisionRecordResponse{}, err
	}
	return decisionRecordResponse(entity), nil
}

func executionResponse(traceID string, result adapter.ExecutionResult) dto.ExecuteResponse {
	return dto.ExecuteResponse{Status: result.Status, Output: result.Output, ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, DurationMS: result.DurationMS, TraceID: traceID}
}
func responseFromError(traceID string, started time.Time, err error) dto.ExecuteResponse {
	var business *Error
	if errors.As(err, &business) {
		return dto.ExecuteResponse{Status: "failed", ErrorCode: &business.Code, ErrorMessage: &business.Message, DurationMS: int(time.Since(started).Milliseconds()), TraceID: traceID}
	}
	code, message := "500_TOOL_RUNTIME_ERROR", "执行器内部异常"
	return dto.ExecuteResponse{Status: "failed", ErrorCode: &code, ErrorMessage: &message, DurationMS: int(time.Since(started).Milliseconds()), TraceID: traceID}
}
func numericInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	case int64:
		return int(number), true
	default:
		return 0, false
	}
}
func round4(value float64) float64 { return math.Round(value*10000) / 10000 }
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}
