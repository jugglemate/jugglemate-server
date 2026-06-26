import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import zhCN from "@/locales/zh-CN.json";
import enUS from "@/locales/en-US.json";
import type { AppLocale } from "./types";
import { DEFAULT_LOCALE } from "./types";

void i18n.use(initReactI18next).init({
  resources: {
    "zh-CN": { translation: zhCN },
    "en-US": { translation: enUS },
  },
  lng: DEFAULT_LOCALE,
  fallbackLng: DEFAULT_LOCALE,
  interpolation: { escapeValue: false },
});

export function syncI18nWithLocale(locale: AppLocale) {
  if (i18n.language !== locale) {
    void i18n.changeLanguage(locale);
  }
}

export default i18n;
