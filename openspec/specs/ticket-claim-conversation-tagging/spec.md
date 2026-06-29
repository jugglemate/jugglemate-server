## Purpose
Ensure ticket claim flows synchronize IM conversation tags for ticket group conversations without recreating ticket groups or memberships.

## Requirements

### Requirement: Claimed ticket group conversations are tagged assigned
After a ticket is successfully claimed, the system SHALL use the IM SDK `TagConvers` method to apply the `assigned` conversation tag to the group conversation whose conversation ID is the claimed `ticket_id` for every member configured on the claimed ticket's inbox.

#### Scenario: Successful claim tags all ticket inbox members
- **WHEN** an authenticated support user successfully claims a pending ticket
- **THEN** the system tags the group conversation with target ID equal to the `ticket_id` and group channel type for every member configured on the ticket's inbox using tag `assigned`
- **AND** the system does not tag the ticket visitor/source user with `assigned` unless that user is also configured as an inbox member

#### Scenario: Ticket ID is used as group conversation target
- **WHEN** the system tags the conversation after a ticket claim
- **THEN** each tag request targets the group conversation whose target ID is the claimed ticket's `ticket_id`
- **AND** each tag request uses the group conversation type

### Requirement: Claiming user receives my_ticket tag
After a ticket is successfully claimed, the system SHALL use the IM SDK `TagConvers` method to apply the `my_ticket` conversation tag to the claimed ticket group conversation for the claiming user.

#### Scenario: Successful claim tags assignee as my ticket
- **WHEN** user `u_1` successfully claims ticket `t_1`
- **THEN** the system tags group conversation `t_1` for user `u_1` using tag `my_ticket`
- **AND** the tag request uses target ID `t_1` and the group conversation type

#### Scenario: Assignee keeps assigned tag
- **WHEN** the claiming user is a member of the ticket inbox
- **THEN** the system applies both `assigned` and `my_ticket` tags to that user's ticket group conversation

### Requirement: Ticket claim does not recreate ticket group membership
The system SHALL NOT create the ticket group or add group members during ticket claim because ticket creation already creates the group with group ID equal to `ticket_id` and adds the ticket inbox members.

#### Scenario: Successful claim only tags conversations
- **WHEN** an authenticated support user successfully claims a pending ticket
- **THEN** the claim flow does not create the ticket group
- **AND** the claim flow does not add members to the ticket group

### Requirement: Tag synchronization failures fail the claim
If the system cannot synchronize the required IM conversation tags after claiming a ticket, the system SHALL return an error for the claim request and SHALL revert the ticket claim when the persisted ticket still belongs to the claiming user.

#### Scenario: Inbox member lookup fails
- **WHEN** a pending ticket is claimed but the system cannot load the claimed ticket's inbox members
- **THEN** the system returns an error and reverts the ticket from processing back to pending for that assignee

#### Scenario: Assigned tag synchronization fails
- **WHEN** a pending ticket is claimed but applying the `assigned` tag fails for any ticket inbox member
- **THEN** the system returns an error and reverts the ticket from processing back to pending for that assignee

#### Scenario: My ticket tag synchronization fails
- **WHEN** a pending ticket is claimed but applying the `my_ticket` tag fails for the claiming user
- **THEN** the system returns an error and reverts the ticket from processing back to pending for that assignee
