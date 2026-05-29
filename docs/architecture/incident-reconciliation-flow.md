# Incident Reconciliation Flow

This note explains why Core checks for an active incident on monitor reports, why `record not found` can appear in logs, and how raw report ingestion turns into derived monitor/server state.

## Why Core Looks For An Incident On Every Monitor Report

Every monitor report is a chance to move the incident state machine forward:

- healthy report with no active incident: do nothing;
- healthy report with an active incident: resolve it;
- failing report with no active incident: open one;
- failing report with an active incident: update it;
- healthy report with expiring TLS: treat it as degraded and open/update an incident.

```mermaid
flowchart TD
  A["Server sends monitor report"] --> B["Core stores monitor_report"]
  B --> C["Core updates monitor.health"]
  C --> D{"Report health?"}

  D -- "up" --> E["Use cached active incident id or indexed lookup"]
  E --> F{"Active incident exists?"}
  F -- "yes" --> G["Resolve incident"]
  F -- "no" --> H["Do nothing"]

  D -- "down/degraded/stale" --> I["Use cached active incident id or indexed lookup"]
  I --> J{"Active incident exists?"}
  J -- "yes" --> K["Update incident latest_event"]
  J -- "no" --> L["Open new incident"]

  D -- "up + TLS expiring" --> M["Treat as degraded"]
  M --> I
```

## The Normal No-Incident Case

The noisy log appears when this lookup returns no rows:

```sql
SELECT *
FROM incidents
WHERE monitor_id = ?
  AND status IN ("open", "acknowledged")
ORDER BY opened_at DESC
LIMIT 1;
```

That does not mean ingestion failed. For a healthy monitor, no active incident is the expected state.

```mermaid
flowchart LR
  A["Look for active incident"] --> B["SELECT active incident by monitor_id"]
  B --> C{"Found?"}
  C -- "yes" --> D["Update or resolve it"]
  C -- "no" --> E["Normal: no active incident"]
```

## Incident State Machine

```mermaid
stateDiagram-v2
  [*] --> NoIncident

  NoIncident --> NoIncident: report up
  NoIncident --> Open: report down/degraded/stale
  NoIncident --> Open: report up but TLS expiring

  Open --> Open: report still failing
  Open --> Resolved: report up and TLS ok

  Resolved --> Open: later failing report
  Resolved --> Resolved: report up
```

## Healthy Report With No Incident

```mermaid
sequenceDiagram
  participant Server
  participant Core
  participant Reports as monitor_reports
  participant Monitors as monitors
  participant Incidents as incidents

  Server->>Core: monitor report: up
  Core->>Reports: insert raw report
  Core->>Monitors: set health = up, last_successful_report_at = now
  Core->>Incidents: find active incident for monitor
  Incidents-->>Core: no rows
  Core-->>Server: OK
```

## Failing Report With No Incident

```mermaid
sequenceDiagram
  participant Server
  participant Core
  participant Reports as monitor_reports
  participant Monitors as monitors
  participant Incidents as incidents
  participant Alerts as alert_deliveries

  Server->>Core: monitor report: down
  Core->>Reports: insert raw report
  Core->>Monitors: set health = down
  Core->>Incidents: find active incident for monitor
  Incidents-->>Core: no rows
  Core->>Incidents: create open incident
  Core->>Incidents: create incident_opened event
  Core->>Alerts: queue notification delivery
  Core-->>Server: OK
```

## Continued Failure

```mermaid
sequenceDiagram
  participant Server
  participant Core
  participant Incidents as incidents

  Server->>Core: monitor report: down
  Core->>Incidents: find active incident for monitor
  Incidents-->>Core: existing open incident
  Core->>Incidents: update latest_event and last_event_at
  Core->>Incidents: add monitor_failed event
  Core-->>Server: OK
```

## Recovery

```mermaid
sequenceDiagram
  participant Server
  participant Core
  participant Incidents as incidents
  participant Alerts as alert_deliveries

  Server->>Core: monitor report: up
  Core->>Incidents: find active incident for monitor
  Incidents-->>Core: existing open incident
  Core->>Incidents: set status = resolved
  Core->>Incidents: set resolved_at
  Core->>Incidents: add incident_resolved event
  Core->>Alerts: queue recovery notification if enabled
  Core-->>Server: OK
```

## Derived Monitor Health

Incident reconciliation uses the reported health and TLS checks directly. Core also computes a derived monitor health for broader health views.
Stale checks use the stored reporting interval for the Server or monitor, not a single global timeout.

```mermaid
flowchart TD
  A["Raw monitor report"] --> B["Store reported health"]
  B --> C["Compute derived monitor health"]
  C --> D["Load last 20 reports"]
  D --> E{"No reports?"}
  E -- "yes" --> U["unknown"]
  E -- "no" --> F{"Latest report stale?"}
  F -- "yes" --> U
  F -- "no" --> G{"Flapping?"}
  G -- "yes" --> DEG["degraded"]
  G -- "no" --> H{"Failure rate >= 30% and < 100%?"}
  H -- "yes" --> DEG
  H -- "no" --> I["Use latest reported health"]

  U --> J["Cache computed_health on monitor"]
  DEG --> J
  I --> J
```

## Derived Server Health

Server health rolls monitor state up to the server level.

```mermaid
flowchart TD
  A["Compute server health"] --> B{"Server in maintenance?"}
  B -- "yes" --> M["maintenance"]
  B -- "no" --> C{"Server last_seen stale?"}
  C -- "yes" --> S["stale"]
  C -- "no" --> D{"Has active monitors?"}
  D -- "no" --> UP["up"]
  D -- "yes" --> E["Compute monitor healths"]
  E --> F{"Any down?"}
  F -- "yes" --> DOWN["down"]
  F -- "no" --> G{"Any degraded?"}
  G -- "yes" --> DEG["degraded"]
  G -- "no" --> H{"Any unknown?"}
  H -- "yes" --> UNK["unknown"]
  H -- "no" --> UP
```

## Current Performance Behavior

The active incident lookup is conceptually correct, but it should not be an expensive table scan or a noisy expected miss.

Core now uses targeted behavior:

- `monitors.active_incident_id` caches the active incident id when one exists;
- `monitors.incident_state` stores the last incident-relevant state;
- repeated healthy reports skip the active incident lookup;
- active incidents are updated or resolved by cached incident id when possible;
- fallback lookup uses an index on `incidents(monitor_id, status, opened_at)`;
- expected empty results use `Find()` plus `RowsAffected`, not `First()` returning `record not found`;
- slow active incident lookups and slow reconciliation calls are logged.
