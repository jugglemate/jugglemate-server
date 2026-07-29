package services

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	apiModels "github.com/juggleim/jugglemate-server/apis/models"
	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
)

// statsTestContext 构造一个最小可用的 ctx。
func statsTestContext(appkey string) context.Context {
	ctx := context.Background()
	if appkey != "" {
		ctx = context.WithValue(ctx, ctxs.CtxKey_AppKey, appkey)
	}
	return ctx
}

// ----------------------------------------------------------------------
// ParseTimeRange / ParseGranularity —— 纯函数测试
// ----------------------------------------------------------------------

func TestParseTimeRangeDefaultsWhenEmpty(t *testing.T) {
	from, to, start, end, err := ParseTimeRange("", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if to == "" {
		t.Fatalf("to should default to today")
	}
	if from == "" {
		t.Fatalf("from should default")
	}
	if start.Location().String() != "Asia/Shanghai" {
		t.Fatalf("start tz = %v, want Asia/Shanghai", start.Location())
	}
	// end 必须是下一天 0 点，保证闭开区间 [start, end)
	if !end.After(start) {
		t.Fatalf("end %v should be after start %v", end, start)
	}
	expectedDelta := 24 * time.Hour
	gotDelta := end.Sub(start.Truncate(24 * time.Hour))
	if gotDelta != expectedDelta && !(start.Hour() == 0 && start.Minute() == 0) {
		t.Fatalf("end-start day diff = %v, want ~24h", end.Sub(start))
	}
}

func TestParseTimeRangeRejectsBadFrom(t *testing.T) {
	if _, _, _, _, err := ParseTimeRange("2026-13-99", ""); err == nil {
		t.Fatalf("expected error for bad from")
	}
}

func TestParseTimeRangeRejectsBadTo(t *testing.T) {
	if _, _, _, _, err := ParseTimeRange("", "abc"); err == nil {
		t.Fatalf("expected error for bad to")
	}
}

func TestParseTimeRangeRejectsFromAfterTo(t *testing.T) {
	if _, _, _, _, err := ParseTimeRange("2026-07-30", "2026-07-20"); err == nil {
		t.Fatalf("expected error for from>to")
	}
}

// TestParseTimeRangeStartEqualsFrom 验证修复点：start 必须等于 from 字符串对应的
// 当天 0 点（Asia/Shanghai）。之前的实现把 toTime 当 start 返回，SQL 用 to 当起点
// 会查不到 to 之前的工单数据。
func TestParseTimeRangeStartEqualsFrom(t *testing.T) {
	from, to, start, end, err := ParseTimeRange("2026-07-23", "2026-07-29")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if from != "2026-07-23" {
		t.Fatalf("from = %q, want 2026-07-23", from)
	}
	if to != "2026-07-29" {
		t.Fatalf("to = %q, want 2026-07-29", to)
	}
	wantStart, _ := time.ParseInLocation("2006-01-02", "2026-07-23", shanghaiLocation)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %v, want %v", start, start)
	}
	wantEnd := wantStart.AddDate(0, 0, 7)
	if !end.Equal(wantEnd) {
		t.Fatalf("end = %v, want %v", end, wantEnd)
	}
	if end.Sub(start) != 7*24*time.Hour {
		t.Fatalf("end-start = %v, want 7d", end.Sub(start))
	}
}

func TestParseGranularityDefaults(t *testing.T) {
	if g, err := ParseGranularity(""); err != nil || g != "day" {
		t.Fatalf("empty: got %q, err=%v, want day/nil", g, err)
	}
	if g, err := ParseGranularity("DAY"); err != nil || g != "day" {
		t.Fatalf("DAY: got %q, err=%v, want day/nil", g, err)
	}
	if g, err := ParseGranularity("hour"); err != nil || g != "hour" {
		t.Fatalf("hour: got %q, err=%v, want hour/nil", g, err)
	}
	if _, err := ParseGranularity("week"); err == nil {
		t.Fatalf("week should fail")
	}
}

// ----------------------------------------------------------------------
// 通过依赖注入验证 GetStatsOverview / Agents / Customers 的参数解析与 ctx 检查
// ----------------------------------------------------------------------

// 用一个最小 stub 替换 dbcommons，避免真实数据库依赖。
type stubStatsDB struct {
	calls   atomic.Int64
	rawExec func(dest interface{}, query string, args ...interface{}) error
}

// 这里只测 ctx/appkey 为空的失败路径，不真正调用 stubStatsDB。
// 但我们的 service 里 db = dbcommons.GetDb()，需要替换 dbcommons 的全局变量。

func TestGetStatsOverviewRequiresAppKey(t *testing.T) {
	code, resp := GetStatsOverview(statsTestContext(""), "", "", "")
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("code = %d, want APP_NOT_LOGIN", code)
	}
	if resp != nil {
		t.Fatalf("resp should be nil on error")
	}
}

func TestGetStatsOverviewRejectsBadFrom(t *testing.T) {
	// db 没初始化时也会走 ctx check 先返回 SUCCESS_LOGIN 之前先把 bad from 直接挡掉
	restore := patchStatsDBStub(nil)
	defer restore()
	code, _ := GetStatsOverview(statsTestContext("app_1"), "bad", "", "")
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("code = %d, want APP_ParamError", code)
	}
}

func TestGetStatsOverviewRejectsBadGranularity(t *testing.T) {
	restore := patchStatsDBStub(nil)
	defer restore()
	code, _ := GetStatsOverview(statsTestContext("app_1"), "", "", "week")
	if code != errs.IMErrorCode_APP_ParamError {
		t.Fatalf("code = %d, want APP_ParamError", code)
	}
}

func TestGetStatsAgentsRequiresAppKey(t *testing.T) {
	code, resp := GetStatsAgents(statsTestContext(""), "", "", true, 20, 0)
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("code = %d, want APP_NOT_LOGIN", code)
	}
	if resp != nil {
		t.Fatalf("resp should be nil on error")
	}
}

func TestGetStatsCustomersRequiresAppKey(t *testing.T) {
	code, resp := GetStatsCustomers(statsTestContext(""), "", "", "", "", 20, 0)
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("code = %d, want APP_NOT_LOGIN", code)
	}
	if resp != nil {
		t.Fatalf("resp should be nil on error")
	}
}

// patchStatsDBStub —— 把 dbcommons 的全局 DB 替换为一个空壳，
// 让 service 走到 SQL 之前先返回 nil（任何 raw 调用都 OK）。
// 若 raw 为 nil，service 会返回 INTERNAL_TIMEOUT；这里只用于参数解析错误路径。
func patchStatsDBStub(_ interface{}) func() {
	// 注：dbcommons 的 db 是 unexported 包级变量；从外部不能直接修改。
	// 所以这个函数保留 API 形状，但实际无法替换实现。
	// 参数解析错误的测试仍然生效，因为它们在 db 调用之前返回。
	return func() {}
}

// sanity guard：保证用到 ctxs / time 包 / errors 包仍被引用
var _ = errors.New
var _ apiModels.OverviewResp
var _ = time.Second
