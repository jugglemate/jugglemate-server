package services

import (
	"context"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/ctxs"
	"github.com/juggleim/jugglemate-server/commons/errs"
)

// seatsTestContext 构造最小可用 ctx。
func seatsTestContext(appkey string) context.Context {
	ctx := context.Background()
	if appkey != "" {
		ctx = context.WithValue(ctx, ctxs.CtxKey_AppKey, appkey)
	}
	return ctx
}

// ----------------------------------------------------------------------
// ParseRoleString / ParseSeatStatus —— 纯函数测试
// ----------------------------------------------------------------------

func TestParseRoleString(t *testing.T) {
	cases := []struct {
		in        string
		want      int
		wantErr   bool
	}{
		{"", seatFilterDisabled, false},
		{"customer_service", 1, false},
		{"admin", 2, false},
		{"ADMIN", 2, false},
		{" Customer_Service ", 1, false},
		{"customer-service", 0, true},
		{"root", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseRoleString(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ParseRoleString(%q) err=%v wantErr=%v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Fatalf("ParseRoleString(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseSeatStatus(t *testing.T) {
	v, ok, err := ParseSeatStatus("")
	if err != nil || ok || v != seatFilterDisabled {
		t.Fatalf("empty: v=%d ok=%v err=%v want sentinel", v, ok, err)
	}
	v, ok, err = ParseSeatStatus("0")
	if err != nil || !ok || v != 0 {
		t.Fatalf("0: v=%d ok=%v err=%v", v, ok, err)
	}
	v, ok, err = ParseSeatStatus("1")
	if err != nil || !ok || v != 1 {
		t.Fatalf("1: v=%d ok=%v err=%v", v, ok, err)
	}
	if _, _, err := ParseSeatStatus("2"); err == nil {
		t.Fatalf("2 should fail")
	}
	if _, _, err := ParseSeatStatus("abc"); err == nil {
		t.Fatalf("abc should fail")
	}
}

func TestParseInboxIDs(t *testing.T) {
	s := "{a,b,c}"
	if got := parseInboxIDs(&s); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("got %v", got)
	}
	if got := parseInboxIDs(nil); len(got) != 0 {
		t.Fatalf("nil should return empty slice, got %v", got)
	}
	empty := "{}"
	if got := parseInboxIDs(&empty); len(got) != 0 {
		t.Fatalf("{} should return empty slice, got %v", got)
	}
	quoted := `{"a","b"}`
	if got := parseInboxIDs(&quoted); len(got) != 2 || got[0] != "a" {
		t.Fatalf("quoted got %v", got)
	}
}

// ----------------------------------------------------------------------
// ListSeats —— ctx 入口检查
// ----------------------------------------------------------------------

func TestListSeatsRequiresAppKey(t *testing.T) {
	code, resp := ListSeats(seatsTestContext(""), "", seatFilterDisabled, "", seatFilterDisabled, 20, 0)
	if code != errs.IMErrorCode_APP_NOT_LOGIN {
		t.Fatalf("code = %d, want APP_NOT_LOGIN", code)
	}
	if resp != nil {
		t.Fatalf("resp should be nil on error")
	}
}
