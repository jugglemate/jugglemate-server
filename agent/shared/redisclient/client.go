// Package redisclient 提供 Agent Redis 客户端和生命周期管理。
package redisclient

import (
	"context"
	"fmt"
	"time"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"github.com/redis/go-redis/v9"
)

// Open 创建并校验 Agent Redis 客户端。
func Open(ctx context.Context, cfg configures.AgentRedisConfig) (*redis.Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("agent.redis.address 不能为空")
	}
	client := redis.NewClient(&redis.Options{
		Addr:        cfg.Address,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: time.Duration(cfg.DialTimeoutSeconds) * time.Second,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("校验 Agent Redis 连接失败: %w", err)
	}
	return client, nil
}

// Close 关闭 Agent Redis 客户端。
func Close(client *redis.Client) error {
	if client == nil {
		return nil
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("关闭 Agent Redis 失败: %w", err)
	}
	return nil
}
