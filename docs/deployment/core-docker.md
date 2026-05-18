# Core Docker Deployment

The Core Docker image is the default deployable unit for a self-hosted Orion Core.

It builds:

- the Console with Vite;
- the Core Go binary;
- the Console static files copied into the Core runtime image.

The final runtime image contains one process: `orion-core`. It serves both the backend API and the Console UI from the runtime `web/` directory.

## Image

Core and Console are shipped together as one Docker image:

```txt
ghcr.io/sunday-studio/orion-core:<version>
```

The image contains one runtime process, `orion-core`, which serves both the backend API and the
Console UI.

## Run With Docker Compose

Download the sample Compose file:

```sh
curl -fsSL -o orion-compose.yaml \
  https://raw.githubusercontent.com/sunday-studio/orion/main/deploy/examples/core-console-compose.yaml
```

Edit these values before starting Core:

- `ORION_ADMIN_USERNAME`
- `ORION_ADMIN_PASSWORD`
- `ORION_JWT_SECRET`

Start Core:

```sh
docker compose -f orion-compose.yaml up -d
```

Core listens on `http://localhost:8999`.

## Run With Docker

```sh
docker run -d \
  --name orion-core \
  --restart unless-stopped \
  -p 8999:8999 \
  -v orion-data:/data \
  -e ORION_DATA_DIR=/data \
  -e ORION_ADMIN_USERNAME=admin \
  -e ORION_ADMIN_PASSWORD='change-me' \
  -e ORION_JWT_SECRET='change-me-to-a-long-random-value' \
  ghcr.io/sunday-studio/orion-core:<version>
```

## Runtime Example

The copyable sample Compose file lives at `deploy/examples/core-console-compose.yaml`.

Check it before running:

```sh
docker compose -f orion-compose.yaml config
```

Start or update Core:

```sh
docker compose -f orion-compose.yaml pull
docker compose -f orion-compose.yaml up -d
```
Stop Core:

```sh
docker compose -f orion-compose.yaml down
```

## Data

The container stores Core data at `/data`, mounted by Docker Compose as the `orion-data` volume.

This includes:

- `orion.db`;
- archive SQLite files;
- lifecycle metadata.

Backups should include the Docker volume. See [SQLite backup and restore](../sqlite-backup-restore.md).

## Agent Connection URL

Agents need a stable `core_url` that points at this Core deployment.

Common examples:

- `http://orion-core.local:8999`;
- `http://192.168.x.y:8999`;
- `http://100.x.y.z:8999` for Tailscale;
- `https://orion.example.com` if placed behind a reverse proxy.

## Publishing

Image publishing is manually triggered from GitHub Actions. Run the `Docker Images` workflow and
provide the version tag to publish.

Agents should be installed separately on each monitored machine. They do not run inside this Core container.
