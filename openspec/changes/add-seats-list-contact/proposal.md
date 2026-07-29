## Why

The frontend needs a unified contact-list view so any logged-in user (admin or customer-service seat) can browse available seats for ticket transfer and collaboration. The two existing surfaces don't fit this use case:

- `GET /jmate/console/users` is the admin-only user-management CRUD; its response is intentionally narrow (id / username / email / roles) and lacks seat context such as inbox bindings, open-session load, or IM registration status.
- `GET /jmate/console/stats/agents` is a time-windowed aggregation panel for the data-statistics module; its semantics are tied to a `from / to` window and would mislead the contact-list UI.

A new endpoint under the regular business route group (`/jmate/*`) — gated only by `apis.Validate` (logged-in) — gives every role the same seat directory while staying read-only and zero-migration.

## What Changes

- Add `GET /jmate/seats/list` returning a paginated seat directory with identity, role, status, IM-registration flag, inbox bindings, open-session count, and last-active time.
- All authentication flows through the existing `apis.Validate` middleware — no new auth code.
- No database schema changes; the endpoint is a thin SQL aggregation over `users` / `inboxmembers` / `tickets`.

## Capabilities

### New Capabilities
- `seats-list`: Read-only contact directory of seats (`users`) plus seat-context aggregates, accessible to any logged-in user of the app.

## Impact

- New service module `services/seats_service.go` issuing parameterized GORM raw queries.
- New HTTP handler `apis/seats.go` parsing query params and dispatching to the service.
- New response models `apis/models/seats.go` defining the JSON shape.
- One new route `GET /jmate/seats/list` mounted in `routers/router.go` before `RouteConsole(...)`.
- New unit tests `apis/seats_test.go` and `services/seats_service_test.go`.
- New design doc `docs/seats-list-api.md`.
- No changes to `console/`, `agent/`, `storages/`, `commons/`, `conf/`, `main.go`, or any existing route handler / service.
