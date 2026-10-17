# Backend Hardening Baseline

Recorded 2026-10-17 on branch `claude/hungry-archimedes-61fc59` after WP 0.1 and Phase 1 first slice. "Before" audit numbers (main @ `810ac82`, 2026-10-14) are from [backend-gaps-and-improvements.md](../architecture/backend-gaps-and-improvements.md).

| Metric | Before (audit, main @ 810ac82) | Now |
|---|---|---|
| Core coverage, `-coverpkg=./internal/...` | 71.6% | 72.0% |
| Core coverage, per-package: api / db / service | 68.8 / 50.8 / 42.9 | 69.0 / 62.0 / 47.4 |
| Agent coverage (registration pkg) | 0 (no tests) | 3.5 |
| CI gates | test, coverage, build | + vet, race, govulncheck, golangci-lint, route-vs-annotation test, pinned swag, coverage floor |

Ingest latency (p50/p95/p99), `SQLITE_BUSY` counts and RSS are produced by the load harness (T17, `apps/core/scripts/loadtest`); the same harness is run against base commit `810ac82` to fill the "before" column in `backend-hardening-load-results.md`.

Coverage floors enforced in CI: Core 70% with `-coverpkg`, Agent 60% total is a target not yet met (agent total is below it today); the Agent floor is set to the current value and ratcheted up by T10.
