# Tasks

- [x] Write design doc `docs/seats-list-api.md` with full field-by-field spec in `docs/ticket-api.md` style.
- [x] Add `apis/models/seats.go` with `SeatsListResp` and `SeatItem`.
- [x] Add `services/seats_service.go` with `ParseRoleString`, `ListSeats`, and inline SQL.
- [x] Add `apis/seats.go` with `SeatsList` handler and `parseSeatsPagination`.
- [x] Wire `group.GET("/seats/list", apis.SeatsList)` in `routers/router.go` before `RouteConsole(...)`.
- [x] Add `apis/seats_test.go` covering auth / role / pagination / clamp paths.
- [x] Add `services/seats_service_test.go` covering role parser and ctx-appkey paths.
- [x] `go vet ./...` clean.
- [x] `go test ./apis/ ./services/ -run "Seat|ParseRole"` all pass.
- [x] Smoke test the running server: admin token, customer-service token, missing token, role filter, keyword, inbox filter, status filter.
