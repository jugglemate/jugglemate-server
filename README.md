# JuggleChat Server AI API

JuggleChat 分身 AI 服务的 HTTP API 文档。

## 概述

- **Base Path**: `/jim`
- **消息回调 Path**: `/botmsgs`
- **认证**: 走现有登录态和 `jimApis.Validate` 中间件
- **Content-Type**: `application/json`

## 通用约定

### 成功响应

```json
{
  "code": 0,
  "msg": "success",
  "data": {}
}
```

### 失败响应

```json
{
  "code": 17602,
  "msg": "bot not found"
}
```

### 分页游标

- `offset` / `cursor` 为字符串游标，基于数据库自增 `id` 编码

### 数据模型

#### AiBotInfo (分身信息)

| 字段 | 类型 | 说明 |
|------|------|------|
| bot_id | string | 分身 ID（服务端生成） |
| unique_name | string | 分身唯一标识名 |
| nickname | string | 分身昵称（用于展示） |
| display_name | string | 分身展示名 |
| avatar | string | 头像地址（兼容字段） |
| avatar_url | string | 头像 URL |
| greeting | string | 问候语 |
| prompts | string | 提示词配置 |
| owner_id | string | 所有者用户 ID |
| status | string | 状态：`untrained` / `training` / `trained` |
| active_version | string | 当前激活版本号 |
| training_mode | string | 训练模式：`mind` / `memory` / `skill` |
| materials_count | int | 素材数量 |
| created_time | int64 | 创建时间戳（毫秒） |
| updated_time | int64 | 更新时间戳（毫秒） |

#### AiMaterialInfo (素材信息)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 素材 ID |
| type | string | 素材类型：`text` / `url` / `file` |
| title | string | 素材标题 |
| source | string | 来源，默认等于 type |
| content | string | 文本内容 |
| url | string | 外链地址 |
| file_path | string | 文件路径 |
| size_bytes | int64 | 文件大小（字节） |

#### LoginResp (登录/注册响应)

| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | string | 用户 ID |
| nick_name | string | 用户昵称 |
| avatar | string | 头像 URL |
| authorization | string | JWT 认证 Token |
| im_token | string | JuggleIM 连接 Token |

---

## 接口列表

### 1. 用户注册

- **Method**: `POST`
- **Path**: `/jim/user/register`
- **认证**: 否（开放接口）

#### Request

```json
{
  "account": "用户名，6-20位字母数字",
  "password": "密码，最少6位"
}
```

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "user_id": "u_xxx",
    "nick_name": "user12345",
    "avatar": "",
    "authorization": "jwt_token",
    "im_token": "juggleim_token"
  }
}
```

**注意**: `im_token` 是连接 JuggleIM 服务器所需的 Token，客户端连接 IM 时需要使用。

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误（账号格式错误或密码太短） |
| 17608 | 用户已存在 |

---

### 2. 用户登录

- **Method**: `POST`
- **Path**: `/jim/user/login`
- **认证**: 否（开放接口）

#### Request

```json
{
  "account": "用户名",
  "password": "密码"
}
```

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "user_id": "u_xxx",
    "nick_name": "user12345",
    "avatar": "",
    "authorization": "jwt_token",
    "im_token": "juggleim_token"
  }
}
```

**注意**: `im_token` 是连接 JuggleIM 服务器所需的 Token，客户端连接 IM 时需要使用。每次登录会获取新的 IM Token。

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误 |
| 17609 | 用户不存在 |
| 17610 | 密码错误 |

---

### 3. 创建分身

- **Method**: `POST`
- **Path**: `/jim/aibots/add`
- **认证**: 是

#### Request

```json
{
  "unique_name": "可选，自定义分身唯一名，不传则默认使用 bot_id",
  "nickname": "可选，分身昵称",
  "display_name": "可选，分身展示名",
  "avatar": "可选，头像地址",
  "avatar_url": "可选，头像 URL",
  "greeting": "可选，问候语",
  "prompts": "可选，提示词配置"
}
```

**注意**: `nickname` 和 `display_name` 至少传一个

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "bot_id": "b1234567890",
    "unique_name": "my_twin",
    "nickname": "我的助手",
    "display_name": "我的助手",
    "avatar": "https://xx/avatar.png",
    "avatar_url": "https://xx/avatar.png",
    "greeting": "你好，我是助手",
    "prompts": "",
    "owner_id": "u10001",
    "status": "untrained",
    "active_version": "",
    "training_mode": "",
    "materials_count": 0,
    "created_time": 1716700000000,
    "updated_time": 1716700000000
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误 |
| 17601 | 创建分身失败（可能已存在） |

---

### 4. 更新分身

- **Method**: `POST`
- **Path**: `/jim/aibots/update`
- **认证**: 是

#### Request

```json
{
  "bot_id": "必填，bot id",
  "unique_name": "可选，分身唯一名",
  "nickname": "可选，分身昵称",
  "display_name": "可选，分身展示名",
  "avatar": "可选，头像地址",
  "avatar_url": "可选，头像 URL",
  "greeting": "可选，问候语",
  "prompts": "可选，提示词配置"
}
```

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "bot_id": "b1234567890",
    "unique_name": "my_twin",
    "nickname": "新名称",
    "display_name": "新名称",
    "avatar": "https://xx/new-avatar.png",
    "avatar_url": "https://xx/new-avatar.png",
    "greeting": "新的问候语",
    "owner_id": "u10001",
    "status": "untrained",
    "active_version": "",
    "training_mode": "",
    "materials_count": 0,
    "created_time": 1716700000000,
    "updated_time": 1716700200000
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误（缺少 bot_id） |
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17604 | 更新分身失败 |

---

### 5. 删除分身

- **Method**: `POST`
- **Path**: `/jim/aibots/remove`
- **认证**: 是

#### Request

```json
{
  "bot_id": "必填"
}
```

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误（缺少 bot_id） |
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17605 | 删除分身失败 |

---

### 6. 查询我的分身列表

- **Method**: `GET`
- **Path**: `/jim/aibots/mybots`
- **认证**: 是

#### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| count | int64 | 否 | 20 | 每页数量 |
| offset | string | 否 | "" | 分页游标 |

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "bot_id": "b1234567890",
        "unique_name": "my_twin",
        "nickname": "我的助手",
        "display_name": "我的助手",
        "avatar": "https://xx/avatar.png",
        "avatar_url": "https://xx/avatar.png",
        "greeting": "你好，我是助手",
        "owner_id": "u10001",
        "status": "untrained",
        "active_version": "",
        "training_mode": "",
        "materials_count": 0,
        "created_time": 1716700000000,
        "updated_time": 1716700000000
      }
    ],
    "offset": "下一页游标"
  }
}
```

---

### 7. 新增素材

- **Method**: `POST`
- **Path**: `/jim/aibots/:unique_name/materials/add`
- **认证**: 是

#### Request

```json
{
  "type": "text",
  "title": "素材标题",
  "source": "可选，来源，不传则默认等于 type",
  "content": "可选，文本内容",
  "url": "可选，外链",
  "file_path": "可选，文件路径",
  "size_bytes": 123
}
```

**注意**: `unique_name` 可通过路径参数或 query 参数 `unique_name` 传递

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": "mat_xxx",
    "type": "text",
    "title": "素材标题",
    "source": "text",
    "content": "素材正文",
    "url": "",
    "file_path": "",
    "size_bytes": 123
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误 |
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17606 | 添加素材失败 |

---

### 8. 查询素材列表

- **Method**: `GET`
- **Path**: `/jim/aibots/:unique_name/materials`
- **认证**: 是

#### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| count | int64 | 否 | 20 | 每页数量 |
| offset | string | 否 | "" | 分页游标 |

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "id": "mat_xxx",
        "type": "text",
        "title": "素材标题",
        "source": "text",
        "content": "素材正文",
        "url": "",
        "file_path": "",
        "size_bytes": 123
      }
    ],
    "offset": "下一页游标"
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |

---

### 9. 删除素材

- **Method**: `POST`
- **Path**: `/jim/aibots/:unique_name/materials/:material_id/remove`
- **认证**: 是

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误（缺少 unique_name 或 material_id） |
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17607 | 删除素材失败 |

---

### 10. 发起训练

- **Method**: `POST`
- **Path**: `/jim/aibots/:unique_name/training`
- **认证**: 是

#### Request

```json
{
  "mode": "mind"
}
```

**mode 可选值**: `mind` / `memory` / `skill`，默认 `mind`

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "bot_id": "b1234567890",
    "unique_name": "my_twin",
    "nickname": "我的助手",
    "display_name": "我的助手",
    "avatar": "https://xx/avatar.png",
    "avatar_url": "https://xx/avatar.png",
    "greeting": "你好，我是助手",
    "owner_id": "u10001",
    "status": "training",
    "active_version": "",
    "training_mode": "mind",
    "materials_count": 3,
    "created_time": 1716700000000,
    "updated_time": 1716700200000
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17001 | 参数错误（缺少 unique_name） |
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17600 | 默认错误（无效的 mode） |

---

### 11. 查询任务列表

- **Method**: `GET`
- **Path**: `/jim/aibots/:unique_name/jobs`
- **认证**: 是

#### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| type | string | 否 | "" | 任务类型过滤 |
| status | string | 否 | "" | 状态过滤：`queued` / `running` / `completed` / `failed` |
| limit | int64 | 否 | 20 | 每页数量 |
| cursor | string | 否 | "" | 分页游标 |

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "bot_id": "job_xxx",
        "unique_name": "my_twin",
        "nickname": "training",
        "display_name": "queued",
        "greeting": "{}",
        "owner_id": "u10001",
        "created_time": 1716700000000,
        "updated_time": 1716700000000
      }
    ],
    "offset": "下一页游标"
  }
}
```

**注意**: 返回结构复用 `AiBotInfo`，字段语义映射：
- `bot_id` → `job_id`
- `nickname` → `type`
- `display_name` → `status`
- `greeting` → `result_json`

---

### 12. 查询版本列表

- **Method**: `GET`
- **Path**: `/jim/aibots/:unique_name/versions`
- **认证**: 是

#### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| limit | int64 | 否 | 20 | 每页数量 |
| cursor | string | 否 | "" | 分页游标 |

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [
      {
        "bot_id": "v2",
        "unique_name": "my_twin",
        "nickname": "mind",
        "display_name": "mind",
        "active_version": "job_xxx",
        "training_mode": "mind",
        "materials_count": 3,
        "created_time": 1716700000000,
        "updated_time": 1716700000000
      }
    ],
    "offset": "下一页游标"
  }
}
```

**注意**: 返回结构复用 `AiBotInfo`，字段语义映射：
- `bot_id` → `version`
- `nickname` / `display_name` → `mode`
- `active_version` → `training_job_id`

---

### 13. 激活版本

- **Method**: `POST`
- **Path**: `/jim/aibots/:unique_name/versions/:version/activate`
- **认证**: 是

#### Response

返回当前分身信息，格式同"创建分身"的 `data`。

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "bot_id": "b1234567890",
    "unique_name": "my_twin",
    "status": "trained",
    "active_version": "v2",
    "training_mode": "mind",
    ...
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |
| 17604 | 更新分身失败 |

---

### 14. 查询当前版本

- **Method**: `GET`
- **Path**: `/jim/aibots/:unique_name/versions/current`
- **认证**: 是

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "bot_id": "v2",
    "unique_name": "my_twin",
    "nickname": "mind",
    "display_name": "mind",
    "active_version": "job_xxx",
    "training_mode": "mind",
    "materials_count": 3
  }
}
```

#### 错误码

| code | 说明 |
|------|------|
| 17602 | 分身不存在 |
| 17603 | 无权限（非所有者） |

---

### 15. 查询评估列表

- **Method**: `GET`
- **Path**: `/jim/aibots/:unique_name/evaluations`
- **认证**: 是

#### Query Parameters

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| limit | int64 | 否 | 20 | 每页数量 |
| cursor | string | 否 | "" | 分页游标 |

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "items": [],
    "offset": ""
  }
}
```

**注意**: 评估功能待实现，当前返回空列表

---

### 16. 消息回调（MsgCallback）

- **Method**: `POST`
- **Path**: `/botmsgs/msgcallback`
- **认证**: 否（IM Server 回调）

这是 IM Server 回调入口，不是客户端直调接口。

#### Request

```json
{
  "event_type": "message",
  "timestamp": 1713456000000,
  "payload": [
    {
      "platform": "iOS",
      "sender": "userid1",
      "receiver": "userid2",
      "conver_type": 1,
      "msg_type": "text",
      "msg_content": "Hello, world!",
      "mention_info": {
        "mention_type": "mention_all",
        "target_user_ids": ["userid1", "userid2"]
      },
      "msg_id": "123",
      "msg_time": 1718980832156
    }
  ]
}
```

#### 处理规则

- 仅处理 `event_type = message`
- 仅处理 `msg_type = text`
- `receiver` 映射到本地分身
- `sender` 作为客户消息发送者
- `msg_id` 用于幂等（重复消息不重复处理）

#### Response

```json
{
  "code": 0,
  "msg": "success",
  "data": null
}
```

---

## 错误码

| code | 常量 | 说明 |
|------|------|------|
| 0 | SUCCESS | 成功 |
| 17001 | APP_ParamError | 参数错误 |
| 17008 | APP_USER_EXISTED | 用户已存在 |
| 17009 | APP_USER_NOT_EXIST | 用户不存在 |
| 17010 | APP_LOGIN_ERR_PASS | 密码错误 |
| 17600 | AIBOT_DEFAULT | 默认错误 |
| 17601 | AIBOT_AddBotFailed | 创建分身失败 |
| 17602 | AIBOT_BotNotFound | 分身不存在 |
| 17603 | AIBOT_NoPermission | 无权限（非所有者） |
| 17604 | AIBOT_UpdateBotFailed | 更新分身失败 |
| 17605 | AIBOT_DelBotFailed | 删除分身失败 |
| 17606 | AIBOT_AddMaterialFailed | 添加素材失败 |
| 17607 | AIBOT_DelMaterialFailed | 删除素材失败 |

---

## 调用示例

### cURL 示例

```bash
# 用户注册
curl -X POST http://localhost:8050/jim/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "account": "testuser",
    "password": "123456"
  }'

# 用户登录
curl -X POST http://localhost:8050/jim/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "account": "testuser",
    "password": "123456"
  }'

# 创建分身
curl -X POST http://localhost:8050/jim/aibots/add \
  -H "Content-Type: application/json" \
  -d '{
    "unique_name": "my-assistant",
    "display_name": "我的助手",
    "greeting": "你好，我是你的AI助手"
  }'

# 查询分身列表
curl -X GET "http://localhost:8050/jim/aibots/mybots?count=10"

# 添加素材
curl -X POST http://localhost:8050/jim/aibots/my-assistant/materials/add \
  -H "Content-Type: application/json" \
  -d '{
    "type": "text",
    "title": "自我介绍",
    "content": "我是一名经验丰富的程序员，擅长Go和Python开发"
  }'

# 发起训练
curl -X POST http://localhost:8050/jim/aibots/my-assistant/training \
  -H "Content-Type: application/json" \
  -d '{"mode": "mind"}'

# 查询任务列表
curl -X GET "http://localhost:8050/jim/aibots/my-assistant/jobs?status=running"

# 激活版本
curl -X POST http://localhost:8050/jim/aibots/my-assistant/versions/v2/activate

# 查询当前版本
curl -X GET http://localhost:8050/jim/aibots/my-assistant/versions/current

# 更新分身
curl -X POST http://localhost:8050/jim/aibots/update \
  -H "Content-Type: application/json" \
  -d '{
    "bot_id": "b1234567890",
    "display_name": "新名称",
    "greeting": "新的问候语"
  }'

# 删除分身
curl -X POST http://localhost:8050/jim/aibots/remove \
  -H "Content-Type: application/json" \
  -d '{"bot_id": "b1234567890"}'
```

---

## 接口路由汇总

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/jim/user/register` | 用户注册 | 否 |
| POST | `/jim/user/login` | 用户登录 | 否 |
| POST | `/jim/aibots/add` | 创建分身 | 是 |
| POST | `/jim/aibots/update` | 更新分身 | 是 |
| POST | `/jim/aibots/remove` | 删除分身 | 是 |
| GET | `/jim/aibots/mybots` | 查询我的分身列表 | 是 |
| POST | `/jim/aibots/:unique_name/materials/add` | 新增素材 | 是 |
| GET | `/jim/aibots/:unique_name/materials` | 查询素材列表 | 是 |
| POST | `/jim/aibots/:unique_name/materials/:material_id/remove` | 删除素材 | 是 |
| POST | `/jim/aibots/:unique_name/training` | 发起训练 | 是 |
| GET | `/jim/aibots/:unique_name/jobs` | 查询任务列表 | 是 |
| GET | `/jim/aibots/:unique_name/versions` | 查询版本列表 | 是 |
| POST | `/jim/aibots/:unique_name/versions/:version/activate` | 激活版本 | 是 |
| GET | `/jim/aibots/:unique_name/versions/current` | 查询当前版本 | 是 |
| GET | `/jim/aibots/:unique_name/evaluations` | 查询评估列表 | 是 |
| POST | `/botmsgs/msgcallback` | 消息回调 | 否 |