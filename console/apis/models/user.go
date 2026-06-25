package models

type UserItem struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Avatar      string   `json:"avatar"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type UserListResp struct {
	List  []UserItem `json:"list"`
	Total int64      `json:"total"`
}

type QryUsersReq struct {
	Limit     int64
	Offset    int64
	Keyword   string
	Role      string
	SortField string
	SortOrder string
}

type CreateUserReq struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}

type UpdateUserReq struct {
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
}
