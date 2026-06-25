# ticket-query-access-control Specification

## Purpose
TBD - created by archiving change add-ticket-query-access-control. Update Purpose after archive.
## Requirements
### Requirement: Ticket list endpoint filters by status
The system SHALL expose an authenticated ticket list endpoint that accepts an optional ticket status filter. Valid statuses MUST be pending `0`, processing `1`, and closed `2`.

#### Scenario: List tickets without status filter
- **WHEN** an authenticated administrator requests the ticket list without a status filter
- **THEN** the system returns tickets for the current app regardless of status

#### Scenario: List tickets with status filter
- **WHEN** an authenticated user requests the ticket list with status `1`
- **THEN** the system only returns tickets whose status is processing `1`

#### Scenario: Reject invalid status filter
- **WHEN** an authenticated user requests the ticket list with an unsupported status value
- **THEN** the system rejects the request with a parameter error

### Requirement: Customer-service users have restricted ticket visibility
The system SHALL restrict users with the customer-service role to tickets that are pending or assigned to the requester.

#### Scenario: Customer-service user sees pending tickets
- **WHEN** a customer-service user requests the ticket list
- **THEN** the system includes pending tickets for the current app

#### Scenario: Customer-service user sees assigned tickets
- **WHEN** a customer-service user requests the ticket list
- **THEN** the system includes tickets assigned to that user's user ID

#### Scenario: Customer-service user cannot see unassigned non-pending tickets
- **WHEN** a customer-service user requests the ticket list
- **THEN** the system excludes non-pending tickets that are not assigned to that user

#### Scenario: Customer-service status filter respects visibility
- **WHEN** a customer-service user requests closed tickets
- **THEN** the system only returns closed tickets assigned to that user

### Requirement: Administrators have full ticket visibility
The system SHALL allow users with the administrator role to query all tickets for the current app, subject only to explicit request filters.

#### Scenario: Administrator sees all statuses
- **WHEN** an administrator requests the ticket list without a status filter
- **THEN** the system returns pending, processing, and closed tickets for the current app

#### Scenario: Administrator status filter
- **WHEN** an administrator requests pending tickets
- **THEN** the system returns all pending tickets for the current app

### Requirement: Ticket list response is structured
The system SHALL return tickets in a structured list response including ticket ID, source ID, customer ID, channel ID, assignee ID, status, created time, and updated time.

#### Scenario: Successful ticket list response
- **WHEN** an authenticated user successfully queries tickets
- **THEN** the response contains a list of ticket items with the required ticket fields

#### Scenario: Empty ticket list response
- **WHEN** an authenticated user has no visible tickets matching the request
- **THEN** the system returns an empty list rather than an error

### Requirement: Ticket list supports limit and offset pagination
The system SHALL order ticket list results by `id` descending and SHALL support `limit` and `offset` pagination parameters. If `limit` is omitted, the system SHALL use a default limit of 20.

#### Scenario: Default page size
- **WHEN** an authenticated user requests the ticket list without a `limit`
- **THEN** the system returns at most 20 tickets

#### Scenario: Explicit limit and offset
- **WHEN** an authenticated user requests the ticket list with `limit=10` and `offset=20`
- **THEN** the system returns at most 10 tickets after skipping the first 20 tickets in `id` descending order

#### Scenario: Newest tickets first
- **WHEN** an authenticated user requests the ticket list
- **THEN** visible tickets are ordered from highest ID to lowest ID

### Requirement: Ticket queries are scoped to the current app
The system SHALL only return tickets whose `app_key` matches the current request app key.

#### Scenario: App isolation
- **WHEN** an authenticated user queries tickets for one app
- **THEN** the system does not return tickets from any other app

