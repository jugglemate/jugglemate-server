## Why

Console administrators can already configure Telegram and Widget inboxes, but there is no first-class JuggleIM channel for routing bot-originated customer messages into the ticket workflow. Adding a JuggleIM inbox channel lets the management console configure JuggleIM bot credentials and lets public callbacks create or reuse customer tickets consistently with existing webhook integrations.

## What Changes

- Add a JuggleIM inbox channel to console inbox management with `bot_name` and `bot_token` configuration fields.
- Add JuggleIM bot SDK setup seams modeled after the Telegram bot client, with placeholder initialization behavior until the concrete SDK integration is available.
- Add public webhook endpoint `POST /jmate/webhooks/juggleim/:inbox_id`.
- Process JuggleIM custom chat callback payloads into the existing customer, customer-inbox relation, IM user, ticket group, and ticket workflow.
- Forward supported JuggleIM text/custom chat messages into the ticket group using the visitor source ID as sender and `ticket_id` as the group conversation target.
- Ignore unsupported/no-op callbacks without creating customers, tickets, or IM messages.

## Capabilities

### New Capabilities
- `juggleim-webhook-processing`: Defines how public JuggleIM webhook callbacks map JuggleIM senders to customers and ticket group messages.

### Modified Capabilities
- `console-telegram-inbox-management`: Extend console inbox management from Widget/Telegram to also support JuggleIM inbox creation and listing behavior.

## Impact

- Console APIs and services for inbox creation/listing.
- Console web inbox setup flow, channel selection, request schemas, mocks, and i18n labels.
- Channel configuration parsing and channel type handling.
- Public router and API handler for `/jmate/webhooks/juggleim/:inbox_id`.
- New JuggleIM webhook service and tests, modeled after Telegram webhook processing.
- OpenSpec main specs for console inbox management and new JuggleIM webhook processing.
