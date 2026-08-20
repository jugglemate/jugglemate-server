package models

type TicketStatus int

const (
	TicketStatusPending    TicketStatus = 0
	TicketStatusProcessing TicketStatus = 1
	TicketStatusClosed     TicketStatus = 2
	TicketStatusReOpen     TicketStatus = 3
)

// TicketEventType 枚举工单事件的类型。
//
// TIPS: 仅 human_takeover 由当前 change 强制落地；其他枚举位预留，本 change 不写
// 数据 —— 后续审计需求新增事件类型时只需追加 enum 常量，不需要改表结构。
type TicketEventType string

const (
	TicketEventTypeHumanTakeover TicketEventType = "human_takeover"
	TicketEventTypeClaim         TicketEventType = "claim"
	TicketEventTypeTransfer      TicketEventType = "transfer"
	TicketEventTypeClose         TicketEventType = "close"
	TicketEventTypeReopen        TicketEventType = "reopen"
)

// TicketEventOperator 枚举事件触发者类型。
type TicketEventOperator string

const (
	TicketEventOperatorCustomer TicketEventOperator = "customer"
	TicketEventOperatorUser     TicketEventOperator = "user"
	TicketEventOperatorBot      TicketEventOperator = "bot"
	TicketEventOperatorSystem   TicketEventOperator = "system"
)

type Ticket struct {
	ID                int64
	TicketId          string
	SourceId          string
	CustomerId        string
	InboxId           string
	ChannelType       string
	AssigneeId        string
	Status            TicketStatus
	IsHumanTakenOver  bool
	HumanTakenOverAt  int64
	HumanTakenOverBy  string
	LastUserMsgAt     int64
	LastCustomerMsgAt int64
	ClosedAt          int64
	CsatNotifiedAt    int64
	CreatedTime       int64
	UpdatedTime       int64
	AppKey            string
}

type ITicketStorage interface {
	Create(item Ticket) error
	Update(item Ticket) error
	Upsert(item Ticket) error
	Delete(appkey, ticketId string) error
	FindByTicketId(appkey, ticketId string) (*Ticket, error)
	FindBySource(appkey, sourceId string) (*Ticket, error)
	QryAll(appkey string, status *TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*Ticket, error)
	QryVisible(appkey, assigneeId string, status *TicketStatus, isHumanTakenOver *bool, limit, offset int64) ([]*Ticket, error)
	QryByCustomer(appkey, customerId string, isHumanTakenOver *bool, startId, limit int64) ([]*Ticket, error)
	QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*Ticket, error)
	QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*Ticket, error)
	QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*Ticket, error)
	UpdateStatus(appkey, ticketId string, status TicketStatus) error
	ClaimIfPending(appkey, ticketId, assigneeId string) (*Ticket, error)
	RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error
	TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*Ticket, error)
	MarkHumanTakenOverIfZero(appkey, ticketId, by string, atMs int64) error

	// UpdateLastUserMsgAt 只在 msg_time > 现有值时更新"坐席最后消息时间"，
	// 防止历史消息回灌覆盖最新活跃时间。
	UpdateLastUserMsgAt(appkey, ticketId string, atMs int64) error
	// UpdateLastCustomerMsgAt 只在 msg_time > 现有值时更新"客户最后消息时间"，
	// 用于判断"最后一句是谁说的"，防止客户发言后被误关。
	UpdateLastCustomerMsgAt(appkey, ticketId string, atMs int64) error
	// CloseByIdle 抢占式关闭工单。仅在 status=1（处理中）或 status=3（重新打开），
	// 且 last_user_msg_at 早于给定阈值时返回 (true, nil)；已被其他实例处理或状态变更
	// 则返回 (false, nil)。
	CloseByIdle(appkey, ticketId string, idleMs int64, atMs int64) (bool, error)
	// MarkCsatNotifiedOnce 在 csat_notified_at IS NULL 时置 now（cas 防重并发）。
	MarkCsatNotifiedOnce(appkey, ticketId string, atMs int64) (bool, error)
	// QryTicketsNeedingCsatNotification 给后台 ticker 找出已关闭但未发邀请的工单。
	QryTicketsNeedingCsatNotification(limit int64) ([]*Ticket, error)
}

// TicketEvent 记录工单生命周期中的事件事实。
//
// TIPS: payload 由调用方序列化为 JSON 字符串再写入 —— 通用 string 字段避免在 storage
// 层引入额外的 JSON 依赖；DAO 层对 MySQL 使用 JSON、Postgres 使用 JSONB，类型由各自 SQL
// 决定；上层不需要为不同方言区分代码。
type TicketEvent struct {
	ID           int64
	AppKey       string
	TicketId     string
	EventType    TicketEventType
	OperatorID   string
	OperatorType TicketEventOperator
	Payload      string
	CreatedTime  int64
}

type ITicketEventStorage interface {
	Create(event TicketEvent) error
	QryByTicket(appkey, ticketId string, limit, offset int64) ([]*TicketEvent, error)
}
