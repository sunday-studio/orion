# Database Evaluation: Is SQLite Still the Right Choice?

Status: proposed, awaiting review. 2026-11-07.
Related: [decision-log.md](decision-log.md) (D-001, D-011, D-025, D-053), [service-correlation-plan.md](service-correlation-plan.md), [product-logs-plan.md](product-logs-plan.md), [backend-hardening-load-results.md](backend-hardening-load-results.md), [backend-hardening-e2e-plan.md](backend-hardening-e2e-plan.md), [../issue-log.md](../issue-log.md), [../sqlite-backup-restore.md](../sqlite-backup-restore.md), [../architecture/system-overview.md](../architecture/system-overview.md).

How to read the numbers. Every figure is tagged:

- **[repo]** measured by the team and recorded in this repository (file cited).
- **[bench]** measured for this evaluation, 2026-11-07, on one Apple-silicon laptop, warm page cache, synthetic data, in a scratch directory. Nothing in the repo was modified. These are order-of-magnitude checks, not a staging result.
- **[ext]** from an external source (URL in Sources).
- **[est]** my estimate or arithmetic from the above. Not a measurement.

## Summary

1. **Verdict: keep SQLite as the engine for the one-container product.** The stated goal (self-hosted, single install, no Postgres or Prometheus) and every measured workload so far fit it. No part of the current product needs a second engine.
2. **D-001 was an assertion, and the evidence since supports it only for the load tested.** 33 reports/s and 2,000 log lines/s for 30 minutes on a laptop passed with zero lock errors [repo]. The 72h soak, a slow-disk or low-RAM run, a full-window prune under load and any test above 3.6M log rows have not been done.
3. **The wall is not write concurrency. It is data volume and scan cost.** Default log retention (14 days, no size cap) means 100 lines/s retains about 91 GB [est]. The histogram and `LIKE` search scan every row in the window, so they grow linearly (about 3 s at 3.6M rows [repo]).
4. **The service-correlation plan should not put sample history in `orion.db`.** 200 series per node for 7 days is about 200 MB per node [bench]; 50 nodes is about 10 GB, 500 nodes about 100 GB [est]. Migration backups copy `orion.db` up to 3 times. Use a separate `metrics.db` behind an interface from day one, and use a narrow table for OTLP.
5. **Re-open the question on measurable triggers (see Recommendation):** a host with logs.db above 50 GB, histogram p95 above 3 s on the default window, sustained ingest above 5,000 lines/s, metrics.db above 20 GB, or a real HA / multi-replica requirement. Until then the cheap steps keep the door open: storage interfaces, a size cap, FTS5, a minute rollup.

## What we store today

### Files and ownership

| File | Holds | Written by | Source |
|---|---|---|---|
| `orion.db` | Nodes, monitors, `monitor_reports`, `node_reports`, incidents, alerts, status pages, settings, users, `service_log_entries` (Node's own diagnostics), `node_app_resources` (latest row only, no history), uptime rollups | API process and Worker process (two processes, one file) | `docs/sqlite-backup-restore.md`, `apps/core/internal/db/models.go` |
| `logs.db` | `log_entries` (product logs), 7 indexes, no FTS table | One in-process writer goroutine in the API; the Worker opens it read-only for log-alert rules | `apps/core/internal/logstore/store.go` |
| `raw-reports-YYYY-MM.sqlite` | Archived old `node_reports` and `monitor_reports` | Archive job | `docs/architecture/persistence-and-lifecycle.md` |
| Node `state.db` | Node-local state and report spool | Node daemon | `apps/node` |

Core is two processes (`orion-core`, `orion-core-worker`) in two containers sharing one `/data` volume (`deploy/docker-compose.yml`). So "one container" is already two, and WAL requires both on the same host with a local volume [ext: sqlite.org/wal.html].

### Write rates and sizes (measured)

| Fact | Value | Source |
|---|---|---|
| Report ingest, load profile | 50 nodes x 20 monitors at 30s = about 33 reports/s; 6,648 sent, 0 failed; p50/p95/p99 0.77 / 3.5 / 6.3 ms; 0 `SQLITE_BUSY` | [repo] `backend-hardening-load-results.md` |
| Reconnect burst | 1,000 simultaneous reports drained in about 300 ms, about 3,300 req/s on one file | [repo] same |
| One report | about 0.13 ms with 1,000 monitors | [repo] same |
| Log ingest, cross-process | 2,000 lines/s for 30 min = 3.6M rows, accepted = stored, p95 ingest 2.1 to 2.4 ms, 0 lock errors, integrity ok | [repo] same |
| Log row size | `logs.db` 2.7 GB for 3.6M rows = about 750 B/row (7 indexes); WAL under 30 MB; API RSS 88 MB | [repo] same |
| Synthetic log row size | 855 MiB for 1.0M rows = about 900 B/row (random 32-char ids, similar shape) | [bench] |
| Bulk insert ceiling | 1M rows in one transaction in 10 s (about 100k rows/s) | [bench] |
| Small-entry store throughput | 20,000 entries/s for 10 s, written = 20,000, dropped = 0 (`TestSustainedIngestThroughputAndLatency`) | [bench] using repo test |
| Uptime rollups | Overview 2,273 ms to 25 ms at 864k rows; 14,175 ms to 85 ms at 5.18M rows | [repo] D-053, load results |
| Histogram | about 2.95 s p95 at 3.6M rows in the window, while ingesting 2,000/s with 5 live tails | [repo] D-025, load results |
| Disk-full | Both DBs degrade loudly, no corruption, recover when space returns | [repo] chaos scenario 5 |
| Growth driver in `orion.db` | About 87% of `service_log_entries` rows are routine per-check INFO lines | [repo] issue I-005 |

### Read patterns

- Reports: point writes, then reads by monitor/node/time with indexes. Cheap (monitor list 4 to 6 ms at 5M rows [repo]).
- Overview, node and monitor uptime: read from daily rollups since D-053 (30 to 90 ms).
- Logs list: `(app_id, ts DESC)` index, milliseconds (4.4 ms p95 [repo]; 100 rows from 1M in under 1 ms [bench]).
- Logs search: `body LIKE '%x%' OR event LIKE ... OR attributes LIKE ...` (`logstore/query.go:118`). Full scan of the matching window. **No FTS table exists.** The product-logs plan specifies FTS5, but the code has none and the Docker build has no `sqlite_fts5` build tag (`apps/core/Dockerfile` builds with plain `go build`; the driver needs the tag [ext: mattn README]).
- Logs histogram, facets, field values, patterns: `GROUP BY` over every row in the window (`logstore/analytics.go`, `facets.go`).
- Retention: hourly, per app, `DELETE ... LIMIT 1000` batches, then `incremental_vacuum(2000)` and `wal_checkpoint(TRUNCATE)` (`logstore/prune.go`, `retention.go`). Default 14 days per app; **per-app size cap defaults to 0 (off)** (`db/app-models.go:12-13`).

## Where SQLite fits

- **Deployment goal.** `docs/architecture/system-overview.md` states the aim: no Prometheus, Postgres or Kubernetes. SQLite is a library, so install is `docker compose up` with one volume. SQLite's own guidance puts "embedded devices", single-host application storage and "no administration" in its good-fit list [ext: sqlite.org/whentouse.html].
- **The current workload is small.** 33 reports/s is 4% of the measured 3,300 req/s burst capacity [repo]. A report costs 0.13 ms of writer time, so 330 reports/s would be roughly 4% of writer time [est].
- **Single writer is mostly solved.** WAL, `busy_timeout=5000`, `synchronous=NORMAL`, `_txlock=immediate`, pool of 8, logs behind an async queue with batched transactions (`db/sqlite-profile.go`, `logstore/store.go`). Log writes go to their own file, so they do not compete with report ingest for one lock. Zero `SQLITE_BUSY` across 7.2M accepted rows in the measured runs [repo].
- **Reads do not block writes.** WAL lets readers and the writer run together [ext: sqlite.org/wal.html]. The measured list query at 4.4 ms during 2,000 lines/s ingest agrees [repo].
- **Operations.** Backup is `sqlite3 .backup` (the image ships `sqlite3`), Core takes its own `VACUUM INTO` copy before migrations with checksum tamper detection (`db/migration-backup.go`), disk-full behaviour is tested. These are real advantages over running a database server.
- **Size ceiling is not the issue.** SQLite caps at 281 TB per file [ext: sqlite.org/whentouse.html]. The practical limits are the disk, backup time and scan cost, covered below.
- **Node side.** The Node uses a pure-Go SQLite (`glebarez/sqlite`, built with `CGO_ENABLED=0`) for `state.db`, so it cross-compiles to Linux and macOS, amd64/arm64/arm, with no toolchain. No reason to change it.

## Where it strains

### 1. Data volume, not engine throughput (logs)

At 750 B/row [repo], default 14-day retention and no size cap:

| Lines/s | Rows/day | GB/day | Retained at 14 days | Retained at 3 days |
|---|---|---|---|---|
| 10 | 0.86M | 0.65 | 9 GB | 2 GB |
| 100 | 8.6M | 6.5 | 91 GB | 19 GB |
| 1,000 | 86M | 65 | 907 GB | 194 GB |
| 5,000 | 432M | 324 | 4.5 TB | 972 GB |

[est] arithmetic. The load test's 2,000 lines/s is a burst test, not a retained rate: it would be 130 GB/day. A 40 GB VPS fills at about 60 lines/s with defaults. **This is a defaults problem first.** A global size cap and a free-disk watermark would turn silent disk exhaustion into controlled pruning (the disk-full path already protects the DB, but the product stops ingesting).

### 2. Scan-based reads grow linearly

Per-row cost of the histogram: 0.82 microseconds under load [repo: 2.95 s / 3.6M] and 0.33 microseconds idle and warm [bench: 1M rows in 0.33 s]. `LIKE` substring scan: about 0.45 microseconds/row warm [bench: 0.45 s per 1M rows, rare term, all apps]; 1.7 s per 1M rows with a cold cache [bench, one observation].

| Window | Rows in window | Histogram (0.33 to 0.82 us/row) | `LIKE` search (warm, about 0.45 us/row) |
|---|---|---|---|
| 1 h at 100/s | 0.36M | 0.1 to 0.3 s | 0.16 s |
| 1 h at 1,000/s | 3.6M | 1.2 to 3 s | 1.6 s |
| 1 h at 5,000/s | 18M | 6 to 15 s | 8 s |
| 24 h at 100/s | 8.6M | 3 to 7 s | 4 s |
| 24 h at 1,000/s | 86M | 28 to 71 s | 39 s |

[est]. D-025's 1-hour default holds up to about 1,000 lines/s. Beyond that, a pre-aggregated minute table is needed (D-025 already anticipates it), or windows must be capped. The Logs plan's target "list under 300 ms on 10M rows" has not been measured at 10M; the largest repo run is 3.6M.

The service-correlation timeline (error-log counts per minute around an incident, 15 min before to 5 min after) is the same histogram query. At 1,000/s it scans 1.2M rows, about 0.4 to 1 s [est]. A rollup fixes both.

### 3. Full-text search is missing, and FTS5 costs disk

Measured on 1M synthetic rows [bench]:

| Index | Build (bulk) | logs.db size | Query |
|---|---|---|---|
| none (today, `LIKE`) | n/a | 855 MiB | 0.14 to 0.45 s |
| FTS5 `unicode61` on body + attributes | 4.2 s | 1,058 MiB (+24%) | 3 ms |
| FTS5 `trigram` (substring parity with `LIKE`) | 29.5 s | 1,991 MiB (+133%) | 5 to 20 ms |

`unicode61` changes meaning from "substring" to "whole token" (`user=976` becomes the phrase `user 976`). Trigram keeps substring semantics and supports indexed `LIKE` [ext: sqlite.org/fts5.html] but more than doubles the file. Bulk build speed says nothing about online insert cost under 2,000 lines/s; that is untested. Also, FTS5 needs `-tags sqlite_fts5` in both `go build` lines of the Dockerfile and in `make`/CI [ext: mattn README].

### 4. Deletes cost almost as much as inserts

Pruning 100,000 rows in 100 batches of 1,000 took 7.4 s [bench], about 13,500 rows/s, 74 ms of writer lock per batch (CLI, `synchronous=FULL`, so the app with `NORMAL` should be somewhat faster). At 2,000 lines/s, steady-state pruning is about 15% of the writer's time [est]. Under the plan's rule that prune "never blocks ingest for more than 1 s" this is fine, but **retention was never exercised over a full window under load** ([repo] load results, "Gaps to close"). With FTS5 added, each delete also updates the FTS index (cost unmeasured).

`PRAGMA incremental_vacuum(2000)` returns at most 8 MB per run; the file mostly plateaus at its peak size rather than shrinking, which is acceptable if the disk is sized for the peak.

### 5. Backup, restore, upgrades scale with file size

- `backupBeforeMigration` runs `VACUUM INTO` on `orion.db` and keeps the 3 newest (`keepMigrationBackups = 3`). Disk needed is up to 4 times `orion.db`. Today `orion.db` is modest because logs live elsewhere. **Putting 7-day samples for 50 nodes (about 10 GB [est]) in `orion.db` would make every upgrade write 10 GB and keep 30 GB of backups.**
- Restore is a file copy plus removing `-wal`/`-shm` (documented). Simple, but a 90 GB `logs.db` copy is hours on a small VPS [est].
- Two files are backed up separately, so they are not a consistent pair [est]: apps in `orion.db` and their entries in `logs.db` can disagree after a restore. A third file (`metrics.db`) makes this worse unless one `orion-core backup` command handles all files.
- 56 numbered SQL migrations with checksums and no down migrations [repo: ORB-029 in `backend-gaps-and-improvements.md`]. Fine for SQLite; this is the main cost if the engine ever changes (see Alternatives).

### 6. Single host, no HA

WAL needs shared memory, so all processes must be on one host and a local filesystem [ext]. Docs state the topology: "scaling out horizontally means moving off SQLite or running a single node" (`docs/architecture/backend-architecture.md`) and "one Core host" (`system-overview.md`). I found no HA or multi-replica goal anywhere in `docs/` (searched for failover, high availability, replica, clustered). If that becomes a requirement, SQLite is out for `orion.db`. Today the recovery model is restore from backup plus Nodes that spool and replay (chaos scenario 1 [repo]).

### 7. Slow disks and small hosts (not measured)

No run exists on an SD card, a throttled VPS disk or a 1 GB host. Known risks, all [est]:

- The read-only Docker rootfs mounts `/tmp` as tmpfs (`deploy/docker-compose.yml`), so SQLite temp files for big sorts and `GROUP BY` live in RAM. `temp_store` is not set.
- Page cache is the SQLite default (a few MB per connection, 8 connections); `cache_size` and `mmap_size` are not set. On a 1 GB host with a 10 GB `logs.db` the histogram is I/O bound: the one cold-cache observation was 4 times the warm time [bench].
- `synchronous=NORMAL` in WAL keeps fsyncs rare, which is the right setting for slow storage. Checkpoints (`TRUNCATE` after every prune) force syncs and wait for readers.
- `wal_autocheckpoint` and `journal_size_limit` are not set. Defaults checkpoint at about 1,000 pages (4 MB) [ext]; a long-lived reader (a live-tail SSE stream or a slow export) can stop the WAL from being recycled and make it grow [ext].

### 8. Driver and build

- Core uses `mattn/go-sqlite3` (cgo) through `gorm.io/driver/sqlite`. Image builds need `build-base`, and CI builds arm64 natively, so this works today. The cost is that Core cannot be shipped as a static no-toolchain binary, and Core and Node use different SQLite drivers (mattn vs `glebarez`/`modernc`).
- Moving Core to the pure-Go driver would remove cgo. Speed difference is **unmeasured here**; do not switch without running the soak harness on both. FTS5 availability in that driver should be checked in the same step (I believe it is included; not verified).

### 9. SQLite-specific code

Core has 8 triggers in 2 migrations (uptime exclusions), `json_extract` in 4 Go files (field facets), `strftime`/`datetime` in 2 Go files and 3 migrations, `rowid` in 2 files, `INSERT OR`/`ON CONFLICT` in 2, `PRAGMA` in 5 plus one migration, partial indexes, and 50 `Transaction(` call sites [bench: grep]. GORM hides most of the rest. A port is real work (see Alternatives) but not a rewrite.

One thing to check: `_txlock=immediate` takes the write lock at `BEGIN`, so any GORM `Transaction` that only reads still serializes with writers. 50 call sites exist; none were audited for read-only use.

## Growth scenarios

Assumptions: 750 B per log row [repo]; 100 B per sample row [bench: `WITHOUT ROWID`, PK `(node_id, key, ts)`, plus a `ts` index, 2.016M rows = 192 MiB]; 200 series per node at 1-minute resolution, 7-day retention (service-correlation plan 4.3); hourly rollups 90 days.

### Metrics samples

| Fleet | Rows/min | Rows at 7 d | Size at 7 d | Hourly rollups (90 d) |
|---|---|---|---|---|
| 5 nodes | 1,000 | 10M | 1 GB | 2M rows, about 0.2 GB |
| 50 nodes | 10,000 | 100M | 10 GB | 22M rows, about 2 GB |
| 500 nodes | 100,000 | 1.0B | 100 GB | 216M rows, about 22 GB |

[est] from [bench] row size. Real data with small numbers should be smaller than my random floats; treat as an upper bound within 2x.

Measured on one node of data (2.016M rows) [bench]:

| Query | Time |
|---|---|
| One series, 24 h (1,440 rows) | 1 ms |
| One series, 7 d (10,080 rows) | under 1 ms |
| Service sum over 1 h (60 buckets, 20 series) | 7 ms |
| Hourly rollup of one day (288k rows -> 5,000 groups) | 0.26 s |
| Delete one day of samples, one transaction (288k rows) | 0.34 s |

Write side: 500 nodes is 1,667 rows/s, about 500 small transactions per minute. That is well under the measured bulk ceiling of 100k rows/s and the 3,300 req/s report burst [repo/bench]. **Throughput is not the problem; file size is.** At 500 nodes the single file is 100 GB, `.backup` and restore are long, and nightly prune deletes 144M rows/day (about 12% of writer time if deletes cost like the log case) [est].

**Where SQLite stops being adequate for metrics:** at about 100 to 200 full-cardinality nodes (a 20 to 40 GB `metrics.db`) the file is still workable but ops-painful; above that, either shorten retention (2 days at 1-minute = 29 GB at 500 nodes [est]), reduce resolution after 24 h, or use an external time-series store through an adapter.

### OTLP metrics (S5) need a different table shape

Section 4.3's table is one wide row per process (7 values). OTLP metrics arrive as one value per (metric name, attribute set). Storing them wide would waste columns; storing them in the same table tagged by `source` would give mostly-null rows. Use a narrow table: `series(id, app_id, name, attrs_hash, attrs_json)` and `samples(series_id, ts, value)` with a `WITHOUT ROWID` primary key `(series_id, ts)`, about 25 to 30 B/row [est]. A single OTel SDK service easily emits 100 to 500 series at 10 to 60 s intervals (HTTP server duration histogram buckets alone are 10 or more series per route). That is 14M to 70M rows/day for one busy app at 10 s [est], so the plan's cap (R-4) must be enforced by default, with a low default per app (for example 500 series).

### Logs

See the tables in "Where it strains". Summary of where SQLite as configured stops being adequate:

| Lines/s | Verdict as configured | Reason |
|---|---|---|
| Up to about 100 | Fine | 91 GB at 14 days is a disk-sizing problem; give a cap. Histograms are sub-second. |
| About 100 to 1,000 | Fine with changes | Needs a size cap, FTS5 (or a hard search window), a minute rollup for histogram and incident bursts. Write path has headroom (measured 2,000/s for 30 min [repo]). |
| About 1,000 to 5,000 | Strained | Disk 65 to 324 GB/day; retention must be hours to a few days; histogram without rollup is 3 to 15 s per hour of data; prune is 15% to 40% of writer time [est]. Might hold on NVMe; unmeasured, and the soak was never run. |
| Above about 5,000 | Not adequate | Beyond what I would promise on one SQLite writer with 7 indexes and FTS; consider a columnar store via an adapter. |

The 5,000 figure is an estimate. The measured facts are 2,000/s sustained for 30 min and 20,000/s tiny entries for 10 s. The honest limit is "unknown between those, and untested on slow disks".

### Main database

Report ingest has headroom: 500 nodes with 20 monitors each at 30 to 60 s is 170 to 330 reports/s [est], against 3,300/s burst measured with 1,000 monitors. Row growth is driven by `monitor_reports` (14M rows/day at that fleet [est]); the archive job and daily rollups exist for this, and raw-row hot retention is configurable. The scenario is untested above 200 monitors and 5.18M rows.

## Alternatives

Judged against: self-hosted, one-command install, low ops, small hosts. "Migration" is the cost from today's GORM models and 56 SQL migrations.

| Option | Fits | Ops burden | Install impact | Migration cost | Licence | Verdict |
|---|---|---|---|---|---|---|
| **A. SQLite as is** | Everything today | None | None | None | Public domain | Fine now; has known gaps (size cap, FTS, rollup). |
| **B. SQLite split files + tuned pragmas** (`orion.db`, `logs.db`, new `metrics.db`) | Metrics samples, rollups, prune isolation | None | None; one more file to back up | Low: a metrics store package with its own schema (logs.db already does this inline). Loses same-transaction writes across files [ext: WAL doc: atomic per DB only] | Public domain | **Recommended.** |
| **C. DuckDB** (embedded) | Columnar aggregation: histograms, rollups, wide scans | Low if embedded | Adds a cgo library; binary size not measured. One read-write process only [ext: DuckDB concurrency doc], but Core is API + Worker in two processes, so the Worker would have to read through the API or use read-only access | Medium: new store behind an interface; no GORM path assumed. Bad at many tiny transactions [ext] | MIT (not re-verified) | Candidate for an optional analytics store later; poor as the main DB. DuckDB 2.0 documents a Quack client-server mode, which removes the benefit of embedding [ext]. |
| **D. Postgres** (optionally + TimescaleDB) | Everything; concurrent writers; real FTS; partition drop for cheap retention; HA possible | Real: a third container, credentials, version upgrades, `pg_dump`, tuning | Breaks the stated "no Postgres" goal in README and `system-overview.md` | High: about 15 Go files and 6 migrations with SQLite-specific SQL (triggers, `json_extract`, `strftime`, `rowid`, `INSERT OR`, PRAGMA), data-copy tool for existing installs, two dialects in CI during transition | PostgreSQL licence (permissive). TimescaleDB core is Apache-2.0, but a `tsl` directory is under the Timescale License [ext: LICENSE]; which features are in `tsl` (compression, continuous aggregates) is from my memory and must be checked before relying on it | Only if HA, multi-replica or 100+ node fleets become goals. TimescaleDB is overkill below that. |
| **E. ClickHouse** | Logs and metrics at 10k+ lines/s; excellent compression and scans | High for a small host: separate server, memory and tuning appetite (my knowledge, not verified here) | Another container; not Pi-class | High for core data (not relational); fits logs/metrics only | Apache-2.0 (not re-verified) | Right answer for a large deployment's logs, wrong default. Offer through an adapter. |
| **F. Embedded or sidecar TSDB** (VictoriaMetrics single node; Badger/Pebble DIY) | Metrics only | VictoriaMetrics: one more process and its own retention/backup; its RAM and compression claims are vendor benchmarks [ext] | Second binary; no longer one process | VM: needs a query layer (PromQL-style) in Console. Badger/Pebble: you would write a TSDB (rollups, retention, compaction) | VM single node Apache-2.0 [ext]; Badger/Pebble permissive (not re-verified) | Not now. Badger/Pebble is the worst trade: high build cost for a problem SQLite solves up to about 100 nodes. |
| **G. Optional external-store adapter** | Large users only | Zero for default users | None for default | Medium: define `LogStore`/`MetricStore` interfaces, keep SQLite impl as default, add one external impl later | n/a | Do the interfaces now; build the adapter only when someone asks. |

## Recommendation and triggers

### Decision

Keep SQLite (option A to B). Do not start a Postgres or ClickHouse migration. Treat D-001 as confirmed for the current product and the S1 to S3 correlation phases, and reconfirm with evidence before S4 ships.

### Do now (small, no new dependencies)

1. **Metrics go to `metrics.db`, not `orion.db`.** Make the plan's R-1 fallback the design. Put it behind a `MetricStore` interface (`WriteSamples(batch)`, `QuerySeries`, `Rollup(hour)`, `Prune(before)`). `metrics.db` gets the same WAL profile (`db.OpenFile` already supports this), its own prune and rollup job, and an explicit exclusion from migration backups.
2. **Use `WITHOUT ROWID` with PK `(node_id, key, ts)`** (measured above) and integer epoch timestamps. Prune in batches (about 10k rows) rather than one transaction.
3. **Add a default log size cap.** For example a global cap of 10 GB or a free-disk watermark (stop accepting new debug/info before the disk fills), and make the first-run setup show the retention math. Today 14 days with no cap can fill a small VPS.
4. **Decide search semantics and build FTS5.** Recommended: FTS5 `unicode61` on `body` + `event` + `attributes` (+24% disk measured), keep a `contains` fallback via `LIKE` only when the window is 1 hour or less. Add `-tags sqlite_fts5` to the Dockerfile, Makefile and CI. Measure online insert cost at 2,000 lines/s first.
5. **Add a per-app minute rollup for logs** (`log_rollups(app_id, minute, severity, count)`), updated by the writer in the same batch transaction. This serves the Logs histogram, incident burst counts (S6) and log-alert "threshold" rules without scanning entries.
6. **Close the evidence gaps** that D-001 never had: run the 72h cross-process soak with retention active and a short `retention_days`, on a throttled-disk 2 vCPU / 2 GB VM and on one Raspberry-Pi-class device if it is a supported target (docs name only Linux/macOS, amd64/arm64). Re-do I-003 (slow SQLite after hours of uptime). Run a 10M-row list/search/histogram check.
7. **Explicit pragmas, evaluated by the soak:** `wal_autocheckpoint`, `journal_size_limit` (cap WAL growth), `cache_size`, `temp_store`, and a periodic `PRAGMA optimize`. Do not change them without the numbers.
8. **Add a single `orion-core backup` / restore command** that snapshots all files together and reports sizes, before there are three databases.

### Keep the door open (cheap, now)

- `LogStore` and `MetricStore` interfaces in front of `logstore` and the new metrics package; Console and API depend on the interface only.
- No new SQLite-only SQL in new tables: no triggers, no `json_extract` in hot paths (store the columns you filter on), integer epoch times, `ON CONFLICT` upserts (ANSI-ish) rather than `INSERT OR REPLACE`.
- Keep the `Repository` seam for `orion.db` services where you touch them anyway; do not do a broad refactor (AGENTS.md).
- Record sizes, rows, writer wait and query p95 per DB in the existing diagnostics (`/v1/diagnostics/core`) so the triggers below are observable by users and by us.

### Triggers to revisit (any one)

| Signal | Threshold | Action |
|---|---|---|
| Histogram or incident-burst p95 on the default 1 h window | above 3 s (above 1 s after the rollup exists) | Check rollup is used; cap windows; then consider the adapter |
| Log list or search p95 | above 1 s on the default window | FTS5 first; then adapter |
| `logs.db` size or rows on one host | above 50 GB or above 100M rows | Enforce caps/retention; evaluate columnar adapter |
| Sustained log ingest | above 5,000 lines/s, or queue-full 429s above 0.1% of requests, or `orion_logs_storage_full` set | Adapter for logs |
| `metrics.db` size | above 20 GB, or above 200M rows (about 100 full-cardinality nodes) | Shorter retention, coarser resolution, or external TSDB adapter |
| `orion.db` size excluding logs | above 10 GB, or migration backup above 1 minute | Find the growth table; move it out |
| Writer contention | any `SQLITE_BUSY` in normal operation, or ingest p95 above the 100 ms plan target at 330 reports/s | Audit read-only `Transaction` use and `_txlock=immediate`; batching; then Postgres decision for `orion.db` |
| WAL | `-wal` above 1 GB for more than a few minutes, or `TRUNCATE` checkpoints failing | Find the long reader; tune checkpointing |
| Product | A requirement for HA, multi-replica Core, or customers above about 200 nodes | Postgres for `orion.db`; decide with a real requirement, not before |

Thresholds are my estimates built from the measurements above; adjust after the soak.

### What to change in the service-correlation plan

1. **4.3 and R-1:** samples live in `metrics.db` from the start, not `orion.db`. The sentence "samples are written in the same transaction as the latest-row upsert" must go: two files are not atomic together [ext]. Samples are append-only and a lost sample is acceptable; write them after the latest-row upsert, off the request path (R-5 already wants this).
2. **4.3 sizing:** replace "about 2M rows per busy Node. Measure before shipping" with the measured 192 MiB per node-week and the fleet table above; make 7-day retention the default for up to about 20 nodes and drop to 24 h at 1 minute then hourly beyond that, configurable.
3. **4.4 OTLP:** narrow `series` + `samples` tables, not the wide per-process table, and a low default series cap per app, enforced from the first release.
4. **4.6 and S6:** use the log minute rollup for the burst counts. Without it, the "one timeline" query scans the log window on every incident page view.
5. **S4 exit criteria:** add measured gates: 200 series x 500 nodes written at 1,667 rows/s with report p95 unchanged; chart query p95 under 100 ms; prune of a day without a writer stall above 1 s.
6. **S1 to S3:** unchanged. They need no new storage, so they can ship before any of the storage work; the plan is right about that.

## Open questions

- Is a Raspberry Pi or other SD-card host a supported target, or only "a VPS"? It sets whether the "small host" profile (3-day logs, 2 GB cap, 24 h samples) must be a documented preset.
- Is HA or a standby Core ever a goal? If "restore from backup in minutes" is enough, SQLite stays; if not, plan the Postgres decision now rather than under pressure.
- What are realistic log rates for the intended users? I assumed 1 to 100 lines/s as typical; the plan's 1,000/s target may be a stress figure, not a customer figure. Ask the first real users.
- Is substring search required, or are tokens enough? It decides FTS5 `unicode61` (+24%) versus `trigram` (+133%).
- Should the Node default (ship only WARN and ERROR, issue I-005) land before sample history, so `orion.db` growth is understood first?
- Is it worth moving Core to a pure-Go SQLite driver for static builds? It needs a speed comparison on the soak harness; I did not measure it.
- After the 72h soak, does `wal_checkpoint(TRUNCATE)` during ingest ever stall the writer for more than a second?

## Sources

External:

- SQLite, Write-Ahead Logging (one writer, readers do not block, same-host shared memory, checkpoint and WAL growth, ATTACH atomicity per DB): https://www.sqlite.org/wal.html
- SQLite, Appropriate uses (many concurrent writers favour client/server; 281 TB limit; network filesystems): https://www.sqlite.org/whentouse.html
- SQLite, FTS5 (external-content tables, `unicode61`, trigram tokenizer, indexed LIKE): https://www.sqlite.org/fts5.html
- SQLite, Implementation limits: https://www.sqlite.org/limits.html
- mattn/go-sqlite3 README (cgo required; `sqlite_fts5` build tag): https://raw.githubusercontent.com/mattn/go-sqlite3/master/README.md
- DuckDB concurrency (one read-write process, appends do not conflict inside it, multi-process via Quack or DuckLake with a PostgreSQL catalog): https://duckdb.org/docs/2.0/connect/concurrency.html
- TimescaleDB LICENSE (Apache-2.0 plus Timescale License for the `tsl` directory): https://raw.githubusercontent.com/timescale/timescaledb/main/LICENSE
- VictoriaMetrics README (single-node under Apache-2.0; vendor benchmark claims): https://github.com/VictoriaMetrics/VictoriaMetrics
- ClickHouse installation page (self-managed server install options): https://clickhouse.com/docs/en/install

Not re-verified (from general knowledge, marked in the text): DuckDB MIT and ClickHouse Apache-2.0 licences, ClickHouse memory appetite, which TimescaleDB features are in the `tsl` tree, modernc driver speed and FTS5 availability, Badger/Pebble licences.

In-repo evidence:

- `docs/plans/decision-log.md` (D-001, D-011, D-025, D-053)
- `docs/plans/backend-hardening-load-results.md`, `backend-hardening-baseline.md`, `backend-hardening-e2e-plan.md`
- `docs/plans/service-correlation-plan.md` (4.3, 4.4, R-1), `docs/plans/product-logs-plan.md` (3.5, 7)
- `docs/issue-log.md` (I-003, I-005), `docs/sqlite-backup-restore.md`, `docs/architecture/system-overview.md`, `backend-architecture.md`, `persistence-and-lifecycle.md`
- `apps/core/internal/db/{sqlite-profile.go,db.go,migration-backup.go,app-models.go}`
- `apps/core/internal/logstore/{store.go,query.go,analytics.go,prune.go,retention.go,perf_test.go}`
- `apps/core/Dockerfile`, `deploy/docker-compose.yml`, `apps/core/go.mod`, `apps/node/go.mod`

Bench method ([bench] figures): `sqlite3` 3.51.0 CLI on an Apple-silicon laptop, scratch files only, schema copied from `logstore/store.go`; 1M synthetic log rows (3 apps, JSON attributes, random ids); 2.016M sample rows (200 series x 10,080 minutes, `WITHOUT ROWID`); one `go test` of the existing `TestSustainedIngestThroughputAndLatency` at 2k, 10k and 20k entries/s for 10 s. Warm page cache except where noted. No slow-disk, low-RAM or concurrent-reader runs.
