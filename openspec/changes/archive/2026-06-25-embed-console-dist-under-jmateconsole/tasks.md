## 1. Backend Console Serving

- [x] 1.1 Implement `console.LoadConsoleWeb` to serve embedded `console/web/dist` files through Gin.
- [x] 1.2 Mount the console static handler under `/jmateconsole` during server startup.
- [x] 1.3 Add SPA fallback for `/jmateconsole` and `/jmateconsole/*` routes to return `index.html`.
- [x] 1.4 Ensure existing `/jmate` API routes and `/botmsgs` callback routes continue to behave independently of console static serving.

## 2. Frontend Base Path And API Origin

- [x] 2.1 Configure the console web build base path as `/jmateconsole/`.
- [x] 2.2 Configure TanStack Router to use `/jmateconsole` as the console router base.
- [x] 2.3 Change the frontend default API base URL to same-origin browser requests while preserving `VITE_API_BASE_URL` override support.
- [x] 2.4 Update development env examples and proxy defaults to match the same-origin production behavior and current backend port.

## 3. Build Artifacts

- [x] 3.1 Build `console/web` so `console/web/dist` reflects the updated base path and API behavior.
- [x] 3.2 Confirm the Go embed directive includes the built dist files required by the console.

## 4. Verification

- [x] 4.1 Run frontend checks or build verification for `console/web`.
- [x] 4.2 Run `go test ./...` and resolve failures.
- [x] 4.3 Manually verify or add automated coverage that `/jmateconsole`, `/jmateconsole/`, and a nested route such as `/jmateconsole/users` return the console entry point.
- [x] 4.4 Verify built asset URLs resolve under `/jmateconsole` and API calls target the current browser origin by default.
