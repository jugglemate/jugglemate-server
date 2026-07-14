// Package database 提供 Agent PostgreSQL 连接和生命周期管理。
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/juggleim/jugglemate-server/commons/configures"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 创建并校验 Agent PostgreSQL 连接。
//
// 返回的连接池由 Agent 模块统一关闭，业务模块不得自行关闭。
func Open(ctx context.Context, cfg configures.AgentPostgresConfig) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("agent.postgres.dsn 不能为空")
	}

	logMode := logger.Silent
	if cfg.Debug {
		logMode = logger.Info
	}
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DSN,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logMode),
	})
	if err != nil {
		return nil, fmt.Errorf("连接 Agent PostgreSQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 Agent PostgreSQL 连接池失败: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("校验 Agent PostgreSQL 连接失败: %w", err)
	}
	return db, nil
}

// Close 关闭 Agent PostgreSQL 连接池。
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取 Agent PostgreSQL 连接池失败: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("关闭 Agent PostgreSQL 失败: %w", err)
	}
	return nil
}
