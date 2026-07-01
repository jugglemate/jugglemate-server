import type { MenuItem } from "@/api/schemas";

export const APP_MENU_TREE: MenuItem[] = [
  {
    id: "1",
    kind: "item",
    name: "menu.dashboard",
    path: "/dashboard",
    icon: "IconLucideLayoutDashboard",
    children: null,
    permissions: null,
    sort: 0,
    hidden: false,
  },
  {
    id: "2",
    kind: "item",
    name: "menu.users",
    path: "/users",
    icon: "IconLucideUsers",
    children: null,
    permissions: ["user:view"],
    sort: 1,
    hidden: false,
  },
  {
    id: "3",
    kind: "item",
    name: "menu.inboxes",
    path: "/inboxes",
    icon: "IconLucideInbox",
    children: null,
    permissions: ["inbox:view"],
    sort: 2,
    hidden: false,
  },
  {
    id: "4",
    kind: "item",
    name: "menu.agents",
    path: "/agents",
    icon: "IconLucideBot",
    children: null,
    permissions: null,
    sort: 3,
    hidden: false,
  },
  {
    id: "5",
    kind: "item",
    name: "menu.tools",
    path: "/tools",
    icon: "IconLucideWrench",
    children: null,
    permissions: null,
    sort: 4,
    hidden: false,
  },
  {
    id: "6",
    kind: "item",
    name: "menu.models",
    path: "/models",
    icon: "IconLucideCpu",
    children: null,
    permissions: null,
    sort: 5,
    hidden: false,
  },
];

function hasRequiredPermissions(
  required: string[] | null | undefined,
  granted: Set<string>,
): boolean {
  if (!required || required.length === 0) return true;
  return required.every((p) => granted.has(p));
}

export function filterMenuTreeByPermissions(
  nodes: MenuItem[],
  permissionList: string[],
): MenuItem[] {
  const granted = new Set(permissionList);

  const walk = (list: MenuItem[]): MenuItem[] =>
    list
      .map((node) => {
        if (!hasRequiredPermissions(node.permissions ?? null, granted)) return null;

        if (node.kind === "group") {
          const children = walk(node.children);
          if (children.length === 0) return null;
          return { ...node, children };
        }

        if (node.children?.length) {
          const children = walk(node.children);
          if (children.length === 0) return null;
          return { ...node, children };
        }

        return node;
      })
      .filter((n): n is MenuItem => n != null);

  return walk(nodes);
}

export function normalizeAppPath(pathname: string): string {
  if (pathname === "/") return pathname;
  return pathname.replace(/\/+$/, "") || "/";
}

export function requiredPermissionForPath(pathname: string): string | null {
  const p = normalizeAppPath(pathname);
  const map: Record<string, string | null> = {
    "/dashboard": null,
    "/users": "user:view",
    "/inboxes": "inbox:view",
    "/settings/inboxes": "inbox:view",
    "/agents": null,
    "/tools": null,
    "/models": null,
    "/403": null,
  };
  return map[p] ?? null;
}

export function canAccessPath(pathname: string, permissions: string[] | undefined): boolean {
  const required = requiredPermissionForPath(pathname);
  if (required == null) return true;
  if (!permissions?.length) return false;
  return permissions.includes(required);
}
