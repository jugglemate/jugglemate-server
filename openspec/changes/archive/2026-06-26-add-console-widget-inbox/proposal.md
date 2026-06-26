## Why

Web 访客会话接口 `/customers/start` 已改为依赖管理后台预先创建的 `widget` 渠道 inbox，但 Console 目前只能创建 Telegram 收件箱，且创建流程直接进入 Telegram 配置，无法选择渠道。管理员无法通过后台创建 widget inbox，导致 Web 客服无法接入。

## What Changes

- 管理后台创建收件箱时增加 **Widget** 渠道；创建时可配置 **欢迎语**（`welcome_message`），序列化存入 `inboxes.channel_conf`（`WebWidgetChannelConf`）。
- 重构 Console 创建收件箱 UI：**先选渠道，再进入对应配置页**，交互参考 Chatwoot 的 `ChannelList` → `ChannelFactory` 分步流程。
- 后端 Console API 扩展为支持多渠道创建与列表（至少 `widget` 与 `telegram`），列表不再仅过滤 Telegram。
- Widget inbox 创建需收件箱名称，欢迎语可选；Telegram 仍要求 bot name 与 bot token。
- 创建完成后继续沿用现有代表分配与完成步骤（与 Telegram 流程一致）。
- `/customers/start` 已从 widget inbox 的 `channel_conf` 解析并返回 `welcome_message`；本变更补齐管理端配置入口。

## Capabilities

### New Capabilities

（无独立新 capability；本变更在现有收件箱管理能力上扩展 widget 与多渠道 UI。）

### Modified Capabilities

- `console-telegram-inbox-management`: 从「仅 Telegram」扩展为 Console 多渠道收件箱管理（列表、创建、代表分配），新增 widget 渠道创建与「先选渠道再配置」的 Web 流程要求。

## Impact

- **Backend**: `console/services/inboxservice.go`、`console/apis/inbox.go`、`console/apis/models/inbox.go`；可能新增通用 create 路由或 `POST /inboxes/widget`。
- **Frontend**: `console/web/src/routes/_auth/inboxes/` 及相关 API schemas；创建流程拆分为渠道选择 + 渠道配置子步骤。
- **Storage**: `inboxes` 表已有 `channel_type` / `channel_conf`，widget 使用 `channel_type=widget`。
- **Related API**: `/jmate/customers/start` 已要求 widget inbox 存在，并返回 `welcome_message`；本变更补齐管理端创建与欢迎语配置能力。
- **Reference**: Chatwoot `settings/inboxes/new` → `ChannelList.vue` + `ChannelFactory.vue` 路由与步骤结构。
