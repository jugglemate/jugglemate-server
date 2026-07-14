package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	knowledgedto "github.com/juggleim/jugglemate-server/agent/modules/knowledge/dto"
	llmdto "github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	toolsdto "github.com/juggleim/jugglemate-server/agent/modules/tools/dto"
	"gorm.io/gorm"
)

// runReAct 执行工具候选选择、调用、观察反馈和最终回答。
//
// 简要描述：每轮先由 Tools v2 基于当前意图评分候选并执行一个工具，再把真实输出作为
// observation 持久化；成功获得观察或没有可调用工具时停止，防止无意义重复副作用。
func (service *Service) runReAct(ctx context.Context, request Request, agent agentmodel.Agent, contextMessages []llmdto.Message, retrieval []knowledgedto.SearchResult, result Result, sink EventSink) (Result, error) {
	maxRounds := mapInt(agent.ReactConfig, "maxRounds", 5)
	if maxRounds < 1 {
		maxRounds = 1
	}
	if maxRounds > 10 {
		maxRounds = 10
	}
	targetRounds := mapInt(agent.ReactConfig, "targetRounds", 3)
	session := model.ReActSession{ID: uuid.NewString(), ConversationID: request.ConversationID, AgentID: agent.ID, UserID: request.UserID, Status: "reasoning", MaxRounds: maxRounds, TargetRounds: targetRounds}
	if err := service.db.WithContext(ctx).Create(&session).Error; err != nil {
		return Result{}, err
	}
	if required, reason, err := service.policyConfirmationRequired(ctx, request, agent.ID); err != nil {
		return Result{}, err
	} else if required {
		now := time.Now().UTC()
		_ = service.db.WithContext(ctx).Model(&session).Updates(map[string]any{"status": "stopped", "completed_at": now, "error": reason}).Error
		result.Answer = "当前操作需要用户确认后继续执行。"
		result.Status = "degraded"
		result.ErrorCode = "POLICY_CONFIRM_REQUIRED"
		return result, nil
	}
	observations := make([]string, 0, maxRounds)
	toolSucceeded := false
	for round := 1; round <= maxRounds; round++ {
		payload := map[string]any{"input": request.Input}
		for key, value := range request.Metadata {
			payload[key] = value
		}
		conversationID := request.ConversationID
		maxCalls := mapInt(agent.ReactConfig, "toolsPerRound", 2)
		orchestrated, err := service.tools.Orchestrate(ctx, toolsdto.OrchestrateRequest{TraceID: request.TraceID, TurnID: fmt.Sprintf("%s-%d", request.TraceID, round), AgentID: agent.ID, OwnerID: agent.OwnerID, Intent: request.Input, Payload: payload, ConversationID: &conversationID, MaxCalls: &maxCalls})
		if err != nil {
			return Result{}, service.failSession(ctx, &session, err)
		}
		thought := "根据用户意图评估可用工具"
		observation := marshal(orchestrated)
		decision := "stop"
		calls := []map[string]any{}
		if len(orchestrated.Steps) > 0 {
			step := orchestrated.Steps[0]
			calls = append(calls, map[string]any{"selectedTools": step.Decision.SelectedTools, "action": step.Decision.Action, "execution": step.Execution})
			if step.Execution != nil && step.Execution.Status == "success" {
				toolSucceeded = true
				observations = append(observations, observation)
			}
		}
		score := confidence(retrieval, toolSucceeded, round)
		summary := truncate(observation, 500)
		roundEntity := model.ReActRound{ID: uuid.NewString(), SessionID: session.ID, RoundNumber: round, Thought: &thought, ToolCalls: calls, Observation: &observation, ObservationSummary: &summary, ConfidenceScore: &score, Decision: &decision}
		if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&roundEntity).Error; err != nil {
				return err
			}
			return tx.Model(&session).Updates(map[string]any{"current_round": round, "confidence_score": score, "status": "evaluating"}).Error
		}); err != nil {
			return Result{}, err
		}
		result.Confidence = score
		break
	}
	contextMessages = append(contextMessages, llmdto.Message{Role: "user", Content: request.Input})
	if len(observations) > 0 {
		contextMessages = append(contextMessages, llmdto.Message{Role: "system", Content: "工具执行观察如下，请基于真实结果回答用户，不要虚构未返回的信息：\n" + strings.Join(observations, "\n")})
	} else {
		contextMessages = append(contextMessages, llmdto.Message{Role: "system", Content: "当前没有匹配或成功执行的工具，请直接回答并说明能力边界。"})
	}
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	content, usage, err := service.generate(ctx, request, agent, contextMessages, maxTokens, sink)
	if err != nil {
		return Result{}, service.failSession(ctx, &session, err)
	}
	now := time.Now().UTC()
	if err := service.db.WithContext(ctx).Model(&session).Updates(map[string]any{"status": "completed", "completed_at": now, "confidence_score": result.Confidence}).Error; err != nil {
		return Result{}, err
	}
	result.Answer = content
	result.InputTokens = usage.InputTokens
	result.OutputTokens = usage.OutputTokens
	return result, nil
}

func (service *Service) policyConfirmationRequired(ctx context.Context, request Request, agentID string) (bool, string, error) {
	var approvalBindings int64
	if err := service.db.WithContext(ctx).Table("agent_tools").Where("agent_id = ? AND enabled=true AND approval_required=true", agentID).Count(&approvalBindings).Error; err != nil {
		return false, "", err
	}
	action := metadataString(request.Metadata, "action")
	if action == "" {
		return approvalBindings > 0 && metadataBool(request.Metadata, "execute_high_risk"), "工具挂载要求用户确认", nil
	}
	var policy struct {
		HighRiskOperations []string `gorm:"column:high_risk_operations;serializer:json"`
		ApprovalFlow       string   `gorm:"column:approval_flow"`
	}
	err := service.db.WithContext(ctx).Table("policy_rules").Where("status='active'").Order("version DESC").First(&policy).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return false, "", err
	}
	for _, operation := range policy.HighRiskOperations {
		if strings.EqualFold(operation, action) {
			return true, "高风险操作需按 " + policy.ApprovalFlow + " 流程确认", nil
		}
	}
	return false, "", nil
}

func (service *Service) failSession(ctx context.Context, session *model.ReActSession, cause error) error {
	now := time.Now().UTC()
	message := cause.Error()
	_ = service.db.WithContext(context.WithoutCancel(ctx)).Model(session).Updates(map[string]any{"status": "failed", "completed_at": now, "error": message}).Error
	return cause
}

func metadataBool(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}
