## ADDED Requirements

### Requirement: Trust the Go proxy's internal identity
The agent-server authentication SHALL accept a request as authenticated when it carries a valid internal shared-secret header matching the configured value, treating the injected identity header (`X-User-ID`) as the authenticated subject — without requiring the HS256 JWT.

#### Scenario: Valid internal secret authenticates
- **WHEN** a request arrives with a correct internal shared-secret header and `X-User-ID: u_abc`
- **THEN** the agent-server treats it as authenticated with subject `u_abc` and processes the request

#### Scenario: Wrong or missing internal secret is not trusted
- **WHEN** a request presents an incorrect or absent internal shared-secret header
- **THEN** the agent-server does NOT grant access on the basis of the injected identity headers

### Requirement: Owner scoping from injected identity
For proxied requests, the agent-server SHALL derive the owner/tenant scope from the injected identity (`X-User-ID`) rather than a hardcoded owner value, so data is scoped to the logged-in console user.

#### Scenario: Data scoped to console user
- **WHEN** an authenticated proxied request from `u_abc` lists or mutates owner-scoped resources
- **THEN** the agent-server scopes the operation to owner `u_abc`

### Requirement: Retire the legacy JWT login
The agent-server's own login (`POST /api/v1/auth/login`) SHALL be disabled and SHALL NOT be a valid authentication path; the trusted internal shared-secret header injected by the Go proxy SHALL be the only way to authenticate protected routes. Public paths (health, webhook, docs) remain open.

#### Scenario: Legacy login no longer authenticates
- **WHEN** a client calls the legacy login endpoint or presents a previously issued HS256 JWT
- **THEN** the agent-server does NOT grant access to protected routes on that basis

#### Scenario: Public paths still open
- **WHEN** a request hits a configured public path (health, webhook, docs)
- **THEN** it is allowed

#### Scenario: Direct request without internal secret rejected
- **WHEN** a non-public request arrives without a valid internal shared-secret header
- **THEN** the agent-server rejects it with the standard unauthorized response
