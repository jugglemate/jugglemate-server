import { useNavigate } from "@tanstack/react-router";
import {
  App,
  Button,
  Card,
  Checkbox,
  Col,
  Divider,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Plus, Save, Trash2 } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";

const PROTOCOLS = ["openai", "anthropic", "openai-compatible"];
const STATUSES = ["draft", "active", "disabled"];

interface ExistingModel {
  id: string;
  modelId: string;
  displayName: string;
  status: string;
  supportsReasoning?: boolean;
  supportsSummary?: boolean;
  supportsEmbedding?: boolean;
  contextLength?: number;
  apiModelName?: string;
}

interface ProviderDetail {
  name: string;
  protocol: string;
  status: string;
  baseUrl?: string | null;
  base_url?: string | null;
  timeoutSeconds?: number;
  timeout_seconds?: number;
  retryCount?: number;
  retry_count?: number;
  models?: ExistingModel[];
}

interface InlineModel {
  model_id: string;
  display_name: string;
  api_model_name: string;
  status?: string;
  context_length: number;
  max_output: number;
  input_price_per_1k: number;
  output_price_per_1k: number;
  supports_reasoning?: boolean;
  supports_summary?: boolean;
  supports_embedding?: boolean;
  embedding_dimension?: number;
}

interface FormValues {
  name: string;
  protocol: string;
  status: string;
  base_url?: string;
  api_key?: string;
  replace_api_key?: boolean;
  timeout_seconds: number;
  retry_count: number;
  models?: InlineModel[];
}

const DEFAULT_MODEL: Partial<InlineModel> = {
  status: "draft",
  supports_reasoning: true,
  context_length: 128000,
  max_output: 4096,
  input_price_per_1k: 0,
  output_price_per_1k: 0,
};

export function ModelConfigView({
  mode,
  providerId,
}: {
  mode: "create" | "edit";
  providerId?: string;
}) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm<FormValues>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [existingModels, setExistingModels] = useState<ExistingModel[]>([]);
  const replaceApiKey = Form.useWatch("replace_api_key", form) ?? false;
  const isEdit = mode === "edit";

  const loadDetail = useMemo(
    () => async () => {
      if (!isEdit || !providerId) return;
      setLoading(true);
      try {
        const data = await agentApi.get<ProviderDetail>(`/admin/llm/providers/${providerId}`);
        setExistingModels(data.models ?? []);
        form.setFieldsValue({
          name: data.name,
          protocol: data.protocol,
          status: data.status,
          replace_api_key: false,
          base_url: data.baseUrl ?? data.base_url ?? undefined,
          timeout_seconds: data.timeoutSeconds ?? data.timeout_seconds ?? 60,
          retry_count: data.retryCount ?? data.retry_count ?? 3,
        });
      } catch (err) {
        message.error((err as Error).message || t("agentAdmin.loadFailed"));
      } finally {
        setLoading(false);
      }
    },
    [form, isEdit, providerId, message, t],
  );

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  const submit = async (values: FormValues) => {
    const validModels = (values.models ?? [])
      .filter((m) => m && m.model_id && m.display_name)
      .map((m) => {
        if (m.supports_embedding)
          return { ...m, embedding_dimension: m.embedding_dimension ?? 1536 };
        const { embedding_dimension: _omit, ...rest } = m;
        return rest;
      });
    setSaving(true);
    try {
      if (isEdit && providerId) {
        if (values.replace_api_key && !values.api_key?.trim()) {
          message.error(t("models.form.apiKeyReplaceRequired"));
          return;
        }
        const payload: Record<string, unknown> = {
          providerId,
          name: values.name,
          protocol: values.protocol,
          status: values.status,
          base_url: values.base_url || null,
          timeout_seconds: values.timeout_seconds,
          retry_count: values.retry_count,
          models: validModels.length ? validModels : undefined,
        };
        if (values.replace_api_key && values.api_key?.trim()) {
          payload.api_key = values.api_key.trim();
          payload.replace_api_key = true;
        }
        await agentApi.post("/admin/llm/providers/update", payload);
      } else {
        await agentApi.post("/admin/llm/providers", {
          name: values.name,
          protocol: values.protocol,
          status: values.status,
          api_key: values.api_key?.trim() ?? "",
          base_url: values.base_url || null,
          timeout_seconds: values.timeout_seconds,
          retry_count: values.retry_count,
          models: validModels.length ? validModels : undefined,
        });
      }
      message.success(t("common.done"));
      void navigate({ to: "/models" });
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const deleteModel = async (m: ExistingModel) => {
    try {
      await agentApi.delete(`/admin/llm/models/${m.id}`);
      message.success(t("common.deleted"));
      await loadDetail();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Space align="center" size={12}>
        <Button icon={<ArrowLeft size={16} />} onClick={() => void navigate({ to: "/models" })} />
        <Typography.Title level={4} style={{ margin: 0 }}>
          {isEdit ? t("models.form.titleEdit") : t("models.form.titleCreate")}
        </Typography.Title>
      </Space>
      <Card loading={loading}>
        <Form<FormValues>
          layout="vertical"
          form={form}
          onFinish={submit}
          initialValues={{
            protocol: "openai",
            status: "draft",
            timeout_seconds: 60,
            retry_count: 3,
          }}
        >
          <Row gutter={16}>
            <Col xs={24} sm={12} md={6}>
              <Form.Item label={t("models.form.name")} name="name" rules={[{ required: true }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Form.Item
                label={t("models.colProtocol")}
                name="protocol"
                rules={[{ required: true }]}
              >
                <Select options={PROTOCOLS.map((p) => ({ label: p, value: p }))} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Form.Item label={t("models.colStatus")} name="status" rules={[{ required: true }]}>
                <Select options={STATUSES.map((s) => ({ label: s, value: s }))} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Form.Item label={t("models.colBaseUrl")} name="base_url">
                <Input placeholder="https://api.openai.com/v1" />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Form.Item
                label={t("models.apiKey")}
                name="api_key"
                rules={isEdit ? [] : [{ required: true }]}
              >
                <Input.Password
                  disabled={isEdit && !replaceApiKey}
                  autoComplete={isEdit ? "new-password" : "off"}
                  placeholder={isEdit && !replaceApiKey ? t("models.apiKeyKeep") : undefined}
                />
              </Form.Item>
            </Col>
            {isEdit ? (
              <Col xs={24} sm={12} md={6}>
                <Form.Item
                  name="replace_api_key"
                  valuePropName="checked"
                  initialValue={false}
                  label={<span style={{ visibility: "hidden" }}>_</span>}
                >
                  <Checkbox>{t("models.form.replaceApiKey")}</Checkbox>
                </Form.Item>
              </Col>
            ) : null}
            <Col xs={24} sm={12} md={6}>
              <Form.Item
                label={t("models.timeout")}
                name="timeout_seconds"
                rules={[{ required: true }]}
              >
                <InputNumber min={10} max={300} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Form.Item label={t("models.retry")} name="retry_count" rules={[{ required: true }]}>
                <InputNumber min={0} max={5} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider titlePlacement="start">{t("models.form.modelsSection")}</Divider>
          <Typography.Text type="secondary" style={{ marginBottom: 16, display: "block" }}>
            {t("models.form.modelsSectionDesc")}
          </Typography.Text>

          {isEdit && existingModels.length > 0 ? (
            <div style={{ marginBottom: 24 }}>
              <Typography.Text strong style={{ marginBottom: 8, display: "block" }}>
                {t("models.form.configuredModels", { count: existingModels.length })}
              </Typography.Text>
              <Table<ExistingModel>
                dataSource={existingModels}
                rowKey="id"
                size="small"
                pagination={false}
                columns={[
                  { title: t("models.form.modelId"), dataIndex: "modelId", width: 180 },
                  {
                    title: t("models.form.modelDisplayName"),
                    dataIndex: "displayName",
                    width: 150,
                  },
                  {
                    title: t("models.colStatus"),
                    dataIndex: "status",
                    width: 90,
                    render: (s: string) => (
                      <Tag color={s === "active" ? "green" : s === "draft" ? "blue" : "default"}>
                        {s}
                      </Tag>
                    ),
                  },
                  {
                    title: t("models.form.modelCapabilities"),
                    key: "cap",
                    render: (_: unknown, r: ExistingModel) => (
                      <Space size={4}>
                        {r.supportsReasoning ? (
                          <Tag color="blue">{t("models.form.supportsReasoning")}</Tag>
                        ) : null}
                        {r.supportsSummary ? (
                          <Tag color="purple">{t("models.form.supportsSummary")}</Tag>
                        ) : null}
                        {r.supportsEmbedding ? (
                          <Tag color="cyan">{t("models.form.supportsEmbedding")}</Tag>
                        ) : null}
                      </Space>
                    ),
                  },
                  {
                    title: t("models.form.apiModelName"),
                    dataIndex: "apiModelName",
                    ellipsis: true,
                  },
                  {
                    title: t("common.actions"),
                    key: "actions",
                    width: 90,
                    render: (_: unknown, r: ExistingModel) => (
                      <Button
                        type="link"
                        size="small"
                        danger
                        icon={<Trash2 size={14} />}
                        onClick={() => void deleteModel(r)}
                      >
                        {t("common.delete")}
                      </Button>
                    ),
                  },
                ]}
              />
            </div>
          ) : null}

          <Form.List name="models">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Card
                    key={key}
                    size="small"
                    style={{ marginBottom: 16 }}
                    title={`${t("models.form.addModel")} #${name + 1}`}
                    extra={
                      <Button
                        type="text"
                        danger
                        size="small"
                        icon={<Trash2 size={14} />}
                        onClick={() => remove(name)}
                      >
                        {t("models.form.removeModel")}
                      </Button>
                    }
                  >
                    <Row gutter={16}>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "model_id"]}
                          label={t("models.form.modelId")}
                          rules={[{ required: true }]}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "display_name"]}
                          label={t("models.form.modelDisplayName")}
                          rules={[{ required: true }]}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "api_model_name"]}
                          label={t("models.form.apiModelName")}
                          rules={[{ required: true }]}
                        >
                          <Input />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "status"]}
                          label={t("models.colStatus")}
                          initialValue="draft"
                        >
                          <Select options={STATUSES.map((s) => ({ label: s, value: s }))} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "context_length"]}
                          label={t("models.form.contextLength")}
                          rules={[{ required: true }]}
                        >
                          <InputNumber min={1} style={{ width: "100%" }} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "max_output"]}
                          label={t("models.form.maxOutput")}
                          rules={[{ required: true }]}
                        >
                          <InputNumber min={1} style={{ width: "100%" }} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "input_price_per_1k"]}
                          label={t("models.form.inputPrice")}
                          rules={[{ required: true }]}
                        >
                          <InputNumber min={0} step={0.0001} style={{ width: "100%" }} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} sm={12} md={6}>
                        <Form.Item
                          {...restField}
                          name={[name, "output_price_per_1k"]}
                          label={t("models.form.outputPrice")}
                          rules={[{ required: true }]}
                        >
                          <InputNumber min={0} step={0.0001} style={{ width: "100%" }} />
                        </Form.Item>
                      </Col>
                      <Col xs={24}>
                        <Space size="large">
                          <Form.Item
                            {...restField}
                            name={[name, "supports_reasoning"]}
                            valuePropName="checked"
                            noStyle
                          >
                            <Checkbox>{t("models.form.supportsReasoning")}</Checkbox>
                          </Form.Item>
                          <Form.Item
                            {...restField}
                            name={[name, "supports_summary"]}
                            valuePropName="checked"
                            noStyle
                          >
                            <Checkbox>{t("models.form.supportsSummary")}</Checkbox>
                          </Form.Item>
                          <Form.Item
                            {...restField}
                            name={[name, "supports_embedding"]}
                            valuePropName="checked"
                            noStyle
                          >
                            <Checkbox>{t("models.form.supportsEmbedding")}</Checkbox>
                          </Form.Item>
                        </Space>
                      </Col>
                    </Row>
                  </Card>
                ))}
                <Button
                  type="dashed"
                  onClick={() => add({ ...DEFAULT_MODEL })}
                  block
                  icon={<Plus size={16} />}
                >
                  {t("models.form.addModel")}
                </Button>
              </>
            )}
          </Form.List>

          <Divider />
          <Space>
            <Button icon={<ArrowLeft size={16} />} onClick={() => void navigate({ to: "/models" })}>
              {t("common.cancel")}
            </Button>
            <Button icon={<Save size={16} />} type="primary" htmlType="submit" loading={saving}>
              {t("common.ok")}
            </Button>
          </Space>
        </Form>
      </Card>
    </Space>
  );
}
