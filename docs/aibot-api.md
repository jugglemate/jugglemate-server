# AI Bot 接口说明

基于当前路由定义（`routers/router.go`）：

- `POST /aibots/add`
- `POST /aibots/update`
- `POST /aibots/remove`
- `GET /aibots/mybots`

说明：
- 所有接口都需要登录态。
- bot 的更新和删除会校验归属，只有 bot 的 owner（当前登录用户）才可操作。

## 1. 创建 Bot

- 方法：`POST`
- 路径：`/aibots/add`
- Body(JSON)：

```json
{
  "nickname": "必填，bot名称",
  "avatar": "可选，头像地址",
  "prompts": "必填，提示词"
}
```

- 成功返回 data 示例：

```json
{
  "bot_id": "b1234567890",
  "nickname": "我的助手",
  "avatar": "https://xx/avatar.png"
}
```

## 2. 更新 Bot

- 方法：`POST`
- 路径：`/aibots/update`
- Body(JSON)：

```json
{
  "bot_id": "必填",
  "nickname": "可选，bot名称",
  "avatar": "可选，头像地址",
  "prompts": "可选，提示词"
}
```

- 业务规则：
- 只允许当前登录用户更新自己创建的 bot。

- 成功返回 data 示例：

```json
{
  "bot_id": "b1234567890",
  "nickname": "新名称",
  "avatar": "https://xx/new-avatar.png",
  "prompts": "新的提示词"
}
```

## 3. 删除 Bot

- 方法：`POST`
- 路径：`/aibots/remove`
- Body(JSON)：

```json
{
  "bot_id": "必填"
}
```

- 业务规则：
- 只允许当前登录用户删除自己创建的 bot。

- 成功返回：`data = null`

## 4. 分页查询我的 Bot 列表

- 方法：`GET`
- 路径：`/aibots/mybots`
- Query 参数：
- `count`：可选，每页数量，默认 `20`
- `offset`：可选，游标（上一页返回的 `offset`）

- 请求示例：

```text
GET /aibots/mybots?count=20&offset=eyJpZCI6MTIzfQ==
```

- 成功返回 data 示例：

```json
{
  "items": [
    {
      "bot_id": "b1234567890",
      "nickname": "我的助手",
      "avatar": "https://xx/avatar.png",
      "prompts": "你是一个...",
      "owner_id": "u10001",
      "created_time": 1716700000000,
      "updated_time": 1716700200000
    }
  ],
  "offset": "下一页游标"
}
```

## 错误码（AIBOT）

- `17601`：默认错误
- `17602`：bot 不存在
- `17603`：无权限（非 owner）
- `17604`：创建 bot 失败
- `17605`：更新 bot 失败
- `17606`：删除 bot 失败
- `17607`：添加素材失败
- `17608`：删除素材失败

## 5. 新增素材

- 方法：`POST`
- 路径：`/aibots/:unique_name/materials/add`

- Body(JSON)：

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

- 说明：
  - `id` 由后端生成，不由前端传入
