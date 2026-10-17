# Backend Hardening: End-to-End Plan

Status: proposed, 2026-10-17. Owner: backend thread (`apps/core/`, `apps/agent/`, `deploy/`).
Source: [backend-gaps-and-improvements.md](../architecture/backend-gaps-and-improvements.md) (ORB-001 to ORB-043) and [backend-architecture.md](../architecture/backend-architecture.md).

## 1. Goal and exit definition

Make Orion trustworthy as a monitor: **when something breaks, a human is told, exactly once, within a bounded time, and the system stays safe and observable while doing it.**

The plan is done when every gate in section 8 passes in CI and the soak test in section 6.5 completes clean.

Out of scope: console/UI work (web thread), new product features, multi-user RBAC (ORB-032 long-term part).

## 2. Principles

1. Every fix lands with the test that would have caught it. No test, no merge.
2. Failure-mode tests run on a **file-backed WAL SQLite**, two processes (API plus Worker), not a single in-memory connection. Today's tests miss this class.
3. One design for alerting: transactional outbox plus dispatcher (ORB-001, 004, 005). Do not patch the three separately.
4. Contract changes update `openapi.yaml` (`make generate-openapi`) and `docs/agent-core-contract.md` in the same PR.
5. Each PR needs a Maat ticket, the 500-line limit, and independent staff review (AGENTS.md).

## 3. Phases

```
Phase 0  Foundations        test harness, DB profile, CI gates        (unblocks everything)
Phase 1  Make alerting real outbox, dispatcher, sweeper, ownership   (core job works)
Phase 2  Close exposure     auth default, SSRF, limits, enrollment    (safe to expose)
Phase 3  Operate safely     retention, migrations, agent, deploy      (safe to run for months)
Phase 4  Prove it           metrics, load, chaos, soak                (evidence, not belief)
```

Phases 1 and 2 can run in parallel after Phase 0 if they touch disjoint files. Phase 4 test work starts in Phase 0 and grows.

### Phase 0: Foundations (S to M, about 3 days)

| WP | Work | Findings | Done when |
|---|---|---|---|
| 0.1 | **SQLite production profile**: WAL, `busy_timeout=5000`, `foreign_keys` deferred until cleanup, `synchronous=NORMAL`, small pool plus write serialization, UTC `NowFunc`, `0700` dir / `0600` file, GORM log level Warn and env-configurable | ORB-006, 007, 020, 031 | Starting API+Worker on one file under 50 concurrent writers produces no `database is locked`; logs contain no bound values |
| 0.2 | **Test harness** `internal/testkit`: file-backed WAL DB factory, fake clock, fake webhook server (configurable fail/slow/redirect), two-agent fixture, helper to run API and Worker loops in-process | ORB-042 | New harness used by at least WP 1.1 and 1.3 tests |
| 0.3 | **CI gates**: add `go vet`, `go test -race`, `govulncheck`, `golangci-lint`, route-vs-annotation diff; pin `swag`; Dependabot for gomod and actions | ORB-039, 038 | A PR with a race or an unannotated route fails CI |
| 0.4 | **Baseline snapshot**: record coverage, p95 ingest latency, and SQLite busy counts from the current build as the "before" numbers | n/a | Numbers saved in `docs/plans/` for comparison in Phase 4 |

### Phase 1: Make alerting real (M, about 1.5 weeks)

| WP | Work | Findings | Done when |
|---|---|---|---|
| 1.1 | **Outbox plus dispatcher.** Incident open/resolve writes `alert_deliveries(pending)` in the same transaction. A dispatcher loop in the Worker claims due rows with a lease (`locked_by`, `locked_until`), sends, records attempts, schedules retry with backoff, and handles `delayed_summary` groups. Ingestion returns after commit and never waits on webhook HTTP | ORB-001, 004, 005 | Tests in 6.1 pass |
| 1.2 | **Transactional, unique incidents.** One transaction for open/update/resolve. Partial unique index on active incidents per monitor (dedupe existing rows in the migration) | ORB-004, 024 | 20 concurrent failing reports for one monitor yield exactly one incident and one `incident_opened` delivery |
| 1.3 | **Silent-agent sweeper.** Core loop every 30-60s: agents past `last_seen` stale window get an `agent_offline` incident; monitors with no recent report go stale through the normal reconcile path; recovery resolves. Heartbeat monitors that never started get a grace-period rule. Health staleness uses server time. Remove the O(all monitors) work from `ReconcileStaleMonitors` | ORB-002, 021, 022 | Killing an agent process raises an incident and a delivery within window plus 1 interval; restart resolves it |
| 1.4 | **Ownership check.** Unregister verifies the caller owns the monitor before resolving incidents; 404 otherwise | ORB-003 | Two-agent regression test passes |
| 1.5 | **Idempotent reports.** Agent-generated `report_id`, `INSERT ... ON CONFLICT DO NOTHING`, no 500 after a committed insert. Needs an agent change plus a contract note | ORB-019 | Replaying the same report 5 times stores one row |

### Phase 2: Close exposure (S to M, about 1 week, parallel with Phase 1)

| WP | Work | Findings | Done when |
|---|---|---|---|
| 2.1 | **Fail-closed auth.** Required by default; `ORION_ALLOW_INSECURE_NO_AUTH=true` is the opt-out and logs a loud WARN and binds to loopback. JWT: valid methods pinned, expiry required | ORB-008, 032 (partial) | Starting with no creds exits with a clear error |
| 2.2 | **HTTP server limits.** `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`; global body cap (1 MiB, 5 MiB for log batches); `SetTrustedProxies`; limiter janitor and atomic increment; bounded route label in diagnostics; custom access logger that logs `FullPath()` so heartbeat tokens never appear; generic error bodies; security headers; swagger gated | ORB-010, 012, 033, 035 | Oversized body returns 413; spoofed `X-Forwarded-For` does not evade the login limit; no token in logs |
| 2.3 | **Webhook SSRF guard.** Reuse `CoreMonitorTargetPolicy` for the alert client; require https unless flagged; reject URL credentials; disable redirects | ORB-011 | Loopback, link-local, metadata, and private targets rejected at create and at dial |
| 2.4 | **Agent enrollment.** `ORION_ENROLLMENT_TOKEN` required on `/v1/register`; per-IP rate limit; machine id from `/etc/machine-id` or persisted UUID; migrate and remove legacy plaintext tokens. Touches Core, Agent, and `agent-bootstrap.sh` together | ORB-009, 036 | Register without the secret returns 401; cloned-VM test gets distinct ids |
| 2.5 | **Query clamps.** Central `period` max 365d and `limit` max 200; keyset pagination for events | ORB-030 | Hostile `period` and `limit` return 400 or clamp, with bounded memory |

### Phase 3: Operate safely for months (M, about 2 weeks)

| WP | Work | Findings | Done when |
|---|---|---|---|
| 3.1 | **Retention for all growth tables** (`service_log_entries`, deliveries, attempts, events, audit, worker status) with per-table windows in `data_lifecycle_settings`; archive in 500-1000 row batches in short transactions; close the archive DB; incremental vacuum; DB size in diagnostics | ORB-026, 027 | 100k-row backlog archives without error; DB size plateaus in the soak test |
| 3.2 | **Migration safety.** Advisory lock for a single migrator, SHA-256 per file with verify, backup before apply, fold the ad-hoc ALTER into a numbered migration, read-only worker healthcheck. Add composite and unique indexes (dedupe first); enable `foreign_keys` after orphan cleanup | ORB-028, 029, 040 | Two processes starting together migrate once; editing an applied file is detected |
| 3.3 | **Agent robustness.** In-process registration retry with capped backoff; fix or remove SIGHUP reload; guard `internalState` and `authToken` for concurrent access; requeue the remainder on shutdown and use request contexts; validate intervals and emit a failed report instead of exiting silently | ORB-014 to 018 | `-race` stress test with concurrent token refresh passes; agent started with Core down for 5 minutes comes up once Core returns |
| 3.4 | **Worker concurrency.** Bounded pool (default 16), lease renewal for long checks, lease release on shutdown | ORB-023 | 100 slow targets do not delay a fast check beyond its interval |
| 3.5 | **Release integrity.** Publish `SHA256SUMS` plus a cosign/minisign signature; installer verifies before `chmod +x`; docs pin `VERSION` | ORB-013 | Tampered binary fails install |
| 3.6 | **Container and compose hardening.** Non-root, read-only FS, `cap_drop: [ALL]`, limits, pinned digests, `GIN_MODE=release`, worker env allowlist (drop admin/JWT secrets from the worker and Playwright subprocess), proxy example, `stop_grace_period: 30s` | ORB-037, 034 | `docker compose up` works under the hardened profile; Trivy clean at agreed severity |
| 3.7 | **Maintenance expiry and audit.** `maintenance_until`, auto-expire, audit event | ORB-025 | Expired maintenance resumes alerting |
| 3.8 | **Contract and docs refresh** (retry spool, concurrency, stale rule, Server/Agent naming, missing annotations) | ORB-038 | Doc matches code; CI route diff green |

### Phase 4: Prove it (M, runs alongside Phase 3)

| WP | Work | Findings | Done when |
|---|---|---|---|
| 4.1 | **Metrics and self-monitoring.** `/metrics`: request and ingestion rates, delivery queue depth, oldest pending delivery age, SQLite busy count, worker heartbeat age. Built-in "Core self" dead-man heartbeat and a worker-stale alert | ORB-041 | Stopping the Worker raises an alert from the system itself |
| 4.2 | **Load and chaos suite** (section 6.4, 6.5) | ORB-042 | Targets in section 8 met |

## 4. Dependency map

```
0.1 DB profile ──┬─> 1.1 outbox ──> 1.2 unique incidents
0.2 testkit ─────┤        └──────> 1.5 idempotent reports (agent change)
0.3 CI gates ────┘
1.3 sweeper needs 0.1, 0.2
2.4 enrollment: Core + Agent + bootstrap script in one change set
3.1 retention needs 0.1; 3.2 migrations before enabling foreign keys
4.2 load/chaos needs 0.1, 1.1, 1.3, 3.1
```

## 5. Environments

| Env | Purpose | Setup |
|---|---|---|
| Unit/integration (CI) | Fast, deterministic | `internal/testkit`: file-backed WAL DB, fake clock, fake webhook, in-process API plus Worker |
| Local stack | Manual and e2e | `make docker-up` with the hardened compose profile plus 2 agents (host agent and containerized agent) |
| Soak (staging host) | Time-based confidence | Core plus Worker plus 50 simulated agents (section 6.5), run 72 hours |

## 6. Test strategy

### 6.1 Alerting correctness (WP 1.1, 1.2, 1.5)

| # | Scenario | Expected |
|---|---|---|
| A1 | Webhook returns 500 once then 200 | Delivery `failed` then `sent`; exactly 2 attempts recorded |
| A2 | Webhook always fails | Retries per backoff, ends `failed` after max attempts, no further sends |
| A3 | Webhook takes 30s | Ingestion request returns in under 1s; dispatcher not blocked for other channels |
| A4 | Two dispatcher instances | Each delivery sent once (lease works) |
| A5 | `delayed_summary` group of 5 incidents | One summary sent after the delay |
| A6 | Crash between incident insert and delivery insert | Impossible: one transaction; kill-point test proves both or neither |
| A7 | 20 concurrent failing reports, same monitor | One incident, one `incident_opened` |
| A8 | Same report ID posted 5 times | One stored report, 200 each time |

### 6.2 Detection (WP 1.3)

| # | Scenario | Expected |
|---|---|---|
| D1 | Agent stops reporting | `agent_offline` incident and delivery within stale window plus sweep interval |
| D2 | Agent resumes | Incident resolves; recovery delivery sent once |
| D3 | Future-dated `collected_at` | Still treated stale by server time |
| D4 | Latest report `down` with flappy history | Health is `down`, with `flapping` as a separate flag |
| D5 | Heartbeat monitor never pinged | Alerts after grace period |
| D6 | Agent in maintenance | No alert; after `maintenance_until` alerting resumes |

### 6.3 Security (Phase 2)

| # | Scenario | Expected |
|---|---|---|
| S1 | Agent B calls unregister on agent A's monitor | 404; A's incident untouched |
| S2 | Start with no admin credentials | Process refuses to start without the opt-out flag |
| S3 | Create webhook to `http://169.254.169.254`, `127.0.0.1`, `10.x`, and a redirecting host | All rejected |
| S4 | `POST /v1/register` without enrollment token; 100 rapid requests | 401, then 429 |
| S5 | 10 MiB body to report/log endpoints | 413 |
| S6 | Spoofed `X-Forwarded-For` rotation against login | Limiter still trips |
| S7 | grep access and app logs after a heartbeat call and a channel insert | No heartbeat token, no webhook secret, no bound SQL values |
| S8 | `period=999999999d`, `limit=1e9` | 400 or clamp, flat memory |
| S9 | Cloned VM with identical MAC | Distinct agent identities |
| S10 | Tampered agent binary in installer | Install aborts |

### 6.4 Load and failure injection (Phase 4)

- **Load:** 50 agents, 20 monitors each, 30s interval (about 33 reports/s). Measure ingest p50/p95/p99, `SQLITE_BUSY` count, CPU/RSS of API and Worker.
- **Chaos, scripted with the testkit and `docker compose`:**
  1. Kill Core for 5 minutes: agents spool, reconnect, replay with no duplicates (idempotency).
  2. Kill an agent: sweeper alert (D1).
  3. Kill the Worker mid-check: lease expires, check re-claimed once.
  4. Kill Core mid-ingest (kill-point): no half-written incident.
  5. Fill the disk to 95%: Core degrades loudly, does not corrupt the DB.
  6. Clock skew +/- 10 minutes on an agent: health stays correct.
  7. Webhook receiver down 1 hour then up: backlog drains in order, no storm of duplicates.
  8. Agent boot with Core down for 5 minutes (ORB-015).

### 6.5 Soak (72h on staging)

50 simulated agents plus 3 real ones, 1% of monitors flapping, 2 webhook receivers with random 5% failure. Pass if: zero lost alerts (every injected outage has exactly one open and one resolve delivery), DB size growth flat after retention window, no goroutine or memory growth above 10% after hour 24, no `database is locked` errors, p95 ingest under target.

### 6.6 Regression guard

Every finding keeps its test permanently. Coverage floor enforced in CI: Core 70% (with `-coverpkg`), Agent 60%, no package at 0%.

## 7. Rollout and rollback

- Migrations are forward-only, so every migration PR ships with a pre-migration DB backup step (WP 3.2) and is released behind a documented restore procedure.
- Auth fail-closed (2.1) and enrollment (2.4) are **breaking**. Release notes must list the new env vars; ship one release with a WARN and the old behaviour opt-in via flag, then flip the default the release after.
- Dispatcher (1.1) ships behind `ORION_ALERT_DISPATCHER=on` for one release in staging, then default on.
- Roll back by redeploying the previous image; the outbox table is additive, and the unique incident index is the only constraint that can block a downgrade (documented).

## 8. Gates and targets

| Gate | Target |
|---|---|
| Alert delivery | 100% of injected outages produce exactly one open and one resolve delivery |
| Time to detect silent agent | under stale window plus 60s |
| Ingest latency (50 agents x 20 monitors) | p95 under 200ms, zero `database is locked` |
| Ingestion blocked by webhook | never (A3) |
| Auth | no unauthenticated mutation with default config |
| CI | vet, race, govulncheck, lint, route-diff, tests all green |
| Coverage | Core 70% (`-coverpkg`), Agent 60% |
| Soak | section 6.5 passes |
| Docs | contract doc and OpenAPI match code; Maat tickets complete with evidence |

## 9. Ticket map (Maat)

Create one goal ("Backend hardening") with tickets, each with outcome, description, and acceptance criteria:

| Ticket | Work packages | Findings | Phase |
|---|---|---|---|
| T0 Foundations | 0.1 to 0.4 | 006, 007, 020, 031, 039, 042 | 0 |
| T1 Outbox and dispatcher | 1.1, 1.2 | 001, 004, 005, 024 | 1 |
| T2 Silent-agent sweeper | 1.3 | 002, 021, 022 | 1 |
| T3 Ownership check | 1.4 | 003 | 1 |
| T4 Idempotent reports | 1.5 | 019 | 1 |
| T5 Secure defaults and limits | 2.1, 2.2, 2.5 | 008, 010, 012, 030, 033, 035 | 2 |
| T6 Webhook SSRF guard | 2.3 | 011 | 2 |
| T7 Enrollment and identity | 2.4 | 009, 036 | 2 |
| T8 Retention and archive | 3.1 | 026, 027 | 3 |
| T9 Migration safety | 3.2 | 028, 029, 040 | 3 |
| T10 Agent robustness | 3.3 | 014 to 018 | 3 |
| T11 Worker concurrency | 3.4 | 023 | 3 |
| T12 Release integrity | 3.5 | 013 | 3 |
| T13 Container hardening | 3.6 | 034, 037 | 3 |
| T14 Maintenance expiry | 3.7 | 025 | 3 |
| T15 Contract refresh | 3.8 | 038 | 3 |
| T16 Metrics and self-monitoring | 4.1 | 041 | 4 |
| T17 Load, chaos, soak | 4.2 | 042 | 4 |

Not scheduled (Later): ORB-032 full (multi-user auth), ORB-043 code consolidation.

## 10. Risks

| Risk | Mitigation |
|---|---|
| SQLite single-writer ceiling hit before targets | Measure in 0.4 and 4.2; if exceeded, write-batching first, Postgres as a separate decision, not part of this plan |
| Breaking auth/enrollment defaults lock users out | Two-release rollout (section 7), clear startup errors |
| Sweeper creates alert storms on a Core restart | Grace window after Core boot before the first sweep; per-agent dedupe |
| Outbox migration with existing active incidents | Backfill pending deliveries only for incidents without an `incident_opened` row; dry-run test on a copy of production data |
| Plan too large for one thread | Phases 1 and 2 are independent; ship each ticket as its own PR |
