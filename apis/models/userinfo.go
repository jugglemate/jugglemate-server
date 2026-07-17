package models

type UserInfo struct {
	Id       string `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type CustomerInfo struct {
	CustomerId string `json:"customer_id"`
	SourceId   string `json:"source_id"`
	Nickname   string `json:"nickname"`
	Avatar     string `json:"avatar"`
}
