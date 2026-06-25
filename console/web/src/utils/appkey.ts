const APPKEY_QUERY = "appkey";
const APPKEY_STORAGE_KEY = "jgm-console-appkey";

export function saveAppKey(appkey: string): void {
  const value = appkey.trim();
  if (value) {
    localStorage.setItem(APPKEY_STORAGE_KEY, value);
  }
}

export function getStoredAppKey(): string {
  return localStorage.getItem(APPKEY_STORAGE_KEY)?.trim() ?? "";
}

/** Read `appkey` from URL search and persist to localStorage when present. */
export function persistAppKeyFromUrl(search = window.location.search): string {
  const fromUrl = new URLSearchParams(search).get(APPKEY_QUERY)?.trim() ?? "";
  if (fromUrl) {
    saveAppKey(fromUrl);
    return fromUrl;
  }
  return "";
}

/** Resolve appkey: URL param → localStorage → optional build-time fallback. */
export function getAppKey(): string {
  const fromUrl = persistAppKeyFromUrl();
  if (fromUrl) return fromUrl;

  const stored = getStoredAppKey();
  if (stored) return stored;

  return import.meta.env.VITE_APP_KEY?.trim() ?? "";
}

/** @deprecated Use persistAppKeyFromUrl */
export const syncAppKeyFromUrl = persistAppKeyFromUrl;
