## Context

The server now has a shared customer/ticket startup path used by Web visitors and Telegram webhook visitors. Console inbox management stores channel-specific configuration in `inboxes.channel_conf`, redacts bot tokens in responses, and routes Telegram callbacks through a public `/jmate/webhooks/telegram/:inbox_id` endpoint outside the authenticated API middleware.

The new JuggleIM channel should follow the same product shape as Telegram: administrators configure an inbox with `bot_name` and `bot_token`, representatives are assigned through the existing inbox member APIs, and incoming callbacks create or reuse ticket conversations. The user specifically requested that JuggleIM bot SDK initialization temporarily follow the Telegram bot client shape but not implement concrete SDK behavior yet.

## Goals / Non-Goals

**Goals:**

- Add a `juggleim` inbox channel to backend console APIs and the console web setup flow.
- Store JuggleIM channel config as JSON with `bot_name` and `bot_token`, and redact `bot_token` in list/create responses.
- Add placeholder JuggleIM bot setup helpers with injectable seams for future validation/webhook setup.
- Add public `POST /jmate/webhooks/juggleim/:inbox_id` routing.
- Parse `CustomChatMsgReq` callbacks and route supported customer messages into the existing ticket workflow.
- Preserve source ID generation alignment with Web/Telegram visitor channels: `customer_ + GenerateUUIDShort22()`.

**Non-Goals:**

- Implement a real JuggleIM bot SDK client or call an external JuggleIM bot management API.
- Add new database tables or migrations beyond reusing existing `inboxes`, `inboxmembers`, customers, relations, and tickets.
- Support non-message callback types beyond the provided `CustomChatMsgReq` shape.
- Change existing Telegram or Widget behavior except where shared channel handling must include JuggleIM.

## Decisions

1. **Represent JuggleIM as another inbox channel type.**
   - Add `ChannelType_JuggleIM` and `JuggleIMChannelConf` beside existing Widget/Telegram channel config types.
   - Keep the same `bot_name`/`bot_token` JSON field names as Telegram to reduce console API and UI divergence.
   - Alternative considered: store JuggleIM config in a provider-specific structure. That adds no value while the required fields are identical.

2. **Model JuggleIM bot setup as injectable no-op seams.**
   - Add a small `commons/juggleim` client shape analogous to `commons/telegram`, with a bot info response and setup method names that can be faked in tests.
   - The default implementation should not perform concrete external API calls until the real SDK contract exists; it should validate required inputs and return success.
   - Alternative considered: skip client helpers entirely. That would make later real SDK integration harder and would not match the Telegram implementation pattern requested.

3. **Register webhook route outside authenticated middleware.**
   - Add `/jmate/webhooks/juggleim/:inbox_id` next to Telegram's public webhook route.
   - Resolve the inbox by `inbox_id` without `appkey` and derive `app_key` from the inbox row.
   - Reject unknown, ambiguous, or non-JuggleIM inboxes without side effects.

4. **Use `CustomChatMsgReq` sender as visitor identity.**
   - Treat `sender` as the visitor identity used to find or create `customers.identifier` in the inbox app scope.
   - Generate customer relation `source_id` via the same random `customer_` policy used by Web and Telegram.
   - Use `receiver` only as callback context for future extension; it is not the ticket target.
   - Alternative considered: use `sender` directly as `source_id`. That would diverge from the user's requested Web-aligned source ID rule.

5. **Forward supported message callbacks into the ticket group.**
   - For supported payloads, call the shared customer/ticket startup helper with `ChannelType_JuggleIM`.
   - Send an IM group message with sender equal to the relation `source_id`, target equal to `ticket_id`, `MsgType` from the callback, `MsgContent` from the callback, and idempotency `MsgId` derived from callback `msg_id`.
   - No-op callbacks and unsupported message types should return success to avoid retry loops.

6. **Extend console web in the existing inbox setup flow.**
   - Add `juggleim` to the channel union, channel selection, schemas, mock handlers, labels, and display formatting.
   - Reuse the Telegram-like form fields with JuggleIM-specific text labels.

## Risks / Trade-offs

- [Risk] Placeholder bot setup may appear to validate credentials even though no real SDK call occurs. -> Keep the service and tests explicit that the helper is a seam/no-op until concrete SDK integration is available.
- [Risk] Public webhook can be called without auth. -> Scope strictly by stored inbox config and only process callbacks for a unique JuggleIM inbox ID.
- [Risk] Callback `MsgType` semantics may evolve. -> Initially support the provided custom chat callback shape and text/custom message forwarding, with tests covering unsupported/no-op behavior.
- [Risk] Shared ticket startup changes could regress Widget/Telegram. -> Keep changes additive, reuse existing helpers, and run console/services/apis test suites.

## Migration Plan

1. Add channel type/config parsing and console API models.
2. Add JuggleIM inbox creation service/API route and console web support.
3. Add JuggleIM webhook handler/service and route.
4. Add unit tests for inbox creation/listing and webhook processing.
5. Run focused Go tests, console web checks if available, and broader Go tests.

Rollback is removing the JuggleIM routes/channel creation paths and leaving existing Widget/Telegram records untouched.

## Open Questions

- What exact JuggleIM bot SDK initialization and webhook registration API should replace the placeholder client once available?
- Which `MsgType` values beyond text/custom chat should be considered supported in the first real integration?
- Should future callbacks validate a signature or token from JuggleIM once the upstream webhook contract is finalized?
