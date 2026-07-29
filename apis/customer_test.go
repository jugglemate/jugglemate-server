package apis

import "testing"

func TestQryCustomerInfoRequiresTicketId(t *testing.T) {
	if got := qryCustomerInfoTicketId(newTicketTestContext("/customers/info?ticket_id=ticket_1")); got != "ticket_1" {
		t.Fatalf("ticket_id = %q, want ticket_1", got)
	}
	if got := qryCustomerInfoTicketId(newTicketTestContext("/customers/info?ticketId=ticket_2")); got != "ticket_2" {
		t.Fatalf("ticketId = %q, want ticket_2", got)
	}
	if got := qryCustomerInfoTicketId(newTicketTestContext("/customers/info?customer_id=customer_1")); got != "" {
		t.Fatalf("customer_id unexpectedly accepted as %q", got)
	}
}

func TestParseQryCustomerTicketsReq(t *testing.T) {
	req, ok := parseQryCustomerTicketsReq(newTicketTestContext("/customers/tickets?customer_id=customer_1"))
	if !ok || req.CustomerId != "customer_1" || req.StartId != 0 || req.Limit != 20 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}
	if req.IsHumanTakenOver != nil {
		t.Fatalf("IsHumanTakenOver = %v, want nil", req.IsHumanTakenOver)
	}

	req, ok = parseQryCustomerTicketsReq(newTicketTestContext("/customers/tickets?customerId=customer_1&start_id=99&limit=100&is_human_taken_over=true"))
	if !ok || req.CustomerId != "customer_1" || req.StartId != 99 || req.Limit != 100 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}
	if req.IsHumanTakenOver == nil || !*req.IsHumanTakenOver {
		t.Fatalf("IsHumanTakenOver = %v, want true", req.IsHumanTakenOver)
	}
}

func TestParseQryCustomerTicketsReqRejectsInvalidHumanTakeOver(t *testing.T) {
	if _, ok := parseQryCustomerTicketsReq(newTicketTestContext("/customers/tickets?customer_id=customer_1&is_human_taken_over=xyz")); ok {
		t.Fatalf("invalid human_takeover flag should be rejected")
	}
}

func TestParseQryCustomerTicketsReqRejectsInvalidPagination(t *testing.T) {
	for _, target := range []string{
		"/customers/tickets",
		"/customers/tickets?customer_id=customer_1&limit=0",
		"/customers/tickets?customer_id=customer_1&limit=101",
		"/customers/tickets?customer_id=customer_1&limit=abc",
		"/customers/tickets?customer_id=customer_1&start_id=-1",
		"/customers/tickets?customer_id=customer_1&start_id=abc",
	} {
		if req, ok := parseQryCustomerTicketsReq(newTicketTestContext(target)); ok {
			t.Fatalf("target=%q req=%+v, want invalid", target, req)
		}
	}
}
