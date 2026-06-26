## Why

Console operators need a settings workflow for creating Telegram inboxes and assigning customer-service representatives, similar to Chatwoot's Settings > Inboxes experience. The backend already has `inboxes` and `inboxmembers` tables plus a Telegram channel configuration type, but the console has no UI or API flow to manage them.

## What Changes

- Add console APIs for listing inboxes, creating Telegram inboxes, reading/updating inbox members, and selecting users as representatives.
- Add a console Settings/Inboxes page that follows the Chatwoot inbox management flow for Telegram only.
- Persist Telegram bot configuration as JSON in `inboxes.channel_conf` using `services.TelegramChannelConf`.
- Persist assigned representatives in `inboxmembers` using user IDs selected from the `users` table.
- Generate `inbox_id` server-side with `tools.GenerateUUIDShort22`.
- Keep the implementation scoped to Telegram; other channel types remain out of scope.

## Capabilities

### New Capabilities
- `console-telegram-inbox-management`: Console administrators can create Telegram inboxes, store Telegram bot configuration, and assign users as inbox members.

### Modified Capabilities

## Impact

- Backend console API routes and models under `console/apis` and `console/apis/models`.
- Backend service/storage use of `inboxes`, `inboxmembers`, and `users`.
- Frontend console routes, schemas, API client modules, menu/navigation, and mock handlers under `console/web`.
- Reference implementation research from `/Users/yuwnloy/Documents/github/chatwoot` Settings > Inboxes and Telegram channel flow.
