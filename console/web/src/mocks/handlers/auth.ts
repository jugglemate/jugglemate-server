import { http } from "msw";
import { MOCK_USERS, GUEST_AUTH_USER_BODY } from "../data";
import {
  withDelay,
  errorResponse,
  ERROR_CODES,
  successWithSchema,
  successWithNullBody,
} from "../createHandler";
import {
  AuthTokensSchema,
  AuthUserResponseSchema,
  JmateRegisterRequestSchema,
  LoginRequestSchema,
  PermissionsListSchema,
  RefreshTokenRequestSchema,
} from "@/api/schemas";

const GUEST_REFRESH = "mock-guest-refresh";
const REGISTERED_ACCESS = "mock-registered-access";
const REGISTERED_REFRESH = "mock-registered-refresh";

/** Last successful register (in-memory) so /user matches the new account in MSW */
let mockRegisteredSession: { username: string; email: string | null } | null = null;

function isGuestAuth(authorization: string | null): boolean {
  return (authorization ?? "").includes("mock-guest-access");
}

function isRegisteredAuth(authorization: string | null): boolean {
  return (authorization ?? "").includes(REGISTERED_ACCESS);
}

export const authHandlers = [
  http.post("/api/auth/login", async ({ request }) => {
    await withDelay(300);
    let json: unknown;
    try {
      json = await request.json();
    } catch {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Invalid JSON body");
    }
    const parsed = LoginRequestSchema.safeParse(json);
    if (!parsed.success) {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Invalid login body");
    }
    const { account, password } = parsed.data;
    if (account === "guest" && password === "guest") {
      return successWithSchema(AuthTokensSchema, {
        accessToken: "mock-guest-access",
        refreshToken: GUEST_REFRESH,
      });
    }
    if (account === "admin" && password === "admin") {
      return successWithSchema(AuthTokensSchema, {
        accessToken: "mock-access-token",
        refreshToken: "mock-refresh-token",
      });
    }
    return errorResponse(ERROR_CODES.INVALID_CREDENTIALS, "Invalid username or password");
  }),

  http.post("/api/auth/register", async ({ request }) => {
    await withDelay(300);
    let json: unknown;
    try {
      json = await request.json();
    } catch {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Invalid JSON body");
    }
    const parsed = JmateRegisterRequestSchema.safeParse(json);
    if (!parsed.success) {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Invalid register body");
    }
    const { account, password } = parsed.data;
    if (password.length < 6) {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Password must be at least 6 characters");
    }
    const taken = account === "guest" || MOCK_USERS.some((u) => u.username === account);
    if (taken) {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Username already taken");
    }
    mockRegisteredSession = {
      username: account,
      email: null,
    };
    return successWithSchema(AuthTokensSchema, {
      accessToken: REGISTERED_ACCESS,
      refreshToken: REGISTERED_REFRESH,
    });
  }),

  http.post("/api/auth/refresh", async ({ request }) => {
    await withDelay(100);
    let json: unknown;
    try {
      json = await request.json();
    } catch {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "Invalid JSON body");
    }
    const parsed = RefreshTokenRequestSchema.safeParse(json);
    if (!parsed.success) {
      return errorResponse(ERROR_CODES.BAD_REQUEST, "refreshToken is required");
    }
    if (parsed.data.refreshToken === GUEST_REFRESH) {
      return successWithSchema(AuthTokensSchema, {
        accessToken: "mock-guest-access-refreshed",
        refreshToken: "mock-guest-refresh-refreshed",
      });
    }
    if (
      parsed.data.refreshToken === REGISTERED_REFRESH ||
      parsed.data.refreshToken.startsWith(`${REGISTERED_REFRESH}-`)
    ) {
      return successWithSchema(AuthTokensSchema, {
        accessToken: `${REGISTERED_ACCESS}-refreshed`,
        refreshToken: `${REGISTERED_REFRESH}-refreshed`,
      });
    }
    return successWithSchema(AuthTokensSchema, {
      accessToken: "mock-new-access-token",
      refreshToken: "mock-new-refresh-token",
    });
  }),

  http.post("/api/auth/logout", () => successWithNullBody()),

  http.get("/api/auth/user", ({ request }) => {
    const auth = request.headers.get("authorization");
    if (isRegisteredAuth(auth)) {
      const session = mockRegisteredSession;
      if (session) {
        return successWithSchema(AuthUserResponseSchema, {
          id: "reg-mock",
          username: session.username,
          avatar: null,
          email: session.email,
          roles: ["editor"],
        });
      }
    }
    if (isGuestAuth(auth)) {
      return successWithSchema(AuthUserResponseSchema, GUEST_AUTH_USER_BODY);
    }
    const { permissions: _permissions, ...userWithoutPermissions } = MOCK_USERS[0]!;
    return successWithSchema(AuthUserResponseSchema, userWithoutPermissions);
  }),

  http.get("/api/auth/permissions", ({ request }) => {
    const auth = request.headers.get("authorization");
    if (isRegisteredAuth(auth) && mockRegisteredSession) {
      return successWithSchema(PermissionsListSchema, ["user:view"] as string[]);
    }
    if (isGuestAuth(auth)) {
      return successWithSchema(PermissionsListSchema, [] as string[]);
    }
    return successWithSchema(PermissionsListSchema, MOCK_USERS[0]!.permissions);
  }),
];
