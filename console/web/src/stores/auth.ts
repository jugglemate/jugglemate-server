import type { AuthTokens, User, MenuItem } from "@/api/schemas";
import { APP_MENU_TREE, filterMenuTreeByPermissions } from "@/utils/appMenu";
import { createPersistentStore } from "./createPersistentStore";

interface AuthState {
  tokens: AuthTokens | null;
  user: User | null;
  menus: MenuItem[];
  isAuthenticated: boolean;
  setTokens: (tokens: AuthTokens) => void;
  setUser: (user: User) => void;
  setMenus: (menus: MenuItem[]) => void;
  hasPermission: (point: string) => boolean;
  logout: () => void;
}

export const useAuthStore = createPersistentStore<AuthState>(
  (set, get) => ({
    tokens: null,
    user: null,
    menus: [],
    isAuthenticated: false,

    setTokens: (tokens) => set({ tokens, isAuthenticated: true }),
    setUser: (user) => set({ user }),
    setMenus: (menus) => set({ menus }),

    hasPermission: (point) => {
      const { user } = get();
      return user?.permissions.includes(point) ?? false;
    },

    logout: () =>
      set({
        tokens: null,
        user: null,
        menus: [],
        isAuthenticated: false,
      }),
  }),
  {
    name: "auth-storage",
    partialize: (state) => ({
      tokens: state.tokens,
      user: state.user,
      menus: state.menus,
      isAuthenticated: state.isAuthenticated,
    }),
    merge: (persistedState, currentState) => {
      const p = persistedState as Partial<
        Pick<AuthState, "tokens" | "user" | "menus" | "isAuthenticated">
      > | null;
      const user = p?.user ?? currentState.user;
      const menus = user?.permissions?.length
        ? filterMenuTreeByPermissions(APP_MENU_TREE, user.permissions)
        : (p?.menus ?? currentState.menus);
      return {
        ...currentState,
        tokens: p?.tokens ?? currentState.tokens,
        user,
        menus,
        isAuthenticated: p?.isAuthenticated ?? currentState.isAuthenticated,
      };
    },
  },
);

export function getAuthTokens(): AuthTokens | null {
  return useAuthStore.getState().tokens;
}

export function getAccessToken(): string | null {
  return useAuthStore.getState().tokens?.accessToken ?? null;
}
