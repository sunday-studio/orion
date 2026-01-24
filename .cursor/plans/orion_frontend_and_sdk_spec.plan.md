---
name: Orion Frontend and SDK Spec
overview: Spec and tasks for SDK (from OpenAPI), backend API changes, and frontend. OpenAPI covers only frontend routes that are not agent-protected; app is bundled with backend for deployment.
todos:
  - id: o1
    content: Create openapi.yaml with info, servers, paths, components/schemas (no securitySchemes)
    status: pending
  - id: o3
    content: Extend GET /v1/agents in OpenAPI with search, status, last_seen, uptime, sort, order and response fields
    status: pending
  - id: o4
    content: Add GET /v1/agents/:id/reports to OpenAPI (limit, offset)
    status: pending
  - id: o5
    content: Add GET /v1/monitors/:id/uptime and GET /v1/agents/:id/uptime to OpenAPI
    status: pending
  - id: o6
    content: Document remaining frontend paths and error schemas in OpenAPI
    status: pending
  - id: o7
    content: Add npm/Make script to generate SDK from OpenAPI
    status: pending
  - id: o8
    content: Generate TypeScript SDK into sdk/ or frontend/src/lib/api/
    status: pending
  - id: a1
    content: Add User model (id, username, password_hash, created_at) and migration
    status: pending
  - id: a2
    content: "Add AuthUserService: CreateUser, GetUserByUsername, VerifyPassword (bcrypt)"
    status: pending
  - id: a3
    content: Add POST /v1/auth/login handler (JWT or session)
    status: pending
  - id: a4
    content: Add FrontendAuthMiddleware and apply to frontend routes
    status: pending
  - id: a5
    content: "Optional: POST /v1/auth/logout, GET /v1/auth/me"
    status: pending
  - id: a6
    content: Add initial admin user creation (CLI/seed) and document
    status: pending
  - id: b1
    content: Implement ListAgents with search, status, last_seen, uptime, sort, order
    status: pending
  - id: b2
    content: Include monitor_count per agent in list response
    status: pending
  - id: b3
    content: Include ip and uptime_seconds from latest AgentReport in list
    status: pending
  - id: b4
    content: Implement status filter (on-the-fly or cached computed_health)
    status: pending
  - id: b5
    content: Wire new query params in listAgents handler
    status: pending
  - id: b6
    content: Add getAgentReports handler and GET /v1/agents/:id/reports route
    status: pending
  - id: b7
    content: Register agents/:id/reports in routes.go
    status: pending
  - id: b8
    content: Add GetMonitorUptime(monitorID, period) and daily_buckets
    status: pending
  - id: b9
    content: Add GetAgentUptime(agentID, period) aggregating monitors
    status: pending
  - id: b10
    content: Add getMonitorUptime, getAgentUptime handlers and routes
    status: pending
  - id: f0
    content: Choose frontend framework and create frontend/ app shell
    status: pending
  - id: f1
    content: Integrate SDK and set Bearer token from login
    status: pending
  - id: f2
    content: Add global auth guard and redirect to /login
    status: pending
  - id: f3
    content: Create /login route and LoginPage
    status: pending
  - id: f4
    content: Add username, password form and Login button
    status: pending
  - id: f5
    content: "On submit: login API, store token, navigate; on 401 show error"
    status: pending
  - id: f6
    content: Loading and error state on login form
    status: pending
  - id: f7
    content: Create home route and HomePage list view
    status: pending
  - id: f8
    content: Fetch and render agents table (name, ip, description, status, last_seen, uptime, monitor_count)
    status: pending
  - id: f9
    content: Search input with debounced GET /v1/agents?search=
    status: pending
  - id: f10
    content: Filters for status, last_seen, uptime and refetch
    status: pending
  - id: f11
    content: Expandable row with monitors sub-table from GET /v1/agents/:id/monitors
    status: pending
  - id: f12
    content: Link agent row to /agents/:id
    status: pending
  - id: f13
    content: Pagination or load more for agents list
    status: pending
  - id: f14
    content: Create /agents/:id and AgentDetailPage with data fetching
    status: pending
  - id: f15
    content: Metadata code block (agent + latest_report as JSON)
    status: pending
  - id: f16
    content: Reusable UptimeSLA component (daily bars + uptime_percent)
    status: pending
  - id: f17
    content: Monitors list with links to /monitors/:id
    status: pending
  - id: f18
    content: Agent reports table with pagination
    status: pending
  - id: f19
    content: Create /monitors/:id and MonitorDetailPage
    status: pending
  - id: f20
    content: Monitor basic details and link to agent
    status: pending
  - id: f21
    content: Monitor metadata code block
    status: pending
  - id: f22
    content: UptimeSLA on monitor detail
    status: pending
  - id: f23
    content: Monitor reports table with pagination
    status: pending
  - id: f24
    content: View toggle List vs Canvas on home
    status: pending
  - id: f25
    content: Fetch agents and monitors for tree data
    status: pending
  - id: f26
    content: Integrate canvas library and render agent/monitor nodes
    status: pending
  - id: f27
    content: Node click to /agents/:id or /monitors/:id
    status: pending
isProject: false
---

# Orion Frontend and SDK Spec

Spec and tasks for the SDK (from OpenAPI), backend API changes, and frontend. **OpenAPI only covers frontend routes that are not protected by the agent token.** The frontend app is bundled with the backend for deployment.

---

## 1. OpenAPI Spec

**Location:** `core/openapi.yaml` or `docs/openapi.yaml`

### In scope (document these)

- `GET /v1/agents` — add query params: `search`, `status`, `last_seen`, `uptime`, `sort`, `order`
- `GET /v1/agents/:id`
- `GET /v1/agents/:id/health`
- `GET /v1/agents/:id/monitors`
- `GET /v1/agents/:id/reports` (new; `limit`, `offset`)
- `GET /v1/agents/:id/uptime` (new; `period`)
- `GET /v1/monitors/:id`
- `GET /v1/monitors/:id/history`
- `GET /v1/monitors/:id/uptime` (new; `period`)
- `GET /v1/health/summary`
- `GET /v1/health/issues`
- `GET /v1/incidents/candidates`

### Out of scope (do not document)

- `POST /v1/register`
- `POST /v1/agents/:agent_id/register-monitor`
- `POST /v1/agents/:agent_id/unregister-monitor`
- `POST /v1/agents/:agent_id/report`
- `POST /v1/agents/:agent_id/:monitor_id/report`
- `PUT /v1/agents/:agent_id/maintenance`

### Rules

- No `securitySchemes` or `security` on paths; routes are unauthenticated.
- No `POST /v1/auth/login` in the spec unless you add frontend user auth later.
- Response envelope: `{ success, message, data?, error? }` (see [utils/response.go](core/internal/utils/response.go)).
- Schemas from [db/models.go](core/internal/db/models.go) and handlers; include `monitor_count`, `ip`, `uptime_seconds` where added.

### OpenAPI tasks

- O1: Create `openapi.yaml` with `info`, `servers`, `paths` (in-scope only), `components/schemas`. No `securitySchemes` or `security`.
- O3: Extend `GET /v1/agents` in OpenAPI with query params and response fields `monitor_count`, `ip`, `uptime_seconds` (or `latest_report`).
- O4: Add `GET /v1/agents/:id/reports` with `limit`, `offset`; response `{ reports, count, limit, offset }`.
- O5: Add `GET /v1/monitors/:id/uptime` and `GET /v1/agents/:id/uptime` with `period`; response shape for SLA/heatmap.
- O6: Document the rest of the in-scope paths and shared error schemas.
- O7: Add npm or Make script to generate SDK from the OpenAPI file (e.g. openapi-generator, orval, openapi-fetch).
- O8: Generate TypeScript SDK into `sdk/` or `frontend/src/lib/api/`.

---

## 2. Backend: Auth (optional)

Only if you add username/password for the dashboard. OpenAPI does not include login or security.

- A1: Add `User` model: `id`, `username` (unique), `password_hash`, `created_at`; run migration.
- A2: Add `AuthUserService`: `CreateUser`, `GetUserByUsername`, `VerifyPassword` (bcrypt).
- A3: Add `POST /v1/auth/login` handler: bind `username`/`password`, verify, issue JWT or create session; return `{ token }` or set cookie; 401 on failure.
- A4: Add `FrontendAuthMiddleware` (JWT or session) and apply to frontend route group; exclude `POST /v1/auth/login`.
- A5: Optional: `POST /v1/auth/logout`, `GET /v1/auth/me`.
- A6: Add way to create initial admin (CLI, seed script, or first-run API); document.

---

## 3. Backend: API changes

### List agents (`GET /v1/agents`)

- B1: Implement `ListAgents` with `search`, `status`, `last_seen`, `uptime`, `sort`, `order` in the service layer.
- B2: Include `monitor_count` per agent in the response (subquery or join).
- B3: Include `ip` and `uptime_seconds` from the latest `AgentReport` per agent (join or subquery); null when no reports.
- B4: Implement `status` filter: either call `ComputeAgentHealth` in batch or use a cached `computed_health` on `Agent`; document choice.
- B5: Wire the new query params in the `listAgents` handler.

### Agent reports

- B6: Add `getAgentReports` handler: `GET /v1/agents/:id/reports` with `limit`, `offset`; use `ReportService.GetAgentReportsById` and `GetAgentReportCountById`; return `{ reports, count, limit, offset }`.
- B7: Register the route in [routes.go](core/internal/api/routes.go) under the frontend group.

### Uptime SLA

- B8: Add `GetMonitorUptime(monitorID, period)`: from `MonitorReport` group by day; compute up/total per day and overall `uptime_percent`; return `daily_buckets` and `uptime_percent`.
- B9: Add `GetAgentUptime(agentID, period)`: aggregate from the agent’s monitors (e.g. average or by check count).
- B10: Add handlers `getMonitorUptime`, `getAgentUptime`; query param `period` (e.g. `90d`, `30d`); register `GET /v1/monitors/:id/uptime` and `GET /v1/agents/:id/uptime`.

---

## 4. Frontend

### App shell and SDK

- F0: Choose framework (e.g. React+TS, Next.js) and create `frontend/` with app shell, routing, env for API base URL.
- F1: Integrate generated SDK; set `Authorization: Bearer <token>` from login when you add auth.
- F2: Add global auth guard: redirect to `/login` when unauthenticated on protected routes (only if you add frontend auth).

### Login (only if you add auth)

- F3: Create `/login` route and `LoginPage`.
- F4: Form with `username`, `password`, and Login button.
- F5: On submit: call login API; on success store token and go to home; on 401 show error.
- F6: Loading and error state on the form.

### Home list view

- F7: Create home route (e.g. `/` or `/agents`) and `HomePage`.
- F8: Fetch `GET /v1/agents`; render list/table with: `name`, `ip`, `description` (from `meta`), `status`, `last_seen`, `uptime`, `monitor_count`. Format `last_seen` (e.g. “2h ago”) and `uptime` (e.g. “3d 4h”).
- F9: Search input; debounce and call `GET /v1/agents?search=...`.
- F10: Filters for `status`, `last_seen`, `uptime`; map to query params and refetch.
- F11: Expandable row: on expand, fetch `GET /v1/agents/:id/monitors` and show monitors sub-table (`name`, `status`, `last_seen`, `uptime`).
- F12: Link each agent row to `/agents/:id`.
- F13: Pagination or “load more” via `limit`/`offset` and `count`.

### Agent detail (`/agents/:id`)

- F14: Create route and `AgentDetailPage`; fetch `GET /v1/agents/:id`, `.../uptime?period=90d`, `.../monitors`, `.../reports`.
- F15: Metadata: render `agent` (and optionally `latest_report`) as JSON in `<pre><code>`.
- F16: Reusable `UptimeSLA` component: daily bars (e.g. green/red/gray) and `uptime_percent`; style like [OpenStatus](https://themes.openstatus.dev/).
- F17: Monitors list with `name`, `status`, `last_seen`, `uptime`; each links to `/monitors/:id`.
- F18: Agent reports table: columns e.g. `id`, `timestamp`, `uptime_seconds`, `cpu.usage_percent`, `memory.used_percent`, `disk.used_percent`; pagination.

### Monitor detail (`/monitors/:id`)

- F19: Create route and `MonitorDetailPage`; fetch `GET /v1/monitors/:id`, `.../uptime?period=90d`, `.../history`.
- F20: Basic details: `name`, `type`, `description`, `health`, and link to parent agent.
- F21: Metadata: `monitor` (and optionally `recent_reports`) as JSON in `<pre><code>`.
- F22: Reuse `UptimeSLA` with monitor uptime data.
- F23: Monitor reports table: `id`, `collected_at`, `health`, summarized `payload`; pagination.

### Home canvas / tree view

- F24: View toggle on home: “List” vs “Canvas” (or “Tree”).
- F25: For Canvas: fetch `GET /v1/agents` and for each `GET /v1/agents/:id/monitors`; build tree: root → agents → monitors.
- F26: Use a canvas/tree library (e.g. React Flow, D3); render agent and monitor nodes with the same fields as the list.
- F27: On node click: go to `/agents/:id` or `/monitors/:id`.

---

## 5. Data notes

- **`ip`:** From `AgentReport.Location.IP` (latest report); null if none.
- **`description`:** From `Agent.Meta` or `meta.description`; for monitors from `Monitor.Meta`.
- **`status`:** For agents use derived health (`up`/`down`/`degraded`/`unknown`); for monitors use `health` or `computed_health`.
- **`uptime` in list:** From latest `AgentReport.uptime_seconds` (agent) or from monitor’s SLA (monitor). In the SLA component it is the percent over the chosen period.
