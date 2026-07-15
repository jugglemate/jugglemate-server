/**
 * JuggleMate 品牌配色（对齐设计稿的 Material Design 3 调色板）。
 * TIPS: 这些常量是设计稿里固定的品牌色，与 antd 主题算法派生的 token 并行使用；
 * 侧边栏、角色标签、统计卡等需要精确还原设计稿处直接引用本文件，避免各处硬编码色值漂移。
 */
export const BRAND = {
  /** 主色（按钮 / 选中态 / 链接） */
  primary: "#005bb2",
  /** 主色悬浮态 */
  primaryHover: "#0073df",
  /** 主色浅底（角色胶囊、统计卡图标底） */
  primarySoft: "rgba(0, 91, 178, 0.1)",

  /** 侧边栏深色底（on-background） */
  sidebarBg: "#151c27",
  /** 侧边栏品牌名 / 主要文字（surface-bright） */
  sidebarText: "#f9f9ff",
  /** 侧边栏次要文字（AI ENGAGEMENT 副标题等） */
  sidebarTextMuted: "rgba(220, 226, 243, 0.7)",
  /** 侧边栏未选中菜单文字 */
  sidebarItemText: "rgba(235, 241, 255, 0.75)",
  /** 侧边栏悬浮底 */
  sidebarItemHoverBg: "rgba(220, 226, 243, 0.1)",
  /** 侧边栏分隔线 */
  sidebarBorder: "rgba(193, 198, 214, 0.2)",

  /** 页面底色（surface） */
  pageBg: "#f9f9ff",
  /** 卡片 / 表格白底（surface-container-lowest） */
  cardBg: "#ffffff",
  /** 表头 / 分页条浅底（surface-subtle） */
  subtleBg: "#f9fafb",
  /** 浅边框（border-low） */
  borderLow: "#e5e7eb",
  /** 统计卡底色（surface-container） */
  statCardBg: "#e7eefe",
  /** 统计卡边框 */
  statCardBorder: "rgba(193, 198, 214, 0.3)",

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

  /** 紫色强调（Manager 角色 / ai-accent） */
  aiAccent: "#6550b9",
  /** 绿色（在线 / Active 状态、Active Seats 图标） */
  success: "#16a34a",
  successSoft: "rgba(22, 163, 74, 0.1)",
  tertiary: "#00685c",
  tertiarySoft: "rgba(0, 104, 92, 0.1)",
  /** 橙色（Invited 状态 / Pending 图标） */
  warning: "#f97316",
  warningSoft: "#ffedd5",
} as const;
