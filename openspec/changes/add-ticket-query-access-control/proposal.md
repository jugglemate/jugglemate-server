## Why

The project has ticket persistence and a `/tickets/list` route stub, but no implemented query behavior or access rules. Support staff and administrators need a safe way to list tickets by status without exposing tickets beyond each user's permissions.

## What Changes

- Implement the ticket list API behind `GET /tickets/list`.
- Support filtering tickets by status: pending, processing, or closed.
- Support `limit` and `offset` pagination with ID descending order and a default page size of 20.
- Enforce role-based visibility:
  - Customer-service users can see pending tickets and tickets assigned to themselves.
  - Administrators can see all tickets.
- Return ticket list data in a stable API response shape.
- Add or extend storage query methods as needed for combined role/status filtering.

## Capabilities

### New Capabilities
- `ticket-query-access-control`: Defines ticket list filtering and role-based visibility rules for the ticket query API.

### Modified Capabilities

## Impact

- Affected API layer: `apis/ticket.go`, ticket response models.
- Affected service layer: `services/ticketservice.go`.
- Affected storage layer: ticket and user storage lookup/query methods.
- Affected auth context: uses requester ID from validated auth context and user role from `users`.
- No external dependency changes expected.
