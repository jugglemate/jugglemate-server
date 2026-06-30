## ADDED Requirements

### Requirement: Ticket group message callbacks are eligible for outbound channel forwarding
The system SHALL evaluate IM message subscription payloads received by `WebhookMsgs` for outbound channel forwarding when they represent ticket group messages.

#### Scenario: Ticket group message is eligible
- **WHEN** `WebhookMsgs` receives a message payload with `conver_type` equal to `2`
- **AND** the payload `receiver` starts with `ticket_`
- **THEN** the system treats `receiver` as the ticket ID for outbound channel forwarding

#### Scenario: Private message is ignored
- **WHEN** `WebhookMsgs` receives a message payload with `conver_type` not equal to `2`
- **THEN** the system acknowledges the payload without looking up a ticket or sending to an external channel

#### Scenario: Non-ticket group is ignored
- **WHEN** `WebhookMsgs` receives a group message payload whose `receiver` does not start with `ticket_`
- **THEN** the system acknowledges the payload without looking up a ticket or sending to an external channel

### Requirement: Ticket and inbox determine outbound channel
The system SHALL use the eligible group message's ticket ID to load the ticket and then load the ticket's inbox to determine the outbound channel behavior.

#### Scenario: Ticket inbox is loaded
- **WHEN** an eligible ticket group message has receiver `ticket_123`
- **THEN** the system loads ticket `ticket_123`
- **AND** the system loads the inbox referenced by the ticket's `inbox_id`

#### Scenario: Missing ticket is ignored
- **WHEN** no ticket exists for the eligible group message receiver
- **THEN** the system acknowledges the payload without sending to an external channel

#### Scenario: Missing inbox is ignored
- **WHEN** the ticket exists but the ticket inbox cannot be found
- **THEN** the system acknowledges the payload without sending to an external channel

#### Scenario: Unsupported inbox channel is ignored
- **WHEN** the ticket inbox channel type has no outbound channel implementation
- **THEN** the system acknowledges the payload without sending to an external channel

### Requirement: Visitor-originated ticket group messages do not echo back
The system SHALL skip outbound channel forwarding when the IM message sender is the ticket visitor source user.

#### Scenario: Sender is ticket source user
- **WHEN** an eligible ticket group message sender equals the loaded ticket's `source_id`
- **THEN** the system acknowledges the payload without sending to an external channel

#### Scenario: Sender is not ticket source user
- **WHEN** an eligible ticket group message sender differs from the loaded ticket's `source_id`
- **THEN** the system may forward the message according to the ticket inbox channel type

### Requirement: Multi-message webhook payloads are processed independently
The system SHALL evaluate each message payload in a `WebhookMsgs` request independently.

#### Scenario: Mixed payload batch
- **WHEN** a single webhook request contains both eligible and ineligible message payloads
- **THEN** the system evaluates each payload separately
- **AND** ineligible payloads do not prevent eligible payloads from being forwarded

#### Scenario: Malformed request body
- **WHEN** `WebhookMsgs` receives a body that cannot be parsed as the expected callback envelope
- **THEN** the system acknowledges the request without sending to any external channel
