-- 工单评价邀请补发支持：tickets 加 csat_notified_at 字段（Postgres 版本）。
-- TIPS: 与 commons/dbcommons/sqls/20260731.sql 语义同步；不复用 000006 内的
-- ALTER，避免回滚时与已上线部署冲突。

ALTER TABLE tickets
  ADD COLUMN IF NOT EXISTS csat_notified_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_tickets_app_csat_notified ON tickets (app_key, status, csat_notified_at);
