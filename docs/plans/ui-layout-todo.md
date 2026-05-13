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

- [x] Incidents is the first navigation item.
- [x] `/` redirects to `/incidents`.
- [x] Incidents page reads persisted incidents with `getIncidents`.
- [x] Servers page reads servers with `getAgents`.
- [x] Server filters exist: search, status, maintenance, stale only, has incidents.
- [x] Server filter controls live in `servers/components/agent-filters.tsx`.
- [x] Servers and Incidents own their own list pagination.
- [x] Detail pages use breadcrumbs to return to list context.
- [x] shadcn config resolves `@/*` to `src/*`.

## Phase 1: App Shell

Goal: add the navigation structure from the IA without filling every page yet.

- [x] Add app header.
  - Orion name/logo text.
  - Global health indicator.
  - User/profile action.
  - Operations: `getHealthSummary`, session state.
- [x] Add primary navigation.
  - Incidents.
  - Servers.
  - Logs.
  - Settings.
- [x] Add placeholder routes for pages/detail views not built yet.
  - Keep placeholders plain and useful.
  - Include the operation gaps needed for each page.
- [x] Ensure active navigation state is obvious.


## Phase 2: Incidents And Servers Lists

Goal: keep operational list views separate, text-first, and easy to scan.

- [x] Add Incidents page.
  - Route: `/incidents`.
  - First navigation item.
  - Operation: `getIncidents`.
- [x] Add Servers page.
  - Route: `/servers`.
  - Operation: `getAgents`.
- [x] Tighten the Incidents list layout.
  - Keep rows text-first.
  - Show severity, title, server, monitor, status, opened time, latest event.
  - Operation: `getIncidents`.
  - Pagination stays on the Incidents page.
- [x] Tighten the Servers list layout.
  - Keep server rows scannable.
  - Show name, status, platform, monitor count, last seen, maintenance.
  - Operation: `getAgents`.
  - Pagination stays on the Servers page.
- [x] Expand server rows into monitor rows.
  - Show monitor name, type, health, last success, latest error when present.
  - Operation: `getAgentMonitors`.
- [x] Add direct server detail navigation from each server row.
- [x] Make empty/loading/error states consistent across both lists.


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
- [x] Add report log.
  - Timestamp.
  - CPU usage.
  - Memory usage.
  - Disk usage.
  - Uptime.
  - Operation: `getAgentReports`.
- [x] Add configuration snapshot.
  - Agent config summary.
  - Monitor config summary.
  - Explain `config.yaml` vs Agent `state.db`.
  - Operations: `getAgent`, `getAgentMonitors`.
- [x] Defer server events until an event log API exists.

## Phase 4: Monitor Detail Page

Goal: explain a single check and why it is failing or healthy.

- [x] Add monitor detail route.
  - Route should use monitor id.
  - Operation: `getMonitor`.
- [x] Add monitor detail header.
  - Name.
  - Parent server.
  - Type.
  - Status.
  - Current incident if any.
  - Last checked.
  - Last success.
  - Operations: `getMonitor`, `getIncidents`.
- [x] Add current result block.
  - Health.
  - Latency.
  - Status code.
  - Resolved IP.
  - TLS expiry.
  - Failure reason.
  - Raw error details.
  - Operation: `getMonitor`.
- [x] Add check history list.
  - Timestamp.
  - Status.
  - Latency.
  - Result summary.
  - Error payload.
  - Operation: `getMonitorHistory`.
- [x] Add uptime summary.
  - 90 day uptime percentage.
  - Recent daily uptime buckets.
  - Operation: `getMonitorUptime`.
- [x] Add configuration snapshot.
  - Type.
  - Interval.
  - Timeout.
  - Expected status/body/regex.
  - Thresholds.
  - Alert enabled state.
- [x] Keep uptime text-first; defer graphing until it is useful.

## Phase 5: Incident Detail Page

Goal: explain one operational event without duplicating the Incidents list.

- [x] Add incident detail route.
  - Route uses incident id.
  - Operation: `getIncidents` until a dedicated detail operation exists.
- [x] Add incident header.
  - Title.
  - Status.
  - Severity.
  - Server.
  - Monitor.
  - Opened time.
  - Resolved time.
  - Duration.
- [x] Add cause summary.
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
- [x] Keep incident list/table on Incidents.
  - Incidents page owns the incident list and pagination.
  - Latest event.
  - Operation: `getIncidents`.

## Phase 6: Settings Page

Goal: expose Core-owned settings without pretending Agent-local config is editable.

- [x] Add Settings route.
  - Keep the page focused on settings users can understand or change.
  - Do not show Core internals just because the API exposes them.
- [x] Add Data Lifecycle section.
  - Raw report hot window.
  - Archive enabled.
  - Archive directory.
  - Rollups enabled.
  - Rollup retention.
  - Archive schedule.
  - Last rollup/archive status.
  - Operations: `getDataLifecycleSettings`, `updateDataLifecycleSettings`.
- [x] Add manual data actions.
  - Run rollup.
  - Run archive.
  - Show result counts/status.
  - Operations: `runDataLifecycleRollup`, `runDataLifecycleArchive`.
- [x] Keep Agent setup out of Settings for now.
  - Setup references stay in docs until there is a focused setup flow.
  - Do not mix install notes into editable Core settings.
- [x] Defer alert channels/rules until Core exposes read APIs.

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

- [x] Add generated operation for server report history: `GET /v1/agents/{id}/reports`.
- [x] Add generated operation for agent uptime: `GET /v1/agents/{id}/uptime`.
- [x] Add generated operation for monitor uptime: `GET /v1/monitors/{id}/uptime`.
- [ ] Add incident detail endpoint and generated operation.
- [ ] Add incident event timeline endpoint and generated operation.
- [ ] Add Orion event log endpoint and generated operation.
- [ ] Add alert channel listing endpoint and generated operation.
- [ ] Add alert rule listing endpoint and generated operation.

## Backend-to-Console Priority List

1. [x] Show agent uptime on Agent detail.
  - Backend route exists: `GET /v1/agents/{id}/uptime`.
  - Generated SDK operation and rendered on the Agent CPU tab.
2. [x] Show monitor uptime on Monitor detail.
  - Backend route exists: `GET /v1/monitors/{id}/uptime`.
  - Generated SDK operation and rendered on Monitor detail.
3. [x] Ignore API health as a UI priority for now.
  - If Core health is unavailable, Console cannot meaningfully render a remediation view.
4. [ ] Add incident detail contract.
  - Current Console detail can only resolve from `getIncidents`.
  - Add `GET /v1/incidents/{id}` before treating this as complete.
5. [ ] Add incident event timeline contract.
  - Needed for incident opened, alert matched, delivery attempts, recovery, and resolution events.
6. [ ] Add alert delivery listing contract.
  - Backend stores `alert_deliveries`; Console needs a read endpoint for notification logs.
7. [ ] Add alert channel listing contract.
  - Backend reads webhook/email channels from config; Console needs redacted channel metadata only.
8. [ ] Add alert rules/settings listing contract.
  - Console should show cooldown/recovery/suppression behavior once Core exposes it.
9. [ ] Leave agent-to-Core write routes out of Console unless explicit admin tooling is planned.
  - Register/report/monitor write endpoints are runtime protocol endpoints, not normal UI features.

## First Execution Order

1. Build App Shell navigation.
2. Build Incidents and Servers list pages.
3. Build Server Detail page.
4. Build Monitor Detail page.
5. Build Incident Detail page.
6. Build Settings data lifecycle controls.
7. Add backend gaps before Logs and detailed incident timelines.
