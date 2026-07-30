# 客服数据统计 API

本文档描述控制台「客服数据统计」「坐席列表」「客户列表」三个面板使用的数据统计接口。接口统一返回：

```json
{
  "code": 0,
  "msg": "success",
  "data": {}
}
```

非 `0` 的 `code` 表示失败。常见错误码：

| code | 说明 |
| --- | --- |
| `17001` | 缺少 `appkey` |
| `17002` | 应用不存在或 IM SDK 未初始化 |
| `17003` | 未登录或无权限 |
| `17005` | 参数错误 |
| `17006` | 服务内部错误 |

## 通用约定

### 路径与权限

三个接口统一挂载在 `/jmate/console/*` 下，调用方必须拥有**管理员角色**（`role=2`）。

### 鉴权 Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 调用 `/jmate/user/login` 接口返回的 `data.authorization` 字符串（不带 `Bearer ` 前缀，直接放到 Header） |

`Authorization` 必须对应角色为 2 的用户；非管理员调用会返回 `17003`。服务端从该 Header 解析出登录态后再校验角色。

### 时间窗

- 所有时间相关参数按 `Asia/Shanghai` 时区解析与对比，数据库时区也已固定为 `Asia/Shanghai`，天然对齐。
- 时间窗参数 `from` / `to` 接受 `YYYY-MM-DD` 字符串。
- 比较方式为「左闭右开」：`created_time >= from 00:00:00 AND created_time < (to+1) 00:00:00`。
- 非法格式返回 `17005`。

### 暂时未支持的字段

- `csat_avg`（坐席平均满意度） / `frt_avg_ms`（坐席首次响应毫秒数）：当前固定为 `null`，并附 `csat_pending: true` / `frt_pending: true` 标识尚未采集。

> `transferred_to_human` 已接通真实数据（2026-07-29 上线）：基于 `tickets.is_human_taken_over` 字段统计时间窗内新建工单中曾转人工的工单数。`transferred_to_human_pending` 永远为 `false`。字段上线前的历史工单 `is_human_taken_over` 默认为 `false`，因此上线后该指标从 0 开始累加，不做历史回填。

前端在渲染时可依据其他 `*_pending` 字段显示「数据采集中」占位。

---

## 客服数据统计概览

按时间窗返回会话总数、AI 解决率、新增会话趋势与渠道访问量排行，用于「客服数据统计」面板。

### 请求

`GET /jmate/console/stats/overview`

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 管理员登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `from` | string | 否 | `to` 减去 7 天 | 起始日期 `YYYY-MM-DD`（含） |
| `to` | string | 否 | 服务器当前日期 | 结束日期 `YYYY-MM-DD`（含），对应的 `created_time < to+1` |
| `granularity` | string | 否 | `day` | 趋势桶粒度，取值 `day` / `hour`。非法值返回 `17005` |
| `timezone` | string | 否 | `Asia/Shanghai` | 仅作文档化说明，实际时区固定为 `Asia/Shanghai` |

`from` 不可晚于 `to`，否则返回 `17005`。

### 请求示例

```bash
curl -G 'http://localhost:8050/jmate/console/stats/overview' \
  -H 'appkey: nsw3sue72begyv7y' \
  -H 'Authorization: <token from /jmate/user/login>' \
  --data-urlencode 'from=2026-07-20' \
  --data-urlencode 'to=2026-07-27' \
  --data-urlencode 'granularity=day'
```

### 成功响应

`granularity=day` 时：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "from": "2026-07-20",
    "to": "2026-07-27",
    "granularity": "day",
    "totals": {
      "total_sessions": 1234,
      "ai_resolved_sessions": 678,
      "ai_resolution_rate": 54.94,
      "transferred_to_human": 87,
      "transferred_to_human_pending": false,
      "open_sessions": 88,
      "closed_sessions": 1146
    },
    "new_sessions_trend": [
      { "bucket": "2026-07-20", "total": 120, "ai_resolved": 70, "transferred_to_human": 5 },
      { "bucket": "2026-07-21", "total": 150, "ai_resolved": 86, "transferred_to_human": 12 },
      { "bucket": "2026-07-22", "total": 200, "ai_resolved": 110, "transferred_to_human": 9 }
    ],
    "channel_ranking": [
      { "channel_type": "widget",    "count": 800, "percentage": 64.83 },
      { "channel_type": "juggleim", "count": 300, "percentage": 24.31 },
      { "channel_type": "telegram", "count": 134, "percentage": 10.86 }
    ]
  }
}
```

`granularity=hour` 时 `bucket` 字符串格式为 `YYYY-MM-DD HH`（24 小时制，0 补齐）：

```json
{
  "data": {
    "from": "2026-07-27",
    "to": "2026-07-27",
    "granularity": "hour",
    "new_sessions_trend": [
      { "bucket": "2026-07-27 09", "total": 5, "ai_resolved": 2, "transferred_to_human": 1 },
      { "bucket": "2026-07-27 10", "total": 11, "ai_resolved": 6, "transferred_to_human": 3 }
    ]
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `from` | string | 响应实际使用的起始日期 `YYYY-MM-DD` |
| `to` | string | 响应实际使用的结束日期 `YYYY-MM-DD` |
| `granularity` | string | 响应实际使用的趋势粒度 `day` / `hour` |
| `totals` | object | 时间窗内的总量统计 |
| `totals.total_sessions` | int64 | 时间窗内的工单总数 |
| `totals.ai_resolved_sessions` | int64 | AI 独立闭环的会话数（`status=2` 且 `assignee_id` 为空或空字符串） |
| `totals.ai_resolution_rate` | float | AI 解决率百分比，2 位小数。`total_sessions=0` 时为 `0`。公式：`ai_resolved_sessions / total_sessions × 100` |
| `totals.transferred_to_human` | int64 | 时间窗内新建工单中，曾触发转人工的工单数（基于 `tickets.is_human_taken_over` 字段） |
| `totals.transferred_to_human_pending` | bool | 当前始终为 `false`。保留是为前端统一处理各类 `*_pending` 标志 |
| `totals.open_sessions` | int64 | 时间窗内处于 `未关闭` 状态的工单数（`status ∈ {0,1,3}`） |
| `totals.closed_sessions` | int64 | 时间窗内已关闭工单数（`status=2`） |
| `new_sessions_trend` | array | 按 `granularity` 桶化的时间序列，按 `bucket` 升序排列 |
| `new_sessions_trend[].bucket` | string | 桶标签：`day` 为 `YYYY-MM-DD`，`hour` 为 `YYYY-MM-DD HH` |
| `new_sessions_trend[].total` | int64 | 该桶内的新增工单数 |
| `new_sessions_trend[].ai_resolved` | int64 | 该桶内 AI 独立闭环的工单数 |
| `new_sessions_trend[].transferred_to_human` | int64 | 该桶内转人工的工单数（按 `created_time` 分桶，与 total 同口径） |
| `channel_ranking` | array | 渠道访问量排行，按 `count` 降序 |
| `channel_ranking[].channel_type` | string | 渠道类型：`widget` / `telegram` / `juggleim` |
| `channel_ranking[].count` | int64 | 时间窗内该渠道的工单数 |
| `channel_ranking[].percentage` | float | 该渠道占总工单的百分比，2 位小数 |

### 计算规则

- `total_sessions` 统计 `tickets` 表中 `app_key` 等于当前应用、`created_time` 落在时间窗内的所有行。
- `ai_resolved_sessions` 在 `total_sessions` 的基础上进一步过滤 `status=2 AND (assignee_id IS NULL OR assignee_id='')`：表示「AI 接待到关闭，人工从未 Claim 过」的会话。
- `transferred_to_human` 在 `total_sessions` 的基础上过滤 `is_human_taken_over=true`：基于持久化字段统计，自 2026-07-29 上线后方开始累加，上线前历史工单该字段默认为 `false`，**不**做回填。
- `open_sessions` / `closed_sessions` 同样基于 `status` 字段过滤，与 AI / 转人工无关。
- `new_sessions_trend` 用 PostgreSQL `date_trunc('day' | 'hour', created_time)` 桶化，按桶聚合；空桶不会出现（每天 0 工单 → 无返回项）。`transferred_to_human` 沿用 `created_time` 分桶，与 `total` 同口径。
- `channel_ranking.percentage` = 该渠道 `count` / 所有渠道总 `count` × 100，2 位小数。

### 错误响应

| code | 场景 |
| --- | --- |
| `17001` | 缺少 `appkey` Header |
| `17003` | 未登录或当前用户不是管理员 |
| `17005` | `from` / `to` 不是 `YYYY-MM-DD` 格式，或 `from > to`，或 `granularity` 不是 `day` / `hour` |
| `17006` | 数据库查询失败 |

---

## 坐席列表

按时间窗返回客服坐席（或同时包含管理员）及其回复会话量，用于「坐席列表」面板。

### 请求

`GET /jmate/console/stats/agents`

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 管理员登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `from` | string | 否 | `to` 减去 7 天 | 起始日期 `YYYY-MM-DD`（含） |
| `to` | string | 否 | 服务器当前日期 | 结束日期 `YYYY-MM-DD`（含） |
| `include_admin` | bool | 否 | `true` | 是否包含角色为 `2` 的管理员。`false` 时只列出 `role=1` 的纯客服 |
| `page` | int | 否 | `1` | 页码，从 1 开始。`<1` 返回 `17005` |
| `pageSize` | int | 否 | `20` | 每页条数。`<1` 返回 `17005`；超过 `100` 会被服务端夹紧到 `100` |

非法布尔值（`true` / `false` 之外的字符串）返回 `17005`。

### 请求示例

```bash
curl -G 'http://localhost:8050/jmate/console/stats/agents' \
  -H 'appkey: nsw3sue72begyv7y' \
  -H 'Authorization: <token from /jmate/user/login>' \
  --data-urlencode 'from=2026-07-20' \
  --data-urlencode 'to=2026-07-27' \
  --data-urlencode 'include_admin=true' \
  --data-urlencode 'page=1' \
  --data-urlencode 'pageSize=20'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "user_id": "u_gc3kYabrl9M",
        "login_account": "helena",
        "nickname": "Helena",
        "avatar": "https://example.com/agent-helena.png",
        "email": "helena@acme.inc",
        "role": 1,
        "inbox_count": 3,
        "total_sessions": 142,
        "responded_sessions": 138,
        "open_sessions": 4,
        "csat_avg": null,
        "frt_avg_ms": null,
        "csat_pending": true,
        "frt_pending": true
      }
    ],
    "total": 8,
    "limit": 20,
    "offset": 0
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | 坐席用户列表，按 `total_sessions DESC NULLS LAST, nickname` 排序 |
| `items[].user_id` | string | 坐席用户 ID（`users.user_id`） |
| `items[].login_account` | string | 登录账号 |
| `items[].nickname` | string | 显示昵称 |
| `items[].avatar` | string | 头像 URL，可能为空 |
| `items[].email` | string | 邮箱，可能为空 |
| `items[].role` | int | 角色：`1` = 客服，`2` = 管理员 |
| `items[].inbox_count` | int64 | 该坐席被绑定的 Inbox 数量（`inboxmembers.member_id = user_id` 的行数） |
| `items[].total_sessions` | int64 | 时间窗内，被该坐席 `Claim` 过的工单总数（`tickets.assignee_id` = `user_id`） |
| `items[].responded_sessions` | int64 | 时间窗内被该坐席处理中的工单数（`status ∈ {1,2,3}` 且分配给本人） |
| `items[].open_sessions` | int64 | 时间窗内该坐席名下未关闭的工单数（`status ∈ {0,1,3}`） |
| `items[].csat_avg` | float/null | 客户满意度平均分（1-5）。当前固定 `null` |
| `items[].frt_avg_ms` | int/null | 首次响应平均耗时毫秒数。当前固定 `null` |
| `items[].csat_pending` | bool | `true` 表示尚未采集 CSAT |
| `items[].frt_pending` | bool | `true` 表示尚未采集 FRT |
| `total` | int64 | 满足 `include_admin` 过滤的坐席总人数（不受分页影响） |
| `limit` | int64 | 服务端实际使用的 `pageSize`，夹紧后值 |
| `offset` | int64 | 服务端计算出的偏移量 `= (page-1) × limit` |

### 计算规则

- 时间窗参数作用于 `tickets.created_time`，用户的 `inbox_count` 与 `include_admin` 过滤无关时间窗。
- `total_sessions` / `responded_sessions` / `open_sessions` 按 `tickets.assignee_id = user_id` + 时间窗聚合。
- `responded_sessions = COUNT(*) FILTER (WHERE status IN (1,2,3))`：表示被该坐席实际处理过的会话。
- `open_sessions = COUNT(*) FILTER (WHERE status IN (0,1,3))`：当前还挂在他名下的会话。
- `inbox_count` 来自 `inboxmembers` 全表统计，与时间窗无关。
- `total` 是 `users` 表里 `app_key` 匹配 + 满足 `include_admin` 过滤的总人数。

### 错误响应

| code | 场景 |
| --- | --- |
| `17001` | 缺少 `appkey` Header |
| `17003` | 未登录或当前用户不是管理员 |
| `17005` | `from` / `to` 格式错误、`include_admin` 不可解析、`page < 1` 或 `pageSize < 1` |
| `17006` | 数据库查询失败 |

---

## 客户列表

按时间窗返回有会话记录的客户，用于「客户列表」面板。

### 请求

`GET /jmate/console/stats/customers`

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 管理员登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `from` | string | 否 | `to` 减去 7 天 | 起始日期 `YYYY-MM-DD`（含） |
| `to` | string | 否 | 服务器当前日期 | 结束日期 `YYYY-MM-DD`（含） |
| `keyword` | string | 否 | 空 | 模糊匹配，对 `customers.nickname` / `customers.identifier` / `customers.phone` / `customers.email` 做大小写不敏感的子串匹配。空字符串表示不过滤 |
| `channel_type` | string | 否 | 空 | 只列出时间窗内至少有一张该渠道工单的客户，取值 `widget` / `telegram` / `juggleim`。空字符串表示不过滤 |
| `page` | int | 否 | `1` | 页码，从 1 开始。`<1` 返回 `17005` |
| `pageSize` | int | 否 | `20` | 每页条数。`<1` 返回 `17005`；超过 `100` 会被服务端夹紧到 `100` |

### 请求示例

```bash
curl -G 'http://localhost:8050/jmate/console/stats/customers' \
  -H 'appkey: nsw3sue72begyv7y' \
  -H 'Authorization: <token from /jmate/user/login>' \
  --data-urlencode 'keyword=alice' \
  --data-urlencode 'channel_type=widget' \
  --data-urlencode 'page=1' \
  --data-urlencode 'pageSize=20'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "customer_id": "5TdqzXPf4kQbvjc2IGpteG",
        "nickname": "Alice",
        "avatar": "https://example.com/customer-alice.png",
        "phone": "13900000000",
        "email": "alice@example.com",
        "identifier": "u_alice_xxx",
        "source": "widget",
        "session_count": 7,
        "last_session_time": 1785151073616
      }
    ],
    "total": 234,
    "limit": 20,
    "offset": 0
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | 客户列表，按 `last_session_time DESC NULLS LAST, customer_id DESC` 排序 |
| `items[].customer_id` | string | 客户 ID（`customers.customer_id`） |
| `items[].nickname` | string | 客户昵称 |
| `items[].avatar` | string | 客户头像 URL，可能为空 |
| `items[].phone` | string | 客户手机号，可能为空 |
| `items[].email` | string | 客户邮箱，可能为空 |
| `items[].identifier` | string | 客户在 IM 中的固定身份 ID（`customers.identifier`），不同渠道下保持一致 |
| `items[].source` | string | 时间窗内最近一次会话所属渠道：`widget` / `telegram` / `juggleim`。客户在时间窗内无任何工单时为空字符串 |
| `items[].session_count` | int64 | 时间窗内该客户的工单总数 |
| `items[].last_session_time` | int64 | 时间窗内最近一次会话的 `created_time`，毫秒时间戳；时间窗内无任何工单时为 `0` |
| `total` | int64 | 满足 `keyword` / `channel_type` 过滤的客户总人数（不受分页影响） |
| `limit` | int64 | 服务端实际使用的 `pageSize`，夹紧后值 |
| `offset` | int64 | 服务端计算出的偏移量 `= (page-1) × limit` |

### 计算规则

- 客户范围：当前 `app_key` 下的所有客户；`keyword` / `channel_type` 还会再过滤。
- `keyword` 通过 PostgreSQL `ILIKE '%keyword%'` 做子串匹配，对 `customers.nickname` / `identifier` / `phone` / `email` 任一命中即视为匹配（OR 连接）。
- `channel_type` 通过 `EXISTS (SELECT 1 FROM tickets WHERE ... AND channel_type=?)` 子查询过滤。
- `source` 取时间窗内该客户**最近一张**工单的 `channel_type`，由相关子查询 `ORDER BY created_time DESC LIMIT 1` 计算。
- `session_count` 与 `last_session_time` 同样基于时间窗内的 `tickets` 子查询聚合。
- `total` 是过滤后客户总数，与 `items.length` 解耦，便于前端正确分页。

### 错误响应

| code | 场景 |
| --- | --- |
| `17001` | 缺少 `appkey` Header |
| `17003` | 未登录或当前用户不是管理员 |
| `17005` | `from` / `to` 格式错误、`page < 1` 或 `pageSize < 1` |
| `17006` | 数据库查询失败 |

---

## 附录：客户端集成提示

### 获取管理员 Token

调三个接口前必须先调用登录接口拿到 Token：

```bash
curl -X POST 'http://localhost:8050/jmate/user/login' \
  -H 'appkey: nsw3sue72begyv7y' \
  -H 'Content-Type: application/json' \
  -d '{"account":"admin","password":"<admin_password>"}'
```

返回数据中取出 `data.authorization` 字符串，作为后续接口的 `Authorization` Header（**不带** `Bearer ` 前缀）。

### 占位字段渲染

`items[].csat_pending`、`items[].frt_pending` 这两个布尔字段供前端识别"数据采集中"。建议在面板中显示「—」或「数据采集中」徽章，避免误把 `0` / `null` 当成真实数据。

> `transferred_to_human_pending` 已在 2026-07-29 切到真实值，统一固定为 `false`。前端如果仍按 `*_pending` 做占位判断也兼容；建议前端逻辑里把这一项从「采集中」文案里去掉，转为直接渲染数字。

### 数据缓存与刷新

- 时间窗越长、过滤条件越多，SQL 成本越高；建议前端不要高频轮询（最低 30 秒）。
- 切换时间窗时无需新建 Token，复用同一个登录会话即可。
- 数据存在最长约 5 秒的最终一致性（PG 主从同步时间），同一时间窗内的总览数据与列表数据可能短暂不一致，建议前端不强制强一致性展示。
