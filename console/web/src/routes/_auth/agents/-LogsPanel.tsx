import { App, Card, Empty, Input, List, Pagination, Space, Spin, Tag, Typography } from "antd";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Clock, MessageSquare, User } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";

const { Text } = Typography;

export interface ConversationDetail {
  id: string;
  agentId: string;
  userId: string;
  messages?: { id: string; role: string; content: string }[];
}
// agent-server 会话接口返回 snake_case，这里按原始键读取。
interface ConversationItem {
  id: string;
  user_id: string;
  status: string;
  message_count: number;
  total_tokens: number;
  last_active_at: string;
}
interface RawDetail {
  id: string;
  agent_id: string;
  user_id: string;
  messages?: { id: string; role: string; content: string }[];
}
interface Paged {
  items: ConversationItem[];
  pagination: { page: number; total: number };
}

export function LogsPanel({
  agentId,
  onSelect,
}: {
  agentId: string;
  onSelect: (conv: ConversationDetail | null) => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [items, setItems] = useState<ConversationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [userIdFilter, setUserIdFilter] = useState<string>("");
  const [appliedUserId, setAppliedUserId] = useState<string>("");

  // 防抖：输入停止 400ms 后才应用过滤，避免每次按键都整表重拉。
  useEffect(() => {
    const timer = setTimeout(() => setAppliedUserId(userIdFilter.trim()), 400);
    return () => clearTimeout(timer);
  }, [userIdFilter]);

  const load = useCallback(
    async (p: number) => {
      setLoading(true);
      try {
        const res = await agentApi.get<Paged>(`/owner/agents/${agentId}/conversations`, {
          params: { page: p, page_size: 10, user_id: appliedUserId || undefined },
        });
        setItems(res?.items ?? []);
        setTotal(res?.pagination?.total ?? 0);
        setPage(res?.pagination?.page ?? p);
      } catch (err) {
        message.error((err as Error).message || t("agentAdmin.loadFailed"));
      } finally {
        setLoading(false);
      }
    },
    [agentId, appliedUserId, message, t],
  );

  useEffect(() => {
    void load(1);
  }, [load]);

  const openDetail = async (c: ConversationItem) => {
    setSelectedId(c.id);
    try {
      const raw = await agentApi.get<RawDetail>(`/owner/conversations/${c.id}`);
      onSelect({ id: raw.id, agentId: raw.agent_id, userId: raw.user_id, messages: raw.messages });
    } catch (err) {
      setSelectedId(null);
      onSelect(null);
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
    }
  };

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder={t("agents.logs.userId")}
          allowClear
          style={{ width: 200 }}
          value={userIdFilter}
          onChange={(e) => setUserIdFilter(e.target.value)}
        />
      </Space>
      <Spin spinning={loading}>
        {items.length === 0 && !loading ? (
          <Empty description={t("agents.logs.empty")} style={{ padding: 40 }} />
        ) : (
          <List
            dataSource={items}
            renderItem={(item) => (
              <Card
                style={{
                  marginBottom: 12,
                  borderRadius: 12,
                  cursor: "pointer",
                  border: item.id === selectedId ? `1px solid ${BRAND.primary}` : undefined,
                }}
                hoverable
                onClick={() => void openDetail(item)}
              >
                <Space direction="vertical" size={4} style={{ width: "100%" }}>
                  <Space style={{ width: "100%", justifyContent: "space-between" }}>
                    <Space>
                      <User size={14} color={BRAND.outline} />
                      <Text type="secondary">{item.user_id?.slice(0, 8)}...</Text>
                    </Space>
                    <Tag color={item.status === "active" ? "green" : "default"}>
                      {item.status === "active" ? t("agents.logs.active") : t("agents.logs.closed")}
                    </Tag>
                  </Space>
                  <Space style={{ width: "100%", justifyContent: "space-between" }}>
                    <Space>
                      <MessageSquare size={14} color={BRAND.primary} />
                      <Text>{t("agents.logs.msgCount", { count: item.message_count })}</Text>
                    </Space>
                    <Text type="secondary">{item.total_tokens} tokens</Text>
                    <Space>
                      <Clock size={14} color={BRAND.outline} />
                      <Text type="secondary">
                        {item.last_active_at
                          ? new Date(item.last_active_at).toLocaleString()
                          : "--"}
                      </Text>
                    </Space>
                  </Space>
                </Space>
              </Card>
            )}
          />
        )}
      </Spin>
      {total > 0 ? (
        <Pagination
          current={page}
          pageSize={10}
          total={total}
          showSizeChanger={false}
          onChange={(p) => void load(p)}
          style={{ marginTop: 16, textAlign: "center" }}
        />
      ) : null}
    </div>
  );
}
