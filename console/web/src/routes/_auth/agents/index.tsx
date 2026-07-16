import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { App, Button, Flex, Input, Select, Space, Spin, Table, Typography, theme } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bot, Pause, Pencil, Play, Plus, Trash2 } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";

const { Title, Text } = Typography;

export const Route = createFileRoute("/_auth/agents/")({
  component: AgentsPage,
});

interface AgentListItem {
  agentId: string;
  agentName: string;
  primaryModel?: string;
  status: string;
  knowledgeCount?: number;
  toolCount?: number;
}

interface PagedResponse<T> {
  items: T[];
  total: number;
}

/** 状态色映射：启用=绿、暂停=橙、草稿=灰，用于状态点。 */
const STATUS_TONE: Record<string, string> = {
  active: BRAND.success,
  paused: BRAND.warning,
  draft: BRAND.outline,
};

function AgentsPage() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<AgentListItem[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [query, setQuery] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<PagedResponse<AgentListItem>>("/agents", {
        params: { page: 1, page_size: 100 },
      });
      setItems(data?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const lifecycle = async (record: AgentListItem, action: "activate" | "pause") => {
    try {
      await agentApi.post(`/agents/${record.agentId}/${action}`);
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const remove = (record: AgentListItem) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: record.agentName,
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        await agentApi.post("/agents/delete", { agentId: record.agentId });
        message.success(t("common.deleted"));
        await load();
      },
    });
  };

  const openEdit = (r: AgentListItem) =>
    void navigate({ to: "/agents/$agentId", params: { agentId: r.agentId } });

  /* TIPS: 状态下拉与名称搜索均为当前已加载列表的客户端过滤（默认 page_size=100）。 */
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((i) => {
      if (statusFilter && i.status !== statusFilter) return false;
      if (q && !i.agentName.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [items, statusFilter, query]);

  const listColumns: ColumnsType<AgentListItem> = [
    {
      title: t("agents.colName"),
      dataIndex: "agentName",
      render: (v: string, r) => {
        const tone = STATUS_TONE[r.status] ?? BRAND.outline;
        return (
          <Flex align="center" gap={token.marginSM}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 32,
                height: 32,
                borderRadius: token.borderRadiusLG,
                background: BRAND.primarySoft,
                color: BRAND.primary,
                flex: "0 0 auto",
              }}
            >
              <Bot size={16} />
            </Flex>
            <Flex vertical gap={0} style={{ minWidth: 0 }}>
              <Text strong ellipsis>
                {v}
              </Text>
              <Flex align="center" gap={5}>
                <span
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    background: tone,
                    display: "inline-block",
                  }}
                />
                <span style={{ fontSize: 11, textTransform: "uppercase", color: BRAND.outline }}>
                  {r.status}
                </span>
              </Flex>
            </Flex>
          </Flex>
        );
      },
    },
    { title: t("agents.colModel"), dataIndex: "primaryModel", render: (v: string) => v || "--" },
    {
      title: t("agents.knowledgeCount"),
      dataIndex: "knowledgeCount",
      width: 110,
      render: (v: number) => v ?? 0,
    },
    {
      title: t("agents.toolCount"),
      dataIndex: "toolCount",
      width: 90,
      render: (v: number) => v ?? 0,
    },
    {
      title: t("common.actions"),
      key: "actions",
      width: 120,
      align: "right",
      render: (_, r) => (
        <Space size={2}>
          <Button
            type="text"
            size="small"
            icon={<Pencil size={16} />}
            aria-label={t("common.edit")}
            onClick={() => openEdit(r)}
          />
          {r.status === "active" ? (
            <Button
              type="text"
              size="small"
              icon={<Pause size={16} />}
              aria-label={t("agents.pause")}
              onClick={() => void lifecycle(r, "pause")}
            />
          ) : (
            <Button
              type="text"
              size="small"
              icon={<Play size={16} />}
              aria-label={t("agents.activate")}
              onClick={() => void lifecycle(r, "activate")}
            />
          )}
          <Button
            type="text"
            size="small"
            danger
            icon={<Trash2 size={16} />}
            aria-label={t("common.delete")}
            onClick={() => remove(r)}
          />
        </Space>
      ),
    },
  ];

  return (
    <Flex vertical gap={token.marginLG}>
      <Flex justify="space-between" align="flex-end" gap={token.margin} wrap="wrap">
        <Flex vertical gap={token.marginXXS} style={{ minWidth: 0 }}>
          <Title level={3} style={{ margin: 0 }}>
            {t("agents.listTitle")}
          </Title>
          <Text type="secondary">{t("agents.listSubtitle")}</Text>
        </Flex>
        <Flex wrap gap={token.marginSM} align="center">
          <Input.Search
            allowClear
            placeholder={t("agents.searchPlaceholder")}
            style={{ width: 260 }}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Select
            value={statusFilter || undefined}
            onChange={(v) => setStatusFilter(v ?? "")}
            allowClear
            placeholder={t("agents.statusAll")}
            style={{ width: 160 }}
            options={[
              { value: "active", label: t("agents.colStatus") + ": active" },
              { value: "paused", label: t("agents.colStatus") + ": paused" },
              { value: "draft", label: t("agents.colStatus") + ": draft" },
            ]}
          />
          <Button
            type="primary"
            icon={<Plus size={18} />}
            onClick={() => void navigate({ to: "/agents/new" })}
          >
            {t("agents.addAgent")}
          </Button>
        </Flex>
      </Flex>

      {loading ? (
        <Flex justify="center" style={{ padding: 80 }}>
          <Spin />
        </Flex>
      ) : (
        <div
          style={{
            background: BRAND.cardBg,
            border: `1px solid ${BRAND.borderLow}`,
            borderRadius: token.borderRadiusLG,
            overflow: "hidden",
          }}
        >
          <Table<AgentListItem>
            rowKey="agentId"
            columns={listColumns}
            dataSource={filtered}
            pagination={{ pageSize: 12, hideOnSinglePage: true }}
            scroll={{ x: "max-content" }}
            locale={{ emptyText: t("agents.empty") }}
          />
        </div>
      )}
    </Flex>
  );
}
