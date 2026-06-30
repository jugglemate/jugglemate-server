## Why

Ticket conversations can receive agent messages in the IM group, but outbound channel delivery is not yet connected for those group messages. Telegram visitors need replies from ticket groups to be forwarded back to Telegram while avoiding loops for messages originally sent by the visitor source user.

## What Changes

- Extend `WebhookMsgs` message subscription handling to inspect each incoming IM message payload.
- Process only group messages where `conver_type=2` and the group ID in `receiver` starts with `ticket_`.
- Treat the receiver group ID as `ticketId`, load the ticket, then load the ticket's inbox to determine channel behavior.
- Skip forwarding when the incoming message sender equals the ticket `source_id`.
- For Telegram inboxes, send the message back to Telegram through a Telegram SDK/client helper using the stored Telegram bot configuration.
- Ignore unsupported inbox channels, non-ticket groups, private messages, malformed payloads, and lookup misses without breaking webhook acknowledgement.

## Capabilities

### New Capabilities
- `ticket-group-outbound-channel-forwarding`: Defines how ticket group messages from IM webhook subscriptions are routed back to the ticket's external channel, starting with Telegram.

### Modified Capabilities
- `telegram-webhook-processing`: Extends Telegram channel behavior so ticket group replies can be delivered back to Telegram visitors, complementing inbound Telegram webhook processing.

## Impact

- `apis.WebhookMsgs` and related message subscription parsing.
- Ticket and inbox storage lookups from webhook processing.
- Telegram channel configuration parsing and Telegram API/client helpers for outbound sends.
- Tests for group-message filtering, loop prevention, ticket/inbox resolution, Telegram send success/failure, and no-op cases.
- API docs or webhook docs describing outbound ticket group forwarding behavior.
