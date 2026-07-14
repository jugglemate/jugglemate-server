package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	llmdto "github.com/juggleim/jugglemate-server/agent/modules/llm/dto"
	"github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	"gorm.io/gorm"
)

type extractedMemory struct {
	Type       string `json:"type"`
	Content    string `json:"content"`
	Importance string `json:"importance"`
}

// CloseConversation 关闭会话，并按 Agent 配置提取可复用的长期记忆。
//
// 简要描述：先锁定并关闭会话，随后调用摘要模型提取结构化事实；相同 Agent、用户、
// 类型和内容只更新有效期，不重复插入，避免多次关闭或任务重试造成记忆膨胀。
func (service *Service) CloseConversation(ctx context.Context, conversationID string) error {
	var conversation model.Conversation
	var agent agentmodel.Agent
	if err := service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&conversation, "id = ?", conversationID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return businessError(404, "404_CONVERSATION_NOT_FOUND", "会话不存在")
			}
			return err
		}
		if err := tx.First(&agent, "id = ?", conversation.AgentID).Error; err != nil {
			return err
		}
		return tx.Model(&conversation).Updates(map[string]any{"status": "closed", "updated_at": gorm.Expr("now()")}).Error
	}); err != nil {
		return err
	}
	if !mapBool(agent.MemoryConfig, "longTermEnabled", false) || !mapBool(agent.MemoryConfig, "longTermExtractOnClose", true) {
		return nil
	}
	var messages []model.Message
	if err := service.db.WithContext(ctx).Where("conversation_id = ?", conversation.ID).Order("created_at").Limit(100).Find(&messages).Error; err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		lines = append(lines, message.Role+": "+message.Content)
	}
	modelID := requestModel(agent.MemoryConfig, agent.LLMModel)
	maxTokens := 1000
	response, err := service.llm.Call(ctx, llmdto.CallRequest{ModelID: modelID, MaxTokens: &maxTokens, Messages: []llmdto.Message{
		{Role: "system", Content: `提取未来对话有价值且稳定的用户记忆。只返回 JSON 数组，每项格式 {"type":"preference|fact|context","content":"内容","importance":"high|medium|low"}；没有则返回 []。`},
		{Role: "user", Content: strings.Join(lines, "\n")},
	}, Metadata: map[string]any{"call_type": "summary", "agent_id": agent.ID, "conversation_id": conversation.ID}})
	if err != nil {
		return err
	}
	memories := parseExtractedMemories(response.Content)
	expireDays := mapInt(agent.MemoryConfig, "longTermExpireDays", 90)
	expiresAt := time.Now().UTC().Add(time.Duration(expireDays) * 24 * time.Hour)
	return service.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range memories {
			var existing agentmodel.LongTermMemory
			err := tx.Where("agent_id = ? AND user_id = ? AND memory_type = ? AND content = ? AND status = 'active'", agent.ID, conversation.UserID, item.Type, item.Content).First(&existing).Error
			if err == nil {
				if err := tx.Model(&existing).Updates(map[string]any{"importance": item.Importance, "expires_at": expiresAt, "updated_at": gorm.Expr("now()")}).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			sourceID := conversation.ID
			memory := agentmodel.LongTermMemory{ID: uuid.NewString(), AgentID: agent.ID, UserID: conversation.UserID, MemoryType: item.Type, Content: item.Content, Importance: item.Importance, SourceConversationID: &sourceID, Status: "active", ExpiresAt: &expiresAt}
			if err := tx.Create(&memory).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func parseExtractedMemories(raw string) []extractedMemory {
	normalized := strings.TrimSpace(raw)
	if start, end := strings.Index(normalized, "["), strings.LastIndex(normalized, "]"); start >= 0 && end >= start {
		normalized = normalized[start : end+1]
	}
	var items []extractedMemory
	if json.Unmarshal([]byte(normalized), &items) != nil {
		return []extractedMemory{}
	}
	result := make([]extractedMemory, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item.Type = strings.ToLower(strings.TrimSpace(item.Type))
		item.Content = strings.TrimSpace(item.Content)
		item.Importance = strings.ToLower(strings.TrimSpace(item.Importance))
		if item.Type != "preference" && item.Type != "fact" && item.Type != "context" {
			continue
		}
		if item.Importance != "high" && item.Importance != "medium" && item.Importance != "low" {
			item.Importance = "medium"
		}
		key := item.Type + "\x00" + item.Content
		if item.Content == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}
