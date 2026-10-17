# Web Status Map (apps/console)

Read-only audit, 2026-10-14, branch `main` @ 810ac82. Evidence from source, `npm run build/lint/format:check`, `make code-line-limit`.

## 1. Route / page inventory
Router: `apps/console/src/router/app-router.tsx`. Layout: `components/shared/layout.tsx`, nav in `components/shared/app-header.tsx`.

| Route | Page file | Purpose |
|---|---|---|
| `/login` | `features/auth/login.view.tsx` | Username/password login via `authLogin` (`api/client.ts:139`) |
| `/` | redirect | to `/incidents` |
| `/incidents` | `features/incidents/incidents.view.tsx` -> `components/incident-list.tsx` | Incident list, summary cards |
| `/incidents/:incidentId` | `features/incidents/incident-detail.view.tsx` | Detail, timeline, lifecycle actions, status-page draft |
| `/monitors` | `features/monitors/monitors.view.tsx` | Monitor list/summary, create Core monitor dialog |
| `/monitors/:monitorId` | `features/monitor-detail/monitor-detail.view.tsx` | History, config, delete, operational data |
| `/servers` | `features/agents/agents.view.tsx` -> `agent-list.tsx` | Agent (server) list |
| `/servers/:agentId` | `features/agent-detail/agent-detail.view.tsx` | Health, CPU, monitors, report logs, service logs, token panel |
| `/agents`, `/agents/:agentId` | redirects | to `/servers...` |
| `/alerts` | `features/alerts/alerts.view.tsx` | Tabs: rules, webhook channels, notification log |
| `/status-pages` | `features/status-pages/status-pages.view.tsx` | Admin: pages, components, sections, incidents, subscribers, preview |
| `/logs` | `features/event-log/logs.view.tsx` | Orion event log |
| `/settings` | `features/settings/settings.view.tsx` | Data lifecycle/retention, archive, maintenance |

Not present: public status page route (`/status/:slug`) in the console, 404/catch-all route, client-side auth guard (only 401 handler redirects, `api/client.ts:108`; header only toggles on `getToken()`, `app-header.tsx:32`).

## 2. Per-feature status

| Feature | Status | Evidence |
|---|---|---|
| Auth (login/logout) | Partial | Login + logout (`app-header.tsx:35`) + 401 redirect work, e2e covers it (`e2e/console-smoke.spec.ts:6`). No route guard, no refresh, token in localStorage. |
| Monitors list/create/delete | Done | Real SDK; create Core monitor (HTTP, heartbeat) tested in `e2e/monitor-alert-smoke.spec.ts:13,157`; unsupported types rejected (`:73`). |
| Monitor detail | Done | `monitor-detail.view.tsx` uses `getMonitorHistory`, config, `deleteMonitor`. Edit/update flow not confirmed. |
| Incidents list + detail | Done | Real SDK, tabs/lifecycle tested (`e2e/status-incident-smoke.spec.ts:6,94`). `GET /v1/incidents/candidates` unused. |
| Agents (servers) list/detail | Done | Real SDK; token panel, CPU, logs tabs. Maintenance mode endpoint not used from UI. |
| Alerts (rules/webhooks/notif log) | Partial | UI uses `AlertRoutes` hooks (`alert-rules-tab.tsx:28,54`); OpenAPI now exposes both `/alerts/routes` and `/alerts/rules`. Alerts tables are hand-built, no nuqs. Webhooks tested (`monitor-alert-smoke.spec.ts:89`). |
| Event logs | Done | `event-log/logs.view.tsx` (474 lines), DataTable + nuqs. |
| Settings (data lifecycle) | Done | Real SDK; 4 e2e tests. `settings.view.tsx` is 496 lines (at limit). |
| Status pages (admin) | Partial | Large real-SDK admin UI (`status-pages.view.tsx` 500 lines, 14 components). Setup and subscribers tabs not tables-pattern. Public page rendering lives only in Core/preview panel. |
| Public status page | Missing in console | Core `/status/{slug}*` endpoints (15) unused by console; only preview panel. |
| Health issues / core diagnostics | Partial | `core-worker-diagnostics.tsx` exists but `GET /v1/diagnostics/core` and `/v1/health/issues` not called by name. |
| Report inspection drawer | Done | `features/report-inspection/components/report-inspection-drawer.tsx`. |
| Mock data | None found | grep for mock/stub/fake/TODO/FIXME in `src` (excl. SDK) returns nothing relevant. |
| Generated Core assets | Stale | `apps/core/web` last committed 2026-05-30; console source last 2026-08-02. Run `make build-static` before release. |
| `packages/sdk` | Not present | AGENTS.md references it, directory does not exist; SDK is `apps/console/src/orion-sdk/index.ts` (12,352 lines, orval). |

## 3. Data layer
- All pages use real API via orval-generated hooks (`src/orion-sdk/index.ts`) through `orvalFetchClient` (`src/api/client.ts`). Only hand-written call: `authLogin`. No mock data layer. No stray `fetch(` outside SDK/client.
- Hard-coded `/v1/...` strings only as query-key invalidations (`monitors.view.tsx:44`, `incident-detail.view.tsx:53`) - fragile vs generated keys.
- OpenAPI operations not referenced by console (76 of 117 used by name; remaining 41):
  - Agent-ingest, expected non-UI: `register`, `receiveAgentReport`, `receiveMonitorReport`, `receiveAgentLogBatch`, `registerMonitor`, `unregisterMonitor`, `heartbeats/{token}/success|failure`.
  - Public status API (15): `/status/{slug}` and badge/history/uptime/incidents/subscribers endpoints.
  - Alerts `rules` aliases (7, UI uses `routes` equivalents).
  - Candidate UI gaps: `GET /v1/incidents/candidates`, `PUT /v1/agents/{id}/maintenance`, `GET /v1/diagnostics/core`, `GET /v1/health/issues`, `GET /health`, status-page `list/update` for components, mappings, sections, incidents (UI may use alternate ops; verify).
- Console calls with no endpoint: none found (SDK is generated from the same spec; CI checks drift, `.github/workflows/ci.yml:108-113`).

## 4. Components & Console Table Rules
- UI primitives: `components/ui/*` (14, base-ui + cva). Shared: `components/shared/*` (data-table, list-pagination, empty-state, page-header, status-badges, uptime-sla, etc.).
- `components/shared/data-table.tsx` (132 lines) wraps TanStack Table; there is no OpenStatus-style schema-driven data-table (no filter/faceted/sort toolbar, no `useReactTable` in features - 0 direct uses).
- nuqs used in: agent-list, incident-list, monitor-list, logs, agent-detail logs/monitors/service-logs, alerts view, monitor-detail view. TanStack Query used on all data pages.

| Log-like table | File | nuqs URL state | Verdict |
|---|---|---|---|
| Orion event log | `event-log/logs.view.tsx` | yes | Conforms (partial: basic DataTable) |
| Agent report logs | `agent-detail/components/agent-logs-tab.tsx` | yes | Conforms (partial) |
| Agent service logs | `agent-detail/components/agent-service-logs-tab.tsx` | yes | Conforms (partial) |
| Monitor check logs | `monitor-detail/components/monitor-detail-operational-data.tsx` / `-shared.tsx` | via view only | Partial |
| Alert notification (delivery) log | `alerts/components/notification-log-tab.tsx` | **no** | Violates |
| Alert rules / webhooks tables | `alert-rules-tab.tsx`, `webhook-channels-tab.tsx` | **no** | Violates (config tables, lower priority) |
| Incident timeline/events | `incidents/components/incident-detail-utils.tsx`, `incident-detail.view.tsx` | **no** | Violates (timeline not a data-table, no URL state) |
| Agents monitor list | `agents/components/monitor-list.tsx` | **no** | Violates |
| Status page subscribers | `status-pages/components/status-pages-subscribers-tab.tsx` | **no**, no DataTable | Violates |

## 5. Code health
- Build `npm run build`: PASS (tsc -b + vite). Warning: main chunk 864.86 kB (254 kB gzip) > 500 kB; no code splitting.
- `npm run lint` (oxlint --deny-warnings): PASS, 0 warnings/errors.
- `npm run format:check` (oxfmt): PASS.
- `make code-line-limit`: PASS ("all app source files at or below 500 lines"). Near limit: `status-pages.view.tsx` 500, `settings.view.tsx` 496, `logs.view.tsx` 474, `monitor-detail.view.tsx` 427, `monitors/monitor-list.tsx` 401.
- Unit tests: none (no vitest/jest; no `*.test.*` files; no test script).
- Browser smoke (Playwright, `e2e/`, 677 lines, 14 tests in 4 specs): console-smoke (2), monitor-alert (4), settings (4), status-incident (4), using a real Core via `e2e/support/start-core-smoke.mjs`. Not run in this audit. CI (`ci.yml`) runs lint, format:check, build, contract drift; Playwright is NOT in CI (no `playwright` reference in workflows).
- Gaps in e2e: agents/server detail, event logs filters, alert rule editor, status page admin CRUD (components/sections/subscribers).

## 6. Prioritized gaps & recommended tickets
1. Add Playwright smoke to CI (console job) - tests exist but are unguarded.
2. Alerts notification-log, rules, webhooks tables: migrate to the data-table pattern with nuqs + server pagination; same for incident timeline, agents monitor-list, subscribers.
3. Build a real schema-driven data-table (toolbar, filters, sort, pagination) on `components/shared/data-table.tsx`; refactor event/agent/monitor logs onto it.
4. Add client-side auth guard (redirect to `/login` when no token) plus a 404 route.
5. Public status page route/UI in the console (or document Core-served page) and consume `/status/{slug}` endpoints; verify status-page component/section/mapping update ops.
6. Surface missing ops: agent maintenance mode, incident candidates, core diagnostics/health issues.
7. Split `status-pages.view.tsx` (500) and `settings.view.tsx` (496) before they grow; add route-level code splitting (864 kB bundle).
8. Replace hard-coded query-key strings with generated `get*QueryKey`.
9. Rebuild `apps/core/web` (`make build-static`) - stale by ~2 months; fix AGENTS.md reference to nonexistent `packages/sdk`.
10. Add unit tests (vitest) for domain files (`*.domain.ts`) and `api/client.ts`.
