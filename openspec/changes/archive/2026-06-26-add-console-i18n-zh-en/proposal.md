## Why

JuggleMate 管理后台（`console/web`）当前界面文案以英文硬编码为主，Ant Design 也固定使用 `en_US`，对中文运营人员不友好。需要支持中英文切换，并以中文为默认语言，降低国内团队使用门槛，同时保留英文以便对外或双语场景。

## What Changes

- 为 `console/web` 引入国际化（i18n）基础设施，支持 **中文（zh-CN）** 与 **英文（en-US）** 两种界面语言。
- **默认语言为中文（zh-CN）**；首次访问无偏好时展示中文界面。
- 在布局头部（与主题切换相邻）提供语言切换入口，用户选择后持久化到本地存储，刷新后保持。
- Ant Design `ConfigProvider` 的 `locale` 随界面语言同步切换（`zh_CN` / `en_US`）。
- 将现有页面与组件中的用户可见文案迁移到翻译资源（菜单、面包屑、登录/注册、Dashboard、Users、Inboxes、403/404、表格空态、表单校验提示、通用 CRUD 提示等）。
- 设置 `document.documentElement.lang` 与当前语言一致。

## Capabilities

### New Capabilities

- `console-web-i18n`: 管理后台前端界面语言切换（中/英）、默认中文、语言偏好持久化，以及 Ant Design 组件库 locale 联动。

### Modified Capabilities

（无。本变更仅影响前端展示与本地偏好，不改变 `embedded-console-web` 的嵌入、路由前缀或 API 契约。）

## Impact

- **Frontend**: `console/web/src/` 全站 UI 文案；新增 i18n 模块与 locale 资源文件；扩展 `stores/settings` 或独立 locale store；更新 `routes/__root.tsx`、`components/Layout/Header`、各业务路由与共享组件。
- **Dependencies**: 可能新增 `i18next`、`react-i18next`（或项目认可的轻量 i18n 方案）；Ant Design 已有 `zh_CN` / `en_US` locale 包，无需新后端依赖。
- **Backend**: 无 API 或数据库变更。
- **Build**: `pnpm build` 需通过；翻译资源随前端打包嵌入 `console/web/dist`。
