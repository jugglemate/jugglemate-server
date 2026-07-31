-- 客户最后消息时间戳（与 last_user_msg_at 配合，判断"最后一句是谁说的"）。
-- PostgreSQL 版本。
-- TIPS: 同步至 commons/dbcommons/sqls/ 的 MySQL 版本。

ALTER TABLE tickets
  ADD COLUMN IF NOT EXISTS last_customer_msg_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_tickets_app_lcma ON tickets (app_key, last_customer_msg_at);
