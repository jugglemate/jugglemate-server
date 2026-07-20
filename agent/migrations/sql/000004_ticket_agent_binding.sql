-- 工单与 Agent 的绑定关系，供控制台在工单维度关联 Agent 使用。
--
-- 与 inbox_agent_bindings 的区别：
--   * inbox_agent_bindings 是「Inbox 默认由哪个 Agent 接待」，绑定会真实同步 IM 群成员，
--     因此需要 bot_id 与 sync_error 记录外部副作用；
--   * 本表只记录「这个工单关联了哪个 Agent」，不产生任何 IM 侧操作，因此不需要 bot_id、
--     status、sync_error，也不设外键 —— 工单与 Agent 的生命周期由各自业务域管理，
--     外键会让删除 Agent 被工单历史卡住。
CREATE TABLE IF NOT EXISTS ticket_agent_bindings (
	id VARCHAR(64) PRIMARY KEY,
	app_key VARCHAR(64) NOT NULL,
	ticket_id VARCHAR(64) NOT NULL,
	agent_id VARCHAR(64) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 唯一索引而不是普通索引：一个工单同时只能关联一个 Agent，唯一性让「重复绑定」退化为
-- 覆盖更新（ON CONFLICT DO UPDATE），否则会堆积多行、无法判定当前生效的是哪一个。
-- 它同时就是按工单查询绑定关系所需的索引。
CREATE UNIQUE INDEX IF NOT EXISTS uq_tab_app_ticket ON ticket_agent_bindings (app_key, ticket_id);
CREATE INDEX IF NOT EXISTS idx_tab_app_agent ON ticket_agent_bindings (app_key, agent_id);
