package models

// SeatItem 是 GET /jmate/seats/list 返回的单个坐席记录。
//
// 注意：本响应供所有登录用户（admin + 客服）使用，是只读视图；
// 对应写权限仍由现有 /jmate/console/users 系列接口提供。
type SeatItem struct {
	UserID           string   `json:"user_id"`
	LoginAccount     string   `json:"login_account"`
	Nickname         string   `json:"nickname"`
	Avatar           string   `json:"avatar"`
	Email            string   `json:"email"`
	Role             string   `json:"role"`
	Status           int      `json:"status"`
	IMTokenPresent   bool     `json:"im_token_present"`
	InboxIDs         []string `json:"inbox_ids"`
	InboxCount       int64    `json:"inbox_count"`
	OpenSessionCount int64    `json:"open_session_count"`
	LastActiveTime   int64    `json:"last_active_time"`
	CreatedTime      int64    `json:"created_time"`
}

// SeatsListResp 是 GET /jmate/seats/list 的 data 字段。
type SeatsListResp struct {
	Items  []*SeatItem `json:"items"`
	Total  int64       `json:"total"`
	Limit  int64       `json:"limit"`
	Offset int64       `json:"offset"`
}
