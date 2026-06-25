## Why

The console admin Users page still depends on mock `/api/users` endpoints. Administrators need real backend APIs under `/jmate/console` to list and manage app-scoped users, with handlers in `console/apis`, models in `console/apis/models`, and routes registered in `RouteConsole`.

## What Changes

- Add console user management APIs: list (with pagination/filter/sort), create, update, and delete
- Define console API request/response models in `console/apis/models`
- Implement handlers in `console/apis` and register routes in `RouteConsole` (admin-only via existing `console/apis.Validate`)
- Extend user storage queries needed for console list/filter operations
- Wire the console web Users page to `/jmate/console/users` endpoints instead of mock `/api/users`
- Map frontend user fields (`username`, `roles`, `email`) to backend user fields (`login_account`, `role`, `email`, `nickname`, `avatar`)

## Capabilities

### New Capabilities

- `console-user-management`: Admin-only console APIs and frontend integration for querying and managing users within the current app

### Modified Capabilities

- (none)

## Impact

- `console/apis/`, `console/apis/models/`, `console/services/` (if added)
- `routers/router.go` (`RouteConsole`)
- `storages/models/user.go`, `storages/dbs/userdao.go`
- `console/web/src/api/user.ts`, `console/web/src/routes/_auth/users/*`
- Vite dev proxy may need `/jmate/console` passthrough if not already covered
