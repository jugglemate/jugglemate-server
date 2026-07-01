// Agent-server 管理 API 代理客户端。
//
// 所有请求经由后端 Go 反向代理 `/jmate/agentapi/*` 转发到 agent-server 的 `/api/v1/*`：
// - 同源，无需 CORS；
// - 复用 console 的 httpClient（自动注入 appkey + Authorization，统一解包 {code,msg,data} 信封）；
// - 身份/owner 头（X-User-ID 等）由 Go 代理注入，前端不发送。
//
// 用法：agentApi.get("/tool-providers") -> GET /jmate/agentapi/tool-providers
import { httpClient } from "./http";

/** Go 反向代理前缀，对应 agent-server 的 /api/v1。 */
export const AGENT_API_PREFIX = "/jmate/agentapi";

type RequestOptions = Parameters<typeof httpClient.get>[1];

const withPrefix = (path: string): string => `${AGENT_API_PREFIX}${path}`;

export const agentApi = {
  get: <T>(path: string, options?: RequestOptions) => httpClient.get<T>(withPrefix(path), options),
  post: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    httpClient.post<T>(withPrefix(path), body, options),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    httpClient.put<T>(withPrefix(path), body, options),
  delete: <T>(path: string, options?: RequestOptions) =>
    httpClient.delete<T>(withPrefix(path), options),
};
