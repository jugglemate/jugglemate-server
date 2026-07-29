# 1. DB 迁移

- [ ] 1.1 MySQL：新写 `commons/dbcommons/sqls/20260729.sql`
  - `ALTER TABLE tickets ADD COLUMN is_human_taken_over TINYINT(1) NOT NULL DEFAULT 0`、
    `ADD COLUMN human_taken_over_at DATETIME(3) NULL`、
    `ADD COLUMN human_taken_over_by VARCHAR(64) NOT NULL DEFAULT ''`
  - 建 `ticket_events` 表（含 `(app_key, ticket_id, id)` 唯一键 + `(app_key, ticket_id, event_type)` 与 `(app_key, event_type, created_time)` 索引）
- [ ] 1.2 Postgres：新写 `agent/migrations/sql/000005_ticket_human_takeover_mark.sql`
  - 同步 `tickets` 字段：SMALLINT/BIGINT、`TIMESTAMPTZ`、VARCHAR(64)
  - 同步 `ticket_events` 表（含 `CREATE INDEX IF NOT EXISTS`）

# 2. 模型层

- [ ] 2.1 `storages/models/ticket.go`
  - `Ticket` struct 加 `IsHumanTakenOver bool / HumanTakenOverAt int64 / HumanTakenOverBy string`
  - 新增 `TicketEventType` 枚举（`human_takeover` 等）
  - 新增 `TicketEvent` struct + `ITicketEventStorage` 接口
- [ ] 2.2 新增 `storages/dbs/ticketeventdao.go`
  - `Create(event)` / `QryByTicket(appkey, ticketId, limit, offset)`
  - 配套单测 `storages/dbs/ticketeventdao_test.go`
- [ ] 2.3 `storages/dbs/ticketdao.go`
  - `Create` / `Update` / `Upsert` 同步 `is_human_taken_over` / `human_taken_over_at` / `human_taken_over_by` 字段
  - 新增 `MarkHumanTakenOverIfZero(appkey, ticketId, at, by)`：UPDATE … WHERE is_human_taken_over=0
- [ ] 2.4 `storages/storage.go`：暴露 `NewTicketEventStorage()`

# 3. 服务层

- [ ] 3.1 新增 `services/tickethumanhandoffservice.go`
  - `MarkHumanTakeover(ctx, appKey, ticketId, operatorId, operatorType string)`
  - 内部顺序：① `MarkHumanTakenOverIfZero` ② `ticketEventDao.Create` ③ 返回成功
  - 任意步骤失败：返回错误，调用方负责中断 IM 副作用
  - 配套单测 `services/tickethumanhandoffservice_test.go`
- [ ] 3.2 `agent/modules/message/service/inbound.go`
  - 命中 `MatchHandoffKeyword` 后，先 `service.markHumanTakeover(ctx, appKey, ticketID, customerID, "customer")`
  - 仅在 `markHumanTakeover` 成功（或幂等已置位）后再调 `service.humanHandoff`
  - 在 `agent/modules/message/service/service.go` 增加 `MarkHumanTakeoverFunc` 类型 + `SetMarkHumanTakeover` 注入点
  - `agent/bootstrap/module.go` 透传注入到 `services.tickethumanhandoffservice.MarkHumanTakeover`

# 4. API 层

- [ ] 4.1 `apis/models/ticket.go`
  - `TicketInfo` 加 `IsHumanTakenOver bool / HumanTakenOverAt int64 / HumanTakenOverBy string`
  - `QryTicketsReq` / `QryCustomerTicketsReq` 加 `IsHumanTakenOver *bool`
- [ ] 4.2 `apis/ticket.go`
  - `parseQryTicketsReq` / `parseQryCustomerTicketsReq` 解析 `is_human_taken_over`
  - `ticketsToAPI` 透出 3 个字段
  - `QryTickets` / `QryCustomerTickets` 在 storage 层加 `is_human_taken_over` 过滤（storage 接口与 DAO 同步扩展）
- [ ] 4.3 新增 `apis/ticketevent.go`
  - `QryTicketEvents` handler：仅 `apis.Validate`，返回 `TicketEventInfo` 列表
  - 单测 `apis/ticketevent_test.go`
- [ ] 4.4 `apis/models/ticketevent.go`（新）：
  - `TicketEventInfo` 定义 + `QryTicketEventsReq` / `QryTicketEventsResp`
- [ ] 4.5 `routers/router.go`
  - 注册 `GET /jmate/tickets/:ticket_id/events`（在 `QryTickets` 之后即可）

# 5. 测试

- [ ] 5.1 `storages/dbs/ticketeventdao_test.go`
- [ ] 5.2 `services/tickethumanhandoffservice_test.go`
- [ ] 5.3 `apis/ticketevent_test.go`
- [ ] 5.4 `apis/ticket_test.go`：补 `TicketInfo` 新字段透出断言
- [ ] 5.5 `agent/modules/message/service/inbound_test.go` 或 `handoff_keyword_test.go`：补「命中关键词 → 调 markHumanTakeover 优先」分支

# 6. 文档

- [ ] 6.1 `docs/ticket-api.md` 增补：
  - `TicketInfo` 表加 3 列
  - `QryTicketsReq` / `QryCustomerTicketsReq` 加 `is_human_taken_over` 字段说明
  - 新增 `GET /jmate/tickets/:ticket_id/events` 接口章节
