# SQLite Backup And Restore

Orion Core keeps its data in two SQLite databases in `ORION_DATA_DIR` (default `apps/core/data` for local
runs, `/data` in Docker):

| File | Holds |
|---|---|
| `orion.db` | Everything except product logs: servers, monitors, reports, incidents, alerts, status pages, settings, users. |
| `logs.db` | Product logs sent by your apps (the Logs section). |

Back up both. Each runs in WAL mode, so each has `-wal` and `-shm` files beside it. Never copy the `.db` file
alone; use SQLite's online backup, which is safe while Core is running.

## Backup

```sh
sqlite3 "$ORION_DATA_DIR/orion.db" ".backup '$ORION_DATA_DIR/orion-backup.db'"
sqlite3 "$ORION_DATA_DIR/logs.db" ".backup '$ORION_DATA_DIR/logs-backup.db'"
```

For Docker Compose (use `-f deploy/docker-compose.yml` instead if you run from a checkout of this repository):

```sh
docker compose -f orion-compose.yaml exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker compose -f orion-compose.yaml exec orion-core sqlite3 /data/logs.db ".backup '/data/logs-backup.db'"
docker cp "$(docker compose -f orion-compose.yaml ps -q orion-core):/data/orion-backup.db" ./orion-backup.db
docker cp "$(docker compose -f orion-compose.yaml ps -q orion-core):/data/logs-backup.db" ./logs-backup.db
```

Store the copies somewhere outside the Docker volume, and delete `orion-backup.db` and `logs-backup.db` from
`/data` afterwards so they do not fill the volume. You can skip `logs.db` if you do not mind losing product
logs; everything else is in `orion.db`.

Before an upgrade, take this backup first. Core also copies `orion.db` itself (not `logs.db`) before applying
migrations; see [Migrations](#migrations).

## Restore

Stop Core and the Worker before replacing a database:

```sh
docker compose -f orion-compose.yaml down
```

Put the backup files in place inside the volume. With Docker, a throwaway container does it (the volume is
named `<project>_orion-data`; `docker volume ls` shows it):

```sh
docker run --rm -v <project>_orion-data:/data -v "$PWD":/backup alpine sh -c '
  rm -f /data/orion.db-wal /data/orion.db-shm /data/logs.db-wal /data/logs.db-shm &&
  cp /backup/orion-backup.db /data/orion.db &&
  cp /backup/logs-backup.db /data/logs.db &&
  chown 10001:10001 /data/orion.db /data/logs.db'
docker compose -f orion-compose.yaml up -d
```

Omit the `logs.db` lines if you only restore `orion.db`. For a local binary run, stop the process, remove the
`-wal` and `-shm` files of the database you replace, copy the backup over it, and start Core again.

Remove stale `-wal` and `-shm` files before restoring so old log frames are not replayed onto the restored
database. The API and Worker must share one host and data volume.

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

Core applies embedded migrations at start. Each runs under an exclusive write lock and re-checks that it is still pending, so the API and the Worker starting together migrate once. Every applied migration stores a SHA-256 of its file; if an applied file is edited, Core refuses to start with "modified after it was applied" - restore the original and add a new migration instead. Before applying migrations to an existing database Core writes a consistent copy of `orion.db` to `$ORION_DATA_DIR/backups/orion-pre-migrate-<version>-<timestamp>-<pid>.db` (mode `0600`, newest three kept; `<version>` is the first migration about to run, for example `000055`). To roll back after a bad upgrade, stop Core and the Worker, then restore the newest backup as described under Restore and run the previous image.

The Worker container healthcheck (`orion-core-worker healthcheck`) is read-only: it opens the database read-only and compares the schema version, so it never migrates or takes a write lock.
