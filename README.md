# Orion

Orion is a self-hosted monitoring app for small server setups.

An Agent runs on each machine, collects system metrics and monitor results, and sends them to Core.
Core stores the data in SQLite, computes health, opens incidents, sends alerts, and serves the
Console UI. Core-managed monitors run in a separate Core monitor worker process so API health stays
separate from monitor execution health.

## Preview

| Incidents | Agents |
|---|---|
| ![Incidents list](assets/incidents-list.png) | ![Agents list](assets/agents-list.png) |

| Monitors | Monitor detail |
|---|---|
| ![Monitors list](assets/monitors-list.png) | ![Monitor detail](assets/monitor-detail.png) |

## How It Works

```mermaid
flowchart LR
    subgraph servers [Servers]
        A1[Orion Agent]
        A2[Orion Agent]
    end
    subgraph core [Core host]
        C[Orion Core API]
        W[Core monitor worker]
        DB[(SQLite)]
        UI[Console]
    end
    A1 -->|HTTP/S + token| C
    A2 -->|HTTP/S + token| C
    C --> DB
    W --> DB
    UI --> C
```

- **Agent** runs on Linux/macOS hosts and reports system metrics plus monitor checks.
- **Core** receives reports, stores data, derives health, manages incidents, and serves the API.
- **Core monitor worker** executes Core-managed checks and records worker heartbeat diagnostics.
- **Console** is the web UI for incidents, agents, monitors, alerts, logs, and settings.

## Deploy

### Core

Deploy Core API, Core monitor worker, and Console from the published Docker image. Core stores data
in `/data`, so mount that path to persistent storage. The Compose file starts the API and worker as
separate services that share the same SQLite volume.

With Docker Compose:

```sh
curl -fsSL -o orion-compose.yml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/docker-compose.yml
```

Create a `.env` file next to `orion-compose.yml`:

```sh
cat > .env <<'EOF'
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:<version>
ORION_HTTP_PORT=8999
ORION_ADMIN_USERNAME=admin
ORION_ADMIN_PASSWORD=replace-with-a-strong-password
ORION_JWT_SECRET=replace-with-a-long-random-secret
EOF
```

Optional public status page subscriber email uses a dedicated public sender. Set
`ORION_PUBLIC_STATUS_MAIL_ENABLED=true` plus the `ORION_PUBLIC_STATUS_MAIL_*`,
`ORION_PUBLIC_STATUS_URL_ORIGIN`, and `ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET` variables documented in
[docs/deployment/core-docker.md](docs/deployment/core-docker.md).

Start it:

```sh
docker compose -f orion-compose.yml up -d
```

With plain Docker, run one API container and one worker container against the same `/data` volume:

```sh
docker run -d \
  --name orion-core \
  --restart unless-stopped \
  -p 8999:8999 \
  -v orion-data:/data \
  -e ORION_DATA_DIR=/data \
  -e ORION_PORT=8999 \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD='replace-with-a-strong-password' \
  -e ORION_JWT_SECRET='replace-with-a-long-random-secret' \
  ghcr.io/sunday-studio/orion-core:<version>

docker run -d \
  --name orion-core-worker \
  --restart unless-stopped \
  -v orion-data:/data \
  -e ORION_DATA_DIR=/data \
  -e ORION_WORKER_ID=core-monitor-worker \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD='replace-with-a-strong-password' \
  -e ORION_JWT_SECRET='replace-with-a-long-random-secret' \
  ghcr.io/sunday-studio/orion-core:<version> \
  ./orion-core-worker
```

Expose Core through a stable URL that Agents can reach, then open that URL in the browser. See
[Core Docker deployment](docs/deployment/core-docker.md) for backup, CORS, and upgrade details.

### Agent

Install the Agent on each Linux or macOS host you want to monitor. Use the Core URL that host can
reach:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --core-url https://core.your-domain.tld
```

Pin a release when you want reproducible installs:

```sh
curl -fsSL https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh | bash -s -- \
  --version 0.1.2 \
  --core-url https://core.your-domain.tld
```

The installer creates an editable config with the Core URL, a default reporting interval, location
collection disabled, and no monitor checks:

```yaml
core_url: https://core.your-domain.tld
interval: 60s
geo_location: false
monitors: []
```

Add monitor checks to the installed config when you are ready to track services.

The Agent keeps local runtime state in SQLite:

- Linux config: `/etc/orion/config.yaml`
- Linux state: `/var/lib/orion/state.db`
- Linux log: `/var/log/orion/agent.log`
- macOS config: `/usr/local/etc/orion/config.yaml`
- macOS state: `/usr/local/var/lib/orion/state.db`
- macOS log: `/usr/local/var/log/orion/agent.log`

Do not delete `state.db` during a normal upgrade. It contains the Agent identity, token,
maintenance state, and monitor mapping.

## Operate

Check the installed Agent:

```sh
orion-agent version
orion-agent status
orion-agent doctor
orion-agent logs
orion-agent logs --level error
```

Create a starter config without the release installer:

```sh
orion-agent setup --core-url https://core.your-domain.tld --init-state
```

Run one collection cycle with the installed config and state:

```sh
orion-agent run -once
```

Use verbose output when diagnosing registration, monitor collection, transport, or retry behavior:

```sh
orion-agent run -once -verbose
```

Use JSON output for command surfaces that support automation:

```sh
orion-agent status --json
orion-agent config show --json
orion-agent doctor --json
```

Normal monitor config changes do not need a new install. Edit the installed config, then restart the
service so the Agent reconciles monitors by name.

Linux:

```sh
orion-agent restart
```

macOS:

```sh
orion-agent restart
```

If you change `core_url`, point the Agent at a fresh Core database, or otherwise need a new Agent
identity, use reconfigure:

```sh
orion-agent reconfigure
```

Update the installed Agent binary while preserving config and state. The update command also resets
service failure throttles, starts the service again, prints service status, and shows recent service
logs:

```sh
orion-agent update
orion-agent update -version 0.1.2
```

See [Agent install and upgrade](docs/deployment/agent-install-upgrade.md) for service logs,
rollback, Docker monitor permissions, and local network notes.

## Monitor Types

Supported checks:

- HTTP health checks
- Websites
- TCP ports
- Resource thresholds
- Docker containers
- systemd services
- PM2 processes
- Commands
- Internal services

See [Agent monitors](docs/architecture/agent-monitors.md) for config details.

## Running Locally

Runtime examples live under `deploy/examples/`. Use them for local smoke tests or as a starting
point for your own Compose file.

Run the bundled Core and Console example from this repository:

```sh
cd deploy/examples
docker compose -f ./core-console-compose up -d
curl http://localhost:8999/health
curl http://localhost:8999/v1/diagnostics/core-worker
```

Run the Console dev server against local Core:

```sh
cd apps/console
npm install
npm run dev
```

Set `VITE_API_BASE_URL=http://localhost:8999/v1` in `apps/console/.env`.

Seed local demo data:

```sh
make seed-demo-data
```

This writes to `apps/core/data/orion.db`.

## Development

Run tests and builds:

```sh
cd apps/core && go test ./...
make core-worker-build
cd apps/agent && go test ./...
cd apps/console && npm run build
```

Common maintainer commands:

```sh
make generate-openapi
make generate-sdk
make agent-build VERSION=0.1.2
```

OpenAPI is generated from Core route annotations. Do not edit `apps/core/openapi.yaml` or the
generated Console SDK by hand.

## Documentation

- [System design](docs/system-design.md)
- [Architecture overview](docs/architecture/system-overview.md)
- [Core features](docs/architecture/core-features.md)
- [Data ingestion](docs/architecture/data-ingestion.md)
- [Persistence and lifecycle](docs/architecture/persistence-and-lifecycle.md)
- [Incident reconciliation](docs/architecture/incident-reconciliation-flow.md)
- [Deployment guide](docs/deployment/README.md)
- [Core Docker deployment](docs/deployment/core-docker.md)
- [Agent install and upgrade](docs/deployment/agent-install-upgrade.md)
- [Seed demo data](docs/development/seed-demo-data.md)
- [Milestones](docs/milestones/README.md)

## Project Layout

```txt
orion/
├── apps/
│   ├── agent/    # Go daemon and CLI
│   ├── core/     # Go API server, SQLite, OpenAPI, embedded Console
│   └── console/  # React/Vite UI source
├── deploy/       # Docker Compose, systemd, launchd, install scripts
├── docs/         # architecture, deployment, development, milestones
├── packages/     # shared/generated package space
└── Makefile
```
