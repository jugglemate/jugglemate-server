package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	juggleimsdk "github.com/juggleim/imserver-sdk-go"
	"github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/agentclient"
	"github.com/juggleim/jugglemate-server/commons/imsdk"
	"github.com/juggleim/jugglemate-server/storages"
	stomodels "github.com/juggleim/jugglemate-server/storages/models"
)

var (
	getFeedbackIMSDK = imsdk.GetImSdk
	sendFeedbackIM   = func(sdk *juggleimsdk.JuggleIMSdk, message juggleimsdk.Message) (juggleimsdk.ApiCode, string, error) {
		return sdk.SendPrivateMsg(message)
	}
)

// ==================== Session ====================

// GetSession 查询客服会话详情及其消息记录。
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
			PendingSource:    m.PendingSource,
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

// TakeoverSession 切换会话的人工、协助或自动接管模式。
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
	if req.AutoMode == 0 {
		sess.UniqueName = "" // 人工回复时解绑 agent
	}
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

// LookupSession 按 session_id 查找会话，用于前端点击会话时回显状态。
func LookupSession(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Query("session_id")
	if sessionId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "session_id required"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		log.Printf("[LookupSession] not found: session_id=%s, err=%v", sessionId, err)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil})
		return
	}

	log.Printf("[LookupSession] found: session_id=%s, status=%d, auto_mode=%d", sess.SessionId, sess.Status, sess.AutoMode)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": &models.LookupSessionInfo{
		SessionId:  sess.SessionId,
		UniqueName: sess.UniqueName,
		Status:     sess.Status,
		AutoMode:   sess.AutoMode,
		MsgCount:   sess.MsgCount,
		Agent:      getAgentSnapshot(as, appkey, sess.UniqueName),
	}})
}

// SubmitFeedback 提交对 Agent 建议的采纳、编辑或拒绝反馈。
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
	if (req.Action == "adopted" || req.Action == "edited") && targetMsg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "没有可发送的 Agent 建议"})
		return
	}
	if req.Action == "edited" && req.FinalReplyText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "final_reply_text required"})
		return
	}

	feedbackId := "af_" + uuid.New().String()[:13]
	agentReplyText := ""
	if targetMsg != nil {
		agentReplyText = targetMsg.Text
		if req.AgentMsgId == "" {
			req.AgentMsgId = strconv.FormatInt(targetMsg.ID, 10)
		}
	}

	// 简要描述：采纳或编辑建议必须先真实发送给客户；IM 不可用时不写入“已处理”反馈，
	// 防止控制台显示成功但客户实际未收到消息。拒绝动作不产生外发消息。
	if req.Action == "adopted" || req.Action == "edited" {
		text := agentReplyText
		if req.Action == "edited" {
			text = req.FinalReplyText
		}
		twin, err := as.FindTwin(appkey, sess.UniqueName)
		if err != nil || twin == nil || twin.BotId == "" {
			log.Printf("[SubmitFeedback] 查询 IM Bot 失败: session_id=%s err=%v", sessionId, err)
			c.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": "未找到可用 IM Bot"})
			return
		}
		if err := sendSessionFeedbackReply(appkey, twin.BotId, sess.CustomerId, text); err != nil {
			log.Printf("[SubmitFeedback] 发送 IM 回复失败: session_id=%s err=%v", sessionId, err)
			c.JSON(http.StatusBadGateway, gin.H{"code": -1, "msg": "发送客户回复失败"})
			return
		}
	}

	switch req.Action {
	case "adopted":
		if targetMsg != nil {
			if err := as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "adopted"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "更新建议状态失败"})
				return
			}
		}
		if err := as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:         appkey,
			FeedbackId:     feedbackId,
			SessionId:      sessionId,
			UniqueName:     sess.UniqueName,
			AgentMsgId:     req.AgentMsgId,
			Action:         "adopted",
			AgentReplyText: agentReplyText,
			FinalReplyText: agentReplyText,
			OperatorId:     operatorId,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "保存反馈失败"})
			return
		}
	case "edited":
		if targetMsg != nil {
			if err := as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "edited"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "更新建议状态失败"})
				return
			}
		}
		if err := as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:         appkey,
			FeedbackId:     feedbackId,
			SessionId:      sessionId,
			UniqueName:     sess.UniqueName,
			AgentMsgId:     req.AgentMsgId,
			Action:         "edited",
			AgentReplyText: agentReplyText,
			FinalReplyText: req.FinalReplyText,
			OperatorId:     operatorId,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "保存反馈失败"})
			return
		}
	case "rejected":
		if targetMsg != nil {
			if err := as.UpdateMessageSuggestionStatus(appkey, targetMsg.ID, "rejected"); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "更新建议状态失败"})
				return
			}
		}
		if err := as.CreateFeedback(stomodels.AgentFeedback{
			AppKey:       appkey,
			FeedbackId:   feedbackId,
			SessionId:    sessionId,
			UniqueName:   sess.UniqueName,
			AgentMsgId:   req.AgentMsgId,
			Action:       "rejected",
			RejectReason: req.RejectReason,
			OperatorId:   operatorId,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "保存反馈失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"feedback_id": feedbackId}})
}

// sendSessionFeedbackReply 将运营采纳或编辑后的文本以机器人身份发送给会话客户。
func sendSessionFeedbackReply(appkey, botID, customerID, text string) error {
	if appkey == "" || botID == "" || customerID == "" || text == "" {
		return errors.New("应用、机器人、客户或回复内容为空")
	}
	sdk := getFeedbackIMSDK(appkey)
	if sdk == nil {
		return errors.New("IM SDK 未初始化")
	}
	store, count := true, true
	code, messageID, err := sendFeedbackIM(sdk, juggleimsdk.Message{SenderId: botID, TargetIds: []string{customerID}, MsgType: "jg:text", MsgContent: fmt.Sprintf(`{"content":%q}`, text), IsStorage: &store, IsCount: &count})
	if err != nil {
		return err
	}
	if code != juggleimsdk.ApiCode_Success {
		return fmt.Errorf("IM 返回失败状态: code=%d message_id=%s", code, messageID)
	}
	return nil
}

// ListAgentSessions 分页查询指定 Agent 的客服会话。
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

// ListAgentFeedbacks 分页查询指定 Agent 的运营反馈。
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

// ==================== Create / Update Session Agent ====================

// CreateSession 获取或创建客服会话，并返回会话及 Agent 快照。
func CreateSession(c *gin.Context) {
	appkey := getAppkey(c)
	var req models.CreateSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "参数格式错误"})
		return
	}
	if req.ConvId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "conv_id required"})
		return
	}
	if req.UniqueName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "unique_name required"})
		return
	}

	as := storages.NewAgentStorage()

	// 先查找是否已存在（status != 2 的活跃 session）
	sess, err := as.FindSessionByConv(appkey, req.ConvId, req.UniqueName)
	if err == nil && sess != nil {
		// 已存在，直接返回 + agent 快照
		info := buildSessionInfo(sess)
		info.Agent = getAgentSnapshot(as, appkey, sess.UniqueName)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": info, "created": false})
		return
	}

	// 不存在活跃 session，检查是否有已关闭的（status=2）
	sessionId := req.ConvId + "_" + req.ConvType
	closedSess, _ := as.FindSession(appkey, sessionId)
	if closedSess != nil && closedSess.Status == 2 {
		// 复活已关闭的 session
		closedSess.Status = 0
		closedSess.AutoMode = req.AutoMode
		closedSess.LastMsgAt = time.Now().UnixMilli()
		if err := as.UpdateSession(*closedSess); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "复活 session 失败: " + err.Error()})
			return
		}
		info := buildSessionInfo(closedSess)
		info.Agent = getAgentSnapshot(as, appkey, closedSess.UniqueName)
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": info, "created": false, "resurrected": true})
		return
	}

	// 完全不存在，创建新 session
	now := time.Now().UnixMilli()
	newSess := stomodels.AgentSession{
		AppKey:         appkey,
		SessionId:      sessionId,
		UniqueName:     req.UniqueName,
		CustomerId:     req.CustomerId,
		Platform:       req.Platform,
		PlatformConvId: req.ConvId,
		Status:         0,
		AutoMode:       req.AutoMode,
		FirstMsgAt:     now,
		LastMsgAt:      now,
	}
	if err := as.CreateSession(newSess); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "创建 session 失败: " + err.Error()})
		return
	}

	info := buildSessionInfo(&newSess)
	info.Agent = getAgentSnapshot(as, appkey, newSess.UniqueName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": info, "created": true})
}

// UpdateSessionAgent 更新客服会话绑定的 Agent。
func UpdateSessionAgent(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Param("session_id")

	var req models.UpdateSessionAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "参数格式错误"})
		return
	}
	if req.UniqueName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "unique_name required"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	sess.UniqueName = req.UniqueName
	if err := as.UpdateSession(*sess); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "msg": "更新失败: " + err.Error()})
		return
	}

	info := buildSessionInfo(sess)
	info.Agent = getAgentSnapshot(as, appkey, sess.UniqueName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": info})
}

// SessionChat 将前端会话消息交给进程内 Go Agent 推理并持久化问答记录。
func SessionChat(c *gin.Context) {
	appkey := getAppkey(c)
	sessionId := c.Param("session_id")

	var req models.SessionChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "参数格式错误"})
		return
	}
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "msg": "message required"})
		return
	}

	as := storages.NewAgentStorage()
	sess, err := as.FindSession(appkey, sessionId)
	if err != nil || sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "msg": "session not found"})
		return
	}

	// 保存用户消息
	customerMsg := stomodels.AgentMessage{
		AppKey:     appkey,
		UniqueName: sess.UniqueName,
		CustomerId: sess.CustomerId,
		SessionId:  sessionId,
		Role:       "customer",
		Text:       req.Message,
		Platform:   sess.Platform,
		MsgTime:    time.Now().UnixMilli(),
	}
	if err := as.CreateMessage(customerMsg); err != nil {
		log.Printf("[SessionChat] SaveMessage(customer) failed: %v", err)
	}

	// 调用 agent Chat API
	client := newAgentClient()
	chatCtx := context.Background()
	resp, _, chatErr := client.Chat(chatCtx, sess.CustomerId, "jim", sess.UniqueName, agentclient.ChatRequest{Message: req.Message})

	reply := "抱歉，助手暂时无法回复，请稍后再试。"
	fallback := true
	agentMsgID := ""
	if chatErr != nil {
		log.Printf("[SessionChat] agent Chat error: code=<see client log> err=%v", chatErr)
	} else if resp == nil {
		log.Printf("[SessionChat] agent Chat returned nil response (no error but nil struct)")
	} else if resp.Reply == "" {
		log.Printf("[SessionChat] agent Chat returned empty reply: message_id=%s session_id=%s fallback=%v",
			resp.MessageID, resp.SessionID, resp.Fallback)
	} else {
		reply = resp.Reply
		fallback = resp.Fallback
		agentMsgID = resp.MessageID
	}

	// 保存 agent 回复
	twinMsg := stomodels.AgentMessage{
		AppKey:     appkey,
		UniqueName: sess.UniqueName,
		CustomerId: sess.CustomerId,
		SessionId:  sessionId,
		Role:       "twin",
		Text:       reply,
		Fallback:   fallback,
		Source:     "agent",
		Platform:   sess.Platform,
		MsgTime:    time.Now().UnixMilli(),
	}
	if agentMsgID != "" {
		twinMsg.AgentMessageId = agentMsgID
	}
	if err := as.CreateMessage(twinMsg); err != nil {
		log.Printf("[SessionChat] CreateMessage(twin) failed: %v", err)
	}

	// 更新 session 最后消息时间 — 使用精准更新避免清空 unique_name
	_ = as.UpdateSessionLastMsgAt(appkey, sessionId, time.Now().UnixMilli())

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": models.SessionChatResp{
		MessageID: agentMsgID,
		SessionID: sessionId,
		Reply:     reply,
		Fallback:  fallback,
		CreatedAt: fmt.Sprintf("%d", time.Now().UnixMilli()),
	}})
}

// ==================== Helpers ====================

func newAgentClient() *agentclient.Client {
	return agentclient.New(agentclient.Config{})
}

func buildSessionInfo(sess *stomodels.AgentSession) *models.AgentSessionInfo {
	return &models.AgentSessionInfo{
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
		CreatedTime:    sess.CreatedTime,
		UpdatedTime:    sess.UpdatedTime,
	}
}

func getAgentSnapshot(as stomodels.AgentStorage, appkey, uniqueName string) *models.AgentSnapshot {
	if uniqueName == "" {
		return nil
	}
	twin, err := as.FindTwin(appkey, uniqueName)
	if err != nil || twin == nil {
		return &models.AgentSnapshot{UniqueName: uniqueName}
	}
	return &models.AgentSnapshot{
		UniqueName:  twin.UniqueName,
		DisplayName: twin.DisplayName,
		AvatarURL:   twin.AvatarURL,
		Greeting:    twin.Greeting,
		Status:      twin.Status,
	}
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
