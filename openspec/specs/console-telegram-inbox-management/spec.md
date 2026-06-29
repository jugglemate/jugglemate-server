# console-telegram-inbox-management Specification

## Purpose
TBD - created by archiving change add-console-telegram-inboxes. Updated by add-console-widget-inbox for multi-channel inbox management.

## Requirements

### Requirement: Console Telegram inbox APIs are admin-only and app-scoped
The system SHALL expose authenticated console APIs for inbox management across supported channels (at minimum `widget` and `telegram`). Only administrators MAY create, list, read, or update inbox assignments, and all operations MUST be scoped to the current request app.

#### Scenario: Admin lists inboxes
- **WHEN** an authenticated administrator requests the console inbox list
- **THEN** the system returns inboxes whose `app_key` matches the current app and whose `channel_type` is a supported console channel

#### Scenario: Non-admin is rejected
- **WHEN** a non-administrator requests a console inbox management API
- **THEN** the system rejects the request with an auth error

### Requirement: Console can create Telegram inboxes
The system SHALL allow administrators to create a Telegram inbox with a display name, Telegram bot name, and Telegram bot token. During creation the system SHALL validate the bot token with Telegram, create the inbox with a server-generated `inbox_id`, and configure Telegram's webhook URL to `jmateBaseUrl + /jmate/webhooks/telegram/:inbox_id`.

#### Scenario: Create Telegram inbox
- **WHEN** an administrator submits a valid Telegram inbox creation request
- **THEN** the system validates the Telegram bot token against Telegram Bot API
- **AND** the system creates an `inboxes` record with a server-generated `inbox_id`, `channel_type` set to `telegram`, `name` set from the request, and `channel_conf` containing JSON encoded from `TelegramChannelConf`
- **AND** the system configures the Telegram bot webhook URL using the created `inbox_id`

#### Scenario: Reject missing Telegram config
- **WHEN** an administrator submits a Telegram inbox creation request without bot name or bot token
- **THEN** the system rejects the request with a request-body error

#### Scenario: Reject invalid Telegram bot token
- **WHEN** an administrator submits a Telegram inbox creation request with a bot token rejected by Telegram
- **THEN** the system rejects the request and does not create the inbox

#### Scenario: Reject missing public base URL
- **WHEN** an administrator submits a valid Telegram inbox creation request but `jmateBaseUrl` is empty
- **THEN** the system rejects the request and does not create the inbox

#### Scenario: Reject webhook setup failure
- **WHEN** Telegram bot token validation succeeds but Telegram webhook configuration fails
- **THEN** the system rejects the request and does not leave a usable Telegram inbox without a configured webhook

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

### Requirement: Console can list Telegram inboxes
The system SHALL return inboxes in a response shape suitable for the console inbox list, including inbox id, name, channel type, created time, updated time, and member count, for all supported console channel types.

#### Scenario: List inbox rows by channel
- **WHEN** an administrator opens the Inboxes page
- **THEN** the page can render each inbox with its name, channel label derived from `channel_type`, and number of assigned representatives

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
