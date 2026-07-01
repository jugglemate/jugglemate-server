import { createFileRoute } from "@tanstack/react-router";
import {
  App,
  Button,
  Card,
  Flex,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
} from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Power, Trash2, CheckCircle2 } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";

export const Route = createFileRoute("/_auth/tools/")({
  component: ToolsPage,
});

interface PagedResponse<T> {
  items: T[];
  total?: number;
}

interface ToolProvider {
  id: string;
  name: string;
  displayName?: string;
  providerType: string;
  authType: string;
  status: string;
  visibility?: string;
}

interface ToolDefinition {
  id: string;
  providerId: string;
  name: string;
  toolType: string;
  riskLevel?: string;
  status: string;
}

interface ToolCredential {
  id: string;
  providerId: string;
  name: string;
  status: string;
}

const STATUS_COLOR = (s: string) => (s === "active" ? "blue" : s === "revoked" ? "red" : "default");

function parseJsonField(
  text: string | undefined,
  fallback: Record<string, unknown> = {},
): Record<string, unknown> {
  if (!text || !text.trim()) return fallback;
  return JSON.parse(text) as Record<string, unknown>;
}

function ProvidersTab() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ToolProvider[]>([]);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<PagedResponse<ToolProvider>>("/tool-providers", {
        params: { page: 1, page_size: 100 },
      });
      setItems(data?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const transition = async (r: ToolProvider, targetStatus: string) => {
    try {
      await agentApi.post(`/tool-providers/${r.id}/transition`, { targetStatus });
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const remove = (r: ToolProvider) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: r.name,
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        await agentApi.delete(`/tool-providers/${r.id}`);
        message.success(t("common.deleted"));
        await load();
      },
    });
  };

  const submit = async (values: Record<string, string>) => {
    setSubmitting(true);
    try {
      await agentApi.post("/tool-providers", {
        name: values.name,
        providerType: values.providerType,
        displayName: values.displayName,
        authType: values.authType,
        visibility: values.visibility,
        connectionConfig: parseJsonField(values.connectionConfig),
        authSchema: parseJsonField(values.authSchema),
      });
      message.success(t("common.created"));
      setOpen(false);
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setSubmitting(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t("tools.colName"),
        dataIndex: "name",
        render: (v: string, r: ToolProvider) => r.displayName || v,
      },
      {
        title: t("tools.colType"),
        dataIndex: "providerType",
        render: (v: string) => <Tag>{v}</Tag>,
      },
      { title: t("tools.colAuth"), dataIndex: "authType", render: (v: string) => <Tag>{v}</Tag> },
      {
        title: t("tools.colStatus"),
        dataIndex: "status",
        render: (v: string) => <Tag color={STATUS_COLOR(v)}>{v}</Tag>,
      },
      {
        title: t("tools.colVisibility"),
        dataIndex: "visibility",
        render: (v: string) => v || "--",
      },
      {
        title: t("common.actions"),
        key: "actions",
        render: (_: unknown, r: ToolProvider) => (
          <Space>
            {r.status === "active" ? (
              <Button
                type="link"
                size="small"
                icon={<Power size={14} />}
                onClick={() => void transition(r, "disabled")}
              >
                {t("tools.disable")}
              </Button>
            ) : (
              <Button
                type="link"
                size="small"
                icon={<Power size={14} />}
                onClick={() => void transition(r, "active")}
              >
                {t("tools.enable")}
              </Button>
            )}
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
    [t],
  );

  return (
    <Flex vertical gap={12}>
      <Flex justify="flex-end">
        <Button
          type="primary"
          icon={<Plus size={16} />}
          onClick={() => {
            form.resetFields();
            form.setFieldsValue({
              providerType: "custom",
              authType: "api_key",
              visibility: "private",
            });
            setOpen(true);
          }}
        >
          {t("tools.newProvider")}
        </Button>
      </Flex>
      <Table<ToolProvider>
        rowKey="id"
        size="small"
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={{ pageSize: 20, hideOnSinglePage: true }}
      />
      <Modal
        open={open}
        title={t("tools.newProvider")}
        okText={t("common.ok")}
        cancelText={t("common.cancel")}
        confirmLoading={submitting}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        destroyOnClose
        width={560}
      >
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item name="name" label={t("tools.colName")} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="displayName" label={t("tools.displayName")}>
            <Input />
          </Form.Item>
          <Form.Item name="providerType" label={t("tools.colType")} rules={[{ required: true }]}>
            <Select options={["plugin", "custom"].map((v) => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item name="authType" label={t("tools.colAuth")} rules={[{ required: true }]}>
            <Select
              options={["none", "api_key", "oauth", "basic"].map((v) => ({ label: v, value: v }))}
            />
          </Form.Item>
          <Form.Item name="visibility" label={t("tools.colVisibility")}>
            <Select options={["private", "system"].map((v) => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item
            name="connectionConfig"
            label={t("tools.connectionConfig")}
            extra={t("tools.jsonHint")}
          >
            <Input.TextArea rows={3} placeholder='{"base_url":"https://..."}' />
          </Form.Item>
          <Form.Item name="authSchema" label={t("tools.authSchema")} extra={t("tools.jsonHint")}>
            <Input.TextArea rows={3} placeholder="{}" />
          </Form.Item>
        </Form>
      </Modal>
    </Flex>
  );
}

function DefinitionsTab() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ToolDefinition[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<PagedResponse<ToolDefinition>>("/tool-definitions", {
        params: { page: 1, page_size: 100 },
      });
      setItems(data?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const activate = async (r: ToolDefinition) => {
    try {
      await agentApi.post(`/tool-definitions/${r.id}/activate`);
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const remove = (r: ToolDefinition) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: r.name,
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        await agentApi.delete(`/tool-definitions/${r.id}`);
        message.success(t("common.deleted"));
        await load();
      },
    });
  };

  const columns = useMemo(
    () => [
      { title: t("tools.colName"), dataIndex: "name" },
      { title: t("tools.colType"), dataIndex: "toolType", render: (v: string) => <Tag>{v}</Tag> },
      {
        title: t("tools.colRisk"),
        dataIndex: "riskLevel",
        render: (v: string) => (v ? <Tag>{v}</Tag> : "--"),
      },
      {
        title: t("tools.colStatus"),
        dataIndex: "status",
        render: (v: string) => <Tag color={STATUS_COLOR(v)}>{v}</Tag>,
      },
      {
        title: t("common.actions"),
        key: "actions",
        render: (_: unknown, r: ToolDefinition) => (
          <Space>
            <Button
              type="link"
              size="small"
              icon={<CheckCircle2 size={14} />}
              disabled={r.status === "active"}
              onClick={() => void activate(r)}
            >
              {t("tools.activate")}
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
    [t],
  );

  return (
    <Table<ToolDefinition>
      rowKey="id"
      size="small"
      columns={columns}
      dataSource={items}
      loading={loading}
      pagination={{ pageSize: 20, hideOnSinglePage: true }}
    />
  );
}

function CredentialsTab() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ToolCredential[]>([]);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<PagedResponse<ToolCredential>>("/tool-credentials", {
        params: { page: 1, page_size: 100 },
      });
      setItems(data?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const revoke = async (r: ToolCredential) => {
    try {
      await agentApi.post(`/tool-credentials/${r.id}/revoke`);
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const submit = async (values: Record<string, string>) => {
    setSubmitting(true);
    try {
      await agentApi.post("/tool-credentials", {
        providerId: values.providerId,
        name: values.name,
        credentialPayload: parseJsonField(values.credentialPayload),
      });
      message.success(t("common.created"));
      setOpen(false);
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setSubmitting(false);
    }
  };

  const columns = useMemo(
    () => [
      { title: t("tools.colName"), dataIndex: "name" },
      { title: t("tools.colProvider"), dataIndex: "providerId" },
      {
        title: t("tools.colStatus"),
        dataIndex: "status",
        render: (v: string) => <Tag color={STATUS_COLOR(v)}>{v}</Tag>,
      },
      {
        title: t("common.actions"),
        key: "actions",
        render: (_: unknown, r: ToolCredential) => (
          <Button
            type="link"
            size="small"
            danger
            disabled={r.status === "revoked"}
            onClick={() => void revoke(r)}
          >
            {t("tools.revoke")}
          </Button>
        ),
      },
    ],
    [t],
  );

  return (
    <Flex vertical gap={12}>
      <Flex justify="flex-end">
        <Button
          type="primary"
          icon={<Plus size={16} />}
          onClick={() => {
            form.resetFields();
            setOpen(true);
          }}
        >
          {t("tools.newCredential")}
        </Button>
      </Flex>
      <Table<ToolCredential>
        rowKey="id"
        size="small"
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={{ pageSize: 20, hideOnSinglePage: true }}
      />
      <Modal
        open={open}
        title={t("tools.newCredential")}
        okText={t("common.ok")}
        cancelText={t("common.cancel")}
        confirmLoading={submitting}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item name="providerId" label={t("tools.colProvider")} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="name" label={t("tools.colName")} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="credentialPayload"
            label={t("tools.credentialPayload")}
            extra={t("tools.jsonHint")}
          >
            <Input.TextArea rows={3} placeholder='{"api_key":"..."}' />
          </Form.Item>
        </Form>
      </Modal>
    </Flex>
  );
}

function ToolsPage() {
  const { t } = useTranslation();
  return (
    <Card>
      <Flex vertical gap={16}>
        <Typography.Title level={5} style={{ margin: 0 }}>
          {t("menu.tools")}
        </Typography.Title>
        <Tabs
          items={[
            { key: "providers", label: t("tools.tabProviders"), children: <ProvidersTab /> },
            { key: "definitions", label: t("tools.tabDefinitions"), children: <DefinitionsTab /> },
            { key: "credentials", label: t("tools.tabCredentials"), children: <CredentialsTab /> },
          ]}
        />
      </Flex>
    </Card>
  );
}
