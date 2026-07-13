## Purpose
Ensure ticket claim flows tag the claiming user's ticket group conversation without recreating ticket groups or memberships.

## Requirements

### Requirement: Claiming user receives my_ticket tag
After a ticket is successfully claimed, the system SHALL use the IM SDK `TagConvers` method to apply the `my_ticket` conversation tag to the claimed ticket group conversation for the claiming user.

#### Scenario: Successful claim tags assignee as my ticket
- **WHEN** user `u_1` successfully claims ticket `t_1`
- **THEN** the system tags group conversation `t_1` for user `u_1` using tag `my_ticket`
- **AND** the tag request uses target ID `t_1` and the group conversation type

#### Scenario: Other ticket members are not tagged during claim
- **WHEN** an authenticated support user successfully claims a pending ticket
- **THEN** the system does not apply an `assigned` tag to inbox members or other ticket group members
- **AND** the only claim-related conversation tag request is the claiming user's `my_ticket` tag

### Requirement: Ticket claim does not recreate ticket group membership
The system SHALL NOT create the ticket group or add group members during ticket claim because ticket creation already creates the group with group ID equal to `ticket_id` and adds the ticket inbox members.

#### Scenario: Successful claim only tags conversations
- **WHEN** an authenticated support user successfully claims a pending ticket
- **THEN** the claim flow does not create the ticket group
- **AND** the claim flow does not add members to the ticket group

### Requirement: Tag synchronization failures fail the claim
If the system cannot apply the required `my_ticket` conversation tag after claiming a ticket, the system SHALL return an error for the claim request and SHALL revert the ticket claim when the persisted ticket still belongs to the claiming user.

#### Scenario: My ticket tag synchronization fails
- **WHEN** a pending ticket is claimed but applying the `my_ticket` tag fails for the claiming user
- **THEN** the system returns an error and reverts the ticket from processing back to pending for that assignee
