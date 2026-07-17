package main

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type columnDef struct {
	name      string
	mysqlExpr string
	transform func(string) string
}

type tableDef struct {
	name    string
	columns []columnDef
}

type snapshotHeader struct {
	Format    string   `json:"format"`
	CreatedAt string   `json:"createdAt"`
	Database  string   `json:"database"`
	Tables    []string `json:"tables"`
}

type snapshotRow struct {
	Table   string    `json:"table"`
	Columns []string  `json:"columns,omitempty"`
	Values  []*string `json:"values"`
}

type legacyRecord struct {
	id       string
	ownerID  string
	explicit string
}

func precheck(ctx context.Context, cfg config, mysqlDB, postgresDB *sql.DB) error {
	sourceTx, err := beginSourceSnapshot(ctx, mysqlDB)
	if err != nil {
		return err
	}
	defer sourceTx.Rollback()
	definitions, err := loadTableDefinitions(ctx, sourceTx, cfg.mysqlDatabase)
	if err != nil {
		return err
	}
	version, err := schemaVersion(ctx, postgresDB)
	if err != nil {
		return err
	}
	fmt.Println("PostgreSQL Schema:", version)
	fmt.Println("MySQL 源表行数:")
	for _, definition := range definitions {
		count, countErr := sourceCount(ctx, sourceTx, cfg.mysqlDatabase, definition.name)
		if countErr != nil {
			return countErr
		}
		fmt.Printf("  %s=%d\n", definition.name, count)
	}
	nulCount, err := queryInt(ctx, sourceTx, fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`inboxes` WHERE ORD(LEFT(inbox_id,1))=0", cfg.mysqlDatabase))
	if err != nil {
		return fmt.Errorf("检查 Inbox NUL 失败: %w", err)
	}
	fmt.Println("需删除首字节 NUL 的 Inbox:", nulCount)
	if version != requiredSchemaVersion {
		fmt.Printf("PostgreSQL 尚未到 %s；应用 Schema 后再执行 migrate\n", requiredSchemaVersion)
		return nil
	}
	fmt.Println("PostgreSQL 目标表行数:")
	for _, definition := range definitions {
		count, countErr := queryInt(ctx, postgresDB, "SELECT COUNT(*) FROM "+definition.name)
		if countErr != nil {
			return countErr
		}
		fmt.Printf("  %s=%d\n", definition.name, count)
	}
	invalidAgents, invalidBots, err := validateLegacyMappings(ctx, sourceTx, postgresDB, cfg.mysqlDatabase)
	if err != nil {
		return err
	}
	fmt.Printf("历史 AppKey 映射: invalid_agents=%d invalid_bots=%d\n", invalidAgents, invalidBots)
	return nil
}

func migrate(ctx context.Context, cfg config, mysqlDB, postgresDB *sql.DB) error {
	if err := requireSchema(ctx, postgresDB); err != nil {
		return err
	}
	if err := ensureTargetEmpty(ctx, postgresDB); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.backupDir, 0o700); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}
	if err := os.Chmod(cfg.backupDir, 0o700); err != nil {
		return fmt.Errorf("设置备份目录权限失败: %w", err)
	}
	sourceTx, err := beginSourceSnapshot(ctx, mysqlDB)
	if err != nil {
		return err
	}
	defer sourceTx.Rollback()
	definitions, err := loadTableDefinitions(ctx, sourceTx, cfg.mysqlDatabase)
	if err != nil {
		return err
	}
	invalidAgents, invalidBots, err := validateLegacyMappings(ctx, sourceTx, postgresDB, cfg.mysqlDatabase)
	if err != nil {
		return err
	}
	if invalidAgents > 0 || invalidBots > 0 {
		return fmt.Errorf("历史 AppKey 无法唯一映射: agents=%d bots=%d", invalidAgents, invalidBots)
	}
	timestamp := time.Now().Format("20060102150405")
	postgresBackup := filepath.Join(cfg.backupDir, "postgres-before-"+timestamp+".dump")
	if err := backupPostgres(ctx, cfg.postgresDSN, postgresBackup); err != nil {
		return err
	}
	mysqlBackup := filepath.Join(cfg.backupDir, "mysql-business-"+timestamp+".jsonl.gz")
	if err := writeSourceSnapshot(ctx, sourceTx, cfg.mysqlDatabase, definitions, mysqlBackup); err != nil {
		return err
	}

	targetTx, err := postgresDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("开启 PostgreSQL 事务失败: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = targetTx.Rollback()
		}
	}()
	// TIPS: 锁住迁移命令并在事务内再次检查空表，避免两个发布进程并发导入。
	if _, err := targetTx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext('jmate_mysql_to_postgres_direct'))"); err != nil {
		return fmt.Errorf("获取迁移锁失败: %w", err)
	}
	if err := ensureTargetEmpty(ctx, targetTx); err != nil {
		return err
	}
	sourceCounts := make(map[string]int64, len(definitions))
	for _, definition := range definitions {
		count, copyErr := copyTable(ctx, sourceTx, targetTx, cfg.mysqlDatabase, definition)
		if copyErr != nil {
			return copyErr
		}
		sourceCounts[definition.name] = count
		fmt.Printf("已迁移 %s=%d\n", definition.name, count)
	}
	if err := resetSequences(ctx, targetTx, definitions); err != nil {
		return err
	}
	if err := backfillLegacyAppKeys(ctx, targetTx); err != nil {
		return err
	}
	if err := verifyTarget(ctx, targetTx, sourceCounts); err != nil {
		return err
	}
	if err := targetTx.Commit(); err != nil {
		return fmt.Errorf("提交 PostgreSQL 事务失败: %w", err)
	}
	committed = true
	if err := verify(ctx, cfg, mysqlDB, postgresDB); err != nil {
		return fmt.Errorf("提交后校验失败，请保持停机并按 PostgreSQL 备份回滚: %w", err)
	}
	fmt.Println("MySQL 源快照:", mysqlBackup)
	fmt.Println("PostgreSQL 迁移前备份:", postgresBackup)
	return nil
}

func verify(ctx context.Context, cfg config, mysqlDB, postgresDB *sql.DB) error {
	if err := requireSchema(ctx, postgresDB); err != nil {
		return err
	}
	sourceTx, err := beginSourceSnapshot(ctx, mysqlDB)
	if err != nil {
		return err
	}
	defer sourceTx.Rollback()
	definitions, err := loadTableDefinitions(ctx, sourceTx, cfg.mysqlDatabase)
	if err != nil {
		return err
	}
	counts := make(map[string]int64, len(definitions))
	for _, definition := range definitions {
		count, countErr := sourceCount(ctx, sourceTx, cfg.mysqlDatabase, definition.name)
		if countErr != nil {
			return countErr
		}
		counts[definition.name] = count
	}
	if err := verifyTarget(ctx, postgresDB, counts); err != nil {
		return err
	}
	fmt.Println("迁移校验通过")
	return nil
}

func loadTableDefinitions(ctx context.Context, source queryer, database string) ([]tableDef, error) {
	hasChannel, err := queryInt(ctx, source, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=? AND table_name='tickets' AND column_name='channel_type'", database)
	if err != nil {
		return nil, fmt.Errorf("检查 tickets.channel_type 失败: %w", err)
	}
	return tableDefinitions(hasChannel == 1), nil
}

func tableDefinitions(hasTicketChannel bool) []tableDef {
	created := "DATE_FORMAT(COALESCE(created_time,CURRENT_TIMESTAMP(3)),'%Y-%m-%d %H:%i:%s.%f')"
	updated := "DATE_FORMAT(COALESCE(updated_time,CURRENT_TIMESTAMP(3)),'%Y-%m-%d %H:%i:%s.%f')"
	textColumn := func(name string) columnDef { return columnDef{name: name, mysqlExpr: "COALESCE(" + name + ",'')"} }
	numberColumn := func(name, fallback string) columnDef {
		return columnDef{name: name, mysqlExpr: "CAST(COALESCE(" + name + "," + fallback + ") AS CHAR)"}
	}
	timeColumn := func(name, expression string) columnDef { return columnDef{name: name, mysqlExpr: expression} }
	inboxColumn := func() columnDef {
		return columnDef{name: "inbox_id", mysqlExpr: "COALESCE(inbox_id,'')", transform: stripLeadingNUL}
	}
	ticketChannel := columnDef{name: "channel_type", mysqlExpr: "''"}
	if hasTicketChannel {
		ticketChannel.mysqlExpr = "COALESCE(channel_type,'')"
	}
	return []tableDef{
		{name: "apps", columns: []columnDef{numberColumn("id", "0"), textColumn("app_key"), textColumn("app_secret"), numberColumn("app_status", "0"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_name")}},
		{name: "appexts", columns: []columnDef{numberColumn("id", "0"), textColumn("app_key"), textColumn("app_item_key"), textColumn("app_item_value"), timeColumn("updated_time", updated)}},
		{name: "users", columns: []columnDef{numberColumn("id", "0"), textColumn("user_id"), textColumn("nickname"), textColumn("avator"), textColumn("login_account"), textColumn("email"), textColumn("login_pass"), numberColumn("role", "1"), numberColumn("status", "1"), textColumn("im_token"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_key")}},
		{name: "customers", columns: []columnDef{numberColumn("id", "0"), textColumn("customer_id"), textColumn("nickname"), textColumn("avator"), textColumn("phone"), textColumn("email"), textColumn("identifier"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_key")}},
		{name: "inboxes", columns: []columnDef{numberColumn("id", "0"), inboxColumn(), textColumn("channel_type"), textColumn("channel_conf"), textColumn("name"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_key")}},
		{name: "customerinboxrels", columns: []columnDef{numberColumn("id", "0"), textColumn("customer_id"), inboxColumn(), textColumn("source_id"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_key")}},
		{name: "inboxmembers", columns: []columnDef{numberColumn("id", "0"), inboxColumn(), textColumn("member_id"), timeColumn("created_time", created), textColumn("app_key")}},
		{name: "tickets", columns: []columnDef{numberColumn("id", "0"), textColumn("ticket_id"), textColumn("source_id"), textColumn("assignee_id"), textColumn("customer_id"), inboxColumn(), ticketChannel, numberColumn("status", "0"), timeColumn("created_time", created), timeColumn("updated_time", updated), textColumn("app_key")}},
	}
}

func copyTable(ctx context.Context, source *sql.Tx, target *sql.Tx, database string, definition tableDef) (int64, error) {
	rows, err := source.QueryContext(ctx, sourceSelect(database, definition))
	if err != nil {
		return 0, fmt.Errorf("读取 MySQL %s 失败: %w", definition.name, err)
	}
	defer rows.Close()
	statement, err := target.PrepareContext(ctx, targetInsert(definition))
	if err != nil {
		return 0, fmt.Errorf("准备 PostgreSQL %s 写入失败: %w", definition.name, err)
	}
	defer statement.Close()
	var count int64
	for rows.Next() {
		values, scanErr := scanTextRow(rows, len(definition.columns))
		if scanErr != nil {
			return count, fmt.Errorf("读取 MySQL %s 行失败: %w", definition.name, scanErr)
		}
		arguments := make([]any, len(values))
		for index, value := range values {
			if value == nil {
				arguments[index] = nil
				continue
			}
			text := *value
			if definition.columns[index].transform != nil {
				text = definition.columns[index].transform(text)
			}
			arguments[index] = text
		}
		if _, err := statement.ExecContext(ctx, arguments...); err != nil {
			return count, fmt.Errorf("写入 PostgreSQL %s 第 %d 行失败: %w", definition.name, count+1, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("遍历 MySQL %s 失败: %w", definition.name, err)
	}
	return count, nil
}

func writeSourceSnapshot(ctx context.Context, source *sql.Tx, database string, definitions []tableDef, destination string) (returnErr error) {
	partial := destination + ".partial"
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("创建 MySQL 源快照失败: %w", err)
	}
	defer func() {
		_ = file.Close()
		if returnErr != nil {
			_ = os.Remove(partial)
		}
	}()
	zipper := gzip.NewWriter(file)
	encoder := json.NewEncoder(zipper)
	tableNames := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		tableNames = append(tableNames, definition.name)
	}
	if err := encoder.Encode(snapshotHeader{Format: "jmate-mysql-business-v1", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Database: database, Tables: tableNames}); err != nil {
		return fmt.Errorf("写入快照头失败: %w", err)
	}
	for _, definition := range definitions {
		rows, queryErr := source.QueryContext(ctx, sourceSelect(database, definition))
		if queryErr != nil {
			return fmt.Errorf("快照读取 %s 失败: %w", definition.name, queryErr)
		}
		columns := columnNames(definition)
		// TIPS: 先写表结构标记，确保空表也能在源快照中被完整识别。
		if err := encoder.Encode(snapshotRow{Table: definition.name, Columns: columns}); err != nil {
			rows.Close()
			return fmt.Errorf("快照写入 %s 表结构失败: %w", definition.name, err)
		}
		for rows.Next() {
			values, scanErr := scanTextRow(rows, len(definition.columns))
			if scanErr != nil {
				rows.Close()
				return fmt.Errorf("快照读取 %s 行失败: %w", definition.name, scanErr)
			}
			if err := encoder.Encode(snapshotRow{Table: definition.name, Values: values}); err != nil {
				rows.Close()
				return fmt.Errorf("快照写入 %s 失败: %w", definition.name, err)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("快照遍历 %s 失败: %w", definition.name, err)
		}
		rows.Close()
	}
	if err := zipper.Close(); err != nil {
		return fmt.Errorf("完成 MySQL 源快照压缩失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("完成 MySQL 源快照写入失败: %w", err)
	}
	if err := os.Rename(partial, destination); err != nil {
		return fmt.Errorf("提交 MySQL 源快照失败: %w", err)
	}
	return nil
}

func backupPostgres(ctx context.Context, dsn, destination string) error {
	partial := destination + ".partial"
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("PostgreSQL 备份已存在，拒绝覆盖: %s", destination)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 PostgreSQL 备份路径失败: %w", err)
	}
	// TIPS: DSN 通过环境传递，避免数据库密码出现在远端进程参数和进程审计中。
	command := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--file="+partial)
	command.Env = append(os.Environ(), "PGDATABASE="+dsn)
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("备份 PostgreSQL 失败: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := os.Chmod(partial, 0o600); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("设置 PostgreSQL 备份权限失败: %w", err)
	}
	if err := os.Rename(partial, destination); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("提交 PostgreSQL 备份失败: %w", err)
	}
	return nil
}

func validateLegacyMappings(ctx context.Context, source queryer, target queryer, database string) (int, int, error) {
	userApps, err := loadSourceUserApps(ctx, source, database)
	if err != nil {
		return 0, 0, err
	}
	systemApps, err := loadSourceAppKeys(ctx, source, database)
	if err != nil {
		return 0, 0, err
	}
	for appKey := range systemApps {
		addCandidate(userApps, "system", appKey)
	}
	agents, err := loadLegacyRecords(ctx, target, "SELECT id,owner_id,app_key FROM agents")
	if err != nil {
		return 0, 0, fmt.Errorf("读取历史 Agent 失败: %w", err)
	}
	agentApps := make(map[string]string, len(agents))
	invalidAgents := 0
	for _, agent := range agents {
		resolved, ok := resolveRecordApp(agent.explicit, userApps[agent.ownerID])
		if !ok {
			invalidAgents++
			continue
		}
		agentApps[agent.id] = resolved
	}
	bots, err := loadLegacyRecords(ctx, target, "SELECT id,owner_id,app_key FROM bots")
	if err != nil {
		return 0, 0, fmt.Errorf("读取历史 Bot 失败: %w", err)
	}
	boundApps := map[string]map[string]struct{}{}
	rows, err := target.QueryContext(ctx, "SELECT bot_id,agent_id FROM bot_agent_bindings WHERE status='active'")
	if err != nil {
		return 0, 0, fmt.Errorf("读取 Bot-Agent 绑定失败: %w", err)
	}
	for rows.Next() {
		var botID, agentID string
		if err := rows.Scan(&botID, &agentID); err != nil {
			rows.Close()
			return 0, 0, err
		}
		if appKey := agentApps[agentID]; appKey != "" {
			addCandidate(boundApps, botID, appKey)
		}
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	invalidBots := 0
	for _, bot := range bots {
		candidates := cloneCandidates(userApps[bot.ownerID])
		for appKey := range boundApps[bot.id] {
			candidates[appKey] = struct{}{}
		}
		if _, ok := resolveRecordApp(bot.explicit, candidates); !ok {
			invalidBots++
		}
	}
	return invalidAgents, invalidBots, nil
}

func loadSourceAppKeys(ctx context.Context, source queryer, database string) (map[string]struct{}, error) {
	rows, err := source.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(app_key,'') FROM `%s`.`apps`", database))
	if err != nil {
		return nil, fmt.Errorf("读取 MySQL AppKey 失败: %w", err)
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var appKey string
		if err := rows.Scan(&appKey); err != nil {
			return nil, err
		}
		if appKey != "" {
			result[appKey] = struct{}{}
		}
	}
	return result, rows.Err()
}

func loadSourceUserApps(ctx context.Context, source queryer, database string) (map[string]map[string]struct{}, error) {
	rows, err := source.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(user_id,''),COALESCE(app_key,'') FROM `%s`.`users`", database))
	if err != nil {
		return nil, fmt.Errorf("读取 MySQL 用户 AppKey 失败: %w", err)
	}
	defer rows.Close()
	result := map[string]map[string]struct{}{}
	for rows.Next() {
		var userID, appKey string
		if err := rows.Scan(&userID, &appKey); err != nil {
			return nil, err
		}
		if userID != "" && appKey != "" {
			addCandidate(result, userID, appKey)
		}
	}
	return result, rows.Err()
}

func loadLegacyRecords(ctx context.Context, target queryer, statement string) ([]legacyRecord, error) {
	rows, err := target.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []legacyRecord
	for rows.Next() {
		var item legacyRecord
		if err := rows.Scan(&item.id, &item.ownerID, &item.explicit); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func resolveRecordApp(explicit string, candidates map[string]struct{}) (string, bool) {
	if explicit != "" {
		return explicit, true
	}
	if len(candidates) != 1 {
		return "", false
	}
	for candidate := range candidates {
		return candidate, true
	}
	return "", false
}

func addCandidate(target map[string]map[string]struct{}, key, candidate string) {
	if candidate == "" {
		return
	}
	if target[key] == nil {
		target[key] = map[string]struct{}{}
	}
	target[key][candidate] = struct{}{}
}

func cloneCandidates(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}

func backfillLegacyAppKeys(ctx context.Context, target *sql.Tx) error {
	const statement = `
CREATE TEMP TABLE stage_owner_app_keys ON COMMIT DROP AS
SELECT user_id AS owner_id,app_key FROM users
UNION
SELECT 'system' AS owner_id,app_key FROM apps;
CREATE TEMP TABLE stage_agent_app_keys ON COMMIT DROP AS
SELECT a.id, MIN(u.app_key) AS app_key, COUNT(DISTINCT u.app_key) AS candidate_count
FROM agents a LEFT JOIN stage_owner_app_keys u ON u.owner_id=a.owner_id WHERE a.app_key='' GROUP BY a.id;
DO $$ DECLARE invalid_count BIGINT; BEGIN
  SELECT COUNT(*) INTO invalid_count FROM stage_agent_app_keys WHERE candidate_count<>1;
  IF invalid_count>0 THEN RAISE EXCEPTION '存在 % 个历史 Agent 无法唯一映射 AppKey，迁移已回滚',invalid_count; END IF;
END $$;
UPDATE agents a SET app_key=mapped.app_key,updated_at=now() FROM stage_agent_app_keys mapped WHERE a.id=mapped.id;
CREATE TEMP TABLE stage_bot_app_keys ON COMMIT DROP AS
SELECT b.id,MIN(candidate.app_key) AS app_key,COUNT(DISTINCT candidate.app_key) AS candidate_count
FROM bots b LEFT JOIN (
  SELECT binding.bot_id,a.app_key FROM bot_agent_bindings binding JOIN agents a ON a.id=binding.agent_id WHERE binding.status='active' AND a.app_key<>''
  UNION
  SELECT owned.id,u.app_key FROM bots owned JOIN stage_owner_app_keys u ON u.owner_id=owned.owner_id
) candidate ON candidate.bot_id=b.id WHERE b.app_key='' GROUP BY b.id;
DO $$ DECLARE invalid_count BIGINT; BEGIN
  SELECT COUNT(*) INTO invalid_count FROM stage_bot_app_keys WHERE candidate_count<>1;
  IF invalid_count>0 THEN RAISE EXCEPTION '存在 % 个历史 Bot 无法唯一映射 AppKey，迁移已回滚',invalid_count; END IF;
END $$;
UPDATE bots b SET app_key=mapped.app_key,updated_at=now() FROM stage_bot_app_keys mapped WHERE b.id=mapped.id;`
	if _, err := target.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("回填历史 Agent/Bot AppKey 失败: %w", err)
	}
	return nil
}

func verifyTarget(ctx context.Context, target queryer, sourceCounts map[string]int64) error {
	for tableName, sourceCount := range sourceCounts {
		targetCount, err := queryInt(ctx, target, "SELECT COUNT(*) FROM "+tableName)
		if err != nil {
			return err
		}
		if sourceCount != targetCount {
			return fmt.Errorf("行数不一致: %s mysql=%d postgres=%d", tableName, sourceCount, targetCount)
		}
	}
	checks := []struct{ query, message string }{
		{"SELECT COUNT(*) FROM inboxmembers im LEFT JOIN inboxes i ON i.app_key=im.app_key AND i.inbox_id=im.inbox_id WHERE i.id IS NULL", "存在无主 InboxMember"},
		{"SELECT COUNT(*) FROM apps WHERE app_key='' OR app_secret=''", "存在空 AppKey/AppSecret"},
		{"SELECT COUNT(*) FROM agents a LEFT JOIN apps app ON app.app_key=a.app_key WHERE a.app_key='' OR app.id IS NULL", "存在未归属有效 AppKey 的 Agent"},
		{"SELECT COUNT(*) FROM bots b LEFT JOIN apps app ON app.app_key=b.app_key WHERE b.app_key='' OR app.id IS NULL", "存在未归属有效 AppKey 的 Bot"},
		{"SELECT COUNT(*) FROM bot_agent_bindings binding JOIN bots b ON b.id=binding.bot_id JOIN agents a ON a.id=binding.agent_id WHERE binding.status='active' AND b.app_key<>a.app_key", "存在 Agent 与 Bot 跨 AppKey 绑定"},
	}
	for _, check := range checks {
		count, err := queryInt(ctx, target, check.query)
		if err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("%s: %d", check.message, count)
		}
	}
	return nil
}

func resetSequences(ctx context.Context, target *sql.Tx, definitions []tableDef) error {
	for _, definition := range definitions {
		statement := fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s','id'),COALESCE(MAX(id),1),MAX(id) IS NOT NULL) FROM %s", definition.name, definition.name)
		if _, err := target.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("重置 %s 序列失败: %w", definition.name, err)
		}
	}
	return nil
}

func ensureTargetEmpty(ctx context.Context, target queryer) error {
	const statement = "SELECT (SELECT COUNT(*) FROM apps)+(SELECT COUNT(*) FROM appexts)+(SELECT COUNT(*) FROM users)+(SELECT COUNT(*) FROM customers)+(SELECT COUNT(*) FROM inboxes)+(SELECT COUNT(*) FROM customerinboxrels)+(SELECT COUNT(*) FROM inboxmembers)+(SELECT COUNT(*) FROM tickets)"
	count, err := queryInt(ctx, target, statement)
	if err != nil {
		return fmt.Errorf("检查 PostgreSQL 目标表失败: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("目标业务表非空，拒绝覆盖: total=%d", count)
	}
	return nil
}

func schemaVersion(ctx context.Context, target queryer) (string, error) {
	var version string
	if err := target.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),'') FROM agent_schema_migrations").Scan(&version); err != nil {
		return "", fmt.Errorf("读取 PostgreSQL Schema 版本失败: %w", err)
	}
	return version, nil
}

func requireSchema(ctx context.Context, target queryer) error {
	version, err := schemaVersion(ctx, target)
	if err != nil {
		return err
	}
	if version != requiredSchemaVersion {
		return fmt.Errorf("PostgreSQL Schema 未到 %s，当前=%s", requiredSchemaVersion, version)
	}
	return nil
}

func sourceCount(ctx context.Context, source queryer, database, tableName string) (int64, error) {
	return queryInt(ctx, source, fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", database, tableName))
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryInt(ctx context.Context, source queryer, statement string, args ...any) (int64, error) {
	var count int64
	if err := source.QueryRowContext(ctx, statement, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func sourceSelect(database string, definition tableDef) string {
	expressions := make([]string, 0, len(definition.columns))
	for _, column := range definition.columns {
		expressions = append(expressions, column.mysqlExpr)
	}
	return fmt.Sprintf("SELECT %s FROM `%s`.`%s` ORDER BY id", strings.Join(expressions, ","), database, definition.name)
}

func targetInsert(definition tableDef) string {
	placeholders := make([]string, 0, len(definition.columns))
	for index := range definition.columns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", definition.name, strings.Join(columnNames(definition), ","), strings.Join(placeholders, ","))
}

func columnNames(definition tableDef) []string {
	names := make([]string, 0, len(definition.columns))
	for _, column := range definition.columns {
		names = append(names, column.name)
	}
	return names
}

func scanTextRow(rows *sql.Rows, size int) ([]*string, error) {
	raw := make([]sql.RawBytes, size)
	destinations := make([]any, size)
	for index := range raw {
		destinations[index] = &raw[index]
	}
	if err := rows.Scan(destinations...); err != nil {
		return nil, err
	}
	result := make([]*string, size)
	for index, value := range raw {
		if value == nil {
			continue
		}
		text := string(value)
		result[index] = &text
	}
	return result, nil
}

func stripLeadingNUL(value string) string {
	if strings.HasPrefix(value, "\x00") {
		return strings.TrimPrefix(value, "\x00")
	}
	return value
}
