# Public Status Page IA And Design Breakdown

Side ticket: `T-20260529-191906-7006` [Certain]

## Scope

This note breaks down the supplied public status page references at the information architecture and visual design levels. [Certain]
It is guidance for Orion public status pages, not a requirement to copy any single reference exactly. [Certain]

## Reference Patterns

The references show two dominant public status page models. [Certain]
The first model is a focused uptime timeline page with a top navigation, an overall state banner, component uptime bars, and recent incident/event history. [Certain]
The second model is a denser system-status table with grouped services, date-window controls, compact uptime bars, and expandable component groups. [Certain]

Useful patterns to preserve:

- Put the brand and primary status state in the first viewport. [Certain]
- Keep the overall state banner above component detail. [Certain]
- Show component uptime as a scan-friendly strip of daily bars. [Certain]
- Pair each component with both a human status label and a numeric uptime value when data exists. [Likely]
- Keep incident history below current health unless an incident is active. [Likely]
- Make subscription/update access visible but secondary to current status. [Likely]
- Support light and dark theme variants without changing the information hierarchy. [Likely]

Patterns to avoid copying directly:

- Do not make public pages depend on a large authenticated Console bundle. [Certain]
- Do not expose internal monitor, server, or incident identifiers in the public layout. [Certain]
- Do not make decorative branding stronger than service health readability. [Likely]
- Do not require every page to use the same exact visual density. [Likely]

## IA Level

### Primary Page Structure

Recommended top-level structure:

1. Header: brand mark, page title or organization name, optional navigation, update subscription action. [Likely]
2. Hero status summary: current overall state, status icon, last updated timestamp, optional one-sentence explanation. [Likely]
3. Active incidents and scheduled maintenance: shown immediately below the status summary when present. [Likely]
4. Component health: grouped component list with current status, uptime bars, and uptime percentage. [Likely]
5. Recent public events: published incidents, maintenance entries, and resolved issues. [Likely]
6. History access: link or control for deeper incident/event history. [Likely]
7. Feed and badge links: secondary utility links for Atom feed and embeddable badges. [Likely]

### Navigation Model

The references use `Status`, `Events`, and `Monitors` as simple public navigation. [Certain]
For Orion, `Status`, `Events`, and `Components` is clearer than `Monitors` because public readers should not learn or reason about internal monitor topology. [Likely]

Recommended public nav:

- `Status`: current state, uptime bars, active incidents, and recent events. [Likely]
- `Events`: full public incident and maintenance history. [Likely]
- `Components`: component or group detail, including uptime history and public description. [Likely]
- `Get updates`: subscription entry point. [Likely]

### Component Grouping

The references show both flat component lists and grouped regional/product cards. [Certain]
Orion should support both through existing status page sections. [Certain]

Recommended section behaviors:

- Flat page: sections render as subtle headings or no visible container when there is only one section. [Likely]
- Grouped page: sections render as bordered bands or compact panels with section-level status. [Likely]
- Component rows remain consistent across section styles. [Likely]
- Hidden components never appear in public navigation, counts, subscriber preferences, or uptime summaries. [Certain]

### Uptime Bar IA

The uptime strip is the strongest recurring pattern in the references. [Certain]
It works because it compresses 45 to 90 days of reliability into one scannable row. [Likely]

Recommended uptime data model for the public UI:

- Window: support `24h`, `7d`, `30d`, and `90d`, with `90d` as the default public history view. [Likely]
- Buckets: daily buckets for `30d` and `90d`, hourly or coarser buckets for `24h` if supported later. [Likely]
- Color states: operational, degraded, outage, maintenance, and no data. [Likely]
- Tooltip: date, state label, and duration or uptime value. [Likely]
- No-data state: neutral bars, not green bars. [Certain]

### Incident And Event IA

The references put recent incidents below uptime when all systems are operational. [Certain]
When there is an active incident, Orion should elevate active incident cards above component uptime. [Likely]

Recommended event hierarchy:

1. Active incident or active maintenance. [Likely]
2. Scheduled maintenance that has not started. [Likely]
3. Recently resolved incidents. [Likely]
4. Older incident history behind an events-history link. [Likely]

Each event should show title, public status, affected components, visible timestamp, and published public update text. [Likely]
Internal incident IDs, server names, raw monitor errors, and operator notes must not render. [Certain]

### Subscription IA

The references keep subscription controls in the header or hero area. [Certain]
Orion should expose `Get updates` in the header and repeat it near events only when subscriptions are enabled. [Likely]

Subscription flow IA:

- Entry: `Get updates` button. [Likely]
- Modal or page: email field plus optional component preferences. [Likely]
- Confirmation state: generic success message that does not reveal whether an address was already subscribed. [Likely]
- Manage preferences: token-based public self-service page with masked destination. [Likely]
- Unsubscribe: idempotent public action with generic success copy. [Certain]

## Design Level

### Layout

The references use a narrow centered content column, usually between 760px and 960px wide. [Likely]
Orion should use a centered public layout with generous top spacing, but keep the status summary and component rows above the fold on desktop. [Likely]

Recommended layout rules:

- Desktop max width: about 880px for the main public status page. [Likely]
- Mobile: single-column layout with compact status bars and no horizontal overflow. [Certain]
- Header: sticky is optional, but the first viewport should keep status content visible. [Likely]
- Cards: use cards for grouped components or event items, not for every page section. [Likely]

### Visual System

The references split into minimalist light, dense dark, and corporate table styles. [Certain]
Orion should provide a default restrained style with optional dark mode and accent-color branding. [Likely]

Recommended design tokens:

- Background: neutral surface, not a decorative gradient. [Likely]
- Text: high-contrast foreground with muted secondary text. [Likely]
- Accent: page-level accent color from sanitized theme settings. [Certain]
- Success: green, degraded: yellow/amber, outage: red, maintenance: blue or violet, unknown/no data: neutral gray. [Likely]
- Radius: small radii around 6px to 8px for banners, cards, and buttons. [Likely]
- Typography: system sans by default, optional monospace only for timestamps or dense uptime labels. [Likely]

### Status Summary Banner

The status summary should be the strongest single element on the page. [Likely]
It should include icon, status phrase, and last-updated timestamp. [Likely]

Recommended states:

- Operational: calm green banner. [Likely]
- Degraded: amber banner with specific affected count if available. [Likely]
- Partial outage: orange/red banner. [Likely]
- Major outage: red banner with active incident link. [Likely]
- Maintenance: blue/violet banner with scheduled window. [Likely]
- Unknown: neutral banner with no-data explanation. [Likely]

### Component Row Design

The component row should scan from name to history to current status. [Likely]
The references consistently keep the component name on the left and uptime/status on the right. [Certain]

Recommended row contents:

- Component name. [Certain]
- Optional public description or tooltip. [Likely]
- Uptime bar strip. [Likely]
- Uptime percentage when calculated. [Likely]
- Current public status with icon. [Likely]
- Date-window labels such as `90 days ago` and `today`. [Likely]

### Uptime Bars

The bars should be semantic data visualization, not decoration. [Certain]

Recommended bar behavior:

- Each bar has a stable width and gap. [Likely]
- Bars use rounded ends in the default theme and square bars in compact/dense themes. [Likely]
- Hover/focus tooltip shows date, status, and uptime/duration. [Likely]
- Keyboard focus should be available for bars or an adjacent accessible summary. [Likely]
- No-data bars should not imply success. [Certain]

### Events Design

Recent events should read as public communication, not as an operator log. [Likely]

Recommended event card:

- Date column on desktop. [Likely]
- Title and affected component chip. [Likely]
- Status timeline entries for investigating, identified, monitoring, resolved, or scheduled. [Likely]
- Public update body with comfortable line length. [Likely]
- Older events behind a history control. [Likely]

### Empty State Design

The references include a no-recent-notifications block. [Certain]
Orion should use a smaller empty state that does not push component health too far down the page. [Likely]

Recommended empty state:

- Show `No recent incidents` only after component uptime. [Likely]
- Include a `View history` link when historical data exists. [Likely]
- Avoid large illustrative skeleton cards on dense operational pages. [Likely]

## Orion-Specific Recommendations

### First Public Page Target

Build the first public page as a simple, high-trust operational page. [Likely]
Do not start with a highly branded marketing page. [Likely]

Minimum useful first viewport:

- Brand and page title. [Likely]
- `Get updates` action when subscriptions are configured. [Likely]
- Overall status banner. [Likely]
- At least the first three visible components with uptime bars. [Likely]

### Theme Modes

Support three presentation densities rather than many arbitrary visual options. [Likely]

- `standard`: open layout with rounded bars and recent events. [Likely]
- `compact`: denser rows for many components. [Likely]
- `dark`: dark color tokens with the same IA. [Likely]

### Admin Preview Implications

The Console preview should show the same public IA as the unauthenticated page. [Likely]
The preview should include uptime bars and recent public incidents, not only component names and current state. [Likely]

### Data Requirements

The public renderer needs component history buckets to fully match the references. [Certain]
Current public history endpoints already provide component uptime/history data that can feed this UI. [Certain]
The current public HTML path renders component state but does not yet render full uptime bar history. [Certain]

## Implementation Slices

1. Public renderer IA update: refactor public HTML to render status summary, component rows with uptime bars, active incidents, and recent events. [Likely]
2. Public DTO/data update: ensure public page response can include enough safe history summary for first-page rendering without N+1 endpoint calls. [Likely]
3. Console preview update: render the public layout or a close preview using the public DTO and history data. [Likely]
4. Theme token update: support standard, compact, and dark variants through sanitized theme settings. [Likely]
5. Accessibility update: add semantic labels for status banners, component status, uptime bars, and event timelines. [Likely]
6. Browser verification: add screenshots or E2E assertions for light, compact, dark, no-data, degraded, and active-incident states. [Likely]

## Open Decisions

- Whether public status pages should default to 45-day or 90-day uptime windows. [Guessing]
- Whether section groups should be expanded by default when there are many components. [Guessing]
- Whether `Monitors` should ever appear as public navigation text. [Guessing]
- Whether Orion should add a separate public bundle or keep improving the current Core-rendered HTML path first. [Guessing]

## Acceptance Checklist For Design Implementation

- First viewport shows the status answer without requiring scroll on common desktop viewports. [Likely]
- Mobile layout has no horizontal overflow from uptime bars. [Certain]
- Public pages expose no internal identifiers or raw operational details. [Certain]
- Active incidents appear before historical incidents. [Likely]
- Uptime no-data state is visually distinct from operational state. [Certain]
- Dark and light themes preserve the same reading order. [Likely]
- Subscription entry points are visible only when useful and safe. [Likely]
