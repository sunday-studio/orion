# Implementation handoff for the web thread

Read `design-language.md` (rules and tokens) first, then this file. The canvas is https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY; artboard names below match its file names (`<name>.dc.html`). `screen-map.md` numbers screens; this file maps them to code and API. API items marked "approved" are covered by D-004; items marked "proposed" need a Core route, OpenAPI annotations and `docs/agent-core-contract.md` review before the UI can ship, so build the screen against a typed stub and a feature flag.

## 1. Build order

1. **Foundation.** Tokens and type in `index.css` (TWK Lausanne, tabular figures, color and radius tokens from design-language sections 4 to 6). New header with the tray, account menu and health chip. Restyle `status-badges`, `page-header`, `tabs`, `dialog`, `empty-state`, `data-table` (one-line rows, 38px, dense 30px).
2. **Shared pieces.** `FilterBar` and `FilterToken` over nuqs, `ListRow`, drawer, callout, chips input, token reveal box, histogram, facet rail. Keep AGENTS.md console table rules (TanStack Table, nuqs, server-side filtering).
3. **Restyle existing screens** with no API change: Incidents, Servers, Monitors, Alerts log, Status pages, Settings.
4. **Frontend-only new screens:** Overview (use `/v1/health/summary`, `/v1/health/issues`, `/v1/incidents`), shortcuts sheet, account menu, command palette, empty and error states, avatar menu theme and timezone.
5. **API-backed new screens** in the order of section 3.

## 2. Screen to code map

| Artboard(s) | Route | Console file | API |
|---|---|---|---|
| login, session-expired | `/login` | `features/auth/login.view.tsx` | existing |
| Main (Overview), overview-all-good, onboarding, dashboard-dark | `/` (D-003) | new `features/overview/` | `/v1/health/summary`, `/v1/health/issues`, `/v1/incidents` |
| health-issues | `/health` or panel | new | `/v1/health/issues` |
| incidents-list, incidents-bulk, incidents-empty | `/incidents` | `features/incidents/` | existing; bulk proposed |
| incident-detail, incident-resolved, incident-actions, incident-public-update | `/incidents/:id` | `features/incidents/incident-detail.view.tsx` | existing |
| incident-declare | dialog | `features/incidents/` | `POST /v1/incidents` proposed |
| incident-candidates | `/incidents/triage` | new | `GET /v1/incidents/candidates`; dismiss approved |
| servers-list, servers-empty | `/servers` | `features/agents/` | existing |
| server-detail, server-monitors, server-metrics, server-stale, server-token, server-token-new | `/servers/:id` (tabs) | `features/agent-detail/` | existing; filter monitors by agent proposed |
| server-edit, server-decommission | dialogs | `features/agent-detail/` | `PATCH /v1/agents/{id}` (name, labels, notes) and `keep_history` on delete proposed |
| add-server, add-server-connected, add-server-failed | `/servers/new` | new | `POST /v1/register`, poll `/v1/agents` |
| monitors-list, monitors-bulk, monitors-empty | `/monitors` | `features/monitors/` | existing; bulk and clone proposed |
| monitor-create, monitor-create-tcp, heartbeat-setup, monitor-agent-setup | dialogs and panels | `features/monitors/` | existing |
| monitor-detail, monitor-paused, monitor-edit, monitor-test, monitor-maintenance, report-drawer, delete-confirm | `/monitors/:id` | `features/monitor-detail/` | existing; run test result approved |
| alerts-log, delivery-detail | `/alerts` | `features/alerts/` | existing; retry delivery approved |
| alerts-channels, channel-dialog, channel-webhook, channel-email, channel-test, channel-detail | `/alerts/channels` | `features/alerts/` | channels exist; email type, recipient confirmation, secret regenerate, disable proposed |
| email-confirm | `/alerts/email/confirm/:token` (public) | Core-served page | proposed |
| alerts-rules, alerts-routes | `/alerts/rules`, `/alerts/routes` | `features/alerts/` | existing (`/alerts/rules`, `/alerts/routes`) |
| status-pages-list, status-create | `/status-pages` | `features/status-pages/` | existing |
| status-setup, status-components, status-incidents, status-subscribers, status-preview | `/status-pages/:id` (tabs) | `features/status-pages/` | existing |
| status-domain, status-maintenance, status-publish | dialogs and panel | `features/status-pages/` | domain verify and maintenance windows proposed; publish exists |
| public-* , status-badge | `/status/:slug/*` | Core-served | existing JSON and badge API; HTML is Core's |
| logs-events (status logs), logs-service, logs-app, logs-app-detail, logs-app-empty | `/logs` (tabs) | `features/event-log/logs.view.tsx` | `/v1/events`, `/v1/logs/service`; query, cursor and facets proposed |
| logs-app-connect, logs-sources | `/logs/sources` | new | source keys CRUD and ingest stats proposed |
| logs-live | `/logs` (view) | same | `GET /v1/events/stream` (SSE) proposed |
| logs-patterns | `/logs` (view) | same | `GET /v1/events/patterns` proposed |
| logs-detail | drawer | same | `/v1/events/{id}` and surrounding events proposed |
| logs-views | menus | same | saved views in localStorage first |
| logs-export | dialog | same | `POST /v1/events/export` (async) proposed |
| settings-lifecycle, settings-system | `/settings/*` | `features/settings/` | existing |
| settings-general | `/settings/general` | same | instance name, public URL, timezone proposed |
| settings-tokens, settings-token-new | `/settings/security` | same | API token CRUD proposed |
| settings-audit | `/settings/audit` | same | `GET /v1/audit` proposed |
| command-palette, shortcuts, avatar-menu | global | `components/shared/` | client side; search can start on loaded data |
| states-gallery | global | `components/shared/` | none |

## 3. API work for the backend thread

Approved (D-004): retry delivery (`POST /v1/alerts/deliveries/{id}/retry`), dismiss incident candidate (`POST /v1/incidents/candidates/{id}/dismiss`), run monitor test with a richer result (status, timings, error text).

Proposed, grouped by value:

1. App log ingest: `POST /v1/ingest/otlp` (OpenTelemetry logs over HTTP; read service.name, severity, body, trace_id, span_id, resource attributes), `POST /v1/ingest/json`, per-source ingest keys (create, revoke, list with last received, rate, dropped), retention and per-source rate limits. Status logs filter by `server_id` and `monitor_id`; service logs by `component`. Link app log lines to events by server and time window.
2. Logs at scale: `q` (field:value query), `level`, `source`, `from`, `to`, `cursor` on events; `/v1/events/histogram`; `/v1/events/stream` (SSE); `/v1/events/patterns`; async export. These make the Logs redesign real and are the largest item.
3. Channels: `type = webhook | email`; email recipients with a confirmation token and resend; public confirmation endpoint; webhook signing secret (HMAC SHA-256, `X-Orion-Signature`) with regenerate; custom headers; disable flag.
4. Servers: `PATCH` name, labels, notes; list monitors by agent with a status filter; delete with `keep_history`.
5. Incidents and monitors: declare incident; bulk acknowledge, resolve, cover; bulk pause, maintenance, clone, delete; `runbook_url` on monitors; TLS expiry.
6. Status pages: custom domain verification, maintenance windows (hold alerts for mapped components).
7. Settings: general (instance name, public URL, timezone), API tokens, audit log.

## 4. Copy and content rules

- Sentence case everywhere. Buttons are verbs: "Create monitor", "Send test", "Retry delivery".
- Destructive dialogs name the object and the count of what is lost ("4 monitors on this server will be deleted").
- Empty states give one sentence and one button.
- Sample data on the canvas (edge-01, api-https, INC-0231) is illustrative; use real data in the app.

## 5. Not designed

Mobile and narrow layouts, print views, password reset (done through an environment variable on the Core host), multi-user roles (the Console has one admin).

## 6. Verification

Compare each screen to its artboard at 1280px width. Check: one-line rows, centered column widths (1040 and 760), tab rule width, badge alignment, tabular numbers in every column, focus states, and the empty, loading, error and offline states in the System states artboard.
