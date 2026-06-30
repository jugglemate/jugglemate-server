## 1. WebhookMsgs Delegation And Payload Handling

- [x] 1.1 Identify or add reusable message callback payload structs for `WebhookMsgs` without breaking existing parsing.
- [x] 1.2 Refactor `apis.WebhookMsgs` to delegate parsed payload items to a service layer seam while preserving success acknowledgement for malformed request bodies.
- [x] 1.3 Ensure each payload item in a multi-message request is evaluated independently.
- [x] 1.4 Keep logging context for skipped and failed payloads including sender, receiver, conver_type, msg_type, and msg_id.

## 2. Ticket Group Eligibility And Loop Prevention

- [x] 2.1 Filter outbound forwarding to messages with `conver_type=2`.
- [x] 2.2 Filter outbound forwarding to group receiver IDs starting with `ticket_`.
- [x] 2.3 Treat the receiver group ID as `ticketId` and load the corresponding ticket.
- [x] 2.4 Skip processing when no ticket is found.
- [x] 2.5 Skip processing when payload sender equals the loaded ticket `source_id`.
- [x] 2.6 Add tests for private messages, non-ticket groups, missing tickets, and visitor-source loop prevention.

## 3. Channel Resolution

- [x] 3.1 Load the ticket inbox using the ticket app key and inbox ID.
- [x] 3.2 Skip processing when the inbox is missing.
- [x] 3.3 Dispatch outbound forwarding by inbox `channel_type`.
- [x] 3.4 Skip unsupported channel types without failing the whole webhook request.
- [x] 3.5 Add tests for missing inbox and unsupported channel no-op behavior.

## 4. Telegram Outbound Sending

- [x] 4.1 Extend the Telegram helper/client with an injectable outbound send function for bot token, Telegram target, and message text.
- [x] 4.2 Resolve the Telegram target from the ticket customer identifier, not from ticket `source_id`.
- [x] 4.3 Parse Telegram inbox channel config and skip when bot token is missing.
- [x] 4.4 Extract text-compatible content from supported IM message payloads.
- [x] 4.5 Send eligible ticket group messages to Telegram using the resolved target and bot token.
- [x] 4.6 Add tests for successful Telegram send, missing bot token, missing customer/identifier, empty content, and send failure behavior.

## 5. API And Service Tests

- [x] 5.1 Add `WebhookMsgs` API tests proving the handler delegates parsed message payloads and acknowledges malformed request bodies.
- [x] 5.2 Add service tests for multi-payload requests where eligible and ineligible payloads are mixed.
- [x] 5.3 Add service tests proving one skipped payload does not prevent later eligible payloads from being forwarded.
- [x] 5.4 Ensure existing `MsgCallback`, Telegram inbound webhook, and ticket service tests still pass.

## 6. Documentation

- [x] 6.1 Update relevant API docs or ticket docs with outbound ticket group forwarding behavior.
- [x] 6.2 Document that only `conver_type=2` and `receiver=ticket_*` messages are considered.
- [x] 6.3 Document that messages from `ticket.source_id` are skipped to prevent echo loops.
- [x] 6.4 Document that Telegram outbound target is derived from the ticket customer identifier.

## 7. Verification

- [x] 7.1 Run focused tests for the new WebhookMsgs outbound service and API handler.
- [x] 7.2 Run focused tests for Telegram helper/client behavior.
- [x] 7.3 Run `go test ./...`.
- [x] 7.4 Run `openspec validate forward-ticket-group-messages-to-telegram --strict`.
- [x] 7.5 Manually review route/auth behavior to confirm `/webhooks/message` remains public while regular APIs remain protected.
