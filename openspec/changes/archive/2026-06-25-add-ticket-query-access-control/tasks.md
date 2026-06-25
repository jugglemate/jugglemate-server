## 1. API Models And Parsing

- [x] 1.1 Add ticket list request parsing for optional `status`, `limit`, and `offset` query parameters in `apis/ticket.go`.
- [x] 1.2 Add ticket list response models containing ticket ID, source ID, customer ID, channel ID, assignee ID, status, created time, and updated time.
- [x] 1.3 Validate status query values so only `0`, `1`, and `2` are accepted.

## 2. Storage Queries

- [x] 2.1 Add a ticket storage method that supports app-scoped `limit`/`offset` pagination with optional status filtering.
- [x] 2.2 Add a ticket storage method or predicate path for customer-service visibility: pending tickets or tickets assigned to the requester.
- [x] 2.3 Ensure storage query results are ordered consistently by descending ID and respect limit/offset pagination, defaulting limit to 20 when omitted.

## 3. Service Authorization

- [x] 3.1 Implement `services.QryTickets` to read `appkey` and requester user ID from context.
- [x] 3.2 Load the requester user from `users` storage and determine whether the user is customer-service or admin.
- [x] 3.3 For admins, query all app tickets subject to optional status filtering.
- [x] 3.4 For customer-service users, query only pending tickets and tickets assigned to the requester, subject to optional status filtering.
- [x] 3.5 Return parameter or auth errors for invalid status, missing requester, or missing requester user.

## 4. Handler Integration

- [x] 4.1 Wire `apis.QryTickets` to call `services.QryTickets`.
- [x] 4.2 Convert service results into the ticket list response shape.
- [x] 4.3 Keep `/tickets/list` authenticated and scoped to the current request app.

## 5. Verification

- [x] 5.1 Add or update tests for admin ticket queries with and without status filters.
- [x] 5.2 Add or update tests for customer-service visibility rules.
- [x] 5.3 Add or update tests for invalid status handling.
- [x] 5.4 Add or update tests for ID descending order and default/explicit limit-offset pagination.
- [x] 5.5 Run `go test ./...` and resolve failures.
