## Context

Two consoles exist today:

- **JuggleMate console** (`console/web`, this repo): React 19 + TanStack Router + antd 6 + a custom `fetch`-based `http` client + react-i18next + zustand. Served embedded by the Go server at `:8050/jmateconsole` (`//go:embed web/dist`). Auth: login via `POST /jmate/user/login` returns a **Go token** = `base64url(protobuf AuthToken{appkey, TokenValue})` where `TokenValue = AES-CBC(protobuf AuthTokenValue{userId,deviceId,tokenTime,role}, key=appsecret, iv=appsecret[:16])`. Every request sends `appkey` header + `Authorization: <token>`; the Go `Validate` middleware decrypts and sets `RequesterId`/`RoleType`.
- **agent-server admin** (`agent-server/admin`): React 18 + react-router-dom + antd 5 + axios + i18next. Auth: `POST /api/v1/auth/login` returns an HS256 HMAC JWT (`app/shared/security/access_token.py`); `BearerAuthMiddleware` verifies `Authorization: Bearer <jwt>` for all non-public paths. Owner scoping uses `X-User-ID`/`X-Admin-Id`/`X-Invite-Code`, currently hardcoded to `uu08en3w` in the admin http client.

We are moving three admin menus into the JuggleMate console while keeping the data APIs in the Python agent-server. The three menus map to Python endpoints (all under `/api/v1`):
- 智能体列表 → `/agents`, `/agents/{id}`, `/agents/create|update|delete`, `/agents/{id}/activate|pause`, plus referenced `/admin/llm/models`, `/capabilities/*`, `/knowledge` for the config views.
- 工具列表 → `/tool-providers`, `/tool-definitions(/plugin|/custom|/{id}/test|/activate)`, `/tool-credentials`.
- 模型设置 → `/admin/llm/providers(/{id}|/update|/delete|/{id}/set-default)`.

Constraints: openspec artifacts and Go/frontend edits are in `jugglemate-server`; the Python auth change is in the sibling `agent-server` repo. The Go token is not a standard JWT, so cross-language verification is non-trivial.

## Goals / Non-Goals

**Goals:**
- Show agent list, tools list, and model settings inside the console using the console's existing login.
- Keep the Python data endpoints unchanged in shape; only auth/owner handling changes.
- Single origin: the console calls the Python APIs through a Go reverse proxy (no browser CORS).
- One source of truth for identity: the Go token; the agent-server trusts the proxy's injected identity.

**Non-Goals:**
- Porting other admin menus (dashboard, knowledge, billing, chat) — out of scope this change.
- Reimplementing the Go token format in Python.
- Multi-tenant redesign beyond mapping owner scope to the logged-in `userId`.
- Migrating existing `uu08en3w`-owned agent-server data (left as-is; new data is scoped to the real `userId`).

## Decisions

### D1 — Transport: Go reverse proxy at `/jmate/agentapi/*` (chosen)
The console frontend calls same-origin `/jmate/agentapi/...`; the Go server forwards to `<agentServer>/api/v1/...`. Implemented with `httputil.ReverseProxy` (or an equivalent gin handler) mounted under a route group that already passes the `Validate` middleware.

- **Why**: same-origin (no CORS), lets Go be the single point that verifies the Go token and injects trusted identity, and avoids exposing the appsecret to Python.
- **Alternatives**: (a) Frontend calls Python directly with CORS + Python verifying the Go token — rejected: requires the appsecret + a protobuf/AES reimplementation in Python. (b) Python calls a Go introspection endpoint — rejected: extra network hop per request.

### D2 — Auth: proxy verifies Go token, injects trusted internal headers (chosen)
The Go proxy relies on `Validate` to authenticate, then before forwarding: strips client `Authorization` and any client-supplied `X-User-ID`/internal-secret headers, and sets `X-Internal-Auth: <shared-secret>`, `X-User-ID = X-Admin-Id = X-Invite-Code = <userId>`. The agent-server’s `BearerAuthMiddleware` is modified so that a request bearing a valid `X-Internal-Auth` is authenticated with `sub = X-User-ID`, bypassing the JWT check; owner-scoped queries read the injected `X-User-ID` instead of `uu08en3w`.

- **Why**: no cross-language token crypto; the browser never sees the shared secret (it’s injected server-side); owner scope follows the logged-in user.
- **Alternatives**: reimplement AES-CBC + protobuf verification in Python (fragile, duplicates appsecret); shared JWT scheme (larger change to both login flows).

### D2a — Disable the agent-server legacy JWT login (chosen)
The Go proxy becomes the only authenticated entry point, so the agent-server's own login is retired: remove/disable `POST /api/v1/auth/login` (and drop it from the auth middleware's public prefixes), and the JWT branch of `BearerAuthMiddleware` is no longer accepted for protected routes. Only a valid internal shared-secret header authenticates; public paths (health, webhook, docs) stay open.

- **Why**: the user directed retiring the standalone login; a single auth path reduces surface area and avoids a second, weaker credential.
- **Trade-off**: the agent-server can no longer be used standalone via its own login; it must sit behind the Go proxy. Acceptable per the decision.

### D2b — Owner scoping without data migration (chosen)
Owner scope is derived from the injected `X-User-ID` (real console `userId`). Existing data owned by `uu08en3w` is **not** migrated; it simply won't appear under real users, and new records are created under the real `userId`.

### D3 — Config additions
Go: `agentServer.baseURL` (Python origin, e.g. `http://127.0.0.1:8000`) and `agentServer.internalSecret`. Python: `auth_internal_secret` (and its header name), read via `get_settings()`. The shared secret must match on both sides and never be shipped to the browser.

### D4 — Frontend port strategy
Re-create the three admin features under `console/web/src` adapted to the console stack rather than copying axios/react-router code verbatim:
- Replace axios `httpClient` with the console `http` client; set the proxied base path (`/jmate/agentapi`) and add envelope unwrap (`code===0`) + `snake_case→camelCase` conversion (the console client currently does neither for these responses — add a helper mirroring admin's `keysToCamel`).
- antd 5 → antd 6 component API deltas (verify Table/Form/Modal props) and TanStack Router routes instead of react-router.
- Register three menu entries + routes; add i18n keys in `zh-CN`/`en-US`.
- Owner headers are injected by the proxy, so the frontend must NOT send `X-User-ID` etc.

### D5 — Rebuild/serve
The console is embedded via `go:embed web/dist`; after building the new UI, the Go binary must be rebuilt/restarted to serve it (dev `vp dev` is currently broken under the `/jmateconsole/` base — build + embed is the working path).

## Risks / Trade-offs

- [Anyone reaching the agent-server directly could forge `X-Internal-Auth`] → the secret must be strong and the agent-server should only be reachable by the Go proxy (network policy/localhost binding); never expose the secret to the browser.
- [antd 5→6 and react-router→TanStack migration friction] → port feature-by-feature; verify each page against the live proxied API before moving on.
- [Response envelope/camelCase differences between the two http clients] → centralize envelope+camel handling in one console helper used only for `/jmate/agentapi` calls, so console-native endpoints are unaffected.
- [Owner-scope semantics differ: agent-server keyed on `uu08en3w`, existing data may be owned by that value] → decide whether to migrate/seed data ownership for real console `userId`s, or keep a compatibility owner during transition (see Open Questions).
- [Cross-repo change] → land Python auth change and Go proxy together; guard the new Python trust path behind config so it’s inert until the secret is set.

## Migration Plan

1. Python: add `auth_internal_secret` config + trust-proxy branch in `BearerAuthMiddleware`; owner scope reads injected `X-User-ID`. Inert until secret configured.
2. Go: add `agentServer.baseURL`/`internalSecret` config; add `/jmate/agentapi/*` proxy group behind `Validate` with header stripping/injection.
3. Frontend: port the three features, register menus/routes/i18n, wire the proxied http helper.
4. Build console (`pnpm build`) → rebuild/restart Go binary to embed the new dist.
5. Verify each menu end-to-end against the live agent-server through the proxy.
- **Rollback**: unset `agentServer.baseURL`/secret (proxy disabled) and hide the three menus. Note: because the legacy JWT login is retired (D2a), rolling back the Python auth change also means the agent-server has no login of its own — keep the internal-secret path landed and simply gate it behind config so it can be re-enabled without restoring JWT login.

## Open Questions

- Exact Python agent-server base URL/port for the proxy target in each environment.

## Resolved Decisions

- Owner scope uses the real console `userId`; existing `uu08en3w`-owned data is **not** migrated (D2b).
- The agent-server legacy JWT login is **disabled**; the Go proxy is the only auth entry point (D2a).
