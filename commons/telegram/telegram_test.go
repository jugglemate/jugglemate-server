package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendMessage(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL

	if err := client.SendMessage("bot-token", "12345", "hello"); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if gotPath != "/botbot-token/sendMessage" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "12345" || gotBody["text"] != "hello" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestClientSendMessageFailsOnTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"bad chat"}`))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL

	if err := client.SendMessage("bot-token", "12345", "hello"); err == nil {
		t.Fatal("SendMessage error = nil, want error")
	}
}
