## 1. Configuration And Telegram Bot Setup

- [x] 1.1 Ensure `jmateBaseUrl` is available in `commons/configures.AppConfig`, `conf/config.yml`, and `conf/config_example.yml`.
- [x] 1.2 Add a small Telegram Bot API client/helper with injectable functions for `getMe`, `deleteWebhook`, and `setWebhook`.
- [x] 1.3 Update Telegram inbox creation to validate bot token via Telegram `getMe` before persisting the inbox.
- [x] 1.4 Build the callback URL as `strings.TrimRight(jmateBaseUrl, "/") + "/jmate/webhooks/telegram/" + inbox_id`.
- [x] 1.5 Configure the Telegram webhook after generating the inbox ID and before returning success.
- [x] 1.6 Ensure failed token validation or webhook setup does not leave a usable Telegram inbox record.

## 2. Shared Customer And Ticket Startup Flow

- [x] 2.1 Extract reusable customer/inbox/ticket startup logic from `services.StartWebCustom` without changing the widget API response contract.
- [x] 2.2 Keep widget validation restricted to `channel_type=widget`.
- [x] 2.3 Add Telegram inbox validation restricted to `channel_type=telegram`.
- [x] 2.4 Add Telegram source ID generation aligned with Web channel, using `customer_` plus `GenerateUUIDShort22()`.
- [x] 2.5 Reuse the existing ticket group creation behavior: group ID equals `ticket_id`, members include visitor source ID plus inbox members.
- [x] 2.6 Add tests proving widget `/customers/start` behavior still works after the shared-flow refactor.

## 3. Telegram Webhook Endpoint And Payload Parsing

- [x] 3.1 Add `POST /jmate/webhooks/telegram/:inbox_id` routing outside the normal authenticated API middleware.
- [x] 3.2 Add request models or parser helpers for Telegram updates used by this change: `message`, `from`, `chat`, `text`, `caption`, and `message_id`.
- [x] 3.3 Resolve webhook inbox by `inbox_id`, verify it is a Telegram inbox, and derive `app_key` from the inbox row.
- [x] 3.4 Reject or acknowledge unknown, ambiguous, or non-Telegram inbox lookups without creating customers or tickets.
- [x] 3.5 Ignore non-private Telegram chats without creating customers, tickets, or IM messages.
- [x] 3.6 Extract Telegram identity from `message.from.id` and display name from `first_name`, `last_name`, or `username`.
- [x] 3.7 Acknowledge unsupported Telegram update types and unsupported message bodies without sending IM messages.

## 4. Telegram Visitor Message Processing

- [x] 4.1 For supported private Telegram text/caption messages, create or reuse the customer, customer-inbox relation, IM user, ticket group, and ticket.
- [x] 4.2 Send the Telegram text/caption into the ticket group using the Telegram visitor source ID as sender and `ticket_id` as group target.
- [x] 4.3 Preserve idempotency for repeated Telegram updates by reusing customer and ticket records for the same Telegram identity.
- [x] 4.4 Log processing failures and return webhook responses that avoid unnecessary Telegram retry loops for unsupported/no-op cases.

## 5. Tests And Documentation

- [x] 5.1 Add console service tests for successful Telegram inbox creation with webhook setup.
- [x] 5.2 Add console service tests for invalid bot token, empty `jmateBaseUrl`, and webhook setup failure.
- [x] 5.3 Add webhook API/service tests for unknown inbox, non-Telegram inbox, non-private chat, unsupported payload, and supported text message.
- [x] 5.4 Add service tests that Telegram user numeric ID maps to customer identity while source ID uses the Web channel format.
- [x] 5.5 Add service tests that repeated Telegram messages reuse the same customer/ticket workflow state.
- [x] 5.6 Update relevant API docs or README snippets with Telegram webhook URL behavior and config requirements.

## 6. Verification

- [x] 6.1 Run focused Go tests for console inbox creation, customer service, and Telegram webhook handling.
- [x] 6.2 Run `go test ./...`.
- [x] 6.3 Manually review the route registration to confirm Telegram webhooks bypass authenticated middleware while regular APIs remain protected.
- [x] 6.4 Manually review the configured Telegram webhook URL shape against `jmateBaseUrl` and `/jmate/webhooks/telegram/:inbox_id`.
