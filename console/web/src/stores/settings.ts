import { createPersistentStore } from "./createPersistentStore";
import { syncI18nWithLocale } from "@/i18n";
import type { AppLocale } from "@/i18n/types";
import { DEFAULT_LOCALE } from "@/i18n/types";

function getInitialDarkMode() {
  if (typeof window === "undefined") {
    return false;
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

interface SettingsState {
  darkMode: boolean;
  sidebarCollapsed: boolean;
  locale: AppLocale;
  toggleDarkMode: () => void;
  toggleSidebar: () => void;
  setSidebarCollapsed: (collapsed: boolean) => void;
  setLocale: (locale: AppLocale) => void;
}

export const useSettingsStore = createPersistentStore<SettingsState>(
  (set) => ({
    darkMode: getInitialDarkMode(),
    sidebarCollapsed: false,
    locale: DEFAULT_LOCALE,
    toggleDarkMode: () => set((s) => ({ darkMode: !s.darkMode })),
    toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
    setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
    setLocale: (locale) => {
      syncI18nWithLocale(locale);
      set({ locale });
    },
  }),
  {
    name: "settings-storage-basic",
    partialize: (state) => ({
      darkMode: state.darkMode,
      sidebarCollapsed: state.sidebarCollapsed,
      locale: state.locale,
    }),
    merge: (persisted, current) => {
      const merged = { ...current, ...(persisted as Partial<SettingsState>) };
      if (merged.locale) {
        syncI18nWithLocale(merged.locale);
      }
      return merged;
    },
  },
);

// Keep i18n in sync when locale is restored from storage before React mounts.
if (typeof window !== "undefined") {
  useSettingsStore.persist.onFinishHydration((state) => {
    syncI18nWithLocale(state.locale ?? DEFAULT_LOCALE);
  });
}
