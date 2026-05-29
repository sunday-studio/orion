# Monitor Readiness Reconciliation - 2026-05-29

## Goal

Reconcile Maat monitor planning rows with the repository state so active tickets do not imply unstarted work that is already implemented, documented, or intentionally deferred.

## Audit Scope

This audit covered active Maat tickets that directly describe Core-managed monitors, heartbeat monitors, Core monitor noise controls, monitor catalog runners, Console monitor workflows, worker diagnostics, and Playwright monitor planning.

## Shipped Or Documented

These ticket groups are no longer open implementation work:

| Area | Maat rows | Evidence |
|---|---|---|
| Core-managed monitor planning | `T-20260526-215303-4c65`, `T-20260527-105659-e52b`, `T-20260527-111350-220a` | `docs/plans/core-managed-monitors.md` records product scope, worker architecture, milestones, Maat row mappings, and review decisions. |
| Monitor coordination rows | `T-20260527-122246-1e31`, `T-20260527-181943-c51c`, `T-20260527-182935-9802`, `T-20260527-184114-1e6b`, `T-20260527-184831-3984`, `T-20260527-185233-118b` | The resulting milestone goals, worker branches, integrated code, split runner files, M6 milestone evidence, and this reconciliation close out the coordination layer that created and integrated the work. |
| Ownership, scheduler, HTTP runner, reports, diagnostics, deployment | `T-20260526-215548-e89f`, `T-20260526-215555-1883`, `T-20260526-215601-1319`, `T-20260527-111641-85ec`, `T-20260527-111653-3a22`, `T-20260527-111705-6c56`, `T-20260527-111715-8985`, `T-20260527-111724-8c57`, `T-20260527-111737-edb2` | `docs/agent-core-contract.md`, `apps/core/cmd/worker/main.go`, `apps/core/internal/service/core-monitor-scheduler-service.go`, `apps/core/internal/worker/http_runner.go`, `apps/core/internal/api/core_monitor_management.go`, `apps/core/internal/service/worker-diagnostics-service.go`, `deploy/docker-compose.yml`, and focused Core tests. |
| Console Core monitor workflow | `T-20260526-215607-6bf1`, `T-20260527-111749-59d7`, `T-20260527-111759-a920`, `T-20260527-111824-6353`, `T-20260527-111835-b813`, `T-20260527-111846-2d78` | `apps/console/src/features/monitors/monitors.view.tsx`, `apps/console/src/features/monitors/components/core-monitor-dialog.tsx`, `apps/console/src/features/monitors/components/monitor-list.tsx`, `apps/console/src/features/monitor-detail/monitor-detail.view.tsx`, `apps/core/internal/api/core_monitor_management.go`, and Console smoke coverage for Core monitor create/test/pause/resume. |
| Heartbeats | `T-20260526-215619-f244`, `T-20260527-111856-87f6`, `T-20260527-111907-416e`, `T-20260527-111918-7ad7`, `T-20260527-111931-a4dc` | `apps/core/internal/api/core_monitor_heartbeat.go`, `apps/core/internal/worker/app.go`, `apps/core/internal/worker/app_test.go`, `apps/core/internal/api/integration_test.go`, `apps/console/src/features/monitors/components/heartbeat-setup-panel.tsx`, and monitor detail heartbeat evidence rendering. |
| Noise controls | `T-20260526-215627-b569`, `T-20260527-111943-73e5`, `T-20260527-111956-14f8`, `T-20260527-112007-e8ac`, `T-20260527-112022-51a5`, `T-20260527-112035-ca71` | Confirmation, recovery, maintenance, severity, and flapping behavior are covered in `apps/core/internal/api/integration_test.go` and `apps/core/internal/service/health-service_test.go`; configuration fields are surfaced in Core monitor API and Console detail views. |
| Status page component mapping groundwork | `T-20260526-215635-f363`, `T-20260527-112050-0cfb`, `T-20260527-112103-2087`, `T-20260527-112118-7764`, `T-20260527-112139-910e` | `docs/architecture/status-pages.md`, `apps/core/internal/db/migrations/000011_status_pages.up.sql`, `apps/core/internal/api/status_pages.go`, and Console status page component mapping UI. |
| M6 worker catalog | `T-20260527-112154-d351`, `T-20260527-112209-1875`, `T-20260527-112220-3115`, `T-20260527-112232-63ef`, `T-20260527-112242-b01d`, `T-20260527-112250-8206`, `T-20260527-112259-4dfc`, `T-20260527-112313-2c00`, `T-20260527-112337-7951`, `T-20260527-112346-811e`, `T-20260527-112401-3ba3`, `T-20260527-112412-1f05` | `apps/core/internal/worker/*_runner.go`, matching runner tests, `docs/milestones/m6-core-monitor-catalog.md`, and `docs/plans/core-managed-monitors.md`. |
| Monitor target policy, validation, and runtime packaging hardening | `T-20260528-033324-7aae`, `T-20260528-033343-7b3c`, `T-20260528-033343-a2d9`, `T-20260528-033357-f576` | `apps/core/internal/service/core-monitor-target-policy.go`, type-specific validation in `apps/core/internal/service/core-monitor-management-service.go`, Playwright runtime docs in `docs/deployment/core-docker.md`, and WHOIS fallback tests in `apps/core/internal/worker/domain_expiration_runner_test.go`. |

## Still Active

These rows should remain active because they represent current readiness work rather than stale planning:

| Area | Maat rows | Reason |
|---|---|---|
| Scope honesty and Console type coverage | `G-20260529-191440-d81b`, `G-20260529-191451-5e80`, `G-20260529-191503-bee9` and their tickets | The backend runner catalog is broader than first-release Console creation affordances, so the product needs explicit scope copy, type-specific forms, validation feedback, and detail summaries. |
| Worker health UX | `G-20260529-191509-e97e` and its tickets | Core exposes worker diagnostics, but the release still needs an operator-facing Console panel and clear absent-worker warnings. |
| Monitor security | `G-20260529-191516-ddf7` and its tickets | Target policy and exposed deployment auth posture are still being hardened as release gates. |
| E2E monitor coverage | `G-20260529-191529-d379` and its tickets | Browser coverage for create/test history, incidents, and heartbeats is still active validation work. |
| Playwright removal | `G-20260529-191523-aad2` and its tickets | Main still contains Playwright worker, API, Console, and docs support; first-release readiness has moved to removing that surface rather than expanding it. |

## Maat Cleanup

Complete shipped rows with evidence that cites the repository files above, this audit file, and successful validation. Deferred or superseded rows should receive comments pointing to the active goal that now owns the work instead of being marked done.

## Verification

- `maat validate --storage /Users/casprine/Desktop/vendor/personal/maat-storage`
- `git diff --check`
