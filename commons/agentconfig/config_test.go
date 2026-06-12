package agentconfig

import (
	"os"
	"testing"
	"time"
)

func TestAgentConfigReadsAgentServer(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp+"/conf", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp+"/conf/config.yml", []byte("agentServer:\n  baseURL: http://agent/v1\n  timeoutSeconds: 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if got := BaseURL(); got != "http://agent/v1" {
		t.Fatalf("BaseURL()=%q", got)
	}
	if got := Timeout(); got != 9*time.Second {
		t.Fatalf("Timeout()=%s", got)
	}
}
