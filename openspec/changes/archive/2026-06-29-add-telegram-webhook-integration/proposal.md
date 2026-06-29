## Why

Telegram inboxes can be configured in the console, but the bot is not yet connected to Telegram callbacks and incoming Telegram visitors cannot enter the existing customer/ticket workflow. Connecting Telegram through webhooks lets the server receive bot messages, create or reuse the visitor and ticket, and route the conversation into the same IM group flow used by widget visitors.

## What Changes

- Configure Telegram bot webhooks when a Telegram inbox is created, using `jmateBaseUrl` from the server config as the callback URL prefix.
- Add a public `POST /jmate/webhooks/telegram/:inbox_id` endpoint that accepts Telegram webhook updates for the configured inbox.
- Validate that webhook inboxes exist in the current app, are `telegram` inboxes, and have a usable bot token in `TelegramChannelConf`.
- Process supported private Telegram message updates by using the Telegram user numeric ID as the visitor identity.
- Create or reuse the customer, customer-inbox relation, IM user, ticket group, and ticket using the same core flow as `POST /jmate/customers/start`.
- Forward Telegram text messages into the ticket group conversation whose group ID is the ticket ID.
- Acknowledge unsupported Telegram update types without breaking Telegram delivery retries, while logging enough detail for diagnostics.

## Capabilities

### New Capabilities
- `telegram-webhook-processing`: Defines receiving Telegram webhook updates and converting supported visitor messages into JuggleMate customers, tickets, and IM group messages.

### Modified Capabilities
- `console-telegram-inbox-management`: Telegram inbox creation shall validate/configure the bot webhook URL using `jmateBaseUrl` and the generated inbox ID.

## Impact

- Affected config: `commons/configures.AppConfig`, `conf/config.yml`, and `conf/config_example.yml` for `jmateBaseUrl`.
- Affected console service: Telegram inbox creation in `console/services/inboxservice.go`.
- Affected API/router layer: new webhook route under `/jmate/webhooks/telegram/:inbox_id`.
- Affected service layer: new Telegram webhook processing service and shared customer/ticket startup helper reused with the widget flow.
- Affected storage: existing `customers`, `customerinboxrels`, `tickets`, `inboxes`, and `inboxmembers` tables; no new tables expected.
- Affected external dependency: Telegram Bot API `getMe`, `deleteWebhook`, `setWebhook`, and incoming webhook payload format.
- Affected tests/docs: service/API tests for webhook setup, Telegram payload handling, customer/ticket reuse, and route behavior.
