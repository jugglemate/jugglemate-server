# MySQL 业务数据迁移到 PostgreSQL

该工具只迁移 `apps`、`appexts`、`users`、`customers`、`inboxes`、`customerinboxrels`、`inboxmembers`、`tickets`，明确跳过 `aibots` 和所有 `agent_*`/Twins 表。

## 上线顺序

1. 进入维护窗口，停止旧服务写入 MySQL。
2. 使用新版程序执行 PostgreSQL `000001`、`000002` Schema 迁移，但不对外放量。
3. 执行 `precheck`，核对源表和目标表行数。
4. 执行 `migrate`；脚本先备份两个数据库，再使用单个 PostgreSQL 事务导入，并按 Owner/绑定关系回填历史 Agent、Bot 的 AppKey。
5. 执行 `verify`，确认行数、InboxMember 关联、App 凭证以及 Agent/Bot AppKey 归属完整。
6. 启动新服务并验证登录、Inbox 列表、Agent Bot 创建与 Ticket 建群，再开放流量。

## 配置

```bash
export JMATE_MYSQL_HOST=127.0.0.1
export JMATE_MYSQL_PORT=3306
export JMATE_MYSQL_USER='<mysql-user>'
export JMATE_MYSQL_PASSWORD='<mysql-password>'
export JMATE_MYSQL_DATABASE=jmate_db
export JMATE_POSTGRES_DSN='<postgres-dsn>'
export JMATE_MIGRATION_BACKUP_DIR='/absolute/backup/path'
```

预检：

```bash
./scripts/migrations/mysql-to-postgres/migrate.sh precheck
```

执行：

```bash
export JMATE_MIGRATION_CONFIRM=MIGRATE_JMATE_BUSINESS_DATA
./scripts/migrations/mysql-to-postgres/migrate.sh migrate
```

再校验：

```bash
./scripts/migrations/mysql-to-postgres/migrate.sh verify
```

## 安全约束

- 目标八张业务表非空时拒绝导入，防止覆盖或错误合并。
- 每次执行都保留 MySQL 逻辑备份和 PostgreSQL custom-format 备份。
- 字符串以 HEX 中间格式传输，避免制表符、换行、反斜杠及 JSON 导致 COPY 转义错误。
- 已确认的历史异常数据会删除 `inbox_id` 开头的 `0x00`，并对三张 Inbox 关联表使用相同转换。
- 历史 Agent/Bot 只有在能通过 Owner 或既有绑定唯一映射到一个 AppKey 时才回填；无映射或跨应用冲突会回滚整个导入事务。
- 临时导出会在进程退出时清理；备份不会自动删除。
