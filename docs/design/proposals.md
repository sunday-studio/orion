# Orion Console: feature and implementation proposals

Date: 2026-10-14. Companion to `screen-map.md` and `design-language.md`. The canvas carries a purple note per page marking these. Nothing here is built; every API item needs Core route annotations, OpenAPI and `docs/agent-core-contract.md` review first.

Priority: P0 closes a gap in the main loop (detect, notify, resolve). P1 saves operator time every day. P2 is polish or growth.

## 1. Cross-cutting implementation proposals

| # | Proposal | Why | Priority |
|---|----------|-----|----------|
| X1 | Extract one `SummaryTiles` component. `incident-summary.tsx`, `agent-summary.tsx` and `monitor-summary.tsx` are near copies of the same dithered-tile grid | Three places to change for one visual fix; the canvas draws them identically | P0 |
| X2 | Extract a `FilterBar` (search input plus select triggers plus Clear) used by incidents, monitors, alerts log and both log tabs | Same markup is repeated per feature | P1 |
| X3 | Keep log-like tables as plain TanStack tables with select filters. Update the `AGENTS.md` Console Table Rules, which still describe the OpenStatus left filter panel the Console does not use | Docs and product disagree | P0 |
| X4 | Shared toast region and offline banner | Mutations currently report errors as inline text; offline state is undesigned in code | P1 |
| X5 | One timezone setting (default browser) applied by `formatDate` | Timestamps mix relative and absolute across screens | P1 |
| X6 | Self-host licensed GT America files; keep Instrument Serif for serif moments | Required by the font decision | P0 |
| X7 | Add `Overview` as an index route and nav item (new; not in the Console today) | The canvas leads with it | P1 |
| X8 | Keyboard: `g` then letter navigation, `j`/`k` in tables, `?` shortcut sheet | Operators live in lists | P2 |

## 2. Per-screen proposals

### Overview (new)
- P1 "Since you last looked" strip: new incidents, recoveries, failed deliveries since the last visit (per browser).
- P1 Setup card progress derived from real data so steps tick themselves.
- P2 Saved time range for the uptime bar.
- API: `GET /v1/overview` aggregating health summary, open incidents, issues and recent events in one round trip.

### Incidents
- P0 Dismiss candidate and open-from-candidate (approved endpoint).
- P1 Bulk acknowledge and resolve from row checkboxes.
- P1 Saved views as URL presets (`nuqs` already holds the state).
- P1 Runbook link on the monitor, surfaced on incident detail.
- P1 Copy-as-summary (title, severity, timeline) for chat.
- P2 MTTA and MTTR strip over a chosen range.
- API: bulk lifecycle action, `GET /v1/incidents/stats`, `monitor.runbook_url`.

### Servers
- P1 Labels and groups; filter by label; bulk maintenance for a group.
- P1 Inline CPU and memory sparklines in the row.
- P1 Metric charts with threshold lines on System Metrics.
- P2 Decommission flow that also removes monitors with a count preview (drawn in delete confirmation).
- API: agent labels, `GET /v1/agents/{id}/metrics?range=`.

### Monitors
- P1 Response-time chart (p50, p95) on detail.
- P1 Bulk pause, resume, maintenance; clone monitor.
- P1 TLS expiry column and warning state.
- P2 Multi-region checks with per-region result and quorum rule.
- P2 Change history (who edited what, when).
- API: bulk and clone endpoints, `region` on core monitors, audit events.

### Alerts
- P0 Retry delivery and Run test (approved endpoints).
- P1 Escalation: notify channel A, then channel B after N minutes without acknowledgement.
- P1 Quiet hours per rule.
- P2 Rule templates for the common cases.
- Note: the live Console has three Alerts tabs. Routes is a canvas proposal; decide whether routes stay folded into rules.
- API: escalation steps on rules, `POST /v1/alerts/deliveries/{id}/retry`.

### Status pages
- P0 Publish checklist with blockers (drawn in Preview).
- P1 Scheduled maintenance that shows publicly and suppresses alerts for mapped resources.
- P1 Custom domain wizard with the DNS record to add and a Verify action.
- P1 Incident templates and update presets.
- P2 Subscriber import and export CSV; embeddable widget beside the badge.
- API: maintenance windows, domain verify, subscriber export.

### Logs
- P1 Live tail toggle, pause on hover.
- P1 Copy row as JSON or curl; export CSV for the filtered range.
- P2 Saved queries.
- API: SSE tail, CSV export.

### Settings
- P1 Audit log of who changed what.
- P1 API tokens and agent registration token rotation.
- P2 Version and update check; backup status instead of "not tracked".
- API: `GET /v1/audit`, API token CRUD.

### Public pages
- P1 90-day tooltips per component with day detail.
- P1 Subscribe per component (drawn), RSS and Atom (drawn).
- P2 Dark theme and locale; widget.

## 3. Suggested sequence

1. Foundation (X1, X3, X6, X4): parity, shared components, docs.
2. Main loop (retry delivery, dismiss candidate, run test, bulk incident actions, runbook link).
3. Visibility (Overview, sparklines, response-time charts, labels).
4. Resilience of alerts (escalation, quiet hours, status page maintenance).
5. Growth (multi-region, audit log, API tokens, widget).

## 4. Redesign applied to the canvas (2026-10-15)

- New shell (sidebar, command search, top bar), new tokens and components, every screen rebuilt including dark Overview, public pages and dialogs.
- Shown in design and needing implementation: Overview hero, sparklines in lists, response-time and resource charts, channel cards, incident detail rail, status page preview frame, Settings API tokens.
- Earlier polish notes below are superseded.

## 5. Earlier polish pass

- Native selects now share the Console select style; inline error and success boxes are flat callouts instead of rounded cards.
- Dialogs show a close control; public pages drop the accent bar and shadow so they read as calm documents.
- Summary tile text sits above the dither pattern; artboard heights fit their content and rows were re-flowed (three artboards had been clipped).
