# Core Docker Deployment

The Core Docker image is the default deployable unit for a self-hosted Orion Core.

It builds:

- the Console with Vite;
- the Core Go binary;
- the Core monitor worker Go binary;
- the Console static files copied into the Core runtime image.

The final runtime image contains two binaries: `orion-core` for the API/Console process and
`orion-core-worker` for Core-managed monitor execution. Run them as separate containers that share
the same `/data` volume.

## Image

Core API, Core monitor worker, and Console are shipped together as one Docker image:

```txt
ghcr.io/sunday-studio/orion-core:<version>
```

The default command runs `orion-core`, which serves the backend API and Console UI. Override the
command with `./orion-core-worker` to run the monitor worker.

## Run With Docker Compose

Download the sample Compose file:

```sh
curl -fsSL -o orion-compose.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/examples/core-console-compose.yaml
```

Optionally pin a release image and set stronger admin credentials in the same directory:

```sh
cat > .env <<'EOF'
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:<version>
ORION_HTTP_PORT=8999
ORION_ADMIN_USERNAME=admin
ORION_ADMIN_PASSWORD=replace-with-a-strong-password
ORION_JWT_SECRET=replace-with-a-long-random-secret
EOF
```

Public status page subscriber email is disabled by default. To send confirmation and public incident
update mail, configure a dedicated sender for public subscribers:

```sh
ORION_PUBLIC_STATUS_MAIL_ENABLED=true
ORION_PUBLIC_STATUS_MAIL_HOST=smtp.example.com
ORION_PUBLIC_STATUS_MAIL_PORT=587
ORION_PUBLIC_STATUS_MAIL_USERNAME=status-sender
ORION_PUBLIC_STATUS_MAIL_PASSWORD=replace-with-smtp-password
ORION_PUBLIC_STATUS_MAIL_FROM_EMAIL=status@example.com
ORION_PUBLIC_STATUS_MAIL_FROM_NAME="Example Status"
ORION_PUBLIC_STATUS_MAIL_REPLY_TO=support@example.com
ORION_PUBLIC_STATUS_URL_ORIGIN=https://status.example.com
ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET=replace-with-a-long-random-secret
```

Start Core. If you skip the `.env` file, Compose uses the defaults in `orion-compose.yaml`.
Compose starts two services:

- `orion-core`: API, Console, incidents, alerts, and diagnostics;
- `orion-core-worker`: Core-managed monitor worker heartbeat and check execution.

```sh
docker compose -f orion-compose.yaml up -d
```

Core listens on `http://localhost:8999`.
Worker state is exposed through the API diagnostics route:

```sh
curl http://localhost:8999/v1/diagnostics/core-worker
```

The plain `/health` endpoint only reports API and database availability. It does not fail because
the worker is paused, stale, or stopped.

## Core Monitor Target Policy

Core-managed monitors reject localhost, loopback, private RFC1918, link-local, multicast,
unspecified, and known cloud metadata targets by default. Private and loopback targets are allowed
only when the worker is running in a trusted internal deployment and
`ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS=true` is set explicitly. Link-local, multicast,
unspecified, and cloud metadata targets stay blocked. Redirects are checked with the same policy
before the worker follows them, and report payloads strip URL user info, query strings, and fragments
from recorded target URLs.

## Playwright Transaction Runtime

Playwright transaction monitors use an explicit runner executable on the Core worker host. The
default `ghcr.io/sunday-studio/orion-core` image intentionally does not bundle Node, Playwright, or
browser binaries. That keeps the standard Core image small and avoids silently adding a browser
sandbox to every self-hosted install.

Supported first-release model:

- keep the default Core API and Core worker image browser-free;
- install or mount a trusted Node/Playwright runner executable on worker hosts that need browser
  transaction checks;
- set `ORION_PLAYWRIGHT_RUNNER` on the `orion-core-worker` process to the executable path;
- leave `ORION_PLAYWRIGHT_RUNNER` unset on workers that should not run browser checks.

The runner contract is stdin/stdout JSON: Core passes the validated transaction request on stdin and
expects a bounded JSON result on stdout. If `ORION_PLAYWRIGHT_RUNNER` is unset, Playwright monitors
fail explicitly with `failure_stage: runtime_unavailable`; other Core monitor types keep running.

Docker users can either build a derived worker image that includes the runner and browsers, or mount
the runner into the worker container and point the environment variable at it:

```yaml
services:
  orion-core-worker:
    environment:
      ORION_PLAYWRIGHT_RUNNER: /opt/orion/playwright-runner
    volumes:
      - ./playwright-runner:/opt/orion/playwright-runner:ro
```

An official optional Playwright worker image or sidecar is intentionally deferred until the browser
sandbox, browser set, and artifact retention policy are versioned as a separate runtime contract.

From this repository, you can run the example directly:

```sh
cd deploy/examples
docker compose -f ./core-console-compose up -d
```

When Core serves the bundled Console, browser API calls stay on the same origin and do not need
CORS. Set `ORION_CORS_ORIGINS` only when a separately hosted Console or custom browser origin calls
this Core API:

```sh
ORION_CORS_ORIGINS=https://console.example.com,https://orion-core.examples.orb.local
```

## Run With Docker

```sh
docker run -d \
  --name orion-core \
  --restart unless-stopped \
  -p 8999:8999 \
  -v orion-data:/data \
  -e ORION_DATA_DIR=/data \
  -e ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS=3600 \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD='change-me' \
  -e ORION_JWT_SECRET='change-me-to-a-long-random-value' \
  ghcr.io/sunday-studio/orion-core:<version>

docker run -d \
  --name orion-core-worker \
  --restart unless-stopped \
  -v orion-data:/data \
  -e ORION_DATA_DIR=/data \
  -e ORION_WORKER_ID=core-monitor-worker \
  -e ORION_WORKER_HEARTBEAT_SECONDS=15 \
  -e ORION_WORKER_STALE_SECONDS=60 \
  -e ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS=false \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD='change-me' \
  -e ORION_JWT_SECRET='change-me-to-a-long-random-value' \
  ghcr.io/sunday-studio/orion-core:<version> \
  ./orion-core-worker
```

## Runtime Example

The copyable sample Compose file lives at `deploy/examples/core-console-compose.yaml`.

Start or update Core:

```sh
docker compose -f orion-compose.yaml pull
docker compose -f orion-compose.yaml up -d
```

To inspect the resolved Compose file without starting Core:

```sh
docker compose -f orion-compose.yaml config
```

Stop Core:

```sh
docker compose -f orion-compose.yaml down
```

## Data

Both Core containers store Core data at `/data`, mounted by Docker Compose as the `orion-data`
volume. The first worker release uses shared SQLite access on a single Docker host. Do not point
multiple hosts at the same SQLite file.

This includes:

- `orion.db`;
- archive SQLite files;
- lifecycle metadata.
- Core worker heartbeat diagnostics.

Backups should include the Docker volume. See [SQLite backup and restore](../sqlite-backup-restore.md).

## Server Connection URL

Servers need a stable `core_url` that points at this Core deployment.

Common examples:

- `http://orion-core.local:8999`;
- `http://192.168.x.y:8999`;
- `http://100.x.y.z:8999` for Tailscale;
- `https://orion.example.com` if placed behind a reverse proxy.

## Publishing

Image publishing is manually triggered from GitHub Actions. Run the `Docker Images` workflow and
provide the version tag to publish.

Servers should be installed separately on each monitored machine. They do not run inside the Core API
or Core worker containers.
