# 1. DB 迁移

- [x] 1.1 MySQL `commons/dbcommons/sqls/20260730.sql`：tickets 加 last_user_msg_at /
      closed_at；建 ticket_messages（含两层索引）、建 ticket_ratings（含 UNIQUE + assignee 索引）
- [x] 1.2 Postgres `agent/migrations/sql/000006_ticket_auto_close_and_csat.sql`：同步字段与两表
- [x] 1.3 `agent/migrations/migrator_test.go` 加 000006 断言（baseline 4→5→6）

# 2. 模型与存储

- [x] 2.1 `storages/models/ticket.go`：Ticket 加 LastUserMsgAt / ClosedAt；
      ITicketStorage 加 UpdateLastUserMsgAt / CloseByIdle
- [x] 2.2 `storages/models/ticketmessage.go` (新)：TicketMessage + ITicketMessageStorage
- [x] 2.3 `storages/models/ticketrating.go` (新)：TicketRating + ITicketRatingStorage + CsatAggregate
- [x] 2.4 `storages/dbs/ticketdao.go`：UpdateLastUserMsgAt（GREATEST/CASE 防回退） +
      CloseByIdle（WHERE 抢占）；toModel 加新字段映射
- [x] 2.5 `storages/dbs/ticketmessagedao.go` (新)：UpsertByMsgId + QryLastByTicket
- [x] 2.6 `storages/dbs/ticketratingdao.go` (新)：Create + FindByTicketAndCustomer +
      QryCsatByAssigneeAndTime；Create 翻译 UNIQUE 冲突为 ErrTicketRatingAlreadyExists
- [x] 2.7 `storages/dbs/dbhelpers.go` (新)：isErrRecordNotFound + isDuplicateKeyError
- [x] 2.8 `storages/storage.go`：暴露 NewTicketMessageStorage / NewTicketRatingStorage

# 3. IM SDK

- [x] 3.1 `agent/modules/message/imbot/manager.go`：SendCustomMessage(appKey,
      botUserID, targetID, channelType, msgType, payload) 通用方法

# 4. 服务层

- [x] 4.1 `services/csatinvitation.go`：CsatInvitationPayload / CsatReplyPayload /
      BuildCsatInvitationPayload / ValidateCsatReplyPayload
- [x] 4.2 `services/ticketratingrecorder.go`：RecordFromCustomMessage (webhook) +
      GetTicketRating (GET handler) + ErrCsatXxx 错误族 + OverrideTestRatingStorage
- [x] 4.3 `services/notifycsatinvitation.go`：NotifyCsatInvitation（按 ticket 查 inbox →
      调注入的 csatIMSender）
- [x] 4.4 `services/ticketautocloseservice.go`：StartAutoCloseTicker + runAutoCloseOnce
      + fetchAutoCloseCandidates 注入点 + ConfigFromAppConfig
- [x] 4.5 `services/ticketoutboundservice.go`：webhook 路由 jgm:csatreply 新分支
      + processCsatReplyCustomMessage
- [x] 4.6 `services/inboxagentservice.go`：GetInboxAgent 抽到包级 getInboxAgentFn

# 5. 装配

- [x] 5.1 `agent/bootstrap/module.go`：BotConnections() accessor
- [x] 5.2 `main.go`：装配 csatIMSender（桥接 IM SDK）+ 启动 AutoCloseTicker

# 6. API 与路由

- [x] 6.1 `apis/models/ticketrating.go`：TicketRatingInfo + QryTicketRatingResp
- [x] 6.2 `apis/ticketrating.go`：QryTicketRating handler（ctx 双路径读 AppKey）
- [x] 6.3 `routers/router.go`：新增 `GET /jmate/tickets/:ticket_id/rating`

# 7. 控制台统计

- [x] 7.1 `apis/models/stats.go`：AgentStatsItem 加 RatingCount + csat_pending 注释更新
- [x] 7.2 `services/console_stats_service.go`：GetStatsAgents SQL 增加
      `ticket_ratings` 聚合 LEFT JOIN；item.CSATAvg 从真实值读、CSATPending=false

# 8. 测试

- [x] 8.1 `storages/dbs/ticketdao_test.go`：TestTicketDaoAutoCloseMapping
- [x] 8.2 `storages/dbs/ticketmesratingdao_test.go` (新)：TicketMessage / TicketRating 模型映射
- [x] 8.3 `services/csatinvitation_test.go` (新)：Build + Validate schema
- [x] 8.4 `services/ticketratingrecorder_test.go` (新)：RecordFromCustomMessage 6 个场景
      + GetTicketRating 2 个场景
- [x] 8.5 `services/ticketoutboundservice_test.go`：追加 csatReply 5 个场景
- [x] 8.6 `services/ticketautocloseservice_test.go` (新)：runAutoCloseOnce 关闭逻辑 + 无 DB 安全
- [x] 8.7 `apis/ticketrating_test.go` (新)：handler 测试
- [x] 8.8 既有 mockTicketStorage 全部补 UpdateLastUserMsgAt / CloseByIdle 占位方法

# 9. 文档

- [x] 9.1 `docs/ticket-api.md`：新增 ticket_messages / ticket_ratings 表说明；
      jgm:csat / jgm:csatreply 协议；GET 接口
- [x] 9.2 `docs/console-stats-api.md`：csat_avg 字段语义更新；
      FRT 仍 pending；前端占位说明调整

# 10. 落地补充（与原 plan 的差异）

- [x] 10.1 **CSAT 通知通过 csatIMSender 注入桥接**：避免 services 持有 IM SDK，
       解耦 agent 平台模块与客服域
- [x] 10.2 **`source` 字段简化为 2 种枚举**：card_button / card_with_comment（不含
       reply/api）—— 评价只走 IM 通道
- [x] 10.3 **comment 超长采用截断而非拒收**：避免前端 emoji 多字节字符数错漏
- [x] 10.4 **handler 双路径读 AppKey**：`ctx.GetString(...)` 优先 +
       `ctxs.GetAppKeyFromCtx(ctx)` 兜底；修复 gin.Context 自定义类型 key
       不被 ctx.Value 查到的已知坑
- [x] 10.5 **`csat_notified_at` 字段 + 阶段 5 补发扫描**（追加）：解决 IM 临时失联
       时 jgm:csat 失败不重试的隐患；ticker 单次扫"已关闭但未发邀请"工单
       调 NotifyCsatInvitation + MarkCsatNotifiedOnce（CAS 防并发）。
       配套新增 DB 迁移 000007 / 索引 `(app_key, status, csat_notified_at)`。

# 11. 后续增量（追加的 csat_notified_at 落地的详细 tasks）

- [x] 11.1 MySQL `commons/dbcommons/sqls/20260731.sql`：tickets 加 csat_notified_at
       DATETIME(3) NULL + 索引 `(app_key, status, csat_notified_at)`
- [x] 11.2 Postgres `agent/migrations/sql/000007_ticket_csat_notified_at.sql`：同步
       字段与索引
- [x] 11.3 `agent/migrations/migrator_test.go`：baseline 列表 6 → 7
- [x] 11.4 `storages/models/ticket.go`：Ticket 加 CsatNotifiedAt；ITicketStorage 接口
       加 MarkCsatNotifiedOnce + QryTicketsNeedingCsatNotification
- [x] 11.5 `storages/dbs/ticketdao.go`：MarkCsatNotifiedOnce（CAS WHERE csat_notified_at IS NULL
       AND status=2）+ QryTicketsNeedingCsatNotification
- [x] 11.6 `apis/models/ticket.go`：TicketInfo 加 CsatNotifiedAt
- [x] 11.7 `services/ticketservice.go`：ticketsToAPI 透出 CsatNotifiedAt
- [x] 11.8 `services/ticketautocloseservice.go`：runAutoCloseOnce 阶段 5 调 runCsatRetryPass；
       runCsatRetryPass 扫描未通知工单并发 jgm:csat
- [x] 11.9 `services/csatretries_test.go`（新）：3 个补发场景测试（空候选、
       正常发送+标记、IM 失败不标）
- [x] 11.10 `docs/ticket-api.md`：TicketInfo 字段表加 csat_notified_at
- [x] 11.11 `openspec/changes/ticket-auto-close/proposal.md`：Implementation Notes 加第 5 项
       说明 csat_notified_at 重发设计
