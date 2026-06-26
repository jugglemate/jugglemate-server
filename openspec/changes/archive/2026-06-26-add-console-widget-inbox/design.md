## Context

Console 收件箱管理当前实现为 Telegram-only：`QryInboxes` 仅查询 `channel_type=telegram`，创建入口为 `POST /jmate/console/inboxes/telegram`，前端 Inboxes 页在 Modal 内直接进入 Telegram 三步向导（配置 → 代表 → 完成）。

`/customers/start` 已要求客户端传入管理后台创建的 `widget` inbox `inbox_id`，并从 `channel_conf` 返回 `welcome_message`（`services.WebWidgetChannelConf`）。Console 尚无法创建 widget inbox 或配置欢迎语。

Chatwoot 参考实现（`settings/inboxes/new`）采用嵌套路由：
1. `ChannelList` — 渠道卡片网格，用户选择渠道
2. `ChannelFactory` / `:sub_page` — 渠道专属配置表单
3. `AddAgents` — 分配客服
4. `FinishSetup` — 完成确认

## Goals / Non-Goals

**Goals:**

- 支持在 Console 创建 `widget` 渠道 inbox，并可配置欢迎语（`welcome_message`），持久化为 `WebWidgetChannelConf` JSON。
- 列表展示当前 app 下所有 Console 管理的 inbox（至少 `widget` + `telegram`），按 `channel_type` 区分展示。
- 前端创建流程改为：**渠道选择 → 渠道配置 → 代表分配 → 完成**，结构对齐 Chatwoot。
- Widget inbox 创建后其 `inbox_id` 与欢迎语可供 `/customers/start` 使用。

**Non-Goals:**

- Widget 嵌入脚本、外观主题、预聊天表单等 `channel_conf` 细项（欢迎语除外；其余后续迭代）。
- Telegram 渠道行为变更（除纳入统一列表/流程外）。
- 非 widget/telegram 渠道（Email、WhatsApp 等）。

## Decisions

1. **新增 `POST /jmate/console/inboxes/widget`**
   - 请求体：`{ "name": string, "welcome_message"?: string }`。
   - 持久化：`channel_type=widget`，`channel_conf` 为 `WebWidgetChannelConf` 的 JSON（含 `welcome_message`；未填时存空字符串或省略字段）。
   - 理由：与现有 `POST .../telegram` 对称；欢迎语由访客侧 `/customers/start` 读取返回。
   - 备选：统一 `POST /inboxes` + `channel_type` 字段 — 留作后续重构，本变更保持最小侵入。

2. **列表 API 改为不按单一 channel 过滤**
   - `QryInboxes` 查询当前 app 下 `widget` 与 `telegram` inbox（或去掉 channel 过滤、由 UI 展示 `channel_type`）。
   - 理由：Inboxes 页需同时管理 Web 与 Telegram 收件箱。

3. **响应模型 `channel_conf` 多态化**
   - 列表/详情按 `channel_type` 返回不同 conf 形状：Telegram 返回 `bot_name`（token 脱敏）；Widget 返回 `{ "welcome_message": string }`。
   - 理由：避免前端强依赖 `bot_name` 字段导致 widget 行展示异常；列表可预览欢迎语或留空。

4. **前端路由/步骤结构（参考 Chatwoot）**
   - Step 0：渠道选择页（卡片：Widget、Telegram），对应 Chatwoot `ChannelList`。
   - Step 1：渠道配置（Widget：名称 + 欢迎语可选输入；Telegram：名称 + bot 字段），对应 `ChannelFactory`。
   - Step 2–3：代表分配与完成（复用现有逻辑）。
   - 实现方式：可在现有 Modal 内用 `Steps` + 条件渲染，或拆子路由 `/inboxes/new`、`/inboxes/new/:channel`；优先 Modal 内分步以降低路由改动量，UI 布局参考 Chatwoot 卡片选择。

5. **成员分配 API 不变**
   - `GET/PUT /inboxes/:id/members` 对 widget 与 telegram 通用；服务层校验 inbox 属于当前 app 即可，不限制 channel type。

6. **`services.ChannelType_Widget` 与 `WebWidgetChannelConf`**
   - Widget 配置结构：`services.WebWidgetChannelConf{ WelcomeMessage }`，JSON 字段 `welcome_message`。
   - 与 `/customers/start` 校验 `channel_type == widget` 及返回 `welcome_message` 保持一致。

## Risks / Trade-offs

- [列表 `channel_conf` 类型不一致] → 前端用 discriminated union / 按 `channel_type` 分支渲染副标题。
- [欢迎语过长] → 服务端可对 `welcome_message` 设合理长度上限（如 500 字符）；空值允许。
- [Telegram-only 搜索过滤] → 列表搜索需同时匹配 name 与 telegram `bot_name`，widget 仅匹配 name。
- [Spec 名称仍为 console-telegram-inbox-management] → 本变更通过 delta 扩展需求；归档时可考虑重命名为 `console-inbox-management`（非本变更阻塞项）。

## Migration Plan

1. 部署后端 widget 创建接口与列表查询调整。
2. 部署 Console Web 新创建流程。
3. 管理员手动创建 widget inbox，将 `inbox_id` 配置到 Web 客户端 `/customers/start` 调用。
4. 无需数据迁移；已有 Telegram inbox 不受影响。

## Open Questions

- Widget 列表副标题展示文案（如 "Website" / "Widget"）— 实现时前端定英文标签即可。
- 是否在列表行展示 `inbox_id` 供复制 — 建议本变更在创建完成页展示，列表可选后续加。
