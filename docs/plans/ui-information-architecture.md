# Orion UI/UX Information Architecture

## Goal

Design Orion as an operations-first home server monitoring UI. The app should answer, in order:

1. What needs attention?
2. Which server owns it?
3. What monitor failed?
4. What happened over time?
5. What configuration exists?

Use **Servers** as the user-facing term for agents. Configuration stays read-only in the UI for now.

## Status Language

- Server and monitor: `up`, `down`, `degraded`, `maintenance`, `unknown`, `stale`.
- Incident: `open`, `acknowledged`, `resolved`.
- Alert: `pending`, `sent`, `failed`, `suppressed`, `cooldown`.

## Navigation

- Home.
- Servers.
- Incidents.
- Logs.
- Settings.
- Account menu with session details and sign out.

## Home

Purpose: show current problems first, then a quick server overview.

Show:

- Counts for open incidents, failing monitors, stale servers, failed/suppressed alerts, and expiring TLS certificates.
- A "Needs Attention" list for incidents, failing monitors, and stale servers.
- A server list with status, uptime, last seen, monitor count, incident count, and maintenance state.
- Expandable monitor rows with type, status, latency, last checked, last success, and current failure reason.

Filters:

- Search by server or monitor.
- Status.
- Maintenance.
- Stale only.
- Has incidents.

## Servers

Purpose: compare all monitored servers.

Show:

- Name, status, OS/platform, IP/location when available.
- CPU, memory, disk, uptime, last seen, monitor count, and open incidents.
- Grouping for all, healthy, needs attention, maintenance, and stale.
- Row actions for detail, monitors, incidents, and events.

## Server Detail

Purpose: explain one server's current health and recent history.

Show:

- Name, status, maintenance state, last seen, uptime, and Agent version.
- Health summary with monitor counts, stale state, latest report, and active incidents.
- System metrics: CPU, memory, disk, load, uptime, and location/IP metadata.
- Monitors grouped as failing first, unknown/stale second, healthy last.
- Server events: registration, reconnect, reports, maintenance, stale, recovery, alerts, and incidents.
- Read-only server and monitor configuration snapshot.

## Monitor Detail

Purpose: explain one check's behavior, reliability, and latest failure.

Show:

- Name, parent server, type, status, current incident, last checked, and last success.
- Current result: health, latency, status code, resolved IP, TLS expiry, failure reason, and raw error details.
- Uptime percentage, uptime duration, daily buckets, and recent down/degraded windows.
- Check history with timestamp, status, latency, result summary, error payload, and collected metrics.
- Related incidents and notification outcomes.
- Read-only monitor configuration snapshot.

## Incidents

Purpose: show operational history for things that broke or needed attention.

List:

- Severity, status, title, affected server/monitor, opened time, duration, latest event, and notification status.

Filters:

- Status.
- Severity.
- Server.
- Monitor.
- Date range.

Detail:

- Header with title, status, severity, affected server/monitor, opened time, resolved time, and duration.
- Cause summary from the triggering report through the latest or recovery result.
- Timeline for open, alert match, notifications, status changes, recovery, and resolution.
- Links to related monitor reports, server events, and alert attempts.

## Logs

Purpose: keep Orion's own event trail separate from future service logs.

Orion event log:

- Registration, monitor lifecycle, reports, health changes, incidents, alerts, maintenance, cleanup, and migrations.

Future service logs:

- Source, server, timestamp, level, message, and optional monitor/incident correlation.

## Settings

Purpose: show current configuration and runtime facts without editing them.

Show:

- Core version, database status/path, retention settings, known servers, and monitor types.
- Alert channels with name, type, enabled state, last delivery status, and secret environment variable references.
- Alert rules with trigger, severity, cooldown, recovery notification, maintenance suppression, and target channels.
- Retention values, last cleanup run, and estimated database size.
- Agent setup reference: config shape, known servers, install paths, service files, and Tailscale/local network notes.

## Assumptions

- Home prioritizes current problems before inventory.
- YAML/env configuration stays the source of truth.
- Logs are split between Orion events and future service logs.
- Deferred features should not appear as active UI promises.
