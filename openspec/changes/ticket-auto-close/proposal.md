## Why

工单域目前没有自动关闭路径：客户群里触发"转人工"或坐席回复之后，没有机制把工单从
processing 推进到 closed。结果：

- 客服数据统计（`GET /jmate/console/stats/overview`）里 `open_sessions` 长期等于
  total，新增会话/转人工都无差别累计，无法体现真实工作负荷；
- 工单列表里"处理中"行越积越多，列表 UI 没有自然终点；
- 服务端没有任何"工单关闭"事件，前端的"已解决"tag、自定义的会话存档功能都缺底层信号。

新增 client 评价记录（CSAT）也是缺失的：管理员看不到"客户对坐席打分"，无法做
人员 / 团队绩效看板，**且**对应的评价数据没有持久化通道。

## What Changes

- 工单表新增 `last_user_msg_at` / `closed_at` 字段；新增 `ticket_messages` 表记录
  群消息元数据（who/when/msg_id），不存正文。
- 新增 `ticket_ratings` 表 + 服务端 Service，专用于存储客户对工单的人工服务评分
  （1-5 星 + 可选 comment）。
- 后台 ticker：每 30s 扫描一次工单表，对 `status=1` 且 `last_user_msg_at < now - 5min`
  的工单按抢占式 SQL 关闭（status→2、closed_at=now、写 ticket_events）。
- ticker 关闭工单后，**通过 IM SDK 发送一条** `jgm:csat` 自定义消息邀请客户评价。
- 客户评分全部走**自定义 IM 消息** `jgm:csatreply`（不经 HTTP 写接口）：
  - 前端 widget / juggleim 端卡片"5★"点击后 imSdk.sendGroupMsg(msg_type="jgm:csatreply",
    content={"rating":5,...})；
  - telegram 端暂不在本 change 适配；
  - webhook 入站收到 `jgm:csatreply` 后解析 `msg_content`、强校验 schema、写库。
- 新增只读 HTTP 接口 `GET /jmate/tickets/:ticket_id/rating?customer_id=...`，供前端
  UI 在展示评分卡前判断"已评过"。
- 客服 stats 接口 `GET /jmate/console/stats/agents` 接通 `csat_avg` / `rating_count`
  真实聚合；`csat_pending` 改 false；`frt_avg_ms` / `frt_pending` 仍保持 pending
  （下次 change 接 FRT）。

## Capabilities

### New Capabilities

- `ticket-auto-close`: 自动关闭超时工单（last_user_msg_at + 5min）。
- `ticket-csat`: IM 评价系统（jgm:csat 邀请 + jgm:csatreply 录入）。

## Impact

### DB

- MySQL `commons/dbcommons/sqls/20260730.sql`：tickets 加 2 字段、新增
  `ticket_messages` / `ticket_ratings` 两张表（含两层索引）。
- Postgres `agent/migrations/sql/000006_ticket_auto_close_and_csat.sql`：同步字段与表。

### 模型与存储

- `storages/models/ticket.go`: Ticket struct 加 `LastUserMsgAt` / `ClosedAt`；
  `ITicketStorage` 新增 `UpdateLastUserMsgAt` / `CloseByIdle` 方法。
- `storages/models/ticketmessage.go` (新): `TicketMessage` struct +
  `ITicketMessageStorage` 接口（UpsertByMsgId + QryLastByTicket）。
- `storages/models/ticketrating.go` (新): `TicketRating` struct +
  `ITicketRatingStorage` 接口（含 `QryCsatByAssigneeAndTime` 给 stats 聚合）。
- `storages/dbs/ticketdao.go`: 增 `UpdateLastUserMsgAt` / `CloseByIdle` 实现；
  增更新 `LastUserMsgAt` / `ClosedAt` 字段映射。
- `storages/dbs/ticketmessagedao.go` (新)、`storages/dbs/ticketratingdao.go` (新)。
- `storages/dbs/dbhelpers.go` (新): `isDuplicateKeyError` 跨方言 UNIQUE 冲突判定。
- `storages/storage.go`: 暴露 `NewTicketMessageStorage` / `NewTicketRatingStorage`。

### 服务层

- `services/csatinvitation.go` (新): `BuildCsatInvitationPayload` + `ValidateCsatReplyPayload` +
  `CsatReplyPayload` 结构。
- `services/ticketratingrecorder.go` (新): `RecordFromCustomMessage`
  （webhook 评分写入入口）+ `GetTicketRating`（GET 接口读）。
- `services/notifycsatinvitation.go` (新): `NotifyCsatInvitation`，查 ticket 拿到 inbox，
  调 `csatIMSender` 桥接函数发 jgm:csat。
- `services/ticketautocloseservice.go` (新): `StartAutoCloseTicker` 后台 ticker
  + `runAutoCloseOnce` 单次扫描 + 抢占式关闭 + ticket_events 写入
  + `NotifyCsatInvitation` 调用。
- `services/ticketoutboundservice.go`: webhook 路由新增 `jgm:csatreply` 分支
  （`processCsatReplyCustomMessage`），引入 `ErrCsatXxx` 错误族。
- `services/inboxagentservice.go`: 把 `GetInboxAgent` 实现抽到包级 `getInboxAgentFn`，
  便于单测注入 mock。

### IM SDK

- `agent/modules/message/imbot/manager.go`: 新增 `SendCustomMessage`，
  发任意 msg_type + 任意 JSON msg_content 的群消息。本 change 用来发 jgm:csat。

### 接入 / 装配

- `agent/bootstrap/module.go`: `Module.BotConnections()` accessor
  暴露 IM Bot 长连接管理器。
- `main.go`: 装配 `services.SetCsatIMSender(...)` 桥接 IM SDK +
  `services.StartAutoCloseTicker(...)` 启动后台关闭。

### API / 路由

- `apis/models/ticketrating.go` (新): `TicketRatingInfo` + `QryTicketRatingResp`。
- `apis/ticketrating.go` (新): `QryTicketRating` handler。
- `apis/ticketevent.go` QryTicketRating 走 ctx.GetString（先 gin 后 ctx）修补路径。
- `routers/router.go`: 新增 `GET /jmate/tickets/:ticket_id/rating`。

### 控制台统计

- `apis/models/stats.go`: `AgentStatsItem` 加 `RatingCount int64`；`csat_pending` 由
  `true` 改 `false`（字段注释同步）。
- `services/console_stats_service.go`: `GetStatsAgents` SQL 增加第二个 LEFT JOIN
  `ticket_ratings` 聚合 csat_avg 与 rating_count。

### 测试

- 新增 `storages/dbs/ticketmesratingdao_test.go`、`services/csatinvitation_test.go`、
  `services/ticketratingrecorder_test.go`、`services/ticketautocloseservice_test.go`、
  `services/ticketoutboundservice_test.go`（追加 csat 测试）、
  `apis/ticketrating_test.go`。
- `storages/dbs/ticketdao_test.go` 追加 `TestTicketDaoAutoCloseMapping`。
- `services/ticketservice_test.go` + `services/tickethumanhandoffservice_test.go` +
  `services/telegramwebhookservice_test.go` 的 mockTicketStorage 全部补
  `UpdateLastUserMsgAt` / `CloseByIdle` 占位方法。

### 文档

- `docs/ticket-api.md`: 新增 `ticket_messages` / `ticket_ratings` 表说明、
  `jgm:csat` / `jgm:csatreply` 协议、`GET /jmate/tickets/:id/rating` 接口。
- `docs/console-stats-api.md`: 同步 `csat_avg` / `rating_count` 语义
  （不再 pending，FRT 仍 pending）。

### 不在范围内（明确不做）

- ❌ 主动关闭 / 主动 reopen 工单 HTTP 接口：评价统一由系统自动推进。
- ❌ 退群 / 解散群：关闭工单仅写库 + IM 标签同步，群成员不动。
- ❌ WhatsApp / Telegram 出站消息格式适配：本期仅约定 jgm:csat 协议，
  telegram 出站忽略 `jgm:csatreply` 内容，UI 上呈现"请回复数字"提示文本。
- ❌ FRT（first response time）：stats 字段保留 pending=true。
- ❌ 评价写库管理后台 / 回复建议聚合 / 评价排行榜等运营能力。
- ❌ cron 框架、redis lock：单实例 ticker + SQL 抢占 UPDATE WHERE 兜底多实例竞态。

## Implementation Notes（落地补充）

最终代码与原 plan 的 5 处细化：

1. **CSAT 通知通过 `csatIMSender` 注入桥接**：services 包不持有 IM SDK 直接引用，
   由 main.go 注入闭包（闭包内调 `agentModule.BotConnections().SendCustomMessage`）。
   这样解耦 agent 平台模块与客服域的依赖。
2. **`ticket_ratings.source` 字段枚举只有两种**：`card_button` /
   `card_with_comment`，**不含** `reply` / `reply_with_comment` / `api`。
   评价落库路径只走 IM webhook，不暴露 HTTP 评分接口。
3. **comment 长度校验采用宽容策略**：超 500 字符时**截断**而非拒收，
   避免客户因为前端 emoji 等多字节字符数错漏评。
4. **handler ctx 读取 AppKey 的双路径**：`ctx.GetString(...)` 优先，
   `ctxs.GetAppKeyFromCtx(ctx)` 兜底。修复了 gin.Context.Value 不支持
   `type CtxKey string` 自定义 key 的已知问题（不依赖中间件时也工作）。
5. **`csat_notified_at` 字段 + 重发 worker**（追加）：发现 IM 临时失联
   时 jgm:csat 发送可能失败但工单已 status=2，**永远不会被重试**。
   修复方案：
   - `tickets` 加 `csat_notified_at TIMESTAMPTZ` 字段；
   - 后台 ticker 单次扫描多加一段（阶段 5）：扫"已关闭但 csat_notified_at IS NULL"的工单，
     调 NotifyCsatInvitation + MarkCsatNotifiedOnce（CAS 防并发）；
   - CloseByIdle 不再写 csat_notified_at，留给阶段 5 补发路径标记；
   - MarkCsatNotifiedOnce 用 SQL CAS 抢占：`WHERE csat_notified_at IS NULL AND status=2`，
     多实例下只有一个 UPDATE RowsAffected==1，其余跳过；
   - 客户 IM 失联时，1 ~ 5 分钟的恢复窗口内，下次 ticker 自动重发，
     不需要单独的 cron worker。

## 风险与边界

| 风险 | 影响 | 缓解 |
|---|---|---|
| 多实例后台 ticker 抢相同工单 | 同时关闭 / 重复发 jgm:csat | `UPDATE ... WHERE status=1 AND last_user_msg_at < cutoff` 的 RowsAffected 防多写 |
| 评价 webhook 重复投递 | 同一客户多次入库 | `UNIQUE (app_key, ticket_id, customer_id)`；重复时 err=ErrCsatAlreadyRated 静默 |
| jgm:csat 发送失败 | 客户收不到邀请卡 | 不影响关闭动作（已 status=2），slog 失败可后续人工补 |
| Telegram / WhatsApp 客户收 jgm:csatreply | 无法渲染自定义协议 | 渠道不解析，按纯文本呈现（"请回复数字"）；客户退回 widget 客户端评 |
| CSV / Excel 导出未覆盖 | 报表受限 | admin 走 stats csat_avg 聚合即可，原始数据可后续导出 |
