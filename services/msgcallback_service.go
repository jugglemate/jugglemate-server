package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/juggleim/jugglechat-server-ai/storages"
	stomodels "github.com/juggleim/jugglechat-server-ai/storages/models"
)

var MsgCallbackService = &msgCallbackService{}

type msgCallbackService struct{}

func (s *msgCallbackService) FindOrCreateSession(appkey, uniqueName, customerId, platform, platformConvId string) (*stomodels.AgentSession, error) {
	as := storages.NewAgentStorage()

	// 按 IM 会话维度查找（platform_conv_id = receiver/conversationId）
	sess, err := as.FindSessionByConv(appkey, platformConvId, uniqueName)
	if err == nil && sess != nil {
		log.Printf("[FindOrCreateSession] found existing: session_id=%s, platform_conv_id=%s, unique_name=%s, auto_mode=%d",
			sess.SessionId, platformConvId, uniqueName, sess.AutoMode)
		return sess, nil
	}

	// 新建会话
	now := time.Now().UnixMilli()
	sessionId := "as_" + uuid.New().String()[:13]
	newSess := stomodels.AgentSession{
		AppKey:         appkey,
		SessionId:      sessionId,
		UniqueName:     uniqueName,
		CustomerId:     customerId,
		Platform:       platform,
		PlatformConvId: platformConvId,
		Status:         0, // 等待分配
		AutoMode:       0, // 默认人工
		FirstMsgAt:     now,
		LastMsgAt:      now,
	}
	if err := as.CreateSession(newSess); err != nil {
		return nil, fmt.Errorf("create session failed: %w", err)
	}
	log.Printf("[FindOrCreateSession] created new: session_id=%s, platform_conv_id=%s, unique_name=%s, customer_id=%s",
		sessionId, platformConvId, uniqueName, customerId)
	return &newSess, nil
}

func (s *msgCallbackService) SaveMessage(appkey, uniqueName, customerId, sessionId, imMsgId, role, text, platform string) (int64, error) {
	as := storages.NewAgentStorage()
	msg := stomodels.AgentMessage{
		AppKey:     appkey,
		UniqueName: uniqueName,
		CustomerId: customerId,
		IMMsgId:    imMsgId,
		SessionId:  sessionId,
		Role:       role,
		Text:       text,
		Platform:   platform,
		MsgTime:    time.Now().UnixMilli(),
	}
	if err := as.CreateMessage(msg); err != nil {
		return 0, fmt.Errorf("save message failed: %w", err)
	}
	// update session last_msg_at
	_ = as.UpdateSession(stomodels.AgentSession{
		AppKey:     appkey,
		SessionId:  sessionId,
		LastMsgAt:  time.Now().UnixMilli(),
	})

	items, _ := as.QryMessagesByIM(appkey, imMsgId)
	if len(items) > 0 {
		return items[0].ID, nil
	}
	return 0, nil
}

func (s *msgCallbackService) ShouldAutoReply(session *stomodels.AgentSession) bool {
	return session.AutoMode == 1
}
