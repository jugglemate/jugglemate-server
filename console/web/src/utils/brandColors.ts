/**
 * JuggleMate 品牌配色（对齐设计稿的 Material Design 3 调色板）。
 *
 * TIPS: 这里同时维护浅色与深色两套调色板，两者 key 必须完全一致（由 `typeof LIGHT_PALETTE`
 * 在编译期强制）。业务代码一律引用 `BRAND`，它的值是 CSS 变量而不是具体色值 —— 因为绝大多数
 * 调用点是行内样式（style={{...}}），行内样式无法响应主题切换，只有 CSS 变量能跟着
 * `<html data-theme>` 一起翻转。真实色值只在两处使用：注入 CSS 变量（applyBrandPalette）
 * 和给 antd 主题算法做种子（tokenBuilders）—— antd 需要能解析的真实颜色。
 */

/** 浅色调色板（设计稿基准）。 */
const LIGHT_PALETTE = {
  /* ---------- 品牌主色 ---------- */
  /** 主色，作为“表面上的强调色”使用（文字 / 图标 / 选中描边）。 */
  primary: "#005bb2",
  /** 主色悬浮态 */
  primaryHover: "#0073df",
  /** 主色浅底（角色胶囊、统计卡图标底） */
  primarySoft: "rgba(0, 91, 178, 0.1)",
  /**
   * 主色实底（按钮 / 选中菜单胶囊 / 步骤圆点等“填充”场景），搭配 onPrimaryFill 使用。
   *
   * TIPS: 深色下不能与 primary 合并成一个值：填充态要配白字，必须足够深；而强调色要在深色卡片
   * 上可读，必须足够浅 —— 同一个颜色无法同时满足两边的 4.5:1 对比度，因此按角色拆成两个 token。
   */
  primaryFill: "#005bb2",
  /** primaryFill 之上的文字 / 图标色 */
  onPrimaryFill: "#ffffff",

  /* ---------- 侧边栏 ---------- */
  /** 侧边栏底色 */
  sidebarBg: "#ffffff",
  /** 侧边栏品牌名 / 主要文字 */
  sidebarText: "#151c27",
  /** 侧边栏次要文字（分组标题等） */
  sidebarTextMuted: "#717785",
  /** 侧边栏未选中菜单文字 */
  sidebarItemText: "#414754",
  /** 侧边栏悬浮底 */
  sidebarItemHoverBg: "rgba(0, 91, 178, 0.06)",
  /** 侧边栏分隔线 */
  sidebarBorder: "#e5e7eb",

  /* ---------- 代码 / Prompt 编辑器 ---------- */
  /**
   * 代码块与 Prompt 编辑器底色。
   *
   * TIPS: 两套主题下都保持深色（编辑器惯例），所以浅色值与深色值一致；不要复用 sidebarBg，
   * 侧边栏现在会跟随主题翻转，浅色下会变成白底而让 codeText 不可见。
   */
  codeBg: "#151c27",
  /** 代码块文字 */
  codeText: "#e2e8f0",

  /* ---------- 表面 ---------- */
  /** 页面底色（surface） */
  pageBg: "#f9f9ff",
  /** 卡片 / 表格底（surface-container-lowest） */
  cardBg: "#ffffff",
  /** 表头 / 分页条浅底（surface-subtle） */
  subtleBg: "#f9fafb",
  /** 浅边框（border-low） */
  borderLow: "#e5e7eb",
  /** 统计卡底色（surface-container） */
  statCardBg: "#e7eefe",
  /** 统计卡边框 */
  statCardBorder: "rgba(193, 198, 214, 0.3)",

  /* ---------- 文字 ---------- */
  /** 主文字（on-surface） */
  onSurface: "#151c27",
  /** 次要文字（on-surface-variant） */
  onSurfaceVariant: "#414754",
  /** 弱化文字（outline，表头 / 副标题 / 时间） */
  outline: "#717785",

  /** 灰色角色胶囊底（surface-variant） */
  surfaceVariant: "#dce2f3",
  /** 头像占位底（secondary-container） */
  secondaryContainer: "#d9dff5",
  /** 头像占位文字（secondary） */
  secondary: "#575e70",

  /* ---------- 语义强调色（均用于浅底之上的文字 / 图标） ---------- */
  /** 紫色强调（Manager 角色 / ai-accent） */
  aiAccent: "#6550b9",
  /** 紫色浅底（JuggleIM 渠道图标底） */
  aiAccentSoft: "rgba(101, 80, 185, 0.1)",
  /** 绿色（在线 / Active 状态） */
  success: "#16a34a",
  successSoft: "rgba(22, 163, 74, 0.1)",
  tertiary: "#00685c",
  tertiarySoft: "rgba(0, 104, 92, 0.1)",
  /** 橙色（Invited 状态 / Pending 图标） */
  warning: "#f97316",
  warningSoft: "#ffedd5",
};

/**
 * 深色调色板（按 MD3 深色 surface 层级从浅色稿推导，色相与侧边栏 #151c27 保持同系）。
 *
 * TIPS: 类型标注为 `typeof LIGHT_PALETTE`，漏掉任何一个 key 都会编译失败。
 */
const DARK_PALETTE: typeof LIGHT_PALETTE = {
  /* ---------- 品牌主色 ---------- */
  // 强调色提亮到在深色卡片上可读（#9ec5ff on #171f2b ≈ 9.4:1）。
  primary: "#9ec5ff",
  primaryHover: "#c6dcff",
  primarySoft: "rgba(158, 197, 255, 0.16)",
  // 填充色沿用 antd darkAlgorithm 从 #005bb2 推导出的主色，保证与 antd 自带组件一致，
  // 且白字在其上仍有 8:1 对比度。
  primaryFill: "#03509a",
  onPrimaryFill: "#ffffff",

  /* ---------- 侧边栏 ---------- */
  sidebarBg: "#151c27",
  sidebarText: "#f9f9ff",
  sidebarTextMuted: "rgba(220, 226, 243, 0.7)",
  sidebarItemText: "rgba(235, 241, 255, 0.75)",
  sidebarItemHoverBg: "rgba(220, 226, 243, 0.1)",
  sidebarBorder: "rgba(193, 198, 214, 0.2)",

  /* ---------- 代码 / Prompt 编辑器 ---------- */
  codeBg: "#151c27",
  codeText: "#e2e8f0",

  /* ---------- 表面 ---------- */
  pageBg: "#0f151e",
  cardBg: "#171f2b",
  subtleBg: "#1e2733",
  borderLow: "#2c3542",
  statCardBg: "#1a2740",
  statCardBorder: "rgba(193, 198, 214, 0.16)",

  /* ---------- 文字 ---------- */
  onSurface: "#e6eaf5",
  onSurfaceVariant: "#c3c9d8",
  outline: "#8d94a5",

  surfaceVariant: "#2e3546",
  secondaryContainer: "#333c53",
  secondary: "#bac1d6",

  /* ---------- 语义强调色 ---------- */
  aiAccent: "#b3a4f0",
  aiAccentSoft: "rgba(179, 164, 240, 0.16)",
  success: "#4ade80",
  successSoft: "rgba(74, 222, 128, 0.16)",
  tertiary: "#4dd0c0",
  tertiarySoft: "rgba(77, 208, 192, 0.16)",
  warning: "#fb923c",
  warningSoft: "rgba(251, 146, 60, 0.18)",
};

type BrandKey = keyof typeof LIGHT_PALETTE;

/** 真实色值调色板，仅供 antd 主题种子与 CSS 变量注入使用。 */
export const BRAND_PALETTES = {
  light: LIGHT_PALETTE,
  dark: DARK_PALETTE,
};

/** camelCase key → CSS 变量名，例如 cardBg → --jm-card-bg。 */
function cssVarName(key: string): string {
  return `--jm-${key.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}`;
}

/**
 * 品牌配色（CSS 变量形式）。行内样式引用它即可随主题自动翻转。
 */
export const BRAND = Object.fromEntries(
  (Object.keys(LIGHT_PALETTE) as BrandKey[]).map((key) => [key, `var(${cssVarName(key)})`]),
) as Record<BrandKey, string>;

/**
 * 把指定主题的调色板写入 `<html>` 的 CSS 变量。
 *
 * TIPS: 控制台是纯客户端渲染（index.html 里 #root 为空），因此在 effect 里注入不会闪烁。
 */
export function applyBrandPalette(darkMode: boolean): void {
  const palette = darkMode ? DARK_PALETTE : LIGHT_PALETTE;
  const root = document.documentElement;
  for (const [key, value] of Object.entries(palette)) {
    root.style.setProperty(cssVarName(key), value);
  }
}
