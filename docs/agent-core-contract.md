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
  Registers or returns a server by `machine_id`. No bearer auth (see enrollment below).
  - Rate limited per client IP (`ORION_REGISTER_RATE_LIMIT_PER_MINUTE`, default 30; `429` when exceeded, rejected attempts count).
  - When Core sets `ORION_ENROLLMENT_TOKEN`, the Agent must send the same value in the `X-Orion-Enrollment-Token` header or Core returns `401`. The Agent reads it from `enrollment_token` in its config, or the `ORION_ENROLLMENT_TOKEN` environment variable (which wins). Only first registration needs it; an Agent that already holds state does not re-register. Without the setting Core logs a warning and registration is rate limited only.
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
- Existing `machine_id` returns the existing `agent_id` and token.
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

- `POST /v1/register` carries header `X-Orion-Enrollment-Token: <one-time token>` when the agent has one (`orion-agent enroll --token-file`, or `ORION_ENROLLMENT_TOKEN` / `ORION_ENROLLMENT_TOKEN_FILE`). Agents without a token send no header, which keeps current Core working.
- `401`/`403` means the token is invalid, expired or already used; the agent tells the operator to create a new one in Console under Add server. `429` is retried with backoff.
- On success the response is unchanged (`agent_id`, per-agent `token`). The enrollment token is single use and is never stored by the agent after registration.
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

Rules Core can rely on: `kind` is one of `process`, `unit`, `container`, `path`; `key` is stable across reports; rate fields are omitted (not zero) when not measurable, and the first report after agent start has no rates; at most 200 series per report; no command lines or environment values are ever sent; `net_*` fields are present only for macOS processes and opt-in container collectors; `notes` explain skipped sources and never mean the report failed. Unknown fields are ignored by agents, and Core ignores sections it does not know, so either side can ship first.

## Request compression (agent side, opt-in)

With `transport.compression: gzip` the agent sends request bodies over 1 KiB with `Content-Encoding: gzip`. Default is off because only the log-ingest endpoint decodes gzip request bodies today; Core should add a decoding middleware (with a size limit applied after decompression) before this is enabled in the field.

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
