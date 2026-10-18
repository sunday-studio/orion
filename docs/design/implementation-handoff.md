# Implementation handoff for the web thread

Read `design-language.md` (rules and tokens) first, then this file. The canvas is https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY; artboard names below match its file names (`<name>.dc.html`). `screen-map.md` numbers screens; this file maps them to code and API. API items marked "approved" are covered by D-004; items marked "proposed" need a Core route, OpenAPI annotations and `docs/agent-core-contract.md` review before the UI can ship, so build the screen against a typed stub and a feature flag.

**Artboard exports (build 1:1 from these).** Every screen now has a light and a dark artboard (`<name>.html` and `<name>-dark.html`), grouped by flow on the canvas. Every canvas artboard is exported to `docs/design/artboards/`: `<name>.html` (self-contained, shared `orion.css`), `png/<name>-light.png` and `-dark.png` at 1280px, and `README.md` with the artboard to screen-map to route to Console file index. Artboard names in the table below match those files. Compare each screen to its PNG during review.

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
| Main (Overview), overview-all-good, onboarding | `/` (D-003) | new `features/overview/` | `/v1/health/summary`, `/v1/health/issues`, `/v1/incidents` |
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
| logs, logs-rich, logs-plain, logs-no-apps, logs-no-results, logs-error | `/logs` | `features/event-log/` rebuilt as the unified product logs page (tab Service logs) | L2 (D-011): `GET /v1/logs` (filters, FTS `q`, cursor), `/v1/logs/histogram`, `/v1/logs/facets`, `/v1/logs/fields` (field catalog) |
| logs-detail, logs-detail-plain | drawer | same | `GET /v1/logs/{id}`; related logs by `request_id` or `trace_id` (L2) |
| logs-live | `/logs` live | same | `GET /v1/logs/stream` SSE (L3) |
| logs-views | menus | same | saved views in localStorage first, `/v1/log-views` later (L3) |
| logs-export | dialog | same | `POST /v1/logs/export` async, proposed (D-009) |
| logs-patterns | `/logs` patterns view | same | `GET /v1/logs/patterns`, proposed (D-009) |
| logs-events, logs-service | `/logs/events`, `/logs/service` | same | existing events and Orion service logs; query and cursor proposed (D-009) |
| settings-apps, settings-app-new | `/settings/apps` | `features/settings/` | `GET/POST /v1/apps` (L1) |
| app-keys, app-key-new | `/settings/apps/:id` | same | `/v1/apps/{id}/keys` create (shown once), rotate, revoke (L1) |
| app-retention | same | same | `PATCH /v1/apps/{id}`: retention days, size cap, rate limit, debug sample rate; purge (L1) |
| app-display | same | same | `PATCH /v1/apps/{id}` display profile (L3) |
| app-redaction | same | same | `PATCH /v1/apps/{id}` rules; `POST /v1/apps/{id}/redaction/test` (L1) |
| app-send-test | same | same | `POST /v1/apps/{id}/test-log`; receipt shown from the logs list (L1) |
| cli-install, cli-setup, cli-doctor, cli-status, cli-logs, cli-help-error | terminal (`apps/agent`) | `apps/agent` CLI | plan A2 to A6, C1 to C4: Charm theme generated from tokens, control socket, `--json` schemas |
| add-server, add-server-waiting, add-server-connected, add-server-failed | `/servers/new` | new | one-time enrollment token and agent-connected event (backend ORB-009) |
| app-connect-create, app-connect-snippet, app-connect-waiting, app-connect-received | `/apps/new` (linked from the Overview card, D-006) | new | same app and key endpoints; waiting step polls `last_log_at` on `GET /v1/apps/{id}` |
| settings-lifecycle, settings-system | `/settings/*` | `features/settings/` | existing |
| settings-general | `/settings/general` | same | instance name, public URL, timezone proposed |
| settings-tokens, settings-token-new | `/settings/security` | same | API token CRUD proposed |
| settings-audit | `/settings/audit` | same | `GET /v1/audit` proposed |
| command-palette, shortcuts, avatar-menu | global | `components/shared/` | client side; search can start on loaded data |
| states-gallery | global | `components/shared/` | none |

## 3. API work for the backend thread

Approved (D-004): retry delivery (`POST /v1/alerts/deliveries/{id}/retry`), dismiss incident candidate (`POST /v1/incidents/candidates/{id}/dismiss`), run monitor test with a richer result (status, timings, error text).

Proposed, grouped by value:

1. Product logs: follow `docs/plans/product-logs-plan.md` (L0 to L4). Console needs: ingest keys per App, field catalog, facets, FTS search, SSE tail, redaction test endpoint, per-app limits. Events filter by `server_id` and `monitor_id`; Orion service logs by `component`.
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

## 7. Feature 1 (Entry and home): data and API notes

Checked against `apps/core/openapi.yaml` on 2026-10-18 after the F1 APIs landed.

**Overview** (`Main`, `overview-all-good`, `Main-dark`, `onboarding`): one call, `GET /v1/overview`.
- Greeting uses `GET /v1/auth/me`. Header health chip: `health_issue_count` and `state` (empty, ok, issues).
- Stat cards: Monitors up = `stats.monitors_up` of `monitors_total`; Servers reporting = `servers_reporting` of `servers_total`; Uptime, 30 days = `uptime_30d_percent`, which is null until a daily rollup exists, so show "No data yet" in that card until then.
- Needs you now: `needs_you` (`title`, `severity`, `status`, `summary`, `opened_at`, `notified_at`, `acknowledged_at`, `duration_seconds`). Tracker: Detected = `opened_at`, Notified = `notified_at`, Acknowledged = `acknowledged_at`, Resolved = `resolved_at`. The summary line is the incident `summary`. Acknowledge calls `POST /v1/incidents/{id}/acknowledge`.
- Server health: `services` (the artboard calls it Server health): per server, worst first, at most 8, with `status`, `uptime_percent` and a 90 day strip of `days` (`up`, `warn`, `down`, `none`).
- Ongoing incidents: `ongoing_incidents`, `ongoing_incident_count`. Recent incidents: `recent_incidents`, duration from `duration_seconds`.
- States: `state = empty` and `first_run` flags drive the first-run screen (all four flags false, with the dismissible card, D-006); `state = ok` shows the all-good screen; `state = issues` shows Needs you now.

**Sign in** (`login`): `POST /v1/auth/login`. Error callout text is the API message. No sign-up.

**Add server** (`add-server`, `add-server-waiting`, `add-server-connected`, `add-server-failed`; D-028)
- Platform tiles: Linux and macOS only (Windows is out of scope, D-014; running the agent as a container is out until the last item before go-live, D-031). `POST /v1/enrollments` with the tile as `platform` returns a one-time `token`, `install_command` and `expires_at` (15 minutes). The token is shown only in the command.
- The waiting screen has a static list labelled "The command will:" (download and verify, create user and directories, install and start the service, register, run doctor). It never ticks, because Core cannot observe those steps.
- Live states are only three, from polling `GET /v1/enrollments/{id}`: waiting (`waiting`, `claimed`, `registered`), connected (`reporting`, with `agent_name`, `agent_os`, `agent_arch`, `agent_version`, `first_report_at`), not connected (`expired`, or no report 5 minutes after the command was shown).
- The agent token goes to the agent, not the Console. There is no shown-once token panel on any add-server artboard.

**Connect an app** (`app-connect-create`, `-snippet`, `-waiting`, `-received`): create `POST /v1/apps`; key `POST /v1/apps/{id}/keys` returns the secret once; snippets are client-side; waiting polls the app or the logs list for the first line; "Send a test log" is `POST /v1/apps/{id}/test-log`.

**Command palette**: `GET /v1/search?q=` (at most 5 hits per type, grouped monitor, incident, server, app) plus static actions. **Account menu**: theme and timezone stored in the browser until a settings endpoint exists; sign out clears the token. **Shortcuts**: client side.
