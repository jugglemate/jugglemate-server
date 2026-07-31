-- 客户最后消息时间戳（与 last_user_msg_at 配合，判断"最后一句是谁说的"）。
-- MySQL / InnoDB 版本。
-- TIPS: 同步至 agent/migrations/sql/000007 的 PostgreSQL 版本。

ALTER TABLE tickets
  ADD COLUMN last_customer_msg_at DATETIME(3) NULL;

CREATE INDEX idx_tickets_app_lcma ON tickets (app_key, last_customer_msg_at);
