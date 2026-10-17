# Backend Hardening Baseline

Recorded 2026-10-17 on branch `claude/hungry-archimedes-61fc59` after WP 0.1 and Phase 1 first slice. "Before" audit numbers (main @ `810ac82`, 2026-10-14) are from [backend-gaps-and-improvements.md](../architecture/backend-gaps-and-improvements.md).

| Metric | Before (audit, main @ 810ac82) | Now |
|---|---|---|
| Core coverage, `-coverpkg=./internal/...` | 71.6% | 72.0% |
| Core coverage, per-package: api / db / service | 68.8 / 50.8 / 42.9 | 69.0 / 62.0 / 47.4 |
| Agent coverage (registration pkg) | 0 (no tests) | 3.5 |
| CI gates | test, coverage, build | + vet, race, govulncheck, golangci-lint, route-vs-annotation test, pinned swag, coverage floor |

Ingest latency (p50/p95/p99), `SQLITE_BUSY` counts and RSS are produced by the load harness (T17, `apps/core/scripts/loadtest`); the same harness is run against base commit `810ac82` to fill the "before" column in `backend-hardening-load-results.md`.

Coverage floors enforced in CI: Core 70% with `-coverpkg` (72.0% at the first snapshot, 73.4% now). The plan's Agent target is 60%; the Agent total is **52% today** (was 41.5% in the audit), so CI enforces a 50% floor and the 60% target is **not yet met**. The remaining gap is mostly `internal/cli` (service manager, doctor, logs and update commands, about 33% covered), which needs OS-specific fixtures.
