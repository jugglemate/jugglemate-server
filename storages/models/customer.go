package models

type Customer struct {
	ID         int64
	CustomerId string
	Nickname   string
	Avator     string
	Phone      string
	Email      string
	AppKey     string
}

type ICustomerStorage interface {
	Create(item Customer) error
	Update(item Customer) error
	Upsert(item Customer) error
	Delete(appkey, customerId string) error
	FindByCustomerId(appkey, customerId string) (*Customer, error)
}
