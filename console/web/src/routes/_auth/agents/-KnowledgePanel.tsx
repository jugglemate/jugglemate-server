import {
  Alert,
  App,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Radio,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
} from "antd";
import type { UploadFile } from "antd/es/upload/interface";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  File as FileIcon,
  Link as LinkIcon,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Upload as UploadIcon,
} from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { KnowledgeEditDrawer, type KnowledgeRecord } from "./-KnowledgeEditDrawer";

const { Text } = Typography;

const STATUS_COLORS: Record<string, string> = {
  draft: "default",
  pending: "blue",
  processing: "orange",
  ready: "green",
  failed: "red",
  archived: "default",
};
const STATUS_LABELS: Record<string, string> = {
  draft: "草稿",
  pending: "待处理",
  processing: "处理中",
  ready: "就绪",
  failed: "失败",
  archived: "已归档",
};
const TYPE_OPTIONS = [
  { value: "offline_document", label: "离线文档" },
  { value: "web_crawler", label: "网页爬虫" },
  { value: "api_sync", label: "API同步" },
];

interface KnowledgeItem extends KnowledgeRecord {
  status?: string;
}
interface CreateFormValues {
  name: string;
  type: string;
  sourceType: "url" | "file";
  sourceUrl?: string;
}

export function KnowledgePanel({ agentId }: { agentId?: string }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<KnowledgeItem[]>([]);
  const [mountedIds, setMountedIds] = useState<string[]>([]);
  const [editing, setEditing] = useState<KnowledgeRecord | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [sourceType, setSourceType] = useState<"url" | "file">("url");
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [form] = Form.useForm<CreateFormValues>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await agentApi.get<{ items: KnowledgeItem[] }>("/knowledge", {
        params: { page: 1, pageSize: 100, agentId },
      });
      if (agentId) {
        const detail = await agentApi.get<{ knowledge?: string[] }>(`/agents/${agentId}`);
        setMountedIds(detail.knowledge ?? []);
      } else {
        setMountedIds([]);
      }
      setData(res.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [agentId, message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async (id: string) => {
    try {
      if (agentId && mountedIds.includes(id))
        await agentApi.delete(`/agents/${agentId}/knowledge/${id}`);
      await agentApi.post("/knowledge/delete", { knowledgeId: id });
      message.success(t("common.deleted"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const mount = async (id: string) => {
    if (!agentId) return;
    try {
      await agentApi.post(`/agents/${agentId}/knowledge`, { capabilityId: id });
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const unmount = async (id: string) => {
    if (!agentId) return;
    try {
      await agentApi.delete(`/agents/${agentId}/knowledge/${id}`);
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const revectorize = async (id: string) => {
    try {
      await agentApi.post(`/knowledge/${id}/revectorize`);
      message.success(t("agents.knowledge.vectorizeStarted"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const beforeUpload = (file: File) => {
    const ok = [".pdf", ".md", ".markdown", ".html", ".htm", ".txt"].some((e) =>
      file.name.toLowerCase().endsWith(e),
    );
    if (!ok) {
      message.error(t("agents.knowledge.fileTypeError"));
      return false;
    }
    if (file.size > 10 * 1024 * 1024) {
      message.error(t("agents.knowledge.fileSizeError"));
      return false;
    }
    return false;
  };

  const submitCreate = async (values: CreateFormValues) => {
    setCreateLoading(true);
    try {
      const created = await agentApi.post<{ id: string }>("/knowledge", {
        name: values.name,
        type: values.type,
        sourceType: values.sourceType,
        sourceUrl: values.sourceType === "url" ? values.sourceUrl : undefined,
        triggerVectorize: values.sourceType === "url",
        agentId,
      });
      if (values.sourceType === "file" && fileList[0]?.originFileObj) {
        const fd = new FormData();
        fd.append("file", fileList[0].originFileObj as File);
        await agentApi.post(`/knowledge/${created.id}/upload`, fd);
      }
      message.success(t("common.created"));
      form.resetFields();
      setSourceType("url");
      setFileList([]);
      setCreateOpen(false);
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setCreateLoading(false);
    }
  };

  const columns = [
    {
      title: t("agents.knowledge.name"),
      dataIndex: "name",
      width: 160,
      ellipsis: true,
      render: (v: string) => <Text strong>{v}</Text>,
    },
    {
      title: t("models.colStatus"),
      dataIndex: "status",
      width: 120,
      render: (v: string) => (
        <Tag color={STATUS_COLORS[v] ?? "default"}>{STATUS_LABELS[v] ?? v}</Tag>
      ),
    },
    {
      title: t("agents.knowledge.size"),
      key: "size",
      width: 110,
      render: (_: unknown, r: KnowledgeItem) => (
        <Space direction="vertical" size={0}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {r.stats?.documentCount ?? 0} {t("agents.knowledge.docs")}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {r.stats?.vectorCount ?? 0} {t("agents.knowledge.vectors")}
          </Text>
        </Space>
      ),
    },
    {
      title: t("agents.knowledge.createdAt"),
      dataIndex: "createdAt",
      width: 160,
      render: (v: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {v ? new Date(v).toLocaleString() : "--"}
        </Text>
      ),
    },
    {
      title: t("common.actions"),
      key: "actions",
      render: (_: unknown, r: KnowledgeItem) => {
        const isMounted = mountedIds.includes(r.id);
        return (
          <Space>
            <Button
              type="link"
              size="small"
              icon={<Pencil size={14} />}
              onClick={() => {
                setEditing(r);
                setDrawerOpen(true);
              }}
            >
              {t("common.edit")}
            </Button>
            {r.status === "failed" ? (
              <Button
                type="link"
                size="small"
                icon={<RefreshCw size={14} />}
                onClick={() => void revectorize(r.id)}
              >
                {t("agents.knowledge.revectorize")}
              </Button>
            ) : null}
            {agentId ? (
              isMounted ? (
                <Button type="link" size="small" onClick={() => void unmount(r.id)}>
                  {t("agents.knowledge.unmount")}
                </Button>
              ) : (
                <Button type="link" size="small" onClick={() => void mount(r.id)}>
                  {t("agents.knowledge.mount")}
                </Button>
              )
            ) : null}
            <Popconfirm
              title={t("agents.knowledge.deleteConfirm")}
              onConfirm={() => void remove(r.id)}
              okText={t("common.delete")}
              cancelText={t("common.cancel")}
              okButtonProps={{ danger: true }}
            >
              <Button type="link" size="small" danger icon={<Trash2 size={14} />}>
                {t("common.delete")}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <>
      <Card
        style={{ borderRadius: 16 }}
        title={
          <Space>
            <Text strong style={{ fontSize: 16 }}>
              {agentId ? t("agents.knowledge.mountedTitle") : t("menu.tools")}
            </Text>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t("agents.knowledge.total", { count: data.length })}
            </Text>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<RefreshCw size={14} />} onClick={() => void load()}>
              {t("agents.knowledge.refresh")}
            </Button>
            <Button type="primary" icon={<Plus size={16} />} onClick={() => setCreateOpen(true)}>
              {t("agents.knowledge.new")}
            </Button>
          </Space>
        }
      >
        {agentId && data.length === 0 && !loading ? (
          <Empty description={t("agents.knowledge.noneMounted")} style={{ padding: 32 }} />
        ) : (
          <Table<KnowledgeItem>
            rowKey="id"
            columns={columns}
            dataSource={data}
            loading={loading}
            pagination={false}
            size="small"
          />
        )}
      </Card>

      <KnowledgeEditDrawer
        open={drawerOpen}
        knowledge={editing}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(null);
        }}
        onSuccess={() => {
          setDrawerOpen(false);
          setEditing(null);
          void load();
        }}
      />

      <Modal
        open={createOpen}
        title={t("agents.knowledge.new")}
        width={560}
        onCancel={() => setCreateOpen(false)}
        footer={null}
        destroyOnClose
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Alert type="info" showIcon message={t("agents.knowledge.createHint")} />
          <Form<CreateFormValues>
            form={form}
            layout="vertical"
            onFinish={submitCreate}
            initialValues={{ type: "offline_document", sourceType: "url" }}
          >
            <Form.Item label={t("agents.knowledge.name")} name="name" rules={[{ required: true }]}>
              <Input placeholder={t("agents.knowledge.namePlaceholder")} maxLength={200} />
            </Form.Item>
            <Form.Item label={t("agents.knowledge.type")} name="type" rules={[{ required: true }]}>
              <Select options={TYPE_OPTIONS} />
            </Form.Item>
            <Form.Item
              label={t("agents.knowledge.sourceType")}
              name="sourceType"
              rules={[{ required: true }]}
            >
              <Radio.Group
                onChange={(e) => {
                  setSourceType(e.target.value);
                  form.setFieldValue("sourceUrl", undefined);
                  setFileList([]);
                }}
                value={sourceType}
              >
                <Radio.Button value="url">
                  <Space size={4}>
                    <LinkIcon size={13} /> URL
                  </Space>
                </Radio.Button>
                <Radio.Button value="file">
                  <Space size={4}>
                    <FileIcon size={13} /> {t("agents.knowledge.fileUpload")}
                  </Space>
                </Radio.Button>
              </Radio.Group>
            </Form.Item>
            {sourceType === "url" ? (
              <Form.Item
                label={t("agents.knowledge.docUrl")}
                name="sourceUrl"
                rules={[{ required: true }, { type: "url" }]}
              >
                <Input placeholder="https://example.com/document.pdf" />
              </Form.Item>
            ) : (
              <Form.Item label={t("agents.knowledge.docFile")}>
                <Upload
                  fileList={fileList}
                  beforeUpload={beforeUpload}
                  onChange={({ fileList }) => setFileList(fileList)}
                  maxCount={1}
                  accept=".pdf,.md,.markdown,.html,.htm,.txt"
                >
                  <Button icon={<UploadIcon size={14} />}>
                    {t("agents.knowledge.chooseFile")}
                  </Button>
                </Upload>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {t("agents.knowledge.fileHint")}
                  </Text>
                </div>
              </Form.Item>
            )}
            <Form.Item style={{ marginBottom: 0, textAlign: "right" }}>
              <Space>
                <Button onClick={() => setCreateOpen(false)}>{t("common.cancel")}</Button>
                <Button type="primary" htmlType="submit" loading={createLoading}>
                  {t("agents.knowledge.create")}
                </Button>
              </Space>
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </>
  );
}
