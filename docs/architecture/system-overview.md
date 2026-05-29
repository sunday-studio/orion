# System Overview

## Purpose

Orion monitors self-hosted servers without Prometheus, Postgres, Kubernetes, or another external
metrics stack. The Server does local collection and pushes data to Core. Core owns persistence,
health decisions, incidents, alerts, and lifecycle management.

## Component Map

```mermaid
flowchart LR
  subgraph Server["Monitored server"]
    Config["config.yaml"]
    State["state.db"]
    ServerProcess["Orion Server"]
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

  Config --> ServerProcess
  State <--> ServerProcess
  ServerProcess --> Collectors
  ServerProcess --> RetryQueue
  RetryQueue --> ServerProcess
  ServerProcess -- "HTTP /v1" --> Core
  Core --> Services
  Services --> DB
  Services --> Archive
  Core --> Swagger
```

## Server Responsibilities

- Load user config from YAML and update internal Server state in SQLite.
- Register the server with Core on first run.
- Register configured monitors and unregister removed monitors.
- Collect system reports on the global interval.
- Run each monitor on its own interval.
- Send reports to Core with the server token.
- Retry transient transport failures with exponential backoff and a bounded retry queue.
- Pause report workers while local Server state says maintenance mode is enabled.

## Core Responsibilities

- Run embedded SQL migrations against SQLite.
- Register or reconnect Servers by `machine_id`.
- Generate and validate Server bearer tokens.
- Register, revive, list, and soft-delete monitors.
- Store system reports and monitor reports.
- Update last-seen, monitor health, and last-success timestamps.
- Compute derived health for monitors and servers.
- Open, update, and resolve incidents.
- Send or suppress alert deliveries.
- Manage data lifecycle settings, uptime rollups, and raw report archives.
- Expose API routes and generated OpenAPI/Swagger docs.

## Distribution and UI Boundary

The supported deployment shape is one Core host plus one Server process on each monitored machine.
Core serves both the API and the bundled Console UI. Servers never require inbound network access;
they push reports to Core over HTTP/S with a bearer token.

The Core API is the product boundary for future interfaces. The bundled Console is the supported UI
today. A TUI, automation script, or custom UI can be built against the same API later, but Orion does
not currently ship a supported headless-only or alternative-UI distribution.

## Main Runtime Processes

```mermaid
flowchart TD
  Start["Server process starts"] --> LoadConfig["Load user config"]
  LoadConfig --> LoadState["Load or create internal state"]
  LoadState --> Register["Register Server and monitors if needed"]
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
