package models

type Customer struct {
	ID          int64
	CustomerId  string
	Nickname    string
	Avator      string
	Phone       string
	Email       string
	Identifier  string
	CreatedTime int64
	UpdatedTime int64
	AppKey      string
}

type ICustomerStorage interface {
	Create(item Customer) error
	Update(item Customer) error
	Upsert(item Customer) error
	Delete(appkey, customerId string) error
	FindByCustomerId(appkey, customerId string) (*Customer, error)
	FindByIdentifier(appkey, identifier string) (*Customer, error)
}

type CustomerInboxRel struct {
	ID          int64
	CustomerId  string
	InboxId     string
	SourceId    string
	CreatedTime int64
	UpdatedTime int64
	AppKey      string
}

type ICustomerInboxRelStorage interface {
	Create(item CustomerInboxRel) error
	Update(item CustomerInboxRel) error
	Upsert(item CustomerInboxRel) error
	Delete(appkey, customerId, inboxId, sourceId string) error
	Find(appkey, customerId, inboxId, sourceId string) (*CustomerInboxRel, error)
	FindByCustomerInbox(appkey, customerId, inboxId string) (*CustomerInboxRel, error)
	QryByCustomer(appkey, customerId string, startId, limit int64) ([]*CustomerInboxRel, error)
	QryByInbox(appkey, inboxId string, startId, limit int64) ([]*CustomerInboxRel, error)
}
