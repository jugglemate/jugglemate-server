import {
  Layout,
  Button,
  Space,
  theme,
  Breadcrumb,
  Flex,
  Divider,
  Grid,
  Avatar,
  Dropdown,
  Badge,
} from "antd";
import type { ItemType } from "antd/es/breadcrumb/Breadcrumb";
import type { MenuProps } from "antd";
import { useSettingsStore } from "@/stores/settings";
import { useAuthStore } from "@/stores/auth";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import { Bell, LogOut, PanelLeft } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Theme } from "@/components/Icon";
import { LanguageSwitcher } from "@/components/Layout/LanguageSwitcher";
import { APP_BRAND_NAME } from "@/utils/constants";

const { Header: AntHeader } = Layout;

const PATH_LABEL_KEY: Record<string, string> = {
  "/dashboard": "menu.dashboard",
  "/users": "menu.users",
  "/inboxes": "menu.inboxes",
  "/agents": "menu.agents",
  "/tools": "menu.tools",
  "/models": "menu.models",
  "/403": "errors.forbiddenTitle",
};

function normalizePath(pathname: string): string {
  if (pathname === "/") return pathname;
  return pathname.replace(/\/+$/, "") || "/";
}

export type HeaderProps = {
  /** When `false`, breadcrumb is hidden; header actions stay right-aligned. */
  showBreadcrumb?: boolean;
};

export function Header({ showBreadcrumb = true }: HeaderProps) {
  const { t } = useTranslation();
  const toggleSidebar = useSettingsStore((s) => s.toggleSidebar);
  const toggleDarkMode = useSettingsStore((s) => s.toggleDarkMode);
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;

  /** 顶部右侧用户菜单（头像下拉），提供退出登录入口。 */
  const userMenuItems: MenuProps["items"] = [
    {
      key: "logout",
      icon: <LogOut size={token.fontSize} />,
      label: t("userMenu.signOut"),
      onClick: () => {
        logout();
        void navigate({ to: "/login" });
      },
    },
  ];

  const path = normalizePath(location.pathname);
  const segments = path.split("/").filter(Boolean);
  const firstSegmentPath = segments.length ? `/${segments[0]}` : "/dashboard";
  const onDashboard = path === "/dashboard" || path === "/";
  const leafLabelKey = onDashboard
    ? "menu.dashboard"
    : (PATH_LABEL_KEY[firstSegmentPath] ?? segments[0] ?? "menu.dashboard");
  const leafLabel = t(leafLabelKey);

  /* TIPS: 面包屑对齐设计稿 "JuggleMate › Team" —— 品牌名弱化可点击返回仪表盘，
     当前页用主色加粗并带下划线强调。 */
  const breadcrumbItems: ItemType[] = [
    {
      title: (
        <Link to="/dashboard" style={{ color: token.colorTextSecondary, fontWeight: 500 }}>
          {APP_BRAND_NAME}
        </Link>
      ),
    },
    {
      title: (
        <span
          style={{
            color: token.colorPrimary,
            fontWeight: 700,
            borderBottom: `2px solid ${token.colorPrimary}`,
            paddingBottom: 2,
          }}
        >
          {leafLabel}
        </span>
      ),
    },
  ];

  return (
    <AntHeader
      style={{
        background: token.colorBgContainer,
        borderBottom: `1px solid ${token.colorBorderSecondary}`,
        padding: `0 ${token.paddingLG}px`,
        gap: token.sizeLG,
        display: "flex",
        alignItems: "center",
      }}
    >
      <Flex align="center" flex={1} gap={token.marginXS} style={{ minWidth: 0 }}>
        {isMobile ? (
          <Button
            type="text"
            size="small"
            onClick={toggleSidebar}
            icon={<PanelLeft size={token.size} />}
            aria-label={t("common.toggleSidebar")}
          />
        ) : null}
        {showBreadcrumb ? <Breadcrumb separator="›" items={breadcrumbItems} /> : null}
      </Flex>
      <Space size={token.marginXS} align="center">
        <LanguageSwitcher />
        <Badge dot offset={[-4, 4]} color={token.colorError}>
          <Button
            type="text"
            icon={<Bell size={token.size} />}
            aria-label={t("common.notifications")}
          />
        </Badge>
        <Button
          type="text"
          onClick={toggleDarkMode}
          icon={<Theme size={token.size} />}
          aria-label={t("common.toggleTheme")}
        />
        {user ? (
          <>
            <Divider vertical />
            <Dropdown menu={{ items: userMenuItems }} trigger={["click"]} placement="bottomRight">
              <Flex
                align="center"
                gap={token.marginXS}
                style={{ cursor: "pointer", paddingInline: token.paddingXXS }}
              >
                <Avatar size={32} src={(user.avatar ?? "").trim() || undefined} shape="circle">
                  {user.username?.[0]?.toUpperCase()}
                </Avatar>
                {!isMobile ? (
                  <span
                    style={{
                      maxWidth: 120,
                      fontWeight: 500,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {user.username}
                  </span>
                ) : null}
              </Flex>
            </Dropdown>
          </>
        ) : null}
      </Space>
    </AntHeader>
  );
}
