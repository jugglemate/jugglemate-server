import {
  Alert,
  App,
  Button,
  Empty,
  Flex,
  Form,
  Input,
  Modal,
  Popconfirm,
  Radio,
  Select,
  Space,
  Table,
  Typography,
  Upload,
  theme,
} from "antd";
import type { UploadFile } from "antd/es/upload/interface";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Eye,
  File as FileIcon,
  FileText,
  Link as LinkIcon,
  Plus,
  RefreshCw,
  Trash2,
  UploadCloud,
  Upload as UploadIcon,
} from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";
import { KnowledgeEditDrawer, type KnowledgeRecord } from "./-KnowledgeEditDrawer";

const { Text, Title } = Typography;

/** 状态点颜色映射（对齐设计稿的状态圆点 + 文案）。 */
const STATUS_TONE: Record<string, string> = {
  draft: BRAND.outline,
  pending: BRAND.primary,
  processing: BRAND.primary,
  ready: BRAND.success,
  failed: "#ba1a1a",
  archived: BRAND.outline,
};
const STATUS_LABELS: Record<string, string> = {
  draft: "草稿",
  pending: "待处理",
  processing: "处理中",
  ready: "就绪",
  failed: "失败",
  archived: "已归档",
};

/** 从文件/知识名推断展示用的文件类型标签（PDF/TXT/DOCX…）。 */
function inferFileType(name: string): string {
  const m = /\.([a-z0-9]+)$/i.exec(name.trim());
  return m ? m[1].toUpperCase() : "DOC";
}
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
  const { token } = theme.useToken();
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

  const totals = useMemo(() => {
    let docs = 0;
    let vectors = 0;
    for (const r of data) {
      docs += r.stats?.documentCount ?? 0;
      vectors += r.stats?.vectorCount ?? 0;
    }
    return { docs, vectors };
  }, [data]);

  const th = (label: string) => (
    <span
      style={{
        textTransform: "uppercase",
        letterSpacing: 0.6,
        fontSize: 12,
        fontWeight: 700,
        color: BRAND.outline,
      }}
    >
      {label}
    </span>
  );

  const columns = [
    {
      title: th(t("agents.knowledge.fileName")),
      dataIndex: "name",
      ellipsis: true,
      render: (v: string) => (
        <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
          <FileText size={18} color={BRAND.primary} style={{ flex: "0 0 auto" }} />
          <Text strong ellipsis>
            {v}
          </Text>
        </Flex>
      ),
    },
    {
      title: th(t("agents.knowledge.type")),
      key: "type",
      width: 110,
      render: (_: unknown, r: KnowledgeItem) => (
        <Text type="secondary">{inferFileType(r.name ?? "")}</Text>
      ),
    },
    {
      title: th(t("agents.knowledge.uploadDate")),
      dataIndex: "createdAt",
      width: 150,
      render: (v: string) => (
        <Text type="secondary">{v ? new Date(v).toLocaleDateString() : "--"}</Text>
      ),
    },
    {
      title: th(t("models.colStatus")),
      dataIndex: "status",
      width: 140,
      render: (v: string) => {
        const tone = STATUS_TONE[v] ?? BRAND.outline;
        return (
          <Flex align="center" gap={6}>
            <span
              style={{
                width: 7,
                height: 7,
                borderRadius: "50%",
                background: tone,
                display: "inline-block",
              }}
            />
            <span style={{ color: tone, fontWeight: 500 }}>{STATUS_LABELS[v] ?? v}</span>
          </Flex>
        );
      },
    },
    {
      title: th(t("common.actions")),
      key: "actions",
      width: agentId ? 200 : 120,
      align: "right" as const,
      render: (_: unknown, r: KnowledgeItem) => {
        const isMounted = mountedIds.includes(r.id);
        return (
          <Space size={2}>
            <Button
              type="text"
              size="small"
              icon={<Eye size={16} />}
              aria-label={t("agents.knowledge.view")}
              onClick={() => {
                setEditing(r);
                setDrawerOpen(true);
              }}
            />
            <Button
              type="text"
              size="small"
              icon={<RefreshCw size={16} />}
              aria-label={t("agents.knowledge.revectorize")}
              onClick={() => void revectorize(r.id)}
            />
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
              <Button
                type="text"
                size="small"
                danger
                icon={<Trash2 size={16} />}
                aria-label={t("common.delete")}
              />
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <>
      <Flex vertical gap={token.marginLG}>
        {/* 头部：标题 + 副标题 + 新建集合 */}
        <Flex justify="space-between" align="flex-start" gap={token.marginMD} wrap="wrap">
          <Flex vertical gap={token.marginXXS}>
            <Title level={4} style={{ margin: 0 }}>
              {t("agents.knowledge.baseTitle")}
            </Title>
            <Text type="secondary">{t("agents.knowledge.baseSubtitle")}</Text>
          </Flex>
          <Button type="primary" icon={<Plus size={18} />} onClick={() => setCreateOpen(true)}>
            {t("agents.knowledge.newCollection")}
          </Button>
        </Flex>

        {/* 上传拖拽区 + AI 训练上下文 */}
        <Flex gap={token.marginLG} align="stretch" wrap="wrap">
          <Flex
            vertical
            align="center"
            justify="center"
            gap={token.marginSM}
            onClick={() => setCreateOpen(true)}
            style={{
              flex: "1 1 480px",
              minHeight: 200,
              cursor: "pointer",
              borderRadius: token.borderRadiusLG,
              border: `1.5px dashed ${BRAND.borderLow}`,
              background: BRAND.cardBg,
              padding: token.paddingLG,
              textAlign: "center",
            }}
          >
            <Flex
              align="center"
              justify="center"
              style={{
                width: 56,
                height: 56,
                borderRadius: "50%",
                background: BRAND.primarySoft,
                color: BRAND.primary,
              }}
            >
              <UploadCloud size={26} />
            </Flex>
            <Title level={5} style={{ margin: 0 }}>
              {t("agents.knowledge.uploadTitle")}
            </Title>
            <Text type="secondary">{t("agents.knowledge.uploadHint")}</Text>
            <Space size={token.marginXS}>
              {["PDF", "TXT", "DOCX"].map((x) => (
                <span
                  key={x}
                  style={{
                    padding: "2px 10px",
                    borderRadius: 9999,
                    background: BRAND.subtleBg,
                    border: `1px solid ${BRAND.borderLow}`,
                    fontSize: 12,
                    color: BRAND.onSurfaceVariant,
                  }}
                >
                  {x}
                </span>
              ))}
            </Space>
          </Flex>

          <div
            style={{
              flex: "1 1 300px",
              maxWidth: 360,
              borderRadius: token.borderRadiusLG,
              border: `1px solid ${BRAND.borderLow}`,
              background: BRAND.cardBg,
              padding: token.paddingLG,
            }}
          >
            <Text
              style={{
                display: "block",
                textAlign: "center",
                fontSize: 12,
                fontWeight: 700,
                letterSpacing: 0.6,
                textTransform: "uppercase",
                color: BRAND.outline,
                marginBottom: token.margin,
              }}
            >
              {t("agents.knowledge.aiTrainingContext")}
            </Text>
            <Flex justify="space-between" align="center" style={{ marginBottom: token.marginSM }}>
              <Text type="secondary">{t("agents.knowledge.docCount")}</Text>
              <Text strong>{totals.docs}</Text>
            </Flex>
            <Flex justify="space-between" align="center">
              <Text type="secondary">{t("agents.knowledge.vectorCount")}</Text>
              <Text strong>{totals.vectors}</Text>
            </Flex>
          </div>
        </Flex>

        {/* 源文件表格 */}
        <div
          style={{
            borderRadius: token.borderRadiusLG,
            border: `1px solid ${BRAND.borderLow}`,
            background: BRAND.cardBg,
            overflow: "hidden",
          }}
        >
          <Flex align="center" justify="space-between" style={{ padding: token.paddingLG }}>
            <Title level={5} style={{ margin: 0 }}>
              {t("agents.knowledge.sourceFiles")}
            </Title>
            <Button
              type="text"
              icon={<RefreshCw size={16} />}
              onClick={() => void load()}
              aria-label={t("agents.knowledge.refresh")}
            />
          </Flex>
          {agentId && data.length === 0 && !loading ? (
            <Empty description={t("agents.knowledge.noneMounted")} style={{ padding: 32 }} />
          ) : (
            <Table<KnowledgeItem>
              rowKey="id"
              columns={columns}
              dataSource={data}
              loading={loading}
              pagination={{ pageSize: 8, hideOnSinglePage: true }}
              size="middle"
              scroll={{ x: "max-content" }}
            />
          )}
        </div>
      </Flex>

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
