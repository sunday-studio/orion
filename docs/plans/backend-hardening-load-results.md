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
| 5 | Disk 95% full, Core degrades loudly without corrupting the DB | Not automated | Needs a constrained filesystem; to be run by hand on the staging host |
| 6 | Agent clock skew of 10 minutes either way | Automated | `TestChaosAgentClockSkewDoesNotChangeHealth`, `TestScenarioD3...` |
| 7 | Webhook receiver down about an hour then up, backlog drains in order | Automated | `TestChaosWebhookReceiverDownThenUpDrainsBacklogInOrderWithoutDuplicates` (8 attempts over about an hour of backoff) |
| 8 | Agent boot with Core down 5 minutes | Automated | `TestRegisterWithRetryWaitsOutACoreOutage` |

## Soak (plan section 6.5)

Not run. The 72-hour soak needs a long-lived host. The harness is ready for it:

```sh
ORION_SOAK_AGENTS=50 ORION_SOAK_MONITORS=20 ORION_SOAK_INTERVAL=30s ORION_SOAK_DURATION=72h \
ORION_SOAK_OUTAGES=500 ORION_SOAK_WEBHOOK_FAIL_RATE=0.05 ORION_SOAK_REAL_BACKOFF=1 \
  go test ./internal/soak -run TestSoak -timeout 80h -v
```

Pass criteria (from the plan): zero lost or duplicated alerts (the harness verifies this per outage), flat database size after the retention window, goroutines and memory within 10% of hour 24, no lock errors, p95 ingest under target. Gaps to close before calling the soak done: the harness does not yet include flapping monitors, three real agents, or two receivers, and it runs against an in-process Core rather than a deployed one.

## How the CI gate relates

`go test ./...` runs `TestSoak` at 10 agents x 5 monitors for 10 s with a 35% webhook failure rate so the retry path is always exercised, and fails on any violation or an ingest p95 above 200 ms.
