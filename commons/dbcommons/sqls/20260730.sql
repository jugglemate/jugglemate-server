-- 工单自动关闭 + 评价系统（MySQL 版本）。
-- TIPS: 与 commons/dbcommons/sqls/20260730.sql -- 但 SQL 分库分发（MySQL / Postgres），
-- 本文件仅服务主库 MySQL / InnoDB。

ALTER TABLE tickets
  ADD COLUMN last_user_msg_at DATETIME(3) NULL,
  ADD COLUMN closed_at        DATETIME(3) NULL;

CREATE INDEX idx_tickets_app_luma ON tickets (app_key, last_user_msg_at);

CREATE TABLE IF NOT EXISTS ticket_messages (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  app_key      VARCHAR(20)  NOT NULL DEFAULT '',
  ticket_id    VARCHAR(50)  NOT NULL DEFAULT '',
  sender_id    VARCHAR(64)  NOT NULL DEFAULT '',
  sender_role  VARCHAR(16)  NOT NULL DEFAULT '',       -- customer / user / bot / system
  msg_id       VARCHAR(64)  NOT NULL DEFAULT '',
  msg_type     VARCHAR(16)  NOT NULL DEFAULT 'jg:text',
  created_time DATETIME(3)  DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_msg_id (app_key, msg_id),
  KEY idx_app_ticket_time (app_key, ticket_id, created_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS ticket_ratings (
  id           BIGINT       NOT NULL AUTO_INCREMENT,
  app_key      VARCHAR(20)  NOT NULL DEFAULT '',
  ticket_id    VARCHAR(50)  NOT NULL DEFAULT '',
  customer_id  VARCHAR(64)  NOT NULL DEFAULT '',
  assignee_id  VARCHAR(64)  NOT NULL DEFAULT '',
  rating       TINYINT      NOT NULL,
  comment      TEXT,
  source       VARCHAR(16)  NOT NULL DEFAULT 'card_button', -- card_button / card_with_comment
  created_time DATETIME(3)  DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_ticket_customer (app_key, ticket_id, customer_id),
  KEY idx_app_assignee_time (app_key, assignee_id, created_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
