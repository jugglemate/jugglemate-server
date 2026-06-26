## 1. Backend Models And Storage

- [x] 1.1 Add `CreateWidgetInboxReq` model with `name` and optional `welcome_message` in `console/apis/models/inbox.go`.
- [x] 1.2 Extend inbox list/create response types so `channel_conf` can represent widget config (`welcome_message`) and redacted Telegram config.
- [x] 1.3 Update inbox storage query used by console list to return both `widget` and `telegram` inboxes for the current app (not telegram-only filter).

## 2. Backend Console APIs

- [x] 2.1 Implement `CreateWidgetInbox` in `console/services/inboxservice.go`: require name, accept optional `welcome_message`, set `channel_type=widget`, persist `channel_conf` as `WebWidgetChannelConf` JSON, generate `inbox_id`.
- [x] 2.2 Add `POST /jmate/console/inboxes/widget` handler and register route in `RouteConsole`.
- [x] 2.3 Update `QryInboxes` mapping so list items include correct `channel_type` labels, widget `welcome_message` in `channel_conf`, and widget rows do not require `bot_name`.
- [x] 2.4 Ensure member read/replace APIs accept widget inboxes (app-scoped validation only).
- [x] 2.5 Add backend tests for widget create with/without welcome message, list including widget rows, and member assignment on widget inbox.

## 3. Console Web API Layer

- [x] 3.1 Add `createWidgetInbox` endpoint constant and typed request/response schemas (`name`, `welcome_message`) in `console/web/src/api`.
- [x] 3.2 Update inbox list schema to handle widget `channel_conf.welcome_message` vs Telegram config.
- [x] 3.3 Add or update MSW mock handlers for widget create (with welcome message) and multi-channel list.

## 4. Console Web Inboxes UI (Chatwoot-style Flow)

- [x] 4.1 Refactor create flow to Step 0 channel selection with Widget and Telegram cards (layout reference: Chatwoot `ChannelList.vue`).
- [x] 4.2 Add Step 1 channel-specific config: Widget form with name + welcome message (optional textarea); Telegram existing form.
- [x] 4.3 Wire Widget submit to `POST /inboxes/widget` including `welcome_message`; keep Telegram on existing endpoint.
- [x] 4.4 Update inbox list table: channel badge/label by `channel_type`, safe subtitle for widget rows (e.g. welcome message preview or channel label, no `bot_name` assumption).
- [x] 4.5 Show created `inbox_id` on finish step for widget copy/integration with `/customers/start`.
- [x] 4.6 Reuse existing representative selection and finish steps for both channel types.

## 5. Verification

- [x] 5.1 Run `go test` for console inbox and related packages.
- [x] 5.2 Run console web typecheck/build.
- [x] 5.3 Manual test: create widget inbox with welcome message → assign reps → verify list → call `/customers/start` and confirm `welcome_message` in response.
- [x] 5.4 Manual test: create widget inbox without welcome message → `/customers/start` returns empty `welcome_message`.
- [x] 5.5 Manual test: Telegram create flow still works end-to-end after channel selection refactor.
