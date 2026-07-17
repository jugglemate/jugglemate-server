package models

import (
	"encoding/json"
	"testing"
)

func TestCustomerInfoJSONFields(t *testing.T) {
	data, err := json.Marshal(CustomerInfo{
		CustomerId: "customer_1",
		SourceId:   "source_1",
		Nickname:   "Alice",
		Avatar:     "alice.png",
	})
	if err != nil {
		t.Fatalf("marshal customer info: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal customer info: %v", err)
	}
	if fields["customer_id"] != "customer_1" || fields["source_id"] != "source_1" {
		t.Fatalf("customer info fields = %v", fields)
	}
	if _, exists := fields["id"]; exists {
		t.Fatalf("customer info unexpectedly contains id: %v", fields)
	}
}
