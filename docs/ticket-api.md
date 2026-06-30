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
| `items[].assignee_id` | string | 当前分配的客服用户 ID，未分配时为空 |
| `items[].assignee` | object/null | 当前分配的客服用户信息。未分配或用户不存在时为 `null` |
| `items[].assignee.id` | string | 客服用户 ID |
| `items[].assignee.nickname` | string | 客服昵称 |
| `items[].assignee.avatar` | string | 客服头像 |
| `items[].status` | int | 工单状态：`0` 待处理，`1` 处理中，`2` 关闭 |
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
