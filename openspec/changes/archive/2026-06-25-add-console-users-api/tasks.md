## 1. Console API Models

- [x] 1.1 Add console user list/create/update request and response models in `console/apis/models`
- [x] 1.2 Define paginated list response `{ list, total }` and user item mapping fields (`id`, `username`, `avatar`, `email`, `roles`)

## 2. Storage Layer

- [x] 2.1 Extend `IUserStorage` with app-scoped list query supporting `limit`, `offset`, `keyword`, `role`, and sort parameters
- [x] 2.2 Implement list/update/delete (and reuse create) in `storages/dbs/userdao.go`
- [x] 2.3 Add unit tests or service-level tests for list filtering and role mapping if practical

## 3. Console Service Layer

- [x] 3.1 Implement `console/services/userservice.go` for list/create/update/delete with appkey + admin context
- [x] 3.2 Map storage `User` to console API user item (role int ↔ frontend roles array)
- [x] 3.3 Validate create/update input (account format, password length, duplicate account)

## 4. Console HTTP Handlers And Routes

- [x] 4.1 Implement handlers in `console/apis/user.go` for `GET/POST /users` and `PUT/DELETE /users/:user_id`
- [x] 4.2 Register routes in `RouteConsole` under `/jmate/console`
- [x] 4.3 Ensure handlers return standard `{ code, msg, data }` responses and appropriate error codes

## 5. Frontend Integration

- [x] 5.1 Update `console/web/src/api/user.ts` endpoints to `/jmate/console/users`
- [x] 5.2 Add Vite dev proxy for `/jmate/console` if not already covered
- [x] 5.3 Wire Users page CRUD to real APIs; add password field on create form
- [x] 5.4 Map role filter values (`admin`, `editor`) to backend query params
- [x] 5.5 Verify list pagination, search, sort, create, edit, and delete against running server

## 6. Verification

- [x] 6.1 Run `go test ./...` and fix failures
- [x] 6.2 Run `pnpm build` in `console/web` and fix failures
- [x] 6.3 Manually verify admin can manage users and non-admin receives auth errors
