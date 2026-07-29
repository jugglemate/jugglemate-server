## Why

客户在 IM 群里触发"转人工"后，目前没有任何持久化痕迹能回答两个高频问题：

- 谁本月转人工多？转人工率高？哪些客户多次触发？
- 坐席是否真的接手了？转人工后多久响应？

现有可观察信号仅是 IM 群内 `jgm:ticketassign` (assign_type=3) 实时通知，群消息不可查询、不可统计、不能跨会话审计。本 change 补齐工单域的「转人工」持久化事实 + 事件流水，使上述两个问题直接 SQL 可答，同时为后续转人工率、转人工平均首次响应时长等数据统计能力提供基础。

## What Changes

- 给 `tickets` 表加 3 个字段：`is_human_taken_over`、`human_taken_over_at`、`human_taken_over_by`，表达「是否曾转人工」的二元事实（单调递增）。
- 新建 `ticket_events` 表记录工单事件流水，本 change 只强制落地 `event_type='human_takeover'`，其他枚举值预留。
- 新增 `services/tickethumanhandoffservice.go::MarkHumanTakeover`，在 `agent/modules/message/service/inbound.go` 命中转人工关键词后、原 `humanHandoff` 副作用前调用，先持久化、再做 IM 拉人/退群。
- `apis/ticket.go` 的 `QryTickets` / `QryCustomerTickets` 增加 `is_human_taken_over` 过滤项；`TicketInfo` 透出 3 个新字段。
- 新增 `GET /jmate/tickets/:ticket_id/events` 拉取事件流水。
- MySQL 与 Postgres 两套 DDL 同步更新。
- `docs/ticket-api.md` 增补字段和事件接口说明。

## Capabilities

### New Capabilities

- `ticket-human-takeover-mark`：工单是否转人工的持久化标记与事件审计。

## Impact

- 新增 service：`services/tickethumanhandoffservice.go`
- 新增 storage：`storages/dbs/ticketeventdao.go` + `storages/models/ticket.go` 增字段与接口
- 新增 handler：`apis/ticketevent.go`，路由 `/jmate/tickets/:ticket_id/events`
- 改动现有：`agent/modules/message/service/inbound.go` / `apis/models/ticket.go` / `apis/ticket.go` / `routers/router.go` / 两套 DDL / `docs/ticket-api.md`
- 不改动：console/、agent/modules/* 业务接口、main.go、auth 中间件、`services/ticketglobaltagservice.go`（本轮不动 IM 全局标签，预留后续 change）
