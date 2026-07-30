package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
	"gorm.io/gorm"
)

// shanghaiLocation 用于时间窗字符串解析，避免依赖 /usr/share/zoneinfo。
var shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*3600)

// errInvalidTimeRange 用于时间窗参数格式错误。
var errInvalidTimeRange = errors.New("时间窗参数格式错误（期望 YYYY-MM-DD）")

// errInvalidGranularity 用于 granularity 不是 day/hour 的情况。
var errInvalidGranularity = errors.New("granularity 仅支持 day 或 hour")

// ParseTimeRange 把 from/to 字符串规整为 [from, to+1day) 的绝对时间区间。
//
// 当 from 或 to 为空时，使用 to=今天 / from=to-7d 的默认值；
// 当 from > to 时返回 errInvalidTimeRange。
//
// 输出是 [start, end) 闭开区间，便于 SQL 端用 >= 与 < 比较；同时保留
// From/To 字符串给响应体回显。
func ParseTimeRange(fromStr, toStr string) (from string, to string, start time.Time, end time.Time, err error) {
	toStr = strings.TrimSpace(toStr)
	fromStr = strings.TrimSpace(fromStr)
	now := time.Now().In(shanghaiLocation)
	defaultTo := now.Format("2006-01-02")
	if toStr == "" {
		toStr = defaultTo
	}
	toTime, parseErr := time.ParseInLocation("2006-01-02", toStr, shanghaiLocation)
	if parseErr != nil {
		return "", "", time.Time{}, time.Time{}, errInvalidTimeRange
	}
	// fromTime 在两条分支里都需要，先声明便于函数末尾统一返回。
	var fromTime time.Time
	if fromStr == "" {
		fromTime = toTime.AddDate(0, 0, -7)
		fromStr = fromTime.Format("2006-01-02")
	} else {
		parsed, ferr := time.ParseInLocation("2006-01-02", fromStr, shanghaiLocation)
		if ferr != nil {
			return "", "", time.Time{}, time.Time{}, errInvalidTimeRange
		}
		if parsed.After(toTime) {
			return "", "", time.Time{}, time.Time{}, errInvalidTimeRange
		}
		fromTime = parsed
		fromStr = fromTime.Format("2006-01-02")
	}
	// to 取下一天 0 点，让 end 区间半开半闭
	endTime := toTime.AddDate(0, 0, 1)
	return fromStr, toStr, fromTime, endTime, nil
}

// ParseGranularity 把字符串规整为 day 或 hour，非法时返回 errInvalidGranularity。
func ParseGranularity(g string) (string, error) {
	g = strings.TrimSpace(strings.ToLower(g))
	switch g {
	case "", "day":
		return "day", nil
	case "hour":
		return "hour", nil
	default:
		return "", errInvalidGranularity
	}
}

// GetStatsOverview 返回客服数据统计概览的全部数据。
//
// 三个子查询独立执行：totals / trend / channel_ranking；
// 时间窗统一通过 tickets.created_time 过滤，转人工计数基于 is_human_taken_over 字段
// —— 与 total_sessions / ai_resolved_sessions 保持同口径"时间窗内新建工单中…"。
// 历史（此字段上线前的）工单 is_human_taken_over 默认为 false，因此上线后转人工数从 0
// 开始累加是预期行为，不算回填。
func GetStatsOverview(ctx context.Context, fromStr, toStr, granularity string) (errs.IMErrorCode, *apiModels.OverviewResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	from, to, start, end, err := ParseTimeRange(fromStr, toStr)
	if err != nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	g, gerr := ParseGranularity(granularity)
	if gerr != nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}

	db := dbcommons.GetDb().WithContext(ctx)
	if db == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	totals, terr := loadOverviewTotals(db, appkey, start, end)
	if terr != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	trend, terr := loadOverviewTrend(db, appkey, start, end, g)
	if terr != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	ranking, terr := loadOverviewChannelRanking(db, appkey, start, end)
	if terr != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	return errs.IMErrorCode_SUCCESS, &apiModels.OverviewResp{
		From:             from,
		To:               to,
		Granularity:      g,
		Totals:           totals,
		NewSessionsTrend: trend,
		ChannelRanking:   ranking,
	}
}

// loadOverviewTotals 一次查询拉全部总量；用 FILTER 替代多条 SQL。
func loadOverviewTotals(db *gorm.DB, appkey string, start, end time.Time) (apiModels.OverviewTotals, error) {
	var row struct {
		TotalSessions       int64
		AIResolvedSessions  int64
		OpenSessions        int64
		ClosedSessions      int64
		TransferredToHuman  int64
	}
	// 注意：FILTER 语法是 PostgreSQL 特性。
	// 转人工计数基于 is_human_taken_over 字段；与 total_sessions / ai_resolved 同口径
	// （时间窗内新建工单中曾转人工的工单数）。
	query := `
		SELECT
		  COUNT(*)                                                                  AS total_sessions,
		  COUNT(*) FILTER (WHERE status=2 AND (assignee_id IS NULL OR assignee_id='')) AS ai_resolved_sessions,
		  COUNT(*) FILTER (WHERE status IN (0,1,3))                                  AS open_sessions,
		  COUNT(*) FILTER (WHERE status=2)                                           AS closed_sessions,
		  COUNT(*) FILTER (WHERE is_human_taken_over)                                AS transferred_to_human
		FROM tickets
		WHERE app_key=? AND created_time>=? AND created_time<?`
	if err := db.Raw(query, appkey, start, end).Scan(&row).Error; err != nil {
		return apiModels.OverviewTotals{}, err
	}
	totals := apiModels.OverviewTotals{
		TotalSessions:             row.TotalSessions,
		AIResolvedSessions:        row.AIResolvedSessions,
		OpenSessions:              row.OpenSessions,
		ClosedSessions:            row.ClosedSessions,
		TransferredToHuman:        row.TransferredToHuman,
		TransferredToHumanPending: false,
	}
	if totals.TotalSessions > 0 {
		totals.AIResolutionRate = float64(totals.AIResolvedSessions) * 100 / float64(totals.TotalSessions)
		// 保留 2 位小数
		totals.AIResolutionRate = float64(int(totals.AIResolutionRate*100+0.5)) / 100
	}
	return totals, nil
}

// loadOverviewTrend 按 Granularity 取时间序列。
//
// day: date_trunc('day', created_time)
// hour: date_trunc('hour', created_time)
//
// 桶标签在 Go 侧格式化，避免 PG 端 to_char 引发的格式差异。
func loadOverviewTrend(db *gorm.DB, appkey string, start, end time.Time, granularity string) ([]*apiModels.OverviewTrendBucket, error) {
	trunc := "day"
	labelLayout := "2006-01-02"
	if granularity == "hour" {
		trunc = "hour"
		labelLayout = "2006-01-02 15"
	}
	type row struct {
		Bucket            time.Time
		Total             int64
		AIResolved        int64
		TransferredToHuman int64
	}
	var rows []row
	// 转人工计数基于 is_human_taken_over，桶切分沿用 created_time（与 total / ai_resolved 同口径）。
	query := fmt.Sprintf(`
		SELECT
		  date_trunc('%s', created_time) AS bucket,
		  COUNT(*) AS total,
		  COUNT(*) FILTER (WHERE status=2 AND (assignee_id IS NULL OR assignee_id='')) AS ai_resolved,
		  COUNT(*) FILTER (WHERE is_human_taken_over) AS transferred_to_human
		FROM tickets
		WHERE app_key=? AND created_time>=? AND created_time<?
		GROUP BY bucket
		ORDER BY bucket`, trunc)
	if err := db.Raw(query, appkey, start, end).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*apiModels.OverviewTrendBucket, 0, len(rows))
	for _, r := range rows {
		out = append(out, &apiModels.OverviewTrendBucket{
			Bucket:             r.Bucket.In(shanghaiLocation).Format(labelLayout),
			Total:              r.Total,
			AIResolved:         r.AIResolved,
			TransferredToHuman: r.TransferredToHuman,
		})
	}
	return out, nil
}

// loadOverviewChannelRanking 取渠道访问量排行 + 占比。
func loadOverviewChannelRanking(db *gorm.DB, appkey string, start, end time.Time) ([]*apiModels.OverviewChannelRank, error) {
	type row struct {
		ChannelType string
		Count       int64
	}
	var rows []row
	query := `
		SELECT channel_type, COUNT(*) AS count
		FROM tickets
		WHERE app_key=? AND created_time>=? AND created_time<?
		GROUP BY channel_type
		ORDER BY count DESC`
	if err := db.Raw(query, appkey, start, end).Scan(&rows).Error; err != nil {
		return nil, err
	}
	var total int64
	for _, r := range rows {
		total += r.Count
	}
	out := make([]*apiModels.OverviewChannelRank, 0, len(rows))
	for _, r := range rows {
		item := &apiModels.OverviewChannelRank{
			ChannelType: r.ChannelType,
			Count:       r.Count,
		}
		if total > 0 {
			p := float64(r.Count) * 100 / float64(total)
			item.Percentage = float64(int(p*100+0.5)) / 100
		}
		out = append(out, item)
	}
	return out, nil
}

// GetStatsAgents 返回坐席列表。
//
// includeAdmin=false 时只列 role=1 的纯客服；true 同时纳入 admin。
// 分页用 limit/offset，已在 handler 侧把 page 转成 offset。
func GetStatsAgents(ctx context.Context, fromStr, toStr string, includeAdmin bool, limit, offset int64) (errs.IMErrorCode, *apiModels.AgentsResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	_, _, start, end, err := ParseTimeRange(fromStr, toStr)
	if err != nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	db := dbcommons.GetDb().WithContext(ctx)
	if db == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	roleFilter := "u.role IN (1,2)"
	if !includeAdmin {
		roleFilter = "u.role = 1"
	}
	query := fmt.Sprintf(`
		SELECT
		  u.user_id, u.login_account, u.nickname, u.avator, u.email, u.role,
		  (SELECT COUNT(*) FROM inboxmembers m
		     WHERE m.app_key=u.app_key AND m.member_id=u.user_id) AS inbox_count,
		  COALESCE(t.total_sessions,    0) AS total_sessions,
		  COALESCE(t.responded_sessions,0) AS responded_sessions,
		  COALESCE(t.open_sessions,     0) AS open_sessions,
		  cr.csat_avg, cr.rating_count
		FROM users u
		LEFT JOIN (
		  SELECT assignee_id,
		         COUNT(*)                                                  AS total_sessions,
		         COUNT(*) FILTER (WHERE status IN (1,2,3))                 AS responded_sessions,
		         COUNT(*) FILTER (WHERE status IN (0,1,3))                 AS open_sessions
		  FROM tickets
		  WHERE app_key=? AND assignee_id IS NOT NULL AND assignee_id<>''
		    AND created_time>=? AND created_time<?
		  GROUP BY assignee_id
		) t ON t.assignee_id = u.user_id
		LEFT JOIN (
		  SELECT assignee_id,
		         AVG(rating)::float AS csat_avg,
		         COUNT(*)           AS rating_count
		  FROM ticket_ratings
		  WHERE app_key=? AND assignee_id<>''
		    AND created_time>=? AND created_time<?
		  GROUP BY assignee_id
		) cr ON cr.assignee_id = u.user_id
		WHERE u.app_key=? AND %s
		ORDER BY t.total_sessions DESC NULLS LAST, u.nickname
		LIMIT ? OFFSET ?`, roleFilter)

	var rows []struct {
		UserID            string
		LoginAccount      string
		Nickname          string
		Avator            string
		Email             string
		Role              int
		InboxCount        int64
		TotalSessions     int64
		RespondedSessions int64
		OpenSessions      int64
		CsatAvg           *float64
		RatingCount       int64
	}
	if err := db.Raw(query, appkey, start, end, appkey, start, end, appkey, limit, offset).Scan(&rows).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	// 总数另算一次：仅 role 过滤不影响时间窗
	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM users WHERE app_key=? AND %s`, strings.ReplaceAll(roleFilter, "u.", ""))
	if err := db.Raw(countQuery, appkey).Scan(&total).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	items := make([]*apiModels.AgentStatsItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, &apiModels.AgentStatsItem{
			UserID:            r.UserID,
			LoginAccount:      r.LoginAccount,
			Nickname:          r.Nickname,
			Avatar:            r.Avator,
			Email:             r.Email,
			Role:              r.Role,
			InboxCount:        r.InboxCount,
			TotalSessions:     r.TotalSessions,
			RespondedSessions: r.RespondedSessions,
			OpenSessions:      r.OpenSessions,
			CSATAvg:           r.CsatAvg, // nil 表示该坐席在时间窗内没有评价
			RatingCount:       r.RatingCount,
			FRTAvgMs:          nil,    // FRT 仍 pending
			CSATPending:       false,  // CSAT 已接通
			FRTPending:        true,
		})
	}
	return errs.IMErrorCode_SUCCESS, &apiModels.AgentsResp{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// GetStatsCustomers 返回客户列表。
//
// 三处子查询（最近渠道、session_count、last_session_time）通过相关子查询
// 表达，比 LEFT JOIN LATERAL 在 PG 14 以下兼容性更好。
// keyword / channel_type 在外层 WHERE 过滤以控制总量。
func GetStatsCustomers(ctx context.Context, fromStr, toStr, keyword, channelType string, limit, offset int64) (errs.IMErrorCode, *apiModels.CustomersResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}
	_, _, start, end, err := ParseTimeRange(fromStr, toStr)
	if err != nil {
		return errs.IMErrorCode_APP_ParamError, nil
	}
	db := dbcommons.GetDb().WithContext(ctx)
	if db == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}
	kw := strings.TrimSpace(keyword)
	likeKw := "%" + kw + "%"
	ct := strings.TrimSpace(channelType)

	// 列查询
	var itemsRows []struct {
		CustomerID      string
		Nickname        string
		Avator          string
		Phone           string
		Email           string
		Identifier      string
		Source          *string
		SessionCount    int64
		LastSessionTime *time.Time
	}
	itemsQuery := `
		SELECT
		  c.customer_id, c.nickname, c.avator, c.phone, c.email, c.identifier,
		  (SELECT channel_type FROM tickets t
		    WHERE t.app_key=c.app_key AND t.customer_id=c.customer_id
		      AND t.created_time>=? AND t.created_time<?
		    ORDER BY t.created_time DESC LIMIT 1) AS source,
		  (SELECT COUNT(*) FROM tickets t
		    WHERE t.app_key=c.app_key AND t.customer_id=c.customer_id
		      AND t.created_time>=? AND t.created_time<?) AS session_count,
		  (SELECT MAX(t.created_time) FROM tickets t
		    WHERE t.app_key=c.app_key AND t.customer_id=c.customer_id
		      AND t.created_time>=? AND t.created_time<?) AS last_session_time
		FROM customers c
		WHERE c.app_key=?
		  AND (? = '' OR c.nickname ILIKE ?
		           OR c.identifier ILIKE ?
		           OR c.phone ILIKE ?
		           OR c.email ILIKE ?)
		  AND (? = '' OR EXISTS (
		    SELECT 1 FROM tickets t
		    WHERE t.app_key=c.app_key AND t.customer_id=c.customer_id
		      AND t.channel_type=?
		      AND t.created_time>=? AND t.created_time<?
		  ))
		ORDER BY last_session_time DESC NULLS LAST, c.customer_id DESC
		LIMIT ? OFFSET ?`
	// 参数顺序：start,end,start,end,start,end, appkey, kw, like*4, ct, ct, start, end, limit, offset
	if err := db.Raw(itemsQuery,
		start, end,
		start, end,
		start, end,
		appkey,
		kw, likeKw, likeKw, likeKw, likeKw,
		ct, ct, start, end,
		limit, offset,
	).Scan(&itemsRows).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	// 计数查询：与上面同样 WHERE，但去掉 ORDER BY / LIMIT / OFFSET
	countQuery := `
		SELECT COUNT(*)
		FROM customers c
		WHERE c.app_key=?
		  AND (? = '' OR c.nickname ILIKE ?
		           OR c.identifier ILIKE ?
		           OR c.phone ILIKE ?
		           OR c.email ILIKE ?)
		  AND (? = '' OR EXISTS (
		    SELECT 1 FROM tickets t
		    WHERE t.app_key=c.app_key AND t.customer_id=c.customer_id
		      AND t.channel_type=?
		      AND t.created_time>=? AND t.created_time<?
		  ))`
	var total int64
	if err := db.Raw(countQuery,
		appkey,
		kw, likeKw, likeKw, likeKw, likeKw,
		ct, ct, start, end,
	).Scan(&total).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	items := make([]*apiModels.CustomerStatsItem, 0, len(itemsRows))
	for _, r := range itemsRows {
		item := &apiModels.CustomerStatsItem{
			CustomerID:   r.CustomerID,
			Nickname:     r.Nickname,
			Avatar:       r.Avator,
			Phone:        r.Phone,
			Email:        r.Email,
			Identifier:   r.Identifier,
			SessionCount: r.SessionCount,
		}
		if r.Source != nil {
			item.Source = *r.Source
		}
		if r.LastSessionTime != nil {
			item.LastSessionTime = r.LastSessionTime.UnixMilli()
		}
		items = append(items, item)
	}
	return errs.IMErrorCode_SUCCESS, &apiModels.CustomersResp{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// 静态检查：保证 storageModels 不被误删。
var _ = storageModels.Ticket{}
