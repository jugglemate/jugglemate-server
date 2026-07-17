import { App, Button, Drawer, Form, Input, Space, Spin, Tag } from "antd";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { File as FileIcon } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";

export interface KnowledgeRecord {
  id: string;
  name: string;
  status?: string;
  sourceType?: string;
  sourceUrl?: string | null;
  stats?: { documentCount?: number; vectorCount?: number };
  createdAt?: string;
}

const STATUS_LABELS: Record<string, string> = {
  draft: "草稿",
  pending: "待处理",
  processing: "处理中",
  ready: "就绪",
  failed: "失败",
  archived: "已归档",
};

export function KnowledgeEditDrawer({
  open,
  knowledge,
  onClose,
  onSuccess,
}: {
  open: boolean;
  knowledge: KnowledgeRecord | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [detail, setDetail] = useState<KnowledgeRecord | null>(null);

  useEffect(() => {
    if (!open || !knowledge) {
      form.resetFields();
      setDetail(null);
      return;
    }
    setLoading(true);
    agentApi
      .get<KnowledgeRecord>(`/knowledge/${knowledge.id}`)
      .then((d) => {
        setDetail(d);
        form.setFieldsValue({ name: d.name, sourceUrl: d.sourceUrl ?? "" });
      })
      .catch((err) => message.error((err as Error).message || t("agentAdmin.loadFailed")))
      .finally(() => setLoading(false));
  }, [open, knowledge, form, message, t]);

  const save = async () => {
    if (!knowledge) return;
    try {
      const values = await form.validateFields();
      setSaving(true);
      await agentApi.post("/knowledge/update", {
        knowledgeId: knowledge.id,
        name: values.name,
        sourceUrl: values.sourceUrl,
      });
      message.success(t("common.updated"));
      onSuccess();
    } catch (err) {
      if ((err as { errorFields?: unknown }).errorFields) return;
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const vectorize = async () => {
    if (!knowledge) return;
    try {
      setSaving(true);
      await agentApi.post(`/knowledge/${knowledge.id}/vectorize`);
      message.success(t("agents.knowledge.vectorizeStarted"));
      onSuccess();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    } finally {
      setSaving(false);
    }
  };

  const isUrl = detail?.sourceType === "url";

  return (
    <Drawer
      title={t("agents.knowledge.editTitle")}
      placement="right"
      width={480}
      onClose={onClose}
      open={open}
      footer={
        <Space style={{ display: "flex", justifyContent: "flex-end" }}>
          <Button onClick={onClose}>{t("common.cancel")}</Button>
          {detail?.status === "draft" ? (
            <Button type="primary" onClick={() => void vectorize()} loading={saving}>
              {t("agents.knowledge.triggerVectorize")}
            </Button>
          ) : null}
          <Button type="primary" onClick={() => void save()} loading={saving}>
            {t("common.ok")}
          </Button>
        </Space>
      }
    >
      <Spin spinning={loading}>
        <Form form={form} layout="vertical">
          <Form.Item
            label={t("agents.knowledge.namePlaceholder")}
            name="name"
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          {isUrl ? (
            <Form.Item
              label={t("agents.knowledge.docUrl")}
              name="sourceUrl"
              rules={[{ required: true }, { type: "url" }]}
            >
              <Input placeholder="https://example.com/document.pdf" />
            </Form.Item>
          ) : (
            <Form.Item label={t("agents.knowledge.source")}>
              <Tag icon={<FileIcon size={12} />} color="blue">
                {t("agents.knowledge.fileUploaded")}
              </Tag>
            </Form.Item>
          )}
          {detail ? (
            <div style={{ padding: 16, background: BRAND.subtleBg, borderRadius: 8 }}>
              <div style={{ marginBottom: 8 }}>
                <b>{t("models.colStatus")}：</b>
                {STATUS_LABELS[detail.status ?? ""] ?? detail.status}
              </div>
              <div style={{ color: BRAND.onSurfaceVariant }}>
                {t("agents.knowledge.docCount")}：{detail.stats?.documentCount ?? 0}
              </div>
              <div style={{ color: BRAND.onSurfaceVariant }}>
                {t("agents.knowledge.vectorCount")}：{detail.stats?.vectorCount ?? 0}
              </div>
              <div style={{ color: BRAND.onSurfaceVariant }}>
                {t("page.createdAt", { defaultValue: "创建时间" })}：
                {detail.createdAt ? new Date(detail.createdAt).toLocaleString() : "--"}
              </div>
            </div>
          ) : null}
        </Form>
      </Spin>
    </Drawer>
  );
}
