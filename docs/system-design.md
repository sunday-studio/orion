# Orion System Design

## Overview

Orion monitors small self-hosted infrastructure.

- **Server** runs on each server, collects system and monitor results, and reports to Core.
- **Core** stores reports in SQLite, computes health, exposes the API, and serves the web UI.
- **Console** is the React UI for reading current health, history, incidents, and configuration.

## Architecture

```txt
Server running Server
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

## Server

The Server should stay small and dependable.

- Load YAML config and local state.
- Register itself and configured monitors with Core.
- Collect CPU, memory, disk, uptime, OS, and monitor results.
- Send reports on schedule and once at startup.
- Retry temporary network failures without crashing.
- Keep only the state needed to identify itself and its monitors.

## Core

Core owns persistence, health decisions, and API behavior.

- Register and reconcile servers and monitors.
- Store server and monitor reports.
- Track last-seen and last-success timestamps.
- Compute health, stale state, incidents, alerts, and rollups.
- Enforce auth and request validation.
- Serve the Console from generated static assets.

## Console

The Console is an operations UI, not the configuration source of truth.

- Use **Servers** as the user-facing name for monitored hosts.
- Show current issues before inventory.
- Let users inspect server health, monitor results, incidents, logs, and read-only settings.
- Keep monitor configuration on the Server; manage alert channels through the Core API.

## Data Flow

1. Server starts and loads config/state.
2. Server registers with Core or reuses its existing identity.
3. Server registers configured monitors.
4. Server sends system reports and monitor reports.
5. Core stores reports, updates health, and exposes data to Console.

## Source Of Truth

- API behavior: `apps/core/openapi.yaml`.
- Server/Core responsibility split: `docs/agent-core-contract.md`.
- Backend and Server architecture: `docs/architecture/`.
- Current deployment docs: `docs/deployment/`.
- Completed product history: `docs/milestones/`.
