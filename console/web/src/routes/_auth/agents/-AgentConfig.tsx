import { useNavigate } from "@tanstack/react-router";
import { App, Button, Card, Flex, Form, Input, Select, Space, Tag, Typography, theme } from "antd";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Bot, Cpu, Pause, Play, Rocket, Sparkles, Wrench } from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";
import { KnowledgePanel } from "./-KnowledgePanel";
import { ChatPanel } from "./-ChatPanel";

const { Text } = Typography;

type TabKey = "configuration" | "knowledge";

interface ModelOption {
  value: string;
  label: string;
  providerId: string;
}
interface RefItem {
  id: string;
  name?: string;
  displayName?: string;
}

interface AgentFormValues {
  name: string;
  prompt: string;
  llmModel: string;
  llmProviderId?: string;
  knowledgeIds: string[];
  toolIds: string[];
  skillIds: string[];
}

function idsOf(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((v) =>
      typeof v === "string"
        ? v
        : ((v as { id?: string; capabilityId?: string })?.id ??
          (v as { capabilityId?: string })?.capabilityId),
    )
    .filter((v): v is string => typeof v === "string" && v.length > 0);
}

const cardStyle = {
  borderRadius: 16,
  border: `1px solid ${BRAND.borderLow}`,
  boxShadow: "0 8px 24px rgba(15,23,42,0.04)",
} as const;

export function AgentConfigView({ mode, agentId }: { mode: "create" | "edit"; agentId?: string }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [form] = Form.useForm<AgentFormValues>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>("configuration");
  const [models, setModels] = useState<ModelOption[]>([]);
  const [knowledgeOpts, setKnowledgeOpts] = useState<RefItem[]>([]);
  const [tools, setTools] = useState<RefItem[]>([]);
  const [skills, setSkills] = useState<RefItem[]>([]);
  const { token } = theme.useToken();
  const isEdit = mode === "edit" && !!agentId;

  useEffect(() => {
    // 简要描述：四类下拉选项互不依赖，必须独立加载；任一能力接口失败时，
    // 已成功返回的模型数据仍应填充表单，不能被 Promise.all 整批丢弃。
    void agentApi
      .get<{
        items: Array<
          RefItem & {
            modelId?: string;
            status?: string;
            supportsReasoning?: boolean;
            providerId?: string;
            providerName?: string;
          }
        >;
      }>("/admin/llm/models", { params: { capability: "reasoning" } })
      .then((response) =>
        setModels(
          (response?.items ?? [])
            .filter((item) => item.status === "active" && item.supportsReasoning)
            .map((item) => ({
              value: item.modelId ?? item.id,
              label: `${item.displayName} (${item.modelId ?? item.id})${item.providerName ? ` - ${item.providerName}` : ""}`,
              providerId: item.providerId ?? "",
            })),
        ),
      )
      .catch((error: unknown) => {
        console.error("加载推理模型选项失败", error);
        setModels([]);
      });
    void agentApi
      .get<{ items: RefItem[] }>("/knowledge", { params: { page: 1, pageSize: 100 } })
      .then((response) => setKnowledgeOpts(response?.items ?? []))
      .catch((error: unknown) => {
        console.error("加载知识库选项失败", error);
        setKnowledgeOpts([]);
      });
    void agentApi
      .get<{ items: RefItem[] }>("/capabilities/tools/public", {
        params: { page: 1, pageSize: 100 },
      })
      .then((response) => setTools(response?.items ?? []))
      .catch((error: unknown) => {
        console.error("加载 Tool 选项失败", error);
        setTools([]);
      });
    void agentApi
      .get<{ items: RefItem[] }>("/capabilities/skills/public", {
        params: { page: 1, pageSize: 100 },
      })
      .then((response) => setSkills(response?.items ?? []))
      .catch((error: unknown) => {
        console.error("加载 Skill 选项失败", error);
        setSkills([]);
      });
  }, []);

  useEffect(() => {
    if (!isEdit || !agentId) return;
    setLoading(true);
    void agentApi
      .get<Record<string, unknown>>(`/agents/${agentId}`)
      .then((d) => {
        setStatus((d.status as string) ?? null);
        form.setFieldsValue({
          name: (d.name as string) ?? "",
          prompt: (d.prompt as string) ?? "",
          llmModel: (d.llmModel as string) ?? "",
          llmProviderId: (d.llmProviderId as string) ?? "",
          knowledgeIds: idsOf(d.knowledge),
          toolIds: idsOf(d.tools),
          skillIds: idsOf(d.skills),
        });
      })
      .catch((err) => message.error((err as Error).message || t("agentAdmin.loadFailed")))
      .finally(() => setLoading(false));
  }, [isEdit, agentId, form, message, t]);

  const submit = async (values: AgentFormValues) => {
    setSaving(true);
    try {
      const providerId =
        models.find((m) => m.value === values.llmModel)?.providerId || values.llmProviderId || "";
      if (isEdit && agentId) {
        await agentApi.post("/agents/update", {
          agentId,
          name: values.name,
          prompt: values.prompt,
          llmModel: values.llmModel,
          llmProviderId: providerId,
          knowledgeIds: values.knowledgeIds,
          toolIds: values.toolIds,
          skillIds: values.skillIds,
        });
        message.success(t("common.updated"));
      } else {
        await agentApi.post("/agents/with-bot", {
          bot_name: `${values.name} Bot`,
          name: values.name,
          type: "assistant",
          prompt: values.prompt,
          llmModel: values.llmModel,
          llmProviderId: providerId,
          knowledgeIds: values.knowledgeIds,
          toolIds: values.toolIds,
          skillIds: values.skillIds,
        });
        message.success(t("common.created"));
        void navigate({ to: "/agents" });
      }
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.saveFailed"));
    } finally {
      setSaving(false);
    }
  };

  const lifecycle = async (action: "activate" | "pause") => {
    if (!agentId) return;
    try {
      await agentApi.post(`/agents/${agentId}/${action}`);
      setStatus(action === "activate" ? "active" : "paused");
      message.success(t("common.done"));
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const selectedTags = (
    ids: string[],
    opts: Array<{ value: string; label: string }>,
    color: string,
  ) =>
    ids.length ? (
      <div style={{ marginTop: -8, marginBottom: 16 }}>
        {ids.map((id) => (
          <Tag key={id} color={color} style={{ marginBottom: 8 }}>
            {opts.find((o) => o.value === id)?.label ?? id}
          </Tag>
        ))}
      </div>
    ) : null;

  const knowledgeOptions = knowledgeOpts.map((k) => ({ value: k.id, label: k.name ?? k.id }));
  const toolOptions = tools.map((x) => ({ value: x.id, label: x.name ?? x.displayName ?? x.id }));
  const skillOptions = skills.map((x) => ({ value: x.id, label: x.name ?? x.displayName ?? x.id }));

  const tabs: Array<{ key: TabKey; label: string }> = [
    { key: "configuration", label: t("agents.tabConfiguration") },
    { key: "knowledge", label: t("agents.tabKnowledgeBase") },
  ];

  const sectionTitle = (Icon: typeof Bot, title: string, extra?: React.ReactNode) => (
    <Flex align="center" justify="space-between" style={{ marginBottom: token.margin }}>
      <Flex align="center" gap={token.marginSM}>
        <Icon size={20} color={BRAND.primary} />
        <Text strong style={{ fontSize: token.fontSizeLG }}>
          {title}
        </Text>
      </Flex>
      {extra}
    </Flex>
  );

  const chatPreview = () => (
    <Card
      style={{
        ...cardStyle,
        flex: "1 1 44%",
        minWidth: 360,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
      }}
      styles={{ body: { flex: 1, display: "flex", flexDirection: "column", padding: 0 } }}
    >
      {agentId ? (
        <ChatPanel agentId={agentId} agentStatus={status} />
      ) : (
        <Flex
          vertical
          align="center"
          justify="center"
          style={{ flex: 1, padding: 32, color: BRAND.outline }}
        >
          <Bot size={48} color={BRAND.primary} />
          <Text style={{ color: BRAND.onSurface, marginTop: 12 }}>{t("agents.chat.title")}</Text>
          <Text type="secondary">{t("agents.chat.saveFirst")}</Text>
        </Flex>
      )}
    </Card>
  );

  return (
    <Flex vertical gap={token.marginLG} style={{ maxWidth: 1280, width: "100%", margin: "0 auto" }}>
      {/* 顶部：返回 + 标题 + 状态 + Publish；下方分段标签（Configuration / Knowledge Base / Logs） */}
      <Flex justify="space-between" align="center" gap={token.margin} wrap="wrap">
        <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
          <Button
            type="text"
            icon={<ArrowLeft size={18} />}
            onClick={() => void navigate({ to: "/agents" })}
          />
          <Typography.Title level={4} style={{ margin: 0 }}>
            {isEdit ? t("agents.editAgent") : t("agents.newAgent")}
          </Typography.Title>
          {isEdit && status ? (
            <Tag
              color={status === "active" ? "green" : status === "paused" ? "orange" : "default"}
              style={{ borderRadius: 9999, marginInlineEnd: 0 }}
            >
              {status}
            </Tag>
          ) : null}
        </Flex>
        <Space size={token.marginSM}>
          {isEdit && status === "active" ? (
            <Button icon={<Pause size={16} />} danger onClick={() => void lifecycle("pause")}>
              {t("agents.pause")}
            </Button>
          ) : isEdit ? (
            <Button icon={<Play size={16} />} onClick={() => void lifecycle("activate")}>
              {t("agents.activate")}
            </Button>
          ) : null}
          <Button
            type="primary"
            icon={<Rocket size={16} />}
            loading={saving}
            onClick={() => form.submit()}
          >
            {t("agents.publish")}
          </Button>
        </Space>
      </Flex>

      {/* 分段标签栏 */}
      <Flex gap={token.marginLG} style={{ borderBottom: `1px solid ${BRAND.borderLow}` }}>
        {tabs.map((tab) => {
          const active = activeTab === tab.key;
          return (
            <div
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              style={{
                cursor: "pointer",
                padding: `${token.paddingSM}px 0`,
                fontWeight: 600,
                fontSize: token.fontSize,
                letterSpacing: 0.4,
                textTransform: "uppercase",
                color: active ? BRAND.primary : BRAND.outline,
                borderBottom: `2px solid ${active ? BRAND.primary : "transparent"}`,
                marginBottom: -1,
              }}
            >
              {tab.label}
            </div>
          );
        })}
      </Flex>

      {/* Configuration（表单常挂载，保证 Publish 始终可提交） */}
      <Flex
        gap={token.marginLG}
        align="stretch"
        style={{ display: activeTab === "configuration" ? "flex" : "none" }}
      >
        <Card loading={loading} style={{ ...cardStyle, flex: "1 1 56%", minWidth: 420 }}>
          <Form<AgentFormValues>
            layout="vertical"
            form={form}
            onFinish={submit}
            initialValues={{ knowledgeIds: [], toolIds: [], skillIds: [] }}
          >
            {/* Core Identity */}
            {sectionTitle(
              Sparkles,
              t("agents.coreIdentity"),
              <Tag color="blue" style={{ borderRadius: 9999, marginInlineEnd: 0 }}>
                {t("agents.activePrompt")}
              </Tag>,
            )}
            <Form.Item label={t("agents.colName")} name="name" rules={[{ required: true }]}>
              <Input placeholder={t("agents.namePlaceholder")} />
            </Form.Item>
            <Form.Item label={t("agents.systemPrompt")} name="prompt" rules={[{ required: true }]}>
              {/* TIPS: System Prompt 采用深色代码块样式，对齐设计稿 */}
              <Input.TextArea
                rows={10}
                placeholder={t("agents.promptPlaceholder")}
                style={{
                  background: BRAND.codeBg,
                  color: BRAND.codeText,
                  fontFamily:
                    "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace",
                  fontSize: 13,
                  lineHeight: 1.6,
                  borderRadius: token.borderRadiusLG,
                  border: "none",
                }}
              />
            </Form.Item>
            <Form.Item
              label={
                <Flex align="center" gap={6}>
                  <Cpu size={15} color={BRAND.primary} /> {t("agents.reasoningModel")}
                </Flex>
              }
              name="llmModel"
              rules={[{ required: true }]}
              style={{ marginBottom: token.marginLG }}
            >
              <Select
                placeholder={t("agents.selectModel")}
                options={models}
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>

            <div style={{ borderTop: `1px solid ${BRAND.borderLow}`, paddingTop: token.marginLG }}>
              {sectionTitle(Wrench, t("agents.activeSkills"))}
              <Form.Item label={t("agents.tools")} name="toolIds">
                <Select
                  mode="multiple"
                  placeholder={t("agents.selectTools")}
                  options={toolOptions}
                  showSearch
                  optionFilterProp="label"
                />
              </Form.Item>
              <Form.Item shouldUpdate noStyle>
                {({ getFieldValue }) =>
                  selectedTags(getFieldValue("toolIds") || [], toolOptions, "orange")
                }
              </Form.Item>
              <Form.Item label={t("agents.skills")} name="skillIds">
                <Select
                  mode="multiple"
                  placeholder={t("agents.selectSkills")}
                  options={skillOptions}
                  showSearch
                  optionFilterProp="label"
                />
              </Form.Item>
              <Form.Item shouldUpdate noStyle>
                {({ getFieldValue }) =>
                  selectedTags(getFieldValue("skillIds") || [], skillOptions, "purple")
                }
              </Form.Item>
            </div>

            <div style={{ borderTop: `1px solid ${BRAND.borderLow}`, paddingTop: token.marginLG }}>
              {sectionTitle(Bot, t("agents.knowledgeCollections"))}
              <Form.Item
                label={t("agents.knowledgeLabel")}
                name="knowledgeIds"
                style={{ marginBottom: 0 }}
              >
                <Select
                  mode="multiple"
                  placeholder={t("agents.selectKnowledge")}
                  options={knowledgeOptions}
                  showSearch
                  optionFilterProp="label"
                />
              </Form.Item>
              <Form.Item shouldUpdate noStyle>
                {({ getFieldValue }) =>
                  selectedTags(getFieldValue("knowledgeIds") || [], knowledgeOptions, "blue")
                }
              </Form.Item>
            </div>
          </Form>
        </Card>
        {chatPreview()}
      </Flex>

      {activeTab === "knowledge" ? <KnowledgePanel agentId={agentId} /> : null}
    </Flex>
  );
}
