## MODIFIED Requirements

### Requirement: Console Telegram inbox APIs are admin-only and app-scoped
The system SHALL expose authenticated console APIs for inbox management across supported channels (at minimum `widget` and `telegram`). Only administrators MAY create, list, read, or update inbox assignments, and all operations MUST be scoped to the current request app.

#### Scenario: Admin lists inboxes
- **WHEN** an authenticated administrator requests the console inbox list
- **THEN** the system returns inboxes whose `app_key` matches the current app and whose `channel_type` is a supported console channel

#### Scenario: Non-admin is rejected
- **WHEN** a non-administrator requests a console inbox management API
- **THEN** the system rejects the request with an auth error

### Requirement: Console can create Telegram inboxes
The system SHALL allow administrators to create a Telegram inbox with a display name, Telegram bot name, and Telegram bot token.

#### Scenario: Create Telegram inbox
- **WHEN** an administrator submits a valid Telegram inbox creation request
- **THEN** the system creates an `inboxes` record with a server-generated `inbox_id`, `channel_type` set to `telegram`, `name` set from the request, and `channel_conf` containing JSON encoded from `TelegramChannelConf`

#### Scenario: Reject missing Telegram config
- **WHEN** an administrator submits a Telegram inbox creation request without bot name or bot token
- **THEN** the system rejects the request with a request-body error

### Requirement: Console can list Telegram inboxes
The system SHALL return inboxes in a response shape suitable for the console inbox list, including inbox id, name, channel type, created time, updated time, and member count, for all supported console channel types.

#### Scenario: List inbox rows by channel
- **WHEN** an administrator opens the Inboxes page
- **THEN** the page can render each inbox with its name, channel label derived from `channel_type`, and number of assigned representatives

### Requirement: Console web provides Chatwoot-inspired inbox setup flow
The console web app SHALL provide an Inboxes workflow that mirrors Chatwoot's high-level flow: inbox list, **channel selection**, channel-specific setup, representative selection, and finish confirmation.

#### Scenario: Select channel before setup
- **WHEN** an administrator opens Inboxes and chooses to create a new inbox
- **THEN** the UI presents a channel selection step with at least Widget and Telegram options before showing channel-specific configuration

#### Scenario: Configure Widget inbox
- **WHEN** an administrator selects the Widget channel and submits a valid inbox name and optional welcome message
- **THEN** the UI creates a widget inbox via the console API with `welcome_message` stored in `channel_conf` and advances to representative selection

#### Scenario: Configure Telegram inbox
- **WHEN** an administrator selects the Telegram channel and submits a valid Telegram configuration
- **THEN** the UI creates a Telegram inbox via the console API and advances to representative selection

#### Scenario: Add representatives during setup
- **WHEN** an administrator completes channel-specific setup for a new inbox
- **THEN** the UI advances to representative selection using users from the current app

#### Scenario: Finish setup
- **WHEN** an administrator saves representatives for the new inbox
- **THEN** the UI shows a setup completion state and allows returning to the inbox list

## ADDED Requirements

### Requirement: Console can create Widget inboxes
The system SHALL allow administrators to create a Widget inbox with a display name and an optional welcome message. The welcome message SHALL be stored in `channel_conf` as JSON encoded from `WebWidgetChannelConf`.

#### Scenario: Create Widget inbox with name and welcome message
- **WHEN** an administrator submits a Widget inbox creation request with a non-empty name and a welcome message
- **THEN** the system creates an `inboxes` record with a server-generated `inbox_id`, `channel_type` set to `widget`, `name` set from the request, and `channel_conf` containing `welcome_message`

#### Scenario: Create Widget inbox without welcome message
- **WHEN** an administrator submits a Widget inbox creation request with a non-empty name and no welcome message
- **THEN** the system creates the inbox with `channel_conf` containing an empty or omitted `welcome_message`

#### Scenario: Reject Widget inbox without name
- **WHEN** an administrator submits a Widget inbox creation request without a name
- **THEN** the system rejects the request with a request-body error

#### Scenario: Widget inbox usable by customers start
- **WHEN** a Widget inbox has been created for the current app
- **THEN** its `inbox_id` MAY be passed to `POST /jmate/customers/start`, satisfy the widget inbox validation, and return the configured `welcome_message` in the response
