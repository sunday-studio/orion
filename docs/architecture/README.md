# Orion Backend And Agent Architecture

This folder documents the implemented backend and Agent system. It intentionally excludes the Console UI implementation.

## Read Order

- [system-overview.md](system-overview.md): components, responsibilities, and the main runtime shape.
- [data-ingestion.md](data-ingestion.md): how Agent registration, monitor registration, system reports, and monitor reports move through the system.
- [incident-reconciliation-flow.md](incident-reconciliation-flow.md): how reports open, update, and resolve incidents.
- [agent-monitors.md](agent-monitors.md): every implemented monitor type and what it collects.
- [core-features.md](core-features.md): Core services for health, incidents, alerts, auth, settings, and API routes.
- [persistence-and-lifecycle.md](persistence-and-lifecycle.md): SQLite schema, migrations, rollups, archives, and generated API contract.

## Current System Boundaries

Orion is split into two runtime programs:

- **Agent**: a Go daemon/CLI that runs on a monitored server, reads YAML config, performs local checks, and pushes reports to Core.
- **Core**: a Go API server that stores telemetry in SQLite, computes health, opens/resolves incidents, sends alerts, and exposes an HTTP API.

The frontend is not covered here except where Core exposes frontend-facing API routes.
