import { Button, Result, theme } from "antd";
import { useRouter, type ErrorComponentProps } from "@tanstack/react-router";

export function RouteError({ error }: ErrorComponentProps) {
  const router = useRouter();
  const { token } = theme.useToken();

  const detail = error instanceof Error ? error.message : String(error);

  return (
    <div style={{ padding: token.paddingLG, maxWidth: 560, margin: "48px auto" }}>
      <Result
        status="error"
        title="Something went wrong"
        subTitle={detail || "Please try again or return to the dashboard."}
        extra={
          <Button type="primary" onClick={() => void router.invalidate()}>
            Retry
          </Button>
        }
      />
    </div>
  );
}
