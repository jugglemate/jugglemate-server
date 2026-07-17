import { theme } from "antd";
import type { ConfigProviderProps, ThemeConfig } from "antd";
import { BRAND_PALETTES } from "@/utils/brandColors";

/**
 * Common theme token builders
 * Reduces duplication between light and dark theme definitions
 */

export const SHARED_DESIGN_TOKENS = {
  fontFamily:
    "'Inter', ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif",

  /**
   * 主色对齐设计稿 Material Design 3 蓝。
   *
   * TIPS: 深浅两套主题都用这一个种子。antd 的 darkAlgorithm 会自行把它推导成深色下的
   * #03509a，白字按钮在其上仍然可读；若换成提亮后的强调蓝，antd 主色按钮的白字会失效。
   */
  colorPrimary: BRAND_PALETTES.light.primaryFill,
  borderRadius: 8,
  borderRadiusSM: 6,
  borderRadiusLG: 12,
} as const;

export const CONTROL_COMPONENTS = {
  Button: {
    primaryShadow: "none",
    defaultShadow: "none",
    dangerShadow: "none",
  },
  Input: {
    activeShadow: "none",
  },
} as const;

/**
 * Calculate table row selected color based on theme algorithm
 */
export function buildTableTokens(cfg: ThemeConfig) {
  const rowSelectedBg = theme.getDesignToken(cfg).colorFillAlter;
  return {
    rowSelectedBg,
    rowSelectedHoverBg: rowSelectedBg,
  };
}

/**
 * Build menu tokens for light theme
 */
export const MENU_LIGHT = {
  itemSelectedBg: "rgba(0, 0, 0, 0.06)",
  itemSelectedColor: "rgba(0, 0, 0, 0.88)",
  subMenuItemSelectedColor: "rgba(0, 0, 0, 0.88)",
  itemHoverBg: "rgba(0, 0, 0, 0.04)",
  itemHoverColor: "rgba(0, 0, 0, 0.88)",
  itemActiveBg: "rgba(0, 0, 0, 0.08)",
  itemBorderRadius: SHARED_DESIGN_TOKENS.borderRadiusSM,
  itemMarginInline: 8,
  horizontalItemSelectedColor: "rgba(0, 0, 0, 0.88)",
  horizontalItemHoverColor: "rgba(0, 0, 0, 0.88)",
} as const;

/**
 * Build menu tokens for dark theme
 */
export const MENU_DARK = {
  itemSelectedBg: "rgba(255, 255, 255, 0.08)",
  itemSelectedColor: "rgba(255, 255, 255, 0.85)",
  subMenuItemSelectedColor: "rgba(255, 255, 255, 0.85)",
  itemHoverBg: "rgba(255, 255, 255, 0.06)",
  itemHoverColor: "rgba(255, 255, 255, 0.85)",
  itemActiveBg: "rgba(255, 255, 255, 0.1)",
  itemBorderRadius: SHARED_DESIGN_TOKENS.borderRadiusSM,
  itemMarginInline: 8,
  horizontalItemSelectedColor: "rgba(255, 255, 255, 0.85)",
  horizontalItemHoverColor: "rgba(255, 255, 255, 0.85)",
} as const;

/**
 * Build light theme config
 */
export function buildLightThemeConfig(): ConfigProviderProps {
  const lightSeed: ThemeConfig["token"] = {
    colorBgLayout: BRAND_PALETTES.light.pageBg,
    colorBgContainer: BRAND_PALETTES.light.cardBg,
    colorBorderSecondary: BRAND_PALETTES.light.borderLow,
    ...SHARED_DESIGN_TOKENS,
  };

  return {
    theme: {
      algorithm: theme.defaultAlgorithm,
      token: lightSeed,
      components: {
        ...CONTROL_COMPONENTS,
        Table: buildTableTokens({
          algorithm: theme.defaultAlgorithm,
          token: lightSeed,
        }),
        Menu: MENU_LIGHT,
      },
    },
  };
}

/**
 * Build dark theme config
 */
export function buildDarkThemeConfig(): ConfigProviderProps {
  // TIPS: 深色也必须显式播种表面色，否则 antd 会退回自带的中性灰（#141414 等），
  // 与行内样式引用的 BRAND 深色调色板对不上，出现卡片和页面底色分层错乱。
  const darkSeed: ThemeConfig["token"] = {
    colorBgLayout: BRAND_PALETTES.dark.pageBg,
    colorBgContainer: BRAND_PALETTES.dark.cardBg,
    colorBgElevated: BRAND_PALETTES.dark.subtleBg,
    colorBorderSecondary: BRAND_PALETTES.dark.borderLow,
    ...SHARED_DESIGN_TOKENS,
  };

  return {
    theme: {
      algorithm: theme.darkAlgorithm,
      token: darkSeed,
      components: {
        ...CONTROL_COMPONENTS,
        Table: buildTableTokens({
          algorithm: theme.darkAlgorithm,
          token: darkSeed,
        }),
        Menu: MENU_DARK,
      },
    },
  };
}
