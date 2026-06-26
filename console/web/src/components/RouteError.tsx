import { Button, Result, theme } from "antd";
import { useRouter, type ErrorComponentProps } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export function RouteError({ error }: ErrorComponentProps) {
  const router = useRouter();
  const { token } = theme.useToken();
  const { t } = useTranslation();

  const detail = error instanceof Error ? error.message : String(error);

  return (
    <div style={{ padding: token.paddingLG, maxWidth: 560, margin: "48px auto" }}>
      <Result
        status="error"
        title={t("errors.routeErrorTitle")}
        subTitle={detail || t("errors.routeErrorSubtitle")}
        extra={
          <Button type="primary" onClick={() => void router.invalidate()}>
            {t("common.retry")}
          </Button>
        }
      />
    </div>
  );
}
