# M3: Home Server Monitoring Baseline

## Goal

Make Orion useful for a first real home-server monitoring run.

## Scope

- Agent/Core registration and reporting.
- Implemented monitor types.
- Incidents, alerts, and notification deliveries.
- Agent reliability and local state.
- Core operations, migrations, lifecycle settings, and deployment basics.

## Completed

- Agent/Core endpoint mismatches were fixed.
- OpenAPI and generated frontend SDK are generated from Core route annotations.
- Agent registration, monitor registration, system reports, and monitor reports have integration coverage.
- Agent sends a first report on startup and then reports on configured intervals.
- Agent stores identity, token, maintenance state, and monitor mappings in local `state.db`.
- Core stores agent reports, monitor reports, incidents, incident events, alert deliveries, lifecycle settings, and uptime rollups in SQLite.
- Core derives agent and monitor health, stale state, incident state, and active incident links.
- Incidents open, update, and resolve automatically from monitor reports.
- Alerts support webhook/email channels, cooldowns, recovery notifications, maintenance suppression, and TLS expiry warnings.
- Raw reports can be archived to local SQLite archive files instead of being deleted.
- Core and Console build into one Docker image.
- Agent install, uninstall, upgrade, rollback, systemd, launchd, and local-network notes are documented.

## Monitor Coverage

- HTTP health checks.
- Website checks with status, latency, DNS, and TLS metadata.
- TCP port checks.
- Resource thresholds.
- Docker container status.
- systemd service status.
- PM2 process status.
- Command checks.
- Internal service checks.

## Verification

- Core integration tests cover registration, reports, monitors, incidents, event logs, lifecycle settings, and auth-sensitive response behavior.
- Agent tests cover runtime, collectors, config validation, local state, retry queue, and transport behavior.
- Console build and lint have been run repeatedly during the UI release pass.
- Core Docker image build was smoke-tested with a version tag.
- Agent binary build was smoke-tested with an injected version.

## Open Risks

- No real long-running home-server soak test has been completed yet.
- Durable Agent report spooling is deferred until real use proves it is needed.
- Runtime Core metrics are not implemented yet.

## Next

- Test with one real Core and at least one real Agent.
- Record failures as new testing notes or follow-up issues instead of expanding old planning docs.
