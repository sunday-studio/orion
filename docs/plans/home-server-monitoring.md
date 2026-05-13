# Home Server Monitoring Priorities

## Purpose

Orion is first for my own home server and services.
It should be useful, understandable, and reliable before it tries to be a broad open source monitoring product.

## Direction

- Keep the stack simple: Go Agent, Go Core, SQLite, built-in Console.
- Prefer defaults and clear config over enterprise flexibility.
- Do not require Prometheus, Grafana, Postgres, Kubernetes, or external services.
- Treat open source polish as secondary to operating my own infrastructure well.

## Priority 0: Trust The Current System

- [x] Fix Agent/Core endpoint mismatches.
- [x] Keep generated OpenAPI/Swagger docs aligned with actual Core behavior.
- [x] Add integration coverage for register -> report -> list.
- [x] Cover monitor registration and monitor reports.
- [x] Improve monitor config validation.
- [x] Send the first Agent report immediately on startup.
- [x] Decide health semantics for servers with no monitors.
- [x] Decide how maintenance affects alerts and incidents.

## Priority 1: Useful Monitors

- [x] HTTP status, latency, body contains, and body regex.
- [x] Website monitor with status, latency, DNS, and TLS expiry.
- [x] TCP port check.
- [x] Disk, memory, CPU, and system load thresholds.
- [x] Docker container status.
- [x] systemd service status.
- [x] PM2 process status.
- [x] Command monitor with timeout, exit code, stdout, and stderr.

## Priority 2: Alerts And Incidents

- [x] Alert on down, degraded, stale, high resource usage, and expiring TLS.
- [x] Support cooldowns, recovery notifications, and maintenance suppression.
- [x] Support webhook and email channels.
- [x] Open incidents automatically when health fails.
- [x] Resolve incidents automatically when health recovers.
- [x] Track affected server, monitor, severity, status, timing, and notification attempts.

## Priority 3: Agent Reliability

- [x] Add exponential backoff and jitter.
- [x] Add a bounded retry queue for temporary Core outages.
- [x] Report Agent version and config summary.
- [x] Re-read maintenance mode during runtime.
- [x] Handle shutdown cleanly.
- [x] Improve CLI status and config validation errors.

## Priority 4: Operations

- [x] Add raw report retention and uptime rollups.
- [x] Document SQLite backup and restore.
- [x] Add explicit migrations.
- [x] Add graceful Core shutdown.
- [x] Add configurable CORS origins.
- [x] Add login rate limiting.
- [x] Add useful request logging and database health checks.
- [x] Optimize ingestion-time incident reconciliation.

## Priority 5: Data Lifecycle And Setup

- [x] Add Core data lifecycle settings stored in the database.
- [x] Configure lifecycle defaults during Core setup.
- [ ] Add Console settings UI for lifecycle options.
- [x] Add daily monitor uptime rollup table and service.
- [x] Archive old raw reports to local SQLite archive files instead of deleting them.
- [x] Add Core manual archive and rollup actions.
- [ ] Add Console controls for manual archive and rollup actions.
- [x] Query recent details from hot raw reports and long-term history from rollups.

## Priority 6: Deployment

- [x] Keep Core runnable as a Go binary and with Docker Compose.
- [x] Build Core and Console together as one Docker image.
- [x] Add Agent install and uninstall scripts.
- [x] Add Linux systemd and macOS launchd examples.
- [x] Add example home server config.
- [x] Add Tailscale/local network notes.
- [x] Add upgrade and rollback instructions.

## Later

- Public quick start.
- Example config and env files.
- License, contribution, and security docs.
- CI for Core, Agent, frontend build, and OpenAPI drift.
