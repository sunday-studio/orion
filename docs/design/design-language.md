# Orion Design Language (current)

Canonical spec for the redesigned Console, as drawn on the canvas (https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY) on 2026-10-17. The web thread implements from this file, `screen-map.md` and `implementation-handoff.md`. The journey of how it got here (earlier directions, rejected ideas) is in `design-history.md`; if the two disagree, this file wins. Decisions D-003 to D-006 in `docs/plans/decision-log.md` are applied below.

## 1. Principles

1. Status first. Every screen opens with what is wrong, then what to do about it.
2. One question per screen. Lists show the few things that matter; detail lives one click away.
3. Calm, centered, dense. Content sits in a centered column; rows are one line.
4. Real controls. Filters filter. Tables are tables. No task-board metaphors.
5. Honest numbers. Tabular figures everywhere; every chart says "as of" and its range.

## 2. Layout

- White page, no gray frame. Content is one centered column: 1040px for list pages (Monitors, Servers, Incidents, Health issues, Triage, Logs), 760px for detail, form and settings pages.
- Page header: title (30px, weight 500), one muted line, actions on the right. The header block uses the same column width as the page body.
- Tabs inside a page are centered; the rule sits only under the tabs (`width: max-content`), never full width.
- Containers: a pale field (`#f7f7f6`, 14px radius, 20px padding) holds white cards (10px radius, 1px `#ececea` border, `0 1px 2px rgba(0,0,0,.05)` shadow). Use a field when a page has several groups of settings or checks; use a bare card for a single table.
- No sidebar. No full-width bars other than the top header.

## 3. Navigation (D-003)

Top header, 52px, white, 1px bottom border. Three zones in a grid (`1fr auto 1fr`):

- Left: logo mark and "Orion".
- Center: a tray (`#f4f4f2`, 9px radius, 3px padding) with seven items: Overview, Incidents, Servers, Monitors, Alerts, Status, Logs. The active item is a white chip with a hairline shadow. Overview is first and `/` lands there.
- Right: search field with a Cmd K hint (opens the command palette), a health chip (green "All systems normal", amber "N issues", gray "No data yet"), a Settings gear (active state when on Settings), and the avatar. The avatar opens the account menu: theme (Light, Dark, System), timezone, keyboard shortcuts, documentation, sign out. Sign out is only in the menu.

## 4. Typography (D-005 amended by the owner on 2026-10-17)

- Typeface: TWK Lausanne. Text uses the 300 cut, emphasis uses 500. There is no semibold; anything that was 600 or 700 renders at 500.
- Fallback stack: `"TWK Lausanne", system-ui, -apple-system, "Segoe UI", sans-serif`. The system sans fallback is acceptable (D-005) where the licensed files are not loaded. The files in the repo `fonts/` folder are trial builds: buy the web licence before shipping, then self-host the woff2 files in the Console.
- Numbers: `font-variant-numeric: tabular-nums lining-nums` on everything, so columns of times, counts and percentages align.
- Code blocks and keycaps: IBM Plex Mono (or `ui-monospace`), 12px. Ids and URLs in tables use the body face with tabular figures, muted.
- Sizes: page title 30/500, section title 15/500, body 13 to 14, table 13 (12.5 in dense log tables), meta and captions 12 muted, table header 11.5 muted.
- No Instrument Serif.

## 5. Color

| Token | Value | Use |
|---|---|---|
| page | `#ffffff` | page ground |
| field | `#f7f7f6` | group container |
| sub | `#fafaf9` | table header, row hover |
| border | `#ececea` | cards, dividers; inputs `#e4e4e7` |
| text | `#18181b` | primary |
| muted | `#71717a` | secondary |
| faint | `#a1a1aa` | tertiary, ids, times |
| accent | `#1f7a50` | primary button, active tab rule, check marks; chart green `#2a9f69` |
| focus | `rgba(31,122,80,.22)` | focus ring |

Status colors are reserved for status. Badge fills and text:

| Status | Fill / text | Shape |
|---|---|---|
| up, sent | `#ccfbf1` / `#115e59` | dot |
| down, failed, open | `#ffe4e6` / `#9f1239` | square |
| degraded, warning, acknowledged | `#fef3c7` / `#92400e` | half dot |
| unknown, pending, draft | `#f5f5f4` / `#44403c` | hollow dot |
| maintenance, paused | `#ede9fe` / `#5b21b6` | pause bars |
| stale, disabled | `#f1f5f9` / `#334155` | hollow dot |
| covered, cooldown | `#cffafe` / `#155e75` | dot |
| resolved | `#dbeafe` / `#1e40af` | dot |
| critical | solid `#f43f5e`, white text | square |
| high / medium / low | rose / yellow `#fef9c3` / teal | dot |

Row status indicators use an 8px dot: red `#e5484d`, amber `#f59e0b`, green `#2a9f69`, gray `#a1a1aa`.

Dark mode (Overview is the reference artboard): the sunken look comes from layering, never from white borders.
- Page `#111513`; header the same with a `#1d2321` bottom rule.
- Field is sunken: `#0c0f0e`, 1px `#171c1a` border, inner shadow `inset 0 2px 6px rgba(0,0,0,.55)`.
- Cards and tables sit above the field: `#1a201e`, 1px `#252c29` border, top highlight `inset 0 1px 0 rgba(255,255,255,.035)`, soft drop shadow.
- Nav tray is sunken (`#0b0e0d`, inner shadow); the active chip is raised (`#222926`).
- Text `#e9efec`, muted `#8b9892`. Badges use dark tints: up `#0f2e22`/`#6ee7b7`, down `#3a1720`/`#fda4af`, degraded `#3a2a0a`/`#fcd34d`, unknown `#232a27`/`#a8b4ad`, critical solid `#e5484d`.
- Light is the default.

## 6. Shape and elevation

Radii: fields 14px, cards and tables 10px, buttons and inputs 8px, tray and chips 6 to 9px, badges 4px. No pills, no fully rounded controls except avatars and dots. Empty states use a single solid hairline container (white, 10px radius). No dashed borders and no border inside a border; inside an existing card the empty state has no border of its own.

Buttons are layered: a 1px border, an inset top highlight and a soft drop shadow. Primary is a green gradient with white text; secondary is white; destructive is rose; ghost is text only. Heights: 36px default, 28px small (`sm`), 24px inside table rows.

## 7. Component inventory

Names match the "Components" artboard. "Exists" means a Console component already covers it; "New" means build it.

| Component | Used on | Console |
|---|---|---|
| Header with tray | all product pages | `shared/app-header.tsx`, rebuild |
| Page header | all pages | `shared/page-header.tsx`, restyle to this spec |
| Tabs, centered | Alerts, Status, Logs, Server, Settings | `ui/tabs.tsx`, restyle |
| Filter bar | Incidents, Alerts log, Logs, Subscribers, Reports, Audit | New `FilterBar` over nuqs; AGENTS.md table rules still apply |
| Filter token | filter bar | New: field, operator, value, remove |
| Filter popover | any token | New, uses `ui/combobox.tsx` + `ui/checkbox.tsx` |
| Grid table | every list | `shared/data-table.tsx`, restyle: 38px rows (30px dense), sticky 30px header, one line per row |
| List row | inside cards | New `ListRow`: dot, bold title, muted subline as its own column |
| Badge | everywhere | `shared/status-badges.tsx`, restyle: icon and text centered in one flex row |
| Severity badge | incidents | `shared/status-badges.tsx` |
| Uptime strip | Monitors, Overview, public | `shared/uptime-sla.tsx` |
| Stat cards | Overview, Monitor, Server | `shared/detail-card.tsx` |
| Lifecycle tracker | Incident, Overview | New (Detected, Notified, Acknowledged, Resolved) |
| Expected vs actual | Incident, test result | New |
| Field + card | Settings, Status, Add server | `shared/detail-group.tsx`, restyle |
| Dialog | create, confirm, test | `ui/dialog.tsx`, restyle |
| Drawer | report and log detail | New right-side drawer (460 to 480px) |
| Menu | account, time range, saved views | `ui/dropdown-menu.tsx` |
| Callout | forms, banners | New (up, down, degraded) |
| Empty state | all lists | `shared/empty-state.tsx`, restyle: dashed square container, one button |
| Chips input | recipients, labels | New |
| Token reveal box | token flows | New: dark box, copy button, "shown once" callout |
| Histogram | Logs | New |
| Facet rail | Logs | New |
| Command palette | global | New |
| Toast, offline banner | global | New |

## 8. Rules (apply everywhere)

1. One line per row. A subline becomes its own column (Detail, Target, Type). Never a second line under the name.
2. Issues first, then a link. Where an entity has many children (monitors on a server), show those with problems first, at most 10, then link to the full list filtered by the entity.
3. Lists use real filters (tokens, popover with "is one of"), URL-backed. Status segments (`All 24 / Down 1 / ...`) are quick filters only.
4. Every destructive action opens a confirm dialog that states the blast radius (counts of what will be deleted). One-time secrets (tokens, signing secrets) are shown once, with a "stored it" checkbox before Done.
5. Every error, empty and failure state ends in a button: Retry, Add, Fix.
6. Timestamps: relative for under 24 hours in lists, absolute in detail. One timezone setting (default UTC), applied everywhere.
7. Mono-looking data (ids like INC-0231, URLs, times) is muted, tabular and unbroken. Truncate with an ellipsis; show the full value in detail.
8. Charts always carry "as of HH:MM" and the range.

## 9. Screens, by area

The full list with routes is in `screen-map.md`; flows are on the Flows artboard (twenty journeys). Patterns specific to an area:

- Overview (D-003, D-006): "Needs you now" field holding the highest-severity open incident with the lifecycle tracker; service health with 90 day strips; ongoing incidents table; stat cards; recent incidents. First run is the same route with one sentence and one button (Add your first server); it is not a separate route and carries a dismiss action. When all is well the first field reads "All systems normal".
- Incidents: table (id, title, status, severity, monitor, opened). Bulk actions appear in a floating bar at the bottom of the screen with standard buttons; selected rows get a light green tint. Resolved detail shows one status badge, a muted meta line, one primary action and a More menu. Detail: tracker, expected vs actual, timeline, facts panel, public update dialog. Declare incident is a dialog. Triage lists candidates with Dismiss and Open incident (D-004).
- Servers: table with CPU sparkline. Detail tabs: Overview (needs attention first, chart), Reports, Monitors (filter, issues first), Token. States: no reports (checklist), edit, remove with monitor-count preview, token reveal.
- Monitors: table with target as a column, uptime strip. Create dialog per type (HTTP, TCP, Heartbeat, Agent). Configure is a page. Bulk bar (Pause, Maintenance, Clone, Delete). Paused state banner.
- Alerts: tabs Notification log, Channels, Rules, Routes. Channels are Webhook and Email only. Webhook: URL, signing secret, headers, payload preview, send test. Email: recipients as chips, each confirmed by a public confirmation page, instant or digest. Failed deliveries can be retried (D-004).
- Status: list of pages; per page tabs Setup, Components, Incidents, Subscribers, Preview. Custom domain verification, schedule maintenance, publish and unpublish dialogs. Public pages keep a calm document style but use the same components (stat cards, fields, one-line tables).
- Logs (D-011, `docs/plans/product-logs-plan.md`): tabs Logs, Events, Orion service.
  - **Logs** is the unified product log page for all sources (apps and agents). Query bar (free text plus `field:value`), source and time selects, filter tokens, a volume histogram stacked by severity, a facet rail (Severity, Source, Event, Environment, then Fields from the app's field catalog with type tag and cardinality), and a one-line table: Time, Severity, Source, Message. Message is the title (event, else the first line of the body) followed by muted promoted attributes. Header actions: Copy link, Save view, Live tail (pausable). Severity has six levels: Trace, Debug, Info, Warn, Error, Fatal.
  - **Uniform display from read-time normalization:** the same page renders an app with rich attributes (logs-rich: columns from its display profile, fields facets with value distributions) and a plain-text-only app (logs-plain: Time, Severity, Message only, "severity inferred" tag, a Fields note with a link to set up a text parser).
  - Detail drawer: key facts, message, attribute tree (collapsible, redacted values shown as "redacted"), Copy as JSON and link, related logs (same request or trace), "Show all N logs for this request". Plain-text variant: full body in a code block, no attributes, hint to set up a parser, nearby lines.
  - States: no apps yet (connect your first app), no results (widen range, clear filters), query failed and ingest behind banner (429 backpressure).
  - **Events** (what Orion saw: filter by server and monitor) and **Orion service** (backend internals) keep the same layout.
- Apps (Settings, Apps tab): list with status (Receiving, Quiet, Never), rate, retention; per app tabs Ingest (keys, endpoints, send test), Retention (days, size cap, debug sampling, rate limit, purge), Display (title, columns, hidden fields, live preview), Redaction (default keys, custom key and regex rules, test box). Ingest keys are write-only and shown once. Send a test log offers curl, OpenTelemetry environment variables, TypeScript, Go and Python.
- Connect your first app (onboarding path from the Overview card and from Logs empty state): Create app, Add the key (shown once, snippet), Waiting for the first log (live check, send test log), First log received (open in Logs).
- Settings: tabs Data lifecycle, General, Security (API tokens), System, Audit log.

## 10. Accessibility and states

- Color is never the only signal: badges carry a label and a shape.
- Focus ring on all controls (accent at 22 percent). Dialogs trap focus and close with Escape.
- Keyboard: Cmd K search, `G` then `I/M/S` navigation, `J/K` rows, `/` focus filter, `?` shortcut sheet.
- Every list defines loading (skeleton rows), empty (first use), empty (filters applied), error (retry) and offline (banner) states; see System states.
