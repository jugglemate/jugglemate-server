## Context

`WebhookMsgs` is the public message subscription endpoint for IM message callbacks. It currently parses the message event payload and logs each item, but does not route ticket group messages back to external visitor channels.

Tickets are represented by IM group conversations whose group ID is the ticket ID. The current ticket creation flow uses a `ticket_` prefix for generated ticket IDs, stores the visitor IM user ID in `tickets.source_id`, and stores the channel inbox in `tickets.inbox_id`. Telegram inbox configuration is stored in `inboxes.channel_conf` as `TelegramChannelConf`.

## Goals / Non-Goals

**Goals:**

- Extend `WebhookMsgs` handling to process ticket group message callbacks.
- Only process group messages where `conver_type=2` and `receiver` starts with `ticket_`.
- Use `receiver` as `ticketId`, load the ticket, then load the ticket's inbox.
- Prevent loops by skipping messages where callback `sender` equals `ticket.source_id`.
- For Telegram inboxes, send the message back to Telegram using the Telegram bot configuration.
- Keep webhook responses friendly to the IM callback system: unsupported/no-op messages should be acknowledged without failing the whole payload.

**Non-Goals:**

- Add outbound delivery for non-Telegram channels in this change.
- Change inbound Telegram visitor processing.
- Change ticket group creation, ticket ID generation, or ticket claim behavior.
- Implement rich media conversion beyond forwarding supported message content to Telegram.

## Decisions

1. **Process outbound forwarding from `WebhookMsgs`, not channel webhook handlers.**
   - `WebhookMsgs` receives IM message subscription callbacks, which is the correct point to observe agent replies posted into ticket groups.
   - Telegram/JuggleIM inbound handlers remain responsible for visitor-to-ticket messages only.

2. **Use `receiver` as the ticket group ID.**
   - For group messages, the callback receiver is the group conversation target.
   - The system will require `conver_type=2` and `strings.HasPrefix(receiver, "ticket_")` before doing ticket lookups.
   - Alternative considered: parse ticket ID from message content or sender. That would be less reliable than the group target.

3. **Prevent visitor echo loops using `ticket.source_id`.**
   - If callback `sender == ticket.source_id`, the message originated from the external visitor source user already bridged into the ticket group.
   - The handler must no-op in this case so inbound visitor messages do not get sent back to Telegram.

4. **Resolve channel behavior from the ticket inbox.**
   - After loading the ticket, load the inbox by `ticket.app_key` and `ticket.inbox_id`.
   - Dispatch based on `inbox.channel_type`. This keeps the design extensible for future channel outbound delivery.
   - Unsupported channel types are acknowledged without outbound send.

5. **Add Telegram outbound send seam.**
   - Extend the Telegram helper with an injectable `SendMessage`-style function or service seam that uses `bot_token` plus the external Telegram chat/user target.
   - The implementation needs a reliable Telegram target. Because `tickets.source_id` is an IM user ID like `customer_...`, the Telegram numeric identity is not directly available from the ticket alone. The outbound flow should derive the Telegram target by loading the ticket's customer and using `customers.identifier`, which inbound Telegram processing stores as Telegram `from.id`.
   - If customer lookup or Telegram target resolution fails, log and acknowledge without retrying the entire callback payload unless a storage error occurs.

6. **Payload handling is per message item.**
   - A single webhook request can contain multiple message payloads.
   - Each payload should be evaluated independently; one no-op item should not stop later items.
   - Actual storage or Telegram send failures should be surfaced according to existing error handling policy, but the webhook should avoid unnecessary callback retry loops for unsupported payloads.

## Risks / Trade-offs

- [Risk] Telegram target is not stored on ticket directly. -> Use the ticket's customer identifier, which inbound Telegram visitors already set to the Telegram numeric user ID.
- [Risk] Some IM message types may not map cleanly to Telegram text. -> Start with text-compatible content extraction and add tests for unsupported or empty content no-ops.
- [Risk] Outbound forwarding could loop if visitor-originated group messages are not detected. -> Explicitly skip when callback sender equals `ticket.source_id`.
- [Risk] Multi-message payloads may partially fail. -> Process items independently and log enough context for failed items.

## Migration Plan

1. Add outbound forwarding service seams for ticket/inbox/customer storage and Telegram send.
2. Refactor `WebhookMsgs` to delegate parsed message payloads to a service while preserving current successful acknowledgement behavior.
3. Add Telegram outbound helper method/seam.
4. Add tests for filtering, loop prevention, ticket/inbox lookup, Telegram send, unsupported channel no-op, and multi-payload behavior.
5. Run focused API/service tests and `go test ./...`.

Rollback is removing the new delegation from `WebhookMsgs`; existing inbound webhooks and ticket workflows remain unchanged.

## Open Questions

- What exact Telegram SDK method shape should be used for outbound send once the concrete client abstraction is finalized?
- Should outbound non-text IM message content be converted to Telegram captions/media in later changes?
