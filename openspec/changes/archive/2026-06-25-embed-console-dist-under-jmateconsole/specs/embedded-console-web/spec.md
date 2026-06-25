## ADDED Requirements

### Requirement: Console web is served from embedded dist assets
The system SHALL serve the built console web application from the Go binary using embedded `console/web/dist` assets.

#### Scenario: Serve embedded console asset
- **WHEN** a browser requests an existing console asset
- **THEN** the system serves the asset from the embedded `console/web/dist` filesystem

#### Scenario: Backend build includes console dist
- **WHEN** the backend binary is built for deployment
- **THEN** the built console distribution is included through Go embed

### Requirement: Console web is mounted under jmateconsole
The system SHALL expose the management console under the `/jmateconsole` URL prefix and SHALL NOT require the console to be hosted at the root path.

#### Scenario: Open console base path
- **WHEN** a browser requests `/jmateconsole`
- **THEN** the system returns the console SPA entry point

#### Scenario: Open console slash base path
- **WHEN** a browser requests `/jmateconsole/`
- **THEN** the system returns the console SPA entry point

#### Scenario: Console does not occupy root path
- **WHEN** a browser requests `/`
- **THEN** the system does not depend on the console being mounted at root

### Requirement: Console supports SPA routes under jmateconsole
The system SHALL support direct navigation and refreshes for client-side console routes under `/jmateconsole`.

#### Scenario: Refresh console client route
- **WHEN** a browser refreshes `/jmateconsole/users`
- **THEN** the system returns the console SPA entry point

#### Scenario: Serve existing static file before fallback
- **WHEN** a browser requests an existing static file under `/jmateconsole`
- **THEN** the system returns that static file instead of the SPA fallback

### Requirement: Console frontend uses jmateconsole as build and router base
The console frontend SHALL generate asset URLs and route navigation that work when deployed under `/jmateconsole`.

#### Scenario: Built asset URLs include console base
- **WHEN** the console frontend is built
- **THEN** generated static asset URLs resolve under `/jmateconsole`

#### Scenario: Router navigation keeps console base
- **WHEN** a console user navigates between console routes
- **THEN** navigation remains under the `/jmateconsole` prefix

### Requirement: Console API calls default to current browser origin
The console frontend SHALL default API requests to the browser's current scheme, host, and port, while still allowing explicit environment override.

#### Scenario: Same-origin API request
- **WHEN** the console is opened from `http://localhost:8050/jmateconsole`
- **THEN** default API calls target `http://localhost:8050`

#### Scenario: Environment override remains supported
- **WHEN** `VITE_API_BASE_URL` is configured
- **THEN** API calls use the configured base URL instead of the current browser origin
