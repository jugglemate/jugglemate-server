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
| `17004` | 服务内部错误 |
| `17005` | 未登录或无权限 |
| `17012` | 用户不存在 |
| `17017` | 参数错误 |

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
| `nickname` | string | 否 | 访客昵称。为空时服务端自动生成 |

### 请求示例

```bash
curl -X POST 'http://localhost:8080/jmate/customers/start' \
  -H 'appkey: app_xxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "identifier": "web-user-001",
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
    "im_token": "im-token"
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `conversation_id` | string | 工单 ID，同时作为 IM 群组 ID |
| `conversation_type` | int | 会话类型。工单会话固定为 `2` |
| `user_id` | string | 访客在 IM 中使用的用户 ID，即 `customerchannelrels.source_id` |
| `nickname` | string | 访客昵称 |
| `im_token` | string | 访客 IM 登录 token |

### 处理规则

- 先按 `identifier` 在 `customers` 表中查找访客。
- 不存在时创建 customer，`customer_id` 由服务端生成。
- 按 `customer_id + channel_id` 查找或创建 `customerchannelrels`，Web 渠道的 `channel_id` 为 `web`。
- `source_id` 使用 `customer_` 前缀加服务端生成 ID。
- 查找该 `source_id` 对应工单，不存在时创建新工单。
- 新工单的 `ticket_id` 由服务端生成，状态为 `0`。
- 新工单会同步创建 IM 群组，`group_id` 使用 `ticket_id`。

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
        "channel_id": "web",
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
| `items[].channel_id` | string | 渠道 ID，例如 `web` |
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
  "code": 17017,
  "msg": ""
}
```
