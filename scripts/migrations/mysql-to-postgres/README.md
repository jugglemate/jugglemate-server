# MySQL 业务数据迁移到 PostgreSQL

该工具只迁移 `apps`、`appexts`、`users`、`customers`、`inboxes`、`customerinboxrels`、`inboxmembers`、`tickets`，明确跳过 `aibots` 和所有 `agent_*`/Twins 表。

生产环境推荐使用 `direct/` 下的独立 Go 迁移程序：它通过数据库驱动直接执行 MySQL `SELECT` 和 PostgreSQL 参数化 `INSERT`，不依赖服务器的 `mysql`/`mysqldump` 客户端。根目录 `migrate.sh` 作为已有兼容路径保留。

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

## 直接迁移（推荐）

构建独立静态二进制；该子模块的 MySQL 驱动不会进入主服务依赖：

```bash
cd scripts/migrations/mysql-to-postgres/direct
CGO_ENABLED=0 go build -trimpath -o direct-migrate .
```

预检：

```bash
./direct-migrate precheck
```

执行前必须停止旧服务写入 MySQL，然后显式确认：

```bash
export JMATE_MIGRATION_CONFIRM=MIGRATE_JMATE_BUSINESS_DATA
./direct-migrate migrate
./direct-migrate verify
```

`precheck` 会统计源数据中 PostgreSQL `text/varchar` 无法存储的 NUL 字节字段值；迁移快照保留 MySQL 原值，写入 PostgreSQL 时移除这些 NUL 字节。

直接模式会生成两份 `0600` 备份：MySQL 一致性只读快照 `mysql-business-*.jsonl.gz`，以及 PostgreSQL custom-format 备份 `postgres-before-*.dump`。源 MySQL 不执行任何写入或删除。

## CLI 兼容模式

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
- 历史 Agent/Bot 只有在能通过 Owner 或既有绑定唯一映射到一个 AppKey 时才回填；`system` Owner 仅在源 AppKey 唯一时映射，无映射或跨应用冲突会回滚整个导入事务。
- 临时导出会在进程退出时清理；备份不会自动删除。
