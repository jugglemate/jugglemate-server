package apis

import (
	"testing"
)

func TestParseQryTicketEventsReqDefaults(t *testing.T) {
	req, ok := parseQryTicketEventsReq(newTicketTestContext("/tickets/t_1/events"))
	if !ok || req.Limit != 50 || req.Offset != 0 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}
}

func TestParseQryTicketEventsReqPagination(t *testing.T) {
	req, ok := parseQryTicketEventsReq(newTicketTestContext("/tickets/t_1/events?limit=20&offset=10"))
	if !ok || req.Limit != 20 || req.Offset != 10 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}
}

func TestParseQryTicketEventsReqRejectsInvalid(t *testing.T) {
	for _, target := range []string{
		"/tickets/t_1/events?limit=0",
		"/tickets/t_1/events?limit=101",
		"/tickets/t_1/events?offset=-1",
		"/tickets/t_1/events?limit=abc",
	} {
		if _, ok := parseQryTicketEventsReq(newTicketTestContext(target)); ok {
			t.Fatalf("target=%q should be invalid", target)
		}
	}
}
