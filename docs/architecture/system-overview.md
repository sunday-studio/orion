# System Overview

## Purpose

Orion monitors self-hosted servers without Prometheus, Postgres, Kubernetes, or another external metrics stack. The Agent does local collection and pushes data to Core. Core owns persistence, health decisions, incidents, alerts, and lifecycle management.

## Component Map

```mermaid
flowchart LR
  subgraph Server["Monitored server"]
    Config["config.yaml"]
    State["state.db"]
    Agent["Orion Agent"]
    Collectors["System and monitor collectors"]
    RetryQueue["Retry queue"]
  end

  subgraph CoreHost["Core host"]
    Core["Orion Core API"]
    Services["Core services"]
    DB[("SQLite: orion.db")]
    Archive[("SQLite archive files")]
    Swagger["Generated OpenAPI and Swagger docs"]
  end

  Config --> Agent
  State <--> Agent
  Agent --> Collectors
  Agent --> RetryQueue
  RetryQueue --> Agent
  Agent -- "HTTP /v1" --> Core
  Core --> Services
  Services --> DB
  Services --> Archive
  Core --> Swagger
```

## Agent Responsibilities

- Load user config from YAML.
- Load user config from YAML and update internal Agent state in SQLite.
- Register the server with Core on first run.
- Register configured monitors and unregister removed monitors.
- Collect system reports on the global interval.
- Run each monitor on its own interval.
- Send reports to Core with the server token.
- Retry transient transport failures with exponential backoff and a bounded retry queue.
- Pause report workers while local Agent state says maintenance mode is enabled.

## Core Responsibilities

- Run embedded SQL migrations against SQLite.
- Register or reconnect Agents by `machine_id`.
- Generate and validate Agent bearer tokens.
- Register, revive, list, and soft-delete monitors.
- Store system reports and monitor reports.
- Update last-seen, monitor health, and last-success timestamps.
- Compute derived health for monitors and servers.
- Open, update, and resolve incidents.
- Send or suppress alert deliveries.
- Manage data lifecycle settings, uptime rollups, and raw report archives.
- Expose API routes and generated OpenAPI/Swagger docs.

## Main Runtime Processes

```mermaid
flowchart TD
  Start["Agent process starts"] --> LoadConfig["Load user config"]
  LoadConfig --> LoadState["Load or create internal state"]
  LoadState --> Register["Register Agent and monitors if needed"]
  Register --> Run["Start runtime"]
  Run --> SystemWorker["System report worker"]
  Run --> RetryWorker["Retry queue worker"]
  Run --> MonitorWorkers["One monitor worker per configured monitor"]
  SystemWorker --> CoreReports["Core report API"]
  MonitorWorkers --> CoreReports
  RetryWorker --> CoreReports
```

```mermaid
flowchart TD
  CoreStart["Core process starts"] --> LoadEnv["Load env config"]
  LoadEnv --> Validate["Validate config"]
  Validate --> OpenDB["Open SQLite database"]
  OpenDB --> Migrate["Run embedded SQL migrations"]
  Migrate --> Services["Create Core services"]
  Services --> Router["Configure Gin router"]
  Router --> Listen["Listen for HTTP requests"]
  Listen --> Shutdown["Graceful shutdown on SIGINT/SIGTERM"]
```

## Status Language

Implemented server/monitor health states are:

- `up`
- `down`
- `degraded`
- `maintenance`
- `unknown`
- `stale`

Incident statuses are:

- `open`
- `acknowledged`
- `resolved`

Alert delivery statuses are:

- `pending`
- `sent`
- `failed`
- `suppressed`
- `cooldown`
