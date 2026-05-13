# Core Docker Deployment

The Core Docker image is the default deployable unit for a self-hosted Orion Core.

It builds:

- the Console with Vite;
- the Core Go binary;
- the Console static files embedded into the Core binary.

The final runtime image contains one process: `orion-core`. It serves both the backend API and the Console UI.

## Build

```sh
make docker-build
```

Equivalent command:

```sh
docker build -f apps/core/Dockerfile -t orion-core:latest .
```

## Run With Docker Compose

Create an environment file or export the values in your shell:

```sh
export ORION_ADMIN_USERNAME=admin
export ORION_ADMIN_PASSWORD='change-me'
export ORION_JWT_SECRET='change-me-to-a-long-random-value'
```

Start Core:

```sh
make docker-up
```

Core listens on `http://localhost:8999`.

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

## Published Versions

The Dockerfile is ready for versioned image publishing later. The expected image shape is:

```txt
orion-core:<version>
```

Agents should be installed separately on each monitored machine. They do not run inside this Core container.
