## Why

The console frontend needs three new panels — 客服数据统计 (overview), 坐席列表 (agents), and 客户列表 (customers) — but there is no first-class console API exposing the underlying ticket/customer/user tables aggregated by a configurable time window. Today the only stats endpoint (`GET /jmate/agentapi/admin/stats/platform`) lives in the Agent platform domain and does not model ticket/business dimensions; building the new panels against it would conflate two unrelated domains.

Adding dedicated console stats endpoints keeps the Agent platform domain untouched and gives the frontend SQL-backed, time-windowed aggregates with consistent pagination.

## What Changes

- Add `GET /jmate/console/stats/overview` returning totals (total/AI-resolved/closed/open sessions, AI resolution rate, transferred-to-human placeholder), trend buckets (day/hour), and channel ranking.
- Add `GET /jmate/console/stats/agents` returning a paginated list of seats (role 1 + optional role 2 admin) with session counts, responded counts, open counts, inbox count, and CSAT/FRT placeholders.
- Add `GET /jmate/console/stats/customers` returning a paginated list of customers that have sessions in the window, with the latest channel source, session count, and last-session timestamp.
- All endpoints authenticate via the existing `consoleApis.Validate` middleware; no new auth, no new dependencies, no new tables, no new migration.

## Capabilities

### New Capabilities
- `console-stats`: Console-side time-windowed aggregations over `tickets` / `customers` / `users` / `inboxmembers` for the 客服数据统计 / 坐席 / 客户 panels.

## Impact

- New service module `services/console_stats_service.go` issuing parameterized GORM raw queries.
- New HTTP handlers `apis/console_stats.go` parsing query params and dispatching to the service.
- New response models `apis/models/stats.go` defining the JSON shape of all three endpoints.
- Three new routes registered in `RouteConsole` inside `routers/router.go`.
- New unit tests `apis/console_stats_test.go` and `services/console_stats_service_test.go`.
- New design doc `docs/console-stats-api.md`.
- No existing files in `storages/`, existing services, existing handlers, existing tests, or `conf/` are modified.
