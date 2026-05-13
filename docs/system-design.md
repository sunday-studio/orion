# Orion System Design

## Overview

Orion monitors small self-hosted infrastructure.

- **Agent** runs on each server, collects system and monitor results, and reports to Core.
- **Core** stores reports in SQLite, computes health, exposes the API, and serves the web UI.
- **Console** is the React UI for reading current health, history, incidents, and configuration.

## Architecture

```txt
Server running Agent
  -> collects system metrics and monitor checks
  -> sends authenticated reports to Core

Core
  -> stores data in SQLite
  -> computes server and monitor health
  -> serves REST API and Console assets

Console
  -> reads Core API
  -> shows operational state
```

## Agent

The Agent should stay small and dependable.

- Load YAML config and local state.
- Register itself and configured monitors with Core.
- Collect CPU, memory, disk, uptime, OS, and monitor results.
- Send reports on schedule and once at startup.
- Retry temporary network failures without crashing.
- Keep only the state needed to identify itself and its monitors.

## Core

Core owns persistence, health decisions, and API behavior.

- Register and reconcile agents and monitors.
- Store agent and monitor reports.
- Track last-seen and last-success timestamps.
- Compute health, stale state, incidents, alerts, and rollups.
- Enforce auth and request validation.
- Serve the Console from generated static assets.

## Console

The Console is an operations UI, not the configuration source of truth.

- Use **Servers** as the user-facing name for agents.
- Show current issues before inventory.
- Let users inspect server health, monitor results, incidents, logs, and read-only settings.
- Keep monitor and alert configuration in YAML/env until the product explicitly adds editing.

## Data Flow

1. Agent starts and loads config/state.
2. Agent registers with Core or reuses its existing identity.
3. Agent registers configured monitors.
4. Agent sends system reports and monitor reports.
5. Core stores reports, updates health, and exposes data to Console.

## Source Of Truth

- API behavior: `apps/core/openapi.yaml`.
- Agent/Core responsibility split: `docs/agent-core-contract.md`.
- Backend and Agent architecture: `docs/architecture/`.
- Product priorities: `docs/plans/home-server-monitoring.md`.
- UI direction: `docs/plans/ui-information-architecture.md`.
