# Seats List Capability

## Purpose

Provide a read-only, paginated directory of customer-service seats with seat-context aggregates (inbox bindings, open-session count, last-active time, IM registration flag), accessible to any logged-in user of the app.

## Requirements

### Endpoint: Get Seats List

The system SHALL provide `GET /jmate/seats/list` returning the seat directory.

- The endpoint SHALL authenticate requests via the existing `apis.Validate` middleware (logged-in user of the current app). It SHALL NOT require the admin role.
- The endpoint SHALL accept `keyword`, `role`, `inbox_id`, `status`, `page` (default 1), `pageSize` (default 20, capped at 100) query parameters.
- The `role` query parameter SHALL accept `customer_service` or `admin`; values outside this set return `17005`.
- The `status` query parameter, when supplied, SHALL be one of `0` (normal) or `1` (disabled); other values return `17005`.
- Each item SHALL include `user_id`, `login_account`, `nickname`, `avatar`, `email`, `role` (string), `status` (int), `im_token_present` (bool), `inbox_ids` (array, ascending), `inbox_count`, `open_session_count`, `last_active_time` (int64, ms epoch), `created_time` (int64, ms epoch).
- The `total` field SHALL reflect the count of seats matching the filters regardless of pagination.
- Sorting SHALL be `status ASC, inbox_count DESC, login_account ASC`.

### Data Sources

- All aggregates SHALL be computed in PostgreSQL via parameterized GORM raw queries.
- The system SHALL NOT introduce new tables, indexes, or columns.
