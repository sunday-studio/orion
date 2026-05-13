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

- Add raw report retention and uptime rollups.
- Document SQLite backup and restore.
- Add explicit migrations.
- Add graceful Core shutdown.
- Add configurable CORS origins.
- Add login rate limiting.
- Add useful request logging and database health checks.

## Priority 5: Deployment

- Keep Core runnable as a Go binary and with Docker Compose.
- Add Agent install and uninstall scripts.
- Add Linux systemd and macOS launchd examples.
- Add example home server config.
- Add Tailscale/local network notes.
- Add upgrade and rollback instructions.

## Later

- Public quick start.
- Example config and env files.
- License, contribution, and security docs.
- CI for Core, Agent, frontend build, and OpenAPI drift.
