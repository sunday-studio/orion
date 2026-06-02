# App Code Breakdown Plan

## Goal

Bring `apps/agent/`, `apps/core/`, and `apps/console/` under a 500-line source file limit without changing product behavior.

The limit applies to product and test source files. It excludes docs, configuration files, generated SDK/OpenAPI/Swagger output, built web assets, public assets, and database migrations.

## Enforcement

Use `make code-line-limit` as the repository gate for app source file size.

The Makefile owns the executable check so agents and humans have one command to run before handoff. `AGENTS.md` owns the working rule so future changes split files by responsibility before they grow past 500 lines.

## Breakdown Principles

- Split by responsibility, not by arbitrary line ranges.
- Keep route registration, request parsing, response projection, persistence, and business rules in separate files when a package already has those concepts.
- Keep generated or contract-derived files out of manual cleanup unless the source generator changes.
- Keep tests grouped by behavior so failures still point to a product concept.
- Avoid changing public API contracts as part of line-count cleanup unless the migration ticket explicitly calls for a contract change.

## Current Oversized App Files

These files exceeded 500 lines when this branch started:

```txt
2495 apps/console/src/features/status-pages/status-pages.view.tsx
2163 apps/core/internal/api/status_pages_test.go
1540 apps/core/scripts/seed-demo-data/main.go
1358 apps/core/internal/worker/app_test.go
1327 apps/core/internal/api/alerts.go
1284 apps/core/internal/service/incident-service.go
1167 apps/console/src/features/incidents/incident-detail.view.tsx
1138 apps/core/internal/api/agent_monitor_api_test.go
1131 apps/core/internal/service/alert-service.go
1130 apps/core/internal/api/incident_api_test.go
1070 apps/core/internal/api/core_monitor_api_test.go
1007 apps/console/src/features/monitors/components/core-monitor-dialog.tsx
1001 apps/agent/internal/cli/commands.go
 979 apps/core/internal/api/incident.go
 955 apps/core/internal/api/responses.go
 847 apps/console/src/features/alerts/components/alert-rules-tab.tsx
 826 apps/core/internal/api/alert_api_test.go
 802 apps/console/src/features/monitor-detail/monitor-detail.view.tsx
 800 apps/core/internal/service/alert-service_test.go
 765 apps/core/internal/monitorvalidation/core-monitor-validation.go
 718 apps/console/e2e/console-smoke.spec.ts
 707 apps/core/internal/api/status_page_public_html.go
 696 apps/core/internal/api/integration_test.go
 681 apps/core/internal/service/agent-service.go
 638 apps/core/internal/service/core-monitor-management-service.go
 632 apps/console/src/features/settings/settings.view.tsx
 619 apps/core/internal/api/status_page_subscribers_test.go
 613 apps/core/internal/api/status_page_subscribers.go
 610 apps/core/internal/api/agent.go
 594 apps/core/internal/worker/synthetic_runner.go
 581 apps/core/internal/worker/mail_runner.go
 571 apps/core/internal/api/status_page_history.go
 563 apps/core/internal/api/status_pages.go
 562 apps/core/internal/service/health-service_test.go
 555 apps/core/internal/api/status_page_components.go
 552 apps/agent/internal/cli/logs.go
 533 apps/core/internal/db/models.go
 532 apps/core/internal/api/status_page_admin.go
 514 apps/core/internal/api/status_page_incidents.go
 510 apps/agent/internal/state/store.go
 505 apps/core/internal/worker/playwright_runner.go
 502 apps/agent/internal/agent.go
```

## Core Migration

Split API files by route family and projection boundary. Start with test files that already imply product areas, then split handler files into admin, public, subscriber, incident, component, and history responsibilities.

Split service files by business responsibility. Keep reconciliation, delivery, persistence, and projection behavior in separate files when those responsibilities already exist inside the package.

Split worker files by app orchestration, runner execution, result projection, and error classification.

Split `models.go` by domain model family so API and service work has smaller persistence landing zones.

## Console Migration

Split large views into a page shell, workflow sections, data hooks, form components, and shared empty/error states.

For status pages, separate editor tabs, public preview, component mapping, subscriber controls, and publish validation before changing behavior.

For monitor and incident detail surfaces, keep the current page route stable and extract panels around visible operator workflows.

## Agent Migration

Split CLI files by command group before touching runtime flow, because command boundaries are explicit and testable.

Split state storage by persistence concern: database opening/migration, agent identity, report checkpoints, retry queue state, and transactions.

Split runtime files by orchestration, reporting, spooling, maintenance state, and shutdown behavior.

### Started On This Branch

`apps/agent/internal/agent.go` has been split into runtime orchestration and reporting/spool responsibilities. Runtime orchestration stays in `agent.go`; system reporting, monitor reporting, durable spool flushing, and metric counting live in `apps/agent/internal/reporting.go`.

## Migration Order

1. Land the Makefile gate and this migration map.
2. Start with clean Agent files to prove the split pattern without fighting Core and Console churn.
3. Split generated-adjacent response and model files before deeper Core handler work.
4. Split tests before behavior-heavy files when one test file covers several product surfaces.
5. Split Console views by visible workflow and extract data hooks only when they remove repeated state logic.
6. Run `make code-line-limit` after each area and the relevant test or build command for the app being changed.

## Migration Status

The app source line-limit migration is complete on this branch. `make code-line-limit` passes with all app source files at or below 500 lines.

Verification completed:

- Agent tests: `GOCACHE=/private/tmp/orion-go-cache go test ./...` from `apps/agent`.
- Core targeted tests: `GOCACHE=/private/tmp/orion-go-cache go test ./internal/api ./internal/service ./internal/worker ./internal/db ./internal/monitorvalidation ./scripts/seed-demo-data` from `apps/core`.
- Console type check: `pnpm exec tsc --noEmit` from `apps/console`.

Known follow-up outside the line-limit migration: `pnpm run build` still fails on existing SDK/type-contract drift in incident and status-page features.
