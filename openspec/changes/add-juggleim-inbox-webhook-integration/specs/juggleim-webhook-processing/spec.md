## ADDED Requirements

### Requirement: JuggleIM webhook endpoint accepts custom chat callbacks
The system SHALL expose a public JuggleIM webhook endpoint at `POST /jmate/webhooks/juggleim/:inbox_id` that accepts JuggleIM custom chat callback payloads for the configured inbox.

#### Scenario: JuggleIM posts to inbox webhook
- **WHEN** JuggleIM sends a `CustomChatMsgReq` payload to `/jmate/webhooks/juggleim/inbox_1`
- **THEN** the system resolves `inbox_1` to a JuggleIM inbox and processes the callback using that inbox's app scope and channel configuration

#### Scenario: Unknown inbox webhook
- **WHEN** JuggleIM sends a callback payload for an unknown `inbox_id`
- **THEN** the system rejects or acknowledges the callback without creating customers, relations, tickets, or IM messages

#### Scenario: Non-JuggleIM inbox webhook
- **WHEN** JuggleIM sends a callback payload to an inbox whose `channel_type` is not `juggleim`
- **THEN** the system rejects or acknowledges the callback without creating customers, relations, tickets, or IM messages

### Requirement: JuggleIM callback payload is parsed as custom chat message
The system SHALL parse JuggleIM webhook request bodies using the `CustomChatMsgReq` fields `app_key`, `sender`, `receiver`, `conver_type`, `msg_type`, `msg_content`, `msg_id`, `msg_time`, and optional `mention_info`.

#### Scenario: Supported callback payload is parsed
- **WHEN** a webhook body contains valid JSON matching `CustomChatMsgReq`
- **THEN** the system extracts `sender`, `msg_type`, `msg_content`, and `msg_id` for downstream ticket processing

#### Scenario: Invalid callback payload is acknowledged without side effects
- **WHEN** a webhook body cannot be parsed as `CustomChatMsgReq`
- **THEN** the system acknowledges the callback without creating customers, tickets, or IM messages

#### Scenario: Missing sender is ignored
- **WHEN** a callback payload has no `sender`
- **THEN** the system acknowledges the callback without creating customers, tickets, or IM messages

### Requirement: JuggleIM senders map to customers
The system SHALL use the JuggleIM callback `sender` as the visitor identity when processing supported custom chat callbacks.

#### Scenario: New JuggleIM sender creates customer
- **WHEN** a supported JuggleIM callback contains `sender` `u_123`
- **THEN** the system creates or reuses a customer whose identifier is derived from JuggleIM sender `u_123`
- **AND** the system creates the customer in the app scope derived from the webhook inbox

#### Scenario: Repeat JuggleIM sender reuses customer
- **WHEN** another supported JuggleIM callback arrives from the same `sender`
- **THEN** the system reuses the existing customer for that sender identity in the current app

### Requirement: JuggleIM visitors enter ticket workflow
The system SHALL create or reuse the customer-inbox relation, IM visitor user, ticket group, and ticket for supported JuggleIM callbacks using the same core behavior as the widget customer start flow.

#### Scenario: New JuggleIM visitor creates ticket
- **WHEN** a supported JuggleIM callback arrives for a JuggleIM inbox and no ticket exists for the visitor source ID
- **THEN** the system creates a ticket with `inbox_id` equal to the webhook inbox
- **AND** the system creates an IM group whose group ID is the generated `ticket_id`
- **AND** the IM group members include the JuggleIM visitor source ID and the inbox members configured for that inbox

#### Scenario: Existing JuggleIM ticket is reused
- **WHEN** a supported JuggleIM callback arrives for a visitor source ID that already has a ticket
- **THEN** the system reuses the existing ticket and does not create a duplicate ticket group

#### Scenario: JuggleIM visitor source ID uses widget format
- **WHEN** a JuggleIM customer-inbox relation is created for sender `u_123`
- **THEN** the relation source ID uses the widget random customer source ID format
- **AND** the source ID starts with `customer_`
- **AND** the source ID is not derived from JuggleIM sender `u_123`

### Requirement: JuggleIM messages are forwarded to ticket IM group
The system SHALL forward supported incoming JuggleIM callback message content into the ticket group conversation after the visitor and ticket are ready.

#### Scenario: Incoming custom chat message enters ticket group
- **WHEN** a supported JuggleIM custom chat callback arrives for an inbox
- **THEN** the system sends a group IM message with sender equal to the JuggleIM visitor source ID
- **AND** the group target is the ticket ID
- **AND** the message type and content are derived from `msg_type` and `msg_content`
- **AND** the callback `msg_id` is used for IM message idempotency when present

#### Scenario: Empty content is ignored
- **WHEN** a JuggleIM callback contains no supported `msg_content`
- **THEN** the system acknowledges the callback without sending an IM message

### Requirement: JuggleIM webhook processing is idempotent for visitor and ticket creation
The system SHALL avoid creating duplicate customers, customer-inbox relations, tickets, or ticket groups when JuggleIM retries a callback for the same sender.

#### Scenario: Repeated callback for same sender
- **WHEN** JuggleIM delivers the same supported callback more than once
- **THEN** the system reuses the existing customer and ticket for the JuggleIM sender identity
- **AND** the system does not create duplicate customer-inbox relation rows for the same customer and inbox
