// Package main 提供 MySQL 业务数据直接迁移到 PostgreSQL 的一次性命令。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	requiredSchemaVersion = "000002"
	confirmPhrase         = "MIGRATE_JMATE_BUSINESS_DATA"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type config struct {
	mysqlHost     string
	mysqlPort     string
	mysqlUser     string
	mysqlPassword string
	mysqlDatabase string
	postgresDSN   string
	backupDir     string
}

func main() {
	syscall.Umask(0o077)
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "迁移失败:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	mode := "precheck"
	if len(args) > 0 {
		mode = strings.TrimSpace(args[0])
	}
	if mode != "precheck" && mode != "migrate" && mode != "verify" {
		return errors.New("用法: direct-migrate {precheck|migrate|verify}")
	}
	cfg, err := loadConfig(mode)
	if err != nil {
		return err
	}
	mysqlDB, postgresDB, err := openDatabases(ctx, cfg)
	if err != nil {
		return err
	}
	defer mysqlDB.Close()
	defer postgresDB.Close()

	switch mode {
	case "precheck":
		return precheck(ctx, cfg, mysqlDB, postgresDB)
	case "verify":
		return verify(ctx, cfg, mysqlDB, postgresDB)
	case "migrate":
		if os.Getenv("JMATE_MIGRATION_CONFIRM") != confirmPhrase {
			return fmt.Errorf("请设置 JMATE_MIGRATION_CONFIRM=%s", confirmPhrase)
		}
		return migrate(ctx, cfg, mysqlDB, postgresDB)
	default:
		return nil
	}
}

func loadConfig(mode string) (config, error) {
	value := config{
		mysqlHost:     strings.TrimSpace(os.Getenv("JMATE_MYSQL_HOST")),
		mysqlPort:     strings.TrimSpace(os.Getenv("JMATE_MYSQL_PORT")),
		mysqlUser:     strings.TrimSpace(os.Getenv("JMATE_MYSQL_USER")),
		mysqlPassword: os.Getenv("JMATE_MYSQL_PASSWORD"),
		mysqlDatabase: strings.TrimSpace(os.Getenv("JMATE_MYSQL_DATABASE")),
		postgresDSN:   strings.TrimSpace(os.Getenv("JMATE_POSTGRES_DSN")),
		backupDir:     strings.TrimSpace(os.Getenv("JMATE_MIGRATION_BACKUP_DIR")),
	}
	if value.mysqlPort == "" {
		value.mysqlPort = "3306"
	}
	for name, item := range map[string]string{
		"JMATE_MYSQL_HOST": value.mysqlHost, "JMATE_MYSQL_USER": value.mysqlUser,
		"JMATE_MYSQL_PASSWORD": value.mysqlPassword, "JMATE_MYSQL_DATABASE": value.mysqlDatabase,
		"JMATE_POSTGRES_DSN": value.postgresDSN,
	} {
		if item == "" {
			return config{}, fmt.Errorf("%s 未设置", name)
		}
	}
	if !identifierPattern.MatchString(value.mysqlDatabase) {
		return config{}, errors.New("JMATE_MYSQL_DATABASE 只能包含字母、数字和下划线")
	}
	if _, err := strconv.Atoi(value.mysqlPort); err != nil {
		return config{}, errors.New("JMATE_MYSQL_PORT 必须是数字")
	}
	if mode == "migrate" {
		cleaned := filepath.Clean(value.backupDir)
		if value.backupDir == "" || !filepath.IsAbs(cleaned) || cleaned == string(filepath.Separator) {
			return config{}, errors.New("JMATE_MIGRATION_BACKUP_DIR 必须是非根目录的绝对路径")
		}
		value.backupDir = cleaned
	}
	return value, nil
}

func openDatabases(ctx context.Context, cfg config) (*sql.DB, *sql.DB, error) {
	mysqlConfig := mysqldriver.NewConfig()
	mysqlConfig.User = cfg.mysqlUser
	mysqlConfig.Passwd = cfg.mysqlPassword
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = net.JoinHostPort(cfg.mysqlHost, cfg.mysqlPort)
	mysqlConfig.DBName = cfg.mysqlDatabase
	mysqlConfig.Collation = "utf8mb4_unicode_ci"
	mysqlConfig.Params = map[string]string{"charset": "utf8mb4"}
	mysqlDB, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, nil, fmt.Errorf("初始化 MySQL 连接失败: %w", err)
	}
	mysqlDB.SetMaxOpenConns(2)
	mysqlDB.SetMaxIdleConns(1)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := mysqlDB.PingContext(pingCtx); err != nil {
		mysqlDB.Close()
		return nil, nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	postgresDB, err := sql.Open("postgres", cfg.postgresDSN)
	if err != nil {
		mysqlDB.Close()
		return nil, nil, fmt.Errorf("初始化 PostgreSQL 连接失败: %w", err)
	}
	postgresDB.SetMaxOpenConns(2)
	postgresDB.SetMaxIdleConns(1)
	if err := postgresDB.PingContext(pingCtx); err != nil {
		mysqlDB.Close()
		postgresDB.Close()
		return nil, nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	return mysqlDB, postgresDB, nil
}

func beginSourceSnapshot(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("创建 MySQL 一致性快照失败: %w", err)
	}
	return tx, nil
}
