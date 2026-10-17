# Orion Design Language

Matches the live Console (checked against `apps/console/src` on 2026-10-14). Canvas styles come from that source, not from the earlier draft. Existing Console look (`apps/console/src/index.css`, `components/ui`, `components/shared`). Existing traits to keep: white work surface on neutral-100 chrome, flat square corners, thin neutral borders, small flat status badges, teal/rose/amber semantics, GT America Standard body at 14px, Instrument Serif for rare display moments, dotted diamond empty-state glyph, wavy `hr`.

## 1. Color tokens

Neutrals use Tailwind `neutral`. Light is the default; dark is an extension.

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| `--bg-canvas` | #f5f5f5 (neutral-100) | #0a0a0a | app chrome behind surface |
| `--bg-surface` | #ffffff (body #fdfdfc) | #111111 | main work area |
| `--bg-subtle` | #fafafa | #171717 | table header, inset |
| `--border` | #e5e5e5 | #262626 | all dividers |
| `--text` | #111111 | #ededed | primary |
| `--text-muted` | #737373 | #a3a3a3 | secondary, descriptions |
| `--accent` | #1e1b4b (indigo-950) | #e0e7ff | primary button; nav active text indigo-700 #4338ca |
| `--focus` | #0d9488 | #2dd4bf | focus ring (teal-600/400) |

## 2. Status semantics

Badges are lowercase text labels on a tinted fill (no glyph), as in the Console. The label carries the meaning.

| Status | Light bg / text | Dark bg / text | Glyph | Meaning |
|--------|----------------|----------------|-------|---------|
| up | teal-100 / teal-900 | teal-950 / teal-300 | filled dot | checks passing |
| down | rose-100 / rose-900 | rose-950 / rose-300 | filled square | check failing |
| degraded | amber-100 / amber-900 | amber-950 / amber-300 | half dot | slow or partial |
| unknown | neutral-100 / neutral-900 | neutral-900 / neutral-300 | hollow dot | no data yet |
| maintenance | violet-100 / violet-900 | violet-950 / violet-300 | pause bars | planned |
| stale | slate-100 / slate-900 | slate-900 / slate-300 | clock | no report in window |

Incident lifecycle: open (rose), acknowledged (amber), covered (cyan), resolved (blue). Severity: low teal, medium yellow, high rose, critical red-400 solid. Notification: pending neutral, sent teal, failed rose, suppressed slate, cooldown cyan.

## 3. Type

- Body: GT America Standard, 14px / 1.5, weight 400 (variable opsz 14). The licensed face is required; no system-sans fallback is part of the design. The design canvas cannot embed it and previews show a stand-in. Self-host the licensed files in the Console.
- Page title 24px / 500 (`PageHeader`). Section title 16px / 500. Label 12px / 500 uppercase-free. Meta 12px muted.
- Mono (tokens, URLs, payloads, ids): `ui-monospace`, 12px.
- Instrument Serif: only empty states, the Overview setup card and the public status page heading.
- Numbers in tables and stat cards: `tabular-nums`.

## 4. Spacing, radius, elevation

- 4px base: 4, 8, 12, 16, 24, 32, 40. Page gutter 16 (mobile) / 24; content max 80rem; page top padding 40.
- Radius: 0 for tables, badges, inputs, buttons (current look). Dialogs and popovers 0 with 1px border. Only avatars and status dots are round.
- Elevation: flat by default. Dialog and dropdown get `0 8px 24px rgba(0,0,0,.08)` plus 1px border. No shadow on cards.

## 5. Core components

App header (56px, neutral-100, no rule; logo, nav, health text, avatar, sign-out icon; 8 nav links with Overview first, overall health pill, sign out), page header, tabs (underline), summary stat cards, badge (status, severity, notification), button (primary ink, secondary outline, destructive rose, ghost), input / select / combobox / textarea / checkbox, dialog, drawer (report inspection), dropdown menu, breadcrumbs, empty state, detail card / detail group, uptime bar (SLA), radial avatar, pagination / infinite scroll, flow node (alert rules), toast, skeleton, banner (offline, degraded Core), code block with copy.

## 6. Data-table pattern (OpenStatus style)

Required for every log-like table (AGENTS.md Console Table Rules): reports, monitor checks, incident timelines, events, alert deliveries, service logs.

- TanStack Table with schema-driven column definitions; TanStack Query for data; `nuqs` URL state for filters, sort, page.
- Layout: left collapsible filter panel (status, severity, source, time range, free text), top toolbar (search, live toggle, column visibility, refresh), dense rows (36px), sticky header, monospace for ids and messages.
- Row click opens a right-side detail drawer with key/value list, raw JSON, and prev/next row controls.
- Cells: relative time with absolute on hover, status badge, truncated message with copy.
- Server-side pagination and filtering via Core query params; footer shows "n of total" and page size.
- States: loading skeleton rows, empty (with active-filter reset), error with retry.

## 7. Iconography

lucide-react, 16px, 1.5 stroke, `currentColor`. Status glyphs above replace icons in badges. The diamond-dot glyph marks empty states. No emoji.

## 8. Motion

150ms ease-out for hover, tabs and dropdowns; 200ms for dialogs and drawers (opacity + 8px translate); live-update row highlight fades teal-50 to transparent over 1.2s; skeleton shimmer 1.4s. Respect `prefers-reduced-motion` (disable all but opacity).

## 9. Voice

Plain, operational, calm. Lead with the fact, then the action. Sentence case, no exclamation marks, no blame. Examples: "api.example.com is down. 3 checks failed since 14:02." / "No servers yet. Install the agent to start collecting reports." / Errors say what failed and what to try. Public copy never exposes internal ids or raw payloads.

## 10. Accessibility

AA contrast in both themes, visible 2px focus ring, status never color-only, tables keyboard navigable, dialogs trap focus, targets at least 32px.

## 11. Console parity notes (2026-10-14)

- Header 56px on neutral-100, nav 14px neutral-700, active link indigo-700 with no underline; right side shows a health label, avatar and sign-out icon.
- Tabs are a segmented control: 28px high, 1px neutral-800 border, active tab filled neutral-800 with white text. Status Pages uses a two-button variant of the same.
- Summary tiles: 104px dithered tiles, neutral-100 when unselected, tinted when selected (Incidents 6, Servers 8, Monitors 7).
- Detail cards: 8px radius, 1px neutral-200 border, shadow-xs, 2px accent bar on top (neutral, indigo, emerald, amber, rose, violet).
- Tables: no outer border, 40px header, 8px/12px cell padding, faint row rules, emerald-50 hover. Log-like tables are plain tables with select filters above, not the left filter panel the earlier draft showed.
- Buttons 12px/500, 36px (32px sm), square. Outline buttons use a dark 1px border. Destructive is rose.
- Dialogs: no border, 24px padding, 512px wide, shadow-lg, 50% black overlay.
- Not in the live Console yet and designed here: Overview, Routes tab, delivery detail, candidates, onboarding card, settings security and system, public pages.
- GT America Standard is required. The canvas shows a stand-in because the font is licensed.
