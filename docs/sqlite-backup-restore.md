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
docker compose -f deploy/docker-compose.yml exec core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
docker cp "$(docker compose -f deploy/docker-compose.yml ps -q core):/data/orion-backup.db" ./orion-backup.db
```

## Restore

Stop Core before replacing the active database:

```sh
make docker-down
cp ./orion-backup.db "$ORION_DATA_DIR/orion.db"
make docker-up
```

For a local binary run, stop the process first, replace `orion.db`, then start Core again.

## Verify

After restore, check Core and database health:

```sh
curl http://localhost:8999/health
```
