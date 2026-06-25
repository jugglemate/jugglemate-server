package models

type UserRole int

const (
	UserRoleCustomerService UserRole = 1
	UserRoleAdmin           UserRole = 2
)

type User struct {
	ID           int64
	UserId       string
	Nickname     string
	Avator       string
	LoginAccount string
	Email        string
	LoginPass    string
	Role         UserRole
	Status       int
	ImToken      string
	CreatedTime  int64
	UpdatedTime  int64
	AppKey       string
}

type IUserStorage interface {
	Create(item User) error
	FindByAccount(appkey, account string) (*User, error)
	FindByUserId(appkey, userId string) (*User, error)
	FindByAccountWithAppkey(account, appkey string) (*User, error)
	UpdateImToken(appkey, userId, imToken string) error
}
