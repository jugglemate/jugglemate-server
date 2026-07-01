import { useNavigate } from "@tanstack/react-router";
import { App, Button, Card, Flex, Form, Input, Menu, Select, Space, Tag, Typography } from "antd";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Bot,
  BookText,
  CloudUpload,
  Gauge,
  Pause,
  Play,
  Save,
  Settings,
} from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { KnowledgePanel } from "./-KnowledgePanel";
import { LogsPanel, type ConversationDetail } from "./-LogsPanel";
import { ChatPanel } from "./-ChatPanel";

const { Text } = Typography;

type TabKey = "workspace" | "knowledge" | "logs" | "monitor";

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

const sectionStyle = {
  background: "#FAFAFA",
  border: "1px solid #F0F0F0",
  borderRadius: 16,
  padding: 20,
  marginBottom: 20,
} as const;
const cardStyle = {
  borderRadius: 20,
  border: "1px solid #ECEEF2",
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
  const [activeTab, setActiveTab] = useState<TabKey>("workspace");
  const [models, setModels] = useState<ModelOption[]>([]);
  const [knowledgeOpts, setKnowledgeOpts] = useState<RefItem[]>([]);
  const [tools, setTools] = useState<RefItem[]>([]);
  const [skills, setSkills] = useState<RefItem[]>([]);
  const [selectedConv, setSelectedConv] = useState<ConversationDetail | null>(null);
  const isEdit = mode === "edit" && !!agentId;

  useEffect(() => {
    void (async () => {
      try {
        const [m, k, tl, sk] = await Promise.all([
          agentApi.get<{
            items: Array<
              RefItem & {
                modelId?: string;
                status?: string;
                supportsReasoning?: boolean;
                providerId?: string;
                providerName?: string;
              }
            >;
          }>("/admin/llm/models", { params: { capability: "reasoning" } }),
          agentApi.get<{ items: RefItem[] }>("/knowledge", { params: { page: 1, pageSize: 100 } }),
          agentApi.get<{ items: RefItem[] }>("/capabilities/tools/public", {
            params: { page: 1, pageSize: 100 },
          }),
          agentApi.get<{ items: RefItem[] }>("/capabilities/skills/public"),
        ]);
        setModels(
          (m?.items ?? [])
            .filter((x) => x.status === "active" && x.supportsReasoning)
            .map((x) => ({
              value: x.modelId ?? x.id,
              label: `${x.displayName} (${x.modelId ?? x.id})${x.providerName ? ` - ${x.providerName}` : ""}`,
              providerId: x.providerId ?? "",
            })),
        );
        setKnowledgeOpts(k?.items ?? []);
        setTools(tl?.items ?? []);
        setSkills(sk?.items ?? []);
      } catch {
        /* ignore option load errors */
      }
    })();
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
        await agentApi.post("/agents/create", {
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

  const menuItems = [
    { key: "workspace", icon: <Settings size={16} />, label: "Workspace" },
    { key: "knowledge", icon: <CloudUpload size={16} />, label: "Knowledge" },
    { key: "logs", icon: <BookText size={16} />, label: "Logs" },
    { key: "monitor", icon: <Gauge size={16} />, label: "Monitor" },
  ];

  const chatPreview = (conv?: ConversationDetail | null) => (
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
        <ChatPanel
          agentId={conv?.agentId ?? agentId}
          agentStatus={status}
          conversationId={conv?.id}
          targetUserId={conv?.userId}
          initialMessages={conv?.messages}
        />
      ) : (
        <Flex
          vertical
          align="center"
          justify="center"
          style={{ flex: 1, padding: 32, color: "#999" }}
        >
          <Bot size={48} color="#2287fc" />
          <Text style={{ color: "#111", marginTop: 12 }}>{t("agents.chat.title")}</Text>
          <Text type="secondary">{t("agents.chat.saveFirst")}</Text>
        </Flex>
      )}
    </Card>
  );

  return (
    <Flex vertical gap={16}>
      <Space align="center" size={12}>
        <Button icon={<ArrowLeft size={16} />} onClick={() => void navigate({ to: "/agents" })} />
        <Typography.Title level={4} style={{ margin: 0 }}>
          {isEdit ? t("agents.editAgent") : t("agents.newAgent")}
        </Typography.Title>
        {isEdit && status ? (
          <Tag color={status === "active" ? "green" : status === "paused" ? "orange" : "default"}>
            {status}
          </Tag>
        ) : null}
      </Space>

      <Flex gap={16} align="flex-start">
        {/* Left rail */}
        <div style={{ width: 240, flexShrink: 0 }}>
          <Card style={{ ...cardStyle, borderRadius: 20 }} styles={{ body: { padding: 16 } }}>
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <Menu
                mode="inline"
                selectedKeys={[activeTab]}
                style={{ borderInlineEnd: 0, background: "transparent" }}
                items={menuItems}
                onClick={({ key }) => setActiveTab(key as TabKey)}
              />
              <div
                style={{
                  background: "#F7F8FA",
                  borderRadius: 16,
                  padding: 12,
                  border: "1px solid #F0F0F0",
                }}
              >
                <Space direction="vertical" style={{ width: "100%" }}>
                  <Button
                    type="primary"
                    icon={<Save size={16} />}
                    style={{ width: "100%", height: 40 }}
                    loading={saving}
                    onClick={() => form.submit()}
                  >
                    {t("agents.save")}
                  </Button>
                  {isEdit && status !== "active" ? (
                    <Button
                      icon={<Play size={16} />}
                      style={{ width: "100%", height: 40 }}
                      onClick={() => void lifecycle("activate")}
                    >
                      {t("agents.activate")}
                    </Button>
                  ) : null}
                  {isEdit && status === "active" ? (
                    <Button
                      icon={<Pause size={16} />}
                      danger
                      style={{ width: "100%", height: 40 }}
                      onClick={() => void lifecycle("pause")}
                    >
                      {t("agents.pause")}
                    </Button>
                  ) : null}
                </Space>
              </div>
            </Space>
          </Card>
        </div>

        {/* Content */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {/* Workspace（表单常挂载，保证左侧保存按钮始终可用） */}
          <Flex
            gap={16}
            align="stretch"
            style={{ display: activeTab === "workspace" ? "flex" : "none" }}
          >
            <Card loading={loading} style={{ ...cardStyle, flex: "1 1 56%", minWidth: 420 }}>
              <Form<AgentFormValues>
                layout="vertical"
                form={form}
                onFinish={submit}
                initialValues={{ knowledgeIds: [], toolIds: [], skillIds: [] }}
              >
                <div style={sectionStyle}>
                  <Form.Item label={t("agents.colName")} name="name" rules={[{ required: true }]}>
                    <Input placeholder={t("agents.namePlaceholder")} />
                  </Form.Item>
                  <Form.Item label={t("agents.prompt")} name="prompt" rules={[{ required: true }]}>
                    <Input.TextArea rows={3} placeholder={t("agents.promptPlaceholder")} />
                  </Form.Item>
                  <Form.Item
                    label={t("agents.reasoningModel")}
                    name="llmModel"
                    rules={[{ required: true }]}
                    style={{ marginBottom: 0 }}
                  >
                    <Select
                      placeholder={t("agents.selectModel")}
                      options={models}
                      showSearch
                      optionFilterProp="label"
                    />
                  </Form.Item>
                </div>
                <div style={{ ...sectionStyle, marginBottom: 0 }}>
                  <Space direction="vertical" size={4} style={{ width: "100%", marginBottom: 20 }}>
                    <Text strong style={{ fontSize: 16 }}>
                      Capabilities
                    </Text>
                    <Text type="secondary">{t("agents.capabilitiesDesc")}</Text>
                  </Space>
                  <Form.Item label={t("agents.knowledgeLabel")} name="knowledgeIds">
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
                  <Form.Item
                    label={t("agents.skills")}
                    name="skillIds"
                    style={{ marginBottom: 12 }}
                  >
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
              </Form>
            </Card>
            {chatPreview()}
          </Flex>

          {activeTab === "knowledge" ? <KnowledgePanel agentId={agentId} /> : null}

          {activeTab === "logs" ? (
            <Flex gap={16} align="flex-start">
              <Card style={{ ...cardStyle, flex: "1 1 56%", minWidth: 380 }}>
                <LogsPanel agentId={agentId as string} onSelect={setSelectedConv} />
              </Card>
              {chatPreview(selectedConv)}
            </Flex>
          ) : null}

          {activeTab === "monitor" ? (
            <Card style={cardStyle}>
              <Flex align="center" justify="center" style={{ padding: 40 }}>
                <Text type="secondary">{t("agents.monitorSoon")}</Text>
              </Flex>
            </Card>
          ) : null}
        </div>
      </Flex>
    </Flex>
  );
}
