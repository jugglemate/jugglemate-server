## Purpose

工单域提供自动关闭超时工单的能力，并把客户的工单评价（CSAT）通过 IM 自定义消息落地为可查询、可统计的数据。统计接口同步接通 csat_avg，FRT 字段保留 pending=true（不做）。

## Requirements

### Requirement: 工单超时字段

`tickets` 表 MUST 增加：

- `last_user_msg_at TIMESTAMPTZ`（MySQL: `DATETIME(3)`）：坐席/系统消息最后时间
- `closed_at TIMESTAMPTZ`：工单关闭时间
- 索引 `(app_key, last_user_msg_at)`

#### Scenario: 上线前工单缺这 2 字段

- WHEN migration 跑过
- THEN 历史工单的 `last_user_msg_at = NULL`，不会被 ticker 选中关闭
- AND `closed_at = NULL` 不影响前端展示

### Requirement: ticket_messages 表 metadata

`ticket_messages` 表 MUST 记录群消息 metadata（who/when/msg_id），不存正文。

#### Scenario: 客户文本消息入站

- WHEN webhook 收到客户在群里发任意消息
- THEN `ticket_messages` 写入一条 `(sender_role=customer, msg_id, msg_type, created_time)`
- AND `UNIQUE (app_key, msg_id)` 防 IM 重投重复

#### Scenario: 自定义消息入站

- WHEN 客户端/前端通过 imSdk 发送 `jgm:csatreply` / `jgm:csat` 等自定义 type
- THEN `ticket_messages` 也写入，`msg_type` 保留原始字符串
- AND 不走 telegram 出站（独立 jgm:csatreply 路由）

### Requirement: ticket_ratings 表

`ticket_ratings` 表 MUST：

- 列：`app_key` / `ticket_id` / `customer_id` / `assignee_id` / `rating (1..5)` /
  `comment (<=500)` / `source ('card_button' | 'card_with_comment')` / `created_time`
- 索引：`UNIQUE (app_key, ticket_id, customer_id)`、`(app_key, assignee_id, created_time)`

#### Scenario: 同一客户重复评分

- WHEN 同一 (customer, ticket) 已有评分
- THEN webhook 收到的二次评分被 UNIQUE 拒收
- AND service 判定 `errors.Is(err, dbs.ErrTicketRatingAlreadyExists)`
- AND webhook 返回 SUCCESS（不重投），不写第二条记录

### Requirement: 自动关闭超时工单

后台 ticker 周期 30s（默认），对 `status=1 AND last_user_msg_at < now - 5min` 的工单按
`UPDATE ... WHERE status=1 AND last_user_msg_at < cutoff` 抢占关闭。

#### Scenario: 工单超时被关闭

- WHEN ticker 命中满足条件的工单
- AND 其他实例没抢到（RowsAffected==1 才是自己赢的）
- THEN `tickets.status = 2`、`closed_at = now`、写 `ticket_events` 事件、slog info
- AND 关闭后**立刻**发 `jgm:csat` 评价邀请卡到该工单所在 IM 群

#### Scenario: 多实例竞争

- WHEN 同一工单几乎同时被两实例扫描到
- THEN 只有 1 个实例的 UPDATE RowsAffected==1、抢到关闭动作
- AND 另一实例 RowsAffected==0 直接 continue（已处理）
- AND 不发第二条 jgm:csat（也不重复写 ticket_events）

#### Scenario: 关闭后客户发消息

- WHEN 工单 status=2 之后客户又在群里打字
- THEN webhook 入站，**只**写 ticket_messages
- AND status 不变、不发 jgm:csat、不入库 rating

### Requirement: jgm:csat 邀请卡

`jgm:csat` 自定义消息 MUST 在工单关闭后立刻由服务端发送，含 1-5 评分选项与 ≤3 提示。

#### Scenario: 消息结构

- WHEN jgm:csat 发出
- THEN `msg_type = "jgm:csat"`、`msg_content.kind = "csat"`
- AND `csat.options[]` 至少含 1..5 评分项
- AND `csat.low_rating_threshold = 3` + `csat.reply_msg_type = "jgm:csatreply"`

#### Scenario: IM 发送失败

- WHEN jgm:csat 客户端发送失败
- AND 工单已经 status=2
- THEN 只 slog 错误；工单状态不回滚
- AND 不重试（IM 消息发出是副作用，不保证送达）

### Requirement: jgm:csatreply 评分写入

客户评价通过 IM 群消息 `jgm:csatreply` 送达 webhook。

#### Scenario: 客户发 jgm:csatreply

- WHEN webhook 收到 `msg_type=jgm:csatreply`
- AND `sender_role = customer`（= ticket.source_id）
- AND `msg_content.kind = "csatreply"` 且 rating ∈ [1,5]
- AND `msg_content.app_key / ticket_id` 与 webhook receiver 一致
- THEN `ticket_ratings` 写入一条 + `ticket_messages` 留痕
- AND source = `card_with_comment` (有 comment) / `card_button` (无 comment)

#### Scenario: 字段不一致 / schema 错误

- WHEN msg_content.kind != "csatreply" 或 rating 越界
- OR app_key/ticket_id 不与 ctx/receiver 一致
- OR sender 不是 ticket.source_id
- THEN webhook 返回 `APP_ParamError`（被 processWebhookMessage 静默处理）
- AND 不写 ticket_ratings、不留痕

#### Scenario: 重复评分

- WHEN UNIQUE 冲突
- THEN webhook 返回 SUCCESS，不重投，不写第二条

#### Scenario: 评论过长

- WHEN comment 字节数 > 500
- THEN 服务端截断到 500 字符（不拒收）后入库

### Requirement: 评分 GET 接口

`GET /jmate/tickets/:ticket_id/rating?customer_id=...` MUST 返回该客户对工单的评价（如有）。

#### Scenario: 已评返回

- WHEN 评价已存在
- THEN `data.rating = { ticket_id, customer_id, assignee_id, rating, comment, created_time }`

#### Scenario: 未评返回

- WHEN 评价不存在
- THEN `data.rating = null`
- AND 整体 code=0

### Requirement: stats 接通 csat_avg

`GET /jmate/console/stats/agents` 在时间窗内按坐席聚合 `csat_avg` / `rating_count`。

#### Scenario: 接入前 vs 接入后

- WHEN 接入前：csat_avg=null、csat_pending=true、rating_count 字段不存在
- AND 接入后：csat_avg=真实值 (float 或 null)、csat_pending=false、rating_count>=0
- AND 没评分时 csat_avg=null（区分 0 分）

#### Scenario: FRT 仍 pending

- WHEN frt_avg_ms / frt_pending 字段保留
- THEN frt_pending 仍为 true
- AND frt_avg_ms 仍为 null

## Implementation Notes（落地补充）

1. **CSAT 通知通过 `csatIMSender` 注入桥接**：services 不持有 IM SDK，由 main.go
   注入闭包 → botConnections.SendCustomMessage。
2. **`source` 枚举只有 2 种**：card_button / card_with_comment（不含 reply/api）。
3. **comment 长度采用宽容截断**：超 500 字符截断而非拒收。
4. **handler ctx 双路径读 AppKey**：`ctx.GetString(...)` 优先，`ctxs.GetAppKeyFromCtx(ctx)` 兜底。

## Phasing

| Phase | 范围 | Status |
|---|---|---|
| 1 | DB migration + 字段 + 自动关闭 + jgm:csat + jgm:csatreply webhook + stats csat_avg + GET 接口 | 本 change |
| 2 | FRT（首响时间）统计 | 后续 |
| 3 | agent / customer 级 CSAT 排行榜 + 评价后台 | 后续 |
| 4 | WhatsApp channel 适配 | 后续 |
