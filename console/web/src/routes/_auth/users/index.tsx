import { createFileRoute, stripSearchParams, useNavigate } from "@tanstack/react-router";
import {
  Avatar,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Flex,
  Form,
  App,
  Dropdown,
  Row,
  Space,
  theme,
} from "antd";
import type { TablePaginationConfig } from "antd/es/table/interface";
import { useMemo, useState } from "react";
import { httpClient } from "@/utils/http";
import { USER_ENDPOINTS } from "@/api/user";
import {
  PaginatedResponseSchema,
  UserSchema,
  CreateUserRequestSchema,
  rolesToFormRole,
  toCreateUserApiBody,
  toUpdateUserApiBody,
} from "@/api/schemas";
import type { User, CreateUserRequest } from "@/api/schemas";
import { z } from "zod/v4";
import { MoreVertical, Pencil, ShieldCheck, Trash2, UserRound, UsersRound } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { DataTable } from "@/components/DataTable";
import { useResourceCRUD } from "@/hooks/useResourceCRUD";
import { useCrudToasts } from "@/hooks/useCrudToasts";
import { useUrlSearchState } from "@/hooks/useUrlSearchState";
import { useTranslation } from "react-i18next";
import { BRAND } from "@/utils/brandColors";
import { Toolbar } from "./-Toolbar";
import { MemberDrawer } from "./-MemberDrawer";

const UserSearchParamsSchema = z.object({
  limit: z.number().int().positive().catch(100),
  offset: z.number().int().nonnegative().catch(0),
  sortField: z.string().nullable().catch(null),
  sortOrder: z.enum(["ascend", "descend"]).nullable().catch(null),
  keyword: z.string().catch(""),
  role: z.string().catch(""),
});

type UserSearch = z.infer<typeof UserSearchParamsSchema>;

/** 搜索参数默认值：与之相等的参数不会写入 URL，保持地址整洁。 */
const USER_SEARCH_DEFAULTS: UserSearch = {
  limit: 100,
  offset: 0,
  sortField: null,
  sortOrder: null,
  keyword: "",
  role: "",
};

export const Route = createFileRoute("/_auth/users/")({
  validateSearch: (search: Record<string, unknown>): UserSearch =>
    UserSearchParamsSchema.parse(search),
  search: { middlewares: [stripSearchParams(USER_SEARCH_DEFAULTS)] },
  component: TeamMembersPage,
});

const paginatedUserSchema = PaginatedResponseSchema(UserSchema);

type UserUpdateInput = CreateUserRequest & { id: string; roles: string[] };

/** 表头单元格：大写 + 字间距 + 弱化灰，对齐设计稿。 */
function Th({ label }: { label: string }) {
  return (
    <span
      style={{
        textTransform: "uppercase",
        letterSpacing: 0.6,
        fontSize: 12,
        fontWeight: 600,
        color: BRAND.outline,
      }}
    >
      {label}
    </span>
  );
}

/**
 * 角色胶囊：管理员=主色蓝，客户=灰。对齐设计稿的圆角标签样式。
 * @param role 角色标识（admin / customer 等）
 * @param label 角色显示文案
 */
function RolePill({ role, label }: { role: string; label: string }) {
  const isAdmin = role === "admin";
  const bg = isAdmin ? BRAND.primarySoft : BRAND.surfaceVariant;
  const fg = isAdmin ? BRAND.primary : BRAND.onSurfaceVariant;
  const border = isAdmin ? "rgba(0, 91, 178, 0.2)" : BRAND.statCardBorder;
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        padding: "2px 10px",
        borderRadius: 9999,
        fontSize: 12,
        fontWeight: 500,
        background: bg,
        color: fg,
        border: `1px solid ${border}`,
      }}
    >
      {label}
    </span>
  );
}

/**
 * 底部统计卡片
 * @param icon 卡片图标
 * @param label 指标名称
 * @param value 指标数值
 * @param tone 色调（primary/success/warning）
 */
function StatCard({
  icon: Icon,
  label,
  value,
  tone,
}: {
  icon: LucideIcon;
  label: string;
  value: number | string;
  tone: "primary" | "success" | "warning";
}) {
  const { token } = theme.useToken();
  const toneMap = {
    primary: { bg: BRAND.primarySoft, fg: BRAND.primary },
    success: { bg: BRAND.tertiarySoft, fg: BRAND.tertiary },
    warning: { bg: BRAND.warningSoft, fg: BRAND.warning },
  } as const;
  const colors = toneMap[tone];

  return (
    <Card
      variant="borderless"
      styles={{ body: { padding: token.padding } }}
      style={{ background: BRAND.statCardBg, border: `1px solid ${BRAND.statCardBorder}` }}
    >
      <Flex align="center" gap={token.margin}>
        <Flex
          align="center"
          justify="center"
          style={{
            width: 48,
            height: 48,
            borderRadius: "50%",
            flexShrink: 0,
            background: colors.bg,
            color: colors.fg,
          }}
        >
          <Icon size={22} />
        </Flex>
        <Flex vertical gap={2} style={{ minWidth: 0 }}>
          <span
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: BRAND.outline,
              textTransform: "uppercase",
              letterSpacing: 0.5,
            }}
          >
            {label}
          </span>
          <span style={{ fontSize: 20, fontWeight: 600, color: BRAND.onSurface }}>{value}</span>
        </Flex>
      </Flex>
    </Card>
  );
}

function TeamMembersPage() {
  const { t } = useTranslation();
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const { message, modal } = App.useApp();
  const { token } = theme.useToken();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [form] = Form.useForm();
  const currentPage = Math.floor(search.offset / search.limit) + 1;

  const setSearch = (next: UserSearch) => {
    void navigate({ search: next });
  };

  const { keywordInput, setKeywordInput, applyKeyword } = useUrlSearchState({
    search,
    setSearch,
  });

  const crudToasts = useCrudToasts<CreateUserRequest, UserUpdateInput>({
    message,
    resourceKey: "users",
  });

  const closeDrawer = () => {
    setDrawerOpen(false);
    setEditingUser(null);
    form.resetFields();
  };

  const { data, isLoading, createMutation, updateMutation, deleteMutation } = useResourceCRUD<
    { list: User[]; total: number },
    CreateUserRequest,
    UserUpdateInput
  >({
    queryKey: [
      "users",
      search.limit,
      search.offset,
      search.keyword,
      search.role,
      search.sortField,
      search.sortOrder,
    ],
    queryFn: () =>
      httpClient.get(USER_ENDPOINTS.list, {
        params: {
          limit: search.limit,
          offset: search.offset,
          keyword: search.keyword || undefined,
          role: search.role || undefined,
          sortField: search.sortField ?? undefined,
          sortOrder: search.sortOrder ?? undefined,
        },
      }),
    select: (raw) => paginatedUserSchema.shape.data.parse(raw),
    createFn: (values) =>
      httpClient.post(
        USER_ENDPOINTS.create,
        toCreateUserApiBody(CreateUserRequestSchema.parse(values)),
      ),
    updateFn: ({ id, username, email, role }) =>
      httpClient.put(USER_ENDPOINTS.update(id), toUpdateUserApiBody({ username, email, role })),
    deleteFn: (id) => httpClient.delete(USER_ENDPOINTS.delete(id)),
    optimistic: { update: true, delete: true },
    createLifecycle: {
      onSuccess: (values) => {
        crudToasts.createLifecycle?.onSuccess?.(values);
        closeDrawer();
      },
      onError: crudToasts.createLifecycle?.onError,
    },
    updateLifecycle: {
      onMutate: crudToasts.updateLifecycle?.onMutate,
      onSuccess: (values) => {
        crudToasts.updateLifecycle?.onSuccess?.(values);
        closeDrawer();
      },
      onError: crudToasts.updateLifecycle?.onError,
    },
    deleteLifecycle: {
      onMutate: crudToasts.deleteLifecycle?.onMutate,
      onSuccess: crudToasts.deleteLifecycle?.onSuccess,
      onError: crudToasts.deleteLifecycle?.onError,
    },
  });

  const confirmDelete = (record: User) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: t("crud.deleteConfirmContent"),
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: () => {
        deleteMutation.mutate(record.id);
        closeDrawer();
      },
    });
  };

  const openEdit = (record: User) => {
    setEditingUser(record);
    form.setFieldsValue({
      username: record.username,
      email: record.email,
      role: rolesToFormRole(record.roles),
    });
    setDrawerOpen(true);
  };

  const columns = [
    {
      title: <Th label={t("users.member")} />,
      dataIndex: "username",
      key: "username",
      sorter: true,
      sortOrder: search.sortField === "username" ? search.sortOrder : null,
      render: (_: unknown, record: User) => {
        const src = (record.avatar ?? "").trim() || undefined;
        return (
          <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
            {/* TIPS: 头像右下角绿色在线圆点，对齐设计稿的在线状态标识 */}
            <Badge
              dot
              color={BRAND.success}
              offset={[-4, 34]}
              styles={{
                indicator: { width: 10, height: 10, boxShadow: `0 0 0 2px ${BRAND.cardBg}` },
              }}
            >
              <Avatar size={40} src={src} shape="circle" style={{ flexShrink: 0 }}>
                {record.username?.[0]?.toUpperCase()}
              </Avatar>
            </Badge>
            <span
              style={{
                fontWeight: 600,
                color: BRAND.onSurface,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {record.username}
            </span>
          </Flex>
        );
      },
    },
    {
      title: <Th label={t("users.email")} />,
      dataIndex: "email",
      key: "email",
      sorter: true,
      sortOrder: search.sortField === "email" ? search.sortOrder : null,
      render: (email: string | null) => (
        <span style={{ color: BRAND.onSurfaceVariant }}>{email || "—"}</span>
      ),
    },
    {
      title: <Th label={t("users.roles")} />,
      dataIndex: "roles",
      key: "roles",
      sorter: true,
      sortOrder: search.sortField === "roles" ? search.sortOrder : null,
      render: (roles: string[]) => (
        <Space wrap size={4}>
          {roles.map((role) => (
            <RolePill
              key={role}
              role={role}
              label={role === "admin" ? t("users.role.admin") : t("users.role.customer")}
            />
          ))}
        </Space>
      ),
    },
    {
      title: <Th label={t("users.status")} />,
      key: "status",
      sorter: false,
      /* TIPS: 后端暂无成员状态字段，现有账号均视为"已激活"；后续接入邀请流程后再按真实状态渲染。 */
      render: () => (
        <Flex align="center" gap={6}>
          <span
            style={{
              width: 7,
              height: 7,
              borderRadius: "50%",
              background: BRAND.success,
              display: "inline-block",
            }}
          />
          <span style={{ color: BRAND.success, fontWeight: 500 }}>{t("users.statusActive")}</span>
        </Flex>
      ),
    },
    {
      title: <Th label={t("common.actions")} />,
      key: "actions",
      width: 72,
      align: "right" as const,
      sorter: false,
      render: (_: unknown, record: User) => (
        <Dropdown
          menu={{
            items: [
              {
                key: "edit",
                icon: <Pencil size={token.fontSize} />,
                label: t("common.edit"),
                onClick: () => openEdit(record),
              },
              {
                key: "delete",
                icon: <Trash2 size={token.fontSize} />,
                label: t("common.delete"),
                danger: true,
                onClick: () => confirmDelete(record),
              },
            ],
          }}
          placement="bottomRight"
        >
          <Button
            type="text"
            icon={<MoreVertical size={token.fontSize} />}
            aria-label={t("errors.rowActions")}
          />
        </Dropdown>
      ),
    },
  ];

  const showPagination = (data?.total ?? 0) > search.limit;

  const tablePagination: false | TablePaginationConfig = useMemo(
    () =>
      showPagination
        ? {
            total: data?.total ?? 0,
            current: currentPage,
            pageSize: search.limit,
            showSizeChanger: true,
            showTotal: (total) => t("common.rows", { count: total }),
            onChange: (page, pageSize) => {
              void navigate({
                search: {
                  ...search,
                  limit: pageSize ?? search.limit,
                  offset: (page - 1) * (pageSize ?? search.limit),
                },
              });
            },
          }
        : false,
    [showPagination, data?.total, currentPage, search, navigate, t],
  );

  /* TIPS: 角色统计基于当前已加载的列表（默认 limit=100），小团队场景下与全量一致。 */
  const roleStats = useMemo(() => {
    const list = data?.list ?? [];
    const admins = list.filter((u) => u.roles.includes("admin")).length;
    return { admins, customers: list.length - admins };
  }, [data?.list]);

  return (
    <Flex vertical gap={token.marginLG}>
      <Toolbar
        keywordInput={keywordInput}
        onKeywordChange={setKeywordInput}
        onSearch={applyKeyword}
        onClearSearch={() => {
          setKeywordInput("");
          setSearch({ ...search, keyword: "", offset: 0 });
        }}
        roleValue={search.role || undefined}
        onRoleChange={(role) =>
          void navigate({
            search: {
              ...search,
              role,
              offset: 0,
            },
          })
        }
        onCreateClick={() => {
          setEditingUser(null);
          form.resetFields();
          form.setFieldsValue({ role: "customer" });
          setDrawerOpen(true);
        }}
      />

      {/* TIPS: 表格外层用普通 div（本页为自然滚动布局），并通过 ConfigProvider 注入
          设计稿的表头浅底、大写灰字、行悬浮底等 Table token。 */}
      <ConfigProvider
        theme={{
          components: {
            Table: {
              headerBg: BRAND.subtleBg,
              headerColor: BRAND.outline,
              headerSplitColor: "transparent",
              borderColor: BRAND.borderLow,
              rowHoverBg: BRAND.subtleBg,
              cellPaddingBlock: 14,
              cellPaddingInline: 20,
            },
          },
        }}
      >
        <div>
          <DataTable<User>
            rowKey="id"
            size="middle"
            columns={columns}
            dataSource={data?.list ?? []}
            loading={isLoading}
            pagination={tablePagination}
            scroll={{ x: "max-content" }}
            onChange={(_pagination, _filters, sorter) => {
              if (Array.isArray(sorter)) return;
              const nextSortField = sorter.order ? String(sorter.field) : "username";
              const nextSortOrder = sorter.order ? sorter.order : "descend";
              void navigate({
                search: {
                  ...search,
                  sortField: nextSortField,
                  sortOrder: nextSortOrder,
                },
              });
            }}
          />
        </div>
      </ConfigProvider>

      <Row gutter={[token.margin, token.margin]}>
        <Col xs={24} md={8}>
          <StatCard
            icon={UsersRound}
            label={t("users.totalMembers")}
            value={data?.total ?? 0}
            tone="primary"
          />
        </Col>
        <Col xs={24} md={8}>
          <StatCard
            icon={ShieldCheck}
            label={t("users.adminCount")}
            value={roleStats.admins}
            tone="success"
          />
        </Col>
        <Col xs={24} md={8}>
          <StatCard
            icon={UserRound}
            label={t("users.customerCount")}
            value={roleStats.customers}
            tone="warning"
          />
        </Col>
      </Row>

      <MemberDrawer
        open={drawerOpen}
        editingUser={editingUser}
        form={form}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        onCancel={closeDrawer}
        onFinish={(values) => {
          const parsed = CreateUserRequestSchema.parse(values);
          if (editingUser) {
            updateMutation.mutate({
              id: editingUser.id,
              ...parsed,
              roles: [parsed.role],
            });
          } else {
            createMutation.mutate(parsed);
          }
        }}
        onRemove={confirmDelete}
      />
    </Flex>
  );
}
