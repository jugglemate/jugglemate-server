#!/usr/bin/env bash
set -euo pipefail
umask 077

# TIPS: 该脚本是停机切流前的一次性迁移命令，不由主服务启动时隐式执行。
mode="${1:-precheck}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tables=(apps appexts users customers inboxes customerinboxrels inboxmembers tickets)

require_command() {
  command -v "$1" >/dev/null 2>&1 || { echo "缺少命令: $1" >&2; exit 1; }
}

for command_name in mysql mysqldump psql pg_dump sed mktemp; do
  require_command "$command_name"
done

: "${JMATE_MYSQL_HOST:?JMATE_MYSQL_HOST 未设置}"
: "${JMATE_MYSQL_USER:?JMATE_MYSQL_USER 未设置}"
: "${JMATE_MYSQL_PASSWORD:?JMATE_MYSQL_PASSWORD 未设置}"
: "${JMATE_MYSQL_DATABASE:?JMATE_MYSQL_DATABASE 未设置}"
: "${JMATE_POSTGRES_DSN:?JMATE_POSTGRES_DSN 未设置}"

mysql_port="${JMATE_MYSQL_PORT:-3306}"
mysql_args=(--protocol=TCP -h"$JMATE_MYSQL_HOST" -P"$mysql_port" -u"$JMATE_MYSQL_USER" --batch --skip-column-names --default-character-set=utf8mb4)
export MYSQL_PWD="$JMATE_MYSQL_PASSWORD"
work_dir=""

cleanup() {
  unset MYSQL_PWD
  if [[ -n "$work_dir" && "$work_dir" == */jmate-mysql-pg-* && -d "$work_dir" ]]; then
    # TIPS: 临时 TSV 含应用密钥和密码哈希，无论成败都必须清理；备份目录不会被删除。
    find "$work_dir" -type f -delete
    rmdir "$work_dir" 2>/dev/null || true
  fi
}
trap cleanup EXIT

mysql_query() {
  mysql "${mysql_args[@]}" -e "$1"
}

pg_query() {
  psql "$JMATE_POSTGRES_DSN" -v ON_ERROR_STOP=1 -Atc "$1"
}

verify_schema() {
  local version
  version="$(pg_query "SELECT COALESCE(MAX(version),'') FROM agent_schema_migrations")"
  [[ "$version" == "000002" ]] || { echo "PostgreSQL Schema 未到 000002，当前=$version" >&2; exit 1; }
}

print_counts() {
  echo "MySQL 源表行数:"
  for table_name in "${tables[@]}"; do
    printf '  %s=%s\n' "$table_name" "$(mysql_query "SELECT COUNT(*) FROM ${JMATE_MYSQL_DATABASE}.${table_name}")"
  done
  echo "PostgreSQL 目标表行数:"
  for table_name in "${tables[@]}"; do
    printf '  %s=%s\n' "$table_name" "$(pg_query "SELECT COUNT(*) FROM ${table_name}")"
  done
}

verify_data() {
  local source_count target_count
  for table_name in "${tables[@]}"; do
    source_count="$(mysql_query "SELECT COUNT(*) FROM ${JMATE_MYSQL_DATABASE}.${table_name}")"
    target_count="$(pg_query "SELECT COUNT(*) FROM ${table_name}")"
    [[ "$source_count" == "$target_count" ]] || { echo "行数不一致: $table_name mysql=$source_count postgres=$target_count" >&2; exit 1; }
  done
  [[ "$(pg_query "SELECT COUNT(*) FROM inboxmembers im LEFT JOIN inboxes i ON i.app_key=im.app_key AND i.inbox_id=im.inbox_id WHERE i.id IS NULL")" == "0" ]] || { echo "存在无主 InboxMember" >&2; exit 1; }
  [[ "$(pg_query "SELECT COUNT(*) FROM apps WHERE app_key='' OR app_secret=''")" == "0" ]] || { echo "存在空 AppKey/AppSecret" >&2; exit 1; }
  [[ "$(pg_query "SELECT COUNT(*) FROM agents WHERE app_key=''")" == "0" ]] || { echo "存在未归属 AppKey 的 Agent" >&2; exit 1; }
  [[ "$(pg_query "SELECT COUNT(*) FROM bots WHERE app_key=''")" == "0" ]] || { echo "存在未归属 AppKey 的 Bot" >&2; exit 1; }
  [[ "$(pg_query "SELECT COUNT(*) FROM bot_agent_bindings binding JOIN bots b ON b.id=binding.bot_id JOIN agents a ON a.id=binding.agent_id WHERE binding.status='active' AND b.app_key<>a.app_key")" == "0" ]] || { echo "存在 Agent 与 Bot 跨 AppKey 绑定" >&2; exit 1; }
  echo "迁移校验通过"
}

verify_schema

case "$mode" in
  precheck)
    print_counts
    nul_inboxes="$(mysql_query "SELECT COUNT(*) FROM ${JMATE_MYSQL_DATABASE}.inboxes WHERE ORD(LEFT(inbox_id,1))=0")"
    echo "需删除首字节 NUL 的 Inbox: $nul_inboxes"
    ;;
  verify)
    verify_data
    ;;
  migrate)
    [[ "${JMATE_MIGRATION_CONFIRM:-}" == "MIGRATE_JMATE_BUSINESS_DATA" ]] || { echo "请设置 JMATE_MIGRATION_CONFIRM=MIGRATE_JMATE_BUSINESS_DATA" >&2; exit 1; }
    : "${JMATE_MIGRATION_BACKUP_DIR:?JMATE_MIGRATION_BACKUP_DIR 未设置}"
    [[ "$JMATE_MIGRATION_BACKUP_DIR" == /* && "$JMATE_MIGRATION_BACKUP_DIR" != "/" ]] || { echo "备份目录必须是非根目录的绝对路径" >&2; exit 1; }
    target_total="$(pg_query "SELECT (SELECT COUNT(*) FROM apps)+(SELECT COUNT(*) FROM appexts)+(SELECT COUNT(*) FROM users)+(SELECT COUNT(*) FROM customers)+(SELECT COUNT(*) FROM inboxes)+(SELECT COUNT(*) FROM customerinboxrels)+(SELECT COUNT(*) FROM inboxmembers)+(SELECT COUNT(*) FROM tickets)")"
    [[ "$target_total" == "0" ]] || { echo "目标业务表非空，拒绝覆盖；请先回滚或人工合并" >&2; exit 1; }

    mkdir -p "$JMATE_MIGRATION_BACKUP_DIR"
    timestamp="$(date +%Y%m%d%H%M%S)"
    mysqldump --protocol=TCP -h"$JMATE_MYSQL_HOST" -P"$mysql_port" -u"$JMATE_MYSQL_USER" --single-transaction --set-gtid-purged=OFF --no-tablespaces --result-file="$JMATE_MIGRATION_BACKUP_DIR/mysql-business-$timestamp.sql" "$JMATE_MYSQL_DATABASE" "${tables[@]}"
    pg_dump "$JMATE_POSTGRES_DSN" --format=custom --file="$JMATE_MIGRATION_BACKUP_DIR/postgres-before-$timestamp.dump"

    work_dir="$(mktemp -d /tmp/jmate-mysql-pg-XXXXXX)"
    # TIPS: MySQL DATE_FORMAT 的 `%s` 表示秒，不能再把 `%s` 同时当作 Shell 字段占位符替换。
    created_time_expr="DATE_FORMAT(COALESCE(created_time,CURRENT_TIMESTAMP(3)),'%Y-%m-%d %H:%i:%s.%f')"
    updated_time_expr="DATE_FORMAT(COALESCE(updated_time,CURRENT_TIMESTAMP(3)),'%Y-%m-%d %H:%i:%s.%f')"
    ticket_channel_expr="''"
    if [[ "$(mysql_query "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='${JMATE_MYSQL_DATABASE}' AND table_name='tickets' AND column_name='channel_type'")" == "1" ]]; then
      ticket_channel_expr="HEX(COALESCE(channel_type,''))"
    fi

    mysql_query "SELECT id,HEX(COALESCE(app_key,'')),HEX(COALESCE(app_secret,'')),COALESCE(app_status,0),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_name,'')) FROM ${JMATE_MYSQL_DATABASE}.apps ORDER BY id" > "$work_dir/apps.tsv"
    mysql_query "SELECT id,HEX(COALESCE(app_key,'')),HEX(COALESCE(app_item_key,'')),HEX(COALESCE(app_item_value,'')),$updated_time_expr FROM ${JMATE_MYSQL_DATABASE}.appexts ORDER BY id" > "$work_dir/appexts.tsv"
    mysql_query "SELECT id,HEX(COALESCE(user_id,'')),HEX(COALESCE(nickname,'')),HEX(COALESCE(avator,'')),HEX(COALESCE(login_account,'')),HEX(COALESCE(email,'')),HEX(COALESCE(login_pass,'')),COALESCE(role,1),COALESCE(status,1),HEX(COALESCE(im_token,'')),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.users ORDER BY id" > "$work_dir/users.tsv"
    mysql_query "SELECT id,HEX(COALESCE(customer_id,'')),HEX(COALESCE(nickname,'')),HEX(COALESCE(avator,'')),HEX(COALESCE(phone,'')),HEX(COALESCE(email,'')),HEX(COALESCE(identifier,'')),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.customers ORDER BY id" > "$work_dir/customers.tsv"
    mysql_query "SELECT id,HEX(IF(ORD(LEFT(inbox_id,1))=0,SUBSTRING(inbox_id,2),COALESCE(inbox_id,''))),HEX(COALESCE(channel_type,'')),HEX(COALESCE(channel_conf,'')),HEX(COALESCE(name,'')),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.inboxes ORDER BY id" > "$work_dir/inboxes.tsv"
    mysql_query "SELECT id,HEX(COALESCE(customer_id,'')),HEX(IF(ORD(LEFT(inbox_id,1))=0,SUBSTRING(inbox_id,2),COALESCE(inbox_id,''))),HEX(COALESCE(source_id,'')),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.customerinboxrels ORDER BY id" > "$work_dir/customerinboxrels.tsv"
    mysql_query "SELECT id,HEX(IF(ORD(LEFT(inbox_id,1))=0,SUBSTRING(inbox_id,2),COALESCE(inbox_id,''))),HEX(COALESCE(member_id,'')),$created_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.inboxmembers ORDER BY id" > "$work_dir/inboxmembers.tsv"
    mysql_query "SELECT id,HEX(COALESCE(ticket_id,'')),HEX(COALESCE(source_id,'')),HEX(COALESCE(assignee_id,'')),HEX(COALESCE(customer_id,'')),HEX(IF(ORD(LEFT(inbox_id,1))=0,SUBSTRING(inbox_id,2),COALESCE(inbox_id,''))),$ticket_channel_expr,COALESCE(status,0),$created_time_expr,$updated_time_expr,HEX(COALESCE(app_key,'')) FROM ${JMATE_MYSQL_DATABASE}.tickets ORDER BY id" > "$work_dir/tickets.tsv"

    sed "s|__DATA_DIR__|$work_dir|g" "$script_dir/import.sql.tpl" > "$work_dir/import.sql"
    psql "$JMATE_POSTGRES_DSN" -v ON_ERROR_STOP=1 -f "$work_dir/import.sql"
    verify_data
    echo "备份保留在: $JMATE_MIGRATION_BACKUP_DIR"
    ;;
  *)
    echo "用法: $0 {precheck|migrate|verify}" >&2
    exit 1
    ;;
esac
