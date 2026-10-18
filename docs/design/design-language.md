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

Buttons are layered: a 1px border, an inset top highlight and a soft drop shadow. Primary is a green gradient with white text; secondary is white; destructive is rose; ghost is text only. Heights: 30px default (12.5px text, 14px icons), 24px small (`sm`, used in page headers, table rows and bulk bars), 36px large only for single full-width CTAs on auth and first-run screens.

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

- Overview (D-003, D-006): "Needs you now" field holding the highest-severity open incident with the lifecycle tracker; server health strips (90 days, per server); ongoing incidents table; three stat cards (monitors up, servers reporting, uptime 30 days); recent incidents. Data sources are in `implementation-handoff.md` section 7. First run is the same route with one sentence and one button (Add your first server); it is not a separate route and carries a dismiss action. When all is well the first field reads "All systems normal".
- Incidents: table (id, title, status, severity, monitor, opened). Bulk actions appear in a floating bar at the bottom of the screen with standard buttons; selected rows get a light green tint. Resolved detail shows one status badge, a muted meta line, one primary action and a More menu. Detail: tracker, expected vs actual, timeline, facts panel, public update dialog. Declare incident is a dialog. Triage lists candidates with Dismiss and Open incident (D-004).
- Servers: table with CPU sparkline. Detail tabs: Overview (needs attention first, chart), Reports, Monitors (filter, issues first), Token. States: no reports (checklist), edit, remove with monitor-count preview, token reveal.
- Monitors: table with target as a column, uptime strip. Create dialog per type (HTTP, TCP, Heartbeat, Agent). Configure is a page. Bulk bar (Pause, Maintenance, Clone, Delete). Paused state banner.
- Alerts: tabs Notification log, Channels, Rules, Routes. Channels are Webhook and Email only. Webhook: URL, signing secret, headers, payload preview, send test. Email: recipients as chips, each confirmed by a public confirmation page, instant or digest. Failed deliveries can be retried (D-004).
- Status: list of pages; per page tabs Setup, Components, Incidents, Subscribers, Preview. Custom domain verification, schedule maintenance, publish and unpublish dialogs. Public pages keep a calm document style but use the same components (stat cards, fields, one-line tables).
- Logs (D-011, `docs/plans/product-logs-plan.md`): tabs Service logs, Status logs, Internal logs (owner's names).
  - **Service logs** is the unified product log page for all sources (apps and agents). Query bar (free text plus `field:value`), source and time selects, filter tokens, a volume histogram stacked by severity, a facet rail (Severity, Source, Event, Environment, then Fields from the app's field catalog with type tag and cardinality), and a one-line table: Time, Severity, Source, Message. Message is the title (event, else the first line of the body) followed by muted promoted attributes. Header actions: Copy link, Save view, Live tail (pausable). Severity has six levels: Trace, Debug, Info, Warn, Error, Fatal.
  - **Uniform display from read-time normalization:** the same page renders an app with rich attributes (logs-rich: columns from its display profile, fields facets with value distributions) and a plain-text-only app (logs-plain: Time, Severity, Message only, "severity inferred" tag, a Fields note with a link to set up a text parser).
  - Detail drawer: key facts, message, attribute tree (collapsible, redacted values shown as "redacted"), Copy as JSON and link, related logs (same request or trace), "Show all N logs for this request". Plain-text variant: full body in a code block, no attributes, hint to set up a parser, nearby lines.
  - States: no apps yet (connect your first app), no results (widen range, clear filters), query failed and ingest behind banner (429 backpressure).
  - **Status logs** (what Orion saw: filter by server and monitor) and **Internal logs** (Orion backend internals) keep the same layout.
  - Severity uses the standard badge (dot plus label: Error down, Warn degraded, Info and Debug neutral, Fatal critical), never a separate pill style. The volume histogram is stacked by severity (info soft green, warn amber, error red) with a brush selection and a hover tooltip.
- Apps (Settings, Apps tab): list with status (Receiving, Quiet, Never), rate, retention; per app tabs Ingest (keys, endpoints, send test), Retention (days, size cap, debug sampling, rate limit, purge), Display (title, columns, hidden fields, live preview), Redaction (default keys, custom key and regex rules, test box). Ingest keys are write-only and shown once. Send a test log offers curl, OpenTelemetry environment variables, TypeScript, Go and Python.
- Connect your first app (onboarding path from the Overview card and from Logs empty state): Create app, Add the key (shown once, snippet), Waiting for the first log (live check, send test log), First log received (open in Logs).
- Settings: tabs Data lifecycle, General, Security (API tokens), System, Audit log.

## 10. Accessibility and states

- Color is never the only signal: badges carry a label and a shape.
- Focus ring on all controls (accent at 22 percent). Dialogs trap focus and close with Escape.
- Keyboard: Cmd K search, `G` then `I/M/S` navigation, `J/K` rows, `/` focus filter, `?` shortcut sheet.
- Every list defines loading (skeleton rows), empty (first use), empty (filters applied), error (retry) and offline (banner) states; see System states.

## 11. Token reference (copy into the design system)

Tailwind v4 `@theme` block. Light values are the default; the dark set overrides under `.dark` (class strategy) so a component never hard-codes a color.

```css
@theme {
  /* surfaces */
  --color-page: #ffffff;
  --color-field: #f7f7f6;
  --color-sub: #fafaf9;
  --color-card: #ffffff;
  --color-tray: #f4f4f2;
  /* borders */
  --color-line: #ececea;          /* cards, dividers */
  --color-line-strong: #e4e4e7;   /* inputs, popovers */
  --color-line-hover: #d4d4d8;
  /* text */
  --color-ink: #18181b;
  --color-muted: #71717a;
  --color-faint: #a1a1aa;
  /* accent */
  --color-accent: #1f7a50;
  --color-accent-hover: #238556;
  --color-accent-top: #2c9f6a;     /* primary gradient top */
  --color-accent-edge: #14573a;    /* primary border */
  --color-chart-green: #2a9f69;
  --color-focus: rgb(31 122 80 / 0.22);
  /* danger button */
  --color-danger: #d61f45;
  --color-danger-top: #f0486a;
  --color-danger-edge: #a8123a;
  /* status (bg / fg) */
  --color-up-bg: #ccfbf1;   --color-up-fg: #115e59;
  --color-down-bg: #ffe4e6; --color-down-fg: #9f1239;
  --color-deg-bg: #fef3c7;  --color-deg-fg: #92400e;
  --color-unk-bg: #f5f5f4;  --color-unk-fg: #44403c;
  --color-mnt-bg: #ede9fe;  --color-mnt-fg: #5b21b6;
  --color-stale-bg: #f1f5f9; --color-stale-fg: #334155;
  --color-cov-bg: #cffafe;  --color-cov-fg: #155e75;
  --color-res-bg: #dbeafe;  --color-res-fg: #1e40af;
  --color-crit: #f43f5e;
  --color-med-bg: #fef9c3;  --color-med-fg: #713f12;
  /* status dots */
  --color-dot-red: #e5484d; --color-dot-amber: #f59e0b; --color-dot-green: #2a9f69; --color-dot-gray: #a1a1aa;
  /* radii */
  --radius-badge: 4px; --radius-chip: 6px; --radius-control: 8px; --radius-card: 10px; --radius-field: 14px; --radius-dialog: 16px;
  /* shadows */
  --shadow-card: 0 1px 2px rgb(24 24 27 / 0.05);
  --shadow-raised: 0 1px 2px rgb(24 24 27 / 0.05), 0 4px 14px -4px rgb(24 24 27 / 0.08);
  --shadow-menu: 0 12px 32px -8px rgb(0 0 0 / 0.18), 0 2px 6px rgb(0 0 0 / 0.06);
  --shadow-dialog: 0 24px 48px -12px rgb(24 24 27 / 0.28), 0 6px 14px -4px rgb(24 24 27 / 0.10), 0 0 0 1px rgb(24 24 27 / 0.06);
  --shadow-inset: inset 0 1px 2px rgb(24 24 27 / 0.05);
  /* type */
  --font-sans: "TWK Lausanne", system-ui, -apple-system, "Segoe UI", sans-serif;
  --font-mono: "IBM Plex Mono", ui-monospace, "SF Mono", Menlo, monospace;
}
.dark {
  --color-page: #111513;  --color-field: #0c0f0e;  --color-sub: #161b19;  --color-card: #1a201e;  --color-tray: #0b0e0d;
  --color-line: #252c29;  --color-line-strong: #2a312e; --color-line-hover: #3a4540;
  --color-ink: #e9efec;   --color-muted: #8b9892;  --color-faint: #6a7570;
  --color-up-bg: #0f2e22;   --color-up-fg: #6ee7b7;
  --color-down-bg: #3a1720; --color-down-fg: #fda4af;
  --color-deg-bg: #3a2a0a;  --color-deg-fg: #fcd34d;
  --color-unk-bg: #232a27;  --color-unk-fg: #a8b4ad;
  --color-crit: #e5484d;
}
```

Dark-only structure values: header bottom rule `#1d2321`; field border `#171c1a`; row divider `#222926`; tray chip `#222926`; input background `#0f1311`; secondary button gradient `#242b28` to `#1d2321` with border `#2f3733`; field inner shadow `inset 0 2px 6px rgb(0 0 0 / .55)`; card top highlight `inset 0 1px 0 rgb(255 255 255 / .035)`. The primary button keeps its green gradient but loses the glow: shadow is `inset 0 1px 0 rgb(255 255 255 / .18), 0 1px 2px rgb(0 0 0 / .4)`.

## 12. Scales

- Type (px / line-height / weight): 2xl 30 / 1.15 / 500 with `-0.02em` (page title); xl 18 / 1.2 / 500 (dialog title); lg 15 / 1.4 / 500 (section title); md 14 / 1.5 / 300 (long copy); base 13 / 1.5 / 300 (UI default, table cells); sm 12.5 / 1.4 (dense tables, hints); xs 12 / 1.4 (captions, meta, muted); 2xs 11.5 (table header, facet group labels, uppercase with 0.02em). Weights: 300 text, 500 emphasis. Mono 12px for code.
- Spacing (4px base, half steps allowed): 4, 6, 8, 10, 12, 14, 16, 20, 24, 32, 40, 56. Control inner padding 0 10 to 14; card padding 14 to 16; field padding 20; dialog padding 24; page top padding 40; gap between fields 12; gap between dialog rows 16.
- Layout: header 52; list column 1040; form column 760; facet rail 190 with 18 gap; drawer 460 to 500; dialog widths 340 / 400 / 460 / 520 / 560; command palette 580.
- Row heights: table 38, dense table 30, table header 30 (28 dense), list row 40, day divider 26, input and select 30, button 30 and 24, tray item 28.
- Z-index: header 10, floating bulk bar 40, menu and popover 50, drawer 60, dialog and scrim 70, toast 80.

## 13. Focus, motion, accessibility

- Focus ring: `focus-visible` only. `box-shadow: 0 0 0 3px var(--color-focus)` added to the control's own shadow; outline removed. On dark, the same green at 22 percent. Error focus ring uses `rgb(229 72 77 / .2)`.
- Hit targets: 24px minimum height, 8px minimum gap between adjacent controls in a row.
- Motion: color, background and shadow changes 150ms ease. Dialog: scrim fades in 150ms; panel fades and scales from 0.98 and 4px below in 180ms ease-out; exit 120ms. Drawer: slides in 200ms ease-out. Menu and popover: fade and 2px rise, 120ms. Toast: slide up 8px and fade, 180ms, auto-dismiss at 5s (errors stay until dismissed). Live-tail pulse: 1.6s infinite on a 4px ring. `prefers-reduced-motion`: remove transforms and pulse, keep fades under 100ms.
- Never rely on color alone; badges keep their shape glyph. Dialogs trap focus, return focus to the trigger, close on Escape. Tables expose sort state with `aria-sort`; rows with checkboxes expose the selected count in a live region.

## 14. Component specification

All components are Base UI primitives (`@base-ui/react`) styled with Tailwind utility classes that read the tokens above, written in TypeScript with `cva` variants and `cn`. State columns: D default, H hover, F focus-visible, A active, X disabled, L loading, E error.

### 14.1 Controls

| Component | Base UI | Variants and sizes | States |
|---|---|---|---|
| Button | `Button` | variant: primary, secondary, destructive, ghost. size: sm 24, md 30, lg 36. Icon-only is square at the same height | D: layered (secondary: white gradient `#fff` to `#f7f7f8`, 1px `line-strong`, inset top highlight, `shadow-card`). H: gradient shifts to `#f1f1f3`, shadow `0 1px 3px rgb(24 24 27 / .12)`. A: inset shadow `inset 0 1px 2px rgb(24 24 27 / .12)`, bg `sub`. F: ring. X: 50 percent opacity, no shadow, `not-allowed`. L: 14px spinner replaces the icon, label stays, `aria-busy`, pointer events off. Primary: gradient `accent-top` to `accent`, border `accent-edge`, white text, inset top highlight `rgb(255 255 255 / .25)`, inset bottom `rgb(0 0 0 / .18)`, drop `0 1px 2px rgb(20 87 58 / .4), 0 4px 10px -3px rgb(31 122 80 / .35)`; H: `#32aa73` to `#238556`; A: flat `#1b6e48` with inset shadow. Destructive: `danger-top` to `danger`, border `danger-edge`. Ghost: transparent, text `muted`, H adds `rgb(24 24 27 / .05)` |
| Input, Textarea | `Input`, `Field` | size md 30; textarea min height 64, resizable vertical | D: white, 1px `line-strong`, `shadow-inset`. H: border `line-hover`. F: ring, border unchanged. X: bg `sub`, text `faint`. E: border `#e5484d`, error ring, message 12px `#be123c` below with a 12px icon. Placeholder `faint` |
| Select | `Select` | md 30, min width 120 | Trigger styled like secondary button with a 14px chevron; popup uses Menu styling; selected item shows a check. States as Button |
| Combobox | `Combobox` | md | Input plus popup; empty list text "No matches" |
| Checkbox | `Checkbox` | 15px, 4px radius | D: white, 1.5px `#d4d4d8`. Checked: fill `accent`, white tick. Indeterminate: dash. F: ring. X: 50 percent. E: border danger |
| Radio | `Radio`, `RadioGroup` | 14px circle | Selected: 3px inner ring in `accent`. Used only for mutually exclusive settings in dialogs (remove server history). Type pickers use Choice cards, not radios |
| Choice card | `RadioGroup` item | width fills half row, padding 14, 10px radius | D: white, 1px `line-strong`, title 14/500, description 12.5 muted. Selected: border `accent`, ring `rgb(31 122 80 / .14)`. H: border `line-hover`. No radio dot is drawn |
| Switch | `Switch` | 32 by 18 | On: `accent` track, white thumb; X 50 percent |
| Segmented control | `ToggleGroup` (single) | sm 24 and md 30 | Tray `tray` with 3px padding; active item white with hairline shadow; used for status filters, List/Live/Patterns, severity, delivery mode |
| Chips input | `Input` plus chips | md | Chip 24px high, `tray` bg, 6px radius, remove button; Enter or comma commits; paste splits on commas |
| Token reveal | custom | one size | Dark box `#0f211b` text `#bff3d4` mono 12.5; Copy button flips to "Copied" for 2s; required "I have stored this" checkbox enables Done |

### 14.2 Navigation and structure

| Component | Base UI | Variants | States |
|---|---|---|---|
| Header | none (landmark `header`) | light, dark | 52px, grid 1fr auto 1fr; tray nav is a list of links; active item `aria-current="page"` |
| Tray nav item | link | default, active | D: muted. H: ink. Active: white chip, hairline shadow, ink, weight 500. F: ring |
| Tabs | `Tabs` | centered | List `width: max-content`, 1px bottom rule under the tabs only; selected: ink text, 2px `accent` underline; H: ink |
| Page header | none | with sub, with actions | h1 30/500, sub 13 muted, actions right aligned and baseline-aligned with the title |
| Breadcrumb | `ui/breadcrumb` | | 12.5 muted, current page ink |
| Menu | `Menu` | account, time range, saved views, row actions | Panel 240 wide, 10px radius, 6px padding, `shadow-menu`; item 30px, 6px radius, H bg `tray`; destructive item text `#be123c`; separators 1px `line` |
| Popover | `Popover` | filter popover (260 wide) | Header with back link and Done; operator select; grouped checkbox list with 11.5 uppercase group labels |
| Tooltip | `Tooltip` | | Dark `#18181b`, white 12px, 8px radius, 6 by 10 padding, 500ms delay |
| Command palette | `Dialog` plus list | | 580 wide; input row 44 tall; groups with 11.5 muted headers; rows 34; selected row `tray`; keycap hints right |
| Dialog | `Dialog` | sm 340, md 460 to 520, lg 560 | Scrim `rgb(28 25 23 / .5)`; panel white, 16px radius, 24 padding, `shadow-dialog`, border 1px `line`; footer right aligned with a left-aligned ghost action allowed |
| Drawer | `Dialog` (side) | 460 to 500 | Right edge, full height, 1px left border, scrim lighter (`.25`); header with title, badge, Prev, Next, Close |
| Bulk bar | none | | Fixed bottom center, white, 12px radius, 1px `line-strong`, `shadow-dialog`; count at left separated by a rule; Resolve or primary action uses the primary button |
| Toast | `Toast` | success, error, info | 320 wide card, left status dot; success `up` tint, error `down` tint |

### 14.3 Data display

| Component | Variants | Notes |
|---|---|---|
| Badge | status: up, down, deg, unk, mnt, stale, cov, res, open, ack, sent, failed, cool, pend, supp; severity: crit, high, med, low | 20px high, 4px radius, 12px text weight 500, padding 0 8px 0 7px, 5px gap between glyph and label, glyph 6px: dot (up), square (down, open, failed), half (deg, ack), hollow (unk, pend, stale, supp), bars (mnt). Glyph and label are vertically centered in one flex row |
| Dot | red, amber, green, gray | 8px circle, used as row lead indicator |
| Card | default | white, 10px radius, 1px `line`, `shadow-card`; header row (`bh`) 36 tall on `sub` with title 13/500 and a muted right slot |
| Field | default | `field` bg, 14px radius, 1px `line`, 20 padding, title row 15/500 with muted right text; holds cards |
| Stat card | 3-up grid | label 12 muted, value 22 to 26/500 tabular |
| Table (`DataTable`) | default, dense, selectable | Header 30 (28 dense) on `sub`, 11.5 muted. Rows 38 (30 dense), one line, ellipsis overflow, hover `sub`, selected row `#f2faf5`. Columns declared as CSS grid templates, 12px gap, 14px horizontal padding. Day divider rows 26 tall, mono 11 uppercase. Footer row for "Show N more" or pager. Sticky header inside scroll regions. Right-align numbers only when comparing magnitudes |
| Pager | cursor | "Showing 1 to 100 of N", Newer and Older buttons (sm), rows-per-page select |
| List row | inside a card | 40 tall, lead dot, title 13/500, muted detail as its own column (minmax 120 to 240 for the title), trailing badge or button |
| Filter bar | with tokens | Tokens are 28 tall segmented boxes: field, operator, value, remove; "+ Filter" dashed; Clear all at the right; wraps instead of clipping |
| Facet rail | | 190 wide; group label 11.5 uppercase faint; item row 24 with 15px checkbox, label, right-aligned count tabular faint; "Show N more" link |
| Histogram | severity stacked | 132px high, 3px bar gap, 3px top radius; info `#9bd7b8`, warn `#f5b942`, error `#e5484d`; gridlines every 30px `#f0f0ee`; brush: 1px `accent` border, 9 percent accent fill, label "Zoom to selection" above; tooltip dark card |
| Uptime strip | 24, 30, 45, 60 or 90 bars | 3 to 4px bars, green `#3dba85`, amber, red |
| Lifecycle tracker | 4 steps | circles 18px; done: green fill with tick; current: red ring; connecting line 2px; labels 12.5/500 with time 11 muted below. Line width equals the span between first and last step centers |
| Expected vs actual | | two cards; expected dark green `#142a23` with mint label, actual white with red value |
| Attribute tree | | mono 12.5, 16px indent, keys muted, strings green `#1f7a50`, numbers amber `#b45309`, redacted values as a gray chip, collapse chevrons |
| Code block | tabs variant | `#0f211b` background, mint text, 12px mono, 8px radius; optional tab strip (mono 11 uppercase) above; Copy button top right |
| Callout | up, down, degraded | 8px radius, 10 by 14 padding, bold lead word; tints from the status tokens |
| Empty state | inside card, standalone | Standalone: one solid 1px `line` container, white, 10px radius, 56px vertical padding, title 16/500, one sentence muted, one button. Inside a card: no border of its own |
| Skeleton | row | 12px bars on `tray`, 1.4s shimmer; reduced motion: static |
| Avatar | md 24 | round, generated gradient |

### 14.4 Behavior notes for state coverage

- Loading: tables show five skeleton rows; buttons show their L state; pages never blank the header.
- Error: inline errors under the field; page errors use the empty-state container with Retry; offline and ingest-behind use a callout above the content.
- Disabled rows: text `faint`, no hover.
- Selection: selecting any row shows the bulk bar; Escape clears.
- Keyboard: `/` focuses the filter or query bar; arrows move within tray, tabs and menus; Enter activates; the palette opens on Cmd K.

## 15. CLI and terminal conventions (agent plan C1 to C4)

The agent CLI (`orion-agent`) uses Charm (`lipgloss`, `bubbletea`, `huh`, `fang`) and takes its look from the same tokens as the Console. Canvas page "CLI and agent" draws each screen as an 80 column terminal with notes.

**Color mapping.** Terminal palette is generated from the token source (plan C1): background follows the terminal; accent `#6ee7b7` (dark) or `#1f7a50` (light) from `--color-accent`; muted `#8b9892` and faint `#6a7570` from `--color-muted` and `--color-faint`; danger `#fda4af`/`#be123c`; warning `#fcd34d`/`#92400e`; info `#93c5fd`/`#1e40af`. Truecolor first, then 256 and 16 color fallbacks. `NO_COLOR`, `--no-color` and non-TTY output print plain text.

**Never color alone.** Status uses a glyph and a word: up `●`, down `■`, degraded `◐`, unknown `○`, maintenance `‖`. Results use `✓` (pass), `!` (warning), `✗` (fail) followed by a label. Plain mode prints `[ok]`, `[warn]`, `[fail]` and `up/down/slow/?`. Spinner frames `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` replaced by `...` when not a TTY.

**Primitives.**
- Header: bold name, muted version; no banner art.
- Key and value: muted key, aligned values, 2 space indent.
- Box: `lipgloss` rounded border in faint color with the title in the top edge; used for Status and the installer's Doctor summary.
- Table: muted header, one thin rule, no vertical borders, status column uses glyph plus word.
- Steps: `✓` done, spinner running, `✗` failed, one line each with detail muted after the label.
- Callouts: `ERROR` badge (inverse red) with one line summary; file:line with a caret line; "Did you mean"; `Fix:` and `Docs:` lines.
- Diff: `+` green, `-` red, unchanged muted.
- Focused input: left bar `┃` in accent; hints muted; footer lists keys on an inverse muted bar.
- Width: readable at 80 columns; no line over 78 characters; long values truncate with an ellipsis.

**Screens.** Install script output (plan, step lines, Doctor summary, link to the server). Setup wizard in three steps (connect, choose what to watch with confidence labels, review a diff, apply without restart). Doctor streaming checklist with a fix command under every warning and failure and a stable `--json` schema. Status box plus a table of what is watched and a short "using now" line from passive metrics. Log viewer with a status line (follow, level, source, search), level as a colored word padded to five characters, and a key footer. Help grouped by intent (fang) and errors with file, line and fix. Exit codes: 0 ok, 1 failed checks, 2 usage, 3 needs elevation.

**Console Add server flow (plan A1).** The Console shows one copy-paste command with a one-time enrollment token baked in (expires in 15 minutes). While waiting, a static list labelled "The command will:" describes what the installer does (never live, Core cannot observe it) and a single live row says "Waiting for the server". The only live states are waiting, connected and not connected. Connected shows the server facts; there is no token panel. Platform tiles are Linux, macOS and Docker. Artboards: add-server, add-server-waiting, add-server-connected, add-server-failed.

## 16. Sign in

The left brand panel is dark green and fades into the white form side through a pale mint transition (no hard edge). Behind the headline, a dot grid at 16px spacing twinkles slowly (5s) and a brighter band of dots sweeps left to right (7s). Both stop under `prefers-reduced-motion`. The form column is 340px, centered in the right half.

## 17. Standardization rules (2026-10-18)

- **Elevation scale** (one value per tier, nothing ad hoc): `e1` cards, tables, stat cards, empty states, choice cards: `0 1px 2px rgb(24 24 27 / .05)`. `e2` menus, popovers, tooltips: `0 12px 32px -8px rgb(0 0 0 / .18), 0 2px 6px rgb(0 0 0 / .06)`. `e3` dialogs, drawers, floating bulk bar: `0 24px 48px -12px rgb(24 24 27 / .28), 0 6px 14px -4px rgb(24 24 27 / .10), 0 0 0 1px rgb(24 24 27 / .06)`. Fields have no shadow. Buttons: secondary inset top highlight plus `0 1px 2px rgb(24 24 27 / .08)`; primary and destructive inset highlight, inset bottom line and a 1px drop shadow only (no glow).
- **Radii**: field 14, card 10, control 8, chip 6, badge 4, dialog 16 (drawer rounds its left edge).
- **Alignment**: page headers and everything inside a column are left aligned; actions in a page header sit at the right of the title row; form and wizard footers are right aligned (secondary left of primary); notes and helper text are left aligned. Only empty states and the first-run Overview are centered.
- **Header and body widths match**: list pages use the 1040 header with the 1040 body; detail, form and wizard pages use the 760 header with the 760 body.
- **Wizard pattern** (Add server, Connect an app): narrow header (title, one muted line, ghost Cancel at the right), a stepper, fields, right-aligned actions. Stepper: numbered 22px circles joined by 2px lines; done = green fill with a tick and a green line; current = green ring and ink label; upcoming = gray.
- **Live rows** use a pulsing dot in the same slot as a status dot, never a different layout.

## 18. Dark mode, end to end

Every screen has a dark artboard (`<name>-dark`) next to its light one on the canvas, grouped by flow. The token values are in section 11. Layering rules:

1. The page is `#111513`. Fields are sunken: darker than the page (`#0c0f0e`) with an inner shadow. Cards, tables and dialogs are raised above the field (`#1a201e`) with a hairline `#252c29` border and a faint top highlight; there are no white borders.
2. Text `#e9efec`, muted `#8b9892`, faint `#6a7570`. Inline colors in legacy markup map: `#3f3f46` to `#c9d3ce`, `#71717a` to `#8b9892`, `#a1a1aa` to `#6a7570`.
3. Status badges use dark tints (section 11); the critical badge stays solid. Dots keep their colors.
4. Menus, popovers and dialogs use deeper shadows (`0 12px 32px -8px rgb(0 0 0 / .7)` and `0 24px 60px -12px rgb(0 0 0 / .75)`); cards use no outer shadow.
5. Charts: histogram info bars `#3c6b55`, warn `#f5b942`, error `#e5484d`; uptime strips keep their greens.
6. Inputs `#0f1311` with `#2a312e` border; checkboxes unchecked `#0f1311` with `#3a4540`, checked `#2a9f69`.
7. Sign in: the brand panel fades into the page color (not white). Public status pages use the same dark tokens with the hero unchanged. CLI screens are terminals and keep their own dark palette in both themes.
8. Theme switch lives in the account menu (Light, Dark, System). Default System.
