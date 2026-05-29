# Server/Core Contract

This document defines the responsibility split between Orion Server and Orion Core.
The exact HTTP schema lives in `apps/core/openapi.yaml`.

## Principles

- Server collects and reports data.
- Core stores data, computes health, and owns product decisions.
- Registration is reconciliation: Server declares what exists; Core persists and revives state as needed.
- Frontend needs must not complicate or delay Server reporting.
- Monitor ownership is explicit: Server monitors are owned and executed by a Server, while Core monitors are owned by Core and executed by the Core monitor worker.
- Report ingestion should converge on the same storage and incident reconciliation path regardless of whether the producer is a Server or the Core monitor worker.

## Server Responsibilities

- Load user config from YAML and local Server-owned state from SQLite.
- Register itself with Core and reuse its existing identity on restart.
- Register configured monitors and unregister removed monitors.
- Collect system metrics: uptime, CPU, memory, disk, OS/platform, and optional location metadata.
- Run configured monitors and capture monitor-specific metrics and errors.
- Send system and monitor reports on schedule.
- Ship bounded batches of its own structured Orion JSONL service logs for product debugging.
- Send an initial report soon after startup.
- Retry temporary network failures without crashing.
- Store only identity, token, maintenance state, and monitor mapping state locally.

The Server should not:

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
- Store deduplicated service log entries shipped by Servers.
- Track last seen and last successful check timestamps.
- Compute server, monitor, stale, degraded, and aggregate health states.
- Own incidents, alerts, retention, migrations, and API behavior.
- Serve the Console and public API.
- Own Core-managed monitor definitions, redaction, schedule state, and lifecycle actions.
- Maintain a stable Core owner identity for Core-managed monitors while existing monitor and incident rows still require `agent_id`.
- Expose monitor owner fields so Console and API clients do not infer ownership from Server IDs.

## Core Monitor Worker Responsibilities

- Run as a separate process from the Core API.
- Claim due Core monitor checks using Core-owned schedule and lease state.
- Execute checks with bounded concurrency and per-check timeouts.
- Store Core monitor reports through the same service path used by Server monitor reports, or through an internal Core API that reaches the same service path.
- Update Core monitor schedule fields such as `next_run_at`, `last_run_at`, `last_checked_at`, and success/failure timestamps.
- Trigger the same incident reconciliation path after reports are stored.
- Expose worker health and diagnostics for Core/Console to display.

The Core monitor worker should not:

- Serve normal Console or public API traffic.
- Accept direct Console calls for create, edit, pause, resume, delete, or normal test flows.
- Execute local command monitors from Console configuration in the M1 HTTP monitor scope.

## Core Monitor Worker Responsibilities

- Run as a separate process from the Core API.
- Execute Core-managed monitor checks, not Server-owned host-local checks.
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
- Server token rotation, revocation, and reissue are frontend/admin authenticated actions; Server-scoped bearer tokens must not authorize their own lifecycle changes.
- `X-Request-ID` may be supplied by callers; Core echoes or generates one.
- Path IDs and body IDs must agree on server-scoped routes.
- Server-scoped monitor registration routes are only for Server-owned monitors.
- Core monitor management routes must live outside `/v1/agents/:agent_id/*`.
- API responses must keep `agent_id` and `agent_name` during M1 compatibility, but new Console work should prefer owner fields when present.

## Endpoints

- `POST /v1/register`
  Registers or returns a server by `machine_id`. No auth.
- `POST /v1/agents/:agent_id/register-monitor`
  Registers, revives, or rejects duplicate monitors. Requires auth.
- `POST /v1/agents/:agent_id/unregister-monitor`
  Soft-deletes a monitor. Requires auth.
- `POST /v1/agents/:agent_id/report`
  Stores system metrics and updates server `last_seen`. Requires auth.
- `POST /v1/agents/:agent_id/logs/batch`
  Stores a bounded batch of structured Server service log entries. Requires auth.
- `POST /v1/agents/:agent_id/:monitor_id/report`
  Stores check result and updates monitor state. Requires auth.
- `GET /v1/logs/service`
  Returns paginated service logs across Servers. Requires frontend/admin auth when configured.
- `GET /v1/agents/:id/service-logs`
  Returns paginated service logs for one Server. Requires frontend/admin auth when configured.
- `GET /v1/diagnostics/core-worker`
  Returns Core monitor worker heartbeat status. Requires frontend/admin auth when configured.

Planned Server token lifecycle endpoints:

- `POST /v1/agents/:agent_id/token/rotate`
  Replaces the active Server token, preserves Server identity and monitor IDs, and returns the new token once. Frontend/admin auth.
- `POST /v1/agents/:agent_id/token/revoke`
  Immediately rejects existing Server-scoped tokens for the Server and records optional revocation context. Frontend/admin auth.
- `POST /v1/agents/:agent_id/token/reissue`
  Issues a replacement token for a revoked Server while preserving Server identity and monitor IDs. Frontend/admin auth.
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

### Server Registration

- `machine_id` is the stable identity key.
- Existing `machine_id` returns the existing `agent_id` and token.
- New `machine_id` creates a server and token.
- Server sends `reporting_interval_seconds` from its global config interval.
- Core stores the reporting interval on create and updates it on re-registration.
- Server metadata from config may be stored as stringified JSON.
- Once token lifecycle controls are implemented, revoked Servers must not recover a token through unauthenticated re-registration by `machine_id`.
- Rotation and reissue preserve the existing `agent_id`, `machine_id`, Server-owned monitor IDs, reports, incidents, maintenance state, and status page component mappings.

### Monitor Registration

- Monitor names are unique per server.
- Active duplicate names return a conflict.
- Deleted monitors with the same name are revived.
- Server sends `reporting_interval_seconds` from each monitor config interval.
- Core stores the monitor reporting interval and uses it for stale detection.
- Monitor metadata from config may be stored as stringified JSON.
- Server re-sends configured monitors on startup so Core can refresh monitor metadata and intervals.
- Server monitor registration writes `owner_kind = agent`, `owner_id = agent_id`, and `runner = agent` after owner fields exist.
- Server monitor registration must reject the synthetic Core owner ID.

### Core Monitor Ownership

- M1 keeps `monitors.agent_id` and `incidents.agent_id` non-null by creating one synthetic Core owner row.
- Core-managed monitors use the synthetic Core owner row for compatibility, but expose `owner_kind = core`, `owner_id = core`, `owner_name = Orion Core`, and `runner = core`.
- Existing Server monitors are backfilled as `owner_kind = agent`, `owner_id = agent_id`, and `runner = agent`.
- Core-managed monitor executable config lives in `core_monitor_configs`, keyed by `monitor_id`.
- Core-managed monitor secrets are write-only from Console and must be redacted in API responses, reports, logs, and event payloads.
- Core monitor schedule state belongs to Core, not the Server.
- Server health and Server detail monitor lists should include only Server-owned monitors, even while Core monitors have a compatibility `agent_id`.

### Monitor Unregistration

- Unregistration is a soft delete.
- Deleted monitors should no longer count as active.
- Re-registering the same name can revive the monitor.

### Server Reports

- Reports update `last_seen`.
- Core stores metrics and location metadata when present.
- Server reports may include `agent_version`.
- Server reports may include a compact `config_summary` with reporting interval, monitor count, and monitor type counts.
- Core may refresh the stored server reporting interval from `config_summary.reporting_interval`.
- Core owns retention and rollups.

### Server Service Logs

- Server reads only its configured Orion structured JSONL log file for the first product version.
- Server ships at most a bounded recent batch on its normal system report cadence.
- Core deduplicates service logs by `(agent_id, fingerprint)` so repeated batches are safe.
- Core stores service logs separately from `/v1/events`; operational events and diagnostic service output remain separate product concepts.
- Core exposes service logs with pagination and filters for server, monitor, source, level, component, and text search.
- Server and Core must redact known sensitive field names such as token, secret, password, API key, and authorization before service logs appear in Console.
- Raw service-manager, journal, launchd, Docker, PM2, or command output is not part of the first service-log ingestion contract. Those sources require explicit allowlists and source-specific redaction before they can be shipped.

### Monitor Reports

- Core verifies the monitor belongs to the authenticated server.
- Reports store health, metrics, error payload, and collection time.
- Successful reports update last-success timestamps.
- Server-authenticated report routes are for Server-owned monitors.
- Core monitor worker reports use the same `monitor_reports` table and incident reconciliation service, but are authorized through an internal worker path or direct shared service access, not through a Server token.
- Core-executed report payloads should identify `runner = core`, target summary, duration, result status, failure stage, redacted request metadata, and truncated response/error details.

## API Response Ownership Fields

Monitor responses should add:

- `owner_kind`: `agent`, `core`, or future `heartbeat`.
- `owner_id`: stable owner identifier.
- `owner_name`: display name such as a Server name or `Orion Core`.
- `runner`: `agent` or `core`.
- `target_summary`: redacted human-readable target.
- `next_run_at`: next scheduled Core worker run when applicable.
- `last_checked_at`: latest completed check regardless of success.
- `paused`: whether Core scheduling is paused.

Incident responses should add `owner_kind`, `owner_id`, and `owner_name` so Console can filter and label Core monitor incidents without treating the synthetic Core owner as a normal server.

Existing `agent_id`, `agent_name`, and Server-scoped list routes remain during M1 for compatibility.

## Status Vocabulary

Health states:

- `up`: healthy.
- `down`: failing or unreachable.
- `degraded`: partially healthy or above warning threshold.
- `unknown`: no reliable state yet.
- `stale`: expected data has not arrived recently enough.
- `maintenance`: intentionally suppressed or paused.

Server health rules:

- A server in maintenance reports `maintenance`.
- A stale server reports `stale`.
- A fresh server with no active monitors reports `up`.
- A fresh server keeps `availability_health = up` even when one or more monitors fail.
- Monitor failures roll up separately as `monitor_health`; mixed monitor failures make the server `overall_health = degraded`, not `down`.
- A fresh server reports `overall_health = down` only when all active monitors are failing.
- Maintenance suppresses incident candidates and should suppress future alert delivery.

Stale rules:

- Server stale state is based on `agent.last_seen` and `agent.reporting_interval_seconds`.
- Monitor stale state is based on the latest monitor report time and `monitor.reporting_interval_seconds`.
- Core treats data as stale after five missed reporting intervals, with a minimum stale window of five minutes.
- If an interval is missing or invalid, Core falls back to 60 seconds.
- A stale monitor report does not make a fresh Server stale; it contributes to monitor rollup health and status explanations.
- Core monitor stale state is based on worker-produced monitor reports and the Core monitor interval, not on the synthetic Core owner row `last_seen`.
- The Core API should not report the whole Core owner row stale merely because a Core-managed monitor is stale.

Lifecycle states:

- `active`: currently managed by config.
- `disabled`: intentionally not checked.
- `deleted`: removed from config but recoverable.

## Error Handling

Server behavior:

- Retry temporary transport failures with backoff.
- Log invalid responses and continue where possible.
- Stop reporting and exit visibly on authentication failure until re-registration or user action fixes credentials.
- Continue other checks when one monitor collection fails.
- Keep the first deploy retry queue in memory only. A restart during a Core outage can lose queued historical reports, but the next scheduled reports refresh current state. Durable offline spooling belongs after first deploy if real usage shows unacceptable report gaps.

Core behavior:

- Return `400` for invalid input.
- Return `401` for missing or invalid auth.
- Return `404` for missing server or monitor resources.
- Return `409` for duplicate active monitor names.
- Return `500` for unexpected server errors.
- Include request IDs in errors and logs.

## Security

- Server token lifecycle semantics live in `docs/architecture/agent-token-lifecycle.md`.
- Server stores token in local SQLite state; file permissions must protect it.
- Server location lookup is opt-in through `geo_location: true`.
- Command monitors execute direct processes by default; shell behavior requires explicitly invoking a shell command.
- Production deployments should use HTTPS or a trusted private network.
- Secret values should not be returned to Console.
- The synthetic Core owner token, if stored for schema compatibility, must not authenticate Server-scoped routes.
- Core monitor URL, header, and future body configuration must be validated and redacted before persistence or response.
- Core monitor execution should deny or explicitly gate sensitive network targets according to deployment policy.

## Open Contract Decisions

- Whether Server should send all monitor reports individually or support batch reporting.
- Whether the Core monitor worker writes reports through direct shared services and SQLite access, or through an internal Core API.
- Whether Core monitor private network targets are allowed by default or require an explicit setting.
