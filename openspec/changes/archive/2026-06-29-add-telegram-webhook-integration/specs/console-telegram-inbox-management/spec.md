## MODIFIED Requirements

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
