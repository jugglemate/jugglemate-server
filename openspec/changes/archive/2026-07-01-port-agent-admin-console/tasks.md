## 1. Config & shared secret

- [x] 1.1 Add `agentServer.baseURL` (Python origin, e.g. `http://127.0.0.1:8000`) and `agentServer.internalSecret` to `commons/configures/configures.go` and `conf/config_example.yml`; set real values in local `conf/config.yml`.
- [x] 1.2 In `agent-server`, add `auth_internal_secret` (+ header name) to `server/app/shared/config.py` settings; document it in the agent-server config/env.
- [x] 1.3 Choose the internal header name (e.g. `X-Internal-Auth`) and record it once in both repos; ensure the secret is never exposed to the browser.

## 2. Go reverse proxy

- [x] 2.1 Add a `/jmate/agentapi/*` route group in `routers/` that runs through the existing `Validate` middleware (appkey + Go token).
- [x] 2.2 Implement the proxy handler (`httputil.ReverseProxy` or equivalent) that rewrites `/jmate/agentapi/<rest>` → `<agentServer.baseURL>/api/v1/<rest>`, preserving method, query, and body.
- [x] 2.3 In the proxy director, strip client `Authorization` and any client-supplied `X-User-ID`/`X-Admin-Id`/`X-Invite-Code`/internal-secret headers.
- [x] 2.4 Inject trusted headers: internal shared-secret header, and `X-User-ID = X-Admin-Id = X-Invite-Code = <RequesterId>` from the token.
- [x] 2.5 Disable the proxy (or return a clear config error) when `agentServer.baseURL` is unset.
- [x] 2.6 Add a Go test covering: unauthenticated request rejected (not forwarded); authenticated request forwarded with injected identity and stripped client headers.

## 3. agent-server proxy auth

- [x] 3.1 Modify `server/app/shared/api/bearer_auth.py`: when a valid internal shared-secret header is present, authenticate the request with `sub = X-User-ID` and skip the JWT check.
- [x] 3.2 Reject/ignore injected identity when the internal secret is missing or wrong; keep public paths open and keep direct (non-proxied) requests protected as before.
- [x] 3.3 Update owner scoping to read the injected `X-User-ID` instead of the hardcoded `uu08en3w` for proxied requests (agents, tool-*, admin/llm/providers). Do NOT migrate existing `uu08en3w`-owned data.
- [x] 3.4 Disable the legacy JWT login: remove/disable `POST /api/v1/auth/login`, drop it from the middleware public prefixes, and stop accepting the HS256 JWT for protected routes (internal secret becomes the only auth path).
- [x] 3.5 Add Python tests: valid internal secret authenticates with injected subject; wrong/missing secret is not trusted; legacy login/JWT no longer authenticates; public paths (health/webhook/docs) still open.

## 4. Console frontend — shared plumbing

- [x] 4.1 Add a proxied http helper in `console/web/src` that targets base path `/jmate/agentapi`, unwraps the agent-server envelope (`code===0`), and converts `snake_case`→`camelCase` (mirror admin's `keysToCamel`); reuse the console's existing appkey+Authorization injection.
- [x] 4.2 Ensure the frontend does NOT send `X-User-ID`/owner headers (the proxy injects them).
- [x] 4.3 Add i18n keys for the three menus in `console/web/src` `zh-CN` and `en-US`.

## 5. Console frontend — feature: agent list (智能体列表)

- [x] 5.1 Port `agent-list` pages/components from admin, adapting antd 5→6 and react-router→TanStack Router; wire to the proxied http helper.
- [x] 5.2 Support list/filter/paging, detail view, create/update, delete, activate/pause, and the config detail views (model/tools/skills/knowledge bindings).
- [x] 5.3 Register the route and left-menu entry.

## 6. Console frontend — feature: tools list (工具列表)

- [x] 6.1 Port `skills-tools` pages/components (providers, definitions, credentials) with antd 6 + TanStack Router; wire to proxied http.
- [x] 6.2 Support provider CRUD + transition, definition plugin/custom create + test + activate, credential management.
- [x] 6.3 Register the route and left-menu entry.

## 7. Console frontend — feature: model settings (模型设置)

- [x] 7.1 Port `models` pages/components (`/admin/llm/providers/*`) with antd 6 + TanStack Router; wire to proxied http.
- [x] 7.2 Support list/view/create/update/delete and set-default.
- [x] 7.3 Register the route and left-menu entry.

## 8. Build, integrate, verify

- [x] 8.1 `pnpm -C console/web build`, then rebuild/restart the Go binary so `go:embed` serves the new dist.
- [x] 8.2 Start agent-server with the internal secret configured; verify each menu end-to-end through the proxy against live APIs.
- [x] 8.3 Verify auth: no console token → proxied calls rejected; valid token → data scoped to the logged-in `userId`; client-supplied `X-User-ID` cannot override the proxy value; legacy `/api/v1/auth/login` no longer authenticates.
- [x] 8.4 Confirm new records are created under the real `userId` and that existing `uu08en3w` data is left untouched (no migration).

## 9. Full config pages (model IDs + agent workspace)

- [x] 9.1 Extend console http layer to support multipart FormData bodies (knowledge upload).
- [x] 9.2 Add an SSE fetch-stream helper (auth-aware, proxy-prefixed) for agent chat.
- [x] 9.3 Port ModelConfigPage as full routes `/models/config` + `/models/$providerId` incl. model-ID sub-management (list/create/update/delete via /admin/llm/models).
- [x] 9.4 Change models list "new/edit" to navigate to the config page; add provider connection test.
- [x] 9.5 Port AgentConfigPage as full routes `/agents/new` + `/agents/$agentId` (config form + activate/pause).
- [x] 9.6 Port Logs panel (agent conversations list/detail) into the agent workspace.
- [x] 9.7 Port Knowledge panels (list/create/edit/bind + file upload + vectorize progress).
- [x] 9.8 Port ChatPanel (SSE streaming) into the agent workspace.
- [x] 9.9 Change agents list "new/edit" to navigate to the config page; build + tsc + verify end-to-end.
