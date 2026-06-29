## MODIFIED Requirements

### Requirement: Console Telegram inbox APIs are admin-only and app-scoped
The system SHALL expose authenticated console APIs for inbox management across supported channels (at minimum `widget`, `telegram`, and `juggleim`). Only administrators MAY create, list, read, or update inbox assignments, and all operations MUST be scoped to the current request app.

#### Scenario: Admin lists inboxes
- **WHEN** an authenticated administrator requests the console inbox list
- **THEN** the system returns inboxes whose `app_key` matches the current app and whose `channel_type` is a supported console channel

#### Scenario: Non-admin is rejected
- **WHEN** a non-administrator requests a console inbox management API
- **THEN** the system rejects the request with an auth error

### Requirement: Console can list Telegram inboxes
The system SHALL return inboxes in a response shape suitable for the console inbox list, including inbox id, name, channel type, created time, updated time, and member count, for all supported console channel types.

#### Scenario: List inbox rows by channel
- **WHEN** an administrator opens the Inboxes page
- **THEN** the page can render each inbox with its name, channel label derived from `channel_type`, and number of assigned representatives

#### Scenario: List JuggleIM inbox rows
- **WHEN** an administrator opens the Inboxes page and the current app has a JuggleIM inbox
- **THEN** the system includes the JuggleIM inbox in the list response
- **AND** the response redacts the JuggleIM bot token

### Requirement: Console web provides Chatwoot-inspired inbox setup flow
The console web app SHALL provide an Inboxes workflow that mirrors Chatwoot's high-level flow: inbox list, **channel selection**, channel-specific setup, representative selection, and finish confirmation.

#### Scenario: Select channel before setup
- **WHEN** an administrator opens Inboxes and chooses to create a new inbox
- **THEN** the UI presents a channel selection step with at least Widget, Telegram, and JuggleIM options before showing channel-specific configuration

#### Scenario: Configure Widget inbox
- **WHEN** an administrator selects the Widget channel and submits a valid inbox name and optional welcome message
- **THEN** the UI creates a widget inbox via the console API with `welcome_message` stored in `channel_conf` and advances to representative selection

#### Scenario: Configure Telegram inbox
- **WHEN** an administrator selects the Telegram channel and submits a valid Telegram configuration
- **THEN** the UI creates a Telegram inbox via the console API and advances to representative selection

#### Scenario: Configure JuggleIM inbox
- **WHEN** an administrator selects the JuggleIM channel and submits a valid JuggleIM bot name and bot token
- **THEN** the UI creates a JuggleIM inbox via the console API and advances to representative selection

#### Scenario: Add representatives during setup
- **WHEN** an administrator completes channel-specific setup for a new inbox
- **THEN** the UI advances to representative selection using users from the current app

#### Scenario: Finish setup
- **WHEN** an administrator saves representatives for the new inbox
- **THEN** the UI shows a setup completion state and allows returning to the inbox list

## ADDED Requirements

### Requirement: Console can create JuggleIM inboxes
The system SHALL allow administrators to create a JuggleIM inbox with a display name, JuggleIM bot name, and JuggleIM bot token. During creation the system SHALL use the JuggleIM bot setup seam, create the inbox with a server-generated `inbox_id`, and prepare the callback URL `jmateBaseUrl + /jmate/webhooks/juggleim/:inbox_id` for future concrete bot webhook registration.

#### Scenario: Create JuggleIM inbox
- **WHEN** an administrator submits a valid JuggleIM inbox creation request
- **THEN** the system runs JuggleIM bot setup validation through the injectable bot client seam
- **AND** the system creates an `inboxes` record with a server-generated `inbox_id`, `channel_type` set to `juggleim`, `name` set from the request, and `channel_conf` containing JSON encoded from `JuggleIMChannelConf`
- **AND** the stored channel configuration contains `bot_name` and `bot_token`

#### Scenario: Reject missing JuggleIM config
- **WHEN** an administrator submits a JuggleIM inbox creation request without bot name or bot token
- **THEN** the system rejects the request with a request-body error

#### Scenario: Reject missing public base URL
- **WHEN** an administrator submits a valid JuggleIM inbox creation request but `jmateBaseUrl` is empty
- **THEN** the system rejects the request and does not create the inbox

#### Scenario: Reject JuggleIM bot setup failure
- **WHEN** JuggleIM bot setup through the injectable client seam fails
- **THEN** the system rejects the request and does not create the inbox
