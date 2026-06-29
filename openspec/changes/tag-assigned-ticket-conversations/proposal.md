## Why

After a ticket is claimed, IM clients need conversation tags that let support users distinguish assigned ticket groups and quickly find tickets assigned to themselves. Ticket creation already creates an IM group whose group ID is the `ticket_id` and adds the ticket inbox members to that group, but the claim flow does not yet synchronize the corresponding conversation tag state through the IM SDK.

## What Changes

- Extend successful ticket claim handling to call the IM SDK `TagConvers` method for the ticket group conversation whose conversation ID is the `ticket_id` and whose conversation type is group.
- Tag the ticket group conversation as `assigned` for every member configured on the ticket's inbox.
- Add an additional `my_ticket` tag for the claiming user on the same ticket group conversation.
- Do not create the ticket group or add group members during claim; that is already done when the ticket is created.
- Keep the existing ticket claim API contract and response shape unchanged.
- Treat conversation tagging failures as part of the claim-side IM synchronization path, with rollback/error behavior defined in implementation design.

## Capabilities

### New Capabilities
- `ticket-claim-conversation-tagging`: Defines IM conversation tag synchronization performed after a ticket is claimed.

### Modified Capabilities

## Impact

- Affected service layer: ticket claim flow in `services/ticketservice.go`.
- Affected IM SDK integration: `commons/imsdk` usage and `imserver-sdk-go` `TagConvers` request structures.
- Affected tests: ticket claim service tests should cover `assigned` tags for ticket inbox members, `my_ticket` for the assignee, and failure handling.
