package models

// TicketRatingSource 枚举评价来源。
type TicketRatingSource string

const (
	TicketRatingSourceCardButton        TicketRatingSource = "card_button"
	TicketRatingSourceCardWithComment   TicketRatingSource = "card_with_comment"
)

func IsValidTicketRatingSource(s string) bool {
	switch TicketRatingSource(s) {
	case TicketRatingSourceCardButton, TicketRatingSourceCardWithComment:
		return true
	default:
		return false
	}
}

// TicketRating 客户对工单的人工服务质量评价。
//
// TIPS: UNIQUE (app_key, ticket_id, customer_id) 保证同一客户对同一工单只能评一次。
// 评论建议 ≤ 500 字符；<= 3 分时建议非空（前端 UI 引导，后端不强制）。
type TicketRating struct {
	ID          int64
	AppKey      string
	TicketId    string
	CustomerId  string
	AssigneeId  string
	Rating      int
	Comment     string
	Source      TicketRatingSource
	CreatedTime int64
}

type ITicketRatingStorage interface {
	// Create 写评价；UNIQUE 冲突返回 ErrTicketRatingAlreadyExists（具体错误
	// 由实现层映射 GORM/MySQL duplicate error）。
	Create(item TicketRating) error
	// FindByTicketAndCustomer 单条查询，给 webhook 重放判断 + GET 接口用。
	FindByTicketAndCustomer(appkey, ticketId, customerId string) (*TicketRating, error)
	// QryCsatByAssigneeAndTime 给 stats 接口聚合 csat_avg 与 rating_count。
	QryCsatByAssigneeAndTime(appkey string, startMs, endMs int64) (map[string]CsatAggregate, error)
}

// CsatAggregate 是 stats 接口用的评价聚合结果。
type CsatAggregate struct {
	AssigneeID  string
	CsatAvg     *float64
	RatingCount int64
}
