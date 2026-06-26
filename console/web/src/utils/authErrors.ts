import { ApiError } from "@/utils/http";
import i18n from "@/i18n";

const AUTH_ERROR_KEYS: Record<number, string> = {
  17001: "auth.missingAppkey",
  17002: "auth.appNotFound",
  17004: "auth.invalidCredentials",
  17011: "auth.userExists",
  17012: "auth.userNotFound",
  17013: "auth.wrongPassword",
};

export function getAuthErrorMessage(error: unknown, fallbackKey: string): string {
  if (error instanceof ApiError) {
    const key = AUTH_ERROR_KEYS[error.code];
    if (key) {
      return i18n.t(key);
    }
    return i18n.t("auth.errorWithCode", {
      message: i18n.t(fallbackKey),
      code: error.code,
    });
  }
  if (error instanceof Error) {
    return error.message;
  }
  return i18n.t(fallbackKey);
}

export function getLoginErrorMessage(error: unknown): string {
  return getAuthErrorMessage(error, "auth.loginFailed");
}

export function getRegisterErrorMessage(error: unknown): string {
  return getAuthErrorMessage(error, "auth.registerFailed");
}
