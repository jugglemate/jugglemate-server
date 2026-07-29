package models

// OverviewTotals 是客服数据统计概览的总量指标。
//
// 注意：TransferredToHuman 在 Phase 1 固定返回 0，配合 TransferredToHumanPending=true
// 让前端可以先渲染 UI 骨架，等 Phase 2 加 tickets.handover_at 列后再切到真实值。
type OverviewTotals struct {
	TotalSessions             int64   `json:"total_sessions"`
	AIResolvedSessions        int64   `json:"ai_resolved_sessions"`
	AIResolutionRate          float64 `json:"ai_resolution_rate"`
	TransferredToHuman        int64   `json:"transferred_to_human"`
	TransferredToHumanPending bool    `json:"transferred_to_human_pending"`
	OpenSessions              int64   `json:"open_sessions"`
	ClosedSessions            int64   `json:"closed_sessions"`
}

// OverviewTrendBucket 是时间序列里的一个桶。
//
// Bucket 字段由 service 层按 Granularity 格式化为
//   - day  → "YYYY-MM-DD"
//   - hour → "YYYY-MM-DD HH"
type OverviewTrendBucket struct {
	Bucket             string `json:"bucket"`
	Total              int64  `json:"total"`
	AIResolved         int64  `json:"ai_resolved"`
	TransferredToHuman int64  `json:"transferred_to_human"`
}

// OverviewChannelRank 是渠道访问量排行里的一项，按 count 倒序。
type OverviewChannelRank struct {
	ChannelType string  `json:"channel_type"`
	Count       int64   `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// OverviewResp 是 GET /jmate/console/stats/overview 的 data 字段。
type OverviewResp struct {
	From             string                 `json:"from"`
	To               string                 `json:"to"`
	Granularity      string                 `json:"granularity"`
	Totals           OverviewTotals         `json:"totals"`
	NewSessionsTrend []*OverviewTrendBucket `json:"new_sessions_trend"`
	ChannelRanking   []*OverviewChannelRank `json:"channel_ranking"`
}

// AgentStatsItem 是坐席列表的一项。
//
// CSATAvg / FRT Avg 在 Phase 1 固定为 nil，配合对应 *_pending 字段让前端识别"暂未支持"。
type AgentStatsItem struct {
	UserID            string   `json:"user_id"`
	LoginAccount      string   `json:"login_account"`
	Nickname          string   `json:"nickname"`
	Avatar            string   `json:"avatar"`
	Email             string   `json:"email"`
	Role              int      `json:"role"`
	InboxCount        int64    `json:"inbox_count"`
	TotalSessions     int64    `json:"total_sessions"`
	RespondedSessions int64    `json:"responded_sessions"`
	OpenSessions      int64    `json:"open_sessions"`
	CSATAvg           *float64 `json:"csat_avg"`
	FRTAvgMs          *int64   `json:"frt_avg_ms"`
	CSATPending       bool     `json:"csat_pending"`
	FRTPending        bool     `json:"frt_pending"`
}

// AgentsResp 是 GET /jmate/console/stats/agents 的 data 字段。
type AgentsResp struct {
	Items  []*AgentStatsItem `json:"items"`
	Total  int64             `json:"total"`
	Limit  int64             `json:"limit"`
	Offset int64             `json:"offset"`
}

// CustomerStatsItem 是客户列表的一项。
//
// Source 是该客户最近一次会话所属的渠道类型；可能为空（表示客户从未发起会话）。
type CustomerStatsItem struct {
	CustomerID      string `json:"customer_id"`
	Nickname        string `json:"nickname"`
	Avatar          string `json:"avatar"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	Identifier      string `json:"identifier"`
	Source          string `json:"source"`
	SessionCount    int64  `json:"session_count"`
	LastSessionTime int64  `json:"last_session_time"`
}

// CustomersResp 是 GET /jmate/console/stats/customers 的 data 字段。
type CustomersResp struct {
	Items  []*CustomerStatsItem `json:"items"`
	Total  int64                `json:"total"`
	Limit  int64                `json:"limit"`
	Offset int64                `json:"offset"`
}
