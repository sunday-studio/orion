# Canvas Build Plan: feature by feature

Status: active, restarted 2026-10-17 (D-027). Canvas: https://claude.ai/artifact/L7gFM2gipuTveU5hkGqMRY (116 artboards, 12 pages).
Related: [decision-log.md](decision-log.md), `docs/design/implementation-handoff.md`, `docs/design/screen-map.md`.

## Rules

1. **The canvas is the spec.** The console is built to match how each artboard is laid out, pixel-faithfully at 1280px, light and dark. Where `design-language.md` and an artboard disagree, the artboard wins and design fixes the doc.
2. **One feature at a time.** A feature is one canvas page. We do not start the next feature until the user says they are satisfied with the current one.
3. **Done means the user is satisfied,** not that a build passes. Each feature ends with a review pack from the coordinator (running app next to the artboards) and waits for the user's verdict. Change requests loop back until approved.
4. **Threads talk directly for design and API help** on the active feature, in short messages. Web asks design about layout and states; web asks backend about missing endpoints; design and backend answer in one message. Anything they cannot settle goes to the coordinator, who decides and logs it.
5. **Nothing is skipped because an API is missing.** Backend builds the missing endpoint for the active feature first; if it is not ready, web builds against a typed stub behind a feature flag.
6. Everything is committed as work goes and pushed to `main` (no PRs, CI off, D-023/D-024). Independent review before push still applies.

## Features (in order)

| # | Feature (canvas page) | Artboards | Notes |
|---|---|---|---|
| F1 | Entry and home | 16: Overview (+dark, first run, all good), Sign in, Add server wizard (4 states), Connect an app (4 steps), Command palette, Account menu, Keyboard shortcuts | Add server needs enrollment token flow (backend ready); connect-app needs apps and ingest keys (backend ready) |
| F2 | Incidents | 10: list, detail, resolved, actions, candidates, health issues, declare, bulk, empty, public update | Candidates and health issues have typed schemas now; declare and bulk are proposed endpoints |
| F3 | Servers | 10: list, detail (reports, metrics, monitors, stale), token, new token, edit, remove, empty | Agent PATCH is proposed |
| F4 | Monitors | 14: list, create (+TCP), detail, edit, test result, heartbeat setup, report drawer, maintenance, paused, delete, agent monitor setup, bulk, empty | Bulk is proposed |
| F5 | Alerts | 10: log, channels, channel dialog + webhook/email forms/test/detail, rules, routes, delivery detail | Email channel and retry delivery exist |
| F6 | Status pages | 11: list, create, setup, components, incidents, subscribers, preview, publish, domain, maintenance, badge | Domain verify and maintenance windows are proposed |
| F7 | Logs and settings | 29: events, service logs, live tail, patterns, detail, views, export; settings general/lifecycle/system/tokens/audit; apps, keys, retention, display, redaction, test; empty/error states; session expired | Largest; may be split into F7a logs and F7b settings |
| F8 | Public status | 7: status, history, incident, subscribe, manage, unsubscribe, email confirm | |
| F9 | CLI and agent | 6: install, setup, doctor, status, logs, help/errors | Owned by orion:agent; runs on its own track and is reviewed last |

Cross-cutting pages (States, Flows, Design language) are references used by every feature, not features of their own.

## Per-feature loop

1. **Kickoff (coordinator):** list the feature's artboards, the API each needs, and gaps (this doc plus a short feature brief posted to the threads).
2. **Design check:** orion:design confirms the artboards are final and exported to `docs/design/artboards/` (HTML + light/dark PNG at 1280px).
3. **Backend:** orion:backend closes API gaps for the feature (typed OpenAPI, tests), pushes, and tells web.
4. **Web build:** orion:web rebuilds each screen from its artboard, asking design and backend directly when needed. Adds a Playwright screenshot comparison per screen against the exported PNGs.
5. **Coordinator verification:** run the real app, compare each screen to its artboard, list any mismatches back to web. No hand-waving from summaries.
6. **Review pack to the user:** screenshots side by side (artboard vs app), what is stubbed, what is deferred.
7. **User verdict.** Approved: next feature. Changes: back to step 4 for that feature only.

## Coordination of other threads

- `orion:cleanup`: dead-code removal in `apps/console` waits until all features are approved, to avoid fighting the rebuild.
- `orion:agent`: continues the CLI/agent track and pushes; its feature (F9) is reviewed last.

## Last, before going live with the new version

Deferred on purpose (D-031), done only after features F1 to F9 are approved:

- Agent as a container: container image, Add server "Docker" tab (design + web), Kubernetes DaemonSet/Helm chart (agent plan F2).
- Re-enable GitHub CI (D-024) and cut the release.
