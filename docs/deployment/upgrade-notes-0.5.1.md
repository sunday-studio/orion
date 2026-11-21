# Upgrade notes: 0.5.1

0.5.1 makes the Apps and Logs pages of the Console fast on a large log table. Nothing is removed and the API
is unchanged. The Node has no change; it is rebuilt and released with the same version.

## Core

```sh
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:0.5.1 docker compose up -d
```

- Take a backup first ([SQLite backup and restore](../sqlite-backup-restore.md)), as for any upgrade. There is
  no migration of `orion.db`. Core adds one small table, `log_app_volume`, to `logs.db` on start. A build that
  does not know the table ignores it, so going back to 0.5.0 is safe.
- **What was slow.** The Apps list, an app's page and the log ingest status counted every stored log line on
  each load, and the Logs filter counts, trace lookups and related logs scanned far more of the table than they
  needed. On millions of lines that took tens of seconds, and on very large tables the server cut the request
  off. These are fixed; measured on 15 million lines, the Apps list went from 252 s to about 1 ms and related
  logs from 38 s to under 1 ms.
- **First start after the upgrade.** Core scans the log table once in the background to count lines and bytes
  per app. Until that scan ends, which takes seconds to a few minutes depending on the size of `logs.db`, the
  Apps pages show an estimate of lines and storage. Core saves the result in `logs.db`, so later restarts answer
  at once. The figures follow new logs as they arrive and are rescanned when logs are deleted or the snapshot
  is old (15 seconds to 10 minutes between scans, longer for a large table), so they can lag by a few seconds.
  [Product logs](../product-logs.md) describes this.
- Views over a long range (7 days and up) still scale with the number of lines in the range. On a table of
  tens of millions of lines the 7 day filter counts and histogram can run into the 10 second query limit; use a
  shorter range there.

## Nodes

No change is needed. A Node on 0.5.0 keeps working with this Core.
