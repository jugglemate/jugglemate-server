package dbs

import "testing"

func TestEmailPtrForDB(t *testing.T) {
	if emailPtrForDB("") != nil {
		t.Fatal("empty email should be nil")
	}
	if emailPtrForDB("   ") != nil {
		t.Fatal("whitespace email should be nil")
	}
	ptr := emailPtrForDB("alice@example.com")
	if ptr == nil || *ptr != "alice@example.com" {
		t.Fatalf("email ptr = %v", ptr)
	}
}

func TestEmailStringFromDB(t *testing.T) {
	if got := emailStringFromDB(nil); got != "" {
		t.Fatalf("nil email = %q, want empty string", got)
	}
	email := "alice@example.com"
	if got := emailStringFromDB(&email); got != email {
		t.Fatalf("email = %q", got)
	}
}
