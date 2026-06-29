## 1. Channel Model And Bot Setup Seams

- [x] 1.1 Add `ChannelType_JuggleIM` and `JuggleIMChannelConf` with parse helpers alongside existing channel types.
- [x] 1.2 Add a small placeholder JuggleIM bot client/helper modeled after `commons/telegram`, with injectable setup/validation functions and no concrete external SDK calls.
- [x] 1.3 Add callback URL construction for `strings.TrimRight(jmateBaseUrl, "/") + "/jmate/webhooks/juggleim/" + inbox_id`.
- [x] 1.4 Ensure missing `jmateBaseUrl`, bot name, or bot token rejects JuggleIM inbox creation before persistence.
- [x] 1.5 Ensure JuggleIM bot setup seam failures reject inbox creation without leaving a usable inbox record.

## 2. Console Inbox Backend

- [x] 2.1 Add console API request/response config models for JuggleIM inbox creation and redacted listing.
- [x] 2.2 Add `POST /jmate/console/inboxes/juggleim` route and handler.
- [x] 2.3 Implement `CreateJuggleIMInbox` in console services using the JuggleIM bot setup seam and existing inbox persistence.
- [x] 2.4 Update inbox list conversion to parse/redact JuggleIM channel config.
- [x] 2.5 Update inbox storage queries or supported channel filters so JuggleIM inboxes appear in console inbox lists.
- [x] 2.6 Keep representative assignment APIs working for JuggleIM inbox IDs without channel-specific changes.

## 3. Console Web Inbox Flow

- [x] 3.1 Add JuggleIM channel schemas, API client method, and type union entries.
- [x] 3.2 Add JuggleIM channel selection card and setup form using `bot_name` and `bot_token`.
- [x] 3.3 Add JuggleIM display formatting for inbox list rows and search.
- [x] 3.4 Add mock API handling and mock data for JuggleIM inboxes.
- [x] 3.5 Add English and Chinese i18n labels for JuggleIM channel setup, success, and failure states.

## 4. JuggleIM Webhook Endpoint And Payload Parsing

- [x] 4.1 Add `POST /jmate/webhooks/juggleim/:inbox_id` routing outside authenticated middleware.
- [x] 4.2 Add API handler that reads the request body, delegates to a JuggleIM webhook service, and returns webhook-friendly success/no-op responses.
- [x] 4.3 Add request models for `CustomChatMsgReq` and `MentionInfo` fields: `app_key`, `sender`, `receiver`, `conver_type`, `msg_type`, `msg_content`, `msg_id`, `msg_time`, and optional `mention_info`.
- [x] 4.4 Resolve webhook inbox by `inbox_id`, verify it is a JuggleIM inbox, and derive `app_key` from the inbox row.
- [x] 4.5 Ignore invalid JSON, missing sender, missing content, unknown inbox, ambiguous inbox, and non-JuggleIM inbox callbacks without creating customers, tickets, or IM messages.

## 5. JuggleIM Visitor Message Processing

- [x] 5.1 Add JuggleIM inbox validation to the shared customer/ticket startup flow.
- [x] 5.2 Use callback `sender` as the customer identity while generating relation source IDs with the Web-aligned `customer_ + GenerateUUIDShort22()` policy.
- [x] 5.3 For supported callbacks, create or reuse customer, customer-inbox relation, IM user, ticket group, and ticket.
- [x] 5.4 Send supported callback message content into the ticket group with sender equal to relation source ID and target equal to `ticket_id`.
- [x] 5.5 Preserve callback `msg_type`, `msg_content`, and `msg_id` when sending the group IM message.
- [x] 5.6 Preserve idempotency for repeated callbacks by reusing customer and ticket records for the same sender identity.

## 6. Tests And Documentation

- [x] 6.1 Add console service tests for successful JuggleIM inbox creation, redacted listing, missing config, missing base URL, and setup seam failure.
- [x] 6.2 Add console API tests for the JuggleIM inbox creation route.
- [x] 6.3 Add JuggleIM webhook service tests for supported callback, invalid payload, missing sender/content, unknown inbox, non-JuggleIM inbox, and existing customer/ticket reuse.
- [x] 6.4 Add webhook API handler tests for `/webhooks/juggleim/:inbox_id`.
- [x] 6.5 Add console web tests or type checks covering JuggleIM schema/API/form integration where the existing project supports them.
- [x] 6.6 Update relevant API docs or README snippets with JuggleIM inbox setup and webhook callback behavior.

## 7. Verification

- [x] 7.1 Run focused Go tests for console inbox services, webhook services, and APIs.
- [x] 7.2 Run `go test ./...`.
- [x] 7.3 Run console web lint/typecheck/test commands if available.
- [x] 7.4 Run `openspec validate add-juggleim-inbox-webhook-integration --strict`.
- [x] 7.5 Manually review that JuggleIM webhook route bypasses authenticated middleware while regular console/customer APIs remain protected.
- [x] 7.6 Manually review source ID behavior to confirm JuggleIM uses `customer_` generated IDs and not callback `sender` directly.
