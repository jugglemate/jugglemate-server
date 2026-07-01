## Why

The agent-server admin (`agent-server/admin`) hosts the operational UI for AI agents, tools, and model providers, but it lives in a separate app with its own login and its own HS256 JWT auth against the Python backend. Operators must run and sign into two consoles. We want the three most-used admin menus — 智能体列表 (agent list), 工具列表 (tools list), 模型设置 (model settings) — inside the JuggleMate console (`console/web`, served at `:8050/jmateconsole`) using the console's existing login, while the underlying data APIs stay in the Python agent-server unchanged.

## What Changes

- Port three admin features into `console/web`: agent list, tools list (skills/tools), and model settings — including their sub-pages/config detail views — adapted to the console's stack (React 19, TanStack Router, antd 6, the console `http` client, react-i18next, zustand).
- Add three new left-menu entries + routes in the console for these features.
- Add a **reverse proxy** in the Go server: a new route group (e.g. `/jmate/agentapi/*`) that forwards to the Python agent-server, so the console calls the Python APIs same-origin (no CORS). Requires a new `agentServer` proxy target in config.
- **Unify authentication**: the Go proxy runs requests through the existing console auth (`appkey` + Go token via `Validate`), then injects trusted internal identity headers (owner = the logged-in console user's `userId`) to the Python service. **BREAKING (agent-server)**: modify the Python `BearerAuthMiddleware` so proxied requests authenticate via a trusted internal shared-secret header + injected identity; owner scoping (`X-User-ID`/`X-Admin-Id`) is taken from the token subject rather than the hardcoded `uu08en3w`. The agent-server's **legacy JWT login (`POST /api/v1/auth/login`) is disabled** — the Go proxy becomes the only authenticated entry point. Existing `uu08en3w`-owned data is **not** migrated.

## Capabilities

### New Capabilities
- `console-agent-admin`: Console pages and behavior for the agent list, tools list, and model settings menus (navigation, list/detail/config views, data operations against the proxied agent-server APIs).
- `agent-api-reverse-proxy`: The Go server's authenticated reverse proxy to the Python agent-server, including token verification and injection of trusted internal identity/owner headers.
- `agent-server-proxy-auth`: The agent-server's modified authentication that trusts the Go proxy's internal shared-secret header and derives the request owner from the injected identity.

### Modified Capabilities
<!-- No existing openspec spec's requirements change; this adds new capabilities only. -->

## Impact

- **Go (`jugglemate-server`)**: new proxy route group + middleware wiring in `routers/`; config additions for the agent-server base URL and internal proxy shared secret (`commons/configures`, `conf/config.yml`).
- **Frontend (`console/web`)**: new `src/features` (or routes) for the three menus, menu/route registration, i18n strings, response-envelope + snake_case→camelCase handling to match agent-server responses.
- **Python (`agent-server`)**: `app/shared/api/bearer_auth.py` (trust proxy header path), owner-scoping to use injected identity; config for the shared secret. Data endpoints (`/agents/*`, `/tool-*`, `/admin/llm/providers/*`) are unchanged.
- **Cross-repo**: implementation spans `jugglemate-server` and the sibling `agent-server` repo; openspec artifacts live in `jugglemate-server`.
- **Auth/security**: introduces a shared internal secret between Go and Python; must not be exposed to browsers.
