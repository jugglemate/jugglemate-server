# console-agent-admin Specification

## Purpose
TBD - created by archiving change port-agent-admin-console. Update Purpose after archive.
## Requirements
### Requirement: Agent admin navigation
The console SHALL present three new left-menu entries — 智能体列表 (agent list), 工具列表 (tools list), and 模型设置 (model settings) — each routed to a dedicated page, visible only to authenticated console users.

#### Scenario: Menus visible when authenticated
- **WHEN** an authenticated console user opens the console
- **THEN** the left menu shows 智能体列表, 工具列表, and 模型设置 entries
- **AND** clicking each navigates to its corresponding route without a full page reload

#### Scenario: Unauthenticated access redirects to login
- **WHEN** an unauthenticated visitor navigates directly to any of the three routes
- **THEN** the console redirects to the login page

### Requirement: Agent list management
The console SHALL let users browse, filter, view detail of, create, update, delete, activate, and pause agents by calling the proxied agent-server endpoints, preserving the admin's behavior and configuration detail views.

#### Scenario: List agents
- **WHEN** the user opens 智能体列表
- **THEN** the console requests the proxied `/agents` endpoint and renders the returned agents with paging/keyword filtering

#### Scenario: View and edit agent configuration
- **WHEN** the user opens an agent's detail
- **THEN** the console shows its configuration (model, tools, skills, knowledge bindings) and allows saving changes via the proxied create/update endpoints

#### Scenario: Lifecycle actions
- **WHEN** the user activates or pauses an agent
- **THEN** the console calls the proxied activate/pause endpoint and reflects the new status

### Requirement: Tools list management
The console SHALL let users manage tool providers, tool definitions, and tool credentials via the proxied agent-server endpoints, including create, delete, state transition, activation, and test operations.

#### Scenario: List and create tool providers
- **WHEN** the user opens 工具列表
- **THEN** the console lists tool providers from the proxied `/tool-providers` endpoint and can create a new provider

#### Scenario: Manage tool definitions
- **WHEN** the user adds a plugin or custom tool definition and tests it
- **THEN** the console calls the proxied `/tool-definitions/*` endpoints and shows the test result

### Requirement: Model settings management
The console SHALL let users manage LLM providers (list, view, create, update, delete, set default) via the proxied `/admin/llm/providers/*` endpoints.

#### Scenario: Set a default model provider
- **WHEN** the user marks a provider as default
- **THEN** the console calls the proxied set-default endpoint and the list reflects the new default

### Requirement: Response envelope compatibility
The console's requests to the proxied agent-server SHALL interpret the agent-server response envelope (`{code,msg,data}`, success when `code === 0`) and convert `snake_case` response keys to `camelCase` for the ported UI.

#### Scenario: Business error surfaces to the user
- **WHEN** a proxied call returns an envelope with a non-zero `code`
- **THEN** the console surfaces the `msg` as an error and does not treat the call as successful

