import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { App, Button, Card, Flex, Space, Table, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Star, Pencil, Trash2, Plug } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";

export const Route = createFileRoute("/_auth/models/")({
  component: ModelsPage,
});

/** LLM Provider（模型服务商）读取模型。字段命名与 agent-server 保持一致（部分为 snake_case）。 */
interface LlmProvider {
  id: string;
  name: string;
  protocol: string;
  status: string;
  isDefault: boolean;
  api_key_masked?: string;
  base_url?: string | null;
  modelCount?: number;
}

interface ProviderListResponse {
  items: LlmProvider[];
}

function ModelsPage() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<LlmProvider[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<ProviderListResponse>("/admin/llm/providers");
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

  const setDefault = async (record: LlmProvider) => {
    try {
      await agentApi.post(`/admin/llm/providers/${record.id}/set-default`);
      message.success(t("models.setDefaultOk"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const testConnection = async (record: LlmProvider) => {
    try {
      await agentApi.post(`/admin/llm/providers/${record.id}/test`);
      message.success(t("models.testOk"));
    } catch (err) {
      message.error((err as Error).message || t("models.testFailed"));
    }
  };

  const remove = (record: LlmProvider) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: record.name,
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        await agentApi.post("/admin/llm/providers/delete", { providerId: record.id });
        message.success(t("common.deleted"));
        await load();
      },
    });
  };

  const columns = useMemo(
    () => [
      {
        title: t("models.colName"),
        key: "name",
        render: (_: unknown, r: LlmProvider) => (
          <Space>
            <Typography.Text strong>{r.name}</Typography.Text>
            {r.isDefault ? <Tag color="green">{t("models.defaultTag")}</Tag> : null}
          </Space>
        ),
      },
      {
        title: t("models.colProtocol"),
        dataIndex: "protocol",
        render: (v: string) => <Tag>{v}</Tag>,
      },
      {
        title: t("models.colStatus"),
        dataIndex: "status",
        render: (v: string) => <Tag color={v === "active" ? "blue" : "default"}>{v}</Tag>,
      },
      { title: t("models.colBaseUrl"), dataIndex: "base_url", render: (v: string) => v || "--" },
      {
        title: t("models.colApiKey"),
        dataIndex: "api_key_masked",
        render: (v: string) => v || "--",
      },
      {
        title: t("models.colModelCount"),
        dataIndex: "modelCount",
        render: (v: number) => <Tag>{v ?? 0}</Tag>,
      },
      {
        title: t("common.actions"),
        key: "actions",
        render: (_: unknown, r: LlmProvider) => (
          <Space>
            <Button
              type="link"
              size="small"
              icon={<Star size={14} />}
              disabled={r.isDefault || r.status !== "active"}
              onClick={() => void setDefault(r)}
            >
              {t("models.setDefault")}
            </Button>
            <Button
              type="link"
              size="small"
              icon={<Plug size={14} />}
              onClick={() => void testConnection(r)}
            >
              {t("models.test")}
            </Button>
            <Button
              type="link"
              size="small"
              icon={<Pencil size={14} />}
              onClick={() =>
                void navigate({ to: "/models/$providerId", params: { providerId: r.id } })
              }
            >
              {t("common.edit")}
            </Button>
            <Button
              type="link"
              size="small"
              danger
              icon={<Trash2 size={14} />}
              disabled={r.isDefault}
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
            {t("menu.models")}
          </Typography.Title>
          <Button
            type="primary"
            icon={<Plus size={16} />}
            onClick={() => void navigate({ to: "/models/config" })}
          >
            {t("models.newProvider")}
          </Button>
        </Flex>
        <Table<LlmProvider>
          rowKey="id"
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
