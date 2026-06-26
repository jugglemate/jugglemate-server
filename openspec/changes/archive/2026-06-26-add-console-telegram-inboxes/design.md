## Context

The console already has authenticated admin-only routes under `/jmate/console`, a Users CRUD page backed by `users`, and embedded React routes under `/jmateconsole`. Recent storage work added `inboxes` and `inboxmembers` matching the current database shape:

- `inboxes`: `inbox_id`, `channel_type`, `channel_conf`, `name`, timestamps, `app_key`
- `inboxmembers`: `inbox_id`, `member_id`, `created_time`, `app_key`

Chatwoot's Settings > Inboxes flow provides the product reference: list inboxes, choose a channel, configure the channel, add agents, and finish setup. This project should migrate that experience for Telegram only. Telegram channel definitions live in `services/channeltype.go`; `TelegramChannelConf` contains `bot_name` and `bot_token`, and the serialized JSON belongs in `inboxes.channel_conf`.

## Goals / Non-Goals

**Goals:**
- Add admin-only console APIs for Telegram inbox list/create and inbox member assignment.
- Create a console Settings/Inboxes UI modeled on Chatwoot's list, channel creation, add representatives, and finish flow.
- Persist Telegram inboxes using the existing `inboxes` table with `channel_type = "telegram"` and `channel_conf` JSON produced from `services.TelegramChannelConf`.
- Persist selected representatives into `inboxmembers` using `users.user_id` values.
- Generate `inbox_id` server-side with `tools.GenerateUUIDShort22`.

**Non-Goals:**
- Implement non-Telegram channels.
- Implement Telegram webhook registration, message sending, or incoming webhook processing.
- Change the database schema beyond using the current `inboxes`, `inboxmembers`, and `users` tables.
- Reproduce all Chatwoot settings such as auto-assignment, SLA, business hours, avatar upload, deletion confirmation text, or webhook health.

## Decisions

1. Add a new `console-telegram-inbox-management` capability rather than modifying existing console user specs.
   - Rationale: Inbox management is a separate admin workflow and introduces new API/UI behavior.
   - Alternative considered: Fold representative selection into `console-user-management`. Rejected because users are only the selectable source; the managed resource is an inbox.

2. Store Telegram config as serialized `services.TelegramChannelConf` in `inboxes.channel_conf`.
   - Rationale: The database has a generic `channel_conf` field and the user explicitly requested assignment through `TelegramChannelConf`.
   - Alternative considered: Add typed columns for bot name/token. Rejected because it diverges from the current database and would require a schema migration not requested here.

3. Use `"telegram"` from `services.ChannelType_Telegram` as the persisted `channel_type`.
   - Rationale: The code already defines channel constants in `services/channeltype.go`, and the request scopes this change to Telegram only.
   - Alternative considered: Store Chatwoot-style `Channel::Telegram`. Rejected because it would not match this project's channel definitions.

4. Keep bot token validation local to required-field validation initially.
   - Rationale: Chatwoot calls Telegram `getMe` during creation, but this project does not yet have Telegram webhook or outbound API infrastructure. The implementation can still accept `bot_name` and `bot_token` and persist them deterministically.
   - Alternative considered: Call Telegram from the create endpoint to derive `bot_name`. Rejected for this proposal because it introduces external network failure modes and secret handling concerns beyond the requested persistence flow.

5. Represent inbox members as a full replacement set.
   - Rationale: Chatwoot's inbox member API updates the assigned agent list in one call. A full replacement endpoint avoids stale assignments and maps cleanly to a multi-select UI.
   - Alternative considered: Add one endpoint per add/remove operation. Rejected because it complicates the frontend flow without adding value for the first Telegram-only iteration.

6. Reuse the existing console Users list endpoint for representative selection where practical.
   - Rationale: Users are already app-scoped and admin-protected. The UI can filter/select customer-service and admin accounts without introducing a second user source.
   - Alternative considered: Add a specialized assignable-users endpoint. Acceptable later if the Users list payload becomes too heavy, but not necessary for the initial implementation.

## Risks / Trade-offs

- [Risk] `channel_conf` stores `bot_token` as plaintext JSON. → Treat it as sensitive in UI display: never render the stored token back in full after creation, and avoid logging request bodies containing tokens.
- [Risk] No live Telegram token validation means an invalid token can be saved. → Show clear form validation for required fields and leave external validation as a future enhancement.
- [Risk] Full replacement member updates can remove users if the UI sends a stale list. → Fetch current members before editing and disable save while update is in flight.
- [Risk] Existing untracked storage/SQL work is still in progress. → Implement against the current database-backed storage interfaces and verify with `go test ./...` plus console build.

## Migration Plan

1. Keep the existing database schema as the source of truth.
2. Add backend console handlers, models, services, and storage methods without destructive migrations.
3. Add frontend routes and menu entries under the authenticated console shell.
4. Validate with backend tests and console build.
5. Rollback by removing the new routes/UI; existing persisted inbox rows remain inert if no code consumes them.

## Open Questions

- Should bot token validation against Telegram `getMe` be added in a follow-up once webhook registration is implemented?
- Should admins be allowed to edit an existing Telegram bot token, or should the first version only support create and member updates?
