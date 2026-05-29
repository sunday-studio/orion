# M4: Console Release Pass

## Goal

Bring the Console to a usable first-test state for incidents, servers, monitors, alerts, event logs, and settings.

## Scope

- Primary navigation and app shell.
- Incidents list and incident detail.
- Servers list and server detail.
- Monitors list and monitor detail.
- Alerts, event log, and data lifecycle settings pages.
- Shared table, pagination, empty state, badges, tabs, and URL query state.

## Completed

- `/` redirects to `/incidents`.
- Primary navigation includes Incidents, Servers, Monitors, Alerts, Logs, and Settings.
- Incidents show summary cards, server-side filters, pagination, status/severity/notification badges, linked server/monitor columns, and detail views.
- Server list uses summary cards, server-side filters, pagination, expandable monitor rows, and linked detail navigation.
- Server detail has tabs for reports, monitors, and system metrics.
- Monitor list uses the shared data table, summary cards, filters, pagination, and linked server/incident navigation.
- Monitor detail shows current result, recent uptime buckets, check history, related incidents, and configuration snapshot.
- Alerts page shows notification deliveries, channels, and rules.
- Event log page shows Orion events with server-side source, type, and search filters.
- Settings page can read and update data lifecycle settings and run manual archive/rollup actions.
- Main list and detail views have loading, error, and empty states.
- Tabs, filters, and pagination that matter for navigation are wired to URL query state.

## Verification

- `npm run lint` passed.
- `npm run build` passed.
- Relevant Core API tests passed while adding backend filters needed by Console.

## Open Risks

- There is no browser automation smoke suite yet.
- Visual QA is still manual.
- Service log collection is not implemented; Logs currently means Orion operational events.

## Next

- Use seeded data and one real Server to test the Console end to end.
- Add small browser smoke tests only after the first manual test pass stabilizes the flows.
