## Purpose

为工单域提供「转人工」事实的持久化与可查询能力，使业务可以在不依赖 IM 群消息的前提下回答「哪些工单曾转人工」「谁触发的」「什么时候发生」。

## Requirements

### Requirement: 工单表持久化「曾转人工」事实

`tickets` 表 MUST 增加 3 个字段：

- `is_human_taken_over`：TINYINT(1) / BOOLEAN，默认 0
- `human_taken_over_at`：DATETIME(3) / TIMESTAMPTZ，可空
- `human_taken_over_by`：VARCHAR(64)，可空（语义为触发者 customer_id；如后续由坐席触发则改存 user_id，并在事件表 payload 中保留原始角色）

`is_human_taken_over` 只能 0→1 单调递增；一旦置 1 不可回滚。

#### Scenario: 首次转人工

- WHEN 客户在 IM 群触发转人工关键词
- THEN `is_human_taken_over` 由 0 变为 1
- AND `human_taken_over_at` 记当前时间
- AND `human_taken_over_by` 记触发者 customer_id

#### Scenario: 重复触发

- WHEN 同一工单再次触发转人工
- THEN `is_human_taken_over` 仍为 1，`human_taken_over_at` **不更新**（保留首次时间）
- AND `ticket_events` 表新增一条 `event_type='human_takeover'` 记录

### Requirement: 工单事件流水

`ticket_events` 表 MUST 记录 `(app_key, ticket_id, event_type, operator_id, operator_type, payload, created_time)`，索引 `(app_key, ticket_id, event_type)` 与 `(app_key, event_type, created_time)`。

`operator_type` 取值：`customer` / `user` / `system`。

`payload` 为 JSON，可存扩展字段（如触发的关键词、原始消息 ID 等）。

`event_type` 当前仅 `human_takeover` 强制落地；其他枚举值（`claim` / `transfer` / `close` / `reopen`）预留但本 change 不写数据。

#### Scenario: 事件按工单维度查询

- WHEN 任意登录用户调用 `GET /jmate/tickets/:ticket_id/events`
- THEN 返回该工单所有事件，按 `created_time` 倒序

### Requirement: 转入路径写入顺序

转人工副作用 MUST 按「DB 持久化（`tickets` 更新 + `ticket_events` 插入）→ IM 拉人/退群」原子顺序执行；DB 持久化失败 MUST 中断后续 IM 副作用。

#### Scenario: DB 持久化失败

- WHEN `UPDATE tickets` 或 `INSERT ticket_events` 失败
- THEN 不调用 IM 拉人、不广播通知、不退 Bot

### Requirement: API 字段透出

`TicketInfo` MUST 透出 `is_human_taken_over / human_taken_over_at / human_taken_over_by`；`QryTickets` 与 `QryCustomerTickets` MUST 支持 `is_human_taken_over` 过滤项（仅取 0 / 1，未传则不过滤）。

#### Scenario: 列表筛选

- WHEN 控制台调 `GET /jmate/tickets/list?is_human_taken_over=1`
- THEN 仅返回 `is_human_taken_over=1` 的工单

#### Scenario: 客户维度筛选

- WHEN 控制台调 `GET /jmate/customers/tickets?is_human_taken_over=0`
- THEN 仅返回未转人工的工单

### Requirement: 转人工触发入口

`agent/modules/message/service/inbound.go` 在 `MatchHandoffKeyword` 命中分支 MUST 先调 `MarkHumanTakeover`，再调注入的 `service.humanHandoff`。

#### Scenario: 关键词命中

- WHEN 客户发送的整句归一化后匹配上 `handoffKeywords`
- THEN `MarkHumanTakeover` 先执行
- AND `humanHandoff` 再执行（拉坐席 + 广播 `jgm:ticketassign` + 退 Bot）

#### Scenario: 关键词未命中

- WHEN 客户消息未命中关键词
- THEN 走原 AI 回复流程，不触发 `MarkHumanTakeover`
