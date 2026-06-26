import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Avatar, Button, Space, Form, App, Dropdown, theme, Tag, Flex } from "antd";
import type { TablePaginationConfig } from "antd/es/table/interface";
import { useMemo, useRef, useState } from "react";
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
import { MoreVertical, Pencil, Trash2 } from "lucide-react";
import { DataTable } from "@/components/DataTable";
import { useResourceCRUD } from "@/hooks/useResourceCRUD";
import { useTableFitHeight } from "@/hooks/useTableFitHeight";
import { useCrudToasts } from "@/hooks/useCrudToasts";
import { useUrlSearchState } from "@/hooks/useUrlSearchState";
import { useTranslation } from "react-i18next";
import { Toolbar } from "./-Toolbar";
import { FormModal } from "./-FormModal";

const UserSearchParamsSchema = z.object({
  limit: z.number().int().positive().catch(100),
  offset: z.number().int().nonnegative().catch(0),
  sortField: z.string().nullable().catch(null),
  sortOrder: z.enum(["ascend", "descend"]).nullable().catch(null),
  keyword: z.string().catch(""),
  role: z.string().catch(""),
});

type UserSearch = z.infer<typeof UserSearchParamsSchema>;

export const Route = createFileRoute("/_auth/users/")({
  validateSearch: (search) => UserSearchParamsSchema.parse(search),
  component: UsersPage,
});

const paginatedUserSchema = PaginatedResponseSchema(UserSchema);

type UserUpdateInput = CreateUserRequest & { id: string; roles: string[] };

function UsersPage() {
  const { t } = useTranslation();
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const { message, modal } = App.useApp();
  const { token } = theme.useToken();
  const [modalOpen, setModalOpen] = useState(false);
  const pageShellRef = useRef<HTMLDivElement>(null);
  const toolbarRowRef = useRef<HTMLDivElement>(null);
  const middleSectionRef = useRef<HTMLDivElement>(null);
  const tableFrameRef = useRef<HTMLDivElement>(null);
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
        setModalOpen(false);
        form.resetFields();
      },
      onError: crudToasts.createLifecycle?.onError,
    },
    updateLifecycle: {
      onMutate: crudToasts.updateLifecycle?.onMutate,
      onSuccess: (values) => {
        crudToasts.updateLifecycle?.onSuccess?.(values);
        setModalOpen(false);
        setEditingUser(null);
        form.resetFields();
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
      onOk: () => deleteMutation.mutate(record.id),
    });
  };

  const columns = [
    {
      title: t("users.id"),
      dataIndex: "id",
      key: "id",
      sorter: true,
      sortOrder: search.sortField === "id" ? search.sortOrder : null,
    },
    {
      title: t("users.username"),
      dataIndex: "username",
      key: "username",
      sorter: true,
      sortOrder: search.sortField === "username" ? search.sortOrder : null,
      render: (_: unknown, record: User) => {
        const src = (record.avatar ?? "").trim() || undefined;
        return (
          <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
            <Avatar size={24} src={src} shape="circle" style={{ flexShrink: 0 }}>
              {record.username?.[0]?.toUpperCase()}
            </Avatar>
            <span
              style={{
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
      title: t("users.email"),
      dataIndex: "email",
      key: "email",
      sorter: true,
      sortOrder: search.sortField === "email" ? search.sortOrder : null,
    },
    {
      title: t("users.roles"),
      dataIndex: "roles",
      key: "roles",
      sorter: true,
      sortOrder: search.sortField === "roles" ? search.sortOrder : null,
      render: (roles: string[]) => (
        <Space wrap>
          {roles.map((role) => (
            <Tag
              key={role}
              variant="outlined"
              styles={{
                root: {
                  borderRadius: 9999,
                  background: "transparent",
                  boxShadow: "none",
                },
              }}
            >
              {role === "admin" ? t("users.role.admin") : t("users.role.customer")}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t("common.actions"),
      key: "actions",
      width: 60,
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
                onClick: () => {
                  setEditingUser(record);
                  form.setFieldsValue({
                    username: record.username,
                    email: record.email,
                    role: rolesToFormRole(record.roles),
                  });
                  setModalOpen(true);
                },
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

  const { tableAreaMaxHeight, tableScrollY, lockScrollHeight } = useTableFitHeight({
    pageShellRef,
    toolbarRef: toolbarRowRef,
    middleRef: middleSectionRef,
    tableFrameRef,
    marginLG: token.marginLG,
    rowCount: data?.list?.length ?? 0,
    isLoading,
    showPagination,
  });

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
    [showPagination, data?.total, currentPage, search, navigate],
  );

  return (
    <Flex
      ref={pageShellRef}
      vertical
      gap={token.marginMD}
      style={{ flex: "1 1 0%", minHeight: 0, overflow: "hidden" }}
    >
      <Toolbar
        ref={toolbarRowRef}
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
          setModalOpen(true);
        }}
      />

      <DataTable<User>
        layoutRef={middleSectionRef}
        frameRef={tableFrameRef}
        lockScrollHeight={lockScrollHeight}
        maxHeight={tableAreaMaxHeight}
        frameHeight={
          tableScrollY != null && tableAreaMaxHeight != null ? tableAreaMaxHeight : undefined
        }
        rowKey="id"
        size="small"
        columns={columns}
        dataSource={data?.list ?? []}
        loading={isLoading}
        pagination={tablePagination}
        style={{ flex: 1, minHeight: 0 }}
        scroll={tableScrollY != null ? { x: "max-content", y: tableScrollY } : { x: "max-content" }}
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

      <FormModal
        open={modalOpen}
        editingUser={editingUser}
        form={form}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        onCancel={() => {
          setModalOpen(false);
          setEditingUser(null);
          form.resetFields();
        }}
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
      />
    </Flex>
  );
}
