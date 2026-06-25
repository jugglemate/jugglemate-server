## Context

Ticket persistence already exists in `storages/models/ticket.go` and `storages/dbs/ticketdao.go`, and the router exposes `GET /tickets/list`, but the API handler and service are currently empty. Authentication middleware populates `appkey` and requester user ID in context for authenticated routes, and `users` storage contains the role enum needed to distinguish customer-service users from administrators.

The change should build on the current Gin API, service, and storage patterns without adding dependencies. Ticket status is represented as numeric values: pending `0`, processing `1`, closed `2`.

## Goals / Non-Goals

**Goals:**
- Implement a ticket list endpoint that supports optional status filtering.
- Apply role-based visibility in the service layer.
- Use the requester user ID from context to determine the current user's role and assignment scope.
- Return a predictable list response with ticket fields needed by clients.
- Keep storage queries efficient enough for paginated list use.

**Non-Goals:**
- No ticket creation or assignment workflow changes.
- No change to authentication token format.
- No UI work.
- No database migration beyond any indexes/query helpers required by implementation.

## Decisions

- Enforce authorization in `services.QryTickets`.
  - Rationale: API handlers should parse request parameters, while service logic owns business rules. This keeps role logic reusable and testable.
  - Alternative considered: enforce filtering in the API handler. That would duplicate storage/user lookup details at the HTTP boundary.

- Resolve role from `users` by `appkey + requester_id`.
  - Rationale: the auth context already carries requester ID from the token. The `users` table is the source of truth for `UserRoleCustomerService` and `UserRoleAdmin`.
  - Alternative considered: trust a role value from token/context. The current token parsing does not reliably populate role, so database lookup is safer.

- Treat status as optional; when present, validate it against known ticket status values.
  - Rationale: clients need status-specific queries, but admins should also be able to request an unfiltered list.
  - Alternative considered: require status for all queries. This would prevent admins from listing all tickets and is stricter than the requested behavior.

- Use `limit` and `offset` query parameters for pagination, ordered by ticket `id desc`, with `limit` defaulting to 20.
  - Rationale: the requested API contract is offset-based, and ID descending gives stable newest-first ordering for the existing auto-increment primary key.
  - Alternative considered: cursor pagination with `startId`. Existing storage helpers use this pattern, but it does not match the requested API parameters.

- For customer-service users, query tickets with a visibility predicate equivalent to `status = pending OR assignee_id = requester_id`, then apply any requested status filter.
  - Rationale: this directly models "普通用户只能查询待处理的工单和分配给自己的工单".
  - Alternative considered: run two queries and merge results in memory. A single DB predicate avoids duplicate handling and keeps pagination predictable.

- Add a dedicated storage method for role-aware list predicates instead of composing existing narrow query methods.
  - Rationale: existing methods query one dimension at a time. The required permission predicate combines status, assignee, optional status, appkey, and pagination.
  - Alternative considered: use `QryByAssignee` and a separate pending query. This complicates ordering and pagination.

## Risks / Trade-offs

- [Risk] Users missing from `users` cannot be authorized correctly. → Return a not-login or not-found style error rather than falling back to broad access.
- [Risk] Optional status filtering combined with customer-service visibility may surprise clients, e.g. requesting closed returns only closed tickets assigned to the requester. → Document this in the spec and keep behavior consistent.
- [Risk] Large offsets can become slow on large ticket tables. → Keep the first implementation simple as requested and rely on `id desc` ordering; revisit cursor pagination if data volume requires it.
- [Risk] Large ticket tables may need additional indexes for `app_key/status/assignee_id`. → Start with storage-level query support and add indexes if query plans require it.
- [Risk] Existing empty API stubs may not have response models yet. → Add minimal request parsing and response types scoped to ticket listing.
