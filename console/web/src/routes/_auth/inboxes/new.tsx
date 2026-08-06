import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Checkbox, Flex, Form, Input, Select, Tag, Typography, theme } from "antd";
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  Check,
  Globe,
  Info,
  Lightbulb,
  MessageSquare,
  Phone,
  Plus,
  Search,
  Send,
  Sparkles,
  UserPlus,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { z } from "zod/v4";
import { BRAND } from "@/utils/brandColors";
import { httpClient } from "@/utils/http";
import { agentApi } from "@/utils/agentHttp";
import { USER_ENDPOINTS } from "@/api/user";
import {
  bindInboxAgent,
  createJuggleIMInbox,
  createTelegramInbox,
  createWhatsAppInbox,
  createWidgetInbox,
  replaceInboxMembers,
  type Inbox,
} from "@/api/inbox";
import {
  CreateJuggleIMInboxRequestSchema,
  CreateTelegramInboxRequestSchema,
  CreateWhatsAppInboxRequestSchema,
  CreateWidgetInboxRequestSchema,
  PaginatedResponseSchema,
  UserSchema,
} from "@/api/schemas";

const { Title, Text } = Typography;
const { TextArea } = Input;

export const Route = createFileRoute("/_auth/inboxes/new")({
  component: NewInboxWizard,
  staticData: { hideBreadcrumb: true },
});

type WizardChannel = "widget" | "telegram" | "juggleim" | "whatsapp";
type WizardStep = 0 | 1 | 2 | 3;

const UsersListResponseSchema = PaginatedResponseSchema(UserSchema);

/** 拉取可分配为收件箱代表的成员列表（复用 console 用户接口）。 */
async function fetchAssignableUsers() {
  const raw = await httpClient.get(USER_ENDPOINTS.list, { params: { limit: 500, offset: 0 } });
  return UsersListResponseSchema.shape.data.parse(raw).list;
}

/** 系统内置 Agent 的 ownerId，与后端 agent/service 保持一致。 */
const SYSTEM_OWNER_ID = "system";

interface AgentListItem {
  agentId: string;
  agentName: string;
  ownerId: string;
  primaryModel?: string;
  status: string;
  knowledgeCount?: number;
  toolCount?: number;
}
interface AgentPagedResponse {
  items: AgentListItem[];
  total: number;
}

/** 顶部步骤指示器：已完成=蓝底对勾，当前=蓝底数字，未来=浅色数字。 */
function Stepper({ current, labels }: { current: number; labels: string[] }) {
  const { token } = theme.useToken();
  return (
    <Flex align="center" justify="center" style={{ width: "100%", maxWidth: 560 }}>
      {labels.map((label, index) => {
        const done = index < current;
        const active = index === current;
        const circleBg = done || active ? BRAND.primaryFill : BRAND.cardBg;
        const circleColor = done || active ? BRAND.onPrimaryFill : BRAND.outline;
        const circleBorder = done || active ? BRAND.primaryFill : BRAND.borderLow;
        return (
          <Flex
            key={label}
            align="center"
            style={{ flex: index < labels.length - 1 ? 1 : "0 0 auto" }}
          >
            <Flex vertical align="center" gap={6} style={{ flex: "0 0 auto" }}>
              <Flex
                align="center"
                justify="center"
                style={{
                  width: 32,
                  height: 32,
                  borderRadius: "50%",
                  background: circleBg,
                  color: circleColor,
                  border: `1.5px solid ${circleBorder}`,
                  fontWeight: 600,
                  fontSize: 14,
                }}
              >
                {done ? <Check size={16} /> : index + 1}
              </Flex>
              <span
                style={{
                  fontSize: token.fontSizeSM,
                  fontWeight: active ? 600 : 500,
                  color: done || active ? BRAND.primary : BRAND.outline,
                }}
              >
                {label}
              </span>
            </Flex>
            {index < labels.length - 1 ? (
              <div
                style={{
                  flex: 1,
                  height: 2,
                  margin: "0 8px",
                  marginBottom: 22,
                  background: index < current ? BRAND.primaryFill : BRAND.borderLow,
                }}
              />
            ) : null}
          </Flex>
        );
      })}
    </Flex>
  );
}

/** 右侧信息卡片容器（配置指南 / 角色说明 / 工作原理等）。 */
function AsideCard({
  children,
  tone = "default",
}: {
  children: React.ReactNode;
  tone?: "default" | "primary" | "muted";
}) {
  const { token } = theme.useToken();
  const bg =
    tone === "primary" ? BRAND.primarySoft : tone === "muted" ? BRAND.subtleBg : BRAND.cardBg;
  return (
    <div
      style={{
        background: bg,
        border: `1px solid ${BRAND.borderLow}`,
        borderRadius: token.borderRadiusLG,
        padding: token.paddingLG,
      }}
    >
      {children}
    </div>
  );
}

const CHANNEL_CARDS: Array<{
  key: WizardChannel;
  icon: LucideIcon;
  bg: string;
  fg: string;
  titleKey: string;
  descKey: string;
  ctaKey: string;
}> = [
  {
    key: "widget",
    icon: Globe,
    bg: BRAND.primarySoft,
    fg: BRAND.primary,
    titleKey: "inboxes.websiteTitle",
    descKey: "inboxes.websiteDesc",
    ctaKey: "inboxes.createAndContinue",
  },
  {
    key: "whatsapp",
    icon: Phone,
    bg: "#E9F8EF",
    fg: "#168B4B",
    titleKey: "inboxes.whatsappTitle",
    descKey: "inboxes.whatsappDesc",
    ctaKey: "inboxes.createAndContinue",
  },
  {
    key: "telegram",
    icon: Send,
    bg: BRAND.tertiarySoft,
    fg: BRAND.tertiary,
    titleKey: "inboxes.telegramTitle",
    descKey: "inboxes.telegramDesc",
    ctaKey: "inboxes.createAndContinue",
  },
  {
    key: "juggleim",
    icon: MessageSquare,
    bg: BRAND.aiAccentSoft,
    fg: BRAND.aiAccent,
    titleKey: "inboxes.juggleIMTitle",
    descKey: "inboxes.juggleIMDesc",
    ctaKey: "inboxes.createAndContinue",
  },
];

function NewInboxWizard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const { token } = theme.useToken();

  const [step, setStep] = useState<WizardStep>(0);
  const [channel, setChannel] = useState<WizardChannel | null>(null);
  const [createdInbox, setCreatedInbox] = useState<Inbox | null>(null);
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [memberQuery, setMemberQuery] = useState("");
  const [form] = Form.useForm();

  const stepLabels = [
    t("inboxes.stepChannel"),
    t("inboxes.stepConfigure"),
    t("inboxes.stepTeam"),
    t("inboxes.stepAgent"),
  ];

  const usersQuery = useQuery({
    queryKey: ["inbox-member-users"],
    queryFn: fetchAssignableUsers,
    enabled: step === 2,
    staleTime: 60_000,
  });

  const agentsQuery = useQuery({
    queryKey: ["wizard-agents"],
    queryFn: () =>
      agentApi.get<AgentPagedResponse>("/agents", { params: { page: 1, page_size: 100 } }),
    enabled: step === 3,
  });

  const createMutation = useMutation({
    mutationFn: (values: Record<string, unknown>): Promise<Inbox> => {
      if (channel === "widget") {
        return createWidgetInbox(CreateWidgetInboxRequestSchema.parse(values));
      }
      if (channel === "telegram") {
        return createTelegramInbox(CreateTelegramInboxRequestSchema.parse(values));
      }
      if (channel === "whatsapp") {
        return createWhatsAppInbox(CreateWhatsAppInboxRequestSchema.parse(values));
      }
      return createJuggleIMInbox(CreateJuggleIMInboxRequestSchema.parse(values));
    },
    onSuccess: (inbox) => {
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      setCreatedInbox(inbox);
      setStep(2);
    },
    onError: () => message.error(t("inboxes.widgetCreateFailed")),
  });

  const membersMutation = useMutation({
    mutationFn: () => replaceInboxMembers(createdInbox?.id ?? "", selectedUserIds),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      message.success(t("inboxes.representativesSaved"));
      setStep(3);
    },
    onError: () => message.error(t("inboxes.representativesSaveFailed")),
  });

  const agentMutation = useMutation({
    mutationFn: () => bindInboxAgent(createdInbox?.id ?? "", selectedAgentId ?? ""),
    onSuccess: () => finishSetup(),
    onError: () => message.error(t("agentAdmin.saveFailed")),
  });

  const pickChannel = (next: WizardChannel) => {
    setChannel(next);
    setCreatedInbox(null);
    setSelectedUserIds([]);
    form.resetFields();
    setStep(1);
  };

  const handleConfigureNext = () => {
    // TIPS: 收件箱已创建时（从后续步骤返回再前进）跳过二次创建，直接进入团队步骤。
    if (createdInbox) {
      setStep(2);
      return;
    }
    void form.validateFields().then((values) => createMutation.mutate(values));
  };

  const filteredUsers = useMemo(() => {
    const list = usersQuery.data ?? [];
    const q = memberQuery.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (u) => u.username.toLowerCase().includes(q) || (u.email ?? "").toLowerCase().includes(q),
    );
  }, [usersQuery.data, memberQuery]);

  const toggleUser = (id: string) => {
    setSelectedUserIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id],
    );
  };

  const finishSetup = () => {
    message.success(t("inboxes.setupComplete"));
    void navigate({ to: "/inboxes", search: { limit: 100, offset: 0 } });
  };

  const backLink =
    step === 0
      ? {
          label: t("inboxes.backToInboxesShort"),
          onClick: () => void navigate({ to: "/inboxes", search: { limit: 100, offset: 0 } }),
        }
      : step === 1
        ? { label: t("inboxes.backToSelectChannel"), onClick: () => setStep(0) }
        : step === 2
          ? { label: t("inboxes.backToConfigure"), onClick: () => setStep(1) }
          : { label: t("inboxes.backToTeam"), onClick: () => setStep(2) };

  return (
    <Flex vertical gap={token.marginLG} style={{ maxWidth: 1120, width: "100%", margin: "0 auto" }}>
      {/* 顶部：返回链接 + 取消 */}
      <Flex align="center" justify="space-between">
        <Button type="text" icon={<ArrowLeft size={18} />} onClick={backLink.onClick}>
          {backLink.label}
        </Button>
        <Button
          type="text"
          onClick={() => void navigate({ to: "/inboxes", search: { limit: 100, offset: 0 } })}
        >
          {t("common.cancel")}
        </Button>
      </Flex>

      <Flex justify="center">
        <Stepper current={step} labels={stepLabels} />
      </Flex>

      {step === 0 ? <ChannelStep onPick={pickChannel} /> : null}

      {step === 1 && channel ? (
        <ConfigureStep
          channel={channel}
          form={form}
          submitting={createMutation.isPending}
          onNext={handleConfigureNext}
        />
      ) : null}

      {step === 2 ? (
        <TeamStep
          query={memberQuery}
          onQueryChange={setMemberQuery}
          users={filteredUsers}
          loading={usersQuery.isLoading}
          totalUsers={usersQuery.data?.length ?? 0}
          selectedIds={selectedUserIds}
          onToggle={toggleUser}
          onInvite={() =>
            void navigate({
              to: "/users",
              search: {
                limit: 100,
                offset: 0,
                sortField: null,
                sortOrder: null,
                keyword: "",
                role: "",
              },
            })
          }
          onNext={() => membersMutation.mutate()}
          saving={membersMutation.isPending}
        />
      ) : null}

      {step === 3 ? (
        <AgentStep
          agents={(agentsQuery.data?.items ?? []).filter((a) => a.ownerId !== SYSTEM_OWNER_ID)}
          loading={agentsQuery.isLoading}
          selectedAgentId={selectedAgentId}
          onSelect={setSelectedAgentId}
          onCreateNew={() => void navigate({ to: "/agents/new" })}
          onComplete={() => agentMutation.mutate()}
          submitting={agentMutation.isPending}
        />
      ) : null}
    </Flex>
  );
}

/* ---------- Step 0: Channel ---------- */
function ChannelStep({ onPick }: { onPick: (c: WizardChannel) => void }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <Flex vertical align="center" gap={token.marginXL}>
      <Flex vertical align="center" gap={token.marginXXS} style={{ textAlign: "center" }}>
        <Title level={3} style={{ margin: 0 }}>
          {t("inboxes.wizardChannelTitle")}
        </Title>
        <Text type="secondary">{t("inboxes.wizardChannelSubtitle")}</Text>
      </Flex>
      <Flex gap={token.marginLG} wrap="wrap" justify="center" style={{ width: "100%" }}>
        {CHANNEL_CARDS.map((c) => {
          const Icon = c.icon;
          return (
            <Flex
              key={c.key}
              vertical
              align="center"
              gap={token.margin}
              style={{
                flex: "1 1 280px",
                maxWidth: 340,
                background: BRAND.cardBg,
                border: `1px solid ${BRAND.borderLow}`,
                borderRadius: token.borderRadiusLG,
                padding: token.paddingXL,
                textAlign: "center",
              }}
            >
              <Flex
                align="center"
                justify="center"
                style={{
                  width: 64,
                  height: 64,
                  borderRadius: token.borderRadiusLG,
                  background: c.bg,
                  color: c.fg,
                }}
              >
                <Icon size={30} />
              </Flex>
              <Title level={4} style={{ margin: 0 }}>
                {t(c.titleKey)}
              </Title>
              <Text type="secondary" style={{ minHeight: 44 }}>
                {t(c.descKey)}
              </Text>
              <Button type="primary" block size="large" onClick={() => onPick(c.key)}>
                {t(c.ctaKey)}
              </Button>
            </Flex>
          );
        })}
      </Flex>
    </Flex>
  );
}

/* ---------- Step 1: Configure ---------- */
function ConfigureStep({
  channel,
  form,
  submitting,
  onNext,
}: {
  channel: WizardChannel;
  form: ReturnType<typeof Form.useForm>[0];
  submitting: boolean;
  onNext: () => void;
}) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const visual =
    channel === "widget"
      ? { icon: Globe, bg: BRAND.primarySoft, fg: BRAND.primary }
      : channel === "telegram"
        ? { icon: Send, bg: BRAND.tertiarySoft, fg: BRAND.tertiary }
        : channel === "juggleim"
          ? { icon: MessageSquare, bg: BRAND.aiAccentSoft, fg: BRAND.aiAccent }
          : { icon: Phone, bg: "#E9F8EF", fg: "#168B4B" };
  const Icon = visual.icon;

  return (
    <Flex gap={token.marginLG} align="flex-start" wrap="wrap">
      <div style={{ flex: "1 1 520px", minWidth: 0 }}>
        <div
          style={{
            background: BRAND.cardBg,
            border: `1px solid ${BRAND.borderLow}`,
            borderRadius: token.borderRadiusLG,
            padding: token.paddingLG,
          }}
        >
          <Flex align="center" gap={token.margin} style={{ marginBottom: token.marginLG }}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 44,
                height: 44,
                borderRadius: token.borderRadiusLG,
                background: visual.bg,
                color: visual.fg,
              }}
            >
              <Icon size={22} />
            </Flex>
            <Flex vertical gap={2}>
              <Text strong style={{ fontSize: token.fontSizeLG }}>
                {t("inboxes.wizardConfigureCard")}
              </Text>
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {t("inboxes.wizardConfigureCardDesc")}
              </Text>
            </Flex>
          </Flex>

          <Form form={form} layout="vertical">
            <Form.Item
              name="name"
              label={t("inboxes.inboxName")}
              rules={[{ required: true, message: t("inboxes.inboxNameRequired") }]}
            >
              <Input placeholder={t("inboxes.websitePlaceholder")} />
            </Form.Item>

            {channel === "widget" ? (
              <Form.Item
                name="welcome_message"
                label={t("inboxes.welcomeMessage")}
                extra={t("inboxes.welcomeMessageExtra")}
              >
                <TextArea
                  rows={3}
                  maxLength={500}
                  showCount
                  placeholder={t("inboxes.welcomePlaceholder")}
                />
              </Form.Item>
            ) : channel === "whatsapp" ? (
              <>
                <Form.Item
                  name="phone_number_id"
                  label={t("inboxes.whatsappPhoneNumberId")}
                  rules={[{ required: true, message: t("inboxes.whatsappPhoneNumberIdRequired") }]}
                >
                  <Input placeholder="123456789012345" />
                </Form.Item>
                <Form.Item
                  name="access_token"
                  label={t("inboxes.whatsappAccessToken")}
                  rules={[{ required: true, message: t("inboxes.whatsappAccessTokenRequired") }]}
                >
                  <Input.Password />
                </Form.Item>
                <Form.Item
                  name="webhook_verify_token"
                  label={t("inboxes.whatsappVerifyToken")}
                  rules={[{ required: true, message: t("inboxes.whatsappVerifyTokenRequired") }]}
                >
                  <Input.Password />
                </Form.Item>
                <Form.Item name="app_secret" label={t("inboxes.whatsappAppSecret")}>
                  <Input.Password />
                </Form.Item>
                <Flex gap={token.marginSM} wrap="wrap">
                  <Form.Item name="api_base_url" label={t("inboxes.whatsappApiBaseUrl")} style={{ flex: 1, minWidth: 220 }}>
                    <Input placeholder="https://graph.facebook.com" />
                  </Form.Item>
                  <Form.Item name="api_version" label={t("inboxes.whatsappApiVersion")} style={{ width: 150 }}>
                    <Input placeholder="v24.0" />
                  </Form.Item>
                </Flex>
              </>
            ) : (
              <>
                <Form.Item
                  name="bot_name"
                  label={
                    channel === "telegram" ? t("inboxes.botName") : t("inboxes.juggleIMBotName")
                  }
                  rules={[
                    {
                      required: true,
                      message:
                        channel === "telegram"
                          ? t("inboxes.botNameRequired")
                          : t("inboxes.juggleIMBotNameRequired"),
                    },
                  ]}
                >
                  <Input placeholder="support_bot" />
                </Form.Item>
                <Form.Item
                  name="bot_token"
                  label={
                    channel === "telegram" ? t("inboxes.botToken") : t("inboxes.juggleIMBotToken")
                  }
                  rules={[
                    {
                      required: true,
                      message:
                        channel === "telegram"
                          ? t("inboxes.botTokenRequired")
                          : t("inboxes.juggleIMBotTokenRequired"),
                    },
                  ]}
                >
                  <Input.Password placeholder="123456:ABC-DEF..." />
                </Form.Item>
              </>
            )}
          </Form>
        </div>

        <Flex justify="flex-end" style={{ marginTop: token.marginLG }}>
          <Button
            type="primary"
            size="large"
            loading={submitting}
            onClick={onNext}
            icon={<ArrowRight size={18} />}
            iconPosition="end"
          >
            {t("inboxes.nextAddTeam")}
          </Button>
        </Flex>
      </div>

      <div style={{ flex: "0 1 320px", minWidth: 280 }}>
        <AsideCard tone="primary">
          <Flex align="center" gap={token.marginSM} style={{ marginBottom: token.margin }}>
            <Lightbulb size={18} color={BRAND.aiAccent} />
            <Text strong>{t("inboxes.configGuide")}</Text>
          </Flex>
          <Flex vertical gap={token.marginSM}>
            {[
              t("inboxes.configGuideStep1"),
              t("inboxes.configGuideStep2"),
              t("inboxes.configGuideStep3"),
            ].map((line, i) => (
              <Flex key={i} gap={token.marginSM}>
                <Text strong style={{ color: BRAND.primary }}>
                  {i + 1}.
                </Text>
                <Text type="secondary">{line}</Text>
              </Flex>
            ))}
          </Flex>
        </AsideCard>
      </div>
    </Flex>
  );
}

/* ---------- Step 2: Team ---------- */
function TeamStep({
  query,
  onQueryChange,
  users,
  loading,
  totalUsers,
  selectedIds,
  onToggle,
  onInvite,
  onNext,
  saving,
}: {
  query: string;
  onQueryChange: (v: string) => void;
  users: Array<z.infer<typeof UserSchema>>;
  loading: boolean;
  totalUsers: number;
  selectedIds: string[];
  onToggle: (id: string) => void;
  onInvite: () => void;
  onNext: () => void;
  saving: boolean;
}) {
  const { t } = useTranslation();
  const { token } = theme.useToken();

  return (
    <Flex gap={token.marginLG} align="flex-start" wrap="wrap">
      <div style={{ flex: "1 1 560px", minWidth: 0 }}>
        <Flex vertical gap={token.marginXXS} style={{ marginBottom: token.marginLG }}>
          <Title level={3} style={{ margin: 0 }}>
            {t("inboxes.assignTeamTitle")}
          </Title>
          <Text type="secondary">{t("inboxes.assignTeamSubtitle")}</Text>
        </Flex>

        <Flex gap={token.marginSM} style={{ marginBottom: token.margin }}>
          <Input
            allowClear
            prefix={<Search size={16} style={{ color: BRAND.outline }} />}
            placeholder={t("inboxes.searchPlaceholder")}
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
          />
          <Button icon={<UserPlus size={16} />} onClick={onInvite}>
            {t("inboxes.inviteNewMember")}
          </Button>
        </Flex>

        <div
          style={{
            border: `1px solid ${BRAND.borderLow}`,
            borderRadius: token.borderRadiusLG,
            overflow: "hidden",
            background: BRAND.cardBg,
          }}
        >
          {users.map((u, index) => {
            const checked = selectedIds.includes(u.id);
            const isAdmin = u.roles.includes("admin");
            return (
              <Flex
                key={u.id}
                align="center"
                gap={token.margin}
                onClick={() => onToggle(u.id)}
                style={{
                  padding: token.padding,
                  cursor: "pointer",
                  borderTop: index === 0 ? "none" : `1px solid ${BRAND.borderLow}`,
                  background: checked ? BRAND.subtleBg : BRAND.cardBg,
                }}
              >
                <Flex
                  align="center"
                  justify="center"
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: "50%",
                    background: BRAND.secondaryContainer,
                    color: BRAND.secondary,
                    fontWeight: 600,
                    flex: "0 0 auto",
                  }}
                >
                  {u.username?.[0]?.toUpperCase()}
                </Flex>
                <Flex vertical gap={2} style={{ flex: 1, minWidth: 0 }}>
                  <Text strong ellipsis>
                    {u.username}
                  </Text>
                  <Text type="secondary" ellipsis style={{ fontSize: token.fontSizeSM }}>
                    {u.email || "—"} · {isAdmin ? t("users.role.admin") : t("users.role.customer")}
                  </Text>
                </Flex>
                <Checkbox checked={checked} />
              </Flex>
            );
          })}
          {!loading && users.length === 0 ? (
            <Flex justify="center" style={{ padding: token.paddingLG }}>
              <Text type="secondary">{t("common.noData")}</Text>
            </Flex>
          ) : null}
        </div>

        <Flex justify="space-between" style={{ marginTop: token.margin }}>
          <Text strong style={{ color: BRAND.primary }}>
            {t("inboxes.membersSelected", { count: selectedIds.length })}
          </Text>
          <Text type="secondary">{t("inboxes.totalAgentsAvailable", { count: totalUsers })}</Text>
        </Flex>

        <Flex
          align="center"
          justify="flex-end"
          gap={token.margin}
          style={{ marginTop: token.marginLG }}
        >
          {selectedIds.length === 0 ? (
            <Text style={{ color: BRAND.warning }}>{t("inboxes.selectAtLeastOne")}</Text>
          ) : null}
          <Button
            type="primary"
            size="large"
            disabled={selectedIds.length === 0}
            loading={saving}
            onClick={onNext}
            icon={<ArrowRight size={18} />}
            iconPosition="end"
          >
            {t("inboxes.nextAddAgent")}
          </Button>
        </Flex>
      </div>

      <Flex vertical gap={token.margin} style={{ flex: "0 1 320px", minWidth: 280 }}>
        <AsideCard>
          <Flex align="center" gap={token.marginSM} style={{ marginBottom: token.margin }}>
            <Info size={18} color={BRAND.primary} />
            <Text strong style={{ color: BRAND.primary }}>
              {t("inboxes.understandingRoles")}
            </Text>
          </Flex>
          <Flex vertical gap={token.margin}>
            <Flex vertical gap={2}>
              <Text strong>{t("users.role.admin")}</Text>
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {t("inboxes.roleManagerDesc")}
              </Text>
            </Flex>
            <Flex vertical gap={2}>
              <Text strong>{t("users.role.customer")}</Text>
              <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {t("inboxes.roleAgentDesc")}
              </Text>
            </Flex>
          </Flex>
        </AsideCard>
        <AsideCard tone="primary">
          <Text strong style={{ color: BRAND.primary }}>
            {t("inboxes.proTip")}
          </Text>
          <Text type="secondary" style={{ display: "block", marginTop: token.marginXS }}>
            {t("inboxes.proTipDesc")}
          </Text>
        </AsideCard>
      </Flex>
    </Flex>
  );
}

/* ---------- Step 3: Agent ---------- */
function AgentStep({
  agents,
  loading,
  selectedAgentId,
  onSelect,
  onCreateNew,
  onComplete,
  submitting,
}: {
  agents: AgentListItem[];
  loading: boolean;
  selectedAgentId: string | null;
  onSelect: (id: string) => void;
  onCreateNew: () => void;
  onComplete: () => void;
  submitting: boolean;
}) {
  const { t } = useTranslation();
  const { token } = theme.useToken();

  return (
    <Flex gap={token.marginLG} align="flex-start" wrap="wrap">
      <div style={{ flex: "1 1 560px", minWidth: 0 }}>
        <Flex vertical gap={token.marginXXS} style={{ marginBottom: token.marginLG }}>
          <Title level={3} style={{ margin: 0 }}>
            {t("inboxes.associateAgentTitle")}
          </Title>
          <Text type="secondary">{t("inboxes.associateAgentSubtitle")}</Text>
        </Flex>

        <Flex gap={token.margin} wrap="wrap">
          {/* 新建智能体卡片（虚线，跳转 Agent Builder） */}
          <Flex
            vertical
            align="center"
            justify="center"
            gap={token.marginXS}
            onClick={onCreateNew}
            style={{
              flex: "1 1 260px",
              maxWidth: 360,
              minHeight: 180,
              cursor: "pointer",
              borderRadius: token.borderRadiusLG,
              border: `1.5px dashed ${BRAND.primary}`,
              background: BRAND.primarySoft,
              padding: token.paddingLG,
              textAlign: "center",
            }}
          >
            <Flex
              align="center"
              justify="center"
              style={{
                width: 48,
                height: 48,
                borderRadius: "50%",
                background: BRAND.cardBg,
                color: BRAND.primary,
              }}
            >
              <Plus size={24} />
            </Flex>
            <Text strong style={{ color: BRAND.primary }}>
              {t("inboxes.createNewAgent")}
            </Text>
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {t("inboxes.createNewAgentDesc")}
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
                onClick={() => ready && onSelect(a.agentId)}
                style={{
                  flex: "1 1 260px",
                  maxWidth: 360,
                  minHeight: 180,
                  cursor: ready ? "pointer" : "not-allowed",
                  opacity: ready ? 1 : 0.65,
                  borderRadius: token.borderRadiusLG,
                  border: `1.5px solid ${selected ? BRAND.primary : BRAND.borderLow}`,
                  background: BRAND.cardBg,
                  padding: token.paddingLG,
                }}
              >
                <Flex align="flex-start" justify="space-between">
                  <Flex
                    align="center"
                    justify="center"
                    style={{
                      width: 44,
                      height: 44,
                      borderRadius: token.borderRadiusLG,
                      background: BRAND.primarySoft,
                      color: BRAND.primary,
                    }}
                  >
                    <Bot size={22} />
                  </Flex>
                  <Tag
                    color={ready ? "green" : "default"}
                    style={{ borderRadius: 9999, marginInlineEnd: 0 }}
                  >
                    {ready ? t("inboxes.agentReady") : t("inboxes.agentDrafting")}
                  </Tag>
                </Flex>
                <Text strong style={{ fontSize: token.fontSizeLG }}>
                  {a.agentName}
                </Text>
                <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
                  {a.primaryModel || "—"}
                </Text>
              </Flex>
            );
          })}

          {!loading && agents.length === 0 ? (
            <Flex align="center" style={{ minHeight: 180, flex: "1 1 260px", maxWidth: 360 }}>
              <Text type="secondary">{t("inboxes.noAgents")}</Text>
            </Flex>
          ) : null}
        </Flex>

        <Flex align="center" justify="flex-end" style={{ marginTop: token.marginXL }}>
          <Button
            type="primary"
            size="large"
            disabled={!selectedAgentId}
            loading={submitting}
            onClick={onComplete}
            icon={<Sparkles size={18} />}
          >
            {t("inboxes.completeSetup")}
          </Button>
        </Flex>
      </div>

      <Flex vertical gap={token.margin} style={{ flex: "0 1 320px", minWidth: 280 }}>
        <AsideCard>
          <Flex align="center" gap={token.marginSM} style={{ marginBottom: token.margin }}>
            <Lightbulb size={18} color={BRAND.aiAccent} />
            <Text strong>{t("inboxes.howItWorks")}</Text>
          </Flex>
          <Flex vertical gap={token.marginSM}>
            {[
              t("inboxes.howItWorksStep1"),
              t("inboxes.howItWorksStep2"),
              t("inboxes.howItWorksStep3"),
            ].map((line, i) => (
              <Flex key={i} gap={token.marginSM}>
                <Text strong style={{ color: BRAND.primary }}>
                  {i + 1}.
                </Text>
                <Text type="secondary">{line}</Text>
              </Flex>
            ))}
          </Flex>
        </AsideCard>
        <AsideCard tone="muted">
          <Text
            style={{
              fontSize: 11,
              fontWeight: 600,
              letterSpacing: 0.5,
              textTransform: "uppercase",
              color: BRAND.outline,
            }}
          >
            {t("inboxes.automationLevel")}
          </Text>
          <div style={{ marginTop: token.marginSM }}>
            <Text type="secondary" style={{ fontSize: token.fontSizeSM }}>
              {t("inboxes.agentBehavior")}
            </Text>
            <Select
              style={{ width: "100%", marginTop: token.marginXXS }}
              defaultValue="suggest"
              options={[
                { value: "suggest", label: t("inboxes.behaviorSuggestions") },
                { value: "draft", label: t("inboxes.behaviorDraft") },
                { value: "auto", label: t("inboxes.behaviorFullAuto") },
              ]}
            />
          </div>
        </AsideCard>
      </Flex>
    </Flex>
  );
}
