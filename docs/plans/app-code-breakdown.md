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

These files currently exceed 500 lines under the app source rule:

```txt
5546 apps/core/internal/api/integration_test.go
2581 apps/core/internal/api/status_pages.go
1910 apps/console/src/features/status-pages/status-pages.view.tsx
1582 apps/core/internal/api/alerts.go
1510 apps/core/scripts/seed-demo-data/main.go
1509 apps/core/internal/api/status_pages_test.go
1370 apps/core/internal/service/alert-service.go
1358 apps/core/internal/worker/app_test.go
1284 apps/core/internal/service/incident-service.go
1268 apps/core/internal/service/alert-service_test.go
1001 apps/agent/internal/cli/commands.go
 900 apps/core/internal/api/responses.go
 859 apps/core/internal/api/incident.go
 830 apps/console/src/features/monitors/components/core-monitor-dialog.tsx
 804 apps/console/src/features/incidents/incident-detail.view.tsx
 801 apps/console/src/features/monitor-detail/monitor-detail.view.tsx
 779 apps/console/src/features/alerts/alerts.view.tsx
 765 apps/core/internal/monitorvalidation/core-monitor-validation.go
 681 apps/core/internal/service/agent-service.go
 638 apps/core/internal/service/core-monitor-management-service.go
 610 apps/core/internal/api/agent.go
 606 apps/core/internal/api/status_page_subscribers.go
 594 apps/core/internal/worker/synthetic_runner.go
 581 apps/core/internal/worker/mail_runner.go
 563 apps/core/internal/db/models.go
 562 apps/core/internal/service/health-service_test.go
 552 apps/agent/internal/cli/logs.go
 534 apps/core/internal/api/status_page_history.go
 510 apps/agent/internal/state/store.go
 505 apps/core/internal/worker/playwright_runner.go
 502 apps/agent/internal/agent.go
```

## Core Migration

### API Package

Split `apps/core/internal/api/integration_test.go` by product surface and helper role:

- auth and setup helpers
- agent and monitor report flows
- incident lifecycle flows
- alert flows
- status page flows
- settings and data lifecycle flows
- shared assertions and fixture builders

Split `status_pages.go` into admin CRUD, public projection, component mapping, publication validation, theme handling, and route registration files. Keep public DTO construction near the projection code and keep route annotations on the handlers that own the route.

Split `alerts.go` into rule management, destination management, route grouping, delivery inspection, test-send handling, and response projection files.

Split `responses.go` into files named for response families: agents, monitors, incidents, alerts, status pages, settings, diagnostics, and common primitives.

Split `incident.go` into lifecycle actions, list/detail read paths, timeline/event projection, and status-page impact integration.

Split `agent.go`, `status_page_subscribers.go`, and `status_page_history.go` by endpoint group while keeping shared request validation local to the package.

### Service Package

Split `alert-service.go` into rule evaluation, channel resolution, delivery queueing, templates, and delivery attempt persistence.

Split `incident-service.go` into reconciliation, manual lifecycle actions, event recording, and component impact updates.

Split `agent-service.go` into registration, heartbeat/report ingestion, token handling, and monitor ownership lookups.

Split `core-monitor-management-service.go` into config persistence, validation coordination, test execution, and scheduling side effects.

Split `health-service_test.go` by health state scenario instead of by helper size.

### Worker Package

Split `app_test.go` by worker scheduling, leasing, runner dispatch, diagnostics, and failure recovery.

Split `synthetic_runner.go`, `mail_runner.go`, and `playwright_runner.go` by runner configuration, execution, result projection, and error classification. If Playwright support is being removed, remove the runner before splitting it.

### Data Package And Scripts

Split `models.go` into model files by domain: agents, monitors, incidents, alerts, status pages, settings, diagnostics, and audit.

Split `scripts/seed-demo-data/main.go` into scenario builders, persistence helpers, and the command entrypoint.

## Console Migration

Split `status-pages.view.tsx` into page shell, editor tabs, public preview, component mapping, subscriber controls, publish validation, and form hooks.

Split `core-monitor-dialog.tsx` into dialog shell, monitor type forms, validation error rendering, defaults/builders, and submit orchestration.

Split `incident-detail.view.tsx` into page shell, lifecycle action panel, impact/component panel, evidence timeline, notification context, and data hooks.

Split `monitor-detail.view.tsx` into page shell, current state summary, history tables, incident context, runner diagnostics, and result explanation components.

Split `alerts.view.tsx` into page shell, tab composition, rule data hooks, destination data hooks, and shared empty/error states.

## Agent Migration

Split `internal/cli/commands.go` into root command setup, run command, register command, service lifecycle commands, config commands, and shared command helpers.

Split `internal/cli/logs.go` into log command setup, local file readers, follow/tail behavior, and formatting.

Split `internal/state/store.go` into database opening/migration, agent identity state, report checkpoints, retry queue state, and transaction helpers.

Split `internal/agent.go` into configuration loading, registration, collection loop orchestration, report submission, retry handling, and shutdown behavior.

## Migration Order

1. Split generated-adjacent response and model files first so later API and service work has smaller landing zones.
2. Split tests before behavior-heavy code when test files hide multiple product surfaces.
3. Split Core API handlers by route family before splitting their services.
4. Split Console views by visible workflow and move data orchestration into hooks only when it reduces repeated state logic.
5. Split Agent CLI files before runtime files because command boundaries are already explicit.
6. Run `make code-line-limit` after each area and the relevant test/build command for that app.
