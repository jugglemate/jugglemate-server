# Agent 接口说明（前端对接）

本文覆盖前端控制台需要的四个接口：

1. `GET /jmate/agentapi/agents/active` —— 获取已激活的 Agent 列表（可带工单绑定状态）
2. `POST /jmate/agentapi/agents/sessions/bind` —— 绑定工单与 Agent
3. `POST /jmate/agentapi/agents/sessions/unbind` —— 解除工单与 Agent 的绑定
4. `POST /jmate/agentapi/chat/stream` —— 与 Agent 流式对话（SSE）

> 说明：`/jmate/agentapi/*` 是控制台登录态入口，由 `routers/router.go` 转发到 Agent 模块。
> Agent 模块自身的原生入口是 `/api/v1/*`（走内部密钥鉴权），前端不要直接调。

---

## 通用约定

### 鉴权

这些接口都要求控制台登录态，请求头：

| Header | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 应用 AppKey，缺失直接返回 AppKey 错误 |
| `Authorization` | 是 | 控制台登录后拿到的 token |

未登录或登录过期返回 `code: "401_UNAUTHORIZED"`。

### 响应信封

**非流式接口的 HTTP 状态码恒为 200**，真实语义状态放在响应体的 `code` 字段和
`X-Original-Status` 响应头里。**前端必须用 `code` 判断成败，不能用 HTTP 状态码。**

成功：

```json
{ "code": 0, "msg": "success", "data": { } }
```

失败：

```json
{ "code": "400_INVALID_REQUEST", "msg": "page 或 pageSize 超出范围", "data": null }
```

---

## 1. 获取已激活 Agent 列表

```
GET /jmate/agentapi/agents/active
```

返回**当前应用下**全部 `status = active` 的 Agent。与 `GET /jmate/agentapi/agents` 有两点区别：

1. **状态口径**：后者会把 `draft`、`paused`、`archived` 一并返回（只排除 `deleted`），本接口只留 `active`。
2. **归属范围**：后者只返回当前登录人自己的 Agent，本接口返回同一 AppKey 下**任意 Owner** 创建的
   Agent —— 因为它的用途是「给工单挑一个可用 Agent」，不应受创建者限制。多租户隔离仍由 AppKey 保证。

传入 `session_id` 时，每个 Agent 会额外带上 `binded`，标识它是否已绑定该工单，用于在列表里回显当前选中项。

### 请求参数（Query）

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | int | 否 | 1 | 页码，从 1 开始 |
| `pageSize` | int | 否 | 20 | 每页条数，取值 1~100 |
| `session_id` | string | 否 | - | 工单 ID（`ticket_` 开头）。传入后每项返回 `binded`；不传则 `binded` 恒为 `false` |

`page < 1`、`pageSize < 1` 或 `pageSize > 100` 会返回 `400_INVALID_REQUEST`。

`session_id` 传了不存在的工单不会报错，只是所有项的 `binded` 都是 `false`。

### 响应 `data`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | Agent 列表，见下表 |
| `total` | int | 符合条件的总数 |
| `page` | int | 当前页码 |
| `pageSize` | int | 当前每页条数 |

`items[]` 元素：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `agentId` | string | Agent ID，调对话接口时传这个 |
| `agentName` | string | Agent 名称 |
| `ownerId` | string | 归属者 ID |
| `ownerName` | string | 归属者展示名 |
| `ownerAvatar` | string \| null | 归属者头像 |
| `primaryModel` | string | 主模型名，未配置时为 `"unknown"` |
| `status` | string | 固定为 `"active"` |
| `riskFlag` | string | `normal` / `high_load`（调用量 ≥ 1000）/ `quota_limit`（失败率 ≥ 20%） |
| `createdAt` | string | 创建时间，RFC3339 |
| `publishedAt` | string \| null | 激活时间；未激活过则回落为更新时间 |
| `knowledgeCount` | int | 已挂载知识库数量 |
| `toolCount` | int | 已挂载工具数量 |
| `binded` | bool | 该 Agent 是否已绑定请求中的 `session_id` 工单；未传 `session_id` 时恒为 `false` |

### 注意

- 列表严格按当前 AppKey 隔离，不会额外注入系统级 Agent。
- 排序为 `updatedAt` 倒序。
- 一个工单同时只能绑定一个 Agent，因此**整个列表里最多有一项 `binded === true`**。
  注意它可能不在当前页——绑定的 Agent 恰好排在第 3 页时，第 1 页会全是 `false`。
  前端如果要「回显已选中项」，建议不要依赖翻页查找，直接用列表接口返回的 `binded` 做勾选状态即可。

### 示例

```bash
curl -H "appkey: $APPKEY" -H "Authorization: $TOKEN" \
  "https://<host>/jmate/agentapi/agents/active?page=1&pageSize=20&session_id=ticket_aZrCeNMUk3A8JncZd7wp25"
```

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "agentId": "053793b9-d514-4743-ba14-fdba26057e12",
        "agentName": "哈哈哈",
        "ownerId": "u_Vnr45oakdbL",
        "ownerName": "Owner u_Vnr45oakdbL",
        "ownerAvatar": null,
        "primaryModel": "gpt-4o-mini",
        "status": "active",
        "riskFlag": "normal",
        "createdAt": "2026-07-17T18:39:08Z",
        "publishedAt": "2026-07-17T18:39:11Z",
        "knowledgeCount": 2,
        "toolCount": 1,
        "binded": true
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20
  }
}
```

---

## 2. 绑定工单与 Agent

```
POST /jmate/agentapi/agents/sessions/bind
Content-Type: application/json
```

把一个工单关联到指定 Agent，用于控制台在工单维度选择 Agent。

> **这个接口只写绑定关系**，不产生任何 IM 侧动作——不改 Ticket 群成员、不拉入或移出 Bot、
> 也不影响 Bot 自动回复的路由（后者由 Inbox 级绑定决定）。它的作用是记录「这个工单选了哪个
> Agent」，供前端后续自行使用。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `sessionId` | string | 是 | 工单 ID（`ticket_` 开头），最长 64 |
| `agentId` | string | 是 | 目标 Agent ID，最长 64 |

### 响应 `data`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `sessionId` | string | 回显工单 ID |
| `agentId` | string | 回显绑定的 Agent ID |
| `binded` | bool | 成功时恒为 `true` |

### 语义

- **一个工单同时只能绑定一个 Agent。** 对同一工单重复调用属于**换绑**，直接覆盖旧记录，
  不会报冲突、也不会留下多条记录。前端切换选择时直接调用即可，无需先解绑。
- 目标 Agent 必须是 `active` 且属于当前 AppKey。
- 如需清除当前选择，调用下方的 `sessions/unbind` 接口。

### 错误码

| `code` | 说明 |
| --- | --- |
| `400_INVALID_REQUEST` | `sessionId` 或 `agentId` 为空 |
| `404_TICKET_NOT_FOUND` | 工单不存在，或不属于当前 AppKey |
| `404_AGENT_NOT_BINDABLE` | Agent 不存在、未激活，或不属于当前应用 |

### 示例

```bash
curl -X POST "https://<host>/jmate/agentapi/agents/sessions/bind" \
  -H "appkey: $APPKEY" -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sessionId":"ticket_aZrCeNMUk3A8JncZd7wp25","agentId":"053793b9-d514-4743-ba14-fdba26057e12"}'
```

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "sessionId": "ticket_aZrCeNMUk3A8JncZd7wp25",
    "agentId": "053793b9-d514-4743-ba14-fdba26057e12",
    "binded": true
  }
}
```

### 典型用法

```ts
// 1. 打开工单时拉列表，binded 即为当前选中项
const { data } = await get(`/jmate/agentapi/agents/active?session_id=${ticketId}&pageSize=100`)
const selected = data.items.find(item => item.binded)

// 2. 用户切换选择，直接绑定（换绑无需先解绑）
await post("/jmate/agentapi/agents/sessions/bind", { sessionId: ticketId, agentId: nextAgentId })
```

---

## 3. 解除工单与 Agent 绑定

```
POST /jmate/agentapi/agents/sessions/unbind
Content-Type: application/json
```

清除工单当前选择的 Agent。该接口只删除 `ticket_agent_bindings` 中的选择关系，不移出 Bot、
不修改 Ticket 群成员，也不改变 Inbox 级自动回复路由。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `sessionId` | string | 是 | 要解除绑定的工单 ID，最长 64 |

### 响应 `data`

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `sessionId` | string | 回显工单 ID |
| `agentId` | string | 本次被解除的 Agent ID；原本未绑定时为空字符串 |
| `binded` | bool | 成功时恒为 `false` |

### 语义

- 接口按当前 AppKey 隔离，不会删除其他应用下同名工单的绑定。
- 接口是幂等的：工单不存在、没有绑定或重复解绑均返回成功，`agentId` 为空字符串。
- 解绑与换绑不同；换绑仍可直接调用 `sessions/bind`，无需先解绑。

### 错误码

| `code` | 说明 |
| --- | --- |
| `422` | JSON 格式错误，或 `sessionId` 为空、超过 64 个字符 |

### 示例

```bash
curl -X POST "https://<host>/jmate/agentapi/agents/sessions/unbind" \
  -H "appkey: $APPKEY" -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sessionId":"ticket_aZrCeNMUk3A8JncZd7wp25"}'
```

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "sessionId": "ticket_aZrCeNMUk3A8JncZd7wp25",
    "agentId": "053793b9-d514-4743-ba14-fdba26057e12",
    "binded": false
  }
}
```

---

## 4. 流式对话（SSE）

```
POST /jmate/agentapi/chat/stream
Content-Type: application/json
```

响应为 `text/event-stream`，逐条推送推理事件。响应头带 `X-Accel-Buffering: no`，
Nginx 不会缓冲。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `agentId` | string | 是 | 目标 Agent ID，取自上面的列表接口 |
| `userId` | string | 是 | 终端用户标识，用于会话归属与长期记忆 |
| `message` | string | 是 | 用户本轮输入 |
| `conversationId` | string | 否 | 续接已有会话；不传则新建 |
| `enableHistoryContext` | bool | 否 | 是否注入历史上下文 |
| `traceId` | string | 否 | 链路追踪 ID；不传由服务端生成并在每个事件里回带 |
| `metadata` | object | 否 | 业务自定义透传字段 |

参数不合法（缺必填等）时**不会**进入流，而是返回普通 JSON 信封，`code` 为 `422`/`400` 系列。

### 事件格式

每条事件一行 `data:`，紧跟一个空行：

```
data: {"event":"started","trace_id":"..."}

```

事件对象字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `event` | string | 事件类型，见下 |
| `trace_id` | string | 链路 ID，全程一致 |
| `conversation_id` | string | 会话 ID，`started` 事件可能还没有 |
| `session_id` | string | 推理会话 ID，按需出现 |
| `round_number` | int | 推理轮次，按需出现 |
| `confidence_score` | float | 置信度，出现在 `done` |
| `payload` | object | 事件负载，结构随 `event` 变化 |

四种事件：

| `event` | 时机 | `payload` |
| --- | --- | --- |
| `started` | 流开始，恒为第一条 | 无 |
| `token` | 每产出一段文本 | `{"token": "增量文本"}` |
| `done` | 推理正常结束 | `{"answer": "完整回答", "path": "...", "status": "...", "error_code": null}` |
| `error` | 出错 | `{"code": "错误码", "message": "错误描述"}` |

**结束标记**：无论成功失败，最后恒为

```
data: [DONE]

```

### 前端处理要点

- `token` 是**增量**，需要自行累加拼接；`done.payload.answer` 是完整回答，可用于校对最终文本。
- `error` 事件最多出现一次——服务端已做去重，不会既发业务错误又发 HTTP 层错误。
- 收到 `data: [DONE]` 才算流结束，此时应关闭读取循环。`[DONE]` 不是 JSON，解析前要先判断。
- 该接口是 **POST**，浏览器原生 `EventSource` 只支持 GET，请用 `fetch` + `ReadableStream`
  或 `@microsoft/fetch-event-source` 之类的库。

### 示例

```bash
curl -N -X POST "https://<host>/jmate/agentapi/chat/stream" \
  -H "appkey: $APPKEY" -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agentId":"053793b9-d514-4743-ba14-fdba26057e12","userId":"u_123","message":"你好"}'
```

```
data: {"event":"started","trace_id":"7f3a..."}

data: {"event":"token","trace_id":"7f3a...","conversation_id":"c_88","payload":{"token":"你"}}

data: {"event":"token","trace_id":"7f3a...","conversation_id":"c_88","payload":{"token":"好"}}

data: {"event":"done","trace_id":"7f3a...","conversation_id":"c_88","confidence_score":0.92,"payload":{"answer":"你好，有什么可以帮你？","path":"react","status":"succeeded","error_code":null}}

data: [DONE]

```

前端最小实现：

```ts
const response = await fetch("/jmate/agentapi/chat/stream", {
  method: "POST",
  headers: { "Content-Type": "application/json", appkey, Authorization: token },
  body: JSON.stringify({ agentId, userId, message }),
})
const reader = response.body!.getReader()
const decoder = new TextDecoder()
let buffer = ""
let answer = ""
while (true) {
  const { done, value } = await reader.read()
  if (done) break
  buffer += decoder.decode(value, { stream: true })
  const lines = buffer.split("\n\n")
  buffer = lines.pop() ?? ""
  for (const line of lines) {
    const raw = line.replace(/^data: /, "").trim()
    if (!raw) continue
    if (raw === "[DONE]") return answer
    const event = JSON.parse(raw)
    if (event.event === "token") answer += event.payload.token
    if (event.event === "error") throw new Error(event.payload.message)
    if (event.event === "done") answer = event.payload.answer
  }
}
```

---

## 相关接口

- `POST /jmate/agentapi/chat/completion` —— 同参数的非流式版本，返回
  `{conversationId, answer, path, status, traceId, errorCode}`。
- `POST /jmate/agentapi/chat/app/stream` —— 应用直连流式，鉴权口径不同（需 `X-User-Id` 头），
  不供控制台前端使用。
- `GET /jmate/agentapi/agents` —— 全状态 Agent 列表（含 draft/paused/archived），只返回当前
  登录人自己的 Agent。管理页用它，选 Agent 用上面的 `/active`。

## 已下线接口

`/jmate/agentapi/bot/human-interventions/*`（`enter` / `exit` / `status` / `send-message` /
`poll-messages`）**已全部移除**，调用会返回 404。

转人工改为：客户在 Ticket 群内发送「转人工」等关键词 → 服务端把 Inbox 坐席拉入群、广播一条
`jgm:ticketassign` 通知（`assign_type: 3`，文案「人工接入」）、并把 Agent Bot 移出群。此后该
工单与 Agent 再无关系，坐席直接在 IM 群里回复客户即可，不再需要服务端代发消息或轮询增量。

> 注意：转人工目前**不可逆**，没有「转回机器人」的接口。
