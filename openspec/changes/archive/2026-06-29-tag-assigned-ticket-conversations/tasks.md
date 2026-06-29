## 1. IM SDK Integration Points

- [x] 1.1 Add injectable wrappers in `services/ticketservice.go` for inbox member lookup and `TagConvers`.
- [x] 1.2 Add ticket conversation tag constants for `assigned` and `my_ticket`.
- [x] 1.3 Implement a helper that queries all members configured on the claimed ticket's inbox.

## 2. Claim Flow Tagging

- [x] 2.1 Build the ticket group conversation payload with target ID equal to `ticket_id` and group channel type.
- [x] 2.2 Apply the `assigned` tag to the ticket group conversation for every ticket inbox member.
- [x] 2.3 Apply the `my_ticket` tag to the same ticket group conversation for the claiming user.
- [x] 2.4 On any inbox member query or tag failure, return the mapped IM error and call `RevertClaimIfAssignee` before returning.
- [x] 2.5 Keep the existing assignment notification after successful tag synchronization.
- [x] 2.6 Do not create the ticket group or add group members during claim.

## 3. Tests

- [x] 3.1 Extend the successful claim unit test to assert `assigned` tag requests for all ticket inbox members and `my_ticket` for the assignee.
- [x] 3.2 Add a unit test for inbox member query failure that verifies claim rollback.
- [x] 3.3 Add a unit test for `assigned` tag failure that verifies claim rollback.
- [x] 3.4 Add a unit test for `my_ticket` tag failure that verifies claim rollback.
- [x] 3.5 Add a unit test confirming the visitor/source user is not tagged `assigned` unless they are an inbox member.

## 4. Verification

- [x] 4.1 Run `go test ./services`.
- [x] 4.2 Run the broader relevant Go test suite if service tests pass.
- [x] 4.3 Manually review the claim flow order: claim DB update, query inbox members, tag conversations, send notification, return response.
