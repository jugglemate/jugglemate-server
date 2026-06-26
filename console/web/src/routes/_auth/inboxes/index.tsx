import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  App,
  Button,
  Card,
  Empty,
  Flex,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Steps,
  Table,
  Tag,
  Typography,
  theme,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import { CheckCircle2, Inbox as InboxIcon, Plus, Send, Users } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { z } from "zod/v4";
import {
  createTelegramInbox,
  listInboxMembers,
  listInboxes,
  replaceInboxMembers,
  type CreateTelegramInboxRequest,
  type Inbox,
} from "@/api/inbox";
import { USER_ENDPOINTS } from "@/api/user";
import {
  CreateTelegramInboxRequestSchema,
  PaginatedResponseSchema,
  UserSchema,
} from "@/api/schemas";
import { httpClient } from "@/utils/http";

const { Text, Title } = Typography;

const InboxSearchParamsSchema = z.object({
  limit: z.number().int().positive().catch(100),
  offset: z.number().int().nonnegative().catch(0),
});

export const Route = createFileRoute("/_auth/inboxes/")({
  validateSearch: (search) => InboxSearchParamsSchema.parse(search),
  component: InboxesPage,
});

const UsersListResponseSchema = PaginatedResponseSchema(UserSchema);

async function listUsersForMembers() {
  const raw = await httpClient.get(USER_ENDPOINTS.list, {
    params: {
      limit: 500,
      offset: 0,
    },
  });
  return UsersListResponseSchema.shape.data.parse(raw).list;
}

type FlowStep = 0 | 1 | 2;

function InboxesPage() {
  const search = Route.useSearch();
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const { token } = theme.useToken();
  const [query, setQuery] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [step, setStep] = useState<FlowStep>(0);
  const [activeInbox, setActiveInbox] = useState<Inbox | null>(null);
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([]);
  const [form] = Form.useForm<CreateTelegramInboxRequest>();

  const inboxQuery = useQuery({
    queryKey: ["inboxes", search.limit, search.offset],
    queryFn: () => listInboxes({ limit: search.limit, offset: search.offset }),
  });

  const usersQuery = useQuery({
    queryKey: ["inbox-member-users"],
    queryFn: listUsersForMembers,
    staleTime: 60_000,
  });

  const membersQuery = useQuery({
    queryKey: ["inbox-members", activeInbox?.id],
    queryFn: () => listInboxMembers(activeInbox?.id ?? ""),
    enabled: modalOpen && step === 1 && Boolean(activeInbox),
  });

  const createMutation = useMutation({
    mutationFn: (values: CreateTelegramInboxRequest) =>
      createTelegramInbox(CreateTelegramInboxRequestSchema.parse(values)),
    onSuccess: (inbox) => {
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      setActiveInbox(inbox);
      setSelectedUserIds([]);
      setStep(1);
      message.success("Telegram inbox created");
    },
    onError: () => {
      message.error("Failed to create Telegram inbox");
    },
  });

  const membersMutation = useMutation({
    mutationFn: () => replaceInboxMembers(activeInbox?.id ?? "", selectedUserIds),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["inboxes"] });
      void queryClient.invalidateQueries({ queryKey: ["inbox-members", activeInbox?.id] });
      setStep(2);
      message.success("Representatives saved");
    },
    onError: () => {
      message.error("Failed to save representatives");
    },
  });

  const inboxes = inboxQuery.data?.list ?? [];
  const filteredInboxes = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return inboxes;
    return inboxes.filter(
      (inbox) =>
        inbox.name.toLowerCase().includes(q) ||
        inbox.channel_conf.bot_name.toLowerCase().includes(q),
    );
  }, [inboxes, query]);

  const userOptions = useMemo(
    () =>
      (usersQuery.data ?? []).map((user) => ({
        value: user.id,
        label: `${user.username}${user.email ? ` (${user.email})` : ""}`,
      })),
    [usersQuery.data],
  );

  const openCreate = () => {
    form.resetFields();
    setActiveInbox(null);
    setSelectedUserIds([]);
    setStep(0);
    setModalOpen(true);
  };

  const openMembers = (inbox: Inbox) => {
    setActiveInbox(inbox);
    setSelectedUserIds([]);
    setStep(1);
    setModalOpen(true);
  };

  useEffect(() => {
    if (membersQuery.isSuccess) {
      setSelectedUserIds(membersQuery.data.map((member) => member.id));
    }
  }, [membersQuery.data, membersQuery.isSuccess]);

  const closeModal = () => {
    setModalOpen(false);
    setStep(0);
    setActiveInbox(null);
    setSelectedUserIds([]);
    form.resetFields();
  };

  const columns: ColumnsType<Inbox> = [
    {
      title: "Inbox",
      dataIndex: "name",
      key: "name",
      render: (_, record) => (
        <Flex align="center" gap={token.marginSM}>
          <Flex
            align="center"
            justify="center"
            style={{
              width: 36,
              height: 36,
              borderRadius: token.borderRadius,
              background: token.colorFillTertiary,
              color: token.colorTextSecondary,
              flex: "0 0 auto",
            }}
          >
            <Send size={18} aria-hidden />
          </Flex>
          <Flex vertical style={{ minWidth: 0 }}>
            <Text strong ellipsis>
              {record.name}
            </Text>
            <Text type="secondary" ellipsis>
              @{record.channel_conf.bot_name || "telegram"}
            </Text>
          </Flex>
        </Flex>
      ),
    },
    {
      title: "Channel",
      dataIndex: "channel_type",
      key: "channel_type",
      render: () => <Tag color="blue">Telegram</Tag>,
    },
    {
      title: "Representatives",
      dataIndex: "member_count",
      key: "member_count",
      width: 160,
      render: (count: number) => (
        <Space size={token.marginXS}>
          <Users size={14} aria-hidden />
          <span>{count}</span>
        </Space>
      ),
    },
    {
      title: "Actions",
      key: "actions",
      width: 160,
      render: (_, record) => (
        <Button onClick={() => openMembers(record)} icon={<Users size={14} aria-hidden />}>
          Members
        </Button>
      ),
    },
  ];

  return (
    <Flex vertical gap={token.marginLG} style={{ minHeight: 0 }}>
      <Flex justify="space-between" align="flex-start" gap={token.marginMD} wrap="wrap">
        <Flex vertical gap={token.marginXXS}>
          <Title level={3} style={{ margin: 0 }}>
            Inboxes
          </Title>
          <Text type="secondary">Create Telegram inboxes and assign customer representatives.</Text>
        </Flex>
        <Button type="primary" icon={<Plus size={16} aria-hidden />} onClick={openCreate}>
          New Inbox
        </Button>
      </Flex>

      <Card styles={{ body: { padding: token.paddingLG } }}>
        <Flex vertical gap={token.marginMD}>
          <Input.Search
            allowClear
            placeholder="Search inboxes"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            style={{ maxWidth: 360 }}
          />
          <Table<Inbox>
            rowKey="id"
            columns={columns}
            dataSource={filteredInboxes}
            loading={inboxQuery.isLoading}
            pagination={false}
            locale={{
              emptyText: (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No Telegram inboxes yet" />
              ),
            }}
          />
        </Flex>
      </Card>

      <Modal
        open={modalOpen}
        title={step === 0 ? "Create Telegram Inbox" : "Inbox Representatives"}
        onCancel={closeModal}
        footer={null}
        destroyOnHidden
      >
        <Flex vertical gap={token.marginLG}>
          <Steps
            size="small"
            current={step}
            items={[{ title: "Telegram" }, { title: "Representatives" }, { title: "Finish" }]}
          />

          {step === 0 ? (
            <Form<CreateTelegramInboxRequest>
              form={form}
              layout="vertical"
              onFinish={(values) => createMutation.mutate(values)}
            >
              <Form.Item
                name="name"
                label="Inbox name"
                rules={[{ required: true, message: "Please enter inbox name" }]}
              >
                <Input
                  prefix={<InboxIcon size={14} aria-hidden />}
                  placeholder="Telegram Support"
                />
              </Form.Item>
              <Form.Item
                name="bot_name"
                label="Telegram bot name"
                rules={[{ required: true, message: "Please enter bot name" }]}
              >
                <Input placeholder="support_bot" />
              </Form.Item>
              <Form.Item
                name="bot_token"
                label="Telegram bot token"
                rules={[{ required: true, message: "Please enter bot token" }]}
              >
                <Input.Password placeholder="123456:ABC-DEF..." />
              </Form.Item>
              <Flex justify="flex-end" gap={token.marginSM}>
                <Button onClick={closeModal}>Cancel</Button>
                <Button type="primary" htmlType="submit" loading={createMutation.isPending}>
                  Create and continue
                </Button>
              </Flex>
            </Form>
          ) : null}

          {step === 1 ? (
            <Flex vertical gap={token.marginMD}>
              <Text type="secondary">
                Select users who should handle conversations for {activeInbox?.name ?? "this inbox"}
                .
              </Text>
              <Select
                mode="multiple"
                showSearch
                placeholder="Select representatives"
                value={selectedUserIds}
                onChange={setSelectedUserIds}
                loading={usersQuery.isLoading || membersQuery.isLoading}
                options={userOptions}
                optionFilterProp="label"
              />
              <Flex justify="flex-end" gap={token.marginSM}>
                <Button onClick={closeModal}>Cancel</Button>
                <Button
                  type="primary"
                  loading={membersMutation.isPending}
                  onClick={() => membersMutation.mutate()}
                >
                  Save representatives
                </Button>
              </Flex>
            </Flex>
          ) : null}

          {step === 2 ? (
            <Flex vertical align="center" gap={token.marginMD} style={{ textAlign: "center" }}>
              <CheckCircle2 size={48} color={token.colorSuccess} strokeWidth={1.5} aria-hidden />
              <Flex vertical gap={token.marginXXS}>
                <Text strong>{activeInbox?.name ?? "Telegram inbox"} is ready</Text>
                <Text type="secondary">
                  Your Telegram inbox and representatives have been saved.
                </Text>
              </Flex>
              <Button type="primary" onClick={closeModal}>
                Back to inboxes
              </Button>
            </Flex>
          ) : null}
        </Flex>
      </Modal>
    </Flex>
  );
}
