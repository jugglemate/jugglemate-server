## Context

The backend already contains `console/consoleload.go` with an embedded `console/web/dist` filesystem placeholder, but `LoadConsoleWeb` is empty and is not wired into server startup. The React console is currently configured like a root-hosted SPA, and its API base URL has recently been set to a fixed `http://localhost:8050` default. The requested deployment shape is a single Go server that serves APIs and the console UI from the same browser origin, with the console mounted under `/jmateconsole`.

## Goals / Non-Goals

**Goals:**

- Serve the built console assets from the Go binary using `embed`.
- Mount the console at `/jmateconsole` and keep the existing API namespace at `/jmate`.
- Make direct navigation and refreshes under `/jmateconsole/*` return the SPA entry point.
- Make frontend API requests use the browser's current scheme, host, and port by default.
- Configure the frontend build so asset URLs and router paths work from the `/jmateconsole` base path.

**Non-Goals:**

- No change to backend API routes or payload contracts.
- No new external web server or CDN requirement.
- No database schema changes.
- No redesign of the console UI.

## Decisions

- Use Go `embed.FS` and `fs.Sub` to serve `console/web/dist`.
  - Rationale: this keeps the console bundled with the backend binary and matches the existing `consoleload.go` direction.
  - Alternative considered: serve files from disk at runtime. That makes deployment depend on external files and does not satisfy the embed requirement.

- Mount static console routes at `/jmateconsole`.
  - Rationale: the management UI should not occupy the root path and should be clearly separated from `/jmate` APIs.
  - Alternative considered: mount under `/jmate/console`. That path is already used for console APIs and would mix API validation concerns with static assets.

- Implement SPA fallback only inside the `/jmateconsole` prefix.
  - Rationale: client routes such as `/jmateconsole/users` need to return `index.html`, while API 404 behavior elsewhere should remain unchanged.
  - Alternative considered: global `NoRoute` fallback. That risks hiding missing API routes behind the console UI.

- Configure the frontend with a `/jmateconsole/` base path and router basename.
  - Rationale: Vite-generated asset URLs and TanStack Router navigation must both understand the subdirectory deployment.
  - Alternative considered: rewrite asset paths in Go responses. Build-time base configuration is simpler and less fragile.

- Default `API_BASE_URL` to an empty string and keep `VITE_API_BASE_URL` as an override.
  - Rationale: an empty base makes fetches same-origin, so browser scheme/host/port are used automatically. The environment variable remains useful for local proxy or cross-origin debugging.
  - Alternative considered: compute `window.location.origin` explicitly. Relative same-origin URLs are simpler and work the same for current API paths.

## Risks / Trade-offs

- [Risk] `go build` can fail if `console/web/dist` does not exist at compile time. → Document and add tasks to run the console build before backend build; keep `dist` generated as part of release workflow.
- [Risk] Asset or router paths can break if frontend base path and backend mount path diverge. → Centralize both on `/jmateconsole` and verify with a production build.
- [Risk] Same-origin API calls from `/jmateconsole` require the backend to serve both UI and APIs on the same host. → This is the target deployment model; preserve `VITE_API_BASE_URL` override for exceptions.
- [Risk] SPA fallback could accidentally serve HTML for asset misses. → Prefer serving existing files first, then fallback to `index.html` only for paths under the console prefix.
