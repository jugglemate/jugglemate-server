// Agent 对话 SSE 流式客户端。
//
// 经 Go 反向代理 `/jmate/agentapi/chat/stream` 请求 agent-server，注入 appkey + Authorization
// （owner 身份头由 Go 代理注入，前端不发）。不能用原生 EventSource（无法带鉴权头），
// 改用 fetch + ReadableStream 读取 SSE。
import { API_BASE_URL } from "./constants";
import { AGENT_API_PREFIX } from "./agentHttp";
import { getAppKey } from "./appkey";
import { getAccessToken } from "@/stores/auth";

export interface ChatRequest {
  agentId: string;
  userId: string;
  message: string;
  conversationId?: string | null;
  metadata?: Record<string, unknown>;
}

export interface ChatResponse {
  conversationId: string;
  answer: string;
  path: string;
  status: string;
  traceId: string;
  errorCode?: string | null;
}

export interface SSEEvent {
  event: string;
  traceId: string;
  sessionId?: string;
  roundNumber?: number;
  confidenceScore?: number;
  payload: Record<string, unknown>;
}

async function parseError(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as {
      code?: number | string;
      msg?: string;
      detail?: { message?: string };
    };
    if (data && typeof data === "object" && "msg" in data && data.code !== 0) {
      return typeof data.msg === "string" ? data.msg : `HTTP ${response.status}`;
    }
    return data?.detail?.message ?? `HTTP ${response.status}`;
  } catch {
    return `HTTP ${response.status}`;
  }
}

/**
 * 发起对话 SSE 流。返回一个取消函数。
 */
export function createChatSSE(
  payload: ChatRequest,
  onEvent: (event: SSEEvent) => void,
  onJsonResponse: (response: ChatResponse) => void,
  onError: (error: Error) => void,
  onComplete: () => void,
): () => void {
  const url = `${API_BASE_URL}${AGENT_API_PREFIX}/chat/stream`;
  const controller = new AbortController();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "text/event-stream, application/json",
  };
  const appkey = getAppKey();
  if (appkey) headers.appkey = appkey;
  const token = getAccessToken();
  if (token) headers.Authorization = token;

  void fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify(payload),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const contentType = response.headers.get("content-type") ?? "";

      if (contentType.includes("application/json")) {
        const raw = (await response.json()) as unknown;
        let data = raw;
        if (raw && typeof raw === "object" && "code" in raw && "data" in raw) {
          const env = raw as { code: number | string; msg?: string; data: unknown };
          if (env.code !== 0)
            throw new Error(typeof env.msg === "string" ? env.msg : "对话请求失败");
          data = env.data;
        }
        onJsonResponse(data as ChatResponse);
        onComplete();
        return;
      }

      if (!contentType.includes("text/event-stream")) {
        throw new Error(`Unsupported chat response type: ${contentType || "unknown"}`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error("No reader available");
      const decoder = new TextDecoder();
      let buffer = "";

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const data = line.slice(6);
          if (data === "[DONE]") {
            onComplete();
            return;
          }
          try {
            onEvent(JSON.parse(data) as SSEEvent);
          } catch {
            /* 忽略解析错误 */
          }
        }
      }
      onComplete();
    })
    .catch((error: Error) => {
      if (error.name !== "AbortError") onError(error);
    });

  return () => controller.abort();
}
