# Orion TODO & Product Hardening Roadmap

This roadmap reflects the current repository state as of 2026-05-05. Orion is past the initial MVP: the agent, core API, SQLite persistence, generated frontend client, and React UI are present. The main remaining work is hardening, packaging, test coverage, and operational polish.

## Current Baseline

- [x] Agent registers with Core and stores persistent agent/token state.
- [x] Agent collects system metrics and monitor reports.
- [x] Core stores agents, system reports, monitors, and monitor reports in SQLite.
- [x] Core exposes versioned `/v1` REST routes plus unversioned `/health`.
- [x] Core serves the built SPA from `apps/core/web`.
- [x] Frontend has login, home, list/canvas views, agent detail, and monitor detail pages.
- [x] OpenAPI and Orval-generated frontend API client are wired.
- [x] Dockerfile, `deploy/docker-compose.yml`, and Makefile targets exist.
- [x] `go test ./...` passes for `core` and `agent` when dependencies are available.
- [ ] Frontend build has been reverified in this checkout after installing `apps/console/node_modules`.

## 0. Documentation & Project Hygiene

- [x] Root README describes components, quick start, monitor types, service scripts, and Makefile targets.
- [x] Agent/Core contract documented in `docs/agent-core-contract.md`.
- [x] System design documented in `docs/system-design.md`.
- [ ] Keep this roadmap current after each milestone.
- [x] Add milestone notes under `docs/milestones/`.
- [x] Add planning docs under `docs/plans/`.
- [ ] Reconcile older phase docs with current `/v1` API and monitor model.
- [ ] Document local development flows for Core, Agent, Frontend, Docker, and API generation.
- [ ] Document deployment/release flow once packaging is stable.

## 1. Core Backend

### 1.1 API & Domain

- [x] Version public API under `/v1`.
- [x] Add request ID middleware and `X-Request-ID` response header.
- [x] Agent registration and re-registration by machine ID.
- [x] Track agent `last_seen`.
- [x] Update agent metadata on re-registration.
- [x] Agent maintenance mode field and agent-authenticated maintenance endpoint.
- [x] Monitor registration.
- [x] Monitor unregistration as soft delete.
- [x] Revive soft-deleted monitors.
- [x] Enforce unique monitor name per agent in service logic.
- [x] Persist monitor lifecycle separately from health.
- [x] Store system reports.
- [x] Store monitor reports with JSON payloads.
- [x] Track `last_successful_report_at`.
- [x] Compute derived health for stale data, flapping, and degraded failure rate.
- [x] Cache computed monitor health.
- [ ] Return precise conflict responses for duplicate active monitors instead of generic internal errors.
- [ ] Validate path `agent_id` matches request-body `agent_id` on monitor registration/unregistration.
- [ ] Decide whether maintenance mode should affect health computation and issue summaries.
- [ ] Add token rotation/revocation strategy.
- [ ] Add admin/frontend APIs for toggling maintenance mode if desired.

### 1.2 Frontend-Facing Endpoints

- [x] List agents with pagination, search, status, last-seen, uptime, sort, and order options.
- [x] Agent detail endpoint with latest report.
- [x] Agent health summary endpoint.
- [x] Agent reports endpoint.
- [x] Agent uptime endpoint.
- [x] List monitors per agent with health/lifecycle filters.
- [x] Monitor detail endpoint with recent reports and computed health.
- [x] Monitor history endpoint.
- [x] Monitor uptime endpoint.
- [x] Overall system health endpoint.
- [x] Health issues endpoint.
- [x] Incident candidates endpoint.
- [ ] Add API contract tests for all frontend-facing endpoints.
- [ ] Ensure list counts respect active filters where product expects filtered totals.
- [ ] Add pagination/sort/filter OpenAPI examples.

### 1.3 Data & Storage

- [x] Add GORM indexes for common report, monitor, lifecycle, health, and created-at queries.
- [x] AutoMigrate all current models.
- [ ] Add explicit migration files and migration runner.
- [ ] Add retention policy for system and monitor reports.
- [ ] Add optional hourly/daily rollups.
- [ ] Add soft-delete cleanup job or documented retention behavior.
- [ ] Add database backup/restore guidance.
- [ ] Review SQLite pragmas and connection settings for production use.

### 1.4 Auth, Security & Operations

- [x] Agent bearer-token authentication.
- [x] Frontend login endpoint with JWT when admin env vars are set.
- [x] CORS configured for local frontend development.
- [x] Health endpoint for Core.
- [x] Environment-based config for data dir, port, and frontend auth.
- [ ] Add graceful shutdown for Core HTTP server.
- [ ] Add configurable CORS origins.
- [ ] Add rate limiting or basic abuse protection for login and registration.
- [ ] Avoid logging sensitive values.
- [ ] Add production auth/session documentation.

## 2. Agent

### 2.1 Runtime

- [x] Load YAML config and state files.
- [x] Register agent if needed.
- [x] Register configured monitors.
- [x] Unregister monitors removed from config.
- [x] Run system metrics worker.
- [x] Run one worker per monitor.
- [x] Stop on SIGINT/SIGTERM.
- [x] Support `-once` run mode for debugging.
- [x] Pause reporting while local maintenance mode is enabled at startup.
- [ ] Add retry with exponential backoff for network failures.
- [ ] Add jitter to intervals.
- [ ] Run an immediate collection cycle on startup, or document why the first tick waits.
- [ ] Batch monitor/system reports where useful.
- [ ] Flush pending reports on shutdown.
- [ ] Separate system metrics and monitor pipelines more explicitly.
- [ ] Re-check maintenance state during runtime instead of only at startup.
- [ ] Support Core-triggered maintenance override polling or command channel.
- [ ] Add maintenance reason and optional TTL to Core model/API.

### 2.2 CLI

- [x] `orion-agent start`.
- [x] `orion-agent stop`.
- [x] `orion-agent status`.
- [x] `orion-agent restart`.
- [x] `orion-agent run`.
- [x] `orion-agent maintenance -up`.
- [x] `orion-agent maintenance -down [reason]`.
- [x] `orion-agent config validate`.
- [x] `orion-agent config diff` placeholder/current-config output.
- [ ] Add `orion-agent upgrade`.
- [ ] Make `config diff` compare against a generated/default/reference config.
- [ ] Improve CLI exit codes and user-facing errors.

### 2.3 Installation & Service Management

- [x] Linux systemd unit exists in `deploy/systemd/orion-agent.service`.
- [x] macOS launchd plist exists in `deploy/launchd/com.orion.agent.plist`.
- [x] Uninstall script exists in `deploy/scripts/agent-uninstall.sh`.
- [x] Manual install docs exist in `deploy/scripts/README.md`.
- [ ] Add `agent-install.sh`.
- [ ] Detect OS and architecture in installer.
- [ ] Download or place correct binary in installer.
- [ ] Generate default config on install.
- [ ] Create config/state/log directories with correct permissions.
- [ ] Add dependency checks for optional monitor types such as PM2.
- [ ] Add log rotation guidance or configuration.
- [ ] Add release packaging for Linux/macOS binaries.

### 2.4 Monitor Coverage

- [x] System metrics.
- [x] HTTP healthcheck.
- [x] Website monitor.
- [x] Internal service monitor.
- [x] PM2 monitor.
- [x] Command monitor config type is present and documented in README.
- [ ] Implement command monitor collection or remove it from documented monitor coverage.
- [ ] Postgres monitor.
- [ ] Docker container monitor.
- [ ] systemd service monitor.
- [ ] Redis monitor.
- [ ] Disk threshold monitor.

## 3. Frontend

### 3.1 Foundations

- [x] React/Vite app.
- [x] React Router routes for login, home, agent detail, and monitor detail.
- [x] Orval-generated API client.
- [x] Custom response envelope helper.
- [x] TanStack Query usage for API state.
- [x] Basic login page.
- [x] Environment example for API base URL.
- [ ] Install dependencies and verify `npm run build` in this checkout.
- [ ] Add lint and typecheck verification to CI.
- [ ] Add global API error handling and auth-expiry behavior.
- [ ] Add frontend test harness for key screens.

### 3.2 Product Views

- [x] Agent list.
- [x] Search and status filtering.
- [x] Pagination.
- [x] Expand agent row to show monitors.
- [x] Canvas view.
- [x] Agent detail page.
- [x] Monitor detail page.
- [x] Uptime/SLA component.
- [ ] Dashboard summary using system health, issues, and incident candidates.
- [ ] Clear status badge/color system across all views.
- [ ] Empty states.
- [ ] Error states.
- [ ] Loading states that do not cause layout jumps.
- [ ] Responsive/mobile pass.
- [ ] Polling or realtime updates.
- [ ] Incident-oriented view for down/degraded/stale monitors.

## 4. Testing & Quality Gates

- [x] Minimal Go utility test coverage exists.
- [ ] Core service tests for agent registration/re-registration.
- [ ] Core service tests for monitor lifecycle transitions.
- [ ] Core service tests for report storage and last-seen updates.
- [ ] Health computation tests for up/down/stale/flapping/degraded cases.
- [ ] API handler/integration tests for happy paths and error responses.
- [ ] Agent config validation tests.
- [ ] Agent registration reconciliation tests.
- [ ] Agent collector tests with fakes where possible.
- [ ] Frontend build/typecheck in CI.
- [ ] Frontend component or Playwright smoke tests for main flows.
- [ ] Docker image build smoke test.

## 5. CI/CD & Release

- [x] Dockerfile exists.
- [x] Docker Compose config exists.
- [x] Makefile targets exist for SDK generation, static build, Docker build, and Docker compose up.
- [ ] GitHub Actions: Go test for Core and Agent.
- [ ] GitHub Actions: frontend install/build/lint.
- [ ] GitHub Actions: OpenAPI generation drift check.
- [ ] GitHub Actions: Docker image build.
- [ ] GitHub Actions: release binaries.
- [ ] Versioning scheme for Core, Agent, API, and generated SDK.
- [ ] Changelog/release notes process.

## 6. Suggested Next Milestones

- [x] M1: Documentation and roadmap reset.
- [x] M2: Directory regrouping for agent work.
- [ ] M3: Verification baseline and CI.
- [ ] M4: Core correctness and error semantics.
- [ ] M5: Agent reliability and installer.
- [ ] M6: Frontend operational polish.
- [ ] M7: Retention, migrations, and release packaging.
