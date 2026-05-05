# Product Hardening Plan

## Objective

Turn Orion from a working MVP into a reliable self-hosted monitoring product that can be installed, upgraded, operated, tested, and trusted during real incidents.

## Principles

- Preserve the current simple architecture: Go agent, Go Core, SQLite, React UI.
- Harden the agent/Core contract before adding many new monitor types.
- Prefer automated verification over manual confidence.
- Keep installation and operations boring: clear config, predictable paths, simple recovery.

## Phase 1: Verification Baseline

Goal: make every future change safer.

- Install frontend dependencies and verify `npm run build`.
- Add CI for Core Go tests.
- Add CI for Agent Go tests.
- Add CI for frontend build and lint.
- Add Docker image build check.
- Add OpenAPI generation drift check.
- Document the exact local verification commands.

Exit criteria:

- A clean PR can prove Core, Agent, Frontend, OpenAPI, and Docker still compile/build.
- The root README has a short "Verification" section.

## Phase 2: Core Correctness

Goal: make Core behavior explicit and covered by tests.

- Add service tests for agent registration and re-registration.
- Add service tests for monitor registration, duplicate handling, soft delete, and revive.
- Add report storage tests for system and monitor reports.
- Add health computation tests for up, down, unknown, stale, flapping, and degraded states.
- Return precise HTTP status codes for duplicate monitors and invalid monitor ownership.
- Validate path/body ID consistency on agent-scoped routes.
- Confirm list endpoint counts match filtered results.

Exit criteria:

- Core domain behavior is covered by tests.
- API errors are predictable enough for frontend and agents to handle.

## Phase 3: Agent Reliability

Goal: make agents dependable on poor networks and during restarts.

- Add retry with exponential backoff for transport calls.
- Add jitter to system and monitor intervals.
- Run an immediate first collection cycle or document the delayed-first-report behavior.
- Flush pending work on shutdown if batching is introduced.
- Re-check maintenance mode during runtime.
- Add tests around config validation and registration reconciliation.
- Improve CLI exit codes and error text.

Exit criteria:

- Temporary Core/network failure does not permanently break reporting.
- Maintenance mode behavior is clear and test-backed.

## Phase 4: Install, Upgrade & Release

Goal: make Orion practical to deploy on real hosts.

- Add `agent-install.sh`.
- Detect OS and architecture.
- Install binary, config, state, service files, and permissions.
- Generate default config.
- Add dependency checks for optional monitors.
- Add release binaries for Linux/macOS.
- Add `orion-agent upgrade` design and implementation.
- Add rollback plan for failed upgrades.

Exit criteria:

- A fresh Linux/macOS host can install and run an agent from documented commands.
- Releases include versioned Core and Agent artifacts.

## Phase 5: Frontend Operational UX

Goal: make the UI useful during normal operations and incidents.

- Add dashboard summary using `/v1/health/summary`, `/v1/health/issues`, and `/v1/incidents/candidates`.
- Standardize status badges and color semantics.
- Add empty, loading, and error states for each primary view.
- Add polling for list/detail views.
- Add responsive layout pass.
- Add incident-focused view for down, degraded, and stale monitors.
- Add a small Playwright or component-test smoke suite.

Exit criteria:

- Operators can quickly answer what is down, what is stale, and which agent owns the issue.
- Main views remain usable on desktop and mobile.

## Phase 6: Storage & Operations

Goal: keep long-running installs healthy.

- Add explicit migration files and migration runner.
- Add report retention policy.
- Add optional rollups for hourly/daily uptime history.
- Document SQLite backup and restore.
- Review SQLite pragmas and connection settings.
- Add configurable CORS origins.
- Add graceful Core shutdown.
- Add login/registration rate limiting.

Exit criteria:

- A long-running Core instance has bounded storage growth and an upgrade-safe schema path.

## Near-Term Backlog

- Update older docs that still describe pre-`/v1` or Phase 1 behavior.
- Confirm whether command monitor is implemented or only documented.
- Decide whether Core maintenance mode should suppress incident candidates.
- Decide product semantics for stale agents without monitors.
- Decide release/version naming for Core, Agent, API, and SDK.
