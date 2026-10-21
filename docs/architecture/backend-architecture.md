# Orion Backend Architecture

Status: read-only audit snapshot, 2026-10-14, `main` @ `810ac82`.
Scope: `apps/node/`, `apps/core/`, `deploy/`, `Makefile`, `docs/node-core-contract.md`.
Companion: [backend-gaps-and-improvements.md](backend-gaps-and-improvements.md).

Terminology: the code and API say **Node**; older docs (`docs/node-core-contract.md`, `docs/system-design.md`) say **Server**. They are the same thing.

## 1. Components at a glance

| Component | Binary / entry point | Language / stack | Responsibility |
|---|---|---|---|
| Node | `apps/node/main.go` -> `internal/cli` | Go, gopsutil, GORM + SQLite (state only) | Registers with Core, collects host metrics and per-monitor checks, spools and ships reports and service logs |
| Core API | `apps/core/cmd/api/main.go` | Go 1.25, Gin, GORM, mattn/go-sqlite3 (CGO), golang-jwt | REST API, ingestion, health computation, incidents, alerts, status pages, serves the Console SPA from `./web` |
| Core Worker | `apps/core/cmd/worker/main.go` | Go, same module | Claims due Core-managed monitor configs under a DB lease, runs checks (http, tcp, dns, tls, udp, ping, mail, api_request, domain_expiration, synthetic, playwright), reconciles missed heartbeats, writes worker heartbeats |
| Data lifecycle scheduler | goroutine inside Core API (`service/data-lifecycle-scheduler-service.go`) | Go | Hourly rollup, raw report archive, subscriber privacy retention |
| Console | `apps/console` (React/Vite) | TypeScript | UI, built into `apps/core/web` |
| Storage | one SQLite file `${ORION_DATA_DIR}/orion.db` shared by API and Worker processes | SQLite via GORM, 26 embedded SQL migrations, about 28 live tables | System of record. Archive files `raw-reports-YYYY-MM.sqlite` beside it |

Request lifecycle (Core API): `gin.Default()` (Logger + Recovery) -> CORS -> second `gin.Logger()` + `gin.Recovery()` -> `RequestIDMiddleware` -> `RuntimeDiagnosticsMiddleware` (in-memory latency and status counters) -> route group middleware -> handler -> service -> GORM -> SQLite. Unmatched `GET`/`HEAD` falls through to `NoRoute` and serves the SPA (`routes.go:220`).

Route groups (`internal/api/routes.go`):

| Group | Auth | Examples |
|---|---|---|
| Public status pages | none | `/status/:slug`, `/status/:slug/subscribers`, `/feed.atom`, `/` (custom domain) |
| Public v1 | none | `POST /v1/register`, `POST /v1/auth/login`, `POST /v1/heartbeats/:token[/success|/failure]` (token in path) |
| Frontend v1 | JWT only if `ORION_ADMIN_*` set, otherwise open | all `/v1/nodes`, `/v1/monitors`, `/v1/incidents`, `/v1/alerts/*`, `/v1/settings/*`, status page admin |
| Node-to-Core | `Authorization: Bearer <node token>` + path `node_id` match | `/v1/nodes/:node_id/{register-monitor,unregister-monitor,report,logs/batch,:monitor_id/report,maintenance}` |
| Docs / health | none | `/swagger/*`, `/health` |

## 2. System overview

```mermaid
flowchart LR
  subgraph Hosts["Monitored hosts"]
    A1["Node (orion-node)<br/>systemd or launchd"]
    A1S[("state.db<br/>identity, token,<br/>monitor map, report_spool")]
    A1 --- A1S
  end

  subgraph CoreHost["Core host (docker compose)"]
    API["Core API<br/>Gin :8999"]
    WRK["Core Worker<br/>monitor runner"]
    SCH["Lifecycle scheduler<br/>goroutine in API"]
    DB[("orion.db<br/>SQLite shared file")]
    ARC[("raw-reports-YYYY-MM.sqlite<br/>archive")]
    WEB["web/ SPA assets"]
    API --- DB
    WRK --- DB
    SCH --- DB
    SCH --> ARC
    API --- WEB
  end

  CON["Console SPA<br/>browser"]
  PUB["Public status page visitors<br/>and subscribers"]
  HB["Cron jobs and services<br/>heartbeat pings"]
  HOOK["Alert webhooks<br/>Slack, PagerDuty, custom"]
  SMTP["SMTP server<br/>subscriber mail"]
  TARGETS["Internet targets<br/>HTTP, TCP, DNS, TLS, mail, ping"]

  A1 -- "HTTPS JSON, Bearer node token" --> API
  CON -- "JWT or open" --> API
  PUB --> API
  HB -- "POST /v1/heartbeats/token" --> API
  WRK -- "checks, SSRF-guarded dialer" --> TARGETS
  API -- "inline POST on incident open/resolve" --> HOOK
  API -- "SMTP" --> SMTP
```

Key fact for reading everything below: Node-owned monitors only produce data when the Node pushes. Core-owned monitors are pulled by the Worker. Both converge on `ReportService.StoreMonitorReport`, which is the single choke point for health, incident and alert logic.

## 3. Node registration and reporting

Source: `apps/node/internal/registration/registration.go`, `internal/node.go`, `internal/reporting.go`, `internal/state/report-spool.go`, `internal/transport/client.go`; Core `api/node.go`, `service/node-service.go`, `api/report.go`, `service/report-service.go`.

```mermaid
sequenceDiagram
  autonumber
  participant Ag as Node process
  participant St as Node state.db
  participant API as Core API
  participant DB as orion.db

  Note over Ag,St: Startup (cli runNode)
  Ag->>St: Open state, load identity
  alt not registered
    Ag->>Ag: machine_id = md5 of hostname + first MAC
    Ag->>API: POST /v1/register (no credentials)
    API->>DB: SELECT node by machine_id
    alt new machine_id
      API->>DB: INSERT node, token hash (sha256)
      API-->>Ag: node_id + plaintext token (once)
    else known and hash set
      API-->>Ag: 409 node_token_reissue_required
    else revoked
      API-->>Ag: 409 node_token_revoked
    end
    Ag->>St: save node_id, token, core_url
  end
  loop each configured monitor (serial HTTP calls)
    Ag->>API: POST /v1/nodes/id/register-monitor
    API->>DB: upsert monitor by (node_id, name)
  end
  Ag->>API: POST unregister-monitor for removed ones
  Ag->>St: ReplaceMonitors mapping

  Note over Ag,API: Steady state (one goroutine per monitor plus system worker)
  par system worker (interval)
    Ag->>Ag: collect CPU, memory, disk, uptime
    Ag->>API: POST /v1/nodes/id/report
    API->>DB: INSERT node_report, update reporting interval
    API->>DB: ReconcileStaleMonitors (all nodes scanned)
    API->>DB: UPDATE node.last_seen
    Ag->>API: POST logs/batch (service logs, fingerprint dedupe)
  and monitor worker (interval)
    Ag->>Ag: run check (http, command, docker, systemd, pm2 ...)
    Ag->>API: POST /v1/nodes/id/monitor_id/report
    API->>DB: INSERT monitor_report, UPDATE monitor health
    API->>API: incident reconcile, health compute, alert dispatch (section 4)
  end

  Note over Ag,St: Failure path
  Ag-xAPI: network error or 5xx (3 tries, 200ms to 5s backoff)
  Ag->>St: INSERT report_spool (durable)
  loop every 30s
    Ag->>St: list due spool rows (max 100)
    Ag->>API: replay report
    API-->>Ag: 200 then delete row, else exponential backoff 30s to 15m
  end
  API-->>Ag: 401 or 403
  Ag->>Ag: AuthError is fatal, process exits
```

Authentication detail: `AuthMiddleware` extracts the bearer, then `ValidateNodeToken` loads the node row by `node_id`, rejects revoked nodes, and compares `sha256(token)` to `nodes.token_hash` in constant time. Legacy rows with an empty hash fall back to comparing the raw `nodes.token` column.

## 4. Monitor check -> health -> incident -> alert

Source: `worker/app.go`, `service/core-monitor-scheduler-service.go`, `service/report-service.go`, `service/incident-service.go`, `service/incident-state-service.go`, `service/health-service.go`, `service/alert-service.go`, `service/alert-delivery-service.go`.

```mermaid
flowchart TD
  subgraph Producers
    AGM["Node monitor report<br/>POST /nodes/id/monitor_id/report"]
    WRKR["Worker tick every 5s:<br/>claim due configs with 2m DB lease"]
    HBP["Heartbeat ping<br/>POST /heartbeats/token"]
    RUN["Run check serially<br/>per-check timeout"]
    MISS["reconcileMissedHeartbeats<br/>worker only"]
    WRKR --> RUN
  end

  AGM --> STORE
  RUN --> STORE
  HBP --> STORE
  MISS --> STORE

  STORE["ReportService.StoreMonitorReport<br/>redact payload, INSERT monitor_reports"]
  STORE --> UPD["UPDATE monitors.health,<br/>invalidate computed_health cache"]
  UPD --> REC["IncidentService.ReconcileMonitorReport"]

  REC --> MAINT{"Maintenance window<br/>or node maintenance?"}
  MAINT -- yes --> SUP["Set incident_state maintenance<br/>no incident, no alert"]
  MAINT -- no --> HEALTH{"reported health"}
  HEALTH -- "up and no TLS expiry" --> RECOV{"Active incident and<br/>recovery period met?"}
  RECOV -- no --> RECG["state recovering"]
  RECOV -- yes --> RESOLVE["Resolve incident<br/>status resolved, event row"]
  HEALTH -- "down, degraded, stale, tls expiring" --> ACT{"Active incident exists?"}
  ACT -- yes --> UPDI["Update severity, last_event_at,<br/>add monitor_failed event"]
  ACT -- no --> CONF{"Confirmation period or<br/>check count satisfied?"}
  CONF -- no --> DEF["Defer, state recorded only"]
  CONF -- yes --> OPEN["INSERT incident open<br/>plus incident_opened event"]

  OPEN --> QUEUE["AlertService.QueueIncidentNotifications"]
  RESOLVE --> QUEUE
  QUEUE --> GRP{"Grouping policy"}
  GRP -- suppress --> SUPD["alert_delivery suppressed"]
  GRP -- delayed_summary --> SUMD["alert_delivery pending<br/>next_attempt_at = now + delay"]
  GRP -- none --> ROUTES["Evaluate routes and channels,<br/>cooldown check"]
  ROUTES --> SEND["attemptDelivery: HTTP POST to webhook<br/>INLINE inside the ingestion request, 10s timeout"]
  SEND -- 2xx --> SENT["delivery sent"]
  SEND -- failure --> FAILED["delivery failed, next_attempt_at = +5m, max 3"]

  FAILED -. "ProcessDueDeliveries<br/>has no caller" .-> DEAD(("retries and delayed<br/>summaries never run"))
  SUMD -. "same gap" .-> DEAD

  REC --> HC["HealthService.ComputeMonitorHealth<br/>last 20 reports: stale, flapping, degraded"]

  AGR["Node system report"] --> STALE["ReconcileStaleMonitors(node)<br/>scans ALL monitors, opens stale incidents"]
  STALE --> OPEN
  NONODE["Node silent or host down"] -. "no report arrives, nothing triggers" .-> GAP(("no stale incident,<br/>no alert"))
```

Dashed edges are gaps found in this audit, see findings `ORB-001` and `ORB-002`.

Health model (`service/health-service.go`):

| State | Rule |
|---|---|
| `stale` | newest report older than `max(5 min, 5 x reporting_interval)`, measured from the node-supplied `collected_at` |
| `degraded` | at least 3 up/down transitions in last 20 reports, or 30% to 99% failures with at least 5 reports |
| `up` / `down` | latest report health otherwise |
| node `stale` | `nodes.last_seen` older than the same window |
| node `maintenance` | `nodes.maintenance_mode` flag (no expiry) |

Node overall health = node availability combined with monitor rollup (all down -> down, any down/degraded/stale -> degraded).

## 5. Auth flow

```mermaid
flowchart TD
  REQ["Incoming request"] --> ROUTE{"Route group"}

  ROUTE -- "public: register, login,<br/>heartbeat, status pages" --> PUBH["No auth<br/>login has per-IP failure limiter"]

  ROUTE -- "frontend v1" --> FEON{"ORION_ADMIN_USERNAME and<br/>PASSWORD set?"}
  FEON -- "no (default)" --> OPEN["ALL admin endpoints open<br/>incl. alert webhooks and token rotate"]
  FEON -- yes --> JWT{"Bearer JWT valid?<br/>HS256 with ORION_JWT_SECRET"}
  JWT -- yes --> OKF["handler, actor = sub claim"]
  JWT -- no --> R401F["401"]

  ROUTE -- "node-to-core" --> BEAR{"Bearer present?"}
  BEAR -- no --> R401A["401"]
  BEAR -- yes --> LOOKUP["SELECT node WHERE id = path node_id"]
  LOOKUP --> REVOKED{"token_revoked_at set?"}
  REVOKED -- yes --> R401R["401 node_token_revoked"]
  REVOKED -- no --> HASHC{"sha256 of token equals<br/>token_hash, constant time"}
  HASHC -- no --> R401B["401 Invalid token"]
  HASHC -- yes --> OKA["handler"]

  subgraph Login["POST /v1/auth/login"]
    L1["Rate check by ClientIP<br/>5 failures per 60s"] --> L2["constant-time compare<br/>to plaintext env credentials"]
    L2 --> L3["issue JWT, 24h, HS256"]
  end
  PUBH -.-> Login

  subgraph Lifecycle["Node token lifecycle (admin API)"]
    T1["register: token generated, only hash kept"] --> T2["rotate: new token returned once"]
    T2 --> T3["revoke: hash cleared, node locked out"]
    T3 --> T4["reissue: new token, requires revoked state"]
  end
```

`ORION_REQUIRE_FRONTEND_AUTH=true` (set in the shipped compose files) refuses to boot without a 12+ char password and 32+ char JWT secret; the binary default is auth off.

## 6. Data model

There are no declared foreign keys, `PRAGMA foreign_keys` is not enabled, and deletes are mostly soft (`deleted_at` or `lifecycle = 'deleted'`). Relationships below are logical only. Core-managed monitors are attached to a synthetic owner node row (`node_core`).

```mermaid
erDiagram
  NODES ||--o{ NODE_REPORTS : "node_id"
  NODES ||--o{ MONITORS : "node_id"
  NODES ||--o{ SERVICE_LOG_ENTRIES : "node_id"
  NODES ||--o{ INCIDENTS : "node_id"
  MONITORS ||--o{ MONITOR_REPORTS : "monitor_id"
  MONITORS ||--o| CORE_MONITOR_CONFIGS : "monitor_id (core-owned)"
  MONITORS ||--o{ MONITOR_UPTIME_ROLLUPS : "monitor_id"
  MONITORS ||--o{ INCIDENTS : "monitor_id, active_incident_id"
  INCIDENTS ||--o{ INCIDENT_EVENTS : "incident_id"
  INCIDENT_EVENTS }o--o| MONITOR_REPORTS : "monitor_report_id"
  INCIDENTS ||--o{ ALERT_DELIVERIES : "incident_id"
  ALERT_DELIVERIES ||--o{ ALERT_DELIVERY_ATTEMPTS : "alert_delivery_id (only declared FK)"
  ALERT_CHANNELS ||--o{ ALERT_DELIVERIES : "channel name+type"
  ALERT_ROUTES }o--o{ ALERT_CHANNELS : "channel_ids JSON"
  ALERT_GROUPS ||--o{ ALERT_GROUP_MEMBERS : "group_id"
  ALERT_GROUPS ||--o{ ALERT_DELIVERIES : "alert_group_id"
  STATUS_PAGES ||--o{ STATUS_PAGE_SECTIONS : "status_page_id"
  STATUS_PAGES ||--o{ STATUS_PAGE_COMPONENTS : "status_page_id"
  STATUS_PAGE_COMPONENTS ||--o{ STATUS_PAGE_COMPONENT_MAPPINGS : "component_id"
  MONITORS ||--o{ STATUS_PAGE_COMPONENT_MAPPINGS : "monitor_id"
  STATUS_PAGES ||--o{ STATUS_PAGE_INCIDENTS : "status_page_id"
  STATUS_PAGE_INCIDENTS ||--o{ STATUS_PAGE_INCIDENT_UPDATES : "incident"
  STATUS_PAGES ||--o{ STATUS_PAGE_SUBSCRIBERS : "status_page_id"
  STATUS_PAGE_SUBSCRIBERS ||--o{ STATUS_PAGE_SUBSCRIBER_COMPONENTS : "subscriber_id"
  STATUS_PAGE_SUBSCRIBERS ||--o{ STATUS_PAGE_SUBSCRIBER_DELIVERIES : "subscriber_id"

  NODES {
    text id PK
    text machine_id UK
    text token "sha256 marker or legacy plaintext"
    text token_hash
    int token_version
    datetime token_revoked_at
    bool maintenance_mode
    int reporting_interval_seconds
    datetime last_seen
  }
  MONITORS {
    text id PK
    text node_id
    text name "no unique index with node_id"
    text type
    text lifecycle "active, disabled, deleted"
    text health
    text computed_health "cache"
    text active_incident_id
    text incident_state
  }
  MONITOR_REPORTS {
    text id PK
    text monitor_id
    text payload "redacted JSON"
    text collected_at "node clock, string"
    text health
    datetime created_at
  }
  INCIDENTS {
    text id PK
    text status "open, acknowledged, covered, resolved"
    text severity
    text monitor_id
    datetime opened_at
    datetime resolved_at
    datetime covered_until
    int reopen_count
  }
  CORE_MONITOR_CONFIGS {
    text monitor_id PK
    text kind
    text config_json
    text heartbeat_token_hash
    datetime next_run_at
    text lease_owner
    datetime lease_expires_at
  }
  ALERT_DELIVERIES {
    text id PK
    text incident_id
    text status "pending, sent, failed, suppressed, cooldown"
    int attempt_count
    int max_attempts
    datetime next_attempt_at
  }
```

Other tables: `schema_migrations`, `data_lifecycle_settings` (single row id=1), `audit_events`, `core_worker_statuses`.

Retention model: only `node_reports` and `monitor_reports` are archived to monthly SQLite files after `RawReportHotDays` and rolled into `monitor_uptime_rollups`. Nothing prunes `service_log_entries`, `alert_deliveries`, `alert_delivery_attempts`, `incident_events`, `audit_events`, `core_worker_statuses` or the source tables of soft-deleted rows. Subscriber PII has its own retention job.

## 7. Configuration and state

| Surface | Source | Notes |
|---|---|---|
| Core config | env vars (`ORION_*`) plus optional `.env` in CWD (`config.LoadDotEnv`) | validated in `Config.Validate()`; invalid ints and bools silently fall back to defaults (`getEnvInt`, `getEnvBool`) |
| Core data | `${ORION_DATA_DIR}/orion.db` | opened by both API and Worker; both run migrations at startup |
| Node config | YAML (`config.yaml`), validated by `internal/config/validators.go` | monitors, interval, `core_url`, logging, geo opt-in |
| Node state | SQLite `state.db` with `0600` file and `0700` dir | stores node id, plaintext bearer token, monitor mapping, maintenance flag, `report_spool` |
| Runtime tuning | `data_lifecycle_settings` row, edited from Console | hot days, rollups, archive schedule |

## 8. Deployment and runtime topology

```mermaid
flowchart TB
  subgraph Internet
    BR["Operator browser"]
    VIS["Status page visitors"]
    RP["TLS reverse proxy<br/>NOT shipped, operator supplied"]
  end

  subgraph CoreNode["Core node: deploy/docker-compose.yml"]
    direction TB
    C1["orion-core container<br/>image ghcr.io/sunday-studio/orion-core<br/>runs as root, CMD orion-core<br/>port 8999, healthcheck wget /health"]
    C2["orion-core-worker container<br/>same image, CMD orion-core-worker<br/>healthcheck runs migrations then ping"]
    VOL[("named volume orion-data<br/>mounted at /data in both")]
    C1 --- VOL
    C2 --- VOL
    C2 -. "depends_on healthy" .-> C1
  end

  subgraph Fleet["Monitored hosts"]
    direction TB
    L1["Linux: systemd unit orion-node.service<br/>User orion, ProtectSystem strict<br/>state /var/lib/orion/state.db"]
    L2["macOS: launchd com.orion.node<br/>user _orion"]
    INST["deploy/scripts: node-bootstrap.sh<br/>curl release assets, then node-install.sh"]
  end

  BR --> RP
  VIS --> RP
  RP --> C1
  L1 -- "HTTPS push" --> RP
  L2 -- "HTTPS push" --> RP
  INST -.-> L1
  INST -.-> L2

  subgraph CI["GitHub Actions"]
    CIJ["ci.yml: path-filtered<br/>go test, coverage, build, compose config,<br/>generated contract drift"]
    IMG["core-image.yml: multi-arch build,<br/>push :sha :version, optional :latest"]
    BIN["node-binaries.yml: build and attach<br/>release assets, no checksums"]
  end
  IMG --> C1
  BIN --> INST
```

Notes on topology:

- API and Worker are separate processes sharing one SQLite file; scaling out horizontally means moving off SQLite or running a single node.
- The compose file passes `ORION_ADMIN_*` and `ORION_JWT_SECRET` to the Worker too, although the Worker does not authenticate users.
- There is no log shipping, metrics endpoint or tracing; observability is JSON slog on stdout, GORM SQL logs on stdout, and in-memory diagnostics served at `/v1/diagnostics/core` and `/v1/diagnostics/core-worker`.
- Release gate `make release-readiness` = `node-test core-test release-core-build console-build repository-smoke`. `make core-backend-verify` adds race, golangci-lint, govulncheck and contract drift, but CI does not run those three extra gates (see findings).

## 9. Package map (Core)

| Package | Role | Biggest files |
|---|---|---|
| `internal/api` | Gin handlers, DTO mapping, routes, rate limiter, status page HTML | `monitor.go` 462, `routes.go` 358 |
| `internal/service` | business logic (nodes, tokens, reports, health, incidents, alerts, archive, rollups, settings, audit, diagnostics, status page subscribers) | `runtime-diagnostics-service.go` 482, `monitor-service.go` 474 |
| `internal/worker` | check runners and the scheduler loop | `app.go` 460, `playwright_runner.go` 461 |
| `internal/db` | GORM models, embedded migrations, init | `models.go` 447 |
| `internal/monitorvalidation` | Core monitor config validation and redaction | no tests |
| `internal/config`, `internal/shared`, `internal/utils` | config, bootstrap, helpers | |

Package map (Node): `internal/cli` (CLI command surface, install/service/update commands), `internal/collector` (system + monitor collectors), `internal/registration`, `internal/transport`, `internal/state` (SQLite store + report spool), `internal/config`, `internal/logging`, `internal/services`.

All source files respect the 500-line gate (`make code-line-limit` passes; largest is 482).
