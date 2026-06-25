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

type CustomerChannelRel struct {
	ID          int64
	CustomerId  string
	ChannelId   string
	SourceId    string
	CreatedTime int64
	UpdatedTime int64
	AppKey      string
}

type ICustomerChannelRelStorage interface {
	Create(item CustomerChannelRel) error
	Update(item CustomerChannelRel) error
	Upsert(item CustomerChannelRel) error
	Delete(appkey, customerId, channelId, sourceId string) error
	Find(appkey, customerId, channelId, sourceId string) (*CustomerChannelRel, error)
	FindByCustomerChannel(appkey, customerId, channelId string) (*CustomerChannelRel, error)
	QryByCustomer(appkey, customerId string, startId, limit int64) ([]*CustomerChannelRel, error)
	QryByChannel(appkey, channelId string, startId, limit int64) ([]*CustomerChannelRel, error)
}
