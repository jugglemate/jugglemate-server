-- 工单评价邀请补发支持：tickets 加 csat_notified_at 字段。
--
-- TIPS: 在 000006 之上补一列；不重建 000006 字段，避免回滚时与已上线部署的列冲突。
-- csat_notified_at NULL 表示工单关闭后还没发 jgm:csat 邀请，ticker 周期会补发；
-- 非 NULL 表示已发，避免重复打扰客户。

ALTER TABLE tickets
  ADD COLUMN csat_notified_at DATETIME(3) NULL;

CREATE INDEX idx_tickets_app_csat_notified ON tickets (app_key, status, csat_notified_at);
