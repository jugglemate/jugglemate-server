package redisclient

import (
	"context"
	"os"
	"testing"

	"github.com/juggleim/jugglemate-server/commons/configures"
)

// TestOpenAgainstRedis 在显式提供地址时验收真实 Redis 建连、Ping 与关闭链路。
func TestOpenAgainstRedis(t *testing.T) {
	address := os.Getenv("AGENT_TEST_REDIS_ADDRESS")
	if address == "" {
		t.Skip("未设置 AGENT_TEST_REDIS_ADDRESS")
	}
	client, err := Open(context.Background(), configures.AgentRedisConfig{Address: address, DialTimeoutSeconds: 3})
	if err != nil {
		t.Fatalf("连接验收 Redis 失败: %v", err)
	}
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("Redis Ping 失败: %v", err)
	}
	if err := Close(client); err != nil {
		t.Fatalf("关闭 Redis 失败: %v", err)
	}
}
