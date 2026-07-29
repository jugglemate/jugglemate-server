package models

type QryTicketEventsReq struct {
	TicketId string
	Limit    int64
	Offset   int64
}

type TicketEventInfo struct {
	ID           int64  `json:"id"`
	TicketId     string `json:"ticket_id"`
	EventType    string `json:"event_type"`
	OperatorID   string `json:"operator_id"`
	OperatorType string `json:"operator_type"`
	Payload      string `json:"payload"`
	CreatedTime  int64  `json:"created_time"`
}

type QryTicketEventsResp struct {
	Items       []*TicketEventInfo `json:"items"`
	NextStartId int64              `json:"next_start_id"`
}
