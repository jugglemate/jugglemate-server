// Package main 提供 MySQL 业务数据直接迁移到 PostgreSQL 的一次性命令。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
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
	normalizedDSN, err := normalizePostgresDSN(value.postgresDSN)
	if err != nil {
		return config{}, err
	}
	value.postgresDSN = normalizedDSN
	if mode == "migrate" {
		cleaned := filepath.Clean(value.backupDir)
		if value.backupDir == "" || !filepath.IsAbs(cleaned) || cleaned == string(filepath.Separator) {
			return config{}, errors.New("JMATE_MIGRATION_BACKUP_DIR 必须是非根目录的绝对路径")
		}
		value.backupDir = cleaned
	}
	return value, nil
}

func normalizePostgresDSN(value string) (string, error) {
	if strings.Contains(strings.ToLower(value), "sslmode=") {
		return value, nil
	}
	if strings.HasPrefix(value, "postgres://") || strings.HasPrefix(value, "postgresql://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("JMATE_POSTGRES_DSN 格式非法: %w", err)
		}
		query := parsed.Query()
		query.Set("sslmode", "disable")
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
	// TIPS: 当前服务端 PostgreSQL 未启用 SSL；只有配置未声明策略时才与主服务现状对齐。
	return value + " sslmode=disable", nil
}

func postgresCommandEnvironment(dsn string, base []string) ([]string, error) {
	connectionInfo := dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := pq.ParseURL(dsn)
		if err != nil {
			return nil, fmt.Errorf("解析 PostgreSQL 连接地址失败: %w", err)
		}
		connectionInfo = parsed
	}
	parameters, err := parsePostgresConnectionInfo(connectionInfo)
	if err != nil {
		return nil, err
	}
	parameterEnvironment := map[string]string{
		"host": "PGHOST", "hostaddr": "PGHOSTADDR", "port": "PGPORT",
		"dbname": "PGDATABASE", "user": "PGUSER", "password": "PGPASSWORD",
		"passfile": "PGPASSFILE", "service": "PGSERVICE", "servicefile": "PGSERVICEFILE",
		"options": "PGOPTIONS", "application_name": "PGAPPNAME",
		"connect_timeout": "PGCONNECT_TIMEOUT", "client_encoding": "PGCLIENTENCODING",
		"sslmode": "PGSSLMODE", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY",
		"sslrootcert": "PGSSLROOTCERT", "sslcrl": "PGSSLCRL", "sslcrldir": "PGSSLCRLDIR",
		"sslsni": "PGSSLSNI", "sslpassword": "PGSSLPASSWORD",
		"requirepeer": "PGREQUIREPEER", "target_session_attrs": "PGTARGETSESSIONATTRS",
		"gssencmode": "PGGSSENCMODE", "krbsrvname": "PGKRBSRVNAME",
		"gsslib": "PGGSSLIB", "channel_binding": "PGCHANNELBINDING",
	}
	managed := make(map[string]struct{}, len(parameterEnvironment))
	for _, environmentName := range parameterEnvironment {
		managed[environmentName] = struct{}{}
	}
	result := make([]string, 0, len(base)+len(parameters))
	for _, item := range base {
		name, _, found := strings.Cut(item, "=")
		if _, shouldReplace := managed[name]; found && shouldReplace {
			continue
		}
		result = append(result, item)
	}
	// TIPS: pg_dump 不保证把连接 URI 识别为 PGDATABASE，因此拆成 libpq 环境变量；同时清理继承值，避免误连本机数据库。
	for parameterName, environmentName := range parameterEnvironment {
		if value, exists := parameters[parameterName]; exists {
			result = append(result, environmentName+"="+value)
		}
	}
	return result, nil
}

func parsePostgresConnectionInfo(value string) (map[string]string, error) {
	parameters := make(map[string]string)
	for index := 0; index < len(value); {
		for index < len(value) && isConnectionSpace(value[index]) {
			index++
		}
		if index == len(value) {
			break
		}
		keyStart := index
		for index < len(value) && value[index] != '=' && !isConnectionSpace(value[index]) {
			index++
		}
		key := strings.ToLower(value[keyStart:index])
		for index < len(value) && isConnectionSpace(value[index]) {
			index++
		}
		if key == "" || index == len(value) || value[index] != '=' {
			return nil, errors.New("PostgreSQL 键值 DSN 格式非法")
		}
		index++
		for index < len(value) && isConnectionSpace(value[index]) {
			index++
		}
		var parsed strings.Builder
		if index < len(value) && value[index] == '\'' {
			index++
			closed := false
			for index < len(value) {
				if value[index] == '\'' {
					index++
					closed = true
					break
				}
				if value[index] == '\\' {
					index++
					if index == len(value) {
						return nil, errors.New("PostgreSQL 键值 DSN 转义不完整")
					}
				}
				parsed.WriteByte(value[index])
				index++
			}
			if !closed {
				return nil, errors.New("PostgreSQL 键值 DSN 引号未闭合")
			}
			if index < len(value) && !isConnectionSpace(value[index]) {
				return nil, errors.New("PostgreSQL 键值 DSN 引号后存在非法字符")
			}
		} else {
			for index < len(value) && !isConnectionSpace(value[index]) {
				if value[index] == '\\' {
					index++
					if index == len(value) {
						return nil, errors.New("PostgreSQL 键值 DSN 转义不完整")
					}
				}
				parsed.WriteByte(value[index])
				index++
			}
		}
		parameters[key] = parsed.String()
	}
	return parameters, nil
}

func isConnectionSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
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
