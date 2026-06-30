## Purpose
Define how Telegram Bot API webhook updates enter the customer and ticket workflow for Telegram inboxes.

## Requirements

### Requirement: Telegram webhook endpoint accepts bot updates
The system SHALL expose a public Telegram webhook endpoint at `POST /jmate/webhooks/telegram/:inbox_id` that accepts Telegram update payloads for the configured inbox.

#### Scenario: Telegram posts to inbox webhook
- **WHEN** Telegram sends an update payload to `/jmate/webhooks/telegram/inbox_1`
- **THEN** the system resolves `inbox_1` to a Telegram inbox and processes the update using that inbox's app scope and channel configuration

#### Scenario: Unknown inbox webhook
- **WHEN** Telegram sends an update payload for an unknown `inbox_id`
- **THEN** the system rejects or acknowledges the update without creating customers, relations, tickets, or IM messages

#### Scenario: Non-Telegram inbox webhook
- **WHEN** Telegram sends an update payload to an inbox whose `channel_type` is not `telegram`
- **THEN** the system rejects or acknowledges the update without creating customers, relations, tickets, or IM messages

### Requirement: Telegram private visitors map to customers
The system SHALL use the Telegram user's numeric ID as the visitor identity when processing supported private Telegram message updates.

#### Scenario: New Telegram visitor creates customer
- **WHEN** a private Telegram `message` update contains `from.id` `123456`
- **THEN** the system creates or reuses a customer whose identifier is derived from Telegram user ID `123456`
- **AND** the customer display name is derived from Telegram `first_name`, `last_name`, or `username` when present

#### Scenario: Repeat Telegram visitor reuses customer
- **WHEN** another private Telegram `message` update arrives from the same Telegram `from.id`
- **THEN** the system reuses the existing customer for that Telegram identity in the current app

### Requirement: Telegram visitors enter ticket workflow
The system SHALL create or reuse the customer-inbox relation, IM visitor user, ticket group, and ticket for supported Telegram visitor messages using the same core behavior as the widget customer start flow.

#### Scenario: New Telegram visitor creates ticket
- **WHEN** a supported private Telegram message arrives for a Telegram inbox and no ticket exists for the visitor source ID
- **THEN** the system creates a ticket with `inbox_id` equal to the webhook inbox
- **AND** the system creates an IM group whose group ID is the generated `ticket_id`
- **AND** the IM group members include the Telegram visitor source ID and the inbox members configured for that inbox

#### Scenario: Existing Telegram ticket is reused
- **WHEN** a supported private Telegram message arrives for a visitor source ID that already has a ticket
- **THEN** the system reuses the existing ticket and does not create a duplicate ticket group

#### Scenario: Telegram visitor source ID uses widget format
- **WHEN** a Telegram customer-inbox relation is created for Telegram user ID `123456`
- **THEN** the relation source ID uses the widget random customer source ID format
- **AND** the source ID starts with `customer_`
- **AND** the source ID is not derived from Telegram user ID `123456`

### Requirement: Telegram text messages are forwarded to ticket IM group
The system SHALL forward supported incoming Telegram text or caption content into the ticket group conversation after the visitor and ticket are ready.

#### Scenario: Incoming text enters ticket group
- **WHEN** a supported private Telegram text message arrives for an inbox
- **THEN** the system sends a group IM message with sender equal to the Telegram visitor source ID
- **AND** the group target is the ticket ID
- **AND** the message content contains the Telegram text

#### Scenario: Incoming caption enters ticket group
- **WHEN** a supported private Telegram message contains a caption but no text
- **THEN** the system sends a group IM message containing the caption text

#### Scenario: Unsupported Telegram message type
- **WHEN** a Telegram update contains no supported text or caption content
- **THEN** the system acknowledges the update without sending an IM message

### Requirement: Telegram webhook processing is idempotent for visitor and ticket creation
The system SHALL avoid creating duplicate customers, customer-inbox relations, tickets, or ticket groups when Telegram retries the same visitor message.

#### Scenario: Repeated update for same visitor
- **WHEN** Telegram delivers the same supported update more than once
- **THEN** the system reuses the existing customer and ticket for the Telegram visitor identity
- **AND** the system does not create duplicate customer-inbox relation rows for the same customer and inbox

### Requirement: Telegram webhook ignores unsupported chat scopes
The system SHALL ignore Telegram group, supergroup, and channel chat updates for this integration.

#### Scenario: Telegram group chat message
- **WHEN** a Telegram update contains `message.chat.type` other than `private`
- **THEN** the system acknowledges the update without creating customers, tickets, or IM messages

### Requirement: Telegram ticket group replies are sent back to Telegram
The system SHALL send eligible ticket group messages back to Telegram when the loaded ticket inbox is a Telegram inbox.

#### Scenario: Agent ticket group message is sent to Telegram
- **WHEN** `WebhookMsgs` receives an eligible ticket group message
- **AND** the ticket inbox `channel_type` is `telegram`
- **AND** the message sender is not the ticket `source_id`
- **THEN** the system sends the message content to Telegram using the inbox Telegram bot configuration

#### Scenario: Telegram target is derived from ticket customer identity
- **WHEN** the system forwards a ticket group message for a Telegram inbox
- **THEN** the system derives the Telegram target from the ticket's customer identifier
- **AND** the system does not use the ticket `source_id` as the Telegram target

#### Scenario: Telegram bot token is missing
- **WHEN** the ticket inbox is Telegram but its channel configuration has no bot token
- **THEN** the system acknowledges the payload without sending to Telegram

#### Scenario: Telegram target cannot be resolved
- **WHEN** the ticket customer cannot be found or has no usable Telegram identifier
- **THEN** the system acknowledges the payload without sending to Telegram

### Requirement: Visitor Telegram messages are not echoed
The system SHALL NOT send a ticket group message back to Telegram when the group message sender is the Telegram visitor source user.

#### Scenario: Visitor source sender is skipped
- **WHEN** `WebhookMsgs` receives an eligible ticket group message for a Telegram inbox
- **AND** the payload sender equals the ticket `source_id`
- **THEN** the system acknowledges the payload without sending to Telegram

### Requirement: Telegram outbound message content is text-compatible
The system SHALL forward supported text-compatible IM message content to Telegram.

#### Scenario: Text-compatible content is forwarded
- **WHEN** an eligible Telegram ticket group message contains supported message content
- **THEN** the system sends a Telegram message containing that content

#### Scenario: Empty or unsupported content is ignored
- **WHEN** an eligible Telegram ticket group message has empty or unsupported content
- **THEN** the system acknowledges the payload without sending to Telegram
