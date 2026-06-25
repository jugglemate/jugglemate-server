# console-user-management Specification

## Purpose
TBD - created by archiving change add-console-users-api. Update Purpose after archive.
## Requirements
### Requirement: Console user list is admin-only and app-scoped
The system SHALL expose an authenticated console endpoint to list users for the current app. Only administrators MAY call this endpoint.

#### Scenario: Admin lists users
- **WHEN** an authenticated administrator requests the console user list with a valid `appkey`
- **THEN** the system returns users whose `app_key` matches the current app

#### Scenario: Non-admin is rejected
- **WHEN** a non-administrator requests the console user list
- **THEN** the system rejects the request with an auth error

### Requirement: Console user list supports pagination and filtering
The system SHALL support `limit` and `offset` query parameters on the user list endpoint. The system SHALL support optional `keyword` and `role` filters and optional `sortField` / `sortOrder` sorting.

#### Scenario: Paginated list
- **WHEN** an administrator requests users with `limit=20` and `offset=0`
- **THEN** the system returns at most 20 users and a `total` count for the filtered result set

#### Scenario: Keyword filter
- **WHEN** an administrator requests users with `keyword=alice`
- **THEN** the system returns users whose login account, nickname, or email matches the keyword

#### Scenario: Role filter
- **WHEN** an administrator requests users with `role=admin`
- **THEN** the system returns only users with the administrator role

### Requirement: Console user list response matches admin UI shape
The system SHALL return users in a paginated response envelope `{ list, total }` where each item includes `id`, `username`, `avatar`, `email`, and `roles`.

#### Scenario: Successful list response
- **WHEN** an administrator successfully queries users
- **THEN** each list item includes the fields required by the console Users table

### Requirement: Console user create is admin-only
The system SHALL allow administrators to create users in the current app via a console endpoint.

#### Scenario: Create user
- **WHEN** an administrator submits a valid create request with account, password, optional email, and role
- **THEN** the system creates the user scoped to the current app and returns the created user item

#### Scenario: Reject duplicate account
- **WHEN** an administrator creates a user whose account already exists in the current app
- **THEN** the system rejects the request with a user-existed error

### Requirement: Console user update is admin-only
The system SHALL allow administrators to update an existing user in the current app by user id.

#### Scenario: Update user
- **WHEN** an administrator updates a user that belongs to the current app
- **THEN** the system persists the changes and returns the updated user item

#### Scenario: Update missing user
- **WHEN** an administrator updates a user id that does not exist in the current app
- **THEN** the system rejects the request with a not-found or parameter error

### Requirement: Console user delete is admin-only
The system SHALL allow administrators to delete a user in the current app by user id.

#### Scenario: Delete user
- **WHEN** an administrator deletes a user that belongs to the current app
- **THEN** the system removes the user and returns success

#### Scenario: Delete missing user
- **WHEN** an administrator deletes a user id that does not exist in the current app
- **THEN** the system rejects the request with a not-found or parameter error

### Requirement: Console web Users page uses console APIs
The console web Users page SHALL call `/jmate/console/users` endpoints for list, create, update, and delete instead of mock `/api/users` endpoints.

#### Scenario: Users page loads from backend
- **WHEN** an authenticated administrator opens the Users page
- **THEN** the page loads user data from the console user list API

#### Scenario: Users page mutates through backend
- **WHEN** an administrator creates, edits, or deletes a user from the Users page
- **THEN** the page calls the corresponding console user API and refreshes the list on success

