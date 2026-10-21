# Agent/Core Contract

This document defines the responsibility split between Orion Agent and Orion Core.
The exact HTTP schema lives in `apps/core/openapi.yaml`.

## Principles

- Agent collects and reports data.
- Core stores data, computes health, and owns product decisions.
- Registration is reconciliation: Agent declares what exists; Core persists and revives state as needed.
- Frontend needs must not complicate or delay Agent reporting.
- Monitor ownership is explicit: Agent monitors are owned and executed by a Agent, while Core monitors are owned by Core and executed by the Core monitor worker.
- Report ingestion should converge on the same storage and incident reconciliation path regardless of whether the producer is a Agent or the Core monitor worker.

## Agent Responsibilities

- Load user config from YAML and local Agent-owned state from SQLite.
- Register itself with Core and reuse its existing identity on restart.
- Register configured monitors and unregister removed monitors.
- Collect system metrics: uptime, CPU, memory, disk, OS/platform, and optional location metadata.
- Run configured monitors and capture monitor-specific metrics and errors.
- Send system and monitor reports on schedule.
- Ship bounded batches of its own structured Orion JSONL service logs for product debugging.
- Send an initial report soon after startup.
- Retry temporary network failures without crashing.
- Store locally only identity, token, maintenance window, monitor mapping state, and the durable report spool (a SQLite table of reports that could not be delivered, replayed with capped exponential backoff).

The Agent should not:

- Decide aggregate server health.
- Store historical report data.
- Resolve incidents or alerts.
- Depend on Console behavior.
- Create, edit, pause, resume, or delete Core-managed monitors.
- Execute Core monitor checks from Core-owned Console configuration.

## Core Responsibilities

- Register servers and return stable credentials.
- Authenticate server-scoped requests.
- Register, soft-delete, and revive monitors.
- Enforce unique monitor names per server.
- Store server reports and monitor reports.
- Store deduplicated service log entries shipped by Agents.
- Track last seen and last successful check timestamps.
- Compute agent, monitor, stale, degraded, and aggregate health states.
- Own incidents, alerts, retention, migrations, and API behavior.
- Serve the Console and public API.
- Own Core-managed monitor definitions, redaction, schedule state, and lifecycle actions.
- Maintain a stable Core owner identity for Core-managed monitors while existing monitor and incident rows still require `agent_id`.
- Expose monitor owner fields so Console and API clients do not infer ownership from Agent IDs.

## Core Monitor Worker Responsibilities

- Run as a separate process from the Core API.
- Claim due Core monitor checks using Core-owned schedule and lease state.
- Execute checks with bounded concurrency and per-check timeouts.
- Store Core monitor reports through the same service path used by Agent monitor reports, or through an internal Core API that reaches the same service path.
- Update Core monitor schedule fields such as `next_run_at`, `last_run_at`, `last_checked_at`, and success/failure timestamps.
- Trigger the same incident reconciliation path after reports are stored.
- Expose worker health and diagnostics for Core/Console to display.

The Core monitor worker should not:

- Serve normal Console or public API traffic.
- Accept direct Console calls for create, edit, pause, resume, delete, or normal test flows.
- Execute local command monitors from Console configuration in the M1 HTTP monitor scope.

## Core Monitor Worker Responsibilities

- Run as a separate process from the Core API.
- Execute Core-managed monitor checks, not Agent-owned host-local checks.
- Use the Core SQLite database and shared services for worker diagnostics and future Core monitor reports.
- Record worker heartbeat state so Core can expose monitor execution health separately from API health.
- Shut down and restart without affecting Console/API responsiveness.

The Core API should not run polling checks in-process. `/health` reports API and database
availability only; Core worker state is exposed through `/v1/diagnostics/core-worker`.

## API Rules

- Public API routes are versioned under `/v1`.
- `/health` is unversioned.
- Protected server routes require `Authorization: Bearer <token>`.
- Frontend diagnostics routes use frontend/admin auth when configured.
- Core validates that the token belongs to the `agent_id` in the path.
- Agent token rotation, revocation, and reissue are frontend/admin authenticated actions; Agent-scoped bearer tokens must not authorize their own lifecycle changes.
- `X-Request-ID` may be supplied by callers; Core echoes or generates one.
- Path IDs and body IDs must agree on server-scoped routes.
- Agent-scoped monitor registration routes are only for Agent-owned monitors.
- Core monitor management routes must live outside `/v1/agents/:agent_id/*`.
- API responses must keep `agent_id` and `agent_name` during M1 compatibility, but new Console work should prefer owner fields when present.

## Endpoints

- `POST /v1/register`
  Registers a new server by `machine_id`, or reconnects a known active machine when a fresh one-time Add server session authorizes it. No bearer auth (see enrollment below).
  - Rate limited per client IP (`ORION_REGISTER_RATE_LIMIT_PER_MINUTE`, default 30; `429` when exceeded, rejected attempts count).
  - When Core sets `ORION_ENROLLMENT_TOKEN`, the Agent must send the same value in the `X-Orion-Enrollment-Token` header or Core returns `401`. The Agent reads it from `enrollment_token` in its config, or the `ORION_ENROLLMENT_TOKEN` environment variable (which wins). Only first registration needs it; an Agent that already holds state does not re-register. Without the setting Core logs a warning and registration is rate limited only.
- `POST /v1/register/preflight`
  Validates an Add server or shared registration credential without consuming it. Setup uses this before discovery or config writes.
- `POST /v1/agents/:agent_id/register-monitor`
  Registers, revives, or rejects duplicate monitors. Requires auth.
- `POST /v1/agents/:agent_id/unregister-monitor`
  Soft-deletes a monitor. Requires auth.
- `POST /v1/agents/:agent_id/report`
  Stores system metrics and updates server `last_seen`. Requires auth.
- `POST /v1/agents/:agent_id/logs/batch`
  Stores a bounded batch of structured Agent service log entries. Requires auth.
- `POST /v1/agents/:agent_id/:monitor_id/report`
  Stores check result and updates monitor state. Requires auth.
- `GET /v1/logs/service`
  Returns paginated service logs across Agents. Requires frontend/admin auth when configured.
- `GET /v1/agents/:id/service-logs`
  Returns paginated service logs for one Agent. Requires frontend/admin auth when configured.
- `GET /v1/diagnostics/core-worker`
  Returns Core monitor worker heartbeat status. Requires frontend/admin auth when configured.

Planned Agent token lifecycle endpoints:

- `POST /v1/agents/:agent_id/token/rotate`
  Replaces the active Agent token, preserves Agent identity and monitor IDs, and returns the new token once. Frontend/admin auth.
- `POST /v1/agents/:agent_id/token/revoke`
  Immediately rejects existing Agent-scoped tokens for the Agent and records optional revocation context. Frontend/admin auth.
- `POST /v1/agents/:agent_id/token/reissue`
  Issues a replacement token for a revoked Agent while preserving Agent identity and monitor IDs. Frontend/admin auth.
- `GET /v1/agents/:agent_id/token/status`
  Returns non-secret token lifecycle metadata. Frontend/admin auth.

Core monitor admin endpoints:

- `POST /v1/monitors`
  Creates a Core-managed monitor. Frontend/admin auth.
- `PATCH /v1/monitors/:id`
  Edits a Core-managed monitor config or lifecycle field. Frontend/admin auth.
- `DELETE /v1/monitors/:id`
  Soft-deletes a Core-managed monitor. Frontend/admin auth.
- `POST /v1/monitors/:id/pause`
  Pauses a Core-managed monitor without deleting history. Frontend/admin auth.
- `POST /v1/monitors/:id/resume`
  Resumes a paused Core-managed monitor. Frontend/admin auth.
- `POST /v1/monitors/:id/test`
  Executes one immediate Core-managed check and stores the resulting report. Frontend/admin auth.
- `GET /v1/monitors/:id/config`
  Returns redacted Core monitor configuration for editing. Frontend/admin auth.

## Endpoint Behavior

### Agent Registration

- `machine_id` is the stable identity key.
- Existing `machine_id` without a one-time Add server authorization returns `409 agent_token_reissue_required`; Core cannot recover a stored Agent token because only its hash is retained.
- Existing active `machine_id` with a fresh one-time Add server authorization keeps the existing `agent_id` and monitor ownership, rotates the Agent token, and returns the replacement once.
- Existing revoked `machine_id` remains revoked and requires the explicit admin reissue flow.
- New `machine_id` creates a server and token.
- Agent sends `reporting_interval_seconds` from its global config interval.
- Core stores the reporting interval on create and updates it on re-registration.
- Agent metadata from config may be stored as stringified JSON.
- Once token lifecycle controls are implemented, revoked Agents must not recover a token through unauthenticated re-registration by `machine_id`.
- Rotation and reissue preserve the existing `agent_id`, `machine_id`, Agent-owned monitor IDs, reports, incidents, maintenance state, and status page component mappings.

### Monitor Registration

- Monitor names are unique per server.
- Active duplicate names return a conflict.
- Deleted monitors with the same name are revived.
- Agent sends `reporting_interval_seconds` from each monitor config interval.
- Core stores the monitor reporting interval and uses it for stale detection.
- Monitor metadata from config may be stored as stringified JSON.
- Agent re-sends configured monitors on startup so Core can refresh monitor metadata and intervals.
- Agent monitor registration writes `owner_kind = agent`, `owner_id = agent_id`, and `runner = agent` after owner fields exist.
- Agent monitor registration must reject the synthetic Core owner ID.

### Core Monitor Ownership

- M1 keeps `monitors.agent_id` and `incidents.agent_id` non-null by creating one synthetic Core owner row.
- Core-managed monitors use the synthetic Core owner row for compatibility, but expose `owner_kind = core`, `owner_id = core`, `owner_name = Orion Core`, and `runner = core`.
- Existing Agent monitors are backfilled as `owner_kind = agent`, `owner_id = agent_id`, and `runner = agent`.
- Core-managed monitor executable config lives in `core_monitor_configs`, keyed by `monitor_id`.
- Core-managed monitor secrets are write-only from Console and must be redacted in API responses, reports, logs, and event payloads.
- Core monitor schedule state belongs to Core, not the Agent.
- Agent health and Agent detail monitor lists should include only Agent-owned monitors, even while Core monitors have a compatibility `agent_id`.

### Monitor Unregistration

- Unregistration is a soft delete.
- Deleted monitors should no longer count as active.
- Re-registering the same name can revive the monitor.

### Agent Reports

- Reports update `last_seen`.
- Core stores metrics and location metadata when present.
- Agent reports may include `agent_version`.
- Agent reports may include a compact `config_summary` with reporting interval, monitor count, and monitor type counts.
- Core may refresh the stored server reporting interval from `config_summary.reporting_interval`.
- Core owns retention and rollups.

### Agent Product-Log Shipping and One-Shot Commands

The Agent can ship application logs to the product-log ingest endpoint. This is
an optional no-code path; it does not replace direct OTLP from an application,
and its ingest key is never the Agent token. The full app-log envelope and
retention contract is in `docs/product-logs.md`.

`logs.sources` selects files, a journald unit, or a Docker container. Each
source has its own write-only ingest key, supplied through `ingest_key_file` or
`ingest_key_env`; configuration must never contain the key itself. The Agent
persists source offsets and a bounded local spool, follows rotations, applies
configured edge redaction before writing the spool, and retries batches at
`{core_url}/v1/ingest/logs` unless `logs.endpoint` overrides it.

```yaml
logs:
  spool_mb: 64                 # default when omitted or set to 0
  # endpoint: https://core.example.com/v1/ingest/logs
  sources:
    - name: api
      type: file               # file, journald, or docker
      path: /srv/api/log/app.log
      app: api                 # service.name; defaults to name
      from: end                # end (default) or beginning
      multiline_start: '^\\d{4}-\\d{2}-\\d{2}'
      redact: ['(?i)token=[^\\s]+']
      ingest_key_file: /run/orion/api-ingest-key
```

File sources may name a not-yet-created log file; the tailer waits for it.
Journald sources require `unit`, Docker sources require `container`, and the
`orion` service user must be able to read the selected source and key file.
`orion-agent doctor` reports unreadable keys and a missing or unreachable
ingest endpoint without sending a log entry.

`orion-agent emit` sends a single script or cron entry without a running
daemon. It takes a message plus optional `--severity`, `--event`, repeatable
`--attr key=value`, `--app`, `--endpoint`, `--source`, and `--key-file`.
It resolves credentials in this order: `--key-file`, `ORION_INGEST_KEY`, then a
configured source's key. For example:

```sh
orion-agent emit --source api --severity error --event backup.failed 'backup failed'
```

`orion-agent run-job --name <job> [flags] -- <command> [args...]` mirrors a
job's output, ships its bounded output lines followed by `job.finished` or
`job.failed`, returns the job's exit code, and can ping a heartbeat token from
`--heartbeat-token` or `ORION_HEARTBEAT_TOKEN`. Shipping and heartbeat
failures are printed but never replace the job's exit code. `--max-lines`
defaults to 2000; excess output is counted in the final entry.

### Agent Service Logs

- Agent reads only its configured Orion structured JSONL log file for the first product version.
- Agent ships at most a bounded recent batch on its normal system report cadence.
- Core deduplicates service logs by `(agent_id, fingerprint)` so repeated batches are safe.
- Core stores service logs separately from `/v1/events`; operational events and diagnostic service output remain separate product concepts.
- Core exposes service logs with pagination and filters for server, monitor, source, level, component, and text search.
- Agent and Core must redact known sensitive field names such as token, secret, password, API key, and authorization before service logs appear in Console.
- Raw service-manager, journal, launchd, Docker, PM2, or command output is not part of the first service-log ingestion contract. Those sources require explicit allowlists and source-specific redaction before they can be shipped.

### Monitor Reports

- Core verifies the monitor belongs to the authenticated server.
- Reports store health, metrics, error payload, and collection time.
- Successful reports update last-success timestamps.
- Agent-authenticated report routes are for Agent-owned monitors.
- Core monitor worker reports use the same `monitor_reports` table and incident reconciliation service, but are authorized through an internal worker path or direct shared service access, not through a Agent token.
- Core-executed report payloads should identify `runner = core`, target summary, duration, result status, failure stage, redacted request metadata, and truncated response/error details.
- Core redacts sensitive monitor report payload values before `monitor_reports.payload` is persisted. The redaction boundary covers nested JSON keys and embedded text markers for tokens, passwords, secrets, API keys, authorization, credentials, sessions, cookies, and private keys.
- Incident evidence, incident timeline evidence, monitor report history, and report inspection UI must read only the redacted monitor report payload projection. They may show status, timing, failure stage, redacted target summaries, and bounded response/error snippets, but must not expose raw request headers, bearer tokens, heartbeat tokens, cookies, passwords, API keys, or original secret-bearing response samples.

## API Response Ownership Fields

Monitor responses should add:

- `owner_kind`: `agent`, `core`, or future `heartbeat`.
- `owner_id`: stable owner identifier.
- `owner_name`: display name such as a Agent name or `Orion Core`.
- `runner`: `agent` or `core`.
- `target_summary`: redacted human-readable target.
- `next_run_at`: next scheduled Core worker run when applicable.
- `last_checked_at`: latest completed check regardless of success.
- `paused`: whether Core scheduling is paused.

Incident responses should add `owner_kind`, `owner_id`, and `owner_name` so Console can filter and label Core monitor incidents without treating the synthetic Core owner as a normal server.

Existing `agent_id`, `agent_name`, and Agent-scoped list routes remain during M1 for compatibility.

## Status Vocabulary

Health states:

- `up`: healthy.
- `down`: failing or unreachable.
- `degraded`: partially healthy or above warning threshold.
- `unknown`: no reliable state yet.
- `stale`: expected data has not arrived recently enough.
- `maintenance`: intentionally suppressed or paused.

Agent health rules:

- A server in maintenance reports `maintenance`.
- A stale agent reports `stale`.
- A fresh server with no active monitors reports `up`.
- A fresh server keeps `availability_health = up` even when one or more monitors fail.
- Monitor failures roll up separately as `monitor_health`; mixed monitor failures make the server `overall_health = degraded`, not `down`.
- A fresh server reports `overall_health = down` only when all active monitors are failing.
- Maintenance suppresses incident candidates and should suppress future alert delivery.
- Agent runtime: a long-running Agent retries first registration in-process (2s doubling to 60s) while Core is unreachable or answers 5xx/429/403, and fails fast on credential or conflict errors. `401` is the only status treated as a credential rejection; `403` is assumed to come from a proxy and is retried. `SIGHUP` is ignored (no live config reload; restart the service). A monitor missing from local state is retried every 30s instead of its worker exiting. On shutdown, queued reports are kept for the next flush rather than dropped. All HTTP calls honour the shutdown context.
- Worker checks run concurrently, bounded by `ORION_WORKER_CONCURRENCY` (default 16). A lease is renewed every third of its duration while its check runs, and on shutdown in-flight checks get a 15s grace period before being cancelled; leases still held are released so another worker can take them immediately.
- Maintenance always ends. `PUT /v1/agents/{id}/maintenance` accepts `duration_seconds` (default 86400, max 2592000); Core stores `maintenance_until`, treats the agent as in maintenance only until then, and its Worker clears expired flags every 30s. Enabling, disabling and expiry each write an audit event (`agent.maintenance_enabled|disabled|expired`). The Agent keeps the same end time in its local state (`orion-agent maintenance down --for 2h`, default 24h) and resumes reporting by itself when it passes. Maintenance set before this change ends 24h after upgrade.
- Self-monitoring: Core's API process records the Worker's liveness as the monitor "Core worker" (owned by Orion Core). A Worker without a fresh heartbeat (`ORION_WORKER_STALE_SECONDS`) opens a normal incident, and while no Worker is online the API process itself drains the alert queue (deliveries are leased, so this never double-sends) so that page is actually sent. `ORION_DEADMAN_URL`, when set, is pinged every minute so an external service alarms if Core itself dies. `GET /metrics` serves Prometheus text (request counts by route template, ingestion latency, alert queue depth by status, oldest due pending delivery age, worker heartbeat age, active incidents, SQLite busy count and size); it requires `ORION_METRICS_TOKEN` as a bearer token when set, otherwise it follows admin auth.
- Product logs (apps sending their own logs) are a separate contract from Agent service logs: apps authenticate with write-only per-app ingest keys (`Authorization: Bearer orion_ik_...`), never Agent tokens, and an ingest key cannot call read APIs. Ingest is asynchronous (`202`), OpenTelemetry-shaped and schema-free. The full contract for app authors is `docs/product-logs.md`; the machine-readable one is the `ingest`, `apps` and `logs` tags of `apps/core/openapi.yaml`.
- Monitor reports are idempotent. The Agent sends a random `report_id` (UUIDv4) generated once per collected report and kept through the retry queue and spool. Core stores a given `(monitor_id, report_id)` once and returns the original report id on replays with `200`. Report row, monitor health update and incident reconcile commit in one transaction, so a failed ingest is retried from scratch. Reports without `report_id` (older Agents) are not deduplicated.
- Health rules: staleness is measured on Core receive time (`monitor_reports.created_at`), never the Agent-supplied `collected_at`. If the latest report is `down` the monitor is `down` regardless of earlier flapping. A Core heartbeat monitor that has never received a ping becomes stale once `interval + grace` has passed since creation. The Worker waits 60s after start before its first silent-monitor sweep so a Core outage does not flag every monitor at once.
- A host that reports system metrics but has no monitors of its own is covered too: when its `last_seen` passes the stale window (and it is not in maintenance or revoked) the Worker raises an incident on a synthetic monitor named "Agent connectivity: <agent name>" (type `agent-connectivity`). The next system report from the Agent resolves it and parks the monitor. Agents that have monitors are covered by those monitors' stale incidents instead, so one outage never pages twice.
- Core detects silent Agent monitors itself. The Worker sweeps every 30s and opens a `stale` incident for any active, Agent-reported monitor whose last report (by Core receive time, not Agent clock) is older than the stale window, `max(5m, 5 x reporting interval)`. Agents in maintenance mode and Core-managed monitors are skipped. The next successful report resolves the incident through the normal path.
- Alert webhook URLs are validated on create/update and re-checked at dial time with the Core monitor target policy: link-local, multicast and cloud-metadata addresses are always rejected; loopback and private ranges are rejected unless `ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS=true`. URLs with embedded credentials are rejected and redirects are never followed (a 3xx fails the delivery).
- Operator endpoints (Console only, JWT): `POST /v1/alerts/deliveries/{id}/retry` re-queues a `failed` delivery (`202`; `409` if not failed; one extra attempt is granted when attempts were exhausted) and the Worker dispatcher sends it. `POST /v1/incidents/candidates/{id}/dismiss` (candidate id = monitor id) hides a failing monitor from the candidates list until that monitor reports `up` again.
- Console entry endpoints (Console JWT): `GET /v1/overview` (home screen: state empty|ok|issues, `needs_you` incident with notified/acknowledged milestones, ongoing and recent incidents, stats incl. 30-day uptime, first-run flags, 90-day strip per server worst first, max 8), `GET /v1/search?q=` (command palette: monitors, incidents, servers, apps; 5 per type), `GET /v1/auth/me`. The "Add server" wizard uses enrollment sessions: `POST /v1/enrollments` returns a one-time `orn_enr_...` token (shown only in that response, stored hashed, expires in 15 minutes) plus the install command; the installer passes it as `ORION_ENROLLMENT_TOKEN`, so `POST /v1/register` accepts it in `X-Orion-Enrollment-Token`. The first machine to register binds the token; other machines are refused. `GET /v1/enrollments/{id}` is the polling endpoint (status waiting -> claimed -> registered -> reporting, or expired/cancelled; reporting carries agent name, os, arch, version and first report time); `DELETE` cancels. The shared `ORION_ENROLLMENT_TOKEN` keeps working. Core does not hold agent tokens in plaintext, so the wizard cannot show the agent's own token. "Connect an app" needs no new endpoint: poll `GET /v1/apps/{id}` for `last_ingest_at` and `GET /v1/logs?app_id=&limit=1` for the first entry.
- Core's own watching is flagged `system` (migration 40): the owner agent "Orion Core", the Core worker liveness monitor and the synthetic "Agent connectivity" monitors. They are excluded from `/v1/overview` counts and first-run state, `/v1/agents`, `/v1/monitors`, their summaries, `/v1/health/summary` counts and `/v1/search`, and returned only with `?include_system=true` (responses carry `system`). They still raise incidents and alerts, count toward `overall_health` and `health_issue_count`, and show in `/v1/health/issues`.
- Incidents (F2): every incident has `number` / `display_id` ("INC-0231", migration 41, never reused). `POST /v1/incidents` declares one by hand (`title`, `severity`, `monitor_id` or `agent_id`, `summary`, `notify`); `POST /v1/incidents/bulk` runs acknowledge, cover or resolve on up to 100 ids and returns a result per id; `POST /v1/incidents/{id}/notes` adds a timeline note; `PUT /v1/incidents/{id}/responder` assigns the person on point; cover accepts `covered_by` (id or INC number). `GET /v1/incidents` adds `severity`, `q` (title, INC number, monitor or server name), `opened_since` (24h, 7d, RFC3339), `opened_until` and returns `hidden_resolved_count`; `GET /v1/incidents/export` is the same filter as CSV (formula-injection safe, max 10,000 rows). `GET /v1/incidents/{id}` adds `summary`: milestones, time to acknowledge/resolve, failed checks, notified channels, source with target, expected vs actual, status page state.
- Incident screens (F2, part 2): `/v1/health/issues` rows add `kind` (monitor|server), `target_id/name`, `detail`, `since` and the active `incident_id`/`incident_display_id`; servers that stopped reporting appear as `kind: server`, worst first. `/v1/incidents/candidates` adds `detail` and `seen_at`; `POST /v1/incidents/candidates/{id}/open` opens an incident from a candidate (alerts queued; 409 if one is already active). `GET /v1/incidents/{id}/public-update` lists status pages with confirmed subscriber counts and linked update state; `POST /v1/incidents/{id}/public-updates` creates or reuses the linked status page incident, publishes the update (components taken from the incident's impacted components on that page) and optionally emails matching subscribers, returning `subscribers_notified`. The first post of an incident Orion opened itself needs a `title` (the internal title can name hosts and monitors); `title_required` on the targets list says when. Cover rejects cycles.
- Servers (F3): `PATCH /v1/agents/{id}` edits `name`, `labels` (`key=value`, at most 20) and `notes`; `DELETE /v1/agents/{id}?keep_history=true|false` removes a server: token revoked, monitors deleted, open incidents closed (`resolution_kind: decommissioned`), machine id freed so the host can be added again; kept history is purged after 90 days by retention. Agent responses (list and detail) add `labels`, `notes`, `agent_version`, `last_report_at`, `cpu_percent`, `memory_percent`, `disk_percent`, `cpu_series` (12 points, oldest first), `status_text` ("Up 41d", "No report 12m") and `status_detail` ("Disk 91%"). `GET /v1/agents/{id}/reports` takes `since` (1h, 7d or RFC3339) and `until`; `GET /v1/agents/{id}/monitors` takes a multi-value `status` and returns `total`/`hidden_count` plus per-row `status_text`/`status_detail`. Rotating a server token keeps the previous token valid for 10 minutes (`previous_token_valid_until` in the token status, which also carries `last_used_at`); revoking ends both. Alert routes cannot yet match on labels.
- Monitors (F4 batch A): `GET /v1/monitors` rows add `kind`, `target` ("GET host/path", "TCP host:port", "Heartbeat, every 24h", "Agent, <type> on <server>"), `interval_seconds`, `paused`/`paused_at`/`paused_by`, `runbook_url`, `status_text` ("Down 30m", "Up 3d", "Paused"), `status_detail` and `recent_checks` (last 24, oldest first); filters `status` (multi), `agent_id`, `q`, `paused`; `total`/`hidden_count` report what the status filter hides; the summary adds `paused`. `GET /v1/monitors/{id}` adds `summary` (current state since, last check, 30-day incidents, last result, TLS days remaining, masked heartbeat ping URLs with last ping, expected interval and grace); `.../response-times?range=day|week|month` (bucketed average/max duration, timeout line), `.../availability` (today, 7d, 30d, 365d), history rows add `passed`, `summary` ("Timeout, 10,000 ms"), `duration_ms`, `reported_by`. `POST /v1/monitors/{id}/clone`; `runbook_url` on create/update (http/https only); `DELETE /v1/monitors/{id}` keeps check history for 90 days by default (then retention purges it) and `?keep_history=false` deletes it now; the monitor's active incident is resolved (`monitor_removed`) and its undelivered alerts suppressed; incidents are never deleted. Heartbeat full ping URLs are shown only when the monitor is created.
- Monitors (F4 batch B): `POST /v1/monitors/bulk` (`pause|resume|maintenance|delete|clone`, up to 100 ids, per-id results; pause/resume/delete/clone are Core-managed only); `PUT/DELETE /v1/monitors/{id}/maintenance` (window of at most 30 days, `starts_at` default now, `ends_at` or `duration_seconds`, `reason`): checks keep running and reports are stored, but no incident opens or updates while the monitor or its server is in maintenance (this also makes server maintenance mode hold incidents, which it did not before); rows carry `maintenance` and status text "Maintenance". `POST /v1/monitors/{id}/test-run` and `POST /v1/monitors/test-run` (unsaved config, same validation and target policy as create) run the real check once on a throwaway database and return `{passed, status, expected, actual, duration_ms, error, failure_stage, status_code, notes}`; nothing is recorded, 20 runs per minute; heartbeat monitors cannot be test-run. Agent monitors configured in the Console: `POST|GET /v1/agents/{agent_id}/monitor-definitions`, `DELETE .../{definition_id}` (check types disk, memory, process, port; state `waiting` until the agent has registered a monitor with that name and reported, then `reporting`); the agent fetches its definitions with `POST /v1/agents/{agent_id}/monitor-definitions/sync` (agent token). Core only stores and serves the definitions; the agent must pull them and run the checks.

Agent behavior (implemented): after registering, and then every 60 seconds, the daemon calls the sync endpoint with its token. Each definition becomes an agent monitor with the same `name` (registered through `register-monitor`, then reported through the normal monitor report path), so the monitor leaves `waiting` on the first report: `disk` runs as `disk-usage` (filesystem holding `params.path`, default `/`, down above `threshold_percent`, default 90), `memory` as a memory-percent threshold, `process` as a process-name check (case-insensitive executable name), `port` as a TCP connect to `127.0.0.1:{port}`. `interval_seconds` defaults to 60. A definition the agent cannot run (unknown `check_type`, invalid params) is logged and skipped, never reported as failing. A monitor in the agent config file with the same name wins over the Console definition. A failed sync keeps the last good set, which is also persisted next to the state database (`remote-monitors.json`), so neither a Core outage nor an agent restart during one removes monitors; a deleted definition removes its monitor on the next poll. `fail_after` is passed through as monitor meta; Core does not enforce it yet.
- Logs (F7 batch 1, service logs): `GET /v1/logs/facets` (same filters as the search: severity counts, sources with app names, top events with `events_distinct`, environments, the field catalog for a single app, and `field=<key>` for that attribute's most common values), `GET /v1/logs/patterns` (similar messages grouped, numbers/ids/addresses as placeholders; `pattern`, `severity`, `count`, `trend[12]`, `first_seen`, `last_seen`, `is_new` vs the previous window of the same length; reads at most 20,000 newest lines), `GET /v1/logs/{id}/related` (`by` request_id, trace_id or nearby), `GET /v1/logs/ingest-status` (`behind`, `lag_seconds`, `rate_limited`, `storage_full`, queue depth, `lines_today`, `dropped` since start, storage bytes and the sum of app caps), `severity_inferred` on entries whose sender gave no severity and whose first words are a level (ERROR, WARN, ...), a typed `504 {code:"search_timeout"}` when a search, histogram, facet or pattern query passes 10 seconds, `attr=` filters that no longer fail on entries without attributes and that match dotted keys in nested objects. Apps add `status` receiving|quiet|never, `rate_per_second`, `storage_bytes`, `storage_cap_bytes`, `storage_percent`, `lines`, `rate_limit_per_second` (per-app ingest limit, 0 = server default; migration 50) and read-only `redaction.default_keys`; redaction gains custom `rules` [{type key|regex, pattern, replace_with}] applied after the defaults (regexes also run over text bodies and every string value), `POST /v1/apps/{id}/redaction/test {sample, redaction?}` previews rules without saving, `GET /v1/logs/redaction-defaults`, `POST /v1/apps/{id}/keys/{key_id}/rotate` (new key shown once; the old key's `revoked_at` is set 10 minutes ahead so a deploy can switch), `POST /v1/apps/{id}/purge` (delete all of an app's logs). Exports: `POST /v1/logs/export {kind logs, format csv|jsonl, query "severity=error&from=-24h", include_raw}` starts a background job (relative times are fixed when it starts; 100,000 row cap; spreadsheet formulas are neutralised), `GET /v1/exports/{id}` reports `status` queued|running|done|failed, `rows`, `bytes`, `download_url`, and `GET /v1/exports/{id}/download` serves the file for 24 hours.
- Logs (F7 batch 2): **Status logs** (what Orion saw) at `GET /v1/status-logs` (filters `level` info|warn|error, `type` monitor|alert|server|incident|system, `server_id`, `monitor_id`, `incident_id`, `q`, `from`/`to`, default window 24 hours, at most 30 days per query (400 beyond that), relative times like `-15m`, `-24h`, `-7d`; `limit` up to 200; keyset `cursor` with `direction=older|newer`; response has `entries[{id, at, level, type, kind, title, detail, server_*, monitor_*, incident_id, incident_display_id}]`, `total`, `next_cursor`, `prev_cursor`), `/v1/status-logs/facets` (level and type counts, busiest servers and monitors with `*_distinct`), `/histogram`, `/patterns`, `/stream` (SSE, polls about every 2 seconds, resume with `Last-Event-ID`) and `POST /v1/logs/export {kind:"status"}`. The events are read at query time from monitor reports (check passed/failed, and a went-down/recovered event when a report changes state), agent reports, server and monitor creation, incident events, alert deliveries (with the last attempt's HTTP status and duration) and audit events; `/v1/events` is unchanged. **Internal logs** (Core's own lines): the API and the Worker tee their JSON logs into logs.db under the reserved app id `orion-internal` (component core api|worker|scheduler|agent gateway in `attributes.component`; routine successful HTTP requests are skipped, failed and slow ones kept; kept 7 days / 200 MB by default, `ORION_INTERNAL_LOG_RETENTION_DAYS`, `ORION_INTERNAL_LOG_MAX_STORAGE_MB`); read with `GET /v1/internal-logs`, `/facets` (component counts in `values`), `/histogram`, `/patterns`, `/stream` and `POST /v1/logs/export {kind:"internal"}`. Only the process's main logger is recorded (migration helpers that build their own logger are not); URL credentials, secret query parameters and bearer tokens in the text are scrubbed before storage. The live tail re-reads only the last seconds of each source on every poll. Product searches never include these lines.
- Settings (F7 batch 3): `GET|PUT /v1/settings/general` {instance_name, public_url (+ `public_url_source` settings|environment), timezone (IANA), time_format 12|24}; the saved Public Core URL replaces `ORION_PUBLIC_URL` for emails, install commands and status page links in the API at once and in the Worker within a minute (migration 51). `GET /v1/settings/system` returns `version` (stamped at build time with `-X orion/core/internal/buildinfo.Version`), `sign_in` {auth_enabled, username, session_hours}, `registration` {configured, token, source settings|environment|none, rotated_at}, `worker` {status running|stale|missing, healthy, last_tick_at, version}, `database` {engine, size_bytes} and the backup note; `POST /v1/settings/system/registration-token/rotate` creates a new `orn_reg_` token that replaces `ORION_ENROLLMENT_TOKEN` (connected servers keep working). Console sessions last `ORION_SESSION_HOURS` (default 12; was 24). API tokens: `GET|POST /v1/settings/api-tokens`, `DELETE .../{id}` — the secret `orn_api_...` is shown once; send it as `Authorization: Bearer`; `read` scope allows GET/HEAD only (403 otherwise), `write` allows the rest of the admin API except managing tokens and the registration token (403 for any API token); `last_used_at` updates at most once a minute. API tokens never receive the registration token (`GET /v1/settings/system` gives them `registration.token_mask` only) and see subscriber emails (`email` is omitted) and alert channel recipient emails masked; `write` tokens are effectively admin otherwise: they can create agent, ingest and enrollment credentials and regenerate webhook secrets, so keep them as secret as the Console password. Audit log: every successful admin action on a mapped route is recorded (actor = username or API token name, `actor_type` user|api_token) in addition to the actions services already recorded (server tokens, decommission, data lifecycle, status page changes); `GET /v1/settings/audit` {actor, q, action, object_type, from, to, limit, cursor} returns `entries[{id, at, actor, actor_type, action ("Resolved incident"), action_key, target ("INC-0231"), object_type, object_id}]`, `next_cursor` and `actors[]`; `POST /v1/logs/export {kind:"audit"}` exports it. A test fails when a new mutating admin route is neither mapped nor exempt.
- Public status pages (F8): Core renders the public pages itself as self-contained server HTML (one inline stylesheet, no scripts, light and dark by the visitor's system unless `theme_settings.theme_mode` is `light` or `dark`; `accent_color`, `logo_url`, `show_uptime_summary` and `show_incident_history` are honoured) from the `internal/publicpage` package. Browsers (`Accept: text/html`, unless `?format=json`) get HTML and API clients keep the JSON on the same URLs: `GET /status/{slug}` (status, stats, 90 day bars, active incidents, maintenance), `GET /status/{slug}/history` (archive of published incidents and past maintenance, three months a page, `?before=YYYY-MM`; 404 when `show_incident_history` is off), `GET /status/{slug}/incidents/{id}` (updates newest first, UTC). New: `GET|POST /status/{slug}/subscribe` (form, fields `email`, `scope` all|selected, `component_ids`; the answer is the same for known and new addresses), `GET /status/{slug}/feed.rss` next to `feed.atom`, `POST /status/{slug}/subscribers/manage/{token}` (form save; unlike the JSON PUT it does not rotate the manage and unsubscribe links, and selecting every component means all components), and `GET /status/{slug}/subscribers/unsubscribe/{token}` (asks "Leave this list?" and never changes state; the POST unsubscribes and shows the result, an unknown token shows "Invalid token"). `GET .../subscribers/confirm/{token}` and `GET .../manage/{token}` render "You are subscribed" / the preferences page / "This link has expired" for HTML requests. Token pages are `Cache-Control: no-store`, `Referrer-Policy: no-referrer` and noindex. Links use `/status/{slug}/...` paths, which also resolve on a verified custom domain. `/alerts/email/confirm|unsubscribe/{token}` use the same stylesheet. Compact variant: `theme_settings.density` `standard`|`compact` (validated like the other theme keys) or `?compact=1` / `?compact=0` on any themed public page (the query wins) renders the compact layout: no summary cards, a one line status instead of the hero, tighter spacing.
- Status pages (F6 batch B): custom domains are verified. `GET /v1/status-pages/{id}/domain` returns `{domain, status pending|verified|failed, status_text, last_checked_at, verified_at, certificate none|pending|issued, records[{type:"CNAME",host,value}], target_known, fallback_url, verify_after_seconds}`; `POST .../domain/verify` resolves the CNAME now (checks closer than 10 s return the last result); `DELETE .../domain` removes it. The CNAME target is `ORION_STATUS_PAGES_CNAME_TARGET`, else the host of `ORION_PUBLIC_STATUS_URL_ORIGIN`, else `ORION_PUBLIC_URL`. Core serves a custom domain only when it is verified; changing the domain resets verification; domains configured before this change were grandfathered as verified (migration 49). Certificate state follows verification because TLS is terminated by the operator's proxy. Maintenance windows: `GET|POST /v1/status-pages/{id}/maintenance`, `PATCH|DELETE .../{window_id}` with `{title, description, starts_at, ends_at, component_ids[], hold_alerts, notify_subscribers}` and `status` scheduled|in_progress|completed; ends_at must be after starts_at, at most 30 days, at least one component of that page. The public page and `GET /v1/status-pages/{id}/preview` carry `maintenance[]` (in progress or starting within 14 days, visible components only), affected components show `maintenance` while a window is in progress, and the HTML page has a Scheduled maintenance section. With `hold_alerts`, incidents and alerts for monitors or agents mapped to those components are not opened during the window (the same hold as monitor and agent maintenance). With `notify_subscribers`, confirmed subscribers (respecting component preferences) get one email up to 24 hours before the window starts when public status mail is configured; moving the start re-arms it. `POST .../subscribers/import {csv}` adds pending subscribers (source import) and mails each a confirmation; it needs a published page and the subscriber secret, and takes at most 200 rows per request (each row sends its confirmation mail inside the request), so larger lists are sent in batches. Readiness `ready` now also requires every blocking check to pass; subscriber list/detail responses are `Cache-Control: no-store`.
- Status pages (F6 batch A): `GET /v1/status-pages` rows add `public_url` (custom domain, else `/status/<slug>`), `state` published|draft|maintenance, `overall_status`, `component_count`, `subscriber_count` (confirmed), `last_updated`, and `domain_status`. `GET /v1/status-pages/{id}/readiness` returns the publish checklist `{ready, passed, total, checks[{key,label,ok,blocking,detail}]}` (has_components and components_mapped block publishing; description_set and domain_verified advise). `GET /v1/status-pages/slug-check?slug=&exclude_id=`. `GET /v1/status-pages/{id}/badges` lists badge URLs with Markdown/HTML snippets; `/status/{slug}/badge.json` is the shields.io endpoint form. Badges for unknown, draft or private pages are now the neutral grey `unknown` badge (200) so a badge never reveals whether a page exists; a hidden component on a public page still 404s. Create accepts `start_from` blank|monitors (monitors adds a Services section with one mapped component per user monitor) and `theme_settings.timezone` (IANA name). Subscribers: `GET .../subscribers` adds `counts{confirmed,pending,disabled,unsubscribed,total}`, `?q=` and `?state=`, rows add `email` (full address, operators only, present when `ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET` is configured), `scope` ("All components", "API only") and `since`; `GET .../subscribers/export.csv`. Migration 48 adds the domain verification columns.
- Alerts (F5 batch B): rules add `cooldown_seconds` (overrides the global cooldown for that rule's deliveries), `summary`, `last_24h_fired` and `how_it_runs` {when, if, limit, notify[], quiet}; `POST /v1/alerts/rules/{id}/replay?hours=24` lists the notifications the rule would have sent for recent incidents and delivers nothing. Label routes (`GET|POST /v1/alerts/label-routes`, `PATCH|DELETE .../{id}`, `POST .../reorder`, `POST .../dry-run {labels:["env=prod"]}`; migration 47) are ordered matcher lists (`key=value` / `key!=value`, ANDed; keys are the server labels plus severity, event, monitor_type, server, monitor); the first match wins unless `continue_matching`, and a route's channels are added to the channels chosen by rules (deduped per channel; a suppressing rule still blocks them). With no rules at all, an event that no label route matches still goes to every enabled channel (legacy fallback).
- Alerts (F5 batch A): `GET /v1/alerts/deliveries` rows add `alert_title` (rule name, else incident title), `incident_display_id`, `channel_id`, `result_text` ("502", "Timeout", "Sent") and `duration_ms`; filters `status` (comma list), `channel_id`, `since` (1h, 24h, 7d or RFC3339), `q`; `total`/`count`. `GET /v1/alerts/deliveries/{id}`: attempts with `result_text` ("502 Bad Gateway"), `http_status`, `duration_ms`, the rebuilt `payload`, `channel`, `retryable`. Attempts now record the webhook response code and a 300-byte body snippet (migration 46). Channels: `health` failing|healthy|untested, `consecutive_failures`, `last_delivery_text` ("3 failed, 502"), `header_names` (values are write-only, set with `headers`; Host, Content-*, User-Agent and X-Orion-* are refused), `signing_secret_mask`, email `email_delivery_mode` instant|digest_15m (non-critical alerts wait for the next 15 minute mark, then send individually) and `email_subject_prefix`. `GET /v1/alerts/channels/{id}` adds payload preview, paged recent deliveries and the rules using the channel; `POST /v1/alerts/channels/{id}/secret/regenerate` returns the new secret once; `POST /v1/alerts/channels/{id}/test` now also returns `result` {passed, status_code, duration_ms, body, expected, actual, hint, recipients{delivered, skipped}}; `POST /v1/alerts/channels/test-run` tests an unsaved webhook without storing anything; `GET /v1/alerts/payload-preview`.
- Heartbeat tokens may be sent as `Authorization: Bearer <token>` to `POST /v1/heartbeat/success` and `/v1/heartbeat/failure` so the secret stays out of URLs, proxy logs and shell history; the legacy path forms `/v1/heartbeats/{token}[/success|/failure]` keep working.
- `GET /v1/events` pages with a keyset `cursor` (`next_cursor`/`has_more` in the response); each source is read in bounded batches ordered `(created_at, id)` descending, so cost depends on page size, not table size. `offset` still works but is capped (offset+limit <= 5000) and `count` is a lower bound.
- Email alert channels (`type: "email"`) send through SMTP configured only by environment: `ORION_ALERT_SMTP_HOST`, `_PORT` (587), `_USERNAME`, `_PASSWORD` (or `_PASSWORD_FILE`), `_FROM`, `_TLS` (`starttls` default, `tls`, `none`), plus `ORION_PUBLIC_URL` (base for links) and `ORION_ALERT_EMAIL_SECRET` (or `_FILE`; falls back to `ORION_JWT_SECRET`) which signs links. Recipients are managed per channel (`/v1/alerts/channels/{id}/recipients`) with double opt-in: adding an address mails a confirmation link (HMAC token, 72h); until confirmed (`status: confirmed`) it receives nothing, and a delivery with no confirmed recipients is `suppressed`. Confirm and unsubscribe are public pages `/alerts/email/{confirm|unsubscribe}/{token}`: GET only renders a form (link scanners cannot act), POST performs the change. Every alert mail carries an unsubscribe link and `List-Unsubscribe` / `List-Unsubscribe-Post` headers; unsubscribe tokens do not expire. One delivery fans out to all confirmed recipients; `alert_delivery_recipients` records who already got it, so a retry after a partial SMTP failure never duplicates mail. Webhook signing secrets are unchanged. Set a dedicated `ORION_ALERT_EMAIL_SECRET`: when it falls back to `ORION_JWT_SECRET`, rotating the JWT secret invalidates every unsubscribe link in mail already sent. A confirm link stays valid for 72 hours even if the recipient has since unsubscribed (re-confirming needs that mailbox). `/v1/events` keyset timestamps assume `created_at` is stored in UTC.
- Alert delivery is asynchronous. Incident open/resolve only persists `pending` `alert_deliveries` rows; the Core Worker dispatcher (every 15s) sends them, retries failures with backoff up to `max_attempts`, and sends delayed grouped summaries when due. A delivery is leased (`locked_by`, `locked_until`) while in flight so concurrent dispatchers never double-send. Agent report requests never wait on webhook HTTP.

Stale rules:

- Agent stale state is based on `agent.last_seen` and `agent.reporting_interval_seconds`.
- Monitor stale state is based on the latest monitor report time and `monitor.reporting_interval_seconds`.
- Core treats data as stale after five missed reporting intervals, with a minimum stale window of five minutes.
- Core detects staleness itself: the Worker sweeper (see above) flags silent monitors even when the Agent that should be reporting is dead, so a host outage pages without any Agent cooperation.
- If an interval is missing or invalid, Core falls back to 60 seconds.
- A stale monitor report does not make a fresh Agent stale; it contributes to monitor rollup health and status explanations.
- Core monitor stale state is based on worker-produced monitor reports and the Core monitor interval, not on the synthetic Core owner row `last_seen`.
- The Core API should not report the whole Core owner row stale merely because a Core-managed monitor is stale.

Lifecycle states:

- `active`: currently managed by config.
- `disabled`: intentionally not checked.
- `deleted`: removed from config but recoverable.

## Error Handling

Agent behavior:

- Retry temporary transport failures with backoff.
- Log invalid responses and continue where possible.
- Stop reporting and exit visibly on authentication failure until re-registration or user action fixes credentials.
- Continue other checks when one monitor collection fails.
- Persist undeliverable reports in the durable SQLite spool and replay them with capped exponential backoff after Core returns. The in-memory retry queue is only a fallback when the spool write itself fails; it keeps unsent items when interrupted and is lost on exit. Replays are safe because every report carries a `report_id` that Core de-duplicates.

Core behavior:

- Return `400` for invalid input.
- Return `401` for missing or invalid auth.
- Return `404` for missing server or monitor resources.
- Return `409` for duplicate active monitor names.
- Return `500` for unexpected server errors.
- Include request IDs in errors and logs.

## Enrollment

Implemented on both sides: Core enforces the token (`apps/core/internal/api/agent_enrollment.go`) and the agent sends it.

- `POST /v1/register/preflight` checks the credential before setup runs discovery or writes config; it does not claim the one-time token.
- `POST /v1/register` carries header `X-Orion-Enrollment-Token: <one-time token>` when the agent has one (`orion-agent setup --token-file`, `orion-agent enroll --token-file`, or `ORION_ENROLLMENT_TOKEN` / `ORION_ENROLLMENT_TOKEN_FILE`). Agents without a token send no header, which keeps current Core working.
- `401`/`403` means the token is invalid, expired or already used; the agent tells the operator to create a new one in Console under Add server. `429` is retried with backoff.
- On success the response is unchanged (`agent_id`, per-agent `token`). The enrollment token is single use and is never stored by the agent after registration.
- A fresh one-time Add server token can reconnect an active machine that lost its local state. Core preserves its Agent identity and monitor ownership while issuing a new Agent token; the shared registration credential cannot do this.
- Core's "agent connected" signal is the successful registration, followed by the first accepted report.

## Report idempotency

`POST /v1/agents/{id}/report` and `POST /v1/agents/{id}/{monitor_id}/report` bodies carry `report_id` (32 hex chars, generated once per collected report). Spooled replays reuse the same `report_id`. Core stores a report once per `(agent_id, report_id)` and answers `200` for duplicates (backend ORB-019). Verify that system reports, not only monitor reports, are deduplicated. Agents ignore unknown response fields, so Core may adopt this at any time.

## Passive app resources (agent side, pending Core)

`POST /v1/agents/{id}/report` bodies may carry an `app_resources` object (see `docs/architecture/passive-metrics-support-matrix.md`). Apps need no integration; the agent measures from the OS.

```json
"app_resources": {
  "interval_seconds": 60.1,
  "series": [
    {"kind": "unit", "key": "unit:nginx.service", "name": "nginx.service",
     "cpu_percent": 3.2, "memory_bytes": 52428800, "io_read_bytes_per_sec": 0, "io_write_bytes_per_sec": 1024, "pids": 5},
    {"kind": "container", "key": "container:6eac09fd51a6", "name": "6eac09fd51a6", "cpu_percent": 99.7, "memory_bytes": 24444928},
    {"kind": "process", "key": "process:postgres", "name": "postgres", "cpu_percent": 0.7, "memory_bytes": 830000000, "pids": 9},
    {"kind": "path", "key": "path:/var/lib/postgresql", "name": "/var/lib/postgresql", "storage_bytes": 1073741824, "storage_capped": false}
  ],
  "notes": ["cgroup v2 not found at /sys/fs/cgroup"]
}
```

Rules Core can rely on: `kind` is one of `process`, `unit`, `container`, `path`, `volume`, `docker`; `key` is stable across reports; rate fields are omitted (not zero) when not measurable, and the first report after agent start has no rates; at most 200 series per report; no command lines or environment values are ever sent; `net_*` fields are present only for macOS processes and for Docker containers (when the agent user can read the Docker socket; container `key` is always `container:<short id>`; with Docker access `name` is the real container name, otherwise the short id too); with Docker access, `container` series also carry `storage_bytes` (the writable layer), `volume` series are Docker volumes (`key` `volume:<name>`, `storage_bytes`), and two `docker` series (`docker:images`, `docker:build-cache`) carry the disk Docker holds; Docker computes these sizes by walking disk, so the agent refreshes them at most every five minutes; `notes` explain skipped sources and never mean the report failed. Unknown fields are ignored by agents, and Core ignores sections it does not know, so either side can ship first.

## Request compression (agent side, opt-in)

With `transport.compression: gzip` the agent sends request bodies over 1 KiB with `Content-Encoding: gzip`. Default is off because only the log-ingest endpoint decodes gzip request bodies today; Core should add a decoding middleware (with a size limit applied after decompression) before this is enabled in the field.

## Discovery

The agent scans its own host read-only and keeps the result locally (state database) plus the person's choices (dismissed in the state database, ignored in `discovery.ignore`). The local view is served on the control socket (`GET /v1/discovery`, `POST /v1/discovery/rescan`, `POST /v1/discovery/action` with `dismiss`, `restore`, `review`), and `status` on the socket reports `suggestions`, `new_items`, `things_found` and `scanning`.

The same status response feeds `orion-agent status` and is additive JSON (older clients ignore unknown fields): `host` (live CPU, memory, disk and load percentages plus network bytes per second; omitted until the sampler has two samples), `report_millis` (send time of the last 12 system reports, oldest first), `report_interval_seconds`, `missing_log_keys` (log sources with no usable ingest key), and per-monitor `monitor_states` rows with `target`, `since` (when the current health began), `detail` (failure reason) and `value` (a reading such as `91%`). `health` is one of `up`, `slow`, `down`, `degraded`, `unknown`.

### Reporting the scan to Core

- `POST /v1/agents/{agent_id}/discovery` (agent token). Body: `scanned_at`, `enabled`, `kinds[]` (`kind`, `state`, `message`, `fix`, `why`: how scanning each kind went, including the exact fix for one the agent could not read) and `items[]` (`id`, `kind`, `name`, `detail`, `confidence` high|medium|low, `status` suggested|monitored, `managed_in`, `new`, `dismissed`). At most 500 items and 20 kinds; the body is limited to 512 KiB; text fields are cut at 300 characters. Core stores only the latest report per agent (it replaces the previous one) and returns 400 for a malformed one.
- `GET /v1/agents/{id}/discovery` (Console auth) returns the stored scan as `discovery` (null until the agent has reported) with `received_at` and counts: `things`, `suggested`, `monitored`, `new_items` (dismissed items are listed but not counted). The Console's "Detected on this server" panel reads this.
- The agent sends after each scan (at start and every six hours, and on `discover --rescan`), only when the scan changed or a day has passed since the last send. A failed send is retried after the next scan. A Core that answers 404 or 405 (an older Core) is not asked again for a day.
- Only names, ids and short details of things found are sent (service and container names, ports, mount points, and the full path of each log file found, which the Console needs to suggest a log source), never their contents. Core stores the items and never writes them to its logs. `discovery.report: false` keeps the scan on the host. Core does not act on it: nothing is monitored until a person accepts a suggestion.

## Security

- Agent token lifecycle semantics live in `docs/architecture/agent-token-lifecycle.md`.
- Agent stores token in local SQLite state; file permissions must protect it.
- Agent location lookup is opt-in through `geo_location: true`.
- Command monitors execute direct processes by default; shell behavior requires explicitly invoking a shell command.
- Production deployments should use HTTPS or a trusted private network.
- Secret values should not be returned to Console.
- The synthetic Core owner token, if stored for schema compatibility, must not authenticate Agent-scoped routes.
- Core monitor URL, header, and future body configuration must be validated and redacted before persistence or response.
- Monitor report payload redaction is a storage and API boundary, not only a Console presentation concern. Console should render the redacted payload it receives and should not rely on client-side filtering as the primary secret control.
- Core monitor execution should deny or explicitly gate sensitive network targets according to deployment policy.

## Open Contract Decisions

- Whether Agent should send all monitor reports individually or support batch reporting.
- Whether the Core monitor worker writes reports through direct shared services and SQLite access, or through an internal Core API.
- Whether Core monitor private network targets are allowed by default or require an explicit setting.
