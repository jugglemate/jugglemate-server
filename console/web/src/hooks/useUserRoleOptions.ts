import { useMemo } from "react";
import { useTranslation } from "react-i18next";

export function useUserRoleOptions() {
  const { t } = useTranslation();

  return useMemo(
    () =>
      [
        { label: t("users.role.admin"), value: "admin" as const },
        { label: t("users.role.customer"), value: "customer" as const },
      ] as const,
    [t],
  );
}
