# LLM 文本翻译配置与验证

本文用于配置 JuggleRouter 翻译模型，并验证内部接口 `POST /api/v1/llm/translate`。接口使用 Agent API 统一响应信封：成功时 `code` 为 `0`，调用方应检查响应体 `code`，不能只看 HTTP 状态码。

## 1. 准备变量

以下命令均使用占位变量，不要把真实密钥写入文档、脚本、命令历史或 Git：

```bash
export AGENT_BASE_URL='http://127.0.0.1:8050'
export INTERNAL_AUTH='<internal-auth-secret>'
export INTERNAL_USER_ID='<internal-user-id>'
export AGENT_POSTGRES_DSN='<postgres-dsn>'
```

`X-Internal-Auth` 必须使用服务端配置的内部共享密钥；原生 `/api/v1` 鉴权还要求 `X-User-ID`。这些 Header 只能由可信后端发送，不得暴露给浏览器或终端用户。

## 2. 在 models 页面配置 JuggleRouter

登录管理后台，进入 `/jmateconsole/models`，新建 Provider 并填写：

| 字段 | 值 |
| --- | --- |
| 名称 | `JuggleRouter` |
| Protocol | `openai-compatible` |
| Status | `active` |
| Base URL | `https://jugglerouter.com/v1` |
| API Key | 从安全的密钥管理系统取得；不要使用本文中的占位符 |

在同一页面为该 Provider 新建模型：

| 字段 | 值 |
| --- | --- |
| Model ID | `claude-sonnet-4-5` |
| Display Name | `Claude Sonnet 4.5` |
| API Model Name | `claude-sonnet-4-5` |
| Status | `active` |
| 能力 | 勾选 `reasoning`；无需新增 translation 能力 |
| Context Length / Max Output | 按 JuggleRouter 当前模型规格填写，且 Max Output 不得大于 Context Length |
| Input / Output Price | 按当前结算价格填写，用于 `cost` 计算，不能填负数 |

保存后在 models 列表点击“测试连接”。Provider 和模型都必须为 `active`。

## 3. 设置默认翻译模型

默认配置接口接收字段 **`translation_model`**。不要发送存储层键名 `default_translation_model`。

```bash
curl --fail-with-body -sS \
  -X PUT "${AGENT_BASE_URL}/api/v1/admin/llm/defaults" \
  -H "X-Internal-Auth: ${INTERNAL_AUTH}" \
  -H "X-User-ID: ${INTERNAL_USER_ID}" \
  -H 'Content-Type: application/json' \
  --data '{"translation_model":"claude-sonnet-4-5"}'
```

预期响应（其他默认模型值以当前环境为准）：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "reasoning_model": null,
    "summary_model": null,
    "embedding_model": null,
    "translation_model": "claude-sonnet-4-5"
  }
}
```

## 4. 调用翻译接口

```bash
curl --fail-with-body -sS \
  -X POST "${AGENT_BASE_URL}/api/v1/llm/translate" \
  -H "X-Internal-Auth: ${INTERNAL_AUTH}" \
  -H "X-User-ID: ${INTERNAL_USER_ID}" \
  -H 'Content-Type: application/json' \
  --data '{
    "source": "Welcome to JuggleMate.",
    "source_language": "English",
    "target_language": "简体中文",
    "glossary": {
      "JuggleMate": "JuggleMate"
    }
  }'
```

请求中不要传 `model_id`；模型仅由管理员设置的 `translation_model` 决定。预期成功响应示例：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "content": "欢迎使用 JuggleMate。",
    "model_id": "claude-sonnet-4-5",
    "usage": {
      "input_tokens": 96,
      "output_tokens": 12,
      "total_tokens": 108
    },
    "cost": 0.000468,
    "response_time_ms": 842
  }
}
```

Token、费用、耗时和具体译文由实际调用决定。若未配置默认翻译模型，响应 `msg` 应为 `未配置默认翻译模型，请先在管理后台设置`。

## 5. SQL 验证调用流水

执行一次翻译后查询最新流水：

```bash
psql "$AGENT_POSTGRES_DSN" -v ON_ERROR_STOP=1 -c "
SELECT model_id,
       call_type,
       status,
       input_tokens,
       output_tokens,
       total_tokens,
       cost,
       response_time,
       request_time
FROM llm_model_calls
WHERE call_type = 'translation'
ORDER BY request_time DESC
LIMIT 5;
"
```

至少应出现一行 `call_type=translation`，`model_id=claude-sonnet-4-5`；成功调用的 `status` 为 `success`。部署接口前应先应用允许 `translation` 的数据库迁移，否则流水可能因约束失败而无法写入。

## 6. 安全检查

- 确认 Provider 查询或 models 页面只显示掩码，不返回明文 API key；数据库中应为 `api_key_encrypted`，不要直接修改为明文。
- 检查本次请求对应时段的普通应用日志，确认没有 JuggleRouter API key、`X-Internal-Auth`、完整 `source` 或 glossary 内容。
- `llm_model_calls` 只应保存模型、Provider、Token、费用、耗时、状态和错误诊断，不应保存翻译原文或译文。
- 不要使用 `set -x`，不要把真实 secret 写入 shell 脚本、截图、工单、聊天记录或提交到仓库；验证结束后可执行 `unset INTERNAL_AUTH AGENT_POSTGRES_DSN`。
