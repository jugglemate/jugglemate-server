## Context

The console can create `telegram` inbox rows with `TelegramChannelConf` containing a bot name and bot token, and widget visitors already enter the customer/ticket flow through `POST /jmate/customers/start`. That widget flow validates the inbox, creates or reuses a customer and customer-inbox relation, registers the visitor as an IM user, creates a ticket group using `ticket_id`, adds the visitor and inbox members, and persists the ticket.

Chatwoot's Telegram integration validates the bot token with `getMe`, configures Telegram by calling `deleteWebhook` and `setWebhook`, exposes a webhook endpoint, acknowledges Telegram quickly, and processes only supported private message updates into a contact/conversation. JuggleMate should follow the same high-level approach while mapping Telegram users into the existing customer/ticket model.

The requested callback URL is `jmateBaseUrl + /jmate/webhooks/telegram/:inbox_id`. Telegram callbacks cannot provide the normal authenticated `appkey` header, so the webhook route must resolve the inbox and app from the path parameter instead of using the existing authenticated route group middleware.

## Goals / Non-Goals

**Goals:**
- Configure Telegram bot webhooks when a Telegram inbox is created.
- Use `jmateBaseUrl` from config to build the Telegram callback URL.
- Receive Telegram webhook updates at `POST /jmate/webhooks/telegram/:inbox_id`.
- Use the Telegram user numeric ID as the customer identity.
- Reuse the widget customer/ticket creation behavior for Telegram visitors.
- Forward supported incoming Telegram text messages into the ticket group as IM group messages.
- Keep Telegram delivery reliable by acknowledging unsupported or duplicate webhook updates without surfacing internal errors to Telegram when possible.

**Non-Goals:**
- Do not build full Telegram outbound delivery from agent replies back to Telegram in this change.
- Do not support Telegram group chats or channels; private user chats are the first supported scope.
- Do not add new database tables.
- Do not implement attachment download/upload for Telegram media in the first pass.
- Do not change the console inbox setup UI flow except for surfacing webhook setup success/failure if needed by existing API responses.

## Decisions

1. **Set the webhook during Telegram inbox creation.**
   - Use the stored bot token to call Telegram `getMe` and validate the token/bot identity.
   - Call `deleteWebhook` before `setWebhook`, following Chatwoot's conservative setup pattern.
   - Configure the webhook URL as `strings.TrimRight(jmateBaseUrl, "/") + "/jmate/webhooks/telegram/" + inbox_id`.
   - Alternative considered: let administrators configure the webhook manually. That would be more error-prone and would not satisfy the requested automatic integration.

2. **Route Telegram webhooks outside the authenticated API middleware.**
   - Telegram cannot send the `appkey` and authorization headers expected by `apis.Validate`.
   - The handler will locate the inbox by `inbox_id`, verify `channel_type == telegram`, parse `TelegramChannelConf`, and derive `app_key` from the inbox row.
   - Alternative considered: include `appkey` in the callback URL. The requested route shape does not include it, and exposing app keys in webhook URLs is not necessary if inbox IDs are generated and looked up server-side.

3. **Extract Telegram visitor identity from supported private message payloads.**
   - For `message` updates, use `message.from.id` as the customer identity and `message.chat.id` as the Telegram chat ID.
   - Process only `message.chat.type == "private"` for initial support.
   - Use the same visitor source ID policy as widget channels: `customer_` plus a generated UUID-short value.
   - Alternative considered: use `chat.id` as identity. For private chats it usually matches the user ID, but `from.id` is the stable user identity requested by the user.

4. **Share customer/ticket startup behavior instead of duplicating widget code.**
   - Extract a reusable helper from `StartWebCustom` that accepts channel type, inbox ID, identity, nickname/avatar, source ID policy, and optional response extras.
   - Keep widget validation as `channel_type == widget` and Telegram validation as `channel_type == telegram`.
   - Preserve the existing ticket group behavior: group ID is `ticket_id`; members are visitor source ID plus inbox members.
   - Alternative considered: copy the widget flow into a Telegram service. That would drift quickly and make future ticket behavior fixes harder.

5. **Represent incoming Telegram messages as IM group messages.**
   - After the customer/ticket exists, send the Telegram text into the ticket group with sender equal to the visitor source ID, target equal to `ticket_id`, and group conversation type.
   - Use a text IM message type/content consistent with existing IM usage.
   - Store or pass Telegram `message_id` as an idempotency key where the SDK/storage surface supports it; at minimum, tests should prevent duplicate ticket creation for repeated updates.
   - Alternative considered: persist Telegram messages directly in local storage only. Agents already work from IM conversations, so incoming Telegram text must enter the IM group.

## Risks / Trade-offs

- [Risk] `inbox_id` is only declared unique with `app_key` at the database level. -> Add a storage query for Telegram webhook lookup by `inbox_id`; if multiple rows are ever returned, fail closed and log a configuration error.
- [Risk] Telegram retries webhook delivery when the server returns an error. -> Acknowledge unsupported payloads and known no-op cases with success; reserve errors for invalid inbox/config where retrying might be useful during setup.
- [Risk] Bot token validation and webhook setup can make inbox creation slower or dependent on Telegram availability. -> Use short HTTP timeouts and return a clear request/internal error if setup fails.
- [Risk] Text-only support may surprise users who send media. -> Explicitly log and acknowledge unsupported message types; add attachment handling in a later change.
- [Risk] Shared startup refactor could regress widget `/customers/start`. -> Cover widget behavior with existing and new regression tests while adding Telegram-specific tests.

## Migration Plan

1. Add `jmateBaseUrl` to config structs and sample config if not already present.
2. Add Telegram Bot API client helpers with injectable functions for tests.
3. Update Telegram inbox creation to validate the token and set the webhook URL.
4. Add webhook routing outside the authenticated route group and implement Telegram payload parsing/service handling.
5. Refactor the widget start flow only as needed to share customer/ticket creation behavior.
6. Run focused console/customer/webhook tests, then `go test ./...`.

Rollback is code-only: revert webhook setup and route changes. Existing `telegram` inbox rows remain valid; administrators may reset the Telegram webhook manually or recreate the inbox after rollback.

## Open Questions

- Should the implementation add Telegram `secret_token` support and store a generated secret in `channel_conf` for stronger webhook verification, or keep the first pass to the requested `:inbox_id` callback URL only?
- Should non-text Telegram messages create tickets without forwarding a group message, or simply acknowledge and log until media support is added?
