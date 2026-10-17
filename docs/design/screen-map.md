# Orion Screen Map

Inventory of every Console screen: existing (E), partially present (P), and implied but missing (N).
Sources: `apps/console/src/router/app-router.tsx`, `apps/console/src/features/*`, `apps/core/openapi.yaml`, `docs/architecture/*`.

Terminology: the UI says "Servers" for what Core calls Agents (`/servers`, `/v1/agents`). Navigation is the top bar: Overview, Incidents, Servers, Monitors, Alerts, Status, Logs, Settings, with a command palette on `⌘K`.

## 1. Existing routes

| # | Route | Screen | State | Backing API |
|---|-------|--------|-------|-------------|
| 1 | `/login` | Sign in | E | `POST /v1/auth/login` |
| 2 | `/incidents` | Incidents list + summary cards | E | `GET /v1/incidents`, `/v1/health/summary` |
| 3 | `/incidents/:id` | Incident detail (timeline, ack/cover/resolve/reopen dialogs) | E | `/v1/incidents/{id}`, `/timeline`, actions |
| 4 | `/servers` | Servers (agents) list, summary, filters | E | `GET /v1/agents`, `/v1/agents/summary` |
| 5 | `/servers/:id` | Server detail, tabs: Server Reports, Service Logs, Monitors, System Metrics; token panel; health summary | E | `/v1/agents/{id}/*` |
| 6 | `/monitors` | Monitors list, summary, core-worker diagnostics, create dialog, heartbeat setup | E | `/v1/monitors`, `/v1/diagnostics/*` |
| 7 | `/monitors/:id` | Monitor detail: overview, uptime, history, operational data, report inspection drawer | E | `/v1/monitors/{id}/*` |
| 8 | `/alerts` | Alerts, tabs: Notification Log, Channels, Rules (flow node editor) | E | `/v1/alerts/*` |
| 9 | `/status-pages` | Status pages admin: sidebar, setup, components, incidents, subscribers, preview | E | `/v1/status-pages/*` |
| 10 | `/logs` | Logs, tabs: Operational Events, Service Logs | E | `/v1/events`, `/v1/logs/service` |
| 11 | `/settings` | Settings: data lifecycle (retention, rollup, archive) | E | `/v1/settings/data-lifecycle*` |
| 12 | `/agents`, `/agents/:id` | Redirects to `/servers` | E | none |

## 2. Overlays on existing screens (dialogs, drawers, panels)

| # | Overlay | Parent | State |
|---|---------|--------|-------|
| 13 | Create / edit monitor dialog (HTTP, TCP, heartbeat, agent monitor types) | Monitors | E |
| 14 | Heartbeat setup panel (ping URLs, curl snippet) | Monitors | E |
| 15 | Core worker diagnostics panel | Monitors | E |
| 16 | Report inspection drawer (raw report payload) | Monitor / Server detail | E |
| 17 | Incident action dialogs: acknowledge, cover, resolve, reopen | Incident detail | E |
| 18 | Channel dialogs, Webhook and Email only (Slack, Discord and Telegram are out of scope): type picker, webhook form (secret, headers), email form (confirmed recipients), delete, send test, test results | Alerts > Channels | E (to rework) |
| 19 | Alert rule editor dialog + flow node view + dry-run | Alerts > Rules | E |
| 20 | Alert route create/edit + dry-run | Alerts | P (API exists, UI folded into rules) |
| 21 | Server token panel: status, rotate, reissue, revoke | Server detail | E |
| 22 | Status page: create incident / edit incident / post update forms | Status > Incidents | E |
| 23 | Status page: incident suggestions (from internal incidents) | Status > Incidents | E |
| 24 | Status page: section + component + mapping forms | Status > Components | E |
| 25 | Status page: subscriber actions (disable, anonymize) | Status > Subscribers | E |
| 26 | Status page: settings form (name, slug, domain, branding) and publish/unpublish | Status > Setup | E |

## 3. Public (unauthenticated) screens served by Core

| # | Route | Screen | State |
|---|-------|--------|-------|
| 27 | `/status/:slug` | Public status page (overall, sections, components, active incidents) | P (JSON + badge API; HTML page to design) |
| 28 | `/status/:slug/history` | Public history / uptime bars | P |
| 29 | `/status/:slug/incidents` | Public incident list | P |
| 30 | `/status/:slug/incidents/:id` | Public incident detail with updates | P |
| 31 | `/status/:slug/subscribe` | Subscribe by email form | P (`POST /subscribers`) |
| 32 | `/status/:slug/subscribers/confirm/:token` | Subscription confirmed / expired | P |
| 33 | `/status/:slug/subscribers/manage/:token` | Manage preferences | P |
| 34 | `/status/:slug/subscribers/unsubscribe/:token` | Unsubscribe confirm | P |
| 35 | Status badge SVG (page and component) | Embeddable badge | E (API) |

## 4. Implied but missing

| # | Screen | Why it is needed | State |
|---|--------|------------------|-------|
| 36 | Onboarding: dismissible setup card on Overview, not a route (connect first server, create first monitor, add channel, publish status page). Dismissal is per browser; reopen from Settings, General | `docs/deployment/first-run-checklist.md` | N |
| 37 | Add server wizard (install command per OS, wait for registration, success) | `POST /v1/register`, `docs/deployment/agent-install-upgrade.md` | N |
| 38 | Dashboard / overview home (health summary, open incidents, issues, recent events) | `/v1/health/summary`, `/v1/health/issues` | N (index redirects to incidents) |
| 39 | Health issues drill-down | `GET /v1/health/issues` | N |
| 40 | Incident candidates review, Open incident and Dismiss actions | `GET /v1/incidents/candidates`; NEW endpoint approved: dismiss candidate (`POST /v1/incidents/candidates/{id}/dismiss`) | N |
| 41 | Monitor test result (run test now, show result) | `POST /v1/monitors/{id}/test`; approved: may need a richer result payload (DNS, TLS timings) | N (inline only) |
| 42 | Monitor pause / resume / maintenance window scheduling | `/pause`, `/resume`, `PUT /agents/{id}/maintenance` | P |
| 43 | Monitor edit (config) full page | `PUT /v1/monitors/{id}/config` | P (dialog) |
| 44 | Delete confirmation dialogs (monitor, server, status page) | CRUD completeness | N |
| 45 | Server unregister / decommission flow | token revoke | N |
| 46 | Alert route builder page | `/v1/alerts/routes` | N |
| 47 | Alert delivery detail (payload, retries, error), Retry delivery button | `/v1/alerts/deliveries`; NEW endpoint approved: `POST /v1/alerts/deliveries/{id}/retry` | N |
| 48 | Event detail drawer | `/v1/events` | N |
| 49 | Account / profile and session expiry screen | JWT auth | N |
| 50 | Settings sections: General, Security (admin credentials, tokens), Notifications defaults, About / version | settings only has data lifecycle | N |
| 51 | Backup / restore guidance screen | `docs/sqlite-backup-restore.md` | N |
| 52 | Command palette / global search | nav of 7 sections | N |
| 53 | 404 / route error / Core unreachable | resilience | N |
| 54 | Global states: loading skeleton, empty, error, offline banner | every list | P (EmptyState exists) |

## 4a. Decisions

Authoritative log: `docs/plans/decision-log.md`. Applied here:

- D-003: Overview is the first nav item and `/` lands on it (rows 38, 36).
- D-004: new Core endpoints approved, each needs route annotations, OpenAPI and `docs/agent-core-contract.md` review before the UI ships: retry delivery (`POST /v1/alerts/deliveries/{id}/retry`), dismiss incident candidate (`POST /v1/incidents/candidates/{id}/dismiss`), run monitor test (richer result payload).
- D-005: a system sans fallback is acceptable. Owner amendment 2026-10-17: the typeface is TWK Lausanne (300 and 500), not GT America; see `design-language.md` section 4.
- D-006: onboarding is a dismissible card on Overview, not a route. The first-run artboard is the Overview route in its empty state.
- Owner decision 2026-10-17: alert channels are Webhook and Email only.

## 4b. Artboards added after the first pass

Unsubscribe by token (34, `public-unsubscribe`), status badge SVG (35, `status-badge`), agent monitor setup flow (13, `monitor-agent-setup`). Alerts Routes (20, `alerts-routes`) already has its own artboard.

- Redesign 2026-10-15: shell, tokens and all screens were rebuilt; see `design-language.md` v2. Screen numbers are unchanged. New in the redesign: Overview hero, incident detail two-column layout, response-time chart on monitor detail, channels as cards, public pages polish.

## 5. Flows

Flows 1 to 9 are described below; flows 10 to 20 (webhook channel, email channel, failed delivery, investigate logs, server lifecycle, manual incident, bulk actions, publish a status page, triage candidate, account and session, API tokens) are on the Flows artboard.

1. **First run**: install Core -> login -> onboarding checklist -> add server -> create monitor -> add channel -> create rule -> publish status page.
2. **Add server**: Servers > Add server -> copy install command -> agent registers (`/v1/register`) -> token shown once -> server appears Up.
3. **Create monitor**: Monitors > New -> pick type -> configure -> Test -> Save -> first check -> Up.
4. **Detect to resolve**: check fails -> monitor Down -> incident opened -> rule matches -> channel delivery (sent/failed) -> operator acknowledges -> optional cover -> check recovers or operator resolves -> timeline closed.
5. **Public communication**: incident open -> suggestion -> draft public incident -> post updates -> resolve -> subscribers notified.
6. **Heartbeat**: create heartbeat monitor -> copy ping URL -> job pings -> missed ping -> stale -> down.
7. **Token lifecycle**: status -> rotate -> reissue -> revoke.
8. **Subscriber lifecycle**: subscribe -> confirm email -> manage -> unsubscribe.
9. **Data lifecycle**: set retention -> run rollup -> run archive.

## 6. Totals

Existing routes 11, redirects 1, overlays 14, public 9, missing 19 (rows 1 to 54), plus the 41 screens listed in section 7. Canvas covers all of them, grouped into pages (see artifact).

## 7. Added in the flows pass (2026-10-17)

Channels (Webhook, Email only): channel-dialog (type), channel-webhook, channel-email, channel-test, channel-detail, email-confirm (public).
Servers: server-monitors, server-stale, server-edit, server-decommission, server-token-new, add-server-connected, add-server-failed, servers-empty.
Logs: logs-events and logs-service (explorers), logs-live, logs-patterns, logs-detail, logs-views, logs-export.
Incidents: incident-declare, incidents-bulk, incidents-empty, incident-resolved, incident-public-update.
Monitors: monitor-edit, monitors-bulk, monitors-empty, monitor-create-tcp, monitor-paused.
Status pages: status-pages-list, status-create, status-domain, status-maintenance, status-publish.
Global and settings: avatar-menu, shortcuts, overview-all-good, settings-general, settings-tokens, settings-token-new, settings-audit.
Flows board: twenty journeys.
Still not drawn: password reset (done on the Core host through an environment variable), mobile and narrow layouts, print views.

## 8. Comment pass (2026-10-17, second)

Logs are now three sources: status logs (events, filter by server and monitor), service logs (Orion backend), app logs (ingested from your apps). New artboards: logs-app, logs-app-detail, logs-app-empty, logs-app-connect, logs-sources. Reworked: channel type picker (compact rows), incident-resolved (lifecycle contained, timeline as a table, one primary action), bulk bars (floating, bottom), dark Overview (sunken fields), empty states (single border), public status, history and incident pages (design system components).

## 9. Product logs (D-011, 2026-10-17)

Unified Logs page (logs, logs-rich, logs-plain, logs-detail, logs-detail-plain, logs-no-apps, logs-no-results, logs-error), with logs-live, logs-patterns, logs-views, logs-export. Apps settings (settings-apps, settings-app-new, app-keys, app-key-new, app-retention, app-display, app-redaction, app-send-test). Connect your first app (app-connect-create, app-connect-snippet, app-connect-waiting, app-connect-received). Settings tabs are now Data lifecycle, General, Apps, Security, System, Audit log. Superseded: logs-app, logs-app-detail, logs-app-empty, logs-app-connect, logs-sources (the earlier three-tab design).
