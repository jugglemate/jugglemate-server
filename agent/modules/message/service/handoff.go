package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/imbot-sdk-go/imbotclients/pbdefines/pbobjs"
	agentmodel "github.com/juggleim/jugglemate-server/agent/modules/agent/model"
	"github.com/juggleim/jugglemate-server/agent/modules/message/dto"
	reasoningmodel "github.com/juggleim/jugglemate-server/agent/modules/reasoning/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// EnterHuman 开启或强制接管指定 Agent 用户会话的人工状态。
func (service *Service) EnterHuman(ctx context.Context, operatorID string, request dto.HumanEnterRequest) (dto.HumanOperationResponse, error) {
	if err := service.ensureActiveBinding(ctx, request.AgentID); err != nil {
		return dto.HumanOperationResponse{}, err
	}
	existing, ttl, err := service.activeHumanSession(ctx, request.AgentID, request.UserID)
	if err != nil {
		return dto.HumanOperationResponse{}, err
	}
	if existing != nil && existing.OperatorID != operatorID && !request.ForceTakeover {
		return dto.HumanOperationResponse{}, messageError(409, "409_HUMAN_SESSION_CONFLICT", "当前会话已被其他人工接管")
	}
	now := time.Now().UTC()
	started := now
	if existing != nil && !request.ForceTakeover {
		started = existing.StartedAt
	}
	session := humanSession{OperatorID: operatorID, StartedAt: started, Reason: request.Reason}
	if existing != nil {
		session.LastSendAt = existing.LastSendAt
	}
	if err := service.saveHumanSession(ctx, request.AgentID, request.UserID, session); err != nil {
		return dto.HumanOperationResponse{}, err
	}
	if existing == nil || request.ForceTakeover {
		if err := service.redis.Del(ctx, humanPollKey(request.AgentID, request.UserID)).Err(); err != nil {
			return dto.HumanOperationResponse{}, redisFailure(err)
		}
	}
	_ = ttl
	expireAt := now.Add(humanSessionTTL)
	return dto.HumanOperationResponse{Status: "ok", Code: "200_HUMAN_INTERVENTION_ENTERED", Message: "人工介入已生效", AgentID: request.AgentID, UserID: request.UserID, OperatorID: operatorID, ExpiresInSeconds: pointerInt(int(humanSessionTTL.Seconds())), ExpireAt: &expireAt}, nil
}

// ExitHuman 退出人工状态并清除轮询通知。
func (service *Service) ExitHuman(ctx context.Context, operatorID string, request dto.HumanExitRequest) (dto.HumanOperationResponse, error) {
	if err := service.redis.Del(ctx, humanSessionKey(request.AgentID, request.UserID), humanPollKey(request.AgentID, request.UserID)).Err(); err != nil {
		return dto.HumanOperationResponse{}, redisFailure(err)
	}
	return dto.HumanOperationResponse{Status: "ok", Code: "200_HUMAN_INTERVENTION_EXITED", Message: "人工介入已退出", AgentID: request.AgentID, UserID: request.UserID, OperatorID: operatorID}, nil
}

// HumanStatus 查询人工介入状态及剩余 TTL。
func (service *Service) HumanStatus(ctx context.Context, agentID, userID string) (dto.HumanStatusResponse, error) {
	session, ttl, err := service.activeHumanSession(ctx, agentID, userID)
	if err != nil {
		return dto.HumanStatusResponse{}, err
	}
	response := dto.HumanStatusResponse{Status: "ok", Code: "200_OK", Message: "查询成功", AgentID: agentID, UserID: userID, ManualActive: session != nil}
	if session != nil {
		expireAt := time.Now().UTC().Add(ttl)
		seconds := int(ttl.Seconds())
		response.OperatorID = &session.OperatorID
		response.ExpiresInSeconds = &seconds
		response.ExpireAt = &expireAt
		response.LastSendAt = session.LastSendAt
	}
	return response, nil
}

// SendHumanMessage 使用当前 Agent 的 active Bot 发送人工消息并写入会话。
//
// 简要描述：先校验人工 session 与操作人，再完成 IM 发送；平台返回成功后落 PostgreSQL，
// 若落库失败返回明确错误以便操作端重试和人工核对，避免伪造发送成功记录。
func (service *Service) SendHumanMessage(ctx context.Context, operatorID string, request dto.HumanSendRequest) (dto.HumanSendResponse, error) {
	session, _, err := service.activeHumanSession(ctx, request.AgentID, request.UserID)
	if err != nil {
		return dto.HumanSendResponse{}, err
	}
	if session == nil {
		return dto.HumanSendResponse{}, messageError(409, "409_HUMAN_SESSION_NOT_ACTIVE", "当前会话未开启人工介入")
	}
	if session.OperatorID != operatorID {
		return dto.HumanSendResponse{}, messageError(403, "403_HUMAN_OPERATOR_MISMATCH", "当前人工会话属于其他操作人")
	}
	bot, binding, err := service.resolveBot(ctx, request.AgentID, request.BotID, request.BotUserID)
	if err != nil {
		return dto.HumanSendResponse{}, err
	}
	messageID, err := service.connections.SendText(ctx, bot.AppKey, bot.BotUserID, request.UserID, pbobjs.ChannelType_Private, request.Text)
	if err != nil {
		return dto.HumanSendResponse{}, messageError(502, "502_HUMAN_MESSAGE_SEND_FAILED", err.Error())
	}
	conversation, err := service.latestOrCreateConversation(ctx, request.AgentID, request.UserID, nil)
	if err != nil {
		return dto.HumanSendResponse{}, err
	}
	platformID := messageID
	message := reasoningmodel.Message{ID: uuid.NewString(), ConversationID: conversation.ID, MessageID: &platformID, Role: "assistant", Content: request.Text, Source: "human", OperatorID: &operatorID, SelfTokenCount: estimate(request.Text)}
	if err := service.db.WithContext(ctx).Create(&message).Error; err != nil {
		return dto.HumanSendResponse{}, messageError(500, "500_HUMAN_MESSAGE_PERSIST_FAILED", "消息已发送但落库失败: "+err.Error())
	}
	now := time.Now().UTC()
	session.LastSendAt = &now
	if err := service.saveHumanSession(ctx, request.AgentID, request.UserID, *session); err != nil {
		return dto.HumanSendResponse{}, err
	}
	expireAt := now.Add(humanSessionTTL)
	seconds := int(humanSessionTTL.Seconds())
	return dto.HumanSendResponse{HumanOperationResponse: dto.HumanOperationResponse{Status: "ok", Code: "200_HUMAN_MESSAGE_SENT", Message: "人工消息发送成功", AgentID: request.AgentID, UserID: request.UserID, OperatorID: operatorID, ExpiresInSeconds: &seconds, ExpireAt: &expireAt}, MessageID: &messageID, BotID: &binding.BotID, BotUserID: &bot.BotUserID}, nil
}

// PollHumanMessages 查询人工接管期间的用户增量消息。
func (service *Service) PollHumanMessages(ctx context.Context, agentID, userID string, since time.Time, limit int) (dto.HumanPollResponse, error) {
	if limit < 1 || limit > 100 {
		return dto.HumanPollResponse{}, messageError(400, "400_INVALID_POLL_LIMIT", "limit 必须在 1-100 之间")
	}
	active, _, err := service.activeHumanSession(ctx, agentID, userID)
	if err != nil {
		return dto.HumanPollResponse{}, err
	}
	if active == nil {
		return dto.HumanPollResponse{}, messageError(409, "409_HUMAN_SESSION_NOT_ACTIVE", "当前会话未开启人工介入")
	}
	var rows []struct {
		ID, ConversationID, Role, Content, Source string
		OperatorID                                *string
		CreatedAt                                 time.Time
	}
	query := `SELECT m.id,m.conversation_id,m.role,m.content,m.source,m.operator_id,m.created_at FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE c.agent_id=? AND c.user_id=? AND m.role='user' AND m.created_at>? ORDER BY m.created_at LIMIT ?`
	if err := service.db.WithContext(ctx).Raw(query, agentID, userID, since, limit).Scan(&rows).Error; err != nil {
		return dto.HumanPollResponse{}, err
	}
	items := make([]dto.HumanPollMessage, 0, len(rows))
	next := since
	for _, row := range rows {
		items = append(items, dto.HumanPollMessage{ID: row.ID, ConversationID: row.ConversationID, Role: row.Role, Content: row.Content, Source: row.Source, OperatorID: row.OperatorID, CreatedAt: row.CreatedAt})
		next = row.CreatedAt
	}
	if len(rows) < limit {
		_ = service.redis.Del(ctx, humanPollKey(agentID, userID)).Err()
	}
	return dto.HumanPollResponse{Status: "ok", Code: "200_OK", Message: "查询成功", AgentID: agentID, UserID: userID, SinceTS: since, NextSinceTS: next, Messages: items}, nil
}

func (service *Service) activeHumanSession(ctx context.Context, agentID, userID string) (*humanSession, time.Duration, error) {
	value, err := service.redis.Get(ctx, humanSessionKey(agentID, userID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, redisFailure(err)
	}
	session, err := parseSession(value)
	if err != nil {
		return nil, 0, messageError(500, "500_HUMAN_SESSION_CORRUPTED", "人工会话状态损坏")
	}
	ttl, err := service.redis.TTL(ctx, humanSessionKey(agentID, userID)).Result()
	if err != nil {
		return nil, 0, redisFailure(err)
	}
	if ttl <= 0 {
		return nil, 0, nil
	}
	return &session, ttl, nil
}
func (service *Service) saveHumanSession(ctx context.Context, agentID, userID string, session humanSession) error {
	value, err := marshalSession(session)
	if err != nil {
		return err
	}
	if err := service.redis.Set(ctx, humanSessionKey(agentID, userID), value, humanSessionTTL).Err(); err != nil {
		return redisFailure(err)
	}
	return nil
}
func (service *Service) ensureActiveBinding(ctx context.Context, agentID string) error {
	var count int64
	if err := service.db.WithContext(ctx).Table("bot_agent_bindings bab").Joins("JOIN bots b ON b.id=bab.bot_id").Where("bab.agent_id=? AND bab.status='active' AND b.status='active'", agentID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return messageError(404, "404_ACTIVE_BOT_BINDING_NOT_FOUND", "Agent 没有可用 Bot 绑定")
	}
	return nil
}
func (service *Service) resolveBot(ctx context.Context, agentID string, botID, botUserID *string) (agentmodel.Bot, agentmodel.BotBinding, error) {
	var binding agentmodel.BotBinding
	q := service.db.WithContext(ctx).Where("agent_id=? AND status='active'", agentID)
	if botID != nil {
		q = q.Where("bot_id=?", *botID)
	}
	if err := q.Order("updated_at DESC").First(&binding).Error; err != nil {
		return agentmodel.Bot{}, agentmodel.BotBinding{}, messageError(404, "404_ACTIVE_BOT_BINDING_NOT_FOUND", "未找到可用 Bot 绑定")
	}
	var bot agentmodel.Bot
	q = service.db.WithContext(ctx).Where("id=? AND status='active'", binding.BotID)
	if botUserID != nil {
		q = q.Where("bot_user_id=?", *botUserID)
	}
	if err := q.First(&bot).Error; err != nil {
		return agentmodel.Bot{}, agentmodel.BotBinding{}, messageError(404, "404_BOT_NOT_FOUND", "Bot 不存在或不可用")
	}
	return bot, binding, nil
}
func (service *Service) latestOrCreateConversation(ctx context.Context, agentID, userID string, metadata map[string]any) (reasoningmodel.Conversation, error) {
	var conversation reasoningmodel.Conversation
	err := service.db.WithContext(ctx).Where("agent_id=? AND user_id=? AND status='active'", agentID, userID).Order("updated_at DESC").First(&conversation).Error
	if err == nil {
		return conversation, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation, err
	}
	conversation = reasoningmodel.Conversation{ID: uuid.NewString(), AgentID: agentID, UserID: userID, Status: "active"}
	if metadata != nil {
		if value, _ := metadata["invite_code"].(string); strings.TrimSpace(value) != "" {
			conversation.InviteCode = &value
		}
	}
	return conversation, service.db.WithContext(ctx).Create(&conversation).Error
}
func redisFailure(err error) error {
	return messageError(503, "503_HUMAN_ROUTE_STATE_UNAVAILABLE", "Redis 状态不可用: "+err.Error())
}
