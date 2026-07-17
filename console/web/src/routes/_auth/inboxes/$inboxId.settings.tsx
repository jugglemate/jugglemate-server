import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  App,
  Alert,
  Avatar,
  Button,
  Flex,
  Form,
  Input,
  Modal,
  Select,
  Spin,
  Tabs,
  Tag,
  Typography,
  theme,
} from "antd";
import {
  ArrowLeft,
  Bot,
  Globe,
  Headphones,
  MessageSquare,
  Send,
  ShieldCheck,
  Trash2,
  UserPlus,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { BRAND } from "@/utils/brandColors";
import { httpClient } from "@/utils/http";
import { agentApi } from "@/utils/agentHttp";
import { USER_ENDPOINTS } from "@/api/user";
import {
  bindInboxAgent,
  deleteInbox,
  getInboxAgent,
  getInbox,
  listInboxMembers,
  replaceInboxMembers,
  updateInbox,
  type Inbox,
} from "@/api/inbox";
import { PaginatedResponseSchema, UserSchema } from "@/api/schemas";

const { Title, Text } = Typography;
const { TextArea } = Input;

export const Route = createFileRoute("/_auth/inboxes/$inboxId/settings")({
  component: InboxSettingsPage,
  staticData: { hideBreadcrumb: true },
});

const UsersListResponseSchema = PaginatedResponseSchema(UserSchema);

async function fetchAssignableUsers() {
  const raw = await httpClient.get(USER_ENDPOINTS.list, { params: { limit: 500, offset: 0 } });
  return UsersListResponseSchema.shape.data.parse(raw).list;
}

const CHANNEL_VISUAL: Record<Inbox["channel_type"], { icon: LucideIcon; bg: string; fg: string }> =
  {
    widget: { icon: Globe, bg: BRAND.primarySoft, fg: BRAND.primary },
    telegram: { icon: Send, bg: BRAND.tertiarySoft, fg: BRAND.tertiary },
    juggleim: { icon: MessageSquare, bg: "rgba(101, 80, 185, 0.1)", fg: BRAND.aiAccent },
  };

function SettingsCard({ children }: { children: React.ReactNode }) {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        background: BRAND.cardBg,
        border: `1px solid ${BRAND.borderLow}`,
        borderRadius: token.borderRadiusLG,
        padding: token.paddingLG,
      }}
    >
      {children}
    </div>
  );
}

function InboxSettingsPage() {
  const { t } = useTranslation();
  const { inboxId } = Route.useParams();
  const navigate = useNavigate();
  const { token } = theme.useToken();

  const inboxQuery = useQuery({
    queryKey: ["inbox", inboxId],
    queryFn: () => getInbox(inboxId),
  });

  const inbox = inboxQuery.data;
  const visual = inbox ? CHANNEL_VISUAL[inbox.channel_type] : CHANNEL_VISUAL.widget;
  const Icon = visual.icon;

  return (
    <Flex vertical gap={token.marginLG} style={{ maxWidth: 1000, width: "100%", margin: "0 auto" }}>
      <div>
        <Button
          type="text"
          icon={<ArrowLeft size={18} />}
          onClick={() => void navigate({ to: "/inboxes", search: { limit: 100, offset: 0 } })}
        >
          {t("inboxes.backToInboxesShort")}
        </Button>
      </div>

      {inboxQuery.isLoading || !inbox ? (
        <Flex justify="center" style={{ padding: 80 }}>
          <Spin />
        </Flex>
      ) : (
        <>
          <Flex align="center" gap={token.margin}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 48,
                height: 48,
                borderRadius: token.borderRadiusLG,
                background: visual.bg,
                color: visual.fg,
                flex: "0 0 auto",
              }}
            >
              <Icon size={24} />
            </Flex>
            <Flex vertical gap={2} style={{ minWidth: 0 }}>
              <Flex align="center" gap={token.marginSM} wrap="wrap">
                <Title level={4} style={{ margin: 0 }}>
                  {inbox.name} - {t("inboxes.settings")}
                </Title>
                <Tag color="green" style={{ borderRadius: 9999, marginInlineEnd: 0 }}>
                  {t("inboxes.statusActive")}
                </Tag>
                <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                  · ID: {inbox.id}
                </Text>
              </Flex>
              <Text type="secondary">{t("inboxes.settingsSubtitle")}</Text>
            </Flex>
          </Flex>

          <Tabs
            defaultActiveKey="general"
            items={[
              {
                key: "general",
                label: t("inboxes.tabGeneral"),
                children: <GeneralTab inbox={inbox} />,
              },
              {
                key: "collaborators",
                label: t("inboxes.tabCollaborators"),
                children: <CollaboratorsTab inboxId={inbox.id} />,
              },
              {
                key: "agent",
                label: t("inboxes.tabAIAgent"),
                children: <AIAgentTab inboxId={inbox.id} />,
              },
            ]}
          />
        </>
      )}
    </Flex>
  );
}

/* ---------- General ---------- */
function GeneralTab({ inbox }: { inbox: Inbox }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { message, modal } = App.useApp();
  const [form] = Form.useForm();

  const isWidget = inbox.channel_type === "widget";

  useEffect(() => {
    form.setFieldsValue({
      name: inbox.name,
      welcome_message: isWidget ? inbox.channel_conf.welcome_message : undefined,
    });
  }, [inbox, isWidget, form]);

  const updateMutation = useMutation({
    mutationFn: (values: { name: string; welcome_message?: string }) =>
      updateInbox(inbox.id, values),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inbox", inbox.id] });
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      message.success(t("crud.updateSuccess"));
    },
    onError: () => message.error(t("crud.updateFailed")),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteInbox(inbox.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      message.success(t("crud.deleteSuccess"));
      void navigate({ to: "/inboxes", search: { limit: 100, offset: 0 } });
    },
    onError: () => message.error(t("crud.deleteFailed")),
  });

  const confirmRemove = () => {
    modal.confirm({
      title: t("inboxes.removeChannelConfirm"),
      content: t("inboxes.removeChannelConfirmDesc"),
      okText: t("inboxes.removeChannel"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: () => deleteMutation.mutate(),
    });
  };

  const botName =
    inbox.channel_type === "telegram" || inbox.channel_type === "juggleim"
      ? inbox.channel_conf.bot_name
      : "";

  return (
    <Flex vertical gap={token.marginLG} style={{ maxWidth: 720 }}>
      <SettingsCard>
        <Title level={5} style={{ marginTop: 0 }}>
          {t("inboxes.basicInformation")}
        </Title>
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) =>
            updateMutation.mutate({ name: values.name, welcome_message: values.welcome_message })
          }
        >
          <Form.Item
            name="name"
            label={t("inboxes.channelName")}
            rules={[{ required: true, message: t("inboxes.inboxNameRequired") }]}
          >
            <Input />
          </Form.Item>

          {isWidget ? (
            <Form.Item
              name="welcome_message"
              label={t("inboxes.welcomeMessage")}
              extra={t("inboxes.welcomeMessageExtra")}
            >
              <TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          ) : (
            <Form.Item label={t("inboxes.botName")} extra={t("inboxes.botSettingsReadonly")}>
              <Input value={botName} disabled />
            </Form.Item>
          )}

          <Button type="primary" htmlType="submit" loading={updateMutation.isPending}>
            {t("inboxes.updateChannel")}
          </Button>
        </Form>
      </SettingsCard>

      <div
        style={{
          background: "rgba(186, 26, 26, 0.04)",
          border: `1px solid ${token.colorErrorBorder}`,
          borderRadius: token.borderRadiusLG,
          padding: token.paddingLG,
        }}
      >
        <Title level={5} style={{ marginTop: 0, color: token.colorError }}>
          {t("inboxes.dangerZone")}
        </Title>
        <Text type="secondary" style={{ display: "block", marginBottom: token.margin }}>
          {t("inboxes.dangerZoneDesc")}
        </Text>
        <Button
          danger
          icon={<Trash2 size={16} />}
          onClick={confirmRemove}
          loading={deleteMutation.isPending}
        >
          {t("inboxes.removeChannel")}
        </Button>
      </div>
    </Flex>
  );
}

/* ---------- Collaborators ---------- */
function CollaboratorsTab({ inboxId }: { inboxId: string }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const [addOpen, setAddOpen] = useState(false);
  const [draftIds, setDraftIds] = useState<string[]>([]);

  const membersQuery = useQuery({
    queryKey: ["inbox-members", inboxId],
    queryFn: () => listInboxMembers(inboxId),
  });
  const usersQuery = useQuery({
    queryKey: ["inbox-member-users"],
    queryFn: fetchAssignableUsers,
    staleTime: 60_000,
  });

  const members = membersQuery.data ?? [];
  const roleById = useMemo(() => {
    const map = new Map<string, boolean>();
    for (const u of usersQuery.data ?? []) map.set(u.id, u.roles.includes("admin"));
    return map;
  }, [usersQuery.data]);

  const saveMutation = useMutation({
    mutationFn: (ids: string[]) => replaceInboxMembers(inboxId, ids),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inbox-members", inboxId] });
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      message.success(t("inboxes.representativesSaved"));
      setAddOpen(false);
    },
    onError: () => message.error(t("inboxes.representativesSaveFailed")),
  });

  const openAdd = () => {
    setDraftIds(members.map((m) => m.id));
    setAddOpen(true);
  };

  const removeMember = (id: string) => {
    saveMutation.mutate(members.filter((m) => m.id !== id).map((m) => m.id));
  };

  const userOptions = (usersQuery.data ?? []).map((u) => ({
    value: u.id,
    label: `${u.username}${u.email ? ` (${u.email})` : ""}`,
  }));

  const infoCards: Array<{ icon: LucideIcon; title: string; desc: string; color: string }> = [
    {
      icon: ShieldCheck,
      title: t("inboxes.roleAdmin"),
      desc: t("inboxes.infoAdminsDesc"),
      color: BRAND.primary,
    },
    {
      icon: Headphones,
      title: t("inboxes.roleAgent"),
      desc: t("inboxes.infoAgentsDesc"),
      color: BRAND.primary,
    },
    {
      icon: Bot,
      title: t("inboxes.aiServices"),
      desc: t("inboxes.infoAiServicesDesc"),
      color: BRAND.aiAccent,
    },
  ];

  return (
    <Flex vertical gap={token.marginLG}>
      <SettingsCard>
        <Flex
          justify="space-between"
          align="flex-start"
          gap={token.margin}
          style={{ marginBottom: token.margin }}
        >
          <Flex vertical gap={2}>
            <Text strong style={{ fontSize: token.fontSizeLG }}>
              {t("inboxes.teamAccess")}
            </Text>
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {t("inboxes.teamAccessDesc")}
            </Text>
          </Flex>
          <Button type="primary" icon={<UserPlus size={16} />} onClick={openAdd}>
            {t("inboxes.addCollaborator")}
          </Button>
        </Flex>

        <div style={{ borderTop: `1px solid ${BRAND.borderLow}` }}>
          {members.map((m) => {
            const isAdmin = roleById.get(m.id) ?? false;
            return (
              <Flex
                key={m.id}
                align="center"
                gap={token.margin}
                style={{
                  padding: `${token.paddingSM}px 0`,
                  borderBottom: `1px solid ${BRAND.borderLow}`,
                }}
              >
                <Avatar size={40} src={(m.avatar ?? "").trim() || undefined}>
                  {m.username?.[0]?.toUpperCase()}
                </Avatar>
                <Flex vertical gap={2} style={{ flex: 1, minWidth: 0 }}>
                  <Text strong ellipsis>
                    {m.username}
                  </Text>
                  <Text type="secondary" ellipsis style={{ fontSize: token.fontSizeSM }}>
                    {m.email || "—"}
                  </Text>
                </Flex>
                <Tag
                  color={isAdmin ? "blue" : "default"}
                  style={{ borderRadius: 9999, marginInlineEnd: 0 }}
                >
                  {isAdmin ? t("inboxes.roleAdmin") : t("inboxes.roleAgent")}
                </Tag>
                <Button
                  type="text"
                  icon={<X size={16} />}
                  onClick={() => removeMember(m.id)}
                  aria-label={t("common.delete")}
                />
              </Flex>
            );
          })}
          {!membersQuery.isLoading && members.length === 0 ? (
            <Flex justify="center" style={{ padding: token.paddingLG }}>
              <Text type="secondary">{t("inboxes.noCollaborators")}</Text>
            </Flex>
          ) : null}
        </div>
      </SettingsCard>

      <Flex gap={token.margin} wrap="wrap">
        {infoCards.map((c) => {
          const CIcon = c.icon;
          return (
            <Flex
              key={c.title}
              vertical
              gap={token.marginXS}
              style={{
                flex: "1 1 240px",
                background: BRAND.subtleBg,
                border: `1px solid ${BRAND.borderLow}`,
                borderRadius: token.borderRadiusLG,
                padding: token.paddingLG,
              }}
            >
              <CIcon size={22} color={c.color} />
              <Text strong>{c.title}</Text>
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {c.desc}
              </Text>
            </Flex>
          );
        })}
      </Flex>

      <Modal
        open={addOpen}
        title={t("inboxes.addCollaborator")}
        okText={t("common.ok")}
        cancelText={t("common.cancel")}
        confirmLoading={saveMutation.isPending}
        onCancel={() => setAddOpen(false)}
        onOk={() => saveMutation.mutate(draftIds)}
        destroyOnHidden
      >
        <Select
          mode="multiple"
          showSearch
          style={{ width: "100%" }}
          placeholder={t("inboxes.selectRepresentatives")}
          value={draftIds}
          onChange={setDraftIds}
          loading={usersQuery.isLoading}
          options={userOptions}
          optionFilterProp="label"
        />
      </Modal>
    </Flex>
  );
}

/* ---------- AI Agent ---------- */
interface AgentListItem {
  agentId: string;
  agentName: string;
  primaryModel?: string;
  status: string;
}
interface AgentPagedResponse {
  items: AgentListItem[];
  total: number;
}

function AIAgentTab({ inboxId }: { inboxId: string }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);

  const agentsQuery = useQuery({
    queryKey: ["wizard-agents"],
    queryFn: () =>
      agentApi.get<AgentPagedResponse>("/agents", { params: { page: 1, page_size: 100 } }),
  });
  const bindingQuery = useQuery({
    queryKey: ["inbox-agent", inboxId],
    queryFn: () => getInboxAgent(inboxId),
  });
  useEffect(() => {
    setSelectedAgentId(bindingQuery.data?.agent_id ?? null);
  }, [bindingQuery.data]);
  const bindingMutation = useMutation({
    mutationFn: (agentId: string) => bindInboxAgent(inboxId, agentId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inbox-agent", inboxId] });
      message.success(t("common.updated"));
    },
    onError: () => message.error(t("agentAdmin.saveFailed")),
  });
  const agents = agentsQuery.data?.items ?? [];

  return (
    <Flex vertical gap={token.marginLG}>
      {bindingQuery.data?.sync_error ? (
        <Alert type="error" showIcon message={bindingQuery.data.sync_error} />
      ) : null}
      <Flex gap={token.margin} wrap="wrap">
        <Flex
          vertical
          align="center"
          justify="center"
          gap={token.marginXS}
          onClick={() => void navigate({ to: "/agents/new" })}
          style={{
            flex: "1 1 240px",
            maxWidth: 320,
            minHeight: 150,
            cursor: "pointer",
            borderRadius: token.borderRadiusLG,
            border: `1.5px dashed ${BRAND.primary}`,
            background: BRAND.primarySoft,
            padding: token.paddingLG,
            textAlign: "center",
          }}
        >
          <Bot size={26} color={BRAND.primary} />
          <Text strong style={{ color: BRAND.primary }}>
            {t("inboxes.createNewAgent")}
          </Text>
        </Flex>

        {agents.map((a) => {
          const ready = a.status === "active";
          const selected = selectedAgentId === a.agentId;
          return (
            <Flex
              key={a.agentId}
              vertical
              gap={token.marginSM}
              onClick={() => ready && setSelectedAgentId(a.agentId)}
              style={{
                flex: "1 1 240px",
                maxWidth: 320,
                minHeight: 150,
                cursor: ready ? "pointer" : "not-allowed",
                opacity: ready ? 1 : 0.65,
                borderRadius: token.borderRadiusLG,
                border: `1.5px solid ${selected ? BRAND.primary : BRAND.borderLow}`,
                background: BRAND.cardBg,
                padding: token.paddingLG,
              }}
            >
              <Flex align="center" justify="space-between">
                <Flex
                  align="center"
                  justify="center"
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: token.borderRadiusLG,
                    background: BRAND.primarySoft,
                    color: BRAND.primary,
                  }}
                >
                  <Bot size={20} />
                </Flex>
                <Tag
                  color={ready ? "green" : "default"}
                  style={{ borderRadius: 9999, marginInlineEnd: 0 }}
                >
                  {ready ? t("inboxes.agentReady") : t("inboxes.agentDrafting")}
                </Tag>
              </Flex>
              <Text strong>{a.agentName}</Text>
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {a.primaryModel || "—"}
              </Text>
            </Flex>
          );
        })}
      </Flex>
      <Flex justify="flex-end">
        <Button
          type="primary"
          disabled={!selectedAgentId || selectedAgentId === bindingQuery.data?.agent_id}
          loading={bindingMutation.isPending}
          onClick={() => selectedAgentId && bindingMutation.mutate(selectedAgentId)}
        >
          {t("agents.save")}
        </Button>
      </Flex>
    </Flex>
  );
}
