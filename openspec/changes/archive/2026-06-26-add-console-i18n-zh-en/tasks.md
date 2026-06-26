## 1. i18n 基础设施

- [x] 1.1 在 `console/web` 添加 `i18next`、`react-i18next` 依赖
- [x] 1.2 新增 `src/i18n/index.ts`：初始化 i18n，注册 `zh-CN`（默认）与 `en-US` 资源
- [x] 1.3 新增 `src/locales/zh-CN.json` 与 `src/locales/en-US.json` 基础结构（`common`、`menu` 等命名空间）
- [x] 1.4 在 `src/main.tsx` 引入 i18n 初始化（确保在根组件渲染前加载）

## 2. 语言状态与根配置

- [x] 2.1 扩展 `stores/settings.ts`：增加 `locale: 'zh-CN' | 'en-US'`（默认 `zh-CN`）与 `setLocale`，并持久化到 localStorage
- [x] 2.2 更新 `routes/__root.tsx`：根据 `locale` 切换 Ant Design `zh_CN` / `en_US`，同步 `document.documentElement.lang`
- [x] 2.3 将 i18n 实例语言与 store 双向同步（切换 store 时调用 `i18n.changeLanguage`）

## 3. 布局与全局控件

- [x] 3.1 在 `components/Layout/Header` 增加语言切换控件（中文 / English），与主题按钮并列
- [x] 3.2 国际化 `Header` 面包屑与 `aria-label`
- [x] 3.3 国际化 `components/Layout/Sidebar` 菜单展示（基于 `APP_MENU_TREE` 的 i18n key）
- [x] 3.4 国际化 `components/Layout/UserMenu`、`AppFooter` 等布局子组件文案

## 4. 认证与错误页

- [x] 4.1 国际化 `routes/login`、`routes/register` 页面文案与表单校验提示
- [x] 4.2 国际化 `routes/404`、`routes/_auth/403`、`components/NotFound`、`components/RouteError`

## 5. 业务页面

- [x] 5.1 国际化 `routes/_auth/dashboard` 页面文案
- [x] 5.2 国际化 `routes/_auth/users`（含 Toolbar、FormModal）及 `hooks/useCrudToasts` 通用提示
- [x] 5.3 国际化 `routes/_auth/inboxes` 创建流程与列表文案
- [x] 5.4 国际化共享组件：`DataTable`、`DataTableEmpty`、`FilterToolbar`、`FormModal`、`utils/authErrors`

## 6. 验证与收尾

- [x] 6.1 手工验证：默认中文、切换英文、刷新后偏好保持、Ant Design 分页/空态随语言变化
- [x] 6.2 运行 `pnpm build`（`console/web`）确保 TypeScript 与构建通过
- [x] 6.3 走查主要路由，修复遗漏的硬编码英文字符串
