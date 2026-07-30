# 工单转人工标记 — 前端接入手册

> 本文档面向前端工程师，解释如何在前端展示「工单是否转人工」状态、如何查询事件流水，以及如何对接转人工相关统计。
>
> 后端落地说明见 `docs/ticket-api.md`；本手册的所有 endpoint、字段、参数都对应后端实际返回。

## 一句话总结

> **客服域给工单增加了"是否曾转人工"的事实字段 + 事件流水接口。前端只读即可，不要在本端写入。**

---

## 1. 接口改动列表

| 接口 | 改动 | 是否兼容 |
|---|---|---|
| `GET /jmate/tickets/list` | 响应新增 `is_human_taken_over` 等 3 字段；Query 新增 `is_human_taken_over` 过滤 | ✅ 向前兼容 |
| `GET /jmate/customers/tickets` | 响应新增 3 字段；Query 新增 `is_human_taken_over` 过滤 | ✅ 向前兼容 |
| `GET /jmate/tickets/:ticket_id/events` | **新增**，拉取工单事件流水 | — |
| `POST /jmate/tickets/:ticket_id/claim` | 响应 `TicketInfo` 自动带上 3 字段 | ✅ |
| `POST /jmate/tickets/:ticket_id/transfer` | 响应 `TicketInfo` 自动带上 3 字段 | ✅ |

`POST /jmate/tickets/...` 其他接口无变化。

---

## 2. 字段定义

### 2.1 `TicketInfo` 新增字段

```ts
interface TicketInfo {
  // ... 原有字段不变
  ticket_id: string;
  source_id: string;
  customer_id: string;
  customer?: UserInfo | null;
  inbox_id: string;
  channel_type: 'widget' | 'telegram' | 'juggleim';
  assignee_id: string;
  assignee?: UserInfo | null;
  status: 0 | 1 | 2;

  // ↓ 本次新增的 3 个字段
  is_human_taken_over: boolean;   // 是否曾转人工
  human_taken_over_at: number;    // 首次转人工的毫秒时间戳；未转人工时为 0
  human_taken_over_by: string;    // 首次转人工的触发者 ID（当前为 customer_id）

  // ...
  created_time: number;
  updated_time: number;
}
```

| 字段 | 类型 | 含义 | 触发后行为 |
|---|---|---|---|
| `is_human_taken_over` | `boolean` | 这条工单是否**曾经**被客户触发过转人工 | 始终 `false` 表示从未转人工；置成 `true` 之后**不可回滚** |
| `human_taken_over_at` | `number` (ms) | **首次**转人工的精确时间 | 转人工后该值即写入；后续重复触发**不更新** |
| `human_taken_over_by` | `string` | **首次**转人工的触发者 ID | 当前固定为客户的 `customer_id`；未来由坐席触发可填 `user_id` |

**核心不变量**：

- 这 3 个字段只记录**首次**事实，不随时间变化。
- 想看"这条工单**总共**触发过几次转人工"，用 `events` 接口（见 §4）。
- 想看"最近一次转人工时间"，目前**没有**字段（首次=最近）。如果以后需要，要新增 `last_human_takeover_at` 字段；现在不需要。

### 2.2 事件字段（`TicketEventInfo`）

```ts
interface TicketEventInfo {
  id: number;             // 事件 ID
  ticket_id: string;
  event_type: 'human_takeover' | 'claim' | 'transfer' | 'close' | 'reopen';
  operator_id: string;    // 触发者 ID。客户时为 customer_id
  operator_type: 'customer' | 'user' | 'system';
  payload: string;        // 原始 JSON 字符串，目前恒为 '{"trigger":"keyword"}'
  created_time: number;   // 毫秒时间戳
}
```

**当前已写入的只有 `event_type='human_takeover'`**。其他枚举值预留，本次不写数据。

---

## 3. 列表过滤：筛"已转人工" / "未转人工" 工单

### 3.1 主工单列表

```http
GET /jmate/tickets/list?is_human_taken_over=true&limit=20
GET /jmate/tickets/list?is_human_taken_over=false&limit=20
GET /jmate/tickets/list?is_human_taken_over=1&limit=20       # 等价于 true
GET /jmate/tickets/list?is_human_taken_over=0&limit=20       # 等价于 false
```

参数解析支持 `true` / `false` / `1` / `0`。非法值（`abc`、`2`、`-1` 等）返回 `17005`。

**不传** = 不过滤（与老版本行为完全一致）。

### 3.2 客户维度的工单列表

```http
GET /jmate/customers/tickets?customer_id=c_123&is_human_taken_over=true&limit=20
```

语义同上。

### 3.3 TypeScript 调用示例

```ts
// 通用 fetcher
async function qryTickets(params: {
  status?: 0 | 1 | 2;
  is_human_taken_over?: boolean;
  limit?: number;
  offset?: number;
}) {
  const search = new URLSearchParams();
  if (params.status !== undefined) search.set('status', String(params.status));
  if (params.is_human_taken_over !== undefined) {
    search.set('is_human_taken_over', params.is_human_taken_over ? 'true' : 'false');
  }
  if (params.limit) search.set('limit', String(params.limit));
  if (params.offset) search.set('offset', String(params.offset));

  const res = await fetch(`/jmate/tickets/list?${search.toString()}`, {
    headers: { appkey: APP_KEY, Authorization: token },
  });
  const json = await res.json();
  return json.data; // { items: TicketInfo[], total: number, limit, offset }
}
```

---

## 4. 事件流水接口

### 4.1 拉取

```http
GET /jmate/tickets/:ticket_id/events?limit=50&offset=0
```

- `limit` 默认 50，1-100。
- `offset` 默认 0。
- 按 `id desc` 倒序（新事件在前）。
- 任何登录用户都能查（仅依赖 `apis.Validate` + 同 appkey）。

### 4.2 响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "id": 1024,
        "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
        "event_type": "human_takeover",
        "operator_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
        "operator_type": "customer",
        "payload": "{\"trigger\":\"keyword\"}",
        "created_time": 1782360180000
      }
    ],
    "next_start_id": 1024
  }
}
```

### 4.3 TypeScript 示例

```ts
async function qryTicketEvents(ticketId: string, limit = 50, offset = 0) {
  const search = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  const res = await fetch(`/jmate/tickets/${ticketId}/events?${search}`, {
    headers: { appkey: APP_KEY, Authorization: token },
  });
  const json = await res.json();
  return json.data as { items: TicketEventInfo[]; next_start_id: number };
}
```

---

## 5. UI 落地建议

### 5.1 列表卡片 / 表格

工单对象本来就有 `is_human_taken_over` 字段，**不需要**额外请求。展示建议：

```tsx
function TicketStatusBadge({ ticket }: { ticket: TicketInfo }) {
  if (ticket.is_human_taken_over) {
    return (
      <span className="badge badge-orange" title="客户曾触发转人工">
        已转人工 · {formatTime(ticket.human_taken_over_at)}
      </span>
    );
  }
  return null;
}
```

**注意**：

- `human_taken_over_at` 是**首次**时间，不是最近一次。
- `human_taken_over_by` 是首次触发者的 ID。如果想展示"是谁先说转人工的"，需要再查 `customer_id` 对应的客户资料。

### 5.2 详情页抽屉 / Tab

新增 Tab：**事件历史**。调用 `GET /jmate/tickets/:ticket_id/events`。

```tsx
function TicketEventsTab({ ticketId }: { ticketId: string }) {
  const [items, setItems] = useState<TicketEventInfo[]>([]);
  useEffect(() => {
    qryTicketEvents(ticketId).then((d) => setItems(d.items));
  }, [ticketId]);

  return (
    <Timeline>
      {items.map((e) => (
        <Timeline.Item
          key={e.id}
          label={formatTime(e.created_time)}
          type={e.event_type}
        >
          {e.event_type === 'human_takeover' && '客户触发转人工'}
          {e.event_type === 'claim' && '坐席接单'}
          {e.event_type === 'transfer' && '转接'}
          {e.event_type === 'close' && '关闭'}
          {e.event_type === 'reopen' && '重新打开'}
          {e.operator_id && <span> · 操作者: {e.operator_id}</span>}
        </Timeline.Item>
      ))}
    </Timeline>
  );
}
```

### 5.3 重复触发次数

> 同一工单可能被客户多次触发转人工。**`is_human_taken_over` 不变**，但 `ticket_events` 表会有多条记录。

```ts
const takeoverCount = events.filter((e) => e.event_type === 'human_takeover').length;
```

> 不要用 `human_taken_over_at` 字段判断次数——它只记首次。

### 5.4 实时性

事件流水是**异步写入**的，从客户触发到出现在 events 接口可能延迟 ≤ 1s（DB 同步）。如果需要实时提示，还是看 IM 群消息 `jgm:ticketassign`（assign_type=3）—— 这个没变。

---

## 6. 转人工统计：用 `/jmate/tickets/list` 自己 count

**结论：转人工统计直接基于 `is_human_taken_over` 字段做，不需要新接口。**

后端**没有**单独的 `/jmate/console/stats/human-takeover` 等统计接口。前端若要做看板，最简单的方案是：

```ts
// 1. 拉总数（含全部状态）
const totalResp = await qryTickets({ limit: 1 }); // 拿到 total

// 2. 拉已转人工的总数
const takenResp = await qryTickets({
  is_human_taken_over: true,
  limit: 1,
});

// 3. 转人工率
const rate = takenResp.total / totalResp.total;
```

需要时间段筛选？加 `status` 配合：`status=1` 是处理中、`status=2` 是关闭。

### 6.1 直接 SQL 版的统计（推荐做法）

如果前端要画看板，很可能需要更细的维度（按天、按客户、按坐席）。这要后端写 SQL / 接口，不在本次 change 范围。下面给现成的 SQL 供参考对接后端同学：

```sql
-- 转人工率（按时间段）
SELECT
  COUNT(*) FILTER (WHERE is_human_taken_over) AS taken_over,
  COUNT(*) AS total,
  ROUND(COUNT(*) FILTER (WHERE is_human_taken_over) * 100.0 / NULLIF(COUNT(*), 0), 2) AS rate_pct
FROM tickets
WHERE app_key = 'app_xxx'
  AND created_time >= 1722470400000   -- ms
  AND created_time <  1725148800000;

-- 重复触发转人工的客户（说明体验差）
SELECT customer_id, COUNT(*) AS cnt
FROM tickets
WHERE app_key = 'app_xxx' AND is_human_taken_over
GROUP BY customer_id HAVING COUNT(*) > 1
ORDER BY cnt DESC;

-- 转人工 Top 客户
SELECT customer_id, COUNT(*) AS cnt, MIN(human_taken_over_at) AS first_at
FROM tickets
WHERE app_key = 'app_xxx' AND is_human_taken_over
GROUP BY customer_id
ORDER BY cnt DESC LIMIT 20;

-- 平均首次转人工耗时（秒）
SELECT
  AVG(EXTRACT(EPOCH FROM (human_taken_over_at - created_time))) AS avg_seconds
FROM tickets
WHERE app_key = 'app_xxx' AND is_human_taken_over;

-- 单日转人工量（按工单 created_time 分桶，与 overview 接口同口径）
SELECT
  date_trunc('day', created_time) AS day,
  COUNT(*) FILTER (WHERE is_human_taken_over) AS cnt
FROM tickets
WHERE app_key = 'app_xxx'
GROUP BY day
ORDER BY day;
```

> 如果这些统计需要走 HTTP 接口，请给后端同学单独提一个 change，**当前 change 不阻塞**。

### 6.2 后端已接通的统计接口

2026-07-29 起，`GET /jmate/console/stats/overview` 的 `transferred_to_human` 字段已接通真实数据：

- `totals.transferred_to_human`：时间窗内新建工单中曾转人工的工单数
- `totals.transferred_to_human_pending`：始终 `false`（保留字段为统一 `*_pending` 前端逻辑）
- `new_sessions_trend[].transferred_to_human`：按 `created_time` 分桶的转人工工单数

Stats 接口的转人工口径与 §6.1 第 1 段 SQL 完全一致，前端可以直接用。详细字段说明见 `docs/console-stats-api.md`。

> 该字段上线前的历史工单 `is_human_taken_over` 默认为 `false`，因此上线**后**转人工数从 0 开始累加，不做回填。如果前端需要"该字段上线以来累计" / "假定上线前的历史回填" 等口径，需要再起一个 change。

---

## 7. 兼容性 & 注意事项

| 事项 | 说明 |
|---|---|
| 老客户端 | 不读 `is_human_taken_over` 等字段不影响功能 |
| 不传 `is_human_taken_over` | 行为完全等价于老版本 |
| 字段只在转人工后才有意义 | `false` / `0` / `""` 表示从未转人工，**不代表错误** |
| `human_taken_over_at` 只记首次 | 重复触发不更新；要看"最近一次"用 events 接口 |
| `human_taken_over_by` 当前语义 | 客户时为 `customer_id`，未来由坐席触发可填 `user_id` |
| 转人工副作用顺序 | DB 写入 → IM 拉人 → 广播通知 → 退 Bot。**DB 失败时 IM 副作用仍会执行**（系统降级为保证客户有人应答），但 `is_human_taken_over` 会保持 `false`，需要事后人工补偿 |
| events 接口权限 | 任何登录用户，只要拿到 appkey + token 就能查（同应用内） |
| 翻页语义 | events 接口用 `limit + offset`，不是 keyset。`next_start_id` 字段保留，目前仅作参考 |

---

## 8. 上线 Checklist

- [ ] 后端跑过 `commons/dbcommons/sqls/20260729.sql`（MySQL）
- [ ] 后端跑过 `agent/migrations/sql/000005_ticket_human_takeover_mark.sql`（Postgres）
- [ ] 后端部署后，`GET /jmate/tickets/list` 响应包含 `is_human_taken_over` 等 3 字段
- [ ] 前端发版：在列表卡片/表格加"已转人工"tag（可选；可以分两期）
- [ ] 前端发版：在工单详情页加"事件历史"tab（可选 Second Wave）
- [ ] 通知后端同学：转人工统计 SQL（如需 HTTP 接口）

---

## 9. FAQ

**Q: 客户退出群后还能查到 `is_human_taken_over` 吗？**
A: 能。Bot 退群只影响 IM 实时消息，DB 数据不受影响。

**Q: 工单关闭后再次被 Agent 接待（bot 重拉回来），`is_human_taken_over` 会被清空吗？**
A: 不会。该字段单调递增。

**Q: 这种字段会被改回 `false` 吗？**
A: 不会。本设计明确"事实不可回滚"。如果将来需要"取消转人工"语义（新业务），需要新增 `current_human_taken_over` 字段，与 `is_human_taken_over` 区分。

**Q: 旧系统里 `console_stats_service.go` 已经提过 "tickets 表没有 handover_at 标记" — 现在有了吗？**
A: 有，新增字段 `human_taken_over_at`，对应 `handover_at` 的语义。但**没有自动接入 stats 聚合**，需要单独提需求。

**Q: events 接口能查多久的数据？**
A: 不会主动清理，看客户 PostgreSQL / MySQL 的归档策略。如需冷数据归档请单独提需求。

**Q: 我能不能拿 `is_human_taken_over` 当"是否处理中"的判据？**
A: 不能。`status=1` 才是处理中。`is_human_taken_over` 是历史事实。两者正交：

| status | is_human_taken_over | 含义 |
|---|---|---|
| `0` | `false` | 新建工单，未转人工 |
| `1` | `false` | 处理中，未转人工 |
| `1` | `true` | 处理中，曾转人工（典型：坐席接管的人工工单） |
| `2` | `true` | 已关闭，曾转人工 |

