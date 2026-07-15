import { createFileRoute, useNavigate } from "@tanstack/react-router";
import {
  App,
  Button,
  Dropdown,
  Flex,
  Segmented,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  theme,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Bot,
  LayoutGrid,
  List as ListIcon,
  MoreVertical,
  Pause,
  Pencil,
  Play,
  Plus,
  PlayCircle,
  Trash2,
} from "lucide-react";
import { agentApi } from "@/utils/agentHttp";
import { BRAND } from "@/utils/brandColors";

const { Title, Text } = Typography;

export const Route = createFileRoute("/_auth/agents/")({
  component: AgentsPage,
});

interface AgentListItem {
  agentId: string;
  agentName: string;
  primaryModel?: string;
  status: string;
  knowledgeCount?: number;
  toolCount?: number;
}

interface PagedResponse<T> {
  items: T[];
  total: number;
}

/** 状态色映射：启用=绿、暂停=橙、草稿=灰，用于卡片左边框与状态点。 */
const STATUS_TONE: Record<string, string> = {
  active: BRAND.success,
  paused: BRAND.warning,
  draft: BRAND.outline,
};

function AgentsPage() {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<AgentListItem[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [view, setView] = useState<"grid" | "list">("grid");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agentApi.get<PagedResponse<AgentListItem>>("/agents", {
        params: { page: 1, page_size: 100 },
      });
      setItems(data?.items ?? []);
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.loadFailed"));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const lifecycle = async (record: AgentListItem, action: "activate" | "pause") => {
    try {
      await agentApi.post(`/agents/${record.agentId}/${action}`);
      message.success(t("common.done"));
      await load();
    } catch (err) {
      message.error((err as Error).message || t("agentAdmin.actionFailed"));
    }
  };

  const remove = (record: AgentListItem) => {
    modal.confirm({
      title: t("crud.deleteConfirmTitle"),
      content: record.agentName,
      okText: t("common.delete"),
      okType: "danger",
      cancelText: t("common.cancel"),
      onOk: async () => {
        await agentApi.post("/agents/delete", { agentId: record.agentId });
        message.success(t("common.deleted"));
        await load();
      },
    });
  };

  const openEdit = (r: AgentListItem) =>
    void navigate({ to: "/agents/$agentId", params: { agentId: r.agentId } });

  const filtered = useMemo(
    () => (statusFilter ? items.filter((i) => i.status === statusFilter) : items),
    [items, statusFilter],
  );

  const listColumns: ColumnsType<AgentListItem> = [
    {
      title: t("agents.colName"),
      dataIndex: "agentName",
      render: (v: string, r) => {
        const tone = STATUS_TONE[r.status] ?? BRAND.outline;
        return (
          <Flex align="center" gap={token.marginSM}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 32,
                height: 32,
                borderRadius: token.borderRadiusLG,
                background: BRAND.primarySoft,
                color: BRAND.primary,
                flex: "0 0 auto",
              }}
            >
              <Bot size={16} />
            </Flex>
            <Flex vertical gap={0} style={{ minWidth: 0 }}>
              <Text strong ellipsis>
                {v}
              </Text>
              <Flex align="center" gap={5}>
                <span
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    background: tone,
                    display: "inline-block",
                  }}
                />
                <span style={{ fontSize: 11, textTransform: "uppercase", color: BRAND.outline }}>
                  {r.status}
                </span>
              </Flex>
            </Flex>
          </Flex>
        );
      },
    },
    { title: t("agents.colModel"), dataIndex: "primaryModel", render: (v: string) => v || "--" },
    {
      title: t("agents.knowledgeCount"),
      dataIndex: "knowledgeCount",
      width: 110,
      render: (v: number) => v ?? 0,
    },
    {
      title: t("agents.toolCount"),
      dataIndex: "toolCount",
      width: 90,
      render: (v: number) => v ?? 0,
    },
    {
      title: t("common.actions"),
      key: "actions",
      width: 120,
      align: "right",
      render: (_, r) => (
        <Space size={2}>
          <Button
            type="text"
            size="small"
            icon={<Pencil size={16} />}
            aria-label={t("common.edit")}
            onClick={() => openEdit(r)}
          />
          {r.status === "active" ? (
            <Button
              type="text"
              size="small"
              icon={<Pause size={16} />}
              aria-label={t("agents.pause")}
              onClick={() => void lifecycle(r, "pause")}
            />
          ) : (
            <Button
              type="text"
              size="small"
              icon={<Play size={16} />}
              aria-label={t("agents.activate")}
              onClick={() => void lifecycle(r, "activate")}
            />
          )}
          <Button
            type="text"
            size="small"
            danger
            icon={<Trash2 size={16} />}
            aria-label={t("common.delete")}
            onClick={() => remove(r)}
          />
        </Space>
      ),
    },
  ];

  const renderCard = (r: AgentListItem) => {
    const tone = STATUS_TONE[r.status] ?? BRAND.outline;
    return (
      <div
        key={r.agentId}
        style={{
          flex: "1 1 320px",
          maxWidth: 400,
          background: BRAND.cardBg,
          border: `1px solid ${BRAND.borderLow}`,
          borderLeft: `4px solid ${tone}`,
          borderRadius: token.borderRadiusLG,
          padding: token.paddingLG,
          display: "flex",
          flexDirection: "column",
          gap: token.margin,
        }}
      >
        <Flex align="flex-start" justify="space-between">
          <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 40,
                height: 40,
                borderRadius: token.borderRadiusLG,
                background: BRAND.primarySoft,
                color: BRAND.primary,
                flex: "0 0 auto",
              }}
            >
              <Bot size={20} />
            </Flex>
            <Flex vertical gap={2} style={{ minWidth: 0 }}>
              <Text strong ellipsis style={{ fontSize: token.fontSizeLG }}>
                {r.agentName}
              </Text>
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
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 600,
                    letterSpacing: 0.5,
                    textTransform: "uppercase",
                    color: BRAND.outline,
                  }}
                >
                  {r.status}
                </span>
              </Flex>
            </Flex>
          </Flex>
          <Dropdown
            menu={{
              items: [
                {
                  key: "edit",
                  icon: <Pencil size={token.fontSize} />,
                  label: t("common.edit"),
                  onClick: () => openEdit(r),
                },
                r.status === "active"
                  ? {
                      key: "pause",
                      icon: <Pause size={token.fontSize} />,
                      label: t("agents.pause"),
                      onClick: () => void lifecycle(r, "pause"),
                    }
                  : {
                      key: "activate",
                      icon: <Play size={token.fontSize} />,
                      label: t("agents.activate"),
                      onClick: () => void lifecycle(r, "activate"),
                    },
                {
                  key: "delete",
                  icon: <Trash2 size={token.fontSize} />,
                  label: t("common.delete"),
                  danger: true,
                  onClick: () => remove(r),
                },
              ],
            }}
            placement="bottomRight"
          >
            <Button type="text" size="small" icon={<MoreVertical size={token.fontSize} />} />
          </Dropdown>
        </Flex>

        <Flex align="center" justify="space-between">
          <Text type="secondary">{t("agents.colModel")}</Text>
          <Tag style={{ marginInlineEnd: 0, borderRadius: token.borderRadiusSM }}>
            {r.primaryModel || "--"}
          </Tag>
        </Flex>

        <Flex
          align="center"
          gap={token.marginLG}
          style={{ borderTop: `1px solid ${BRAND.borderLow}`, paddingTop: token.margin }}
        >
          <Flex vertical gap={2}>
            <Text
              style={{
                fontSize: 11,
                fontWeight: 600,
                letterSpacing: 0.5,
                textTransform: "uppercase",
                color: BRAND.outline,
              }}
            >
              {t("agents.knowledgeCount")}
            </Text>
            <Text strong style={{ fontSize: token.fontSizeXL }}>
              {r.knowledgeCount ?? 0}
            </Text>
          </Flex>
          <Flex vertical gap={2}>
            <Text
              style={{
                fontSize: 11,
                fontWeight: 600,
                letterSpacing: 0.5,
                textTransform: "uppercase",
                color: BRAND.outline,
              }}
            >
              {t("agents.toolCount")}
            </Text>
            <Text strong style={{ fontSize: token.fontSizeXL }}>
              {r.toolCount ?? 0}
            </Text>
          </Flex>
        </Flex>

        <Flex gap={token.marginSM}>
          <Button block icon={<Pencil size={15} />} onClick={() => openEdit(r)}>
            {t("common.edit")}
          </Button>
          <Button block icon={<PlayCircle size={15} />} onClick={() => openEdit(r)}>
            {t("agents.test")}
          </Button>
        </Flex>
      </div>
    );
  };

  return (
    <Flex vertical gap={token.marginLG} style={{ maxWidth: 1200, width: "100%", margin: "0 auto" }}>
      <Flex justify="space-between" align="flex-start" gap={token.marginMD} wrap="wrap">
        <Flex vertical gap={token.marginXXS}>
          <Title level={3} style={{ margin: 0 }}>
            {t("agents.listTitle")}
          </Title>
          <Text type="secondary">{t("agents.listSubtitle")}</Text>
        </Flex>
        <Flex align="center" gap={token.marginSM}>
          <Segmented
            value={view}
            onChange={(v) => setView(v as "grid" | "list")}
            options={[
              { value: "grid", icon: <LayoutGrid size={16} />, label: t("agents.viewGrid") },
              { value: "list", icon: <ListIcon size={16} />, label: t("agents.viewList") },
            ]}
          />
          <Button
            type="primary"
            icon={<Plus size={18} />}
            onClick={() => void navigate({ to: "/agents/new" })}
          >
            {t("agents.addAgent")}
          </Button>
        </Flex>
      </Flex>

      <Flex align="center" gap={token.margin} wrap="wrap">
        <Select
          value={statusFilter || undefined}
          onChange={(v) => setStatusFilter(v ?? "")}
          allowClear
          placeholder={t("agents.statusAll")}
          style={{ width: 180 }}
          options={[
            { value: "active", label: t("agents.colStatus") + ": active" },
            { value: "paused", label: t("agents.colStatus") + ": paused" },
            { value: "draft", label: t("agents.colStatus") + ": draft" },
          ]}
        />
        <Text type="secondary">{t("agents.showingAgents", { count: filtered.length })}</Text>
      </Flex>

      {loading ? (
        <Flex justify="center" style={{ padding: 80 }}>
          <Spin />
        </Flex>
      ) : view === "list" ? (
        <div
          style={{
            background: BRAND.cardBg,
            border: `1px solid ${BRAND.borderLow}`,
            borderRadius: token.borderRadiusLG,
            overflow: "hidden",
          }}
        >
          <Table<AgentListItem>
            rowKey="agentId"
            columns={listColumns}
            dataSource={filtered}
            pagination={{ pageSize: 12, hideOnSinglePage: true }}
            scroll={{ x: "max-content" }}
          />
        </div>
      ) : (
        <Flex gap={token.marginLG} wrap="wrap" align="stretch">
          {filtered.map(renderCard)}

          {/* 新建智能体虚线卡片 */}
          <Flex
            vertical
            align="center"
            justify="center"
            gap={token.marginXS}
            onClick={() => void navigate({ to: "/agents/new" })}
            style={{
              flex: "1 1 320px",
              maxWidth: 400,
              minHeight: 240,
              cursor: "pointer",
              borderRadius: token.borderRadiusLG,
              border: `1.5px dashed ${BRAND.borderLow}`,
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
              <Plus size={26} />
            </Flex>
            <Text strong style={{ fontSize: token.fontSizeLG }}>
              {t("agents.createNewAgentTitle")}
            </Text>
            <Text type="secondary" style={{ maxWidth: 260 }}>
              {t("agents.createNewAgentDesc")}
            </Text>
          </Flex>
        </Flex>
      )}

      {!loading && filtered.length === 0 ? (
        <Flex justify="center" style={{ padding: 24 }}>
          <Text type="secondary">{t("agents.empty")}</Text>
        </Flex>
      ) : null}
    </Flex>
  );
}
