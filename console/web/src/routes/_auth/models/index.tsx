import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { App, Button, Card, Flex, Select, Space, Table, Tag, Typography } from "antd";
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

interface LlmModel {
  id: string;
  modelId: string;
  displayName: string;
  providerName: string;
  status: string;
}

interface ModelListResponse {
  items: LlmModel[];
}

interface DefaultModels {
  translation_model: string | null;
}

function ModelsPage() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<LlmProvider[]>([]);
  const [translationLoading, setTranslationLoading] = useState(false);
  const [translationSaving, setTranslationSaving] = useState(false);
  const [models, setModels] = useState<LlmModel[]>([]);
  const [translationModel, setTranslationModel] = useState<string | null>(null);
  const [selectedTranslationModel, setSelectedTranslationModel] = useState<string>();

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

  const loadTranslationDefaults = useCallback(async () => {
    setTranslationLoading(true);
    try {
      const [defaults, modelList] = await Promise.all([
        agentApi.get<DefaultModels>("/admin/llm/defaults"),
        agentApi.get<ModelListResponse>("/admin/llm/models", {
          params: { status: "active" },
        }),
      ]);
      const current = defaults?.translation_model ?? null;
      setTranslationModel(current);
      setSelectedTranslationModel(current ?? undefined);
      setModels(modelList?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("models.translation.loadFailed"));
      setModels([]);
    } finally {
      setTranslationLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
    void loadTranslationDefaults();
  }, [load, loadTranslationDefaults]);

  const saveTranslationModel = async () => {
    if (!selectedTranslationModel) return;
    setTranslationSaving(true);
    try {
      const defaults = await agentApi.put<DefaultModels>("/admin/llm/defaults", {
        translation_model: selectedTranslationModel,
      });
      const current = defaults?.translation_model ?? selectedTranslationModel;
      setTranslationModel(current);
      setSelectedTranslationModel(current);
      message.success(t("models.translation.saveSuccess"));
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setTranslationSaving(false);
    }
  };

  const clearTranslationModel = () => {
    modal.confirm({
      title: t("models.translation.clearConfirmTitle"),
      content: t("models.translation.clearConfirmDesc"),
      okText: t("models.translation.clear"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        setTranslationSaving(true);
        try {
          await agentApi.put<DefaultModels>("/admin/llm/defaults", {
            translation_model: null,
          });
          setTranslationModel(null);
          setSelectedTranslationModel(undefined);
          message.success(t("models.translation.clearSuccess"));
        } catch (err) {
          message.error((err as Error).message || t("agentAdmin.saveFailed"));
        } finally {
          setTranslationSaving(false);
        }
      },
    });
  };

  const modelOptions = models.map((model) => ({
    value: model.modelId,
    label: `${model.displayName} (${model.modelId}) - ${model.providerName}`,
  }));

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
    <Flex vertical gap={16}>
      <Card title={t("models.translation.title")}>
        <Flex vertical gap={16}>
          <Typography.Text type="secondary">{t("models.translation.description")}</Typography.Text>
          <Flex gap={12} align="center" wrap>
            <Select
              showSearch
              optionFilterProp="label"
              loading={translationLoading}
              disabled={translationLoading}
              value={selectedTranslationModel}
              options={modelOptions}
              placeholder={t("models.translation.placeholder")}
              onChange={setSelectedTranslationModel}
              style={{ width: "min(100%, 560px)" }}
              notFoundContent={t("models.translation.noActiveModels")}
            />
            <Button
              type="primary"
              loading={translationSaving}
              disabled={
                translationLoading ||
                !selectedTranslationModel ||
                selectedTranslationModel === translationModel
              }
              onClick={() => void saveTranslationModel()}
            >
              {t("models.translation.save")}
            </Button>
            <Button
              danger
              disabled={translationLoading || translationSaving || !translationModel}
              onClick={clearTranslationModel}
            >
              {t("models.translation.clear")}
            </Button>
          </Flex>
          <Typography.Text>
            {t("models.translation.current")}: {translationModel ?? t("models.translation.notSet")}
          </Typography.Text>
        </Flex>
      </Card>

      <Card>
        <Flex vertical gap={16}>
          <Flex justify="space-between" align="center">
            <Typography.Title level={5} style={{ margin: 0 }}>
              {t("models.providersTitle")}
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
    </Flex>
  );
}
