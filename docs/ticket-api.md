# Ticket API

本文档描述工单相关接口。接口统一返回：

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
| `17012` | 用户不存在 |
| `17014` | 渠道不存在 |

## 工单状态

| 状态值 | 名称 | 说明 |
| --- | --- | --- |
| `0` | 待处理 | 新建工单，尚未进入处理 |
| `1` | 处理中 | 工单正在处理 |
| `2` | 关闭 | 工单已关闭 |

## 工单全局会话标签

工单以 `ticket_id` 作为 IM 群会话 ID。服务端通过 IM SDK 的 `SetGlobalConverTags` 覆盖设置该群会话的全局标签，每次固定写入以下三类标签：

| 分类 | Ticket 字段 | 全局标签 |
| --- | --- | --- |
| 分配状态 | `assignee_id` 为空 | `unassigned` |
| 分配状态 | `assignee_id` 非空 | `assigned` |
| 流转状态 | `status=0`（待处理） | `ticket_status_pending` |
| 流转状态 | `status=1`（处理中） | `ticket_status_processing` |
| 流转状态 | `status=2`（关闭） | `ticket_status_closed` |
| 流转状态 | `status=3`（重新打开） | `ticket_status_reopen` |
| 渠道 | `channel_type=widget` | `channel_widget` |
| 渠道 | `channel_type=telegram` | `channel_telegram` |
| 渠道 | `channel_type=juggleim` | `channel_juggleim` |

同步方法接收 `appkey` 和 `ticket_id`，每次从数据库重新查询工单最新字段后生成完整的三个标签。新工单群组创建并持久化成功后会执行同步；认领、转移及后续关闭、重新打开等字段变化后也必须重新执行同步。由于 `SetGlobalConverTags` 使用完整标签列表覆盖更新，旧的分配状态、流转状态和渠道标签会被替换。

## 发起 Web 客服会话

创建或获取访客对应的客服工单，并返回 IM 登录信息。

### 请求

`POST /jmate/customers/start`

该接口不要求 `Authorization`，但必须传 `appkey`。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Content-Type` | 是 | `application/json` |

### Body

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `identifier` | string | 是 | 访客唯一标识。同一个 app 下用它定位 customer |
| `inbox_id` | string | 是 | 管理后台创建的 widget 渠道 inbox ID |
| `nickname` | string | 否 | 访客昵称。为空时服务端自动生成 |

`inbox_id` 必须在当前 app 的 `inboxes` 表中存在，且 `channel_type` 为 `widget`。服务端不会自动创建 inbox；若 inbox 不存在或渠道类型不是 widget，返回 `17014`（渠道不存在）。

### 请求示例

```bash
curl -X POST 'http://localhost:8080/jmate/customers/start' \
  -H 'appkey: app_xxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "identifier": "web-user-001",
    "inbox_id": "widget_inbox_001",
    "nickname": "Alice"
  }'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "conversation_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
    "conversation_type": 2,
    "user_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
    "nickname": "Alice",
    "im_token": "im-token",
    "welcome_message": "您好，有什么可以帮您？"
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `conversation_id` | string | 工单 ID，同时作为 IM 群组 ID |
| `conversation_type` | int | 会话类型。工单会话固定为 `2` |
| `user_id` | string | 访客在 IM 中使用的用户 ID，即 `customerinboxrels.source_id` |
| `nickname` | string | 访客昵称 |
| `im_token` | string | 访客 IM 登录 token |
| `welcome_message` | string | Widget 收件箱欢迎语，来自 `inboxes.channel_conf` 中的 `welcome_message`；未配置时为空字符串 |

### 处理规则

- 先按 `identifier` 在 `customers` 表中查找访客。
- 不存在时创建 customer，`customer_id` 由服务端生成。
- 校验 `inbox_id` 对应当前 app 下 `channel_type=widget` 的 inbox 记录。
- 按 `customer_id + inbox_id` 查找或创建 `customerinboxrels`。
- `source_id` 使用 `customer_` 前缀加服务端生成 ID。
- 查找该 `source_id` 对应工单，不存在时创建新工单。
- 新工单的 `ticket_id` 由服务端生成，状态为 `0`。
- 新工单会同步创建 IM 群组，`group_id` 使用 `ticket_id`，群成员包含访客 `source_id` 与该工单对应 inbox 在 `inboxmembers` 中配置的全部代表用户。

## 查询客户信息

根据 customer ID 查询当前 app 下的客户用户信息。

### 请求

`GET /jmate/customers/info?customer_id=:customer_id`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `customer_id` | string | 是 | 要查询的 customer ID；同时兼容参数名 `customerId` |

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/customers/info?customer_id=7nKs4PmQ1xZaT8VcY2eR0b' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": "7nKs4PmQ1xZaT8VcY2eR0b",
    "nickname": "Alice",
    "avatar": "https://example.com/customer.png"
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | customer ID |
| `nickname` | string | 客户昵称 |
| `avatar` | string | 客户头像 |

customer 不存在时返回 `17012`，缺少 customer ID 时返回 `17005`。

## 查询客户工单列表

根据 customer ID 查询当前 app 下该客户的工单，按工单内部 ID 倒序进行游标分页。

### 请求

`GET /jmate/customers/tickets?customer_id=:customer_id&start_id=:start_id&limit=:limit`

该接口需要登录。

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `customer_id` | string | 是 | 无 | 要查询的 customer ID；同时兼容参数名 `customerId` |
| `start_id` | int64 | 否 | `0` | 分页游标。第一页传 `0` 或不传，下一页使用上次响应的 `next_start_id` |
| `limit` | int | 否 | `20` | 每页工单数，范围为 `1`–`100` |

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/customers/tickets?customer_id=7nKs4PmQ1xZaT8VcY2eR0b&limit=20' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
        "source_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
        "customer_id": "7nKs4PmQ1xZaT8VcY2eR0b",
        "customer": {
          "id": "7nKs4PmQ1xZaT8VcY2eR0b",
          "nickname": "Alice",
          "avatar": "https://example.com/customer.png"
        },
        "inbox_id": "widget_inbox_001",
        "channel_type": "widget",
        "assignee_id": "u_123",
        "assignee": {
          "id": "u_123",
          "nickname": "客服 A",
          "avatar": "https://example.com/agent.png"
        },
        "status": 1,
        "created_time": 1782360000000,
        "updated_time": 1782360300000
      }
    ],
    "next_start_id": 123
  }
}
```

`items` 中的工单字段与“查询工单列表”接口一致。`next_start_id` 非 `0` 时，将其作为下一次请求的 `start_id`；值为 `0` 表示没有下一页。

customer 不存在时返回 `17012`。缺少 customer ID、`start_id` 小于 `0`，或 `limit` 不在 `1`–`100` 范围内时返回 `17005`。

## 查询 Inbox 成员列表

根据 inbox ID 查询当前 app 下配置的全部代表用户。

### 请求

`GET /jmate/inboxes/members/list`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `inbox_id` | string | 是 | 要查询成员的 inbox ID |

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/inboxes/members/list?inbox_id=inbox_001' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "list": [
      {
        "user_id": "u_123",
        "username": "agent@example.com",
        "avatar": "https://example.com/agent.png",
        "email": "agent@example.com"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `list` | array | Inbox 成员列表 |
| `list[].user_id` | string | 用户 ID |
| `list[].username` | string | 用户登录账号 |
| `list[].avatar` | string | 用户头像 |
| `list[].email` | string | 用户邮箱 |

不存在的用户成员记录会被忽略。Inbox 不存在、不属于当前 app 或渠道类型不受支持时返回 `17005`。

## Telegram Webhook

Telegram 渠道由管理后台创建 inbox 时自动配置 webhook。服务端会先通过 Telegram Bot API 校验 `bot_token`，再把回调地址设置为：

```text
{jmateBaseUrl}/jmate/webhooks/telegram/{inbox_id}
```

`jmateBaseUrl` 必须配置在 `conf/config.yml`，示例：

```yaml
jmateBaseUrl: https://example.com
```

### 请求

`POST /jmate/webhooks/telegram/:inbox_id`

该接口由 Telegram Bot API 回调，不要求 `Authorization` 或 `appkey`。服务端会根据 `:inbox_id` 查询 Telegram inbox，并从 inbox 记录中获取 `app_key`。

### 处理规则

- 只处理 Telegram private chat 中的 `message.text` 或 `message.caption`。
- 非 private chat、非 message update、空文本或不支持的消息体会直接确认成功，不创建 customer、ticket 或 IM 消息。
- Telegram 访客 identity 使用 `message.from.id` 的数字 ID 字符串。
- Telegram 访客在 IM 中的 `source_id` 与 Web 渠道一致，使用 `customer_` 前缀加服务端生成 ID。
- 访客昵称优先使用 `first_name + last_name`，为空时使用 `username`，再为空时使用 Telegram 数字 ID。
- 服务端复用 Web 访客相同的工单启动流程：创建或复用 customer、customer-inbox relation、IM 用户和 ticket。
- 新工单会创建 IM 群组，`group_id` 使用 `ticket_id`，群成员包含 Telegram 访客 `source_id` 与该 inbox 的成员。
- Telegram 消息会以访客 `source_id` 为 sender，发送到 `ticket_id` 对应的群会话，消息类型为 `jg:text`。

## IM Message Webhook Outbound Forwarding

`POST /jmate/webhooks/message` 接收到 IM 消息回调后，会对 ticket 群消息做外部渠道回传。

### 处理规则

- 只考虑 `conver_type=2` 的群消息。
- 只考虑 `receiver` 以 `ticket_` 开头的消息，并把 `receiver` 作为 `ticket_id` 查询工单。
- 当消息 `sender` 等于该工单的 `source_id` 时跳过处理，避免访客消息回环发送到外部渠道。
- 根据工单中的 `inbox_id` 查询 inbox，再按 `channel_type` 分发；当前支持 Telegram，其他渠道会确认成功但不转发。
- Telegram 回传使用 inbox `channel_conf` 中的 `bot_token`，目标 chat/user ID 来自工单 customer 的 `identifier`，不是 `ticket.source_id`。
- 当前只回传 `text` 或 `jg:text` 的文本内容；空内容或不支持的消息类型会跳过。

## JuggleIM Webhook

JuggleIM 渠道由管理后台创建 inbox，配置项包含 `bot_name` 和 `bot_token`。当前 JuggleIM bot SDK 初始化是占位实现，会校验必填配置并准备回调地址，真实 SDK 对接后使用同一扩展点完成注册。

回调地址：

```text
{jmateBaseUrl}/jmate/webhooks/juggleim/{inbox_id}
```

### 请求

`POST /jmate/webhooks/juggleim/:inbox_id`

该接口由 JuggleIM 回调，不要求 `Authorization` 或 `appkey`。服务端会根据 `:inbox_id` 查询 JuggleIM inbox，并从 inbox 记录中获取 `app_key`。

### Body

```go
type CustomChatMsgReq struct {
    AppKey      string       `json:"app_key"`
    Sender      string       `json:"sender"`
    Receiver    string       `json:"receiver"`
    ConverType  int          `json:"conver_type"`
    MsgType     string       `json:"msg_type"`
    MsgContent  string       `json:"msg_content"`
    MsgId       string       `json:"msg_id"`
    MsgTime     int64        `json:"msg_time"`
    MentionInfo *MentionInfo `json:"mention_info,omitempty"`
}
```

### 处理规则

- `sender` 作为 customer identity，用于复用同一个 JuggleIM 访客。
- JuggleIM 访客在 IM 中的 `source_id` 与 Web 渠道一致，使用 `customer_` 前缀加服务端生成 ID。
- 服务端复用 Web 访客相同的工单启动流程：创建或复用 customer、customer-inbox relation、IM 用户和 ticket。
- 新工单会创建 IM 群组，`group_id` 使用 `ticket_id`，群成员包含 JuggleIM 访客 `source_id` 与该 inbox 的成员。
- 回调消息会以访客 `source_id` 为 sender，发送到 `ticket_id` 对应的群会话。
- 转发时保留回调中的 `msg_type`、`msg_content` 和 `msg_id`。
- 无效 JSON、缺少 `sender`、缺少 `msg_content`、未知 inbox 或非 JuggleIM inbox 会直接确认成功或返回错误 envelope，不创建 customer、ticket 或 IM 消息。

## 查询工单列表

按当前登录用户权限查询工单列表，支持按状态过滤和 `limit`/`offset` 分页。

### 请求

`GET /jmate/tickets/list`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `status` | int | 否 | 无 | 工单状态。只能是 `0`、`1`、`2` |
| `is_human_taken_over` | bool | 否 | 无 | 是否曾转人工。`true` / `false` / `1` / `0` 均可，不传则不过滤 |
| `limit` | int | 否 | `20` | 每页数量，必须大于 `0` |
| `offset` | int | 否 | `0` | 跳过数量，必须大于等于 `0` |

返回结果按 `tickets.id desc` 排序。

### 权限规则

| 用户角色 | 可查询范围 |
| --- | --- |
| 普通客服用户 | 待处理工单，或 `assignee_id` 等于当前用户 ID 的工单 |
| 管理员 | 当前 app 下全部工单 |

当普通客服用户传入 `status` 时，状态过滤会和权限范围同时生效。例如查询 `status=2` 时，只返回分配给自己的已关闭工单。

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/tickets/list?status=1&limit=10&offset=20' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
        "source_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
        "customer_id": "7nKs4PmQ1xZaT8VcY2eR0b",
        "customer": {
          "id": "7nKs4PmQ1xZaT8VcY2eR0b",
          "nickname": "Alice",
          "avatar": "https://example.com/customer.png"
        },
        "inbox_id": "widget_inbox_001",
        "channel_type": "widget",
        "assignee_id": "u_123",
        "assignee": {
          "id": "u_123",
          "nickname": "客服 A",
          "avatar": "https://example.com/agent.png"
        },
        "status": 1,
        "is_human_taken_over": true,
        "human_taken_over_at": 1782360180000,
        "human_taken_over_by": "customer_8mQz6RkV2pXnT4bYcS1aE9",
        "created_time": 1782360000000,
        "updated_time": 1782360300000
      }
    ]
  }
}
```

没有匹配工单时：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": []
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | 工单列表 |
| `items[].ticket_id` | string | 工单 ID |
| `items[].source_id` | string | 访客渠道身份 ID |
| `items[].customer_id` | string | 访客 ID |
| `items[].customer` | object/null | 访客信息 |
| `items[].customer.id` | string | 访客 ID |
| `items[].customer.nickname` | string | 访客昵称 |
| `items[].customer.avatar` | string | 访客头像 |
| `items[].inbox_id` | string | inbox ID |
| `items[].channel_type` | string | 工单渠道类型，如 `widget`、`telegram` 或 `juggleim` |
| `items[].assignee_id` | string | 当前分配的客服用户 ID，未分配时为空 |
| `items[].assignee` | object/null | 当前分配的客服用户信息。未分配或用户不存在时为 `null` |
| `items[].assignee.id` | string | 客服用户 ID |
| `items[].assignee.nickname` | string | 客服昵称 |
| `items[].assignee.avatar` | string | 客服头像 |
| `items[].status` | int | 工单状态：`0` 待处理，`1` 处理中，`2` 关闭 |
| `items[].is_human_taken_over` | bool | 是否曾转人工。`false` 表示从未转人工，`true` 表示曾被客户触发转人工 |
| `items[].human_taken_over_at` | int64 | 首次转人工时的毫秒时间戳；未转人工时为 `0` |
| `items[].human_taken_over_by` | string | 首次转人工的触发者标识（当前为客户 `customer_id`，未来由坐席触发可填充 `user_id`）；未转人工时为空串 |
| `items[].created_time` | int64 | 创建时间，毫秒时间戳 |
| `items[].updated_time` | int64 | 更新时间，毫秒时间戳 |

### 参数错误示例

`status` 只能传 `0`、`1`、`2`：

```bash
curl -X GET 'http://localhost:8080/jmate/tickets/list?status=9' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

响应：

```json
{
  "code": 17005,
  "msg": ""
}
```

## 查询工单对应的 Inbox 成员

根据工单 ID 查询该工单所属 Inbox 的成员列表，按 Inbox 成员记录 ID 倒序进行游标分页。

### 请求

`GET /jmate/tickets/inboxmembers/list`

该接口需要登录，仅管理员和客服用户可以调用。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `ticket_id` | string | 是 | 无 | 工单 ID，也兼容 `ticketId` |
| `start_id` | int64 | 否 | `0` | 分页游标，首页传 `0` 或不传，后续页传上一页的 `next_start_id` |
| `limit` | int | 否 | `50` | 每页数量，取值范围为 `1`–`100` |

首次查询不传 `start_id`；当响应中的 `next_start_id` 非 `0` 时，将其作为下一次请求的 `start_id`。查询结果按 Inbox 成员记录 ID 倒序排列。

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/tickets/inboxmembers/list?ticket_id=3xvJK7Xwq2sTnQp6aLm9Z0&limit=50' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "user_id": "u_123",
        "username": "agent@example.com",
        "avatar": "https://example.com/agent.png",
        "email": "agent@example.com"
      }
    ],
    "next_start_id": 128
  }
}
```

没有成员或已到最后一页时：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [],
    "next_start_id": 0
  }
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | Inbox 成员列表 |
| `items[].user_id` | string | 用户 ID |
| `items[].username` | string | 用户登录账号 |
| `items[].avatar` | string | 用户头像 |
| `items[].email` | string | 用户邮箱 |
| `next_start_id` | int64 | 下一页游标；值为 `0` 表示没有下一页 |

工单不存在、不属于当前 app、缺少 `ticket_id`，或分页参数无效时返回 `17005`；当前登录用户不存在时返回 `17012`；未登录或用户角色不受支持时返回 `17003`。成员关联的用户已不存在时，该成员记录会被忽略。

## 认领工单

将待处理工单分配给当前登录用户，并将该用户加入工单对应的 IM 群组。

### 请求

`POST /jmate/tickets/:ticket_id/claim`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Path 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `ticket_id` | string | 是 | 要认领的工单 ID，同时作为 IM 群组 ID |

### 权限规则

| 用户角色 | 是否可认领 |
| --- | --- |
| 普通客服用户 | 是 |
| 管理员 | 是 |

### 处理规则

- 仅状态为 `0`（待处理）的工单可以被认领。
- 认领成功后，将 `assignee_id` 更新为当前用户 ID，状态更新为 `1`（处理中）。
- 通过 IM SDK 将当前用户加入 `ticket_id` 对应的群组。
- 加群成功后，向该群组发送一条工单分配通知消息（见下方「认领通知消息」）。
- 若 IM 加群失败，服务端会回滚认领状态，工单恢复为待处理。
- 工单不存在、已关闭、处理中或已被他人认领时，返回参数错误。

### 认领通知消息

认领成功且加群完成后，服务端会通过 IM SDK 向工单群组发送一条通知消息，供客户端展示「工单已被认领」等状态变化。

| 属性 | 值 | 说明 |
| --- | --- | --- |
| 消息类型 | `jgm:ticketassign` | 工单分配通知 |
| 发送方 | 当前认领用户 ID | 即 `Authorization` 对应用户 |
| 接收群 | `ticket_id` | 工单 ID 即 IM 群组 ID |
| 是否入库 | 是 | `is_storage = true` |
| 是否计入未读 | 否 | `is_count = false` |

消息体 `msg_content` 为 JSON 字符串，结构如下：

```json
{
  "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
  "operator": {
    "id": "u_123",
    "nickname": "客服 A",
    "avatar": "https://example.com/agent.png"
  },
  "assignee": {
    "id": "u_123",
    "nickname": "客服 A",
    "avatar": "https://example.com/agent.png"
  },
  "assign_type": 0
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ticket_id` | string | 工单 ID |
| `operator` | object | 操作人信息。认领场景下为当前认领用户 |
| `operator.id` | string | 操作人用户 ID |
| `operator.nickname` | string | 操作人昵称 |
| `operator.avatar` | string | 操作人头像 |
| `assignee` | object | 被分配客服信息。认领场景下与 `operator` 相同 |
| `assignee.id` | string | 客服用户 ID |
| `assignee.nickname` | string | 客服昵称 |
| `assignee.avatar` | string | 客服头像 |
| `assign_type` | int | 分配类型。认领固定为 `0` |

`assign_type` 枚举：

| 值 | 说明 |
| --- | --- |
| `0` | 认领 |
| `1` | 指定用户 |
| `2` | 指定团队 |
| `3` | Agent 转人工（人工接入）；`operator`、`assignee` 为 `null` |
| `4` | 重启工单；`operator`、`assignee` 为 `null` |

通知消息发送失败不影响认领接口的 HTTP 响应；客户端应以认领接口返回的工单数据为准。

### 请求示例

```bash
curl -X POST 'http://localhost:8080/jmate/tickets/3xvJK7Xwq2sTnQp6aLm9Z0/claim' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "ticket": {
      "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
      "source_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
      "customer_id": "7nKs4PmQ1xZaT8VcY2eR0b",
      "customer": {
        "id": "7nKs4PmQ1xZaT8VcY2eR0b",
        "nickname": "Alice",
        "avatar": "https://example.com/customer.png"
      },
      "inbox_id": "widget_inbox_001",
      "channel_type": "widget",
      "assignee_id": "u_123",
      "assignee": {
        "id": "u_123",
        "nickname": "客服 A",
        "avatar": "https://example.com/agent.png"
      },
      "status": 1,
      "created_time": 1782360000000,
      "updated_time": 1782360600000
    }
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ticket` | object | 认领后的工单信息 |
| `ticket.ticket_id` | string | 工单 ID |
| `ticket.source_id` | string | 访客渠道身份 ID |
| `ticket.customer_id` | string | 访客 ID |
| `ticket.customer` | object/null | 访客信息 |
| `ticket.inbox_id` | string | inbox ID |
| `ticket.channel_type` | string | 工单渠道类型，如 `widget`、`telegram` 或 `juggleim` |
| `ticket.assignee_id` | string | 认领后的客服用户 ID，即当前用户 |
| `ticket.assignee` | object/null | 认领后的客服用户信息 |
| `ticket.status` | int | 工单状态。认领成功后为 `1`（处理中） |
| `ticket.created_time` | int64 | 创建时间，毫秒时间戳 |
| `ticket.updated_time` | int64 | 更新时间，毫秒时间戳 |

### 错误示例

工单不存在或不是待处理状态：

```bash
curl -X POST 'http://localhost:8080/jmate/tickets/not-exist/claim' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

响应：

```json
{
  "code": 17005,
  "msg": ""
}
```

IM 加群失败时，认领状态会回滚，并返回对应错误码（例如 `17006` 服务内部错误）。

## 转移工单

将正在处理的工单转移给指定用户，并向工单对应的 IM 群组发送工单分配通知。

### 请求

`POST /jmate/tickets/:ticket_id/transfer`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Path 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `ticket_id` | string | 是 | 要转移的工单 ID，同时作为 IM 群组 ID |

### Body 参数

Content-Type：`application/json`

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `assignee_id` | string | 是 | 转移后的处理人用户 ID，不能是当前操作人或工单当前处理人 |

### 权限规则

| 用户角色 | 可转移范围 |
| --- | --- |
| 普通客服用户 | 仅限当前分配给自己的工单 |
| 管理员 | 当前 app 下任意符合转移条件的工单 |

目标用户必须存在于当前 app 下，且角色为普通客服用户或管理员。

### 处理规则

- 仅状态为 `1`（处理中）且已有处理人的工单可以转移。
- 转移目标必须是其他用户，不能与当前操作人或工单当前处理人相同。
- 转移成功后，仅更新工单的 `assignee_id` 和 `updated_time`，工单状态保持为 `1`（处理中）。
- 更新时会同时校验工单原处理人和状态；若工单已被其他请求修改，则转移失败，避免覆盖并发更新。
- 转移成功后，服务端删除原处理人对应工单群会话的 `my_ticket` 标签，并给新处理人的对应会话添加 `my_ticket` 标签。
- 若会话标签同步失败，服务端会将工单处理人回滚；新处理人标签写入失败时，还会尽力恢复原处理人的 `my_ticket` 标签。
- 转移成功后，服务端向 `ticket_id` 对应的 IM 群组发送一条工单分配通知。

### 转移通知消息

| 属性 | 值 | 说明 |
| --- | --- | --- |
| 消息类型 | `jgm:ticketassign` | 工单分配通知 |
| 发送方 | 当前操作人用户 ID | 即 `Authorization` 对应用户 |
| 接收群 | `ticket_id` | 工单 ID 即 IM 群组 ID |
| 是否入库 | 是 | `is_storage = true` |
| 是否计入未读 | 否 | `is_count = false` |

消息体 `msg_content` 为 JSON 字符串，结构如下：

```json
{
  "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
  "operator": {
    "id": "u_123",
    "nickname": "客服 A",
    "avatar": "https://example.com/agent-a.png"
  },
  "assignee": {
    "id": "u_456",
    "nickname": "客服 B",
    "avatar": "https://example.com/agent-b.png"
  },
  "assign_type": 1
}
```

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ticket_id` | string | 工单 ID |
| `operator` | object | 发起转移的操作人信息 |
| `operator.id` | string | 操作人用户 ID |
| `operator.nickname` | string | 操作人昵称 |
| `operator.avatar` | string | 操作人头像 |
| `assignee` | object | 转移后的处理人信息 |
| `assignee.id` | string | 新处理人用户 ID |
| `assignee.nickname` | string | 新处理人昵称 |
| `assignee.avatar` | string | 新处理人头像 |
| `assign_type` | int | 分配类型。指定用户固定为 `1`（`AssignUser`） |

通知消息发送失败不影响转移接口的 HTTP 响应；客户端应以转移接口返回的工单数据为准。

### 请求示例

```bash
curl -X POST 'http://localhost:8080/jmate/tickets/3xvJK7Xwq2sTnQp6aLm9Z0/transfer' \
  -H 'Content-Type: application/json' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token' \
  -d '{
    "assignee_id": "u_456"
  }'
```

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "ticket": {
      "ticket_id": "3xvJK7Xwq2sTnQp6aLm9Z0",
      "source_id": "customer_8mQz6RkV2pXnT4bYcS1aE9",
      "customer_id": "7nKs4PmQ1xZaT8VcY2eR0b",
      "customer": {
        "id": "7nKs4PmQ1xZaT8VcY2eR0b",
        "nickname": "Alice",
        "avatar": "https://example.com/customer.png"
      },
      "inbox_id": "widget_inbox_001",
      "channel_type": "widget",
      "assignee_id": "u_456",
      "assignee": {
        "id": "u_456",
        "nickname": "客服 B",
        "avatar": "https://example.com/agent-b.png"
      },
      "status": 1,
      "created_time": 1782360000000,
      "updated_time": 1782360900000
    }
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ticket` | object | 转移后的工单信息 |
| `ticket.ticket_id` | string | 工单 ID |
| `ticket.source_id` | string | 访客渠道身份 ID |
| `ticket.customer_id` | string | 访客 ID |
| `ticket.customer` | object/null | 访客信息 |
| `ticket.inbox_id` | string | inbox ID |
| `ticket.channel_type` | string | 工单渠道类型，如 `widget`、`telegram` 或 `juggleim` |
| `ticket.assignee_id` | string | 转移后的处理人用户 ID |
| `ticket.assignee` | object/null | 转移后的处理人信息 |
| `ticket.status` | int | 工单状态。转移成功后仍为 `1`（处理中） |
| `ticket.created_time` | int64 | 创建时间，毫秒时间戳 |
| `ticket.updated_time` | int64 | 转移完成时间，毫秒时间戳 |

### 错误响应

| code | 场景 |
| --- | --- |
| `17003` | 当前用户无权转移该工单 |
| `17002` | IM SDK 未初始化，工单转移已回滚 |
| `17005` | 请求参数错误、工单不存在、工单不是处理中、目标为当前处理人，或发生并发更新 |
| `17006` | 查询、更新或会话标签同步失败；已完成的工单转移会回滚 |
| `17012` | 当前操作人或目标用户不存在 |

例如，目标用户不存在时：

```json
{
  "code": 17012,
  "msg": ""
}
```

## 查询工单事件流水

按工单 ID 拉取其事件历史（当前仅落地 `human_takeover` 类型），按 `id desc` 倒序返回。

### 请求

`GET /jmate/tickets/:ticket_id/events`

该接口需要登录。

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 用户登录 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `limit` | int | 否 | `50` | 每页数量，1-100 |
| `offset` | int | 否 | `0` | 跳过数量，必须大于等于 `0` |

### 请求示例

```bash
curl -X GET 'http://localhost:8080/jmate/tickets/3xvJK7Xwq2sTnQp6aLm9Z0/events?limit=20' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: user-token'
```

### 成功响应

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

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items` | array | 事件列表 |
| `items[].id` | int64 | 事件 ID |
| `items[].ticket_id` | string | 工单 ID |
| `items[].event_type` | string | 事件类型，目前为 `human_takeover`，未来扩展 `claim` / `transfer` / `close` / `reopen` |
| `items[].operator_id` | string | 触发者标识。客户触发时为 `customer_id` |
| `items[].operator_type` | string | 触发者类型：`customer` / `user` / `system` |
| `items[].payload` | string | 事件扩展字段，原始 JSON 字符串 |
| `items[].created_time` | int64 | 事件时间，毫秒时间戳 |
| `next_start_id` | int64 | 翻页游标（取本页最后一条事件的 `id`），下次请求可用 `WHERE id < next_start_id` 思路继续翻页；当前接口暂不直接做 keyset，使用 `limit` + `offset` 即可 |

### 注意事项

- `is_human_taken_over` 一旦为 `true` 不可回滚；同一工单多次触发转人工只会新增事件，不会覆盖首次时间。
- 转人工副作用由后台事务原子完成：先持久化 `is_human_taken_over`，再写 `ticket_events`，最后才在 IM 群里拉坐席、移 Bot。

## 工单自动关闭 & 评价系统

### 概述

工单超时自动关闭（自 2026-07-29 上线）：系统后台每 30 秒扫描一次，对
`status=1`（processing）且 `last_user_msg_at < now - 5min` 的工单按抢占式 SQL 关闭
（status→2、closed_at=now），同时在 IM 群里发 `jgm:csat` 自定义消息邀请客户评价。

**评价写入全部走 IM 自定义消息**，本服务**不暴露**评分写接口。前端用户在卡片
上点击 5★，通过 `imSdk.sendGroupMsg(msg_type="jgm:csatreply", content=...)` 把
评分作为群消息发出，webhook 入站后服务端解析与落库。

### 工单自动关闭：服务端动作

```
[T0 坐席最后消息]
    ↓ webhook 入站 → ticket_messages 落库 + UPDATE last_user_msg_at = T0
[T0+5min 后台 ticker 命中]
    ├─ UPDATE tickets SET status=2, closed_at=now() WHERE WHERE 抢占条件
    ├─ INSERT ticket_events (event_type='close', op=system, payload={"reason":"idle_timeout"})
    └─ 立刻通过 SDK SendCustomMessage 发 jgm:csat 邀请卡

后续任意时刻：
  - 客户 / 坐席在群里发任意消息 → ticket_messages 落库（不动 status）
  - 客户发 jgm:csatreply {rating, comment} → ticket_ratings 落库（不动 status）
```

### 数据模型增量

#### `tickets` 表新增字段

| 字段 | 类型 | 含义 |
|---|---|---|
| `last_user_msg_at` | `DATETIME(3)` / `TIMESTAMPTZ` | 坐席/系统消息最后时间；客户消息不更新 |
| `closed_at` | `DATETIME(3)` / `TIMESTAMPTZ` | 工单关闭时间；status=2 时被赋值 |

#### `ticket_messages` 表（新增）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT PK | |
| `app_key` | VARCHAR | |
| `ticket_id` | VARCHAR | |
| `sender_id` | VARCHAR | 发送者 IM 身份 |
| `sender_role` | VARCHAR | `customer` / `user` / `bot` / `system` |
| `msg_id` | VARCHAR | IM server 的 msg_id（用于幂等去重） |
| `msg_type` | VARCHAR | 原 msg_type，包括 `jg:text` / `jg:csatreply` / 自定义业务协议 |
| `created_time` | TIMESTAMP | |

约束：`UNIQUE (app_key, msg_id)`；索引 `(app_key, ticket_id, created_time)`。

> TIPS: 不存 message content（正文已存 JuggleIM 自身），只存 who/when/type 用于审计与计时。

#### `ticket_ratings` 表（新增）

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | BIGINT PK | |
| `app_key` | VARCHAR | |
| `ticket_id` | VARCHAR | |
| `customer_id` | VARCHAR | 评价者（客户） |
| `assignee_id` | VARCHAR | 被评的坐席；自动从 tickets.assignee_id 取 |
| `rating` | TINYINT / SMALLINT | 1-5 |
| `comment` | TEXT | ≤ 500 字符（服务端超长截断） |
| `source` | VARCHAR | `card_button`（无 comment） / `card_with_comment`（有 comment） |
| `created_time` | TIMESTAMP | |

约束：`UNIQUE (app_key, ticket_id, customer_id)` —— **同一客户对同一工单只能评一次**。
重复评分走静默忽略（webhook 返回 SUCCESS，service 不写第二条）。

### IM 自定义消息协议

#### `jgm:csat`：邀请评价卡片（服务端 → 客户）

发送时机：工单关闭那一刻。

```json
{
  "msg_type": "jgm:csat",
  "msg_content": {
    "kind": "csat",
    "version": 1,
    "app_key": "nsw3sue72begyv7y",
    "ticket_id": "ticket_xxx",
    "text": "您的会话已结束。请对本次服务做个评价：\n1 很差 / 2 不满意 / 3 一般 / 4 满意 / 5 很满意。\n评分 ≤3 时建议同时写点意见，方便我们改进。",
    "csat": {
      "low_rating_threshold": 3,
      "low_rating_hint": "评分 ≤3 时，请附带文字说明问题，方便我们改进。",
      "options": [
        {"value": 1, "label": "1 很差"},
        {"value": 2, "label": "2 不满意"},
        {"value": 3, "label": "3 一般"},
        {"value": 4, "label": "4 满意"},
        {"value": 5, "label": "5 很满意"}
      ],
      "reply_msg_type": "jgm:csatreply",
      "reply_schema": {
        "rating":  "int 1..5",
        "comment": "string optional (≤ 500 chars); recommended when rating ≤ 3"
      }
    }
  }
}
```

前端 UI 按 `msg_type === "jgm:csat"` 路由到评分卡；点按按钮后**构造 `jgm:csatreply`
消息发到同一 IM 群里**（不调任何 HTTP 接口）。

#### `jgm:csatreply`：客户评分回复（客户 → 服务端）

接收路径：webhook 入站，自动路由到 `services.RecordFromCustomMessage`。

```json
{
  "msg_type": "jgm:csatreply",
  "msg_content": {
    "kind": "csatreply",
    "version": 1,
    "app_key": "nsw3sue72begyv7y",
    "ticket_id": "ticket_xxx",
    "rating": 5,
    "comment": "",
    "client_ts": 1785380000000
  }
}
```

或带 comment：

```json
{
  "msg_type": "jgm:csatreply",
  "msg_content": {
    "kind": "csatreply",
    "version": 1,
    "app_key": "nsw3sue72begyv7y",
    "ticket_id": "ticket_xxx",
    "rating": 3,
    "comment": "客服回复慢",
    "client_ts": 1785380000000
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `kind` | string | 是 | 必须是 `csatreply` |
| `version` | int | 是 | 当前为 1 |
| `app_key` | string | 是 | 必须与 webhook receiver 所在 app 一致（**服务端强校验**） |
| `ticket_id` | string | 是 | 工单 ID（**强校验**） |
| `rating` | int | 是 | [1, 5]；非法值被静默丢 |
| `comment` | string | 否 | ≤ 500 字符；超长服务端截断 |
| `client_ts` | int64 | 否 | 客户端时间，便于审计 |

#### 服务端入站校验

| 规则 | 行为 |
|---|---|
| `kind != "csatreply"` | 静默丢 |
| `app_key` / `ticket_id` 不匹配 ctx 或 receiver | 静默丢 |
| `sender` ≠ `tickets.source_id`（不是客户发的） | 静默丢 |
| `rating` 越界 | 静默丢 |
| `UNIQUE` 冲突（同 customer 重复评分） | 静默丢（webhook 返回 SUCCESS） |
| TGIM server 重投（msg_id 相同） | `ticket_messages` upsert 幂等 |

### 查询工单评价（HTTP）

#### 请求

`GET /jmate/tickets/:ticket_id/rating?customer_id=...`

#### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 登录 token |

#### Query 参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `customer_id` | string | 是 | 客户 ID（按 UNIQUE (app_key, ticket_id, customer_id) 查询） |

#### 请求示例

```bash
curl -G 'http://localhost:8050/jmate/tickets/ticket_xxx/rating' \
  -H 'appkey: app_xxx' \
  -H 'Authorization: <token>' \
  --data-urlencode 'customer_id=customer_xxx'
```

#### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "rating": {
      "ticket_id": "ticket_xxx",
      "customer_id": "customer_xxx",
      "assignee_id": "u_seat_1",
      "rating": 5,
      "comment": "好评",
      "created_time": 1785000000000
    }
  }
}
```

未评时 `data.rating = null`，整体 `code=0`。

#### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `rating` | object/null | 评价对象；不存在时为 `null` |
| `rating.ticket_id` | string | 工单 ID |
| `rating.customer_id` | string | 客户 ID |
| `rating.assignee_id` | string | 被评的坐席 user_id |
| `rating.rating` | int | 1-5 |
| `rating.comment` | string | 评论（超长截断） |
| `rating.created_time` | int64 | 时间，毫秒 |

### 工单列表新增字段

`TicketInfo` 透出工单的关闭时间与最后活跃时间：

```json
{
  "ticket_id": "ticket_xxx",
  "status": 2,
  "is_human_taken_over": false,
  "human_taken_over_at": 0,
  "human_taken_over_by": "",
  "last_user_msg_at": 1785379200000,
  "closed_at": 1785379500000,
  "created_time": 1785378000000,
  "updated_time": 1785379500000,
  ...
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `last_user_msg_at` | int64 | 最后坐席消息毫秒时间戳；0 表示无活跃坐席消息 |
| `closed_at` | int64 | 工单关闭毫秒时间戳；0 表示未关闭 |
| `csat_notified_at` | int64 | 评价邀请送达时间戳；0 表示还没发评价邀请（IM 临时失联场景下，后台 ticker 会补发） |

### 注意事项

- **超时阈值默认 5 分钟**；可通过 `agent.auto_close.idle_minutes` 配置覆盖。
- **多实例安全**：所有 ticker 抢占都用 `UPDATE ... WHERE status=1 AND last_user_msg_at < cutoff`，
  `RowsAffected` 防重复关闭。
- **jgm:csat 发送失败不影响关闭**：`status=2` 已入库，IM 失败仅 slog，不重试。
- **评价通过 webhook 入库**：不在服务侧暴露评分写接口，避免被绕过群消息机制。

## 后续工作（不在本 change 范围）

- FRT（first response time）统计：需要 `ticket_messages` 完整 metadata + 跨窗口聚合。
- WhatsApp channel 适配：telegram/whatsapp 渠道客户 jgm:csatreply 的 UI 适配。
- 管理后台评价模块：评分列表 / 回复建议 / 坐席排行。
