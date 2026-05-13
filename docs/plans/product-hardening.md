# Product Hardening Plan

## Objective

Make Orion reliable enough to install, upgrade, operate, test, and trust during real incidents.

## Principles

- Keep the architecture simple.
- Stabilize the Agent/Core contract before adding many monitor types.
- Prefer automated verification over manual confidence.
- Keep install, upgrade, backup, and rollback paths boring.

## Phase 1: Verification Baseline

- Add CI for Core tests, Agent tests, frontend build/lint, Docker build, and OpenAPI generation drift.
- Document the same local verification commands in the README.

Exit criteria: every PR can prove the main apps and generated API still build.

## Phase 2: Core Correctness

- Add tests for agent registration, monitor lifecycle, report storage, health computation, and list counts.
- Return predictable status codes for invalid input, duplicate monitors, and ownership errors.
- Validate path/body ID consistency on agent-scoped routes.

Exit criteria: Core behavior is explicit enough for Agent and Console to depend on.

## Phase 3: Agent Reliability

- Add retry with exponential backoff and interval jitter.
- Run an immediate first collection cycle.
- Re-check maintenance mode during runtime.
- Improve config validation, registration reconciliation, CLI exit codes, and error text.

Exit criteria: temporary Core or network failure does not permanently break reporting.

## Phase 4: Install, Upgrade, Release

- Add Agent install and uninstall scripts.
- Install binaries, config, state, services, and permissions predictably.
- Publish versioned Core and Agent artifacts.
- Define upgrade and rollback behavior.

Exit criteria: a fresh Linux/macOS host can install and run an Agent from documented commands.

## Phase 5: Console UX

- Build the Console around health summary, issues, servers, incidents, logs, and read-only settings.
- Standardize status badges, loading states, error states, polling, and responsive layouts.
- Add a small smoke suite for the main views.

Exit criteria: the UI quickly answers what is down, what is stale, and which server owns the issue.

## Phase 6: Storage And Operations

- Add migrations, retention, uptime rollups, backup/restore docs, CORS config, graceful shutdown, and rate limiting.

Exit criteria: a long-running Core instance has bounded storage growth and an upgrade-safe schema path.

## Decisions Needed

- Whether command monitor is implemented or only documented.
- Version naming for Core, Agent, API, and SDK.
