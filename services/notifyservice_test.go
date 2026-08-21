package services

import (
	"encoding/json"
	"testing"
)

func TestAssignTypeValues(t *testing.T) {
	tests := []struct {
		name string
		got  AssignType
		want AssignType
	}{
		{"claim", AssignType_Claim, 0},
		{"assign user", AssignType_AssignUser, 1},
		{"assign team", AssignType_AssignTeam, 2},
		{"human takeover", AssignType_HumanTakeover, 3},
		{"reopen", AssignType_Reopen, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("assign type = %d, want %d", tt.got, tt.want)
			}
		})
	}
}

func TestTicketReopenNotificationPayload(t *testing.T) {
	raw, err := json.Marshal(&TicketAssignedNtfMsg{TicketId: "ticket_1", AssignType: AssignType_Reopen})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ticket_id"] != "ticket_1" || payload["assign_type"] != float64(4) {
		t.Fatalf("payload = %s", raw)
	}
	if payload["operator"] != nil || payload["assignee"] != nil {
		t.Fatalf("reopen operator/assignee should be null: %s", raw)
	}
}
