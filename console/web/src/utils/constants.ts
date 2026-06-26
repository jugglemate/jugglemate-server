export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

/** Favicon path under `public/` (respects Vite `base`, e.g. `/jmateconsole/`). */
export const APP_FAVICON_SRC = `${import.meta.env.BASE_URL}favicon.svg`;

/** Product / brand name (login header, sidebar logo text, etc.). */
export const APP_BRAND_NAME = "JuggleMate";

/** Default permissions for console admins after login (until a profile API exists). */
export const CONSOLE_ADMIN_PERMISSIONS = ["user:view", "inbox:view"] as const;
