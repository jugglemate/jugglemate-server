## ADDED Requirements

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
