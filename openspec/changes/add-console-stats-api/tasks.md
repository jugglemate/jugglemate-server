# Tasks

- [x] Write design doc `docs/console-stats-api.md` covering API shape, impact analysis, and rollout phases.
- [x] Add `apis/models/stats.go` with `OverviewResp`, `AgentsResp`, `CustomersResp` and nested types.
- [x] Add `services/console_stats_service.go` with `ParseTimeRange` / `ParseGranularity` helpers and three service functions.
- [x] Add `apis/console_stats.go` with three Gin handlers and `parseStatsPagination`.
- [x] Wire three `group.GET("/stats/...")` routes inside `RouteConsole` in `routers/router.go`.
- [x] Add `apis/console_stats_test.go` covering parameter parsing and pagination clamping.
- [x] Add `services/console_stats_service_test.go` covering time-range / granularity / ctx-appkey paths.
- [x] `go vet ./...` clean.
- [x] `go test ./apis/ ./services/ -run "ConsoleStats|Parse|GetStats"` all pass.
- [x] Verify no existing file outside the documented touches list is modified (review `git status`).
