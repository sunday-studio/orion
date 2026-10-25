---
title: Backups and restore
description: "Back up the SQLite database, restore it and keep disk use in check."
group: Guides
order: 7
---
Core stores its data in SQLite at `ORION_DATA_DIR/orion.db`, and product logs in `logs.db` beside it. Both run in WAL mode.

## Back up

Always use SQLite's online backup. Do not copy `orion.db` alone, because recent writes may live in the `-wal` file.

```sh
sqlite3 "$ORION_DATA_DIR/orion.db" ".backup '$ORION_DATA_DIR/orion-$(date +%Y%m%d-%H%M%S).db'"
```

With Docker Compose:

```sh
docker compose exec orion-core sqlite3 /data/orion.db ".backup '/data/orion-backup.db'"
```

## Restore

Stop Core and the worker, remove stale WAL files so old frames are not replayed, copy the backup in and start again.

```sh
rm -f "$ORION_DATA_DIR/orion.db-wal" "$ORION_DATA_DIR/orion.db-shm"
cp ./orion-backup.db "$ORION_DATA_DIR/orion.db"
```

## Before upgrades

Before applying migrations to an existing database, Core writes a consistent copy to `$ORION_DATA_DIR/backups/orion-pre-migrate-<version>-<timestamp>.db` and keeps the newest three. To roll back a bad upgrade, stop Core and the worker, restore the newest copy and run the previous image.

## Disk space

Log retention defaults to 14 days per app and is pruned hourly. To reclaim space after shrinking retention:

```sh
sqlite3 "$ORION_DATA_DIR/orion.db" "PRAGMA auto_vacuum=INCREMENTAL; VACUUM;"
```
