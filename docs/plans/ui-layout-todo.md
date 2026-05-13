# Orion UI Layout To-Do

This turns the information architecture into a build checklist for the Console UI. Work top to bottom unless a backend contract is missing. Use generated SDK hooks only; do not hand-write OpenAPI or SDK types.

## Rules

- Use **Servers** in UI copy, even when the API uses agents.
- Keep the UI operational and text-first before adding visual polish.
- Build from real Core operations. If an operation is missing, add the Core route annotation, generate OpenAPI, generate the SDK, then build the UI.
- Do not edit generated OpenAPI or SDK files manually.
- Prefer existing shadcn components from `apps/console/src/components/ui`.
- Keep filters and repeated layout blocks in their own small components.

## Current Baseline

- [x] Home shell exists.
- [x] Home uses tabs with Incidents selected by default.
- [x] Home incidents tab reads persisted incidents with `getIncidents`.
- [x] Home servers tab reads servers with `getAgents`.
- [x] Home server filters exist: search, status, maintenance, stale only, has incidents.
- [x] Home filter controls live in `agent-filters.tsx`.
- [x] Home owns server and incident list pagination.
- [x] shadcn config resolves `@/*` to `src/*`.


## Phase 1: App Shell

Goal: add the navigation structure from the IA without filling every page yet.

- [x] Add app header.
  - Orion name/logo text.
  - Global health indicator.
  - User/profile action.
  - Operations: `getHealthSummary`, session state.
- [x] Add primary navigation.
  - Home.
  - Logs.
  - Settings.
- [x] Add placeholder routes for pages/detail views not built yet.
  - Keep placeholders plain and useful.
  - Include the operation gaps needed for each page.
- [x] Ensure active navigation state is obvious.


## Phase 2: Home Layout

Goal: make Home a useful operations summary without leaving the first page.

- [x] Add a compact page header.
  - Title: `Home`.
  - Subtitle: current operational summary in one line.
  - Operation: `getHealthSummary`.
- [x] Add global attention summary row.
  - Open incidents.
  - Down monitors.
  - Degraded monitors.
  - Stale servers.
  - Expiring TLS certificates if available.
  - Operations: `getHealthSummary`, `getIncidents`.
- [x] Tighten the Incidents tab layout.
  - Keep rows text-first.
  - Show severity, title, server, monitor, status, opened time, latest event.
  - Operation: `getIncidents`.
  - Pagination stays here, not on a separate Incidents page.
- [x] Tighten the Servers tab layout.
  - Keep server rows scannable.
  - Show name, status, platform, monitor count, last seen, maintenance.
  - Operation: `getAgents`.
  - Pagination stays here, not on a separate Servers page.
- [x] Expand server rows into monitor rows.
  - Show monitor name, type, health, last success, latest error when present.
  - Operation: `getAgentMonitors`.
- [x] Make empty/loading/error states consistent across both tabs.


## Phase 3: Server Detail Page

Goal: explain one server's current health and recent behavior.

- [x] Add server detail route.
  - Route should use server/agent id.
  - Operation: `getAgent`.
- [x] Add server detail header.
  - Name.
  - Status.
  - Maintenance state.
  - Last seen.
  - Uptime.
  - Agent version.
  - Operations: `getAgent`, `getAgentHealth`.
- [x] Add health summary block.
  - Overall health.
  - Monitor health counts.
  - Stale status.
  - Active incidents affecting this server.
  - Operations: `getAgentHealth`, `getIncidents`.
- [x] Add latest system metrics block.
  - CPU.
  - Memory.
  - Disk.
  - Load.
  - IP/location.
  - Operation: `getAgent`.
- [x] Add monitor list grouped by severity.
  - Down/degraded first.
  - Unknown/stale second.
  - Up last.
  - Operation: `getAgentMonitors`.
- [x] Add configuration snapshot.
  - Agent config summary.
  - Monitor config summary.
  - Explain `config.yaml` vs Agent `state.db`.
  - Operations: `getAgent`, `getAgentMonitors`.
- [x] Defer server events until an event log API exists.

## Phase 4: Monitor Detail Page

Goal: explain a single check and why it is failing or healthy.

- [ ] Add monitor detail route.
  - Route should use monitor id.
  - Operation: `getMonitor`.
- [ ] Add monitor detail header.
  - Name.
  - Parent server.
  - Type.
  - Status.
  - Current incident if any.
  - Last checked.
  - Last success.
  - Operations: `getMonitor`, `getIncidents`.
- [ ] Add current result block.
  - Health.
  - Latency.
  - Status code.
  - Resolved IP.
  - TLS expiry.
  - Failure reason.
  - Raw error details.
  - Operation: `getMonitor`.
- [ ] Add check history list.
  - Timestamp.
  - Status.
  - Latency.
  - Result summary.
  - Error payload.
  - Operation: `getMonitorHistory`.
- [ ] Add configuration snapshot.
  - Type.
  - Interval.
  - Timeout.
  - Expected status/body/regex.
  - Thresholds.
  - Alert enabled state.
- [ ] Defer uptime graph until `GET /v1/monitors/{id}/uptime` has a generated SDK operation.

## Phase 5: Incident Detail Page

Goal: explain one operational event without duplicating the Home incident list.

- [ ] Add incident detail route.
  - Route should use incident id.
  - Operation: future incident detail operation.
- [ ] Add incident header.
  - Title.
  - Status.
  - Severity.
  - Server.
  - Monitor.
  - Opened time.
  - Resolved time.
  - Duration.
- [ ] Add cause summary.
  - Triggering monitor/report.
  - First failing result.
  - Latest result.
  - Recovery result when resolved.
- [ ] Add timeline.
  - Incident opened.
  - Alert rule matched.
  - Notifications sent/failed/suppressed.
  - Status changes.
  - Monitor recovered.
  - Incident resolved.
- [ ] Add linked data.
  - Related monitor reports.
  - Related server events.
  - Alert delivery attempts.
- [ ] Keep incident list/table on Home.
  - Home Incidents tab owns the incident list and pagination.
  - Latest event.
  - Operation: `getIncidents`.

## Phase 6: Settings Page

Goal: expose Core-owned settings without pretending Agent-local config is editable.

- [ ] Add Settings route and overview.
  - Core version.
  - Database path/status if exposed.
  - Known servers.
  - Configured monitor types.
  - Operations: `getHealth`, `getAgents`.
- [ ] Add Data Lifecycle section.
  - Raw report hot window.
  - Archive enabled.
  - Archive directory.
  - Rollups enabled.
  - Rollup retention.
  - Archive schedule.
  - Last rollup/archive status.
  - Operations: `getDataLifecycleSettings`, `updateDataLifecycleSettings`.
- [ ] Add manual data actions.
  - Run rollup.
  - Run archive.
  - Show result counts/status.
  - Operations: `runDataLifecycleRollup`, `runDataLifecycleArchive`.
- [ ] Add Agent Setup section.
  - Expected config shape.
  - Linux/macOS install paths.
  - Agent `state.db` paths.
  - Tailscale/local network notes.
- [ ] Defer alert channels/rules until Core exposes read APIs.

## Phase 7: Logs Page

Goal: add operational history once the backend can serve it.

- [ ] Add Logs route and placeholder.
- [ ] Add Orion Event Log only after Core exposes events.
  - Server registered/reconnected.
  - Monitor registered/unregistered.
  - Report received.
  - Health changed.
  - Incident opened/resolved.
  - Alert sent/failed/suppressed.
  - Maintenance changed.
  - Lifecycle actions.
- [ ] Add Service Logs only after service log collection exists.

## Backend Contract Gaps

- [ ] Add generated operation for server report history: `GET /v1/agents/{id}/reports`.
- [ ] Add generated operation for server uptime: `GET /v1/agents/{id}/uptime`.
- [ ] Add generated operation for monitor uptime: `GET /v1/monitors/{id}/uptime`.
- [ ] Add incident detail endpoint and generated operation.
- [ ] Add incident event timeline endpoint and generated operation.
- [ ] Add Orion event log endpoint and generated operation.
- [ ] Add alert channel listing endpoint and generated operation.
- [ ] Add alert rule listing endpoint and generated operation.

## First Execution Order

1. Finish Home summary and row tightening.
2. Add App Shell navigation.
3. Build Server Detail page.
4. Build Monitor Detail page.
5. Build Incident Detail page.
6. Build Settings data lifecycle controls.
7. Add backend gaps before Logs and detailed incident timelines.
