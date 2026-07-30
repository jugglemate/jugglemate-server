package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildCsatInvitationPayload(t *testing.T) {
	p := BuildCsatInvitationPayload("app_1", "ticket_abc", 1785000000000)
	if p.Kind != "csat" {
		t.Fatalf("kind = %q, want csat", p.Kind)
	}
	if p.Version != 1 {
		t.Fatalf("version = %d, want 1", p.Version)
	}
	if p.AppKey != "app_1" || p.TicketID != "ticket_abc" {
		t.Fatalf("appkey/ticket id mismatch: %+v", p)
	}
	if p.Csat.LowRatingThreshold != 3 {
		t.Fatalf("low_rating_threshold = %d, want 3", p.Csat.LowRatingThreshold)
	}
	if len(p.Csat.Options) != 5 {
		t.Fatalf("options len = %d, want 5", len(p.Csat.Options))
	}
	if p.Csat.Options[0].Value != 1 || p.Csat.Options[4].Value != 5 {
		t.Fatalf("options values wrong: %+v", p.Csat.Options)
	}
	if p.Csat.ReplyMsgType != CsatReplyMsgType {
		t.Fatalf("reply_msg_type = %q, want %q", p.Csat.ReplyMsgType, CsatReplyMsgType)
	}

	// JSON 序列化也要可序列化
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	var round CsatInvitationPayload
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("unmarshal err: %v", err)
	}
	if round.Kind != p.Kind || round.TicketID != p.TicketID {
		t.Fatalf("round-trip mismatch")
	}
}

func TestValidateCsatReplyPayload(t *testing.T) {
	cases := []struct {
		name    string
		p       CsatReplyPayload
		wantErr bool
	}{
		{
			name:    "valid card_button",
			p:       CsatReplyPayload{Kind: "csatreply", Rating: 5, Comment: ""},
			wantErr: false,
		},
		{
			name:    "valid card_with_comment",
			p:       CsatReplyPayload{Kind: "csatreply", Rating: 2, Comment: "回复慢"},
			wantErr: false,
		},
		{
			name:    "wrong kind",
			p:       CsatReplyPayload{Kind: "rating", Rating: 5},
			wantErr: true,
		},
		{
			name:    "rating too low",
			p:       CsatReplyPayload{Kind: "csatreply", Rating: 0},
			wantErr: true,
		},
		{
			name:    "rating too high",
			p:       CsatReplyPayload{Kind: "csatreply", Rating: 6},
			wantErr: true,
		},
		{
			name:    "comment too long",
			p:       CsatReplyPayload{Kind: "csatreply", Rating: 4, Comment: strings.Repeat("a", 501)},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCsatReplyPayload(tc.p)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
