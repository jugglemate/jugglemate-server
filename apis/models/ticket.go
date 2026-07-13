package models

type QryTicketsReq struct {
	Status *int
	Limit  int64
	Offset int64
}

type TicketInfo struct {
	TicketId    string    `json:"ticket_id"`
	SourceId    string    `json:"source_id"`
	CustomerId  string    `json:"customer_id"`
	Customer    *UserInfo `json:"customer"`
	InboxId     string    `json:"inbox_id"`
	ChannelType string    `json:"channel_type"`
	AssigneeId  string    `json:"assignee_id"`
	Assignee    *UserInfo `json:"assignee"`
	Status      int       `json:"status"`
	CreatedTime int64     `json:"created_time"`
	UpdatedTime int64     `json:"updated_time"`
}

type QryTicketsResp struct {
	Items []*TicketInfo `json:"items"`
}

type ClaimTicketResp struct {
	Ticket *TicketInfo `json:"ticket"`
}

type TransferTicketReq struct {
	AssigneeId string `json:"assignee_id"`
}

type TransferTicketResp struct {
	Ticket *TicketInfo `json:"ticket"`
}
