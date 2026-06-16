package models

type AgentSession struct {
	ID             int64
	AppKey         string
	SessionId      string
	UniqueName     string
	CustomerId     string
	Platform       string
	PlatformConvId string
	OperatorId     string
	Status         int    // 0:等待 1:进行中 2:已关闭
	AutoMode       int    // 0:人工 1:Agent接管
	MsgCount       int
	Tags           string
	Summary        string
	FirstMsgAt     int64
	LastMsgAt      int64
	ClosedAt       int64
	UpdatedTime    int64
	CreatedTime    int64
}

type AgentFeedback struct {
	ID             int64
	AppKey         string
	FeedbackId     string
	SessionId      string
	UniqueName     string
	AgentMsgId     string
	AgentReplyText string
	Action         string // adopted / edited / rejected
	FinalReplyText string
	EditDiff       string
	RejectReason   string
	OperatorId     string
	CreatedTime    int64
}
