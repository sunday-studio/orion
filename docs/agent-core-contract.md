# Agent/Core Contract

This document defines the responsibility split between Orion Agent and Orion Core.
The exact HTTP schema lives in `apps/core/openapi.yaml`.

## Principles

- Agent collects and reports data.
- Core stores data, computes health, and owns product decisions.
- Registration is reconciliation: Agent declares what exists; Core persists and revives state as needed.
- Frontend needs must not complicate or delay Agent reporting.

## Agent Responsibilities

- Load config and local state.
- Register itself with Core and reuse its existing identity on restart.
- Register configured monitors and unregister removed monitors.
- Collect system metrics: uptime, CPU, memory, disk, OS/platform, and optional location metadata.
- Run configured monitors and capture monitor-specific metrics and errors.
- Send system and monitor reports on schedule.
- Send an initial report soon after startup.
- Retry temporary network failures without crashing.
- Store only identity, token, and monitor mapping state locally.

The Agent should not:

- Decide aggregate server health.
- Store historical report data.
- Resolve incidents or alerts.
- Depend on Console behavior.

## Core Responsibilities

- Register agents and return stable credentials.
- Authenticate agent-scoped requests.
- Register, soft-delete, and revive monitors.
- Enforce unique monitor names per agent.
- Store agent reports and monitor reports.
- Track last seen and last successful check timestamps.
- Compute server, monitor, stale, degraded, and aggregate health states.
- Own incidents, alerts, retention, migrations, and API behavior.
- Serve the Console and public API.

## API Rules

- Public API routes are versioned under `/v1`.
- `/health` is unversioned.
- Protected agent routes require `Authorization: Bearer <token>`.
- Core validates that the token belongs to the `agent_id` in the path.
- `X-Request-ID` may be supplied by callers; Core echoes or generates one.
- Path IDs and body IDs must agree on agent-scoped routes.

## Endpoints

- `POST /v1/register`
  Registers or returns an agent by `machine_id`. No auth.
- `POST /v1/agents/:agent_id/register-monitor`
  Registers, revives, or rejects duplicate monitors. Requires auth.
- `POST /v1/agents/:agent_id/unregister-monitor`
  Soft-deletes a monitor. Requires auth.
- `POST /v1/agents/:agent_id/report`
  Stores system metrics and updates agent `last_seen`. Requires auth.
- `POST /v1/agents/:agent_id/:monitor_id/report`
  Stores check result and updates monitor state. Requires auth.

## Endpoint Behavior

### Agent Registration

- `machine_id` is the stable identity key.
- Existing `machine_id` returns the existing `agent_id` and token.
- New `machine_id` creates an agent and token.
- Agent metadata from config may be stored as stringified JSON.

### Monitor Registration

- Monitor names are unique per agent.
- Active duplicate names return a conflict.
- Deleted monitors with the same name are revived.
- Monitor metadata from config may be stored as stringified JSON.

### Monitor Unregistration

- Unregistration is a soft delete.
- Deleted monitors should no longer count as active.
- Re-registering the same name can revive the monitor.

### Agent Reports

- Reports update `last_seen`.
- Core stores metrics and location metadata when present.
- Core owns retention and rollups.

### Monitor Reports

- Core verifies the monitor belongs to the authenticated agent.
- Reports store health, metrics, error payload, and collection time.
- Successful reports update last-success timestamps.

## Status Vocabulary

Health states:

- `up`: healthy.
- `down`: failing or unreachable.
- `degraded`: partially healthy or above warning threshold.
- `unknown`: no reliable state yet.
- `stale`: expected data has not arrived recently enough.
- `maintenance`: intentionally suppressed or paused.

Lifecycle states:

- `active`: currently managed by config.
- `disabled`: intentionally not checked.
- `deleted`: removed from config but recoverable.

## Error Handling

Agent behavior:

- Retry temporary transport failures with backoff.
- Log invalid responses and continue where possible.
- Stop reporting on authentication failure until re-registration or user action fixes credentials.
- Continue other checks when one monitor collection fails.

Core behavior:

- Return `400` for invalid input.
- Return `401` for missing or invalid auth.
- Return `404` for missing agent or monitor resources.
- Return `409` for duplicate active monitor names.
- Return `500` for unexpected server errors.
- Include request IDs in errors and logs.

## Security

- Tokens are permanent until explicit rotation/revocation exists.
- Agent stores token in local state; file permissions must protect it.
- Production deployments should use HTTPS or a trusted private network.
- Secret values should not be returned to Console.

## Open Contract Decisions

- Exact stale thresholds for agents and monitors.
- Whether Agent should send all monitor reports individually or support batch reporting.
- Token rotation and revocation flow.
- How maintenance mode affects incident candidates and alerts.
