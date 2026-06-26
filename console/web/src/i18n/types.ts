export type AppLocale = "zh-CN" | "en-US";

export const DEFAULT_LOCALE: AppLocale = "zh-CN";

export const LOCALE_OPTIONS: { value: AppLocale; labelKey: string }[] = [
  { value: "zh-CN", labelKey: "common.locale.zh" },
  { value: "en-US", labelKey: "common.locale.en" },
];
