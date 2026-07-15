import { Menu, Layout, theme, Flex, Grid, Drawer, Button, ConfigProvider } from "antd";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useLocation } from "@tanstack/react-router";
import {
  Book,
  Bot,
  Briefcase,
  CircleDashed,
  Cpu,
  Folder,
  Home,
  Inbox,
  PanelLeft,
  SlidersHorizontal,
  Star,
  User,
  Users,
  Wrench,
  Zap,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { APP_BRAND_NAME } from "@/utils/constants";
import { BRAND } from "@/utils/brandColors";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import type { MenuItem as MenuItemType } from "@/api/schemas";
import type { MenuProps } from "antd";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import "./index.css";

const { Sider } = Layout;
/** API menu `name` → i18n keys for known legacy English names. */
const MENU_LABEL_KEYS: Record<string, string> = {
  Platform: "menu.platform",
  Projects: "menu.projects",
  Settings: "menu.settings",
  Dashboard: "menu.dashboard",
  Users: "menu.users",
  Inboxes: "menu.inboxes",
  "Design Engineering": "menu.projects",
  "Sales & Marketing": "menu.projects",
};

function resolveMenuLabel(name: string, t: TFunction): string {
  if (name.includes(".")) {
    return t(name);
  }
  const key = MENU_LABEL_KEYS[name];
  return key ? t(key) : name;
}

type AntMenuItem = Required<MenuProps>["items"][number];
type BuildMenuResult = {
  items: AntMenuItem[];
  keyToPath: Record<string, string>;
  pathToKeyChain: Record<string, string[]>;
};

const MENU_ICON_MAP: Record<string, LucideIcon> = {
  IconLucideLayoutDashboard: Home,
  IconLucideInbox: Inbox,
  IconLucideUsers: User,
  IconLucideUserList: Users,
  /** Back-compat for older menu payloads still using IconLucideHistory */
  IconLucideHistory: Users,
  IconLucideStar: Star,
  IconLucideSettings: SlidersHorizontal,
  IconLucideBriefcase: Briefcase,
  IconLucideBookOpen: Book,
  IconLucideFolderKanban: Folder,
  IconLucideSparkles: Zap,
  IconLucideBot: Bot,
  IconLucideWrench: Wrench,
  IconLucideCpu: Cpu,
};

function renderMenuIcon(icon: string | null, size = 18) {
  const Icon = (icon && MENU_ICON_MAP[icon]) || CircleDashed;
  return <Icon size={size} />;
}

function buildMenuItems(
  menus: MenuItemType[],
  token: ReturnType<typeof theme.useToken>["token"],
  t: TFunction,
  collapsed = false,
  iconSize = 18,
  parentKeys: string[] = [],
): BuildMenuResult {
  const sorted = menus.filter((m) => !m.hidden).sort((a, b) => a.sort - b.sort);
  const keyToPath: Record<string, string> = {};
  const pathToKeyChain: Record<string, string[]> = {};
  const items: AntMenuItem[] = [];

  for (const menu of sorted) {
    const label = resolveMenuLabel(menu.name, t);
    const key = menu.id;

    if (menu.kind === "group") {
      const built = buildMenuItems(menu.children, token, t, collapsed, iconSize, parentKeys);
      Object.assign(keyToPath, built.keyToPath);
      Object.assign(pathToKeyChain, built.pathToKeyChain);
      if (built.items.length > 0) {
        items.push({
          type: "group",
          key,
          label: collapsed ? (
            <span
              style={{
                width: "100%",
                display: "inline-flex",
                justifyContent: "center",
              }}
            >
              <span
                aria-label={String(label)}
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: "50%",
                  background: BRAND.sidebarTextMuted,
                  display: "inline-block",
                }}
              />
            </span>
          ) : (
            <span
              style={{
                fontSize: token.fontSizeSM,
                color: BRAND.sidebarTextMuted,
              }}
            >
              {label}
            </span>
          ),
          children: built.items,
        });
      }
      continue;
    }

    const nextParents = [...parentKeys, key];
    keyToPath[key] = menu.path;
    const existing = pathToKeyChain[menu.path];
    if (!existing || nextParents.length > existing.length) {
      pathToKeyChain[menu.path] = nextParents;
    }

    let children: AntMenuItem[] | undefined;
    if (menu.children?.length) {
      const built = buildMenuItems(menu.children, token, t, collapsed, iconSize, nextParents);
      Object.assign(keyToPath, built.keyToPath);
      Object.assign(pathToKeyChain, built.pathToKeyChain);
      children = built.items.length ? built.items : undefined;
    }

    items.push({
      key,
      label,
      icon: renderMenuIcon(menu.icon, iconSize),
      children,
    });
  }

  return { items, keyToPath, pathToKeyChain };
}

export function Sidebar() {
  const { t } = useTranslation();
  const menus = useAuthStore((s) => s.menus);
  const collapsed = useSettingsStore((s) => s.sidebarCollapsed);
  const setSidebarCollapsed = useSettingsStore((s) => s.setSidebarCollapsed);
  const toggleSidebar = useSettingsStore((s) => s.toggleSidebar);
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const isMobile = !screens.lg;
  const mobileOpen = collapsed;
  const builtMenu = useMemo(
    () => buildMenuItems(menus, token, t, !isMobile && collapsed, 18),
    [menus, token, t, isMobile, collapsed],
  );
  const { selectedKey, routeOpenKeys } = useMemo(() => {
    const chain = builtMenu.pathToKeyChain[location.pathname] ?? [];
    return { selectedKey: chain.at(-1), routeOpenKeys: chain.slice(0, -1) };
  }, [builtMenu, location.pathname]);
  const routeOpenKeysSig = routeOpenKeys.join("\0");
  const [openKeys, setOpenKeys] = useState<string[]>([]);

  useEffect(() => {
    if (isMobile) {
      setSidebarCollapsed(false);
    }
  }, [isMobile, setSidebarCollapsed]);

  /** Merge open keys required by the route with any submenu the user already expanded. */
  useEffect(() => {
    setOpenKeys((prev) => {
      const next = new Set(prev);
      for (const k of routeOpenKeys) next.add(k);
      return [...next];
    });
  }, [location.pathname, routeOpenKeysSig]);

  /* TIPS: 侧边栏采用固定深色导航（对齐设计稿），与主内容区的浅/深主题解耦。
     Menu 的深色文字对比度通过嵌套 dark 算法的 ConfigProvider 保证，选中项渲染为品牌蓝胶囊。 */
  const brandBlock = (isCollapsed: boolean) => (
    <Flex
      align="center"
      gap={token.marginSM}
      style={{
        minHeight: 40,
        paddingInline: isCollapsed ? 0 : token.paddingXXS,
        justifyContent: isCollapsed ? "center" : "flex-start",
        width: "100%",
        minWidth: 0,
      }}
    >
      <Flex
        align="center"
        justify="center"
        style={{
          width: 36,
          height: 36,
          borderRadius: token.borderRadiusLG,
          background: BRAND.primary,
          color: "#ffffff",
          flexShrink: 0,
        }}
      >
        <Bot size={20} />
      </Flex>
      {!isCollapsed ? (
        <span
          style={{
            fontWeight: 700,
            fontSize: token.fontSizeLG,
            color: BRAND.sidebarText,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            minWidth: 0,
          }}
        >
          {APP_BRAND_NAME}
        </span>
      ) : null}
    </Flex>
  );

  const sidebarContent = (isCollapsed: boolean, showToggle = true) => (
    <ConfigProvider
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: BRAND.primary,
          borderRadius: 8,
          borderRadiusSM: 8,
        },
        components: {
          Menu: {
            itemBg: "transparent",
            itemColor: BRAND.sidebarItemText,
            itemHoverBg: BRAND.sidebarItemHoverBg,
            itemHoverColor: BRAND.sidebarText,
            itemSelectedBg: BRAND.primary,
            itemSelectedColor: "#ffffff",
            itemActiveBg: BRAND.primary,
            subMenuItemSelectedColor: "#ffffff",
            itemBorderRadius: 8,
            itemMarginInline: 0,
            itemHeight: 42,
            iconSize: 18,
            iconMarginInlineEnd: 12,
          },
        },
      }}
    >
      <Flex
        vertical
        style={{
          height: "100%",
          width: "100%",
          minWidth: 0,
          boxSizing: "border-box",
          background: BRAND.sidebarBg,
          padding: token.padding,
        }}
      >
        <Flex
          align="center"
          justify="space-between"
          style={{ marginBottom: token.marginLG, minHeight: 40 }}
        >
          {brandBlock(isCollapsed)}
          {showToggle && !isCollapsed ? (
            <Button
              type="text"
              size="small"
              onClick={toggleSidebar}
              icon={<PanelLeft size={18} />}
              aria-label={t("common.toggleSidebar")}
              style={{ color: BRAND.sidebarItemText, flexShrink: 0 }}
            />
          ) : null}
        </Flex>
        <Menu
          mode="inline"
          selectedKeys={selectedKey ? [selectedKey] : []}
          openKeys={openKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          items={builtMenu.items}
          getPopupContainer={() => document.body}
          onClick={({ key }) => {
            const path = builtMenu.keyToPath[String(key)];
            if (!path) return;
            if (isMobile) {
              setSidebarCollapsed(false);
            }
            void navigate({ to: path });
          }}
          style={{
            borderRight: "none",
            flex: 1,
            minWidth: 0,
            width: "100%",
            boxSizing: "border-box",
            overflowX: "hidden",
            overflowY: "auto",
            background: "transparent",
          }}
        />
        {isCollapsed && showToggle ? (
          <Button
            type="text"
            size="small"
            onClick={toggleSidebar}
            icon={<PanelLeft size={18} />}
            aria-label={t("common.toggleSidebar")}
            style={{ color: BRAND.sidebarItemText, marginTop: token.marginSM }}
          />
        ) : null}
      </Flex>
    </ConfigProvider>
  );

  if (isMobile) {
    return (
      <Drawer
        open={mobileOpen}
        placement="left"
        onClose={() => setSidebarCollapsed(false)}
        size={280}
        styles={{
          body: {
            padding: 0,
            background: BRAND.sidebarBg,
            overflow: "hidden",
            maxWidth: "100%",
            boxSizing: "border-box",
          },
          header: { display: "none" },
          mask: { opacity: 0.5 },
        }}
      >
        {sidebarContent(false, false)}
      </Drawer>
    );
  }

  return (
    <Sider
      key={collapsed ? "collapsed" : "expanded"}
      theme="dark"
      collapsible
      collapsed={collapsed}
      trigger={null}
      width={240}
      collapsedWidth={72}
      breakpoint="lg"
      onBreakpoint={(broken) => {
        if (broken) {
          setSidebarCollapsed(false);
        }
      }}
      style={{
        background: BRAND.sidebarBg,
        alignSelf: "stretch",
        minHeight: "100vh",
        overflow: "visible",
      }}
    >
      {sidebarContent(collapsed)}
    </Sider>
  );
}
