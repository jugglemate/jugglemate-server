import { Button, Flex, Input, Space, Spin, Tag, Typography } from "antd";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bot, Send } from "lucide-react";
import { createChatSSE, type SSEEvent } from "@/utils/agentSSE";
import { BRAND } from "@/utils/brandColors";

interface ChatMsg {
  role: "user" | "assistant";
  content: string;
  streaming?: boolean;
}

export function ChatPanel({
  agentId,
  agentStatus,
  conversationId,
  targetUserId,
  initialMessages,
}: {
  agentId: string;
  agentStatus?: string | null;
  conversationId?: string | null;
  targetUserId?: string;
  initialMessages?: { role: string; content: string }[];
}) {
  const { t } = useTranslation();
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const convIdRef = useRef<string | null>(conversationId ?? null);
  const cancelRef = useRef<(() => void) | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => () => cancelRef.current?.(), []);

  // 切换所选会话（Logs 点击不同会话）时：取消进行中的流、重置会话上下文并载入其历史消息。
  useEffect(() => {
    cancelRef.current?.();
    setSending(false);
    convIdRef.current = conversationId ?? null;
    setMessages(
      (initialMessages ?? []).map((m) => ({
        role: m.role === "user" ? "user" : "assistant",
        content: m.content,
      })),
    );
  }, [conversationId, initialMessages]);
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [messages]);

  const patchAssistant = (fn: (prev: string) => string, done = false) =>
    setMessages((prev) => {
      const next = [...prev];
      for (let i = next.length - 1; i >= 0; i--) {
        if (next[i].role === "assistant") {
          next[i] = { role: "assistant", content: fn(next[i].content), streaming: !done };
          break;
        }
      }
      return next;
    });

  const send = () => {
    const text = input.trim();
    if (!text || sending) return;
    setInput("");
    setMessages((prev) => [
      ...prev,
      { role: "user", content: text },
      { role: "assistant", content: "", streaming: true },
    ]);
    setSending(true);
    cancelRef.current = createChatSSE(
      {
        agentId,
        userId: targetUserId ?? "console-tester",
        message: text,
        conversationId: convIdRef.current,
      },
      (event: SSEEvent) => {
        const p = event.payload || {};
        if (event.event === "token") {
          // 逐 token 流式追加。
          const tok = (p.token as string) ?? (p.content as string) ?? (p.delta as string) ?? "";
          if (tok) patchAssistant((prev) => prev + tok);
        } else if (event.event === "done") {
          // 最终答案与会话 ID 通过 done 事件下发（流式模式不会触发 onJsonResponse）。
          if (typeof p.conversationId === "string") convIdRef.current = p.conversationId;
          // 仅当 answer 非空时才用其定稿，避免空串清空已流式的回复。
          if (typeof p.answer === "string" && p.answer)
            patchAssistant(() => p.answer as string, true);
          setSending(false);
        }
        // "thought" 等其它事件为思考/状态提示，不计入回复正文（由 Spin 指示进行中）。
      },
      (resp) => {
        // 非流式（application/json）回退路径。
        convIdRef.current = resp.conversationId ?? convIdRef.current;
        patchAssistant(() => resp.answer, true);
        setSending(false);
      },
      (err) => {
        patchAssistant((prev) => prev || `⚠️ ${err.message}`, true);
        setSending(false);
      },
      () => {
        patchAssistant((prev) => prev, true);
        setSending(false);
      },
    );
  };

  return (
    <Flex vertical style={{ height: "100%", minHeight: 480 }}>
      <Flex
        align="center"
        justify="space-between"
        style={{ padding: "12px 16px", borderBottom: `1px solid ${BRAND.borderLow}` }}
      >
        <Space>
          <Typography.Text strong>{t("agents.chat.title")}</Typography.Text>
          <Tag color="blue">SSE Mode</Tag>
          {agentStatus ? (
            <Tag color={agentStatus === "active" ? "green" : "default"}>{agentStatus}</Tag>
          ) : null}
        </Space>
      </Flex>

      <div ref={scrollRef} style={{ flex: 1, overflow: "auto", padding: 16 }}>
        {messages.length === 0 ? (
          <Flex
            vertical
            align="center"
            justify="center"
            style={{ height: "100%", color: BRAND.outline }}
          >
            <Bot size={48} color={BRAND.primary} />
            <Typography.Text type="secondary" style={{ marginTop: 12 }}>
              {t("agents.chat.empty")}
            </Typography.Text>
          </Flex>
        ) : (
          <Flex vertical gap={12}>
            {messages.map((m, i) => (
              <Flex key={i} justify={m.role === "user" ? "flex-end" : "flex-start"}>
                <div
                  style={{
                    maxWidth: "80%",
                    padding: "8px 12px",
                    borderRadius: 12,
                    background: m.role === "user" ? BRAND.primaryFill : BRAND.subtleBg,
                    color: m.role === "user" ? BRAND.onPrimaryFill : BRAND.onSurface,
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-word",
                  }}
                >
                  {m.content}
                  {m.streaming ? <Spin size="small" style={{ marginLeft: 8 }} /> : null}
                </div>
              </Flex>
            ))}
          </Flex>
        )}
      </div>

      <div style={{ padding: 12, borderTop: `1px solid ${BRAND.borderLow}` }}>
        <Space.Compact style={{ width: "100%" }}>
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onPressEnter={send}
            placeholder={t("agents.chat.placeholder")}
            disabled={sending}
          />
          <Button type="primary" icon={<Send size={16} />} onClick={send} loading={sending}>
            {t("agents.chat.send")}
          </Button>
        </Space.Compact>
      </div>
    </Flex>
  );
}
