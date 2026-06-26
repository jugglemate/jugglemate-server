## Context

`console/web` 是基于 React + Ant Design + TanStack Router 的管理后台 SPA，当前：

- `routes/__root.tsx` 固定 `ConfigProvider locale={enUS}`，`document.documentElement.lang = "en"`。
- 菜单（`utils/appMenu.ts`）、面包屑（`Header`）、各业务页、表单与空态等文案均为英文硬编码字符串。
- `stores/settings.ts` 已用 `zustand` + `localStorage` 持久化 `darkMode`、`sidebarCollapsed`，可复用同一模式存储语言偏好。

本变更为纯前端横切能力，不涉及 Go 后端或 API 契约变更。

## Goals / Non-Goals

**Goals:**

- 支持 `zh-CN` 与 `en-US` 界面语言切换，**默认 `zh-CN`**。
- 语言偏好持久化（刷新、重新打开标签页后保持）。
- Ant Design 组件内置文案（日期、分页、空态等）随语言联动。
- 覆盖现有已上线页面与共享组件的用户可见文案。
- 在 Header 提供明显的语言切换控件（与主题切换并列）。

**Non-Goals:**

- 后端 API 错误码/消息的多语言（仍展示服务端返回文本或现有映射逻辑）。
- 超过中英以外的更多语言。
- URL 级 locale 路由（如 `/zh/users`）；首版仅用客户端状态 + localStorage。
- 自动根据浏览器 `navigator.language` 检测（首版固定默认中文，用户可手动切换）。

## Decisions

### 1. i18n 库：`react-i18next` + `i18next`

**选择**: 使用 `i18next` 与 `react-i18next`。

**理由**: 生态成熟、与 React 19 兼容、支持命名空间与插值；团队常见，后续扩展语言成本低。

**备选**: 自研 `useLocale` + JSON 字典——实现快但缺少复数/插值规范，不利于维护。

### 2. 翻译资源组织

**选择**: `src/locales/zh-CN.json` 与 `src/locales/en-US.json`（扁平或按模块嵌套 key，如 `menu.dashboard`、`users.title`）。

**理由**: 静态 import，Vite 打包简单，无需运行时拉取翻译文件。

### 3. 语言状态与持久化

**选择**: 扩展 `useSettingsStore`（或同级 `locale` slice），字段 `locale: 'zh-CN' | 'en-US'`，默认 `'zh-CN'`，`partialize` 写入 `localStorage`（与 `settings-storage-basic` 同 key 或独立 key）。

**理由**: 与现有主题/侧栏偏好一致；Header 已依赖 `useSettingsStore`。

### 4. Ant Design locale 联动

**选择**: 在 `__root.tsx` 根据 `locale` 选择 `antd/locale/zh_CN` 或 `antd/locale/en_US`，传入 `ConfigProvider`。

**理由**: 避免 Table、DatePicker、Modal 等组件仍显示英文。

### 5. 菜单与面包屑

**选择**: `APP_MENU_TREE` 保留稳定 `id`/`path`，展示名改为 i18n key（如 `menu.dashboard`）；`Header` 的 `PATH_LABEL` 同样改为 `t()` 查找。

**理由**: 路径与权限逻辑不变，仅展示层国际化。

### 6. 语言切换 UI

**选择**: Header `Space` 内增加 `Dropdown` 或 `Segmented`（中文 / English），选中项高亮；`aria-label` 使用当前语言文案。

**理由**: 与现有主题按钮位置一致，改动面小。

### 7. 默认语言

**选择**: 无存储值时 `locale = 'zh-CN'`；**不**读取 `navigator.language`。

**理由**: 与产品要求「默认中文」一致，避免英文系统用户首次看到英文。

## Risks / Trade-offs

- **[Risk] 漏翻硬编码字符串** → 实施阶段用 `rg` 扫引号文案 + 手工走查主要路由；tasks 中按目录拆分。
- **[Risk] 新增页面未接 i18n** → 在 `tasks.md` 约定新文案必须走 `t()`；可选 ESLint 规则后续再加。
- **[Trade-off] 首版不跟浏览器语言** → 海外用户需手动切英文，可接受因明确默认中文。
- **[Trade-off] 单文件 JSON 变大** → 当前页面数量有限，可接受；日后可按 namespace 拆分。

## Migration Plan

1. 引入依赖与 i18n 初始化（默认 `zh-CN`）。
2. 扩展 settings store 与 Header 切换器。
3. 逐模块迁移文案（layout → auth → users → inboxes → 错误页）。
4. `pnpm build` 验证；重新嵌入 `console/web/dist` 随既有后端构建流程发布。
5. **回滚**: 还原前端提交即可，无数据迁移。

## Open Questions

（无阻塞项；实施按 tasks 执行即可。）
