package models

type LoginReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type RegisterReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type LoginResp struct {
	UserId        string `json:"user_id"`
	NickName      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	Authorization string `json:"authorization"`
	ImToken       string `json:"im_token"`
}