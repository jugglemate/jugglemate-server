package logs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/sirupsen/logrus"
)

// TestLogEntityInfofIncludesSortedFields 校验 Agent 埋点可复用现有日志模块输出稳定上下文。
func TestLogEntityInfofIncludesSortedFields(t *testing.T) {
	oldInfo, oldError := infoLogger, errorLogger
	defer SetLogger(oldInfo, oldError)
	var output bytes.Buffer
	info := logrus.New()
	info.SetOutput(&output)
	info.SetFormatter(&LogFormatter{})
	SetLogger(info, info)

	WithContext(context.Background()).WithField("trace_id", "trace-1").WithField("module", "agent.knowledge").Infof("检索完成")
	line := output.String()
	if !strings.Contains(line, "module:agent.knowledge\ttrace_id:trace-1\t检索完成") {
		t.Fatalf("日志字段或顺序异常: %s", line)
	}
}

// TestInitLogsWritesConfiguredRotatingFiles 校验已有 logPath/logName 配置真正承载普通和错误日志。
func TestInitLogsWritesConfiguredRotatingFiles(t *testing.T) {
	oldConfig, oldInfo, oldError := configures.Config, infoLogger, errorLogger
	defer func() {
		configures.Config = oldConfig
		SetLogger(oldInfo, oldError)
	}()
	configures.Config.Log.LogPath = t.TempDir()
	configures.Config.Log.LogName = "jmate-test"
	InitLogs()
	Infof("agent-info-marker")
	Errorf("agent-error-marker")

	infoContent, err := os.ReadFile(filepath.Join(configures.Config.Log.LogPath, "jmate-test.log"))
	if err != nil {
		t.Fatal(err)
	}
	errorContent, err := os.ReadFile(filepath.Join(configures.Config.Log.LogPath, "jmate-test_err.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(infoContent), "INFO\tagent-info-marker") || !strings.Contains(string(errorContent), "ERROR\tagent-error-marker") {
		t.Fatalf("配置日志文件内容异常: info=%q error=%q", infoContent, errorContent)
	}
}
