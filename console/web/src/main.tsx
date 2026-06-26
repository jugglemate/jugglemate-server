import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { routeTree } from "./routeTree.gen";
import { useAuthStore } from "./stores/auth";
import { useSettingsStore } from "./stores/settings";
import { fetchSessionAndApplyToStore } from "./utils/session";
import { installHttpRouter } from "./utils/http";

import { persistAppKeyFromUrl } from "./utils/appkey";
import "@/i18n";
import { syncI18nWithLocale } from "@/i18n";
import { DEFAULT_LOCALE } from "@/i18n/types";

const router = createRouter({ routeTree, basepath: "/jmateconsole" });
installHttpRouter(router);

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

async function enableMocking() {
  const enableMockInBuild = import.meta.env.VITE_ENABLE_MOCK === "true";
  if (!import.meta.env.DEV && !enableMockInBuild) return;
  const { worker } = await import("./mocks/browser");
  return worker.start({ onUnhandledRequest: "bypass" });
}

enableMocking()
  .then(async () => {
    persistAppKeyFromUrl();
    await Promise.all([useSettingsStore.persist.rehydrate(), useAuthStore.persist.rehydrate()]);
    const { locale } = useSettingsStore.getState();
    syncI18nWithLocale(locale ?? DEFAULT_LOCALE);
    const { isAuthenticated, tokens } = useAuthStore.getState();
    if (isAuthenticated && tokens) {
      try {
        await fetchSessionAndApplyToStore();
      } catch {
        useAuthStore.getState().logout();
      }
    }

    ReactDOM.createRoot(document.getElementById("root")!).render(
      <React.StrictMode>
        <RouterProvider router={router} />
      </React.StrictMode>,
    );
  })
  .catch((err) => {
    console.error("Failed to initialize app:", err);
  });
