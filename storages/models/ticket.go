package models

type TicketStatus int

const (
	TicketStatusOpen   TicketStatus = 0
	TicketStatusClosed TicketStatus = 1
)

type Ticket struct {
	ID          int64
	TicketId    string
	CustomerId  string
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
	QryByCustomer(appkey, customerId string, startId, limit int64) ([]*Ticket, error)
	QryByAssignee(appkey, assigneeId string, status int, startId, limit int64) ([]*Ticket, error)
	UpdateStatus(appkey, ticketId string, status TicketStatus) error
}
