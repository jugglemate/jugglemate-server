## Context

The current ticket claim flow in `services/ticketservice.go` validates the requester, atomically moves a pending ticket to processing with the requester as assignee, obtains the IM SDK, adds the requester to the IM group whose group ID is the `ticket_id`, builds the response, and sends a ticket assignment notification. Ticket creation already creates the IM group whose group ID is the `ticket_id` and adds the visitor plus the ticket inbox members to that group, so claim-time membership synchronization is no longer required by this change.

The IM SDK exposes `TagConvers(TagConversReq)`, where each request targets one `UserId`, one tag, and one or more conversations. The ticket conversation is the group conversation with `TargetId` equal to `ticket_id` and group channel type `2`.

## Goals / Non-Goals

**Goals:**
- After a successful ticket claim, tag the ticket group conversation as `assigned` for every member configured on the claimed ticket's inbox.
- Tag the same conversation as `my_ticket` for the claiming user.
- Keep the existing claim endpoint response and ticket persistence model unchanged.
- Preserve the current claim flow's all-or-error behavior for required IM synchronization.

**Non-Goals:**
- Do not add new ticket database columns or persist tag state locally.
- Do not expose a new API for managing conversation tags.
- Do not create ticket groups or add group members during claim.
- Do not remove tags from previously assigned users; reassignment or close-time tag cleanup is outside this change.
- Do not change the notification message payload or message type.

## Decisions

1. Query inbox members from local storage after the ticket is claimed.
   - Rationale: the requirement says the ticket's inbox members receive the `assigned` tag, while the ticket group also contains the visitor/source user who should not receive support-side assignment tags.
   - Alternative considered: query current IM group members. That would include the visitor and any other group members, which is broader than the intended inbox-member scope.

2. Add injectable function variables for inbox member lookup and conversation tagging.
   - Rationale: the claim flow already uses this pattern for unit tests, and extending it keeps tests deterministic without database or network calls.
   - Alternative considered: wrap the IM SDK in a new interface. That is larger than needed for this narrowly scoped service behavior.

3. Call `TagConvers` once per user and tag.
   - Rationale: `TagConversReq` accepts one `UserId` and one `Tag`, so every inbox member needs an `assigned` request and the assignee needs a separate `my_ticket` request.
   - Alternative considered: batch all users in one request. The current SDK request shape does not support that.

4. Treat tag synchronization failures as claim failures and revert the ticket claim.
   - Rationale: clients depend on the tags immediately after claim for routing and filtering. Returning success without required tags would create inconsistent UI state.
   - Alternative considered: best-effort tagging with logged failures. That lowers claim latency risk but violates the requirement that claimed conversations are tagged.

## Risks / Trade-offs

- Tagging many inbox members increases claim latency -> Keep the implementation straightforward first, use one conversation per request, and rely on existing inbox sizes; add batching/concurrency later only if measurements require it.
- Inbox membership could change while the claim is in progress -> Query local inbox members immediately after the ticket claim succeeds and tag that snapshot.
- Reverting the ticket claim cannot undo tags already applied to some users -> Return the IM error and leave any cleanup to a future compensating workflow if this becomes observable.
- SDK constants for group channel type are defined as `ChannelType_Group` in a different SDK file -> Use the SDK constant converted to `int` instead of a local magic number where possible.
