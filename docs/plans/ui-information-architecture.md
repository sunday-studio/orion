# Orion UI/UX Information Architecture

## Summary

Design Orion as an operations-first home server monitoring UI. The app should help answer, in order:

- What needs attention?
- Which server owns it?
- What monitor failed?
- What happened over time?
- What configuration exists?

Use **Servers** as the primary user-facing term. The UI is mostly read-only and config-driven: it displays current configuration and can explain or reference setup, but monitor and alert configuration remain outside the UI for now.

## Global Layout

### Header

- Orion logo/name.
- Global health indicator: `All good`, `Issues`, `Maintenance`, or `Unknown`.
- Primary navigation:
  - Home.
  - Servers.
  - Incidents.
  - Logs.
  - Settings.
- User/profile icon:
  - Account/session.
  - Sign out.

### Global Status Language

- Server status: `up`, `down`, `degraded`, `maintenance`, `unknown`, `stale`.
- Monitor status: `up`, `down`, `degraded`, `unknown`, `stale`.
- Incident status: `open`, `acknowledged`, `resolved`.
- Alert status: `pending`, `sent`, `failed`, `suppressed`, `cooldown`.

### API Operation Map

Use generated SDK functions from `apps/console/src/orion-sdk/index.ts`. These names come from the generated OpenAPI `operationId` values.

Available generated operations:

- `login`: frontend session login.
- `getHealth`: unversioned Core health check.
- `getHealthSummary`: global health summary.
- `getHealthIssues`: current down/degraded/stale monitor issues.
- `getIncidentsCandidates`: incident candidates derived from current monitor state.
- `getAgents`: server inventory list.
- `getAgent`: server detail and latest report.
- `getAgentHealth`: server aggregate health.
- `getAgentMonitors`: monitors for a server.
- `getMonitor`: monitor detail with recent reports and computed health.
- `getMonitorHistory`: paginated monitor report history.
- `getDataLifecycleSettings`: data lifecycle settings.
- `updateDataLifecycleSettings`: update data lifecycle settings.
- `runDataLifecycleRollup`: manually run a daily monitor uptime rollup.
- `runDataLifecycleArchive`: manually archive old raw reports.

Agent-to-Core generated operations exist for completeness but are not Console UI flows:

- `registerAgent`
- `registerMonitor`
- `unregisterMonitor`
- `receiveAgentReport`
- `receiveMonitorReport`
- `setMaintenanceMode`

Current generated-contract gaps to resolve before building the matching UI sections:

- Server report history route exists in Core, but no generated `operationId` is currently emitted for `GET /v1/agents/{id}/reports`.
- Server uptime route exists in Core, but no generated `operationId` is currently emitted for `GET /v1/agents/{id}/uptime`.
- Monitor uptime route exists in Core, but no generated `operationId` is currently emitted for `GET /v1/monitors/{id}/uptime`.
- Incident list/detail routes are not implemented yet; current UI can only use `getIncidentsCandidates`.
- Alert channel/rule listing routes are not implemented yet.
- Orion event log/service log routes are not implemented yet.
- Agent local `state.db` is not exposed through Core. The UI should only explain expected setup paths and runtime implications, not read Agent-local SQLite directly.

## Home Page

Purpose: show what needs attention first, then provide a fast server overview.

### Operations

- `getHealthSummary`: global health indicator and summary counts.
- `getHealthIssues`: Needs Attention rows for current monitor/server issues.
- `getIncidentsCandidates`: incident-like rows until a full incident list API exists.
- `getAgents`: server list, search, and inventory overview.
- `getAgentHealth`: per-server health if the list response does not include enough derived status.
- `getAgentMonitors`: expandable monitor rows for a server.

### Attention Summary

- Open incidents count.
- Down monitors count.
- Degraded monitors count.
- Stale servers count.
- Alerts failed/suppressed count.
- Expiring TLS certificates count.

### Needs Attention

Incident rows:

- Severity.
- Affected server.
- Affected monitor.
- Status.
- Opened duration.
- Latest event.

Down/degraded monitor rows if no incident model exists yet:

- Monitor name.
- Server name.
- Status.
- Last failure time.
- Failure reason.

Stale server rows:

- Server name.
- Last seen timestamp.
- Last seen duration.
- Expected reporting interval.

### Server List

Server row:

- Name.
- Status.
- Uptime percentage.
- Uptime duration.
- Last seen timestamp.
- Last seen duration.
- Monitor count.
- Active incident count.
- Maintenance indicator.

Expandable monitors:

- Monitor name.
- Type.
- Status.
- Latency where applicable.
- Last checked.
- Last success.
- Current failure reason.

### Filters

- Search by server or monitor name.
- Status filter.
- Maintenance filter.
- Stale only.
- Has incidents.

Filter implementation notes:

- Server search and broad server sorting should use `getAgents`.
- Monitor name filtering across all servers is not a dedicated API yet; start by filtering expanded `getAgentMonitors` results client-side.
- Has incidents should use `getIncidentsCandidates` until incident list APIs exist.

## Servers Page

Purpose: inventory and comparison across all monitored servers.

### Operations

- `getAgents`: primary table data, search, status, last-seen, uptime, sort, and pagination.
- `getHealthSummary`: optional summary counts for page header.
- `getAgentHealth`: row-level health detail when needed.
- `getAgentMonitors`: monitor count/detail expansion.

### Server Table

- Name.
- Status.
- OS/platform.
- IP/location if available.
- CPU.
- Memory.
- Disk.
- Uptime.
- Last seen.
- Monitor count.
- Open incidents.

### Server Grouping

- All.
- Healthy.
- Needs attention.
- Maintenance.
- Stale.

### Server Row Actions

- Open server detail.
- View monitors.
- View related incidents.
- View server events.

Action operation mapping:

- Open server detail: `getAgent`.
- View monitors: `getAgentMonitors`.
- View related incidents: use `getIncidentsCandidates` for now; replace with future incident list route.
- View server events: not implemented yet.

## Server Detail Page

Purpose: explain one server's current health and history.

### Operations

- `getAgent`: header, latest report, Agent version, config summary, latest metrics.
- `getAgentHealth`: aggregate health counts and server status.
- `getAgentMonitors`: monitor inventory for the server.
- `getMonitor`: drill-in data when a monitor row needs recent reports/computed health.
- `getIncidentsCandidates`: active incident-like conditions affecting this server.
- Generated gap: server uptime should use a future generated operation for `GET /v1/agents/{id}/uptime`.
- Generated gap: server report history should use a future generated operation for `GET /v1/agents/{id}/reports`.

### Header

- Server name.
- Status.
- Maintenance state.
- Last seen timestamp and duration.
- Uptime duration.
- Agent version when available.

### Health Summary

- Overall server health.
- Monitor health counts.
- Stale status.
- Latest report timestamp.
- Active incidents affecting this server.

### System Metrics

- CPU usage.
- Memory usage.
- Disk usage.
- System load.
- Uptime.
- Latest location/IP metadata if available.

### Monitors

Monitor row:

- Name.
- Type.
- Status.
- Uptime percentage.
- Last checked.
- Last success.
- Latest error.

Grouped by:

- Down/degraded first.
- Unknown/stale second.
- Up last.

### Server Events

- Agent registered/reconnected.
- Report received.
- Maintenance changed.
- Server became stale.
- Server recovered.
- Alert/incident events related to this server.

### Configuration Snapshot

- Display current server/agent config summary.
- Display monitor config summary.
- Show Agent local state as conceptual setup information only:
  - config file is user-facing YAML.
  - `state.db` is Agent-owned SQLite for identity, token, maintenance, and monitor id mapping.
  - default Linux state path: `/var/lib/orion/state.db`.
  - default macOS state path: `/usr/local/var/lib/orion/state.db`.
- Read-only; no editing in UI.

## Monitor Detail Page

Purpose: explain one check's behavior, reliability, and latest failure.

### Operations

- `getMonitor`: monitor header, current computed health, recent reports.
- `getMonitorHistory`: paginated check history.
- `getIncidentsCandidates`: current incident-like condition until monitor incident list exists.
- Generated gap: monitor uptime should use a future generated operation for `GET /v1/monitors/{id}/uptime`.

### Header

- Monitor name.
- Parent server.
- Type.
- Status.
- Current incident if any.
- Last checked.
- Last successful check.

### Current Result

- Health.
- Latency.
- Status code where applicable.
- Resolved IP for external HTTP/website checks.
- TLS expiry where applicable.
- Failure reason.
- Raw error details.

### Uptime

- Uptime percentage.
- Uptime duration.
- Daily uptime buckets.
- Recent down/degraded windows.
- Recent buckets should come from raw hot reports and older buckets from rollups once the monitor uptime route is exposed in the generated SDK.

### Check History

- Timestamp.
- Status.
- Latency.
- Result summary.
- Error payload.
- Collected payload/metrics.

### Related Incidents

- Open/resolved incidents for this monitor.
- Opened time.
- Resolved time.
- Severity.
- Notifications sent.

### Configuration Snapshot

- Monitor type.
- Interval.
- Timeout.
- Expected status/body/regex where applicable.
- Threshold values.
- Alert enabled/disabled state.
- Read-only.

## Incidents Page

Purpose: operational history of things that broke or needed attention.

### Operations

- Current temporary source: `getIncidentsCandidates`.
- Future required operation: list incidents with filters.
- Future required operation: update incident acknowledgement/status if acknowledgement becomes editable.

### Incident List

- Severity.
- Status.
- Title.
- Affected server.
- Affected monitor.
- Opened timestamp.
- Duration.
- Latest event.
- Notification status.

### Filters

- Open.
- Acknowledged.
- Resolved.
- Severity.
- Server.
- Monitor.
- Date range.

## Incident Detail Page

Purpose: timeline for one operational event.

### Operations

- Not implemented yet.
- Future required operation: get incident detail.
- Future required operation: list incident events.
- Future required operation: list alert deliveries for an incident.
- Related report links should use `getMonitorHistory` or direct report detail routes when available.

### Incident Header

- Title.
- Status.
- Severity.
- Affected server.
- Affected monitor.
- Opened timestamp.
- Resolved timestamp.
- Duration.

### Cause Summary

- Triggering monitor/report.
- First failing result.
- Latest result.
- Recovery result if resolved.

### Timeline

- Incident opened.
- Alert rule matched.
- Notifications sent/failed/suppressed.
- Status changes.
- Monitor recovered.
- Incident resolved.

### Linked Data

- Related monitor reports.
- Related server events.
- Alert delivery attempts.

## Logs

### Orion Event Log

Purpose: internal operational trail for Orion itself.

Operations:

- Not implemented yet.
- Future required operation: list Orion events.
- Future required operation: filter Orion events by server, monitor, incident, event type, and date range.

- Server registered/reconnected.
- Monitor registered/unregistered.
- Report received.
- Health changed.
- Incident opened/resolved.
- Alert sent/failed/suppressed.
- Maintenance changed.
- Data lifecycle settings changed.
- Manual rollup ran.
- Manual archive ran.
- Retention cleanup ran.
- Migration ran.
- Auth/login events if useful.

### Service Logs

Purpose: future separate area for logs collected from monitored services.

Operations:

- Not implemented yet.
- Future required operation: list service logs.
- Future required operation: filter service logs by server, source, level, and date range.

- Keep separate from Orion events.
- Do not mix service logs into incident/event history by default.

Initial IA placeholder:

- Service/source.
- Server.
- Timestamp.
- Level.
- Message.
- Correlated monitor/incident if available.

## Settings

Settings are mostly config-driven. Data lifecycle settings are editable because they are persisted in Core's database and affect Core-owned storage behavior.

### Operations

- `getDataLifecycleSettings`: read lifecycle settings and last run metadata.
- `updateDataLifecycleSettings`: save lifecycle settings.
- `runDataLifecycleRollup`: manual rollup action.
- `runDataLifecycleArchive`: manual archive action.
- Generated gap: alert channel/rule listing is not implemented yet.

### Settings Overview

- Core version.
- Database path/status.
- Data lifecycle settings.
- Last rollup/archive status.
- Configured alert channels.
- Configured alert rules.
- Known agents/servers.
- Configured monitor types.

### Alert Channels

List configured channels:

- Name.
- Type: webhook or email.
- Enabled/disabled.
- Last delivery status.
- Last delivery timestamp.
- Show environment variable references for secrets, not secret values.

Operation status:

- Not implemented yet.
- Use static/config documentation until Core exposes alert settings.

### Alert Rules

List configured rules:

- Name.
- Trigger condition.
- Severity.
- Cooldown.
- Recovery notification enabled.
- Maintenance suppression enabled.
- Target channels.

Operation status:

- Not implemented yet.
- Use static/config documentation until Core exposes alert rules.

### Data Lifecycle

- Raw report hot window in days.
- Archive raw reports enabled/disabled.
- Archive directory.
- Rollups enabled/disabled.
- Optional daily uptime rollup retention.
- Archive schedule: `daily` or `manual`.
- Last rollup run timestamp.
- Last archive run timestamp.
- Last archive status and error.
- Estimated database size.

Editable controls:

- Numeric input for `raw_report_hot_days`.
- Toggle for `archive_raw_reports`.
- Text input for `archive_dir`.
- Toggle for `rollups_enabled`.
- Optional numeric input for `rollup_retention_days`.
- Select or segmented control for `archive_schedule`.
- Save action using `updateDataLifecycleSettings`.

Manual actions:

- Run rollup using `runDataLifecycleRollup`.
  - Optional date input in `YYYY-MM-DD` format.
  - Without a date, Core rolls up yesterday.
- Run archive using `runDataLifecycleArchive`.
  - Show archived report counts and archive path from the result.

Validation display:

- `raw_report_hot_days` must be at least 1.
- `archive_dir` is required when raw report archiving is enabled.
- rollups must be enabled when raw report archiving is enabled.
- `rollup_retention_days` is blank/null or at least 1.
- archive schedule must be `daily` or `manual`.

### Agent Setup

- Expected agent config shape.
- Current known servers.
- Install paths for Linux/macOS.
- Agent local state paths:
  - Linux: `/var/lib/orion/state.db`.
  - macOS: `/usr/local/var/lib/orion/state.db`.
  - Dev fallback: `./state.db`.
- Explain that `config.yaml` is user-edited and `state.db` is Agent-owned.
- systemd/launchd references.
- Tailscale/local network notes.

Operation mapping:

- Current known servers: `getAgents`.
- Server detail and current Agent metadata: `getAgent`.
- Agent local state is not directly exposed through Core.

## Assumptions

- The UI uses **Servers** instead of Agents as the user-facing term.
- Home page prioritizes current problems before inventory.
- Monitor and alert configuration remain read-only in the UI; YAML/env configuration stays source of truth for those areas.
- Core data lifecycle settings are editable through the UI because Core stores them in SQLite.
- Agent local `state.db` is Agent-owned runtime state and should not be hand-edited.
- Logs are split into Orion event logs and future service logs.
- Removed or intentionally deferred features are excluded from the IA.
