import { createFileRoute, stripSearchParams, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Button,
  ConfigProvider,
  Empty,
  Flex,
  Input,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  theme,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import {
  ChevronLeft,
  ChevronRight,
  Globe,
  MessageSquare,
  Phone,
  Plus,
  Send,
  Settings,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMemo, useState } from "react";
import { z } from "zod/v4";
import { useTranslation } from "react-i18next";
import { BRAND } from "@/utils/brandColors";
import { inboxChannelLabel, inboxSubtitle, listInboxes, type Inbox } from "@/api/inbox";

const { Text, Title } = Typography;

const InboxSearchParamsSchema = z.object({
  limit: z.number().int().positive().catch(100),
  offset: z.number().int().nonnegative().catch(0),
});

type InboxSearch = z.infer<typeof InboxSearchParamsSchema>;

/** 搜索参数默认值：与之相等的参数不会写入 URL（stripSearchParams），保持地址整洁。 */
const INBOX_SEARCH_DEFAULTS: InboxSearch = { limit: 100, offset: 0 };

export const Route = createFileRoute("/_auth/inboxes/")({
  validateSearch: (search: Record<string, unknown>): InboxSearch =>
    InboxSearchParamsSchema.parse(search),
  search: { middlewares: [stripSearchParams(INBOX_SEARCH_DEFAULTS)] },
  component: InboxesPage,
});

/** 渠道图标底色映射：网站=蓝、Telegram=青、JuggleIM=紫，对齐设计稿的彩色圆角图标。 */
const CHANNEL_VISUAL: Record<Inbox["channel_type"], { icon: LucideIcon; bg: string; fg: string }> =
  {
    widget: { icon: Globe, bg: BRAND.primarySoft, fg: BRAND.primary },
    telegram: { icon: Send, bg: BRAND.tertiarySoft, fg: BRAND.tertiary },
    juggleim: { icon: MessageSquare, bg: BRAND.aiAccentSoft, fg: BRAND.aiAccent },
    whatsapp: { icon: Phone, bg: "#E9F8EF", fg: "#168B4B" },
  };

/**
 * 格式化 Unix 时间戳为本地日期。
 * @param value 秒或毫秒级时间戳（后端 created_time）
 * @param locale 当前语言，用于日期本地化
 */
function formatInboxDate(value: number, locale: string): string {
  if (!value) return "—";
  const ms = value < 1e12 ? value * 1000 : value;
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(new Date(ms));
}

function InboxesPage() {
  const { t, i18n } = useTranslation();
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const { token } = theme.useToken();
  const [query, setQuery] = useState("");

  const inboxQuery = useQuery({
    queryKey: ["inboxes", search.limit, search.offset],
    queryFn: () => listInboxes({ limit: search.limit, offset: search.offset }),
  });

  const inboxes = inboxQuery.data?.list ?? [];
  const filteredInboxes = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return inboxes;
    return inboxes.filter((inbox) => {
      if (inbox.name.toLowerCase().includes(q)) return true;
      if (inbox.channel_type === "widget") {
        return inbox.channel_conf.welcome_message.toLowerCase().includes(q);
      }
      if (inbox.channel_type === "whatsapp") {
        return (
          inbox.channel_conf.display_phone_number.toLowerCase().includes(q) ||
          inbox.channel_conf.phone_number_id.toLowerCase().includes(q)
        );
      }
      return inbox.channel_conf.bot_name.toLowerCase().includes(q);
    });
  }, [inboxes, query]);

  const th = (label: string) => (
    <span
      style={{
        textTransform: "uppercase",
        letterSpacing: 0.6,
        fontSize: 12,
        fontWeight: 700,
        color: BRAND.outline,
      }}
    >
      {label}
    </span>
  );

  const columns: ColumnsType<Inbox> = [
    {
      title: th(t("inboxes.colChannel")),
      dataIndex: "name",
      key: "name",
      render: (_, record) => {
        const visual = CHANNEL_VISUAL[record.channel_type];
        const Icon = visual.icon;
        return (
          <Flex align="center" gap={token.margin}>
            <Flex
              align="center"
              justify="center"
              style={{
                width: 40,
                height: 40,
                borderRadius: token.borderRadiusLG,
                background: visual.bg,
                color: visual.fg,
                flex: "0 0 auto",
              }}
            >
              <Icon size={20} aria-hidden />
            </Flex>
            <Flex vertical style={{ minWidth: 0 }}>
              <Text strong ellipsis style={{ color: BRAND.onSurface }}>
                {record.name}
              </Text>
              <Text type="secondary" ellipsis style={{ fontSize: token.fontSizeSM }}>
                {inboxSubtitle(record)}
              </Text>
            </Flex>
          </Flex>
        );
      },
    },
    {
      title: th(t("inboxes.colType")),
      dataIndex: "channel_type",
      key: "channel_type",
      width: 160,
      render: (channelType: Inbox["channel_type"]) => (
        <Tag
          color={
            channelType === "widget"
              ? "blue"
              : channelType === "juggleim"
                ? "purple"
                : channelType === "whatsapp"
                  ? "green"
                  : "cyan"
          }
          style={{ borderRadius: 9999, paddingInline: 10 }}
        >
          {inboxChannelLabel(channelType)}
        </Tag>
      ),
    },
    {
      title: th(t("inboxes.representatives")),
      dataIndex: "member_count",
      key: "member_count",
      width: 160,
      render: (count: number) => (
        <Space size={6} style={{ color: BRAND.onSurface }}>
          <Users size={15} aria-hidden style={{ color: BRAND.outline }} />
          <span style={{ fontWeight: 500 }}>{count}</span>
        </Space>
      ),
    },
    {
      title: th(t("inboxes.colCreated")),
      dataIndex: "created_time",
      key: "created_time",
      width: 160,
      render: (value: number) => (
        <span style={{ color: BRAND.onSurfaceVariant }}>
          {formatInboxDate(value, i18n.language)}
        </span>
      ),
    },
    {
      title: th(t("common.actions")),
      key: "actions",
      width: 100,
      align: "right",
      render: (_, record) => (
        <Tooltip title={t("inboxes.settings")}>
          <Button
            type="text"
            onClick={() =>
              void navigate({ to: "/inboxes/$inboxId/settings", params: { inboxId: record.id } })
            }
            icon={<Settings size={18} aria-hidden />}
            aria-label={t("inboxes.settings")}
          />
        </Tooltip>
      ),
    },
  ];

  /* TIPS: 分页基于后端 total 与 URL 中的 limit/offset；搜索为当前页客户端过滤，搜索态下禁用翻页。 */
  const total = inboxQuery.data?.total ?? 0;
  const isSearching = query.trim().length > 0;
  const rangeStart = filteredInboxes.length ? search.offset + 1 : 0;
  const rangeEnd = search.offset + filteredInboxes.length;
  const rangeTotal = isSearching ? filteredInboxes.length : total;
  const prevDisabled = isSearching || search.offset <= 0;
  const nextDisabled = isSearching || search.offset + search.limit >= total;

  const goToOffset = (nextOffset: number) => {
    void navigate({ search: { ...search, offset: Math.max(0, nextOffset) } });
  };

  return (
    <Flex vertical gap={token.marginLG} style={{ minHeight: 0 }}>
      <Flex justify="space-between" align="flex-end" gap={token.margin} wrap="wrap">
        <Flex vertical gap={token.marginXXS} style={{ minWidth: 0 }}>
          <Title level={3} style={{ margin: 0 }}>
            {t("inboxes.connectedTitle")}
          </Title>
          <Text type="secondary">{t("inboxes.connectedSubtitle")}</Text>
        </Flex>
        <Flex gap={token.marginSM} align="center" wrap="wrap">
          <Input.Search
            allowClear
            placeholder={t("inboxes.searchPlaceholder")}
            style={{ width: 260 }}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <Button
            type="primary"
            icon={<Plus size={18} aria-hidden />}
            onClick={() => void navigate({ to: "/inboxes/new" })}
          >
            {t("inboxes.addInbox")}
          </Button>
        </Flex>
      </Flex>

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
        <div
          style={{
            background: BRAND.cardBg,
            border: `1px solid ${BRAND.borderLow}`,
            borderRadius: token.borderRadiusLG,
            overflow: "hidden",
          }}
        >
          <Table<Inbox>
            rowKey="id"
            columns={columns}
            dataSource={filteredInboxes}
            loading={inboxQuery.isLoading}
            pagination={false}
            scroll={{ x: "max-content" }}
            locale={{
              emptyText: (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("inboxes.empty")} />
              ),
            }}
          />
        </div>
      </ConfigProvider>

      <Flex
        align="center"
        justify="space-between"
        gap={token.marginMD}
        wrap="wrap"
        style={{ borderTop: `1px solid ${BRAND.borderLow}`, paddingTop: token.paddingMD }}
      >
        <Text type="secondary">
          {t("inboxes.showingChannels", { start: rangeStart, end: rangeEnd, total: rangeTotal })}
        </Text>
        <Space size={token.marginXS}>
          <Button
            icon={<ChevronLeft size={18} aria-hidden />}
            disabled={prevDisabled}
            onClick={() => goToOffset(search.offset - search.limit)}
            aria-label={t("common.back")}
          />
          <Button
            icon={<ChevronRight size={18} aria-hidden />}
            disabled={nextDisabled}
            onClick={() => goToOffset(search.offset + search.limit)}
            aria-label={t("common.actions")}
          />
        </Space>
      </Flex>
    </Flex>
  );
}
