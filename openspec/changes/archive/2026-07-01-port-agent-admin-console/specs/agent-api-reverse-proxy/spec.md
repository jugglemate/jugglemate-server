## ADDED Requirements

### Requirement: Authenticated reverse proxy to agent-server
The Go server SHALL expose a reverse-proxy route group (e.g. `/jmate/agentapi/*`) that forwards matching requests to the configured Python agent-server base URL, preserving method, path suffix, query, and body.

#### Scenario: Path and method forwarded
- **WHEN** the console requests `GET /jmate/agentapi/agents?limit=20`
- **THEN** the Go server forwards it to `<agentServer>/api/v1/agents?limit=20` and returns the upstream response body and status to the caller

#### Scenario: Request bodies forwarded
- **WHEN** the console sends a `POST` with a JSON body to a proxied path
- **THEN** the Go server forwards the body unmodified to the agent-server

### Requirement: Proxy enforces console authentication
Proxied requests SHALL pass through the existing console authentication (`appkey` header + Go token via the `Validate` middleware) before forwarding; unauthenticated or invalid-token requests SHALL be rejected and NOT forwarded.

#### Scenario: Missing or invalid token rejected
- **WHEN** a request to a proxied path has no valid Go token
- **THEN** the Go server responds with the standard not-logged-in error and does not call the agent-server

#### Scenario: Valid token forwarded
- **WHEN** a request to a proxied path carries a valid `appkey` + Go token
- **THEN** the Go server resolves the caller's `userId` and role and proceeds to forward

### Requirement: Trusted identity injection
Before forwarding, the Go proxy SHALL strip any client-supplied auth/identity headers and inject trusted internal headers: a shared-secret header proving the request came from the Go proxy, and owner/identity headers (`X-User-ID`, `X-Admin-Id`, `X-Invite-Code`) set to the authenticated console user's `userId`.

#### Scenario: Owner derived from token
- **WHEN** a user with `userId=u_abc` calls a proxied endpoint
- **THEN** the forwarded request carries `X-User-ID: u_abc` (and matching owner headers) and the internal shared-secret header

#### Scenario: Client cannot spoof identity
- **WHEN** a client includes its own `X-User-ID` or internal shared-secret header
- **THEN** the Go proxy overwrites/removes them so only the proxy-derived values reach the agent-server

### Requirement: Proxy configuration
The Go server SHALL read the agent-server base URL and the internal shared secret from configuration; when the base URL is unset, the proxy routes SHALL be disabled or return a clear error rather than forwarding to an invalid target.

#### Scenario: Unconfigured target
- **WHEN** the agent-server base URL is not configured
- **THEN** proxied requests return a clear configuration error and no external call is made
