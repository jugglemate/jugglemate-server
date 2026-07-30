package models

// TicketMessage 记录工单内 IM 群消息的 metadata（who / when / type / msg_id）。
//
// TIPS: 不存 message content —— JuggleIM 已经持久化了正文（带 PII 风险且存储
// 成本高）。本表仅用于：（1）工单关闭逻辑判定"最后一条消息是坐席/客户"；
// （2）FRT 计算；（3）审计追溯。
type TicketMessage struct {
	ID          int64
	AppKey      string
	TicketId    string
	SenderId    string
	SenderRole  TicketEventOperator
	MsgId       string
	MsgType     string
	CreatedTime int64
}

type ITicketMessageStorage interface {
	// UpsertByMsgId 按 (app_key, msg_id) 去重写入；存在则更新 sender_role / msg_type / 时间。
	UpsertByMsgId(item TicketMessage) error
	// QryLastByTicket 取该 ticket 最近一条消息，用于判定"最后一条是谁发的"。
	QryLastByTicket(appkey, ticketId string) (*TicketMessage, error)
}
