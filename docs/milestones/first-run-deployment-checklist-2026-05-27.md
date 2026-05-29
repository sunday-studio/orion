# First-Run Deployment Smoke Evidence

Date: 2026-05-27

Ticket: `T-20260526-215204-1d90`

## Scope

This pass exercised a real local self-hosted Core deployment in Docker Compose plus a native Server binary on the host. It used a temporary Core image and isolated Docker Compose project so the deployment path, registration, reporting, restart behavior, logs, and backup flow could be verified without touching installed system services.

This is concrete smoke evidence, but it is not the full first-run checklist closeout. The installed Server service and Console UI checks still need to be run on a host where service installation and interactive Console verification are available.

## Environment

- Core image: `ghcr.io/sunday-studio/orion-core:first-run-checklist`
- Compose project: `orion-first-run`
- Core URL: `http://localhost:18999`
- Server binary: `/private/tmp/orion-agent-first-run`
- Server version: `first-run-checklist`
- Server config: `tmp/first-run-agent-config.yaml`
- Server state: `/private/tmp/orion-first-run-state.db`
- Server log: `/private/tmp/orion-first-run-agent.log`
- Backup copy: `/private/tmp/orion-first-run-backup.db`

## Core Service Evidence

Docker Compose reported both services healthy:

```txt
orion-first-run-orion-core-1          ghcr.io/sunday-studio/orion-core:first-run-checklist   Up 6 minutes (healthy)   0.0.0.0:18999->8999/tcp
orion-first-run-orion-core-worker-1   ghcr.io/sunday-studio/orion-core:first-run-checklist   Up 6 minutes (healthy)   8999/tcp
```

Core health returned:

```json
{"database":"ok","service":"orion-core","status":"healthy"}
```

Core worker diagnostics returned one online worker:

```txt
worker_id=core-monitor-worker status=running health=online online_count=1 stale_count=0
```

## Server Registration And Reports

The first Server run used explicit config and state paths:

```sh
/private/tmp/orion-agent-first-run --config tmp/first-run-agent-config.yaml --state /private/tmp/orion-first-run-state.db --no-color run --once
```

It registered one Server and two monitors:

```txt
agent_id: agent-e2bc0939-441e-4077-82ab-e45f667f916d
monitor_mappings: 2
first-run-core-health: monitor-adce317b-08c6-4852-a7b8-dab46a7cdb42
first-run-core-port: monitor-af405f19-f8ee-4134-b01b-c3db39447410
```

Core reported exactly one Server:

```txt
agents.count=1
agent.id=agent-e2bc0939-441e-4077-82ab-e45f667f916d
agent.status=up
agent.availability_health=up
agent.monitor_health=up
agent.monitor_count=2
agent.reporting_interval_seconds=2
```

Core reported two monitor rows after their first interval:

```txt
monitors.count=2
monitor-af405f19-f8ee-4134-b01b-c3db39447410 type=tcp health=up computed_health=up
monitor-adce317b-08c6-4852-a7b8-dab46a7cdb42 type=http-healthcheck health=up computed_health=up
```

Server report and monitor history evidence:

```txt
agent_reports.count=26
monitor_history.count=26 for monitor-af405f19-f8ee-4134-b01b-c3db39447410
latest_agent_report.agent_version=first-run-checklist
latest_agent_report.config_summary.reporting_interval=2s
latest_agent_report.config_summary.monitor_count=2
latest_agent_report.config_summary.monitor_types=http-healthcheck:1,tcp:1
```

## Local State And Logs

The local Server state database and log file were present:

```txt
/private/tmp/orion-first-run-state.db       28K
/private/tmp/orion-first-run-agent.log     114K
```

The log tail showed successful collection, Core report delivery, monitor report delivery, and clean shutdown:

```txt
report successfully sent to core
monitor report successfully sent to core
retry queue flush started: items=0
Server runtime stopped
```

## Restart Behavior

The same Server binary was run again with the same config and state path:

```sh
/private/tmp/orion-agent-first-run --config tmp/first-run-agent-config.yaml --state /private/tmp/orion-first-run-state.db --no-color run --once
```

Registration reconciled to the same IDs:

```txt
agent_id: agent-e2bc0939-441e-4077-82ab-e45f667f916d
first-run-core-health: monitor-adce317b-08c6-4852-a7b8-dab46a7cdb42
first-run-core-port: monitor-af405f19-f8ee-4134-b01b-c3db39447410
```

The post-restart API check still reported:

```txt
agents.count=1
monitors.count=2
```

No duplicate Server or monitor rows were observed.

## Backup Evidence

SQLite backup was created inside the Core container and copied out:

```sh
docker compose -p orion-first-run -f deploy/docker-compose.yml exec -T orion-core sqlite3 /data/orion.db ".backup '/data/orion-first-run-backup.db'"
docker cp 4f0ecf95b61c:/data/orion-first-run-backup.db /private/tmp/orion-first-run-backup.db
```

Backup artifact:

```txt
/private/tmp/orion-first-run-backup.db 632K
```

Backup contents:

```txt
servers=1
monitors=2
agent_reports=26
monitor_reports=52
```

## Follow-Ups

No product failures were found during this pass. The remaining checklist gaps are tracked in Maat:

- `T-20260527-223714-60db`: Verify installed Server service on first-run host.
- `T-20260527-223711-3fef`: Capture Console first-run verification.

Scope note: the Server ran as a native host process against a local Docker Core rather than as an installed service on a separate remote household machine.
