-- 工单是否转人工：持久化事实 + 事件流水。
-- TIPS: is_human_taken_over 单调递增，重复触发只补事件、不覆盖首次时间。
-- TIPS: human_taken_over_by 当前存 customer_id；如未来由坐席触发并需要区分，事件表的 payload 保留原始角色。

ALTER TABLE tickets
  ADD COLUMN is_human_taken_over TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN human_taken_over_at DATETIME(3) NULL,
  ADD COLUMN human_taken_over_by VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX idx_tickets_app_human ON tickets (app_key, is_human_taken_over);

CREATE TABLE IF NOT EXISTS ticket_events (
  id BIGINT NOT NULL AUTO_INCREMENT,
  app_key VARCHAR(20) NOT NULL DEFAULT '',
  ticket_id VARCHAR(50) NOT NULL DEFAULT '',
  event_type VARCHAR(32) NOT NULL DEFAULT '',
  operator_id VARCHAR(64) NOT NULL DEFAULT '',
  operator_type VARCHAR(16) NOT NULL DEFAULT '',
  payload JSON NULL,
  created_time DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_ticket (app_key, ticket_id, id),
  KEY idx_app_ticket_type (app_key, ticket_id, event_type),
  KEY idx_app_type_time (app_key, event_type, created_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
