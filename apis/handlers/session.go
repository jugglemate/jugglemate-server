package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/juggleim/jugglechat-server-ai/apis/models"
	"github.com/juggleim/jugglechat-server-ai/storages"
	stomodels "github.com/juggleim/jugglechat-server-ai/storages/models"
)

// ==================== Session ====================

func GetSession(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Param("session_id")

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 64)
	if limit <= 0 {
		limit = 50
	}

	msgs, _ := as.QryMessagesBySession(appkey, sessionId, offset, limit)
	msgVOs := make([]*models.AgentMessageVO, 0, len(msgs))
	for _, m := range msgs {
		msgVOs = append(msgVOs, &models.AgentMessageVO{
			ID:               m.ID,
			Role:             m.Role,
			Text:             m.Text,
			Fallback:         m.Fallback,
			Source:           m.Source,
			SuggestionStatus: m.SuggestionStatus,
			MsgTime:          m.MsgTime,
			CreatedTime:      m.CreatedTime,
		})
	}

	info := &models.AgentSessionInfo{
		SessionId:      sess.SessionId,
		UniqueName:     sess.UniqueName,
		CustomerId:     sess.CustomerId,
		Platform:       sess.Platform,
		PlatformConvId: sess.PlatformConvId,
		OperatorId:     sess.OperatorId,
		Status:         sess.Status,
		AutoMode:       sess.AutoMode,
		MsgCount:       sess.MsgCount,
		Tags:           sess.Tags,
		Summary:        sess.Summary,
		FirstMsgAt:     sess.FirstMsgAt,
		LastMsgAt:      sess.LastMsgAt,
		ClosedAt:       sess.ClosedAt,
		Messages:       msgVOs,
		CreatedTime:    sess.CreatedTime,
		UpdatedTime:    sess.UpdatedTime,
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": info})
}

func TakeoverSession(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Param("session_id")
	operatorId := c.GetHeader("X-Operator-Id")
	if operatorId == "" {
		operatorId = c.Query("operator_id")
	}

	var req models.ToggleTakeoverReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "参数格式错误"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	sess.OperatorId = operatorId
	sess.AutoMode = req.AutoMode
	if sess.Status == 0 {
		sess.Status = 1
	}

	if err := as.UpdateSession(*sess); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "更新失败: " + err.Error()})
		return
	}

	info := &models.AgentSessionInfo{
		SessionId:  sess.SessionId,
		UniqueName: sess.UniqueName,
		CustomerId: sess.CustomerId,
		Status:     sess.Status,
		AutoMode:   sess.AutoMode,
		OperatorId: sess.OperatorId,
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": info})
}

// LookupSession 按 IM 会话维度查询 session（conv_id = platform_conv_id）
func LookupSession(c *gin.Context) {
	appkey := getAppkey(c)
	convId := c.Query("conv_id")
	uniqueName := c.Query("unique_name")
	log.Printf("[LookupSession] conv_id=%s, unique_name=%s, appkey=%s", convId, uniqueName, appkey)
	if convId == "" || uniqueName == "" {
		log.Printf("[LookupSession] missing params: conv_id=%q, unique_name=%q", convId, uniqueName)
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "conv_id and unique_name required"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSessionByConv(appkey, convId, uniqueName)
	if err != nil || sess == nil {
		log.Printf("[LookupSession] session not found: conv_id=%s, unique_name=%s, err=%v", convId, uniqueName, err)
		c.JSON(http.StatusOK, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	log.Printf("[LookupSession] found: session_id=%s, status=%d, auto_mode=%d", sess.SessionId, sess.Status, sess.AutoMode)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": &models.LookupSessionInfo{
		SessionId:  sess.SessionId,
		UniqueName: sess.UniqueName,
		Status:     sess.Status,
		AutoMode:   sess.AutoMode,
		MsgCount:   sess.MsgCount,
	}})
}

func SubmitFeedback(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Param("session_id")
	operatorId := c.GetHeader("X-Operator-Id")
	if operatorId == "" {
		operatorId = c.Query("operator_id")
	}

	var req models.SubmitFeedbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "参数格式错误"})
		return
	}

	if req.Action != "adopted" && req.Action != "edited" && req.Action != "rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "action must be adopted/edited/rejected"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	// 查找最近一条 pending twin 消息
	msgs, _ := as.QryMessagesBySession(appkey, sessionId, 0, 100)
	var targetMsg *stomodels.AgentMessage
	for _, m := range msgs {
		if m.Role == "twin" {
			if req.AgentMsgId != "" && fmt.Sprintf("%d", m.ID) == req.AgentMsgId {
				targetMsg = m
				break
			}
			if m.SuggestionStatus == "pending" && targetMsg == nil {
				// 取最近一条 pending
				targetMsg = m
			}
		}
	}

	if targetMsg == nil && req.AgentMsgId != "" {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "agent message not found"})
		return
	}

	feedbackId := "af_" + uuid.New().String()[:13]
	agentReplyText := ""
	if targetMsg != nil {
		agentReplyText = targetMsg.Text
	}

	switch req.Action {
	case "adopted":
		if targetMsg != nil {
			as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "adopted")
		}
		as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:         appkey,
			FeedbackId:     feedbackId,
			SessionId:      sessionId,
			UniqueName:     sess.UniqueName,
			AgentMsgId:     req.AgentMsgId,
			Action:         "adopted",
			AgentReplyText: agentReplyText,
			FinalReplyText: agentReplyText,
			OperatorId:     operatorId,
		})
		// TODO: send IM reply via imsdk
	case "edited":
		if targetMsg != nil {
			as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "edited")
		}
		as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:         appkey,
			FeedbackId:     feedbackId,
			SessionId:      sessionId,
			UniqueName:     sess.UniqueName,
			AgentMsgId:     req.AgentMsgId,
			Action:         "edited",
			AgentReplyText: agentReplyText,
			FinalReplyText: req.FinalReplyText,
			OperatorId:     operatorId,
		})
		// TODO: send edited reply via imsdk
	case "rejected":
		if targetMsg != nil {
			as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "rejected")
		}
		as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:       appkey,
			FeedbackId:   feedbackId,
			SessionId:    sessionId,
			UniqueName:   sess.UniqueName,
			AgentMsgId:   req.AgentMsgId,
			Action:       "rejected",
			RejectReason: req.RejectReason,
			OperatorId:   operatorId,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"feedback_id": feedbackId}})
}

func ListAgentSessions(c *gin.Context) {
	appkey := getAppkey(c)
	uniqueName := c.Param("unique_name")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 64)
	if limit <= 0 {
		limit = 20
	}

	as := storages.NewAgentStorage()
	sessions, err := as.QrySessionsByAgent(appkey, uniqueName, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "查询失败: " + err.Error()})
		return
	}

	items := make([]*models.AgentSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, &models.AgentSessionInfo{
			SessionId:   s.SessionId,
			UniqueName:  s.UniqueName,
			CustomerId:  s.CustomerId,
			Platform:    s.Platform,
			Status:      s.Status,
			AutoMode:    s.AutoMode,
			OperatorId:  s.OperatorId,
			MsgCount:    s.MsgCount,
			LastMsgAt:   s.LastMsgAt,
			CreatedTime: s.CreatedTime,
		})
	}

	resp := &models.AgentSessionInfos{Items: items}
	if len(items) > 0 {
		resp.Offset = fmt.Sprintf("%d", items[len(items)-1].LastMsgAt)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func ListAgentFeedbacks(c *gin.Context) {
	appkey := getAppkey(c)
	uniqueName := c.Param("unique_name")
	offset, _ := strconv.ParseInt(c.Query("offset"), 10, 64)
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 64)
	if limit <= 0 {
		limit = 20
	}

	as := storages.NewAgentStorage()
	fbs, err := as.QryFeedbacksByAgent(appkey, uniqueName, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "查询失败: " + err.Error()})
		return
	}

	items := make([]*models.AgentFeedbackInfo, 0, len(fbs))
	for _, f := range fbs {
		items = append(items, &models.AgentFeedbackInfo{
			FeedbackId:     f.FeedbackId,
			SessionId:      f.SessionId,
			UniqueName:     f.UniqueName,
			AgentMsgId:     f.AgentMsgId,
			AgentReplyText: f.AgentReplyText,
			Action:         f.Action,
			FinalReplyText: f.FinalReplyText,
			EditDiff:       f.EditDiff,
			RejectReason:   f.RejectReason,
			OperatorId:     f.OperatorId,
			CreatedTime:    f.CreatedTime,
		})
	}

	resp := &models.AgentFeedbackInfos{Items: items}
	if len(items) > 0 {
		resp.Offset = fmt.Sprintf("%d", items[len(items)-1].CreatedTime)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func getAppkey(c *gin.Context) string {
	if v := c.GetString("appkey"); v != "" {
		return v
	}
	if v := c.GetHeader("appkey"); v != "" {
		return v
	}
	if v := c.Query("appkey"); v != "" {
		return v
	}
	return c.Query("app_key")
}
