# console-telegram-inbox-management Specification

## Purpose
TBD - created by archiving change add-console-telegram-inboxes. Update Purpose after archive.

## Requirements

### Requirement: Console Telegram inbox APIs are admin-only and app-scoped
The system SHALL expose authenticated console APIs for Telegram inbox management. Only administrators MAY create, list, read, or update Telegram inbox assignments, and all operations MUST be scoped to the current request app.

#### Scenario: Admin lists Telegram inboxes
- **WHEN** an authenticated administrator requests the console inbox list
- **THEN** the system returns only inboxes whose `app_key` matches the current app

#### Scenario: Non-admin is rejected
- **WHEN** a non-administrator requests a console inbox management API
- **THEN** the system rejects the request with an auth error

### Requirement: Console can create Telegram inboxes
The system SHALL allow administrators to create a Telegram inbox with a display name, Telegram bot name, and Telegram bot token.

#### Scenario: Create Telegram inbox
- **WHEN** an administrator submits a valid Telegram inbox creation request
- **THEN** the system creates an `inboxes` record with a server-generated `inbox_id`, `channel_type` set to `telegram`, `name` set from the request, and `channel_conf` containing JSON encoded from `TelegramChannelConf`

#### Scenario: Reject unsupported channel type
- **WHEN** an administrator attempts to create an inbox for a channel other than Telegram
- **THEN** the system rejects the request with a parameter error

#### Scenario: Reject missing Telegram config
- **WHEN** an administrator submits a Telegram inbox creation request without bot name or bot token
- **THEN** the system rejects the request with a request-body error

### Requirement: Console can list Telegram inboxes
The system SHALL return Telegram inboxes in a response shape suitable for the console settings list, including inbox id, name, channel type, created time, updated time, and member count.

#### Scenario: List Telegram inbox rows
- **WHEN** an administrator opens the Settings > Inboxes page
- **THEN** the page can render each Telegram inbox with its name, Telegram channel label, and number of assigned representatives

### Requirement: Console can assign representatives to an inbox
The system SHALL allow administrators to replace the representative list for an inbox using user IDs from the current app's `users` table.

#### Scenario: Assign representatives
- **WHEN** an administrator saves selected users for an existing inbox
- **THEN** the system stores those users in `inboxmembers` with the target `inbox_id` and current `app_key`

#### Scenario: Remove omitted representatives
- **WHEN** an administrator saves an assignment list that omits a previously assigned user
- **THEN** the system removes that user's `inboxmembers` row for the inbox

#### Scenario: Reject users outside current app
- **WHEN** an administrator attempts to assign a user ID that does not belong to the current app
- **THEN** the system rejects the assignment request with a parameter or not-found error

### Requirement: Console web provides Chatwoot-inspired inbox setup flow
The console web app SHALL provide a Settings > Inboxes workflow for Telegram that mirrors Chatwoot's high-level flow: inbox list, channel setup, representative selection, and finish confirmation.

#### Scenario: Create inbox from settings
- **WHEN** an administrator opens Settings > Inboxes and chooses to create a new inbox
- **THEN** the UI presents the Telegram setup form and only exposes Telegram as the available channel

#### Scenario: Add representatives during setup
- **WHEN** an administrator completes the Telegram setup form
- **THEN** the UI advances to representative selection using users from the current app

#### Scenario: Finish setup
- **WHEN** an administrator saves representatives for the new inbox
- **THEN** the UI shows a setup completion state and allows returning to the inbox list
