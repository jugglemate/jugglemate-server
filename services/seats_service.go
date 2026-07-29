package services

import (
	"context"
	"errors"
	"strings"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/dbcommons"
	"github.com/juggleim/jugglemate-server/commons/errs"
	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

// 静态校验：保证 storageModels 引用仍参与编译，避免引入死代码报警。
var _ = storageModels.UserRoleCustomerService
var _ = storageModels.UserRoleAdmin

// errInvalidRoleString 用于 role 查询参数非法。
var errInvalidRoleString = errors.New("role 仅支持 customer_service / admin")

// errInvalidSeatStatus 用于 status 查询参数非法。
var errInvalidSeatStatus = errors.New("status 仅支持 0 / 1")

// seatFilterDisabled 是「不过滤」哨兵值（用 -1 表示 role/status 不启用）。
const seatFilterDisabled = -1

// ParseRoleString 把 role 字符串规整为内部整数（1=客服，2=admin）；空字符串返回 seatFilterDisabled。
func ParseRoleString(s string) (int, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "":
		return seatFilterDisabled, nil
	case "customer_service":
		return int(storageModels.UserRoleCustomerService), nil
	case "admin":
		return int(storageModels.UserRoleAdmin), nil
	default:
		return 0, errInvalidRoleString
	}
}

// ParseSeatStatus 把 status 字符串规整为整数（0/1）；非法返回 errInvalidSeatStatus。
//
// 输入：0 / 1 / 空
// 空字符串返回 (seatFilterDisabled, false, nil) 表示不过滤。
func ParseSeatStatus(s string) (int, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return seatFilterDisabled, false, nil
	}
	switch s {
	case "0":
		return 0, true, nil
	case "1":
		return 1, true, nil
	default:
		return 0, false, errInvalidSeatStatus
	}
}

// ListSeats 是 GET /jmate/seats/list 的业务实现。
//
// 入参：
//   - keyword        模糊匹配 login_account / nickname / email / user_id（PG ILIKE '%kw%'）
//   - roleFilter     seatFilterDisabled (-1) 表示不过滤；1=客服；2=admin
//   - inboxID        空表示不过滤；非空只列绑定了该 Inbox 的坐席
//   - statusFilter   seatFilterDisabled (-1) 表示不过滤；0=正常；1=禁用
//   - limit / offset 分页（上限 100，handler 侧已夹紧）
//
// 出参：SeatsListResp + 错误码。
func ListSeats(ctx context.Context, keyword string, roleFilter int, inboxID string, statusFilter int, limit, offset int64) (errs.IMErrorCode, *apiModels.SeatsListResp) {
	appkey := ctxs.GetAppKeyFromCtx(ctx)
	if appkey == "" {
		return errs.IMErrorCode_APP_NOT_LOGIN, nil
	}

	db := dbcommons.GetDb().WithContext(ctx)
	if db == nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	kw := strings.TrimSpace(keyword)
	likeKw := "%" + kw + "%"
	inbox := strings.TrimSpace(inboxID)

	type row struct {
		UserID           string
		LoginAccount     string
		Nickname         string
		Avator           string
		Email            string
		Role             int
		Status           int
		IMToken          string
		CreatedTime      *int64
		InboxIDs         *string
		InboxCount       int64
		OpenSessionCount int64
		LastActiveTime   *int64
	}

	// 列查询
	//   占位策略：用「(? = <sentinel> OR 真值 = ?)」关闭过滤器。
	//   sentinel = -1 表示不启用；比 NULL / IS FALSE 写法更稳。
	query := `
		WITH seat_inboxes AS (
			SELECT member_id,
			       array_agg(inbox_id ORDER BY inbox_id) AS inbox_ids,
			       COUNT(*) AS inbox_count
			FROM inboxmembers
			WHERE app_key = ?
			GROUP BY member_id
		),
		open_sessions AS (
			SELECT assignee_id,
			       COUNT(*) AS open_session_count,
			       MAX(updated_time) AS last_active_time
			FROM tickets
			WHERE app_key = ?
			  AND assignee_id IS NOT NULL AND assignee_id <> ''
			  AND status IN (0,1,3)
			GROUP BY assignee_id
		)
		SELECT
		  u.user_id, u.login_account, u.nickname, u.avator, u.email,
		  u.role, u.status, u.im_token,
		  (EXTRACT(EPOCH FROM u.created_time) * 1000)::BIGINT AS created_time,
		  si.inbox_ids,
		  COALESCE(si.inbox_count, 0) AS inbox_count,
		  COALESCE(os.open_session_count, 0) AS open_session_count,
		  (EXTRACT(EPOCH FROM os.last_active_time) * 1000)::BIGINT AS last_active_time
		FROM users u
		LEFT JOIN seat_inboxes si ON si.member_id = u.user_id
		LEFT JOIN open_sessions os ON os.assignee_id = u.user_id
		WHERE u.app_key = ?
		  AND (? = ''
		       OR u.login_account ILIKE ?
		       OR u.nickname ILIKE ?
		       OR u.email ILIKE ?
		       OR u.user_id ILIKE ?)
		  AND (? = -1 OR u.role = ?)
		  AND (? = '' OR EXISTS (
		      SELECT 1 FROM inboxmembers m
		      WHERE m.app_key = u.app_key AND m.member_id = u.user_id AND m.inbox_id = ?))
		  AND (? = -1 OR u.status = ?)
		ORDER BY u.status ASC, inbox_count DESC, u.login_account ASC
		LIMIT ? OFFSET ?`

	// placeholder 出现顺序（16 个）：
	//   1  seat_inboxes 内 app_key
	//   2  open_sessions 内 app_key
	//   3  u.app_key
	//   4  kw 空判断
	//   5-8 likeKw × 4（login_account / nickname / email / user_id）
	//   9  role 哨兵 (-1)
	//   10 role 值
	//   11 inbox 空判断
	//   12 EXISTS inbox_id
	//   13 status 哨兵 (-1)
	//   14 status 值
	//   15 LIMIT
	//   16 OFFSET
	var rows []row
	if err := db.Raw(query,
		appkey,
		appkey,
		appkey,
		kw,
		likeKw, likeKw, likeKw, likeKw,
		roleFilter, roleFilter,
		inbox, inbox,
		statusFilter, statusFilter,
		limit, offset,
	).Scan(&rows).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	// 计数查询（13 个 placeholder）
	countQuery := `
		SELECT COUNT(*)
		FROM users u
		WHERE u.app_key = ?
		  AND (? = ''
		       OR u.login_account ILIKE ?
		       OR u.nickname ILIKE ?
		       OR u.email ILIKE ?
		       OR u.user_id ILIKE ?)
		  AND (? = -1 OR u.role = ?)
		  AND (? = '' OR EXISTS (
		      SELECT 1 FROM inboxmembers m
		      WHERE m.app_key = u.app_key AND m.member_id = u.user_id AND m.inbox_id = ?))
		  AND (? = -1 OR u.status = ?)`

	// placeholder 顺序（13 个）：
	//   1  app_key
	//   2  kw 空判断
	//   3-6 likeKw × 4
	//   7  role 哨兵
	//   8  role 值
	//   9  inbox 空判断
	//   10 EXISTS inbox_id
	//   11 status 哨兵
	//   12 status 值
	var total int64
	if err := db.Raw(countQuery,
		appkey,
		kw,
		likeKw, likeKw, likeKw, likeKw,
		roleFilter, roleFilter,
		inbox, inbox,
		statusFilter, statusFilter,
	).Scan(&total).Error; err != nil {
		return errs.IMErrorCode_APP_INTERNAL_TIMEOUT, nil
	}

	items := make([]*apiModels.SeatItem, 0, len(rows))
	for _, r := range rows {
		item := &apiModels.SeatItem{
			UserID:           r.UserID,
			LoginAccount:     r.LoginAccount,
			Nickname:         r.Nickname,
			Avatar:           r.Avator,
			Email:            r.Email,
			Status:           r.Status,
			IMTokenPresent:   strings.TrimSpace(r.IMToken) != "",
			InboxIDs:         parseInboxIDs(r.InboxIDs),
			InboxCount:       r.InboxCount,
			OpenSessionCount: r.OpenSessionCount,
		}
		switch r.Role {
		case int(storageModels.UserRoleCustomerService):
			item.Role = "customer_service"
		case int(storageModels.UserRoleAdmin):
			item.Role = "admin"
		default:
			item.Role = ""
		}
		if r.CreatedTime != nil {
			item.CreatedTime = *r.CreatedTime
		}
		if r.LastActiveTime != nil {
			item.LastActiveTime = *r.LastActiveTime
		} else if r.CreatedTime != nil {
			// 兜底：从未被分配过工单的坐席，用 created_time 作为最近活跃
			item.LastActiveTime = *r.CreatedTime
		}
		items = append(items, item)
	}
	return errs.IMErrorCode_SUCCESS, &apiModels.SeatsListResp{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// parseInboxIDs 把 PG array_agg 返回的字面量文本解析为 []string。
//
// input: nil → 空切片；"{}" → 空切片；"{a,b,c}" → ["a","b","c"]
func parseInboxIDs(p *string) []string {
	if p == nil {
		return []string{}
	}
	s := *p
	if s == "" || s == "{}" {
		return []string{}
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"`)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
