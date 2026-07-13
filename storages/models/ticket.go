package models

type TicketStatus int

const (
	TicketStatusPending    TicketStatus = 0
	TicketStatusProcessing TicketStatus = 1
	TicketStatusClosed     TicketStatus = 2
	TicketStatusReOpen     TicketStatus = 3
)

type Ticket struct {
	ID          int64
	TicketId    string
	SourceId    string
	CustomerId  string
	InboxId     string
	ChannelType string
	AssigneeId  string
	Status      TicketStatus
	CreatedTime int64
	UpdatedTime int64
	AppKey      string
}

type ITicketStorage interface {
	Create(item Ticket) error
	Update(item Ticket) error
	Upsert(item Ticket) error
	Delete(appkey, ticketId string) error
	FindByTicketId(appkey, ticketId string) (*Ticket, error)
	FindBySource(appkey, sourceId string) (*Ticket, error)
	QryAll(appkey string, status *TicketStatus, limit, offset int64) ([]*Ticket, error)
	QryVisible(appkey, assigneeId string, status *TicketStatus, limit, offset int64) ([]*Ticket, error)
	QryByCustomer(appkey, customerId string, startId, limit int64) ([]*Ticket, error)
	QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*Ticket, error)
	QryByInbox(appkey, inboxId string, status int, startId, limit int64) ([]*Ticket, error)
	QryBySource(appkey, sourceId string, status int, startId, limit int64) ([]*Ticket, error)
	UpdateStatus(appkey, ticketId string, status TicketStatus) error
	ClaimIfPending(appkey, ticketId, assigneeId string) (*Ticket, error)
	RevertClaimIfAssignee(appkey, ticketId, assigneeId string) error
	TransferIfAssignee(appkey, ticketId, oldAssigneeId, newAssigneeId string) (*Ticket, error)
}
