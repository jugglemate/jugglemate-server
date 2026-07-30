package models

// TicketRatingInfo 是 GET /jmate/tickets/:id/rating 响应的 data。
//
// rating == 0 表示该客户没评过。
type TicketRatingInfo struct {
	TicketId    string `json:"ticket_id"`
	CustomerId  string `json:"customer_id,omitempty"`
	AssigneeId  string `json:"assignee_id,omitempty"`
	Rating      int    `json:"rating"`
	Comment     string `json:"comment"`
	CreatedTime int64  `json:"created_time"`
}

// QryTicketRatingResp GET 响应信封。
type QryTicketRatingResp struct {
	Rating *TicketRatingInfo `json:"rating"` // nil = 未评
}
