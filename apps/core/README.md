# Orion Core

Core is the Go API server for Orion. It receives Server reports, stores runtime data in SQLite, computes health and incidents, serves alert/settings/event APIs, exposes Core worker diagnostics, and serves the built Console assets in production.

## Run Locally

```sh
go run .
```

Core listens on `:8999` by default and stores data under `data/orion.db`.

Run the Core monitor worker in a separate terminal when developing Core-managed monitors:

```sh
go run ./cmd/worker
```

The worker opens and migrates the same Core database, records heartbeat state, logs periodic database health, and exits on `SIGINT` or `SIGTERM`. API health stays available at `/health`; worker state is exposed separately at `/v1/diagnostics/core-worker`.

Useful environment variables:

- `ORION_DATA_DIR`: SQLite data directory.
- `ORION_PORT`: HTTP listen port.
- `ORION_CORS_ORIGINS`: comma-separated browser origins for development.
- `ORION_WORKER_ID`: Core monitor worker diagnostics identity.
- `ORION_WORKER_HEARTBEAT_SECONDS`: worker heartbeat interval.
- `ORION_WORKER_STALE_SECONDS`: stale threshold used by diagnostics.
- `ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS`: set to `true` only for trusted installs that need
  Core-managed monitors to probe private or loopback addresses; link-local, multicast, and cloud
  metadata targets remain blocked.
- `ORION_PLAYWRIGHT_RUNNER`: optional executable path for Playwright transaction monitors. Leave
  unset on workers without a browser runtime; those monitors report `runtime_unavailable` explicitly.
- `ORION_DATA_LIFECYCLE_SCHEDULER_SECONDS`: API-side data lifecycle archive/rollup scheduler interval.
- `ORION_ADMIN_USERNAME`, `ORION_ADMIN_PASSWORD`, `ORION_JWT_SECRET`: enable Console login.
- `ORION_REQUIRE_FRONTEND_AUTH`: set to `true` for any exposed deployment so Core refuses to
  start unless all Console admin auth values are configured.

See [Core Docker deployment](../../docs/deployment/core-docker.md) for deploy usage.

## API Contract

OpenAPI is generated from Core route annotations:

```sh
make generate-openapi
```

Do not edit `openapi.yaml` or generated Swagger files by hand.

Current behavior is documented in:

- [Server/Core contract](../../docs/agent-core-contract.md)
- [Core features](../../docs/architecture/core-features.md)
- [Data ingestion](../../docs/architecture/data-ingestion.md)
- [Persistence and lifecycle](../../docs/architecture/persistence-and-lifecycle.md)

## Tests

```sh
go test ./...
```
