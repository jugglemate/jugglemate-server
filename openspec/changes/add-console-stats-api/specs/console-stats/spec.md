# Console Stats Capability

## Purpose

Expose time-windowed aggregates of business tickets, agent seats, and customer profiles for the 客服数据统计 / 坐席列表 / 客户列表 panels of the console frontend.

## Requirements

### Endpoint: Get Stats Overview

The system SHALL provide `GET /jmate/console/stats/overview` returning totals, time-series trend buckets, and channel ranking for the configured time window.

- The endpoint SHALL authenticate requests via the existing `consoleApis.Validate` middleware (admin role).
- The endpoint SHALL accept `from`, `to`, and `granularity` query parameters; missing values SHALL default to `[today-7d, today]` and `day`.
- The response SHALL include:
  - `totals.total_sessions`: count of `tickets` rows where `app_key` matches and `created_time` falls in the window.
  - `totals.ai_resolved_sessions`: count of rows where `status=2 AND (assignee_id IS NULL OR assignee_id='')`.
  - `totals.ai_resolution_rate`: percentage of `ai_resolved_sessions / total_sessions`, rounded to two decimals; `0` when `total_sessions = 0`.
  - `totals.transferred_to_human`: integer, set to `0` in this capability; `totals.transferred_to_human_pending = true` MUST accompany it.
  - `totals.open_sessions` and `totals.closed_sessions`: counts of `status IN (0,1,3)` and `status = 2` respectively.
  - `new_sessions_trend`: array of `{bucket, total, ai_resolved, transferred_to_human}` per `date_trunc('day' | 'hour', created_time)` bucket, ordered ascending by bucket; `bucket` is formatted as `YYYY-MM-DD` for day, `YYYY-MM-DD HH` for hour.
  - `channel_ranking`: array of `{channel_type, count, percentage}` descending by count; `percentage` is `count / total * 100`, rounded to two decimals.

### Endpoint: Get Stats Agents

The system SHALL provide `GET /jmate/console/stats/agents` returning a paginated list of seat users with session aggregates.

- The endpoint SHALL authenticate via `consoleApis.Validate`.
- The endpoint SHALL accept `from`, `to`, `page` (default 1), `pageSize` (default 20, capped at 100), and `include_admin` (default `true`).
- Each item SHALL include `user_id`, `login_account`, `nickname`, `avatar`, `email`, `role`, `inbox_count` (count of `inboxmembers` rows linking the user), `total_sessions`, `responded_sessions`, `open_sessions`, plus `csat_avg` and `frt_avg_ms` set to `null` and `csat_pending` / `frt_pending` set to `true`.
- The `total` field SHALL reflect the count of seat users regardless of pagination; sorting SHALL be `total_sessions DESC NULLS LAST, nickname`.
- When `include_admin=false`, the `role` filter SHALL be `role=1` only.

### Endpoint: Get Stats Customers

The system SHALL provide `GET /jmate/console/stats/customers` returning a paginated list of customers that have at least one ticket in the window.

- The endpoint SHALL authenticate via `consoleApis.Validate`.
- The endpoint SHALL accept `from`, `to`, `page`, `pageSize` (with the same defaults and cap as the agents endpoint), `keyword` (substring match against `nickname / identifier / phone / email`), and `channel_type` (filter to customers with at least one ticket of the given channel in the window).
- Each item SHALL include `customer_id`, `nickname`, `avatar`, `phone`, `email`, `identifier`, `source` (the `channel_type` of the customer's most recent ticket in the window, may be empty), `session_count`, and `last_session_time` (ms epoch).

### Data Sources

- All aggregates SHALL be computed in PostgreSQL via parameterized GORM raw queries; no application-side aggregation.
- The system SHALL NOT introduce new tables, indexes, or columns as part of this capability.

## Phasing Notes

The placeholder fields `transferred_to_human`, `csat_avg`, `frt_avg_ms` (and their accompanying `*_pending` flags) are intentional. A future change in this repository MUST populate them from:
- `tickets.handover_at` (Phase 2 migration)
- `ticket_ratings` (Phase 2 migration)
- JuggleIM group-message persistence into a `ticket_messages` table (Phase 2)
