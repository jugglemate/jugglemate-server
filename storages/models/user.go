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

type UserListFilter struct {
	Keyword   string
	Role      *UserRole
	SortField string
	SortOrder string
}

type UserListResult struct {
	List  []*User
	Total int64
}

type UserUpdate struct {
	LoginAccount *string
	Nickname     *string
	Email        *string
	Role         *UserRole
	LoginPass    *string
}

type IUserStorage interface {
	Create(item User) error
	FindByAccount(appkey, account string) (*User, error)
	FindByUserId(appkey, userId string) (*User, error)
	FindByAccountWithAppkey(account, appkey string) (*User, error)
	UpdateImToken(appkey, userId, imToken string) error
	QryByApp(appkey string, filter UserListFilter, limit, offset int64) (*UserListResult, error)
	UpdateUser(appkey, userId string, updates UserUpdate) error
	Delete(appkey, userId string) error
}
