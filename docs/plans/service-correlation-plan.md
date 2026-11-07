# Service Correlation: Plan

Status: proposed, awaiting approval. 2026-11-07.
Related: [product-logs-plan.md](product-logs-plan.md), [incident-detail-information-architecture.md](incident-detail-information-architecture.md), [../architecture/passive-metrics-support-matrix.md](../architecture/passive-metrics-support-matrix.md), [../architecture/incident-reconciliation-flow.md](../architecture/incident-reconciliation-flow.md), [decision-log.md](decision-log.md), [database-evaluation.md](database-evaluation.md) (storage), [information-architecture-review.md](information-architecture-review.md) (navigation).

Updated 2026-11-07 after the database evaluation and the navigation review: sample history now lives in its own `metrics.db` behind an interface, and Services sits under a Monitoring tab. Design boards: Services and Navigation pages of the Orion canvas.

## 1. Goal

When something goes wrong, show the monitor result, the app's logs and the resource curve for the
same service on one page, and let any of the three raise or join the incident.

Today you can see an incident and, sometimes, the logs near it. You cannot ask "this monitor just
went down: what did the app log, and what were CPU and memory doing?", and you cannot ask the
reverse.

## 2. What exists today (verified in code)

| Piece | Where | Behaviour |
|---|---|---|
| Monitors to incidents | `core/internal/service` reconciler, `db.Monitor`, `db.Incident` | A report changes monitor health; Core opens, updates or resolves one incident per monitor and queues alerts (`alert_deliveries`). |
| App logs | `logstore`, `db.App`, `log_entries` in `logs.db` | Per-app ingest keys, JSON or OTLP logs. Entries carry `trace_id`, `span_id`, `request_id`. `logstore.Related` groups by request, then trace, then nearby lines. |
| Log alert rules | `db.LogAlertRule`, `service/log-alert-service.go`, run by the Worker | A rule counts matching entries in a window ("threshold") or their absence. It creates its own Core monitor (`monitor-log-alert-<rule id>`, type `log-alert`, owned by Core's system node) and reports it up or down, so the normal incident pipeline applies. |
| Incident logs | `GET /v1/incidents/{id}/logs` (`api/log_alerts_impl.go`) | Logs from 15 minutes before the incident opened to 5 minutes after it resolved. It finds the app only by looking for a log rule whose `monitor_id` equals the incident's monitor. Also adds the incident's Node service logs. |
| Host metrics | `db.NodeReport` | CPU, memory, disk per Node, kept as report rows. |
| App resource metrics | `db.NodeAppResource` | Latest CPU, memory, I/O, network, storage per process, unit or container. One row per key, replaced each report. **No history.** |
| OTLP | `logstore/otlp.go` | Logs only. No metrics or traces endpoint. |

## 3. The gap

An `App` (log source) and a `Monitor` (check) are unrelated objects. The only bridge is a log rule's
synthetic monitor, which exists to raise log-driven incidents, not to describe the user's real
monitor. So:

- A user's HTTP monitor for `checkout.example.com` and the app `checkout-api` are not known to be the
  same thing. An incident on the HTTP monitor shows no app logs unless a log rule happens to exist.
- A log-driven incident and a check-driven incident for the same outage are separate and unlinked.
- There is no resource history, so "memory climbed for 20 minutes before the crash" cannot be shown.
- Apps cannot report their own metrics.

## 4. Design

### 4.1 Service: the thing everything attaches to

Add a `services` table: `id`, `name`, `slug`, `environment`, `node_id` (optional owner), `runbook_url`,
`created_at`, `updated_at`. Add a nullable `service_id` to `apps`, `monitors` and `node_app_resources`
(or a `service_links` join table if one monitor may serve several services; start with a column and
move to a join table only if needed).

```
                    ┌────────────── Service "checkout" ──────────────┐
                    └──┬──────────────┬─────────────────┬────────────┘
                 Monitors (N)     Apps / logs (N)   Resources (N)
                 http, tcp, tls   log_entries       node_app_resources
                 process, docker  JSON / OTLP       + sample history (4.3)
                       │              │                 │
                       └──────────────┴────────┬────────┘
                                               ▼
                                   Incident (check- or log-driven)
                                   one timeline: checks, error logs,
                                   metric curve, alert deliveries
```

Linking is explicit and cheap in the Console: a "Service" picker on the monitor, the app and the
detected-resource rows, plus **suggestions** so most users only click accept:

- OTel resource attribute `service.name` on incoming logs against app slug and monitor name.
- Node discovery names (process, systemd unit, container name) against app slug.
- Monitor target hostname against the app's `service.name` or environment.

A suggestion never links on its own.

Where it shows in the Console: Services is one of three views inside a **Monitoring** tab (Services,
Monitors, Servers), which keeps the top bar at five items once it ships. An app's setup stays under
**Logs, Apps** (moved out of Settings by the navigation review), and a service page links to it.

### 4.2 Incident logs resolve through the Service

`GET /v1/incidents/{id}/logs` stops looking for a log rule. It resolves
`incident.monitor_id -> monitor.service_id -> service apps`, and falls back to the rule lookup so
nothing regresses. The window stays 15 minutes before to 5 minutes after, with a "pre-incident only"
filter because the cause usually sits before the first failed check.

`logstore.Related` is exposed on the incident page: pick an error log, see every line for its trace
or request.

### 4.3 Resource sample history

Samples go in their own SQLite file, `metrics.db`, behind a `MetricStore` interface, the same way
`logs.db` is separate from `orion.db`. The database evaluation measured about 192 MiB per node for
7 days of 200 series (roughly 100 bytes a row), so about 10 GB at 50 nodes and about 100 GB at 500 nodes
(estimate). Keeping that out of `orion.db` also keeps migration backups, which copy `orion.db` up to
three times, small.

- Shape: a narrow `series` table (node, key, kind, labels) and a `samples` table (series id, ts, value
  columns). The latest-row table `node_app_resources` stays in `orion.db` as it is today.
- One-minute resolution, kept 7 days by default; hourly rollups kept 90 days. Both are settings in the
  existing data lifecycle settings, following how `MonitorUptimeRollup` and its backfill already work.
- Cardinality is already capped on the Node (top N processes, 200 series per host). Core enforces the
  same cap on write and drops the excess rather than failing the report.
- Samples are written off the monitor report path, by a single writer, so the request path stays small
  (see `incident-detail-information-architecture.md`, "What Should Stay In The Request Path"). The two
  files cannot commit together, so a crash can leave a latest row without its sample. That is fine: the
  next report fills it.
- `service_id` is stored on the series, not on every sample, so linking or unlinking a service never
  rewrites history.
- Gate before S4 ships: a measured run at the series cap, with `metrics.db` size, prune time and chart
  query time recorded in `backend-hardening-load-results.md`. Revisit SQLite if `metrics.db` passes
  20 GB or 200M rows (see `database-evaluation.md`).

### 4.4 OTLP metrics ingest

`POST /v1/ingest/otlp/v1/metrics` (protobuf and JSON, gzip), same ingest key, auth, size limits and
rate limiter as logs. Map each metric to a sample row keyed by app and service. Gauges and sums first;
histograms (latency percentiles) in a second step. Samples land in the same `series` and `samples`
tables as the Node's passive metrics, tagged by `source` (`os` or `otlp`), so app-reported and
OS-measured numbers appear on one chart. Start with a low default series cap per app, because apps can
emit unbounded label sets, and raise it per app.

### 4.5 Rules that join an existing incident

Log rules keep their synthetic monitor: it gives a clean up/down state machine and reuses the whole
incident pipeline. What changes is that the synthetic monitor inherits the rule app's `service_id`, and
incidents that share a `service_id` and overlap in time are **grouped** using the existing
`alert_groups` tables, so a check failure and a log burst for the same service page once and show as
one story. A later phase can add metric threshold rules (for example memory above X for N minutes) on
the same pattern.

### 4.6 One incident timeline

The incident page (Timeline, Evidence, Related Context in the IA plan) gains a service timeline:
check results, error-log bursts (counts per minute, click through to entries), the CPU and memory
curve for linked resources, and alert deliveries, all on one time axis with the incident window shaded.
Built with the OpenStatus data-table pattern for the log list and the existing chart components for the
curve (Console Table Rules in `AGENTS.md`).

The error-log lane needs counts per minute for an app. Counting raw rows would hit the scan cost the
database evaluation flagged, so keep a small per-app minute rollup (app, minute, severity, count) in
`logs.db`, written by the log writer. The same rollup can serve the Logs histogram later.

### 4.7 Traces (later, optional)

Not part of this plan. With `trace_id` filtering (4.2) and service linking, most of the value of traces
is covered. OTLP traces ingest and a waterfall view need span storage and new UI; decide separately.

## 5. Phases

| Phase | Scope | Owner area | Exit |
|---|---|---|---|
| S0 | Spec: `services` schema, link columns, OpenAPI for services and suggestions, update `node-core-contract.md` only if the Node wire contract changes (it should not) | core, docs | OpenAPI reviewed |
| N1 | Navigation step 1 (separate, ships first): Apps move from Settings to Logs, Alerts and Status become Notify, names stop colliding. See `information-architecture-review.md` | console, docs | The bar is six items and old URLs redirect |
| S1 | Service CRUD, `service_id` on apps, monitors, resources; Console picker and suggestions; Monitoring tab with Services, Monitors, Servers | core, console | A monitor and an app can be linked and unlinked in the UI, and the bar is five items |
| S2 | Incident logs through the Service, pre-incident filter, trace view | core, console | Incident on a linked monitor shows the app's logs without any log rule |
| S3 | Alert grouping by service for log-rule incidents | core | A check failure and a log burst for one service produce one grouped notification |
| S4 | `metrics.db` and `MetricStore`, resource sample history, rollups, prune job, settings; per-app log minute rollup | core | Chart data endpoint returns a series for a linked resource, and the measured gate in 4.3 is recorded |
| S5 | OTLP metrics ingest (gauges and sums) | core | An OTel SDK pointed at Orion shows metrics on the service |
| S6 | Unified incident timeline with metric chart | console | Incident page shows checks, logs and curve together |
| S7 | Metric threshold rules | core, console | A memory or error-rate rule opens an incident |

Each phase is independently shippable. S1 to S3 deliver the core request ("see logs when a monitor
goes wrong") without any new storage.

## 6. Contract and file rules

- Route changes get Core route annotations, then `make generate-openapi` and `make generate-sdk`.
- New Console work lives in `apps/console/`; keep source files at or under 500 lines.
- A metrics endpoint is a new wire surface: document it in `docs/product-logs.md` (or a new
  `docs/product-metrics.md`) alongside the logs ingest docs.
- No Node change is required until S4 needs a `service_id` hint from discovery; if it does, update
  `docs/node-core-contract.md` in the same change.

## 7. Tests and gates

- Unit: service resolution for incident logs (linked, unlinked, rule-only fallback); grouping of
  log-rule and check incidents by service; suggestion matching; sample prune and rollup.
- Integration: OTLP metrics (protobuf, JSON, gzip, bad key, over limit, partial success); sample write
  does not slow `POST` of a monitor report beyond the existing budget.
- Load: extend the backend hardening baseline with the sample table at the series cap.
- Console: browser test that links a monitor to an app and sees logs on its incident.
- Gates: `cd apps/core && go test ./...`, `make generate-openapi`, `make generate-sdk`,
  `make code-line-limit`.

## 8. Decisions I would make (for review)

1. **Service is a real table, not a label.** A string tag on apps and monitors is faster to ship but
   drifts, and status pages and ownership will want a real object. Cost: one table and a picker.
2. **Keep the log rule's synthetic monitor; group incidents by service.** Attaching a rule directly to a
   user's monitor would mix two state machines on one monitor and make recovery ambiguous. Grouping
   keeps both clean and still pages once.
3. **Metric retention: 7 days at one minute, 90 days hourly, configurable.** Revisit after the load
   test in R-1.
4. **Samples live in `metrics.db`, not `orion.db`.** Size and backup cost, per the database
   evaluation. Cost: two files that can't commit together, which this design tolerates.
5. **Services sit under Monitoring, not in their own tab.** Keeps the bar at five. Promoting Services
   later is a one-line nav change, so this is the reversible choice.
6. **Suggestions never auto-link.** A wrong link would attach another app's logs to an incident.
7. **Traces stay out of scope** until S5 is in use and someone asks for them.

## 9. Risks

- **R-1: SQLite growth.** Sample rows are kept out of `orion.db` by using `metrics.db`. Mitigate with
  the series cap, a separate prune job, rollups, and the measured gate before S4 ships. Revisit the
  engine at the triggers in `database-evaluation.md`.
- **R-2: Wrong links.** Name matching can pair unrelated things. Suggestions need a visible reason
  ("same `service.name`") and one-click unlink.
- **R-3: Alert noise from grouping rules.** Group only within the same service and a bounded overlap
  window, and keep per-incident notifications available.
- **R-4: Cardinality from OTLP.** Apps can emit unbounded label sets. Cap series per app and report
  rejected points in `partial_success`, as logs do.
- **R-5: Request-path cost.** Keep sample writes and grouping out of the monitor report request path.

## 10. Open questions

- Can one monitor belong to more than one service (shared database, shared gateway)? If yes, S1 needs
  the join table from day one.
- Should a Service carry its own status-page component mapping, replacing per-monitor mapping later?
- Which metric names should be first-class (`http.server.request.duration`, `process.memory.usage`) and
  which stay raw?
