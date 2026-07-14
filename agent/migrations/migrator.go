// Package migrations 提供 Agent PostgreSQL 最终态基线迁移。
package migrations

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:embed sql/*.sql
var migrationFS embed.FS

// Migration 描述一份按版本执行的 Agent 数据库迁移。
type Migration struct {
	Version string
	Name    string
	SQL     string
}

// List 返回按版本升序排列的内置 Agent 迁移。
func List() ([]Migration, error) {
	entries, err := migrationFS.ReadDir("sql")
	if err != nil {
		return nil, fmt.Errorf("读取 Agent 迁移目录失败: %w", err)
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		content, err := migrationFS.ReadFile("sql/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("读取 Agent 迁移 %s 失败: %w", entry.Name(), err)
		}
		migrations = append(migrations, Migration{Version: version, Name: entry.Name(), SQL: string(content)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	return migrations, nil
}

// Apply 执行尚未应用的 Agent PostgreSQL 迁移。
//
// 简要描述：每份迁移与版本记录在同一个事务中提交，DDL 任一步失败都会回滚，
// 防止数据库处于“表已创建但版本未记录”的半完成状态。
func Apply(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("Agent PostgreSQL 连接不能为空")
	}
	if err := db.WithContext(ctx).Exec(`
		CREATE TABLE IF NOT EXISTS agent_schema_migrations (
			version VARCHAR(32) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL
		)`).Error; err != nil {
		return fmt.Errorf("创建 Agent 迁移版本表失败: %w", err)
	}

	items, err := List()
	if err != nil {
		return err
	}
	for _, item := range items {
		var count int64
		if err := db.WithContext(ctx).Table("agent_schema_migrations").
			Where("version = ?", item.Version).Count(&count).Error; err != nil {
			return fmt.Errorf("查询 Agent 迁移版本 %s 失败: %w", item.Version, err)
		}
		if count > 0 {
			continue
		}
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(item.SQL).Error; err != nil {
				return fmt.Errorf("执行 %s 失败: %w", item.Name, err)
			}
			return tx.Exec(
				"INSERT INTO agent_schema_migrations(version, name, applied_at) VALUES (?, ?, ?)",
				item.Version,
				item.Name,
				time.Now().UTC(),
			).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

// migrationVersion 从迁移文件名中解析版本号。
func migrationVersion(name string) (string, error) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	version, _, found := strings.Cut(base, "_")
	if !found || version == "" {
		return "", fmt.Errorf("Agent 迁移文件名不合法: %s", name)
	}
	for _, char := range version {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("Agent 迁移版本必须为数字: %s", name)
		}
	}
	return version, nil
}
