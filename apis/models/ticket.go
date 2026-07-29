package models

type QryTicketsReq struct {
	Status           *int
	IsHumanTakenOver *bool
	Limit            int64
	Offset           int64
}

type TicketInfo struct {
	TicketId         string    `json:"ticket_id"`
	SourceId         string    `json:"source_id"`
	CustomerId       string    `json:"customer_id"`
	Customer         *UserInfo `json:"customer"`
	InboxId          string    `json:"inbox_id"`
	ChannelType      string    `json:"channel_type"`
	AssigneeId       string    `json:"assignee_id"`
	Assignee         *UserInfo `json:"assignee"`
	Status           int       `json:"status"`
	IsHumanTakenOver bool      `json:"is_human_taken_over"`
	HumanTakenOverAt int64     `json:"human_taken_over_at"`
	HumanTakenOverBy string    `json:"human_taken_over_by"`
	CreatedTime      int64     `json:"created_time"`
	UpdatedTime      int64     `json:"updated_time"`
}

type QryTicketsResp struct {
	Items []*TicketInfo `json:"items"`
	Total int64         `json:"total"`
	Limit int64         `json:"limit"`
	Offset int64        `json:"offset"`
}

type QryTicketInboxMembersReq struct {
	TicketId string
	StartId  int64
	Limit    int64
}

type TicketInboxMemberInfo struct {
	UserId   string `json:"user_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
}

type QryTicketInboxMembersResp struct {
	Items       []*TicketInboxMemberInfo `json:"items"`
	NextStartId int64                    `json:"next_start_id"`
}

type QryCustomerTicketsReq struct {
	CustomerId       string
	IsHumanTakenOver *bool
	StartId          int64
	Limit            int64
}

type QryCustomerTicketsResp struct {
	Items       []*TicketInfo `json:"items"`
	NextStartId int64         `json:"next_start_id"`
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
