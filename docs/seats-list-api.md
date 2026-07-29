# 坐席目录 API

本文档描述控制台 / 客服前端「联系人」「坐席列表」面板使用的接口。接口统一返回：

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

接口挂载在 `/jmate/*` 下，**仅校验已登录用户**，不强制 admin 角色。

- 任何角色（admin / 客服 / admin 高权限用户）登录后均可访问。
- 客户（widget 访客）由于不会登录到 `/jmate/*` 业务接口，仍会被外层 `apis.Validate` 拦截。

这一开放策略与 `/jmate/console/*` 的 admin-only 限制不同：console 域管"用户管理 / 数据统计 / 系统配置"，本接口管"联系谁"。

### 鉴权 Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 调用 `/jmate/user/login` 接口返回的 `data.authorization` 字符串（不带 `Bearer ` 前缀，直接放到 Header） |

---

## 坐席目录

按用户分页返回当前应用下所有用户（含 admin 与客服），并附带其所属 Inbox 与当前未闭单等上下文信息。该接口供联系人 / 转单选择坐席 / 同事协作等场景使用。

### 请求

`GET /jmate/seats/list`

### Headers

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `appkey` | 是 | 当前应用的 appkey |
| `Authorization` | 是 | 当前应用任一已登录用户的 token |

### Query 参数

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `keyword` | string | 否 | 空 | 大小写不敏感子串匹配，对 `users.login_account` / `users.nickname` / `users.email` / `users.user_id` 做 `ILIKE '%keyword%'`，任一命中即视为匹配 |
| `role` | string | 否 | 空 | 角色过滤，取值 `customer_service`（客服=1）或 `admin`（管理员=2）。空表示不过滤。其它值返回 `17005` |
| `inbox_id` | string | 否 | 空 | 只列出绑定了该 Inbox 的用户。空表示不过滤。Inbox 不存在也不报错，仅返回空列表 |
| `status` | string | 否 | 空 | 账号状态过滤，取值 `0`（正常）或 `1`（禁用）。空表示不过滤。其它值返回 `17005` |
| `page` | int | 否 | `1` | 页码，从 1 开始。`<1` 返回 `17005` |
| `pageSize` | int | 否 | `20` | 每页条数。`<1` 返回 `17005`；超过 `100` 会被服务端夹紧到 `100` |

### 请求示例

```bash
curl -G 'http://localhost:8050/jmate/seats/list' \
  -H 'appkey: nsw3sue72begyv7y' \
  -H 'Authorization: <token from /jmate/user/login>' \
  --data-urlencode 'keyword=helena' \
  --data-urlencode 'role=customer_service' \
  --data-urlencode 'inbox_id=5TdqzXPf4kQbvjc2IGpteG' \
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
        "role": "customer_service",
        "status": 0,
        "im_token_present": true,
        "inbox_ids": [
          "2TLYuZJ94Cu9dciAWVp1Li",
          "5TdqzXPf4kQbvjc2IGpteG"
        ],
        "inbox_count": 2,
        "open_session_count": 1,
        "last_active_time": 1785151235311,
        "created_time": 1785143719064
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
| `items` | array | 坐席 / 用户列表，按 `status ASC, inbox_count DESC, login_account ASC` 排序 |
| `items[].user_id` | string | 用户 ID（`users.user_id`） |
| `items[].login_account` | string | 登录账号 |
| `items[].nickname` | string | 显示昵称 |
| `items[].avatar` | string | 头像 URL，可能为空 |
| `items[].email` | string | 邮箱，可能为空 |
| `items[].role` | string | 角色字符串：`customer_service`（1）或 `admin`（2）。库内的整型值会被翻译为字符串 |
| `items[].status` | int | 账号状态：`0` 正常 / `1` 禁用。前端可据此显示"该账号已停用"徽章，避免转单给已停用的坐席 |
| `items[].im_token_present` | bool | 该用户是否已在 JuggleIM 侧注册（`users.im_token != ''`）。为 `false` 时表示该用户尚未成功登录过 IM，前端可提示"该坐席不在 IM 中，建议先沟通再转单" |
| `items[].inbox_ids` | array\<string\> | 该用户绑定的 Inbox ID 列表（升序） |
| `items[].inbox_count` | int64 | inbox 数量 |
| `items[].open_session_count` | int64 | 当前挂在该用户名下、未关闭的工单数（`tickets.assignee_id = user_id AND status IN {0,1,3}`）。用于做转单建议的负载指示 |
| `items[].last_active_time` | int64 | 最近活跃时间毫秒时间戳。优先取 `tickets.updated_time` 的最大值（在该用户作为 assignee 的工单里）；若从未被分配过工单，则回退到 `created_time` |
| `items[].created_time` | int64 | `users.created_time` 毫秒时间戳 |
| `total` | int64 | 满足所有过滤条件的人数（不受分页影响） |
| `limit` | int64 | 服务端实际使用的 `pageSize`，夹紧后值 |
| `offset` | int64 | 服务端计算出的偏移量 `= (page-1) × limit` |

### 计算规则

- 用户范围：当前 `app_key` 下的所有 `users`。
- `inbox_ids` / `inbox_count` 通过 `inboxmembers` 子查询聚合，使用 PG `array_agg(inbox_id ORDER BY inbox_id)` 得到升序数组。
- `open_session_count` / `last_active_time` 通过 `tickets` 子查询聚合，仅统计 `assignee_id = user_id` 且 `status ∈ {0,1,3}` 的工单。
- `open_session_count` 与 `last_active_time` 都是"实时"指标，不带时间窗。
- `keyword` 通过 PostgreSQL `ILIKE '%keyword%'` 做子串匹配，对 `login_account` / `nickname` / `email` / `user_id` 任一命中即视为匹配（OR 连接）。
- `role` / `status` / `inbox_id` 三个过滤都使用 `(<sentinel> = <sentinel> OR <条件>)` 模式，sentinel 为 `-1` / `""` 时整段短路为 TRUE 关闭该条件。
- 排序固定：`status ASC → inbox_count DESC → login_account ASC`。当前不支持自定义排序。

### 错误响应

| code | 场景 |
| --- | --- |
| `17001` | 缺少 `appkey` Header |
| `17003` | 未登录（缺 `Authorization` 或 token 无效） |
| `17005` | `role` 取值非法、`status` 非 0/1、`page < 1` 或 `pageSize < 1` |
| `17006` | 数据库查询失败 |

---

## 附录：与现有接口的关系

| 接口 | 路径前缀 | 适用场景 | 与本接口的关系 |
| --- | --- | --- | --- |
| `GET /jmate/console/users` | 控制台 | admin 用户管理 CRUD | 字段窄（id / username / roles），无坐席上下文 |
| `GET /jmate/console/stats/agents` | 控制台（数据统计面板） | 时间窗聚合 | 字段绑定时间窗；不适合当联系人面板 |
| `GET /jmate/seats/list`（本接口） | 业务域 | 联系人 / 转单协作 | 完整字段 + 任何登录用户可用 |

新增本接口**完全不影响**前两者，三者可长期并存：

- 控制台维护账号信息用 `/jmate/console/users`；
- 数据统计面板用 `/jmate/console/stats/*`；
- 业务前端（IM 客服工作台、Widget 嵌入端等）拉取联系人列表用本接口。
