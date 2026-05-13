# Data Ingestion

## High-Level Flow

```mermaid
sequenceDiagram
  participant Agent
  participant Core
  participant AgentService
  participant MonitorService
  participant ReportService
  participant DB as SQLite

  Agent->>Core: POST /v1/register
  Core->>AgentService: RegisterAgent(machine_id, name, os, arch, meta)
  AgentService->>DB: create or reconnect agent
  DB-->>AgentService: agent id + token
  Core-->>Agent: agent_id + token

  Agent->>Core: POST /v1/agents/{agent_id}/register-monitor
  Core->>MonitorService: RegisterMonitor(...)
  MonitorService->>DB: create, revive, or reject duplicate monitor
  Core-->>Agent: monitor_id

  Agent->>Core: POST /v1/agents/{agent_id}/report
  Core->>ReportService: StoreAgentReport(...)
  ReportService->>DB: insert agent_reports
  ReportService->>DB: reconcile stale monitor incidents
  Core-->>Agent: report accepted

  Agent->>Core: POST /v1/agents/{agent_id}/{monitor_id}/report
  Core->>ReportService: StoreMonitorReport(...)
  ReportService->>DB: insert monitor_reports
  ReportService->>DB: update monitor health and last success
  ReportService->>DB: reconcile incidents
  ReportService->>DB: compute and cache health
  Core-->>Agent: report accepted
```

## Agent Registration

The Agent stores its durable identity in `state.yaml`.

On first registration:

- Agent generates or reads a machine identity.
- Agent reads local system name, OS, and architecture.
- Agent sends `machine_id`, `name`, `os`, `arch`, and optional config `meta`.
- Core creates an `agents` row with a generated `agent-*` id and token.
- Agent saves `agent_id`, `token`, `core_url`, and registration state.

On reconnect:

- Core looks up the existing `machine_id`.
- Core returns the same token.
- Core refreshes `last_seen`.
- Core updates changed name, OS, arch, and meta.

```mermaid
flowchart TD
  Register["POST /v1/register"] --> Lookup{"machine_id exists?"}
  Lookup -- "no" --> Create["Create agents row"]
  Create --> Token["Generate bearer token"]
  Token --> ReturnNew["Return agent_id + token"]
  Lookup -- "yes" --> Update["Update metadata + last_seen"]
  Update --> ReturnExisting["Return existing agent_id + token"]
```

## Monitor Registration

After Agent registration, configured monitors are reconciled against internal state:

- Configured monitor exists in state: keep it.
- Configured monitor missing from state: register it with Core.
- State monitor missing from config: unregister it from Core.

Core monitor registration behavior:

- New monitor creates a `monitors` row with `lifecycle = active`, `health = unknown`, and `computed_health = unknown`.
- Previously deleted monitor with the same server/name is revived.
- Active duplicate monitor names for a server are rejected.
- Removed monitors are soft-deleted by setting `lifecycle = deleted`, `health = unknown`, and `deleted_at`.

```mermaid
flowchart TD
  ConfigMonitors["Config monitors"] --> Compare["Compare with state monitors"]
  StateMonitors["State monitors"] --> Compare
  Compare --> New["In config, not in state"]
  Compare --> Existing["In config and state"]
  Compare --> Removed["In state, not in config"]
  New --> RegisterCore["POST register-monitor"]
  Existing --> Keep["Keep monitor id"]
  Removed --> UnregisterCore["POST unregister-monitor"]
  RegisterCore --> SaveState["Save refreshed state.yaml"]
  Keep --> SaveState
  UnregisterCore --> SaveState
```

## System Report Ingestion

System reports are sent:

- once immediately when the Agent runtime starts;
- then on the global `interval` from config.

Agent collects:

- hostname, OS, platform, architecture, kernel;
- uptime seconds;
- CPU core count, CPU usage, load 1/5/15;
- memory total/used/free/available/percent;
- root disk total/used/free/percent;
- optional location metadata;
- Agent version;
- config summary with reporting interval, monitor count, and monitor type counts.

Core stores system reports in `agent_reports`.

```mermaid
flowchart TD
  Tick["System interval tick or startup"] --> Maintenance{"Agent local maintenance?"}
  Maintenance -- "yes" --> Skip["Skip system report"]
  Maintenance -- "no" --> Collect["Collect system metrics"]
  Collect --> Send["POST /v1/agents/{agent_id}/report"]
  Send --> Auth["Core validates bearer token"]
  Auth --> Store["Insert agent_reports row"]
  Store --> LastSeen["Update agent last_seen"]
  Store --> Stale["Reconcile stale monitor incidents"]
```

## Monitor Report Ingestion

Each monitor runs in its own worker on its own configured interval. The Agent also runs every monitor once at startup.

Monitor reports contain:

- report timestamp;
- health: generally `up` or `down`, with Core-derived states later adding `degraded`, `unknown`, and `stale`;
- metrics payload for successful checks;
- error payload for failed checks.

Core stores monitor reports in `monitor_reports`. If a monitor reports `up`, Core updates `last_successful_report_at`.

```mermaid
flowchart TD
  Tick["Monitor interval tick or startup"] --> Maintenance{"Agent local maintenance?"}
  Maintenance -- "yes" --> Skip["Skip monitor report"]
  Maintenance -- "no" --> RunCheck["Run monitor collector"]
  RunCheck --> Result{"Check result"}
  Result -- "success" --> Metrics["Build metrics payload"]
  Result -- "failure" --> Error["Build error payload"]
  Metrics --> Send["POST /v1/agents/{agent_id}/{monitor_id}/report"]
  Error --> Send
  Send --> CoreAuth["Core validates bearer token"]
  CoreAuth --> Store["Insert monitor_reports row"]
  Store --> UpdateMonitor["Update monitor health and last success"]
  UpdateMonitor --> Incidents["Reconcile incident state"]
  Incidents --> Health["Compute and cache derived health"]
```

## Retry Behavior

The Agent transport performs short request retries first:

- max attempts: 3;
- base delay: 200ms;
- max delay: 5s;
- jitter ratio: 20%;
- retries on network errors, HTTP `429`, and HTTP `5xx`.

If a system or monitor report still fails after transport retries:

- Agent pushes a send closure into a bounded retry queue.
- Queue capacity defaults to 100.
- If full, oldest item is dropped and replaced.
- A retry worker flushes the queue every 30 seconds.
- Shutdown flushes the queue once with a background context.

```mermaid
flowchart TD
  Send["Send report"] --> RetryHTTP{"Transport retry succeeds?"}
  RetryHTTP -- "yes" --> Done["Done"]
  RetryHTTP -- "no" --> Queue["Push to retry queue"]
  Queue --> Capacity{"Queue full?"}
  Capacity -- "yes" --> DropOldest["Drop oldest item"]
  Capacity -- "no" --> Keep["Keep item"]
  DropOldest --> RetryTick
  Keep --> RetryTick["30s retry worker tick"]
  RetryTick --> Flush["Flush queued sends"]
  Flush --> RetryHTTP
```

