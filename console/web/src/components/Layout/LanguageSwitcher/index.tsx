import { Button, Dropdown } from "antd";
import type { MenuProps } from "antd";
import { Languages } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useSettingsStore } from "@/stores/settings";
import type { AppLocale } from "@/i18n/types";
import { LOCALE_OPTIONS } from "@/i18n/types";

export function LanguageSwitcher() {
  const { t } = useTranslation();
  const locale = useSettingsStore((s) => s.locale);
  const setLocale = useSettingsStore((s) => s.setLocale);

  const items: MenuProps["items"] = LOCALE_OPTIONS.map((option) => ({
    key: option.value,
    label: t(option.labelKey),
    onClick: () => setLocale(option.value as AppLocale),
  }));

  return (
    <Dropdown menu={{ items, selectedKeys: [locale] }} trigger={["click"]}>
      <Button type="text" icon={<Languages size={16} />} aria-label={t("common.switchLanguage")} />
    </Dropdown>
  );
}
