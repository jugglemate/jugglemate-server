package logs

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
)

func TestSlogAndStdlogReachLogFile(t *testing.T) {
	configures.Config.Log.LogPath = t.TempDir()
	configures.Config.Log.LogName = "wiring"
	InitLogs()

	slog.Info("[IMBot] Bot 连接成功", "app_key", "ak1", "bot_user_id", "bot-1")
	slog.Error("[Inbound] IM 入站路由失败", "ticket_id", "ticket_x")
	log.Printf("[WebhookMsgs] received: appkey=%s", "ak1")

	info, err := os.ReadFile(filepath.Join(configures.Config.Log.LogPath, "wiring.log"))
	if err != nil {
		t.Fatal(err)
	}
	errContent, err := os.ReadFile(filepath.Join(configures.Config.Log.LogPath, "wiring_err.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(info), "[IMBot] Bot 连接成功\tapp_key:ak1\tbot_user_id:bot-1") {
		t.Fatalf("slog info 未进主日志: %q", info)
	}
	if !strings.Contains(string(errContent), "[Inbound] IM 入站路由失败\tticket_id:ticket_x") {
		t.Fatalf("slog error 未进错误日志: %q", errContent)
	}
	if !strings.Contains(string(info), "[WebhookMsgs] received: appkey=ak1") {
		t.Fatalf("log.Printf 未进主日志: %q", info)
	}
}
