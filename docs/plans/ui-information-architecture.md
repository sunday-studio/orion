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

## Home Page

Purpose: show what needs attention first, then provide a fast server overview.

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

## Servers Page

Purpose: inventory and comparison across all monitored servers.

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

## Server Detail Page

Purpose: explain one server's current health and history.

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
- Read-only; no editing in UI.

## Monitor Detail Page

Purpose: explain one check's behavior, reliability, and latest failure.

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

- Server registered/reconnected.
- Monitor registered/unregistered.
- Report received.
- Health changed.
- Incident opened/resolved.
- Alert sent/failed/suppressed.
- Maintenance changed.
- Retention cleanup ran.
- Migration ran.
- Auth/login events if useful.

### Service Logs

Purpose: future separate area for logs collected from monitored services.

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

Settings are read-only and config-driven for now.

### Settings Overview

- Core version.
- Database path/status.
- Retention settings.
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

### Alert Rules

List configured rules:

- Name.
- Trigger condition.
- Severity.
- Cooldown.
- Recovery notification enabled.
- Maintenance suppression enabled.
- Target channels.

### Retention

- Raw report retention.
- Daily uptime rollup retention.
- Last cleanup run.
- Estimated database size.

### Agent Setup

- Expected agent config shape.
- Current known servers.
- Install paths for Linux/macOS.
- systemd/launchd references.
- Tailscale/local network notes.

## Assumptions

- The UI uses **Servers** instead of Agents as the user-facing term.
- Home page prioritizes current problems before inventory.
- Configuration remains read-only in the UI; YAML/env configuration stays source of truth.
- Logs are split into Orion event logs and future service logs.
- Removed or intentionally deferred features are excluded from the IA.
