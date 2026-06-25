import { ApiError } from "@/utils/http";

const AUTH_ERROR_MESSAGES: Record<number, string> = {
  17001: "缺少 appkey，请先通过 ?appkey=xxx 访问控制台",
  17002: "应用不存在",
  17004: "请输入有效的账号和密码",
  17011: "用户已存在",
  17012: "用户不存在",
  17013: "密码错误",
};

export function getAuthErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    return AUTH_ERROR_MESSAGES[error.code] ?? `${fallback}（${error.code}）`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}

export function getLoginErrorMessage(error: unknown): string {
  return getAuthErrorMessage(error, "登录失败");
}

export function getRegisterErrorMessage(error: unknown): string {
  return getAuthErrorMessage(error, "注册失败");
}
