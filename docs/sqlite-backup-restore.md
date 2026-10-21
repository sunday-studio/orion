# SQLite Backup And Restore

Orion Core stores its SQLite database at `ORION_DATA_DIR/orion.db`.
The default data directory is `apps/core/data` for local runs and `/data` in Docker.

## Backup

Prefer SQLite's online backup command so the database can be copied while Core is running:

```sh
sqlite3 "$ORION_DATA_DIR/orion.db" ".backup '$ORION_DATA_DIR/orion-$(date +%Y%m%d-%H%M%S).db'"
```

For Docker Compose:

```sh
docker compose -f deploy/docker-compose.yml exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker cp "$(docker compose -f deploy/docker-compose.yml ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
```

## Restore

Stop Core before replacing the active database:

```sh
make docker-down
rm -f "$ORION_DATA_DIR/orion.db-wal" "$ORION_DATA_DIR/orion.db-shm"
cp ./orion-backup.db "$ORION_DATA_DIR/orion.db"
make docker-up
```

For a local binary run, stop the process first, replace `orion.db`, then start Core again.

Core runs SQLite in WAL mode, so `orion.db-wal` and `orion.db-shm` sit beside the database. Always back up with `sqlite3 ... ".backup"` (above), never by copying `orion.db` alone, and remove stale `-wal`/`-shm` files before restoring so old log frames are not replayed onto the restored database. The API and Worker must share one host and data volume.

## Logging

`ORION_DB_LOG_LEVEL` (`silent`, `error`, `warn`, `info`; default `warn`) controls SQL logging. Bound values are never logged at any level; `warn` logs only slow (>200ms) queries and errors.

## Verify

After restore, check Core and database health:

```sh
curl http://localhost:8999/health
```

## Retention and disk space

Core prunes the tables that otherwise grow forever, hourly, in 1000-row batches. Windows live in the data lifecycle settings (days): `service_log_retention_days` (default 14), `alert_delivery_retention_days` (90, deliveries and attempts; pending deliveries are never pruned), `incident_event_retention_days` (365, resolved incidents only), `audit_event_retention_days` (365) and `worker_status_retention_days` (7). Raw monitor and node reports are archived by `raw_report_hot_days` as before, now in batches so a large backlog cannot get stuck.

Pruning frees pages inside the database file. New databases are created with `auto_vacuum=INCREMENTAL`, so retention also returns space to the OS. An existing database created before this change reports `sqlite.auto_vacuum: "none"` in `GET /v1/diagnostics/core`; convert it once during a maintenance window (stop Core and the Worker first, and keep free disk space equal to the database size):

```sh
sqlite3 "$ORION_DATA_DIR/orion.db" "PRAGMA auto_vacuum=INCREMENTAL; VACUUM;"
```

## Migrations

Core applies embedded migrations at start. Each runs under an exclusive write lock and re-checks that it is still pending, so the API and the Worker starting together migrate once. Every applied migration stores a SHA-256 of its file; if an applied file is edited, Core refuses to start with "modified after it was applied" - restore the original and add a new migration instead. Before applying migrations to an existing database Core writes a consistent copy to `$ORION_DATA_DIR/backups/orion-pre-migrate-<version>-<timestamp>.db` (mode `0600`, newest three kept). To roll back after a bad upgrade, stop Core and the Worker, then restore the newest backup as described under Restore and run the previous image.

The Worker container healthcheck (`orion-core-worker healthcheck`) is read-only: it opens the database read-only and compares the schema version, so it never migrates or takes a write lock.
