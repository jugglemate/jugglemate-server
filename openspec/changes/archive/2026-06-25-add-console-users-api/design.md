## Context

The embedded console web app (`console/web`) includes a Users CRUD page that calls mock endpoints at `/api/users`. The Go server already exposes public auth at `/jmate/user/*` and admin gate at `console/apis.Validate` (requires logged-in admin via JWT role). User records live in the `users` table with fields such as `user_id`, `login_account`, `nickname`, `email`, `role`, and `app_key`. Console handlers should live under `console/apis` with models in `console/apis/models`, and routes registered in `RouteConsole`.

## Goals / Non-Goals

**Goals:**

- Provide admin-only console APIs for user list/create/update/delete scoped to the current `appkey`
- Match the existing Users page contract: paginated list with `limit`, `offset`, optional `keyword`, `role`, `sortField`, `sortOrder`
- Register routes under `/jmate/console/users` in `RouteConsole`
- Connect the frontend Users page to real APIs and remove reliance on MSW for user CRUD in normal operation

**Non-Goals:**

- End-user self-service profile APIs outside console
- Permission-point management beyond mapping `role` to frontend `roles` array
- Password reset or bulk import/export
- Cross-app user administration

## Decisions

1. **Route prefix: `/jmate/console/users`**
   - Rationale: Aligns with existing `RouteConsole` group (`/jmate/console`) and separates console admin APIs from public `/jmate/user/*`.
   - Alternative: Reuse `/jmate/user/*` — rejected because console CRUD has different auth and response shape.

2. **Handler layout: `console/apis` + `console/services`**
   - HTTP parsing/response in `console/apis`, business logic in `console/services/userservice.go` (or similar), models in `console/apis/models`.
   - Rationale: Mirrors main `apis` + `services` pattern without mixing console into root `apis`.

3. **List response shape matches frontend paginated envelope**
   - Return `{ code, msg, data: { list, total } }` where each user item maps to frontend `User` schema: `id` ← `user_id`, `username` ← `login_account` (or `nickname`), `avatar`, `email`, `roles` ← mapped from single `role`, `permissions` ← empty or derived later.
   - Rationale: Minimizes frontend schema changes.

4. **Role mapping**
   - Backend `role` int: `1` customer service, `2` admin.
   - Frontend filter `role` query uses string values (`admin`, `editor`/customer-service). Map `admin` → `2`, `editor` or `customer_service` → `1`.
   - Create/update accept `roles[]` but persist primary role as first recognized value (admin wins over editor).
   - Rationale: Keeps DB model unchanged while fitting existing UI.

5. **Storage extensions**
   - Add `IUserStorage` methods: `QryByApp(appkey, filter, limit, offset, sort)`, `Update(appkey, userId, fields)`, `Delete(appkey, userId)` as needed.
   - Keyword search on `login_account`, `nickname`, `email`.
   - Rationale: Current storage only supports lookup by account/user id.

6. **Create user**
   - Accept `username`, optional `email`, `roles`; generate `user_id`, hash password if provided or require password field addition.
   - **Note**: Frontend create form currently has no password field — design adds optional `password` to create API or auto-generate temporary password. Prefer adding `password` to create request (required, min 6) and extend FormModal in frontend integration task.

7. **Authorization**
   - All `/jmate/console/users*` routes use parent `apis.Validate` + `console/apis.Validate` (admin only).

## Risks / Trade-offs

- [Frontend roles do not match backend 1:1] → Document mapping; UI keeps `admin` / `editor` labels mapping to admin/customer-service roles.
- [Create without password in current UI] → Add password field to create modal during frontend wiring.
- [Sort field names differ from DB columns] → Whitelist `id`, `username`, `email`, `roles` and map to SQL columns in service layer.
- [Delete admin self] → Optionally block deleting the current requester; can defer to follow-up.

## Migration Plan

1. Implement storage + service + console APIs and register routes
2. Update frontend `USER_ENDPOINTS` and Vite proxy for `/jmate/console`
3. Verify against running server; MSW remains for offline dev only if explicitly enabled

## Open Questions

- Should delete be soft-delete via `status` or hard delete? **Default: hard delete for v1** unless `status` column is already used for disable.
