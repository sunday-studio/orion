# Information Architecture Review

Status: proposed, awaiting review. 2026-11-07.
Related: [service-correlation-plan.md](service-correlation-plan.md), [product-logs-plan.md](product-logs-plan.md), [decision-log.md](decision-log.md) (D-012, D-017, D-018, D-026), [../product-logs.md](../product-logs.md), [../how-it-works.md](../how-it-works.md), [../design/screen-map.md](../design/screen-map.md), [incident-detail-information-architecture.md](incident-detail-information-architecture.md).

Sources: the design canvas (artifact `L7gFM2gipuTveU5hkGqMRY`, version `1794074058-acdb`, read from board HTML, not rendered), `apps/console` routes and tab definitions, and the docs above. Nothing was edited or published.

## 1. Short answer

Yes, pages can be consolidated, and the "apps and logs" problem is mostly about where things are stored, not about missing pages.

- **Move app (log source) setup from Settings to Logs.** Config sits next to the data it controls. Settings goes back to instance-wide things.
- **Fold Alerts and Status into one "Notify" item.** Both are "tell people", both are mostly set-once, and neither needs a top-level slot.
- **Do not give Services its own top-level slot yet.** Group Services, Monitors and Servers as one "Monitoring" item when Services ships. Servers + Monitors alone is not worth merging.
- **Make each noun mean one thing.** "Apps" has four meanings, "Service" has four, "Status" has two.

Result: 7 items today, 8 with the Services proposal, **6 now and 5 after Services**.

## 2. Inventory

### 2.1 Desktop, as designed (canvas) and built (Console)

Top bar: logo (to Overview), tray of tabs, search (Cmd-K), health chip ("2 issues", opens `/health-issues`), Settings gear, avatar menu (theme, timezone, shortcuts, sign out). Update menu in the header (`update-menu`).

```
Overview            /                      Needs you now, Server health, Ongoing incidents, stats, Recent incidents
                                           First-run card -> Add server, Create monitor, "connect an app" (/apps/new)
                    /health-issues         (no nav item; reached from chip and Overview)
Incidents           /incidents             filter: Active | Needs review | Resolved; Declare, bulk, export
                    /incidents/:id         timeline, ack/cover/resolve, public update
                    /incidents/triage      built route; design shows it as the "Needs review" filter
Servers             /servers               All | Needs attention | Update available; Add server (/servers/new)
                    /servers/:id           tabs: Overview | Reports | Monitors | Apps | Detected (n) | Token
                                           dialogs: Edit, Maintenance, Remove, Track storage, Review detected
Monitors            /monitors              All | Down | Degraded | Up; Create (HTTP, TCP, heartbeat, Node)
                    /monitors/:id          built tabs: Overview | Check history (design: Day | Week | Month range)
                    /monitors/:id/edit, /monitors/new/node
Alerts              /alerts                tabs: Notification log | Channels | Rules | Routes
                    /alerts/channels/:id, /alerts/deliveries/:id
Status              /status-pages          list
                    /status-pages/:id      tabs: Setup | Components | Incidents | Subscribers | Preview
                                           (built adds Maintenance; design draws maintenance as a board without that tab)
                    public pages (Core renders): status, history, incident, subscribe, manage, unsubscribe
Logs                /logs                  tabs: Service logs | Status logs | Internal logs
                                           views: List | Live | Patterns; Copy link, Save view, Export
                                           state in URL: tab, view, q, range, source, severity, event, env, type, server, monitor, component
Settings (gear)     /settings?tab=         Data lifecycle | General | Apps | Security | System | Audit log
                    /settings/apps/:id     per app tabs: Ingest | Retention | Display | Redaction
Outside the bar     /apps/new              Connect an app (4 steps); highlights Overview, not Logs
                    /servers/new           Add server wizard
                    /login, Cmd-K palette, shortcuts sheet, onboarding card
Proposed            Services (8th item)    /services list; /services/:id tabs: Overview | Monitors | Logs | Resources | Incidents | Settings
                    plus changes on 10 existing boards (affected-*, service-*)
```

Counts: 8 bar items (with Services), about 40 tabbed views behind them, 3 views inside Logs, 3 levels deep in two places (Status > page > tab, Settings > Apps > app > tab).

Where one thing is reached from several places:

| Thing | Entry points |
|---|---|
| Create an app (log source) | Settings > Apps "New app" dialog; `/apps/new` wizard (Overview first-run, Logs empty state); Server > Detected "Create log key"; Services "Link app" (proposed) |
| View an app's logs | Logs, Source filter; app page "View logs" (`/logs?source=id`); Service > Logs (proposed) |
| Create a monitor | Monitors > Create; `/monitors/new/node`; Server > Detected "Accept"; Service "Link monitor" (proposed) |
| A server's monitors | Monitors list (Server filter); Server > Monitors tab; Detected |
| Public incident update | Incident detail; Status > Incidents tab; mobile incident sheet |
| Maintenance | Monitor (pause/maintenance); Server (Maintenance button); Status > Maintenance; CLI `maintenance down/up` |
| Credentials | App ingest keys; Settings > Security (API tokens, Node registration token); Server > Token (per server); Add server (enrollment token) |

### 2.2 Mobile (board `mobile-ia`)

Five tabs: **Notifications** (home), **Incidents**, **Servers**, **Monitors**, **More**. More holds Logs (Events | Service logs), Alert deliveries, Search, Notification settings, Settings, Core switcher. Config stays in Console (monitors, rules, routes, channels, status pages, tokens).

Mobile and desktop agree on the nouns for Incidents, Servers, Monitors and Alert deliveries. They differ in these places:

| Topic | Desktop | Mobile |
|---|---|---|
| Home | Overview | Notifications feed |
| Logs tabs | Service logs, Status logs, Internal logs | Events, Service logs (no Internal logs). "Events" is the desktop "Status logs" |
| Delivery history | Alerts > Notification log | More > Alert deliveries |
| Server logs | Server > Reports | Server > "Logs for this server" |
| Alerts, Status, Settings > Apps, Services | in the bar or Settings | absent by design; Services not mentioned in `service-impact` for mobile |

Mobile does not follow the Console bar, so desktop merges do not force mobile merges. Only the log tab names need to line up.

### 2.3 Duplicates, near-duplicates, orphans, shared nouns

| Noun | Meanings found | Where |
|---|---|---|
| **Apps** | (1) log source with keys, retention, redaction, display; (2) process, unit or container the Node measures; (3) "Apps (pm2)" group in Detected; (4) the Service's "Apps and logs" rows (proposed); also "connect an app" wizard | Settings > Apps; Server > Apps tab (board `apps-usage`, "Top apps 6 of 38"); `det-actions`; `service-detail` |
| **Service(s)** | (1) "Service logs" tab (apps and Nodes); (2) Services entity (proposed); (3) "Services 3" group in Detected, meaning systemd units; (4) Node service logs in the API (`/v1/logs/service`, `service_log_entries`); also OTel `service.name` | Logs; `service-list`; `det-actions`; `product-logs.md` |
| **Status** | (1) nav item = public status pages; (2) "Status logs" tab = Orion's own events | nav; `logs-events` |
| **Incidents** | (1) internal list; (2) Status page > Incidents tab = public incidents | nav; `status-incidents` |
| **Logs / history** | `/logs` (3 tabs); Server > Reports; Alerts > Notification log; Monitor > Check history; Settings > Audit log; Status > Incidents; CLI `orion-node logs`; mobile Logs. "Status logs" is the union of four of these (type = monitor, alert, server, incident) | see `logs-events` |
| **Settings** | instance-wide (General, System, Security, Data lifecycle, Audit) plus per-app config | Settings |

Other findings:

- Board `logs-service` is titled "Service logs" but shows the **Internal logs** tab (Core backend). Its file name and its tab disagree.
- Settings > Apps shows "Lines today / Dropped / Log storage" above every Settings tab. These are log numbers, not instance numbers.
- `/apps/new` highlights Overview (a code comment says this follows the canvas), yet its job is logs.
- `/health-issues` and `/incidents/triage` have no nav item. The design puts triage inside the Incidents filter row.
- Server tab "Reports" is named "logs" in old links; the code redirects `?tab=logs` to `reports`.
- Settings > Security (built) holds sign-in, Node registration token and API tokens; System holds worker, database, backups. That split is sound. Not a problem.

### 2.4 Design versus built Console

| Area | Design | Built |
|---|---|---|
| Top bar | 7 items (8 on Services boards) | 7 items; no Services anywhere |
| Server tabs | `server-detail` shows 4 tabs; `apps-usage` and `det-*` show 6 | 6 tabs |
| Status page tabs | 5 (no Maintenance) | 6 (Maintenance added) |
| Monitor detail | Day / Week / Month | `?tab=history` plus overview |
| Incidents "Needs review" | filter inside the list | separate `/incidents/triage` |
| App config | Settings > Apps | same (`/settings/apps/:id`), plus `/apps/new` wizard |

## 3. Diagnosis: why "apps -> logs" feels wrong

1. **Config is far from data.** An app is a log source. Its keys, retention and redaction live in Settings > Apps, and its data lives in Logs. After creating a key you go to Settings to wait for the first log, then to Logs to read it. The app page has a "View logs" link back, which shows the two are one job split in two. The Connect wizard even ends with a link to `/logs`.
2. **Three starts for one job.** Settings "New app" dialog, the `/apps/new` wizard (tied to Overview) and Detected "Create log key" all create an app. Docs say "open Settings, Apps" (`docs-logs`, `product-logs.md`, and two Node error messages in `validators.go:439` and `emit.go:72`), so Settings is also where users are told to go.
3. **Settings is not "instance-wide" any more.** It mixes retention, sign-in, tokens, audit and a per-app console with its own tab bar and storage stats.
4. **"Apps" is used for two different objects.** The Server > Apps tab is the Node's passive measurement of processes (decision D-017: no instrumentation needed). The log-source app is opt-in. Same word, different thing, and the Services proposal links them through a Service.
5. **"Service logs" is a name collision waiting to happen.** D-012 named the tab "Service logs" for apps plus Node-tailed logs. The Services proposal adds a Service entity and a Service facet to the same tab, with a "No service 14,478" row (`affected-logs`). The tab called Service logs would then hold lines that belong to no Service.
6. **"Status logs" collides with the Status item.** Nothing in "Status logs" relates to status pages. Mobile already calls it "Events".
7. **Node logs sit beside app logs without saying why.** The Source filter lists `edge-01 (Node)` next to `billing-api`. That is right (one search across everything), but the config for the Node's side lives in a different place (Server > Detected > Create log key) than apps do.

The bar is growing because each new capability asks for a noun-sized slot: Services, and earlier Logs, Status, Alerts. Three of them are set-once configuration areas, which is what Settings and secondary pages are for.

## 4. Candidate structures

Constraints used: self-hosted, one operator often; Logs must stay fast and one click away; CLI/Node docs and public status pages keep working; one name per thing.

### Option A: Tidy, keep a flat bar (7 items)

```
Overview | Incidents | Services | Servers | Monitors | Logs | Notify
```

- Alerts + Status become Notify (tabs: Notification log, Channels, Rules, Routes, Status pages).
- App setup moves to Logs. Services gets its own item as proposed.
- Gain: smallest change; Services is visible; Servers and Monitors stay separate as today.
- Lose: still 7, so the next feature brings it back to 8. At the design width the 8-item bar is the limit (see section 5.7).

### Option B: Group by job (6 now, 5 after Services). Recommended

```
Now (no Services yet):   Overview | Incidents | Servers | Monitors | Logs | Notify
After Services ships:    Overview | Incidents | Monitoring | Logs | Notify
                                                 Services | Monitors | Servers   (segmented control)
```

- **Overview, Incidents:** unchanged. Overview gets the Services block.
- **Monitoring:** one item, three sibling lists: Services, Monitors, Servers. Each keeps its own URL. The segmented control is the same pattern as the All / Down / Degraded filter row. Lands on Monitors by default; remembers the last segment.
- **Logs:** same three tabs (renamed, see 5.2), plus an **Apps** button with count that opens `/logs/apps` (list, create, keys, retention, display, redaction, connect wizard). Service > Logs and Server > Detected link into the same place.
- **Notify:** Notification log, Channels, Rules, Routes, Status pages. A status page opens as today (`/status-pages/:id`, its own tabs), with a breadcrumb back to Notify.
- **Settings (gear):** General, Data lifecycle, Security, System, Audit log. No Apps.
- Gain: 5 items; Services needs no new slot; the grouping matches how people work (what is wrong, what is watched, what the logs say, who is told).
- Lose: Servers and Monitors take one extra click from the bar; "Status" is no longer a word in the bar (mitigated by the Status pages tab, Cmd-K and the existing `G` shortcuts); "Monitoring" next to "Monitors" is a slightly repetitive pair.

### Option C: Left rail

```
Overview
Incidents
WATCH     Services, Monitors, Servers
Logs      Logs, Apps
NOTIFY    Alerts, Status pages
Settings
```

- Gain: scales to any number of destinations; grouped; Apps can sit under Logs without hiding.
- Lose: redraws the header on all ~190 light boards and the Console layout; the centered tray is part of the visual identity; the Console content is capped at 1040 px, so a 220 px rail costs reading width on small laptops. Worth it only if the product grows past about 10 destinations.

### Merges considered and rejected

| Merge | Verdict |
|---|---|
| Servers + Monitors alone | **Bad on its own.** Different lifecycle (install a Node versus add a URL check), different count (6 versus 24), different detail pages. Core-run checks have no server. Fine as siblings under Monitoring, not as one page. |
| Overview + Incidents | Bad. Overview is a summary; Incidents is the working list and a mobile tab. |
| Logs into Services | Bad. Logs must stay reachable without picking a Service, and many apps and all Node/Core logs have none. Service > Logs should embed the same component, filtered. |
| Services as the home of apps/monitors/resources | Bad as the only home. Linking is optional and suggestion-only (plan section 8, point 4), so the whole product cannot depend on it. |
| Alerts into Settings | Bad. Notification log is operational (did the page go out?), and it is a mobile entry. |

## 5. Recommendation

### 5.1 Rationale

Take Option B, in stages. The first stage fixes what hurts today and needs no Services work: app setup moves to Logs, Alerts and Status become Notify, and the names stop colliding. That alone takes the bar from 7 to 6 and leaves Services room to arrive without adding an item. When Services ships, group it with Monitors and Servers as "Monitoring", which keeps the bar at 5. Keeping Services out of the bar is also reversible: the `/services` routes work either way, so if Services proves to be the main working surface, promoting it is a one-line nav change. The reverse (taking a slot away later) is harder. Logs stays at the top level, one click away, with its three tabs, which meets the large-volume constraint. Apps are not a top-level noun at all: they are the sources of Logs.

### 5.2 Mapping: old place -> new place

| Old | New | Notes |
|---|---|---|
| Nav: Overview, Incidents, Logs | same | Overview "connect an app" link goes to `/logs/apps/new` |
| Nav: Alerts | Notify > tabs (Notification log, Channels, Rules, Routes) | URL `/alerts` unchanged |
| Nav: Status | Notify > Status pages tab | URL `/status-pages` unchanged |
| Nav: Monitors, Servers | now: unchanged. After Services: Monitoring > Monitors, Servers | URLs unchanged |
| Nav: Services (proposed) | Monitoring > Services | `/services`, `/services/:id` |
| Settings > Apps (list, stats, New app) | Logs > Apps (`/logs/apps`) | stats (lines, dropped, storage) move with it |
| Settings > Apps > app > Ingest / Retention / Display / Redaction | Logs > Apps > app (same four tabs) | `/logs/apps/:id` |
| `/apps/new` connect wizard | Logs > Apps > Connect | highlight Logs, not Overview |
| Detected > "Create log key" | same button, then lands in Logs > Apps > app | |
| Settings: Data lifecycle, General, Security, System, Audit log | Settings (5 tabs) | unchanged |
| Logs tab "Service logs" | "App logs" | owner decision; see 5.8 |
| Logs tab "Status logs" | "Events" | matches mobile and flow 13 |
| Logs tab "Internal logs" | keep (or "Core logs") | |
| Server tab "Apps" | "Usage" | matches `orion-node usage`; `?tab=apps` redirects |
| Server > Detected group "Services" | "System services" | avoids the Services entity |
| Server > Detected group "Apps (pm2)" | "PM2 processes" | |
| Service page "Apps and logs" (proposed) | "Apps" with link to Logs > Apps | |
| Mobile Logs "Events / Service logs" | "Events / App logs" | only rename |
| Board `logs-service` (shows Internal logs) | rename the board | |

Resulting vocabulary: **App** = something that sends logs. **Service** = the group a team defines. **Server** = a machine with a Node. **Monitor** = a check. **Usage** = what the Node measures. **Events** = what Orion saw itself.

### 5.3 Screens to redraw

About 190 light boards (and their dark pairs) have the header, so the nav change is one mechanical edit. Real redraws:

- **Relocate and re-frame (about 13):** `settings-apps`, `settings-app-new`, `app-keys`, `app-key-new`, `app-retention`, `app-display`, `app-redaction`, `app-send-test`, `app-connect-create`, `app-connect-snippet`, `app-connect-waiting`, `app-connect-received`, `affected-settings-apps`. New frame: Logs header, breadcrumb "Logs / Apps".
- **Logs header (about 15):** all `logs-*` boards and `affected-logs`: tab names, an "Apps" button with count, empty state "Connect an app" pointing to the new place.
- **Notify strip (about 11):** `alerts-*`, `channel-*`, `delivery-detail`, `affected-alerts-rules`: nav label, add the Status pages tab.
- **Status (about 11):** `status-*`: nav label, breadcrumb back to Notify; add Maintenance tab to the drawn strip to match the build.
- **Small renames (about 6):** `apps-usage`, `apps-states`, `apps-track-storage`, `affected-server-apps`, `det-actions` and any `det-*` showing the group names.
- **Services boards (11 plus the 10 `affected-*`):** header becomes Monitoring with the three-way control; `service-impact` text updated.
- **Overview:** `onboarding` link; `affected-overview` stays.
- **Docs and mobile:** `docs-logs` (step 1), `mobile-logs`, `mobile-ia`, `mobile-more` (tab names only).

### 5.4 Console routes

| Route | Change | Redirect |
|---|---|---|
| `/settings?tab=apps` | moves to `/logs/apps` | yes (already redirects `/settings/apps`) |
| `/settings/apps/:id` | moves to `/logs/apps/:id` | yes |
| `/apps/new` | moves to `/logs/apps/new` | yes |
| `/logs?tab=service\|status\|internal` | keep the values; change labels only | none needed |
| `/servers/:id?tab=apps` | becomes `?tab=usage` | yes (pattern exists in `resolveTab`) |
| `/alerts`, `/status-pages`, `/status-pages/:id`, `/servers`, `/monitors` | unchanged | none |
| `/services`, `/services/:id` | new | none |

Code touches: `components/shared/app-header.tsx` (items, active-state logic, the Overview special case for `/apps/new`), `components/shared/command-palette.tsx` (labels, hints), `router/app-router.tsx`, `features/settings/settings.view.tsx` (drop the tab and the stats), `features/logs/logs.view.tsx`, `features/node-detail/node-detail.view.tsx`, `features/alerts/alerts.view.tsx`. No API changes: `/v1/apps`, `/v1/logs`, `/v1/alerts` and the `source` filter stay as they are. Per AGENTS.md, nothing in `apps/core/web/` is hand-edited; run `make build-static`.

### 5.5 Docs, CLI, mobile

- **Docs:** `docs/product-logs.md` line 9, `docs/design/design-language.md` line 144, `docs/design/screen-map.md` (nav line and route table), the docs-site Send logs page ("open Settings, Apps" -> "Logs, Apps"), README screenshots if any.
- **CLI/Node:** no command or flag changes. Two strings say "Console, Settings, Apps, create a key": `apps/node/internal/config/validators.go:439` and `apps/node/internal/cli/emit.go:72`. Change them in the same release as the move; until then they send users to a redirect, which still works. `orion-node usage` and `discover` already match the new tab names Usage and Detected.
- **Public status pages:** unaffected. They are rendered by Core under their own paths. The email confirm URL `/alerts/email/...` stays.
- **Mobile:** no tab changes. Rename "Service logs" to "App logs". Settings > Mobile (pairing) stays in Console Settings.

### 5.6 Migration order (nothing breaks at any step)

1. **Names first (no routes).** Logs tab labels, Server "Usage", Detected group names, board names, docs wording. Keep old URL values.
2. **Add `/logs/apps/*` and the Apps button**, with redirects from the three old routes, then remove the Settings tab and its stats. Update the two Node strings and docs in the same release.
3. **Notify:** add the Status pages tab to the Alerts strip, change the nav label, make the bar highlight Notify for both route families. Keep old URLs.
4. **Redraw the designs** for 1 to 3 before each Console change (D-026/D-027: canvas is the spec; the user approves screen by screen).
5. **Services (S1 and on):** ship `/services` as the plan describes, with the Overview block; add the Monitoring grouping in the same release so the bar never reaches 8.
6. Revisit after a month: is Monitoring's default segment right, and does Services deserve its own slot?

### 5.7 Risks and unverified items

Risks:

- **Discoverability of "Notify" and "Monitoring".** Both are group names. Mitigation: tab labels inside use the old words (Alerts channels, Status pages, Monitors, Servers); Cmd-K and `G` shortcuts keep working; test with one real user before step 3.
- **Default landing of Monitoring** is a guess (Monitors, remembering the last segment).
- **Naming is the owner's call.** D-012 chose "Service logs / Status logs / Internal logs" ("owner's naming wins"). Renaming two of the three reverses it, so ask first. Fallback if the owner keeps them: at minimum rename "Status logs" now, and rename "Service logs" before the Service facet ships.
- **App vs Service overlap.** An App and a Service share name, slug and environment (`product-logs-plan` and `service-correlation-plan`). If most services end up with one app, consider auto-creating a Service when an App is created. This review does not decide that; the plan lists it as open.
- **Nav width.** Estimated from the CSS, not measured: the header is a three-column grid with a centered tray and a right cluster of about 330 px. Eight items need about 1,000 px before it collides; five need about 800 px. The Console has no collapse behavior, and the designs only show 1280 px.

Not verified:

- I read board HTML source and did not look at rendered boards. About 190 light boards were scanned for header, tabs and headings; login, public, website, docs and most mobile boards only lightly (`mobile-ia` fully).
- I did not run the Console or check behavior, only routes, tab lists and link targets in source.
- No usage data exists in this review, so "most used" claims (Monitors, Servers, Incidents) are structural, not measured.
- Whether `/apps/new` is linked from Settings > Apps itself (the list uses a dialog; the wizard is linked from Overview and Logs empty state).
- Whether the docs site (outside this repo's `docs/`) contains more "Settings, Apps" text.

### 5.8 Decisions needed from the owner

1. Approve moving Apps to Logs (and the removal of Settings > Apps).
2. Approve Notify as the name for Alerts + Status pages.
3. Approve renaming "Service logs" and "Status logs" (reverses part of D-012), or keep them and accept the collisions.
4. Approve the Monitoring grouping when Services ships, or choose Option A.

## 6. Sketches

Recommended bar, after Services (1280 px, same tray style as the canvas):

```
+--------------------------------------------------------------------------------------+
| (o) Orion    [ Overview | Incidents | Monitoring | Logs | Notify ]   Search  2 issues  gear  (av) |
+--------------------------------------------------------------------------------------+

Monitoring   [ Services | Monitors | Servers ]
Logs         [ App logs | Events | Core logs ]   List | Live | Patterns     [Apps 4]
Notify       [ Notification log | Channels | Rules | Routes | Status pages ]
Settings     [ General | Data lifecycle | Security | System | Audit log ]
```

Logs, Apps and Services:

```
                    Service "checkout"  (optional grouping; lives in Monitoring > Services)
                    /            |                  \
              Monitors        Apps (log sources)    Resources (Usage on Servers)
              Monitoring      Logs > Apps           Monitoring > Servers > Usage
                                 |
                                 | ingest key, retention, display, redaction, connect
                                 v
   Logs  >  App logs  <---------- one search over every app and Node log
         >  Events                 (Orion's own: checks, incidents, alerts, servers)
         >  Core logs              (Orion backend)

   Service page "Logs" tab  = Logs filtered to its apps (same component, deep link /logs?service=checkout)
   Server > Detected "Create log key" = creates an App, then opens Logs > Apps > app
   Apps with no Service still show in Logs and in Logs > Apps (the list shows a "not in a service" hint)
```
