package apis

import "testing"

func TestParseQryCustomerTicketsReq(t *testing.T) {
	req, ok := parseQryCustomerTicketsReq(newTicketTestContext("/customers/tickets?customer_id=customer_1"))
	if !ok || req.CustomerId != "customer_1" || req.StartId != 0 || req.Limit != 20 {
		t.Fatalf("req=%+v ok=%v", req, ok)
	}

	req, ok = parseQryCustomerTicketsReq(newTicketTestContext("/customers/tickets?customerId=customer_1&start_id=99&limit=100"))
	if !ok || req.CustomerId != "customer_1" || req.StartId != 99 || req.Limit != 100 {
		t.Fatalf("req=%+v ok=%v", req, ok)
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
