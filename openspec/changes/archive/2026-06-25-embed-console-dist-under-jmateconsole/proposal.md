## Why

The console frontend should be shipped by the Go server as part of the same deployable binary, rather than requiring a separate web server. The management UI also needs to live under a dedicated `/jmateconsole` URL prefix while API requests use the browser's current origin.

## What Changes

- Serve `console/web/dist` from the Go server using Go `embed`.
- Mount the management console under `/jmateconsole`.
- Support SPA fallback under `/jmateconsole` so browser refreshes and client routes work.
- Adjust `console/web` build/runtime configuration so static assets and router paths work when deployed under `/jmateconsole`.
- Adjust frontend API base URL behavior so API calls use the current browser address and port instead of a hard-coded backend origin.

## Capabilities

### New Capabilities

- `embedded-console-web`: Defines serving the built console web app from the Go server under `/jmateconsole` with same-origin API access.

### Modified Capabilities

## Impact

- Affected backend: `console/consoleload.go`, `main.go` or router initialization where console static serving is registered.
- Affected frontend: `console/web/vite.config.ts`, router base configuration, API base URL constant or env behavior, and related build output assumptions.
- Affected deployment: the server binary embeds `console/web/dist`, so production deploys must build the console before compiling/running the Go server.
- No database schema changes expected.
