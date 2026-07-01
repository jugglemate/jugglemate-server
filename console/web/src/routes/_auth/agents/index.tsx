import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { App, Button, Card, Flex, Space, Table, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Pause, Play, Pencil, Trash2 } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";

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

const STATUS_COLORS: Record<string, string> = {
  active: "green",
  paused: "orange",
  draft: "default",
};

function AgentsPage() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<AgentListItem[]>([]);

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

  const columns = useMemo(
    () => [
      {
        title: t("agents.colName"),
        dataIndex: "agentName",
        render: (v: string, r: AgentListItem) => (
          <Button
            type="link"
            style={{ padding: 0 }}
            onClick={() =>
              void navigate({ to: "/agents/$agentId", params: { agentId: r.agentId } })
            }
          >
            {v}
          </Button>
        ),
      },
      { title: t("agents.colModel"), dataIndex: "primaryModel", render: (v: string) => v || "--" },
      {
        title: t("agents.colStatus"),
        dataIndex: "status",
        render: (v: string) => <Tag color={STATUS_COLORS[v] ?? "default"}>{v}</Tag>,
      },
      {
        title: t("agents.colTools"),
        dataIndex: "toolCount",
        render: (v: number) => <Tag>{v ?? 0}</Tag>,
      },
      {
        title: t("agents.colKnowledge"),
        dataIndex: "knowledgeCount",
        render: (v: number) => <Tag>{v ?? 0}</Tag>,
      },
      {
        title: t("common.actions"),
        key: "actions",
        render: (_: unknown, r: AgentListItem) => (
          <Space>
            {r.status === "active" ? (
              <Button
                type="link"
                size="small"
                icon={<Pause size={14} />}
                onClick={() => void lifecycle(r, "pause")}
              >
                {t("agents.pause")}
              </Button>
            ) : (
              <Button
                type="link"
                size="small"
                icon={<Play size={14} />}
                onClick={() => void lifecycle(r, "activate")}
              >
                {t("agents.activate")}
              </Button>
            )}
            <Button
              type="link"
              size="small"
              icon={<Pencil size={14} />}
              onClick={() =>
                void navigate({ to: "/agents/$agentId", params: { agentId: r.agentId } })
              }
            >
              {t("common.edit")}
            </Button>
            <Button
              type="link"
              size="small"
              danger
              icon={<Trash2 size={14} />}
              onClick={() => remove(r)}
            >
              {t("common.delete")}
            </Button>
          </Space>
        ),
      },
    ],
    [t, navigate],
  );

  return (
    <Card>
      <Flex vertical gap={16}>
        <Flex justify="space-between" align="center">
          <Typography.Title level={5} style={{ margin: 0 }}>
            {t("menu.agents")}
          </Typography.Title>
          <Button
            type="primary"
            icon={<Plus size={16} />}
            onClick={() => void navigate({ to: "/agents/new" })}
          >
            {t("agents.newAgent")}
          </Button>
        </Flex>
        <Table<AgentListItem>
          rowKey="agentId"
          size="small"
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={{ pageSize: 20, hideOnSinglePage: true }}
        />
      </Flex>
    </Card>
  );
}
