import { httpClient } from "@/utils/http";
import { AUTH_ENDPOINTS } from "@/api/auth";
import {
  AuthUserResponseSchema,
  JmateLoginResponseSchema,
  PermissionsListSchema,
  roleNumberToRoles,
  UserSchema,
  type JmateLoginResponse,
} from "@/api/schemas";
import { APP_MENU_TREE, filterMenuTreeByPermissions } from "@/utils/appMenu";
import { CONSOLE_ADMIN_PERMISSIONS } from "@/utils/constants";
import { useAuthStore } from "@/stores/auth";

/** Apply console session from `/jmate/user/login` or `/jmate/user/register` response. */
export function applyLoginSession(data: JmateLoginResponse): void {
  const login = JmateLoginResponseSchema.parse(data);
  const permissions = [...CONSOLE_ADMIN_PERMISSIONS];
  const { setTokens, setUser, setMenus } = useAuthStore.getState();

  setTokens({
    accessToken: login.authorization,
    refreshToken: "",
  });
  setUser({
    id: login.user_id,
    username: login.nickname || login.user_id,
    avatar: login.avatar || null,
    email: null,
    roles: roleNumberToRoles(login.role),
    permissions,
  });
  setMenus(filterMenuTreeByPermissions(APP_MENU_TREE, permissions));
}

/** Fetches mock auth profile when available; skips if user is already in store. */
export async function fetchSessionAndApplyToStore(): Promise<void> {
  const store = useAuthStore.getState();
  if (store.user) {
    store.setMenus(filterMenuTreeByPermissions(APP_MENU_TREE, store.user.permissions));
    return;
  }

  const [userBase, permissions] = await Promise.all([
    httpClient.get(AUTH_ENDPOINTS.user).then((d) => AuthUserResponseSchema.parse(d)),
    httpClient.get(AUTH_ENDPOINTS.permissions).then((d) => PermissionsListSchema.parse(d)),
  ]);
  const nextUser = UserSchema.parse({ ...userBase, permissions });
  const menus = filterMenuTreeByPermissions(APP_MENU_TREE, permissions);
  store.setUser(nextUser);
  store.setMenus(menus);
}
