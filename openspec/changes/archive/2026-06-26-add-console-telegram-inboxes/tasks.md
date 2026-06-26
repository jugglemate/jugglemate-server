## 1. Backend Storage And Contracts

- [x] 1.1 Add or extend inbox storage methods needed to list app-scoped inboxes, count members per inbox, replace inbox members, and validate member user IDs against the current app.
- [x] 1.2 Define console API models for inbox list items, Telegram inbox create requests, member list responses, and member replacement requests.
- [x] 1.3 Implement service helpers that map `storages/models.Inbox` to console response items while hiding or redacting sensitive bot token values.
- [x] 1.4 Serialize Telegram config through `services.TelegramChannelConf` and store it as JSON in `inboxes.channel_conf`.

## 2. Backend Console APIs

- [x] 2.1 Add admin-protected console routes under `/jmate/console` for listing inboxes and creating Telegram inboxes.
- [x] 2.2 Add admin-protected console routes for reading and replacing an inbox's assigned representatives.
- [x] 2.3 In create logic, require name, bot name, and bot token, set `channel_type` to `services.ChannelType_Telegram`, and generate `inbox_id` with `tools.GenerateUUIDShort22`.
- [x] 2.4 In member replacement logic, reject unknown inboxes and user IDs outside the current app before writing `inboxmembers`.
- [x] 2.5 Add backend tests for create validation, app scoping, Telegram config JSON, and member replacement behavior.

## 3. Console Web API Layer

- [x] 3.1 Add inbox schemas and endpoint constants in `console/web/src/api`.
- [x] 3.2 Add typed request helpers for inbox list, Telegram inbox creation, member lookup, and member replacement.
- [x] 3.3 Add or update mock handlers for local console development covering inbox list, create, and member update flows.

## 4. Console Web Inboxes UI

- [x] 4.1 Add Settings/Inboxes navigation entry and authenticated route structure under the console shell.
- [x] 4.2 Build the inbox list page with search, Telegram channel labeling, member count display, empty state, loading state, and create action.
- [x] 4.3 Build the Telegram setup form with fields for inbox name, bot name, and bot token, and submit it to the create API.
- [x] 4.4 Build the representative selection step using users from the current app and save selected user IDs to the inbox members API.
- [x] 4.5 Build a finish state that confirms setup and links back to the inbox list.
- [x] 4.6 Ensure the UI only exposes Telegram as a createable channel for this change.

## 5. Verification

- [x] 5.1 Run `go test ./...` and fix any backend regressions.
- [ ] 5.2 Run the console web typecheck/build command and fix any frontend regressions.
- [ ] 5.3 Manually verify the full flow: open Settings > Inboxes, create a Telegram inbox, assign representatives, return to the list, and confirm persisted rows in `inboxes` and `inboxmembers`.
- [ ] 5.4 Confirm bot token values are not displayed in the list or after creation.
