package services

import (
	"fmt"
	"time"

	"github.com/juggleim/jugglechat-server-ai/storages"
	stomodels "github.com/juggleim/jugglechat-server-ai/storages/models"
)

var MsgCallbackService = &msgCallbackService{}

type msgCallbackService struct{}

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
	// update session last_msg_at — use targeted update to avoid clearing unique_name
	_ = as.UpdateSessionLastMsgAt(appkey, sessionId, time.Now().UnixMilli())

	items, _ := as.QryMessagesByIM(appkey, imMsgId)
	if len(items) > 0 {
		return items[0].ID, nil
	}
	return 0, nil
}
