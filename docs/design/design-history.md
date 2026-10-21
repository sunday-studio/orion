# Orion Design Language v2

Redesign of 2026-10-15, revised the same day: top navigation kept, all corners square, flat surfaces with hairline borders (direction taken from incident.io, Better Stack and Midday). This supersedes the earlier Console-parity pass: the visual system, shell and every screen were rebuilt on the design canvas. The goal is a console that is calm and dense, readable at 3 a.m., and obvious about what needs attention. The canvas (artifact `Orion Console Design`) is the visual source of truth; this file records the rules.

## 1. Principles

1. **Status first.** Every screen opens with what is wrong, then what to do. Color means status and nothing else.
2. **Density with air.** 44 to 48px table rows, square cards, 36px gutters. Numbers are tabular, ids are mono.
3. **Always an action.** Alerts, failures and empty states end in a button: acknowledge, retry, add, fix.
4. **Show the shape of data.** Sparklines in lists, a real chart on detail pages, uptime strips for history.

## 2. Shell

Navigation is unchanged from the live Console: a 56px top bar on the ground color with the logo, the horizontal nav (Overview, Incidents, Servers, Monitors, Alerts, Status, Logs, Settings), a health label, avatar and sign-out icon. The active link is dark with a 2px underline. Content max width 1280px, 24px gutters. Overview is the one addition to the shipped nav. The command palette stays on `⌘K` without a visible sidebar search.

## 3. Color

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| ground | #f5f5f4 | #0c0a09 | sidebar, inset areas |
| surface | #ffffff | #141210 | content, cards |
| subtle | #fafaf9 | #1c1917 | table header, hover |
| border | #e7e5e4 | #292524 | all dividers |
| text / muted | #1c1917 / #57534e | #fafaf9 / #d6d3d1 | body / secondary |
| primary / accent | #1f7a50 | #4cba83 | primary button, active tab underline, focus ring (the one green) |
| deep green | #142a23 | #0a1f18 | announcement strip, login panel, "expected" compare card |
| mint | #80ffaa | #80ffaa | text and bullets on deep green |

Status (light bg / text; dark uses 950 bg / 300 text): up teal, down rose, degraded amber, unknown stone, maintenance violet, stale slate, covered cyan, resolved blue. Critical severity is a solid rose chip. Rose, amber and emerald tint a card only to signal state.

## 4. Status badges

Label plus shape plus color: up filled dot, down and open filled square, degraded half dot, unknown and stale hollow dot, maintenance pause bars. Sentence-case labels ("Up", "Acknowledged"). Never color alone.

## 5. Type

- GT America Standard is required (licensed). The canvas cannot embed it and shows a stand-in; the Console must self-host the licensed files.
- Page title 26/600, section title 15/600, body 14/400, meta 12 muted, mono 12 for ids, URLs and payloads. Tabular numerals everywhere.
- Instrument Serif only for hero statements (Overview hero, setup card, login, public pages).

## 6. Components

- **Stat tile**: white card, status dot and label, 26px value, optional sparkline. Selected tile gets the accent ring.
- **Card**: 1px hairline border, square corners, no shadow. No colored top bars.
- **Table**: square bordered box, small uppercase header, 48px rows, row hover, no vertical rules. Filters sit above as search, selects and chips. Pagination below.
- **Tabs**: underline, accent on the active tab. **Segmented control** for modes (Start now / Schedule).
- **Chip**: pill for filters and type pickers.
- **Buttons**: 36px (30px small), square corners; primary indigo-950, secondary outline, ghost, destructive rose.
- **Dialog**: square, 1px border, 520px, 45% scrim, close control. **Drawer**: 460px from the right.
- **Charts**: area chart with dashed gridlines and optional threshold line; uptime strip, one bar per day; sparklines 88x26.
- **Callout / alert bar**: flat tinted bars for inline messages.

## 7. Motion and accessibility

150ms hover, 200ms dialogs and drawers, live rows fade over 1.2s; reduced motion keeps opacity only. AA contrast in both themes, 3px accent focus ring, status never color-only, targets at least 30px.

## 8. Voice

Plain, operational, calm. Lead with the fact, then the action. Sentence case, no exclamation marks. "api.example.com is down. 3 checks failed since 14:02." Public copy never exposes internal ids or raw payloads.

## 9. Parity with the live Console

The live Console (`apps/console`) still uses the previous look. Implementation order is in `proposals.md` section 3; the shell, tokens and shared components (X1, X2, X4) come first so screens can migrate one at a time.

## 10. OpenFX-derived pass (2026-10-16)

Applied from `openfx-design-language.md`, keeping top navigation and square corners. Decisions taken on the three open questions: "Up" moves to the OpenFX green everywhere (status teal is retired); table headers, tile labels, eyebrows and breadcrumbs use IBM Plex Mono 11px uppercase with +0.08em tracking; the dot-matrix texture replaces the diamond glyph in empty states.

- One green (#1f7a50 primary, #2a9f69 status and ticks, #4cba83 uptime bars) replaces indigo and teal.
- Stat strips: tiles join into one bordered band with hairline dividers and 34px numerals.
- Giant numeral on Overview ("30m 12s") and in compare cards.
- Compare cards: deep-green "Expected" beside gray "Actual" on incident detail.
- Choice tiles for OS and check type pickers (Add server, Agent monitor).
- Announcement strip above the header for scheduled maintenance and updates.
- Login panel in deep green with mint mono figures.
- Public status page: centered 970px column, one status statement, stat strip, services in a gray panel with mono uptime, active incident.

## 11. Full OpenFX adoption (2026-10-16, second pass)

Orion's earlier look is deliberately eroded. Top navigation is the only fixed constraint. Square corners were dropped in the third pass (section 12).

- **Dot-matrix ticker** as the signature graphic. A 5x7 LED font rendered as dots (mint on deep green, peach for warnings, a faint unlit grid behind). Used for the Overview hero ("API-HTTPS DOWN"), the first-run card ("NO DATA YET"), the login panel, and the hero of every public status page ("2 OF 3 AFFECTED", "SUBSCRIBED", "INVESTIGATING").
- **Deep-green hero panels** (`#142a23`) with a white 40px statement, mint mono eyebrows and a giant mint numeral. Primary action is mint on deep green.
- **Selected stat tile inverts** to deep green with mint label and white numeral.
- **Mono eyebrow above every page title** ("INCIDENTS · LAST 7 DAYS"), 35px page titles, and a mono footer line on every app page ("ORION 0.9.2 · SELF-HOSTED · CORE HEALTHY · ALL TIMES UTC").
- **Step strips** ("STEP 1 / STEP 2" in mono) for Add server and Agent monitor setup; **choice tiles** for pickers.
- **Dark code panels** with mono tab labels for heartbeat snippets.
- **Compare cards** (deep "Expected" beside gray "Actual") and public cards that mix one deep card with gray ones.
- **Public pages** share one structure: logo and actions, ticker hero, stat strip or panels, mono captions.

## 12. Shapes follow OpenFX (2026-10-16, third pass)

The earlier "no rounded corners" rule was removed because OpenFX, the reference, is soft: pill buttons, 12 to 16px cards and panels, hairline shadow.

- Buttons, chips, status badges, segmented controls and search fields are pills.
- Cards, tables, stat strips, tiles, choice tiles and callouts: 12px radius, `0 1px 2px rgba(0,0,0,.10)` shadow. Gray panels (`#f9f9f9`) have no shadow.
- Hero panels 16px, dialogs 16px (no border, large soft shadow), drawers round on the left edge, inputs 8px, code panels 10px.
- Avatar and status dots stay round; the top navigation is unchanged.

## 13. Refinement pass: layered surfaces, no pills (2026-10-16, fourth pass)

Feedback: fully rounded controls and the flat treatment felt cheap. Direction taken from Cloudflare (layered buttons, inset highlights, soft outer glow) and Better Stack (quiet type, thin borders). This section supersedes section 12 for shape.

- **Radii**: controls 6 to 7px, cards and tables 10px, hero and dialogs 14px. No pills; badges 5px.
- **Layering**: every raised surface stacks a 1px border, an inset top highlight, and a soft drop shadow (`--hl`, `--s1`, `--s2`, `--s3` tokens). Dialogs and drawers use a large two-layer shadow with a hairline ring; scrim blurs 3px.
- **Buttons**: secondary is a white-to-gray gradient with border, inset highlight and 1px shadow; primary is a green gradient with a darker border, inset top highlight, inset bottom shade, tight and wide shadows; destructive is the same in rose; pressed state is inset. Hero buttons are translucent glass, and the primary is mint.
- **Inputs**: inset shadow plus a 1px bottom highlight, green focus ring (3px).
- **Quieter type**: mono is now only for ids, code, hero eyebrows and the ticker. Table headers, tile labels and breadcrumbs are sans. The per-page mono eyebrow and footer were removed. Page title 28/600.
- **Hero**: deep green gradient with a soft mint glow top-right, inner highlight and a long shadow; the ticker is smaller (6px pitch) so it reads as an accent.
- **Selected states** (tile, chip, step, page item) use a deep-green gradient with an inner highlight.
- **Neutrals** moved to zinc (`#18181b` text, `#e4e4e7` borders) for a cleaner gray.

## 14. Density pass (2026-10-16, fifth pass)

Feedback: the system felt too spacious. This pass supersedes the spacing in sections 10 to 13 and targets Better Stack's density.

- Base type 13px; page title 22/600; section 13/600; table 13px.
- Header 48px, announcement strip 28px, page padding 16/20, section gap 10 to 14px.
- Controls 30px (small 24px), radius 6px. Inputs 30px. Badges 18px line. Chips 24px.
- Table header 30px, rows 38px, cell padding 12px. Stat tiles 68px high, 22px numerals; 8 and 7 tile strips 60px.
- Cards 12/14 padding; panels 14/16; dialogs 18px, 480px wide; drawers 420px.
- Hero 16/20 padding, 26px statement, 38px numeral, 34px-high ticker (left aligned).
- Layered shadows, 10px card radius and green accent from section 13 are unchanged.

## 15. Layout experiments: Incidents (2026-10-16)

Layouts are no longer sacred. The Incidents section was rebuilt as a test bed:

- **List**: a seven-day swimlane (one lane per source, bars colored by state, day grid) above a master-detail split. The left list is dense two-line rows with a state rail, severity and duration; the right pane shows the selected incident (facts strip, Expected vs Actual, activity feed) without leaving the list. J/K move between rows.
- **Detail** ("war room"): Expected vs Actual on top, an activity feed with a note composer in the center, and a 300px rail with status clock, responders, impact, evidence chart and public status.
- **Candidates** became a keyboard triage inbox (A accept, D dismiss) with a preview of the signal and threshold chart.
- **Health issues** are grouped by state with collapsible-style group headers.
- Patterns introduced: swimlane, split view, facts strip, activity feed, composer, kbd hints.

## 16. Dark mission-control direction (2026-10-16)

The visual system was replaced rather than tuned. Dark is now the default theme for the whole Console; light is an opt-in theme (`.light`, shown on the "Overview, light theme" artboard). Public status pages stay light. Top navigation is unchanged.

- **Ground**: near-black `#070a0c` canvas with a faint 28px grid, panels `#0d1316` with a hairline `#1a232a` border and an inset top highlight.
- **Accent and status as light**: mint `#4ef0a0` (primary, up), red `#ff4d6d` (down, critical), amber `#ffb020` (degraded), violet (maintenance). Badges are translucent tints with bright text; down, up and critical badges carry a soft glow. The primary button is a mint gradient with a glow.
- **Numbers are mono**: KPI values, clocks and the hero timer use IBM Plex Mono with tight tracking.
- **Live stream rail**: a persistent 236px right rail on every screen with a pulsing LIVE header, All/Errors/Alerts filters, and timestamped events (errors tinted red, warnings amber, recoveries mint). It replaces the old "Recent activity" blocks.
- **Overview**: a red-glow incident hero with a dot-matrix ticker and a large red timer, a KPI strip with sparklines, and a **monitor wall** (one glowing cell per monitor, red and amber cells stand out), then open incidents and needs-attention lists.
- **Incident screens** keep the experimental layouts from section 15: Expected is mint-tinted and Actual red-tinted.
- **Light theme** keeps the layered, dense surfaces from sections 13 and 14.

## 17. Calm direction: one question per screen (2026-10-16)

Supersedes sections 15 and 16 for the main screens. The dark mission-control look was "too much"; the brief became: present only what the operator must act on, in the OpenFX manner. Light is the default again; dark remains an artboard.

- **Each screen answers one question in one sentence**: a mono eyebrow, a centered 32px answer ("One thing needs you.", "3 monitors need attention.", "5 of 6 servers are reporting."), and one line of context.
- **Only actionable items are shown in full.** Healthy things collapse into a single line: a row of small status squares plus a count, with "Show all".
- **OpenFX structure**: pale `#f7f7f6` fields (14px radius) holding white product cards (10px radius, soft shadow); a 780px centered column; a hairline stat row of three numbers at the bottom.
- **Tracking timeline** (from OpenFX's withdrawal tracker): Detected, Notified, Acknowledged, Resolved, with green checks and the current step ringed in red. Used on Overview and incident detail.
- **Sentences over tables**: incidents read as sentences ("web-https was slow for 22 minutes"), grouped by "Open" and "Earlier this week", with "Show 18 more".
- **Expected vs Actual**: deep green card beside a white card with the failing value in red.
- Rebuilt so far: Overview, Incidents, Incident detail, Monitors, Servers. Other screens use the light layered system from sections 13 and 14 until they are rebuilt in this style.

## 18. Linear / Height structure (2026-10-16)

References (Mobbin): Linear issue detail, project overview and grouped issue lists; Height spreadsheet lists with filter chips and view switchers. The calm voice of section 17 stays; the structure changes.

- **Content is never full width.** The app frame is gray (`#f4f4f5`); content sits in an inset white panel (8px from the edges, 10px radius, hairline border). Inside, lists cap at 980 to 1080px and documents at 640px.
- **Every page has a 44px toolbar** inside the panel: breadcrumb, view tabs (All / Open / Mine; List / Board / Uptime), filter and display icons, and the single primary action on the right. Actions live here, not in the page body.
- **Lists are grouped and collapsible** (Open, Acknowledged, Resolved; Needs attention, Healthy). Rows are 38px: mono id, status glyph, title, label chips, severity bars, assignee avatar, relative time.
- **Status glyphs** replace colored badges in lists: ring (open), half ring (acknowledged or degraded), filled check (resolved), red square (down or urgent), dot (up). Severity uses Linear-style signal bars.
- **Filter chips** (Height): `Status | is any of | Open, Acknowledged` with "+ Filter".
- **Detail pages are documents with a Properties rail**: a centered 640px column (title, one-paragraph summary, compact lifecycle track, Expected vs Actual, Signals as sub-items, Activity with inline comment and composer) and a 270px right rail (Properties, Labels, Public status, Related). Monitor detail follows the same pattern with Check, Routing and Stats groups.
- **Overview is an inbox**, not a dashboard: a greeting sentence, a four-cell pulse strip, then "Needs you" (with the action inline), "Watching", "Resolved today"; system health and service uptime move to the right rail.
- **Spreadsheet lists** for Monitors and Servers with group rows and inline sparklines.

## 19. Monitoring-domain patterns (2026-10-16)

Section 18 borrowed task-manager structure (Linear/Height); feedback was that Orion is not a task manager. Screens now follow monitoring and incident tools found on Mobbin: incident.io (incident list, filters, incident detail), Better Stack (monitors, monitor detail, home), and service-health pages (OpenAI Platform). The inset content panel from section 18 stays, so content is never full width.

- **Page header** inside the panel: icon tile, title, one line of context, actions on the right (Export CSV, Filter, primary).
- **Filters are filters**: a Filter button opens a category menu (Status, Severity, Monitor, Server, Opened); choosing one shows an operator ("is one of") and grouped checkboxes (Active / Closed) with Done. Applied filters appear in a "Filtering by" bar as removable tokens, with a result count and "Clear all filters". Search is a separate field. Quick status segments (All / Down / Degraded / Up) sit beside search on Monitors and Servers; secondary attributes (type, server, platform) are dropdowns.
- **Incident rows** are cards: id and title, then chips for severity, status, duration, source monitor and responder, with select checkboxes for bulk actions and a chevron.
- **Incident detail**: title with a lifecycle stepper (Detected, Open, Acknowledged, Resolved), severity and "Ongoing for" chips; actions top right. Main column: summary, Expected vs Actual, tabs (Timeline, Checks, Notifications), note composer above a timeline. Right column: Source, People, Timestamps, Duration (time to ack and resolve), Status page, Related incidents.
- **Monitors** are grouped by state (Down, Degraded, Up) with name, target, state and how long, check interval and a 30-check strip. **Monitor detail** follows Better Stack: three facts (currently down for, last checked, incidents), response-time chart with location and Day/Week/Month, and an availability table by period (availability, downtime, incidents, longest, average).
- **Overview** follows a service-health page: a red banner only when something is ongoing, ongoing incidents table, service health with 90-day bars, and a right column with three numbers and recent incident history.

## 20. De-Linear pass (2026-10-16)

Feedback: still too much like Linear. Removed the parts that read as Linear: the gray app frame with an inset panel, icon-tile page headers, ring and half-circle status glyphs, signal-bar severity, bordered metadata chips and mono ids.

- Pages are plain white with a centered column capped at 1040px (still not full width); a 28px title and one line of context lead each page.
- Status and severity are text badges (Open, Acknowledged, Critical, High; Down 30m, Slow, Late). A small colored dot marks state in lists. Metadata is plain muted text separated by dots ("Open for 30m 12s · api-https on edge-01 · No responder yet").
- Grouping uses the OpenFX language: pale `#f7f7f6` field headers, white rounded cards with a soft shadow, the tracking timeline on incident detail, the green chart line.
- The functional pieces stay: real filters (Filter menu, "is one of", grouped checkboxes, Filtering-by bar with Clear all), status segments, availability table, Expected vs Actual, timeline with composer, side facts panel.

## 21. Propagation pass (2026-10-16)

The patterns the owner approved on Overview, Incidents and Add server now apply to every product screen:

- Centered column: 1040px for lists, 760px for forms and settings; tabs centered.
- Content groups sit in a pale `.field` holding white `.pc` cards (OpenFX container pattern).
- Filters are real filters (Filtering by tokens, popover with "is one of"), not task-board chips.
- Statuses are dot plus text badge; no pills.
- Dialogs (incident actions, create monitor, test result, maintenance, delete, command palette) float on a scrim over the real page; report inspection is a right drawer.
- Delivery detail is a page, not a drawer.
- Rebuilt: Alerts (4), Status pages (5), Logs (2), Settings (2), Server detail, metrics, token, Triage, Health issues, Heartbeat and Agent monitor setup, all dialogs.
- Not yet converted: dark Overview artboard, states gallery, flows, design-language artboard.

## 22. Header rethink and comment pass (2026-10-16)

- Header: logo left; the eight sections collapse to seven in a centered tray (active = white chip); right side holds a search/⌘K field, a health chip ("2 issues"), a Settings gear and the avatar. Sign out moves into the avatar menu.
- Badges: icon and text are one centered flex row (fixes the System states misalignment everywhere).
- Tables: one `.gt` grid table (header row, 38px rows, one line each) replaces card lists. Used by Monitors, Servers, Incidents, Alerts log, Logs, Subscribers, Delivery attempts, Server reports. The monitor target is its own column.
- Filters follow Plain: segmented tokens (field | operator | value | x), a dashed "+ Filter", and Clear all; the bar spans the column and wraps instead of clipping. Checkboxes are custom (green, 4px radius).
- Tabs: the rule sits only under the tabs, not full width.
- Page header: title, one muted line, actions right. Status pages show "Example Status" with a Draft badge and its URL.
- First run: one sentence, one button. Dark Overview is Main in dark.
- Lists use the 1040px column; detail and form pages use 760px.

## 23. Typeface (2026-10-16)

Alegreya Sans (Google Fonts, 400/500/700) replaces GT America and Instrument Serif everywhere, superseding the earlier "require GT America" decision. Tabular lining figures (`tnum`, `lnum`) are on for all text so numbers align in tables, stats and timestamps. IBM Plex Mono remains only for code blocks and keycaps. Alegreya Sans has no 600 weight, so semibold maps to 700. A `font-size-adjust` of .54 compensates for its small x-height.

## 24. Typeface decision (2026-10-17)

TWK Lausanne (300 for text, 500 for emphasis) is the Console typeface, replacing the Alegreya Sans placeholder from section 23. PolySans and GT America were trialled on list pages and dropped. Tabular lining figures stay on everywhere. IBM Plex Mono remains for code blocks and keycaps only. Lausanne ships only 300 and 500 here, so there is no semibold. Licensing: the files in `fonts/` are trial builds; buy the web licence before shipping, and self-host the woff2 files in the Console.

## 25. Rules from the flows pass (2026-10-17)

- **One-line rows.** Every row in a table or card list is a single line. A subline becomes its own column ("Detail", "Target", "Type"), never a second line under the name. Applies to `.gt` tables and `.lr` list rows.
- **Issues first, then a link.** Where an entity has many children (monitors on a server), show those with problems first, at most 10, then link to the full list filtered by that entity.
- **Channels are Webhook and Email only.** Webhook: URL, signing secret, optional headers, payload preview, send test. Email: recipients as chips, each confirmed once through a public confirmation page, instant or digest delivery.
- **Logs at scale.** Four views over one query bar: List (histogram plus facets plus dense one-line rows, cursor paging), Live tail (pausable), Patterns (similar messages grouped), and a detail drawer with surrounding events. Time range and saved views are menus; export runs in the background.
- **Empty states are square-cornered containers**, not rounded cards sitting under a square header.
- **Every destructive or one-time-secret action has its own screen:** token reveal once, remove server with a monitor-count preview, unpublish page.

## 26. Coverage added (2026-10-17)

New screens: channel type picker, webhook form, email form, channel test results, channel detail, email confirmation page; server monitors tab, no-reports state, edit, remove, new token, connected, not connected, empty list; events explorer, service logs explorer, live tail, patterns, event detail, time range and saved views, export; declare incident, bulk actions, none open, resolved detail, public update; monitor configure, bulk actions, empty, TCP form, paused; status pages list, create, custom domain, schedule maintenance, publish and unpublish; account menu, keyboard shortcuts, Overview all good; settings general, API tokens, new token, audit log. The Flows board now holds twenty journeys.

## 27. "Agent" renamed to "Node" (2026-10-21)

Decision D-047. The software that runs on a machine is now the **Node**; the machine itself stays a **Server** in the Console ("Node not reporting", "Install the Node"). The CLI is `orion-node`. The canvas boards, `docs/design/*` and the exported artboards use the new words; layouts are unchanged. Entries above keep the wording of their day. The artboard PNGs were not re-rendered (the TWK Lausanne trial font is not in the repo, D-010), so the images of the renamed boards still show "agent"; the HTML exports are current.
