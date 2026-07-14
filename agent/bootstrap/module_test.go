package bootstrap

import (
	"context"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
)

func TestDisabledModuleDoesNotRequireInfrastructure(t *testing.T) {
	module := New(configures.AgentConfig{Enabled: false})
	if module.Enabled() {
		t.Fatal("模块应保持关闭")
	}
	if err := module.Start(context.Background()); err != nil {
		t.Fatalf("关闭状态不应连接基础设施: %v", err)
	}
	if err := module.Stop(context.Background()); err != nil {
		t.Fatalf("关闭未启动模块不应失败: %v", err)
	}
}
