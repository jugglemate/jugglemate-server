package apis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseQryTicketsReqDefaultPagination(t *testing.T) {
	ctx := newTicketTestContext("/tickets/list")
	req, ok := parseQryTicketsReq(ctx)
	if !ok {
		t.Fatalf("parseQryTicketsReq returned false")
	}
	if req.Status != nil {
		t.Fatalf("Status = %v, want nil", *req.Status)
	}
	if req.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", req.Limit)
	}
	if req.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", req.Offset)
	}
}

func TestParseQryTicketsReqExplicitPagination(t *testing.T) {
	ctx := newTicketTestContext("/tickets/list?status=1&limit=10&offset=20")
	req, ok := parseQryTicketsReq(ctx)
	if !ok {
		t.Fatalf("parseQryTicketsReq returned false")
	}
	if req.Status == nil || *req.Status != 1 {
		t.Fatalf("Status = %v, want 1", req.Status)
	}
	if req.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", req.Limit)
	}
	if req.Offset != 20 {
		t.Fatalf("Offset = %d, want 20", req.Offset)
	}
}

func TestParseQryTicketsReqInvalidStatus(t *testing.T) {
	for _, target := range []string{
		"/tickets/list?status=-1",
		"/tickets/list?status=3",
		"/tickets/list?status=abc",
	} {
		ctx := newTicketTestContext(target)
		if _, ok := parseQryTicketsReq(ctx); ok {
			t.Fatalf("parseQryTicketsReq(%q) returned true, want false", target)
		}
	}
}

func TestParseQryTicketInboxMembersReq(t *testing.T) {
	req, ok := parseQryTicketInboxMembersReq(newTicketTestContext("/tickets/inboxmembers/list?ticket_id=t_1"))
	if !ok || req.TicketId != "t_1" || req.StartId != 0 || req.Limit != 50 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}

	req, ok = parseQryTicketInboxMembersReq(newTicketTestContext("/tickets/inboxmembers/list?ticketId=t_2&start_id=99&limit=100"))
	if !ok || req.TicketId != "t_2" || req.StartId != 99 || req.Limit != 100 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}
}

func TestParseQryTicketInboxMembersReqRejectsInvalidPagination(t *testing.T) {
	for _, target := range []string{
		"/tickets/inboxmembers/list",
		"/tickets/inboxmembers/list?ticket_id=t_1&start_id=-1",
		"/tickets/inboxmembers/list?ticket_id=t_1&start_id=abc",
		"/tickets/inboxmembers/list?ticket_id=t_1&limit=0",
		"/tickets/inboxmembers/list?ticket_id=t_1&limit=101",
		"/tickets/inboxmembers/list?ticket_id=t_1&limit=abc",
	} {
		if req, ok := parseQryTicketInboxMembersReq(newTicketTestContext(target)); ok {
			t.Fatalf("target=%q req=%+v, want invalid", target, req)
		}
	}
}

func newTicketTestContext(target string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Request = req
	return ctx
}
