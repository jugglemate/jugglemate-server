package models

type AgentSessionInfo struct {
	SessionId      string           `json:"session_id"`
	UniqueName     string           `json:"unique_name"`
	CustomerId     string           `json:"customer_id"`
	Platform       string           `json:"platform"`
	PlatformConvId string           `json:"platform_conv_id"`
	OperatorId     string           `json:"operator_id"`
	Status         int              `json:"status"`
	AutoMode       int              `json:"auto_mode"`
	MsgCount       int              `json:"msg_count"`
	Tags           string           `json:"tags"`
	Summary        string           `json:"summary"`
	FirstMsgAt     int64            `json:"first_msg_at"`
	LastMsgAt      int64            `json:"last_msg_at"`
	ClosedAt       int64            `json:"closed_at"`
	Messages       []*AgentMessageVO `json:"messages,omitempty"`
	CreatedTime    int64            `json:"created_time"`
	UpdatedTime    int64            `json:"updated_time"`
}

type AgentMessageVO struct {
	ID               int64  `json:"id"`
	Role             string `json:"role"`
	Text             string `json:"text"`
	Fallback         bool   `json:"fallback"`
	Source           string `json:"source"`
	SuggestionStatus string `json:"suggestion_status"`
	MsgTime          int64  `json:"msg_time"`
	CreatedTime      int64  `json:"created_time"`
}

type AgentSessionInfos struct {
	Items  []*AgentSessionInfo `json:"items"`
	Offset string              `json:"offset"`
}

type LookupSessionInfo struct {
	SessionId  string `json:"session_id"`
	UniqueName string `json:"unique_name"`
	Status     int    `json:"status"`
	AutoMode   int    `json:"auto_mode"`
	MsgCount   int    `json:"msg_count"`
}

type ToggleTakeoverReq struct {
	AutoMode int `json:"auto_mode"`
}

type SubmitFeedbackReq struct {
	AgentMsgId     string `json:"agent_msg_id"`
	Action         string `json:"action"` // adopted / edited / rejected
	FinalReplyText string `json:"final_reply_text,omitempty"`
	RejectReason   string `json:"reject_reason,omitempty"`
}

type AgentFeedbackInfo struct {
	FeedbackId     string `json:"feedback_id"`
	SessionId      string `json:"session_id"`
	UniqueName     string `json:"unique_name"`
	AgentMsgId     string `json:"agent_msg_id"`
	AgentReplyText string `json:"agent_reply_text"`
	Action         string `json:"action"`
	FinalReplyText string `json:"final_reply_text"`
	EditDiff       string `json:"edit_diff"`
	RejectReason   string `json:"reject_reason"`
	OperatorId     string `json:"operator_id"`
	CreatedTime    int64  `json:"created_time"`
}

type AgentFeedbackInfos struct {
	Items  []*AgentFeedbackInfo `json:"items"`
	Offset string               `json:"offset"`
}
