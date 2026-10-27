# Backend Hardening: Load, Chaos and Soak Results

Measured 2026-10-17 on a developer laptop (darwin/arm64, 12 cores), Go `internal/soak` harness. The harness runs the real Core API, the real Worker loop and a file-backed WAL SQLite in one process, driven over HTTP by simulated agents. It is not a substitute for the staging soak below; it is the repeatable gate that runs in CI at small scale.

## Load profile (plan section 6.4)

`ORION_SOAK_AGENTS=50 ORION_SOAK_MONITORS=20 ORION_SOAK_INTERVAL=30s ORION_SOAK_DURATION=200s ORION_SOAK_OUTAGES=20 ORION_SOAK_WEBHOOK_FAIL_RATE=0.05 go test ./internal/soak -run TestSoak -v -timeout 15m`

| Metric | Result | Target |
|---|---|---|
| Fleet | 50 agents x 20 monitors, 30s interval (about 33 reports/s) | same |
| Reports sent / failed | 6648 / 0 | 0 failed |
| Ingest latency (client) p50 / p95 / p99 | 0.77 ms / 3.5 ms / 6.3 ms | p95 under 200 ms |
| `SQLITE_BUSY` / `database is locked` | 0 / 0 | 0 |
| Injected outages | 20, each on its own agent | n/a |
| Alerts per outage | exactly 1 opened and 1 resolved, in both the delivery table and what the receiver accepted | exactly 1 + 1 |
| Webhook receiver | 41 calls for 40 required deliveries (5% injected failure, retried) | no loss, no duplicate |
| Goroutines before / after | 6 / 17 | no growth |
| Database size | 5.8 MB | n/a |

Burst behaviour (found while debugging the harness, which initially fired every monitor in the same instant): a 1000-request simultaneous burst, as happens when a Core outage ends and agents replay, drained in about 300 ms with zero errors (p50 220 ms, p99 300 ms), roughly 3,300 requests/s on one SQLite file.

Single-report cost (`go test ./internal/service -bench StoreMonitorReport`): about 0.13 ms per report with 1000 monitors.

## Before numbers

No "before" run exists. The pre-hardening build has no harness, and the audited defects (no WAL or busy timeout, inline webhook delivery, no dispatcher) are covered by regression tests that fail on the old code rather than by a latency comparison. The coverage baseline is in [backend-hardening-baseline.md](backend-hardening-baseline.md).

## Chaos scenarios (plan section 6.4)

| # | Scenario | Status | Evidence |
|---|---|---|---|
| 1 | Core down 5 minutes, agents spool and replay without duplicates | Covered in parts | Agent spool test `TestAgentPersistsSystemReportDuringCoreOutage`; replay idempotency A8 (`TestScenarioA8...`); reconnect burst above |
| 2 | Agent killed, sweeper alerts | Automated | `TestScenarioD1D2SilentAgentAlertsThenRecovers`, `TestSweepSilentMonitors...` |
| 3 | Worker killed mid-check, lease re-claimed once | Automated | `TestChaosWorkerKilledMidCheckLeaseExpiresAndIsReclaimedOnce`, `TestLeaseIsRenewedWhileLongCheckRuns`, `TestShutdownWaitsForInflightChecksAndReleasesLeases` |
| 4 | Core killed mid-ingest, no half-written incident | Automated (rollback kill-point) | `TestScenarioA6IncidentAndDeliveryCommitTogether`, `TestFailedReconcileRollsBackReportSoRetryRecovers` |
| 5 | Disk full, Core degrades loudly without corrupting the DB | Automated (SQLITE_FULL via `PRAGMA max_page_count`) | `TestDiskFullDegradesLoudlyKeepsDataAndRecovers` (logs.db: batch dropped and counted, ingest answers 503 + Retry-After, `orion_logs_storage_full` gauge, rows unchanged, `integrity_check` ok, recovers once space returns) and `TestChaosMainDatabaseDiskFullFailsCleanlyAndRecovers` (main DB: writes fail with a clear error, reads and health keep working, no corruption, recovers). A real 95%-full volume is still worth one manual run on staging |
| 6 | Agent clock skew of 10 minutes either way | Automated | `TestChaosAgentClockSkewDoesNotChangeHealth`, `TestScenarioD3...` |
| 7 | Webhook receiver down about an hour then up, backlog drains in order | Automated | `TestChaosWebhookReceiverDownThenUpDrainsBacklogInOrderWithoutDuplicates` (8 attempts over about an hour of backoff) |
| 8 | Agent boot with Core down 5 minutes | Automated | `TestRegisterWithRetryWaitsOutACoreOutage` |

## Cross-process load (API + Worker + live tails)

`TestCrossProcessLoad` (`internal/soak/cross_process_test.go`) builds the real `api` and `worker` binaries and runs them against one data directory, as production does. Load: 2,000 log entries/s from 4 writers in NDJSON batches of 50, 5 live SSE tails, 20 heartbeat monitors pinged every second (main-DB writes through the API while the Worker writes the same DB), a log-alert rule the Worker evaluates against `logs.db` through its read-only handle, and a list + histogram query each second. It then SIGTERMs both processes with tails still attached and verifies: every acknowledged entry is stored, `integrity_check` is ok, no 5xx or throttling, ingest p95 <= 250 ms, read p95 <= 1 s, the alert webhook fired, every tail received events, no `database is locked` / `SQLITE_BUSY` / `panic` in either process log, and (runs of 10 minutes or more) RSS did not double.

| Run | Duration | Accepted = stored | Ingest p50 / p95 / p99 | Read p95 | Tails (events each) | Webhook fired | Lock errors | integrity |
|---|---|---|---|---|---|---|---|---|
| Smoke | 90 s | 179,850 = 179,850 | 0.51 / 1.36 / 2.17 ms | 75 ms | 5 x about 179.7k | yes | 0 | ok |
| 30 min (run 1) | 30 min | 3,599,150 = 3,599,150 | 0.49 / 2.13 / 7.68 ms | list 1.9 s with histogram mixed in | 5 x about 3.6M | yes | 0 | ok |
| 30 min (run 2) | 30 min | 3,597,200 = 3,597,200 | 0.49 / 2.38 / 8.52 ms | list 4.4 ms | 5 x about 3.6M | yes | 0 | ok |

Run 1 failed its own read-latency check: the single read p95 mixed the list query (milliseconds) with `/v1/logs/histogram`, which aggregates every row in the requested window (about 3.6M rows by minute 30). The test now measures them separately; run 2 passed with list p95 4.4 ms and histogram p95 2.95 s at 3.6M rows in the window, against a 5 s bound. Finding: the histogram cost grows linearly with rows in the window, so the Console should default to a short window (for example 15 minutes) or the histogram needs a pre-aggregated rollup before very high-volume apps are shown over long windows. RSS after 30 min: API 88 MB, Worker 29 MB (logs.db 2.7 GB, WAL under 30 MB).

```sh
make core-soak-30m          # 30 minutes
make core-soak-72h          # 72 hours; set ORION_CROSS_OUT to keep the JSON samples
```

## Soak (plan section 6.5)

The 72-hour soak has not been run: it needs a host that stays up for three days. Two runners exist and are ready:

1. The monitor and alert soak (in-process Core, agents, outages, webhook failures):

```sh
ORION_SOAK_AGENTS=50 ORION_SOAK_MONITORS=20 ORION_SOAK_INTERVAL=30s ORION_SOAK_DURATION=72h \
ORION_SOAK_OUTAGES=500 ORION_SOAK_WEBHOOK_FAIL_RATE=0.05 ORION_SOAK_REAL_BACKOFF=1 \
  go test ./internal/soak -run TestSoak -timeout 80h -v
```

2. The cross-process soak above (`make core-soak-72h`), which adds the real binaries, product-log ingest, live tails and RSS-growth checks.

Pass criteria (from the plan): zero lost or duplicated alerts (the monitor soak verifies this per outage), flat database size after the retention window, goroutines and memory within 10% of hour 24, no lock errors, p95 ingest under target. Gaps to close before calling the soak done: the monitor soak does not yet include flapping monitors, three real agents, or two receivers; the cross-process soak does not exercise retention pruning over a full window (set `retention_days` low and a long duration to see the size plateau).

## CI workflow dry run

`actionlint` (with shellcheck) reports no findings on `ci.yml`, `agent-binaries.yml` and `core-image.yml`. `act -n` (dry-run, `catthehacker/ubuntu:act-latest`, `ubuntu-24.04` and `ubuntu-24.04-arm` mapped) plans every job of the three workflows successfully; the CI jobs gated on the `changes` output are skipped in a dry run, so they were verified by running their commands (`go test -race ./...`, `make code-line-limit`, `make generate-openapi`) directly. The dry run does not execute steps, so publishing, signing and registry login are untested until a real release.

## How the CI gate relates

`go test ./...` runs `TestSoak` at 10 agents x 5 monitors for 10 s with a 35% webhook failure rate so the retry path is always exercised, and fails on any violation or an ingest p95 above 200 ms.

## Uptime rollups: Overview and monitor list (2026-10-27)

`BenchmarkUptimeEndpoints` (`apps/core/internal/api/uptime-bench_test.go`) seeds a file-backed WAL SQLite with raw reports for every monitor, rolls up each completed day, then times `GET /v1/monitors` (first page of 50) and `GET /v1/overview` through the real router. Darwin/arm64, M4 Pro. "Before" is `e9b90bcc`, the parent of `1b17e71e`; "after" is `1b17e71e`; HEAD is `9cc1e789`, which includes the time-based uptime in `1d172bbd`.

```sh
cd apps/core && ORION_BENCH_MONITORS=100 ORION_BENCH_DAYS=90 ORION_BENCH_INTERVAL=900 \
  go test ./internal/api -run XXX -bench UptimeEndpoints -benchtime 5x
```

| Fleet (90 days of raw reports kept) | Build | `/v1/overview` | `/v1/monitors` |
|---|---|---|---|
| 100 monitors, 900 s interval, 864k rows | before | 2273 ms | 3.4 ms |
| | after `1b17e71e` | 25 ms | 3.8 ms |
| | HEAD | 29 ms | 3.9 ms |
| 200 monitors, 300 s interval, 5.18M rows | before | 14175 ms | 5.9 ms |
| | HEAD | 85 ms | 6.4 ms |

Findings:

- The rollup change fixes the Overview: about 90x at 864k rows and about 165x at 5.18M. The old cost grew with raw rows; the new cost grows with the rows of the current day, which is why HEAD went from 29 ms to 85 ms when the per-monitor check rate tripled.
- The monitor list was not slow in this setup. Both builds take a few milliseconds for a 50-row page, so batching recent checks into one `UNION ALL` made no measurable difference here. It was probably cheap already because SQLite has no network round trip per query. Any real-world slowness in the list needs the slow-query log from a production database, not this fixture.
- Time-based uptime (`1d172bbd`) cost about 4 ms on the Overview at 864k rows and is inside the noise on the list.
- Not covered: the node and monitor detail uptime endpoints, the status page history endpoint, rollup job duration, and a database with missing rollups for completed days (those fall back to raw aggregation and will be slower).

### Detail endpoints and missing rollups (2026-10-27)

Same benchmark, extended: it also times `GET /v1/monitors/{id}/uptime?period=90d` and `GET /v1/nodes/{id}/uptime?period=90d` (a status page component calls the same `GetMonitorUptime` and `GetNodeUptime` per mapped resource, so these are its per-mapping cost; the node case has every monitor mapped), then deletes every row in `monitor_uptime_rollups` and repeats, which forces the raw-report fallback for all 90 days. "Before" is `e9b90bcc`; HEAD is `9cc1e789`.

| Fleet | Case | Build | Monitor uptime | Node uptime | Overview |
|---|---|---|---|---|---|
| 100 monitors, 864k rows | rollups present | before | 24 ms | 2227 ms | 2258 ms |
| | | HEAD | 0.4 ms | 30 ms | 31 ms |
| | no rollups | before | 22 ms | 2273 ms | 2207 ms |
| | | HEAD | 10.7 ms | 1215 ms | 1168 ms |
| 200 monitors, 5.18M rows | rollups present | before | 73 ms | 14561 ms | 14551 ms |
| | | HEAD | 0.6 ms | 87 ms | 91 ms |
| | no rollups | before | 74 ms | 15455 ms | 14699 ms |
| | | HEAD | 37.5 ms | 8158 ms | 9146 ms |

Findings:

- Detail endpoints follow the Overview: with rollups, a single monitor is under 1 ms and a node with 200 monitors is under 100 ms. The before build ignored rollups, so its numbers did not change when they were deleted.
- Without rollups the new code is only about 1.5 to 2x faster than before, because the SQL aggregation still scans every raw row for the missing days. A database with missing rollups is nearly as slow as the old code.
- Rollups are only created by the daily job, which rolls up yesterday (`RunDailyMonitorUptimeRollup`, gated on `RollupsEnabled` and `shouldRunDailyJob`). Nothing backfills existing history. An install that upgrades with weeks of raw reports, or one whose Core was down at job time, keeps taking the slow path for those days until they are rolled up by hand. This is the main remaining risk of the uptime work.
- Status page history (`/status/:slug/history`) was not timed directly; its cost is the per-mapping cost above times the number of mappings, and it caches within a request.

### Rollup backfill (2026-10-27)

`RollupService.BackfillMissingMonitorUptimeDays` (`apps/core/internal/service/rollup-backfill.go`) runs from the data lifecycle scheduler on every tick while rollups are enabled and the schedule is not manual. It finds completed days inside the last 365 days that have raw reports but no rollup rows and rolls them up, newest first, at most 31 days per tick, so a long history catches up over a few hourly ticks instead of one long job. Days with no raw reports are skipped (the read path handles them without scanning). Once nothing is missing it costs one `MIN(created_at)` and one distinct-date lookup on the rollup index.

The benchmark now runs the backfill after deleting every rollup (in batches, as the scheduler would) and then repeats the endpoint timings.

| Fleet | Backfill | Endpoints after backfill |
|---|---|---|
| 100 monitors, 864k rows, 90 days | 0.62 s in 3 ticks | same as "rollups present": Overview 30 ms, node uptime 30 ms, monitor uptime 0.4 ms |
| 200 monitors, 5.18M rows, 90 days | 2.5 s in 3 ticks | not re-timed at this scale; expected to match the 5.18M "rollups present" row |

The no-rollups case this replaces was 1.2 s (864k rows) and 9.1 s (5.18M rows) per Overview request. Backfilled days use the same gap accounting as the nightly job, so days before migration 000056 keep `missed_count` 0.
