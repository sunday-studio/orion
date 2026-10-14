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
