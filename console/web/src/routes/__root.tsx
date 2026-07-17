import { useEffect, useMemo } from "react";
import { createRootRoute, Outlet } from "@tanstack/react-router";
import { ConfigProvider, App } from "antd";
import zhCN from "antd/locale/zh_CN";
import enUS from "antd/locale/en_US";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSettingsStore } from "@/stores/settings";
import { useAppTheme } from "@/hooks/useAppTheme";
import { applyBrandPalette } from "@/utils/brandColors";
import { NotFound } from "@/components/NotFound";
import { RouteError } from "@/components/RouteError";
import type { AppLocale } from "@/i18n/types";
import "@/index.css";

const ANT_LOCALES: Record<AppLocale, typeof zhCN> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

function AppQueryBridge({ children }: { children: React.ReactNode }) {
  const { message } = App.useApp();
  const queryClient = useMemo(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: 1, refetchOnWindowFocus: false },
          mutations: {
            onError: (err) => {
              message.error(err instanceof Error ? err.message : String(err));
            },
          },
        },
      }),
    [message],
  );

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function RootComponent() {
  const darkMode = useSettingsStore((s) => s.darkMode);
  const locale = useSettingsStore((s) => s.locale);
  const configProviderProps = useAppTheme();

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", darkMode ? "dark" : "light");
    applyBrandPalette(darkMode);
  }, [darkMode]);

  return (
    <ConfigProvider {...configProviderProps} locale={ANT_LOCALES[locale]}>
      <App>
        <AppQueryBridge>
          <Outlet />
        </AppQueryBridge>
      </App>
    </ConfigProvider>
  );
}

export const Route = createRootRoute({
  component: RootComponent,
  notFoundComponent: NotFound,
  errorComponent: RouteError,
});
