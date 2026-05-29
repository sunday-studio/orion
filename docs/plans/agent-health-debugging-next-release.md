# Server Health And Debugging Next Release Plan

This is a living issue plan for the next Orion release. It captures what feels wrong while using the app and keeps implementation decisions out until the release scope is ready to build.

## Phase 1: Server Status Is Misleading

### Issue 1: Monitor failures can make the whole server look down

Right now, one or a few failing monitors can cause the server itself to show as `down`, even when the server is alive and reporting normally.

This makes it hard to tell whether:

- the server process or machine is actually unreachable;
- one monitor is failing;
- several monitors are failing;
- the server is healthy but reporting bad monitor results.

### Issue 2: Server health does not clearly separate server availability from monitor health

The app needs a clearer distinction between:

- the server itself reporting system metrics;
- monitors owned by that server reporting check results;
- the combined status shown in the UI.

Today those concepts feel blended together, which makes the status less trustworthy.

### Issue 3: Server status needs threshold-based monitor rollup

A small number of failing monitors should not dominate the entire server status.

The issue to solve is that monitor health should affect the server status only when enough monitors are unhealthy to represent a real server-level problem. A working target is roughly 30% unhealthy monitors. Monitor health alone should not make the server fully down.

## Phase 2: Missing Server Signal

### Issue 4: If the server itself stops reporting, that should be visible

When server system metrics stop coming in, the app should make that obvious.

The important distinction is:

- if monitor reports are still arriving but server system metrics are missing, the server is degraded;
- if neither server metrics nor monitor reports are arriving, the server is down.

### Issue 5: Current status does not explain why the server is degraded or down

The UI can show `degraded`, but it does not give enough context about what caused that state.

Users need to know whether the status came from:

- missing server reports;
- unhealthy monitor percentage;
- stale monitor reports;
- monitor check failures;
- system metrics being unavailable.

## Phase 3: Report Context Is Too Thin

### Issue 6: Monitor reports do not expose enough debugging context

When a monitor fails, the app often does not show enough detail to understand what actually happened.

Examples of missing or hard-to-find context:

- what request or check was attempted;
- what response, status, or error came back;
- how long it took;
- whether DNS, TLS, HTTP, process, command, Docker, or service checks failed;
- what the raw result from the server looked like.

### Issue 7: Successful reports also need useful metadata

Debugging should not only be possible when something fails.

Successful checks should also preserve useful context, such as:

- status code;
- latency;
- resolved target;
- checked resource;
- command, service, or container identity;
- collected timestamp;
- server and monitor metadata.

## Phase 4: UI Inspection Tools

### Issue 8: Users need drawers for raw report inspection

When clicking a report or metric row, the UI should open a drawer with the actual data received from the backend.

This should start with:

- server report rows;
- monitor report and history rows.

The drawer should make it easy to inspect:

- summarized status;
- timestamps;
- metrics;
- errors;
- metadata;
- raw JSON payload.

### Issue 9: The current UI makes operational debugging too indirect

Right now, when something is degraded, the user has to infer too much from tables and badges.

The issue is not only missing data, but missing interaction. The UI needs a quick "show me what actually came in" path.

## Phase 5: Incident Lifecycle Controls

### Issue 10: Users cannot manually resolve an incident from the browser

Incidents currently resolve when status changes or new data comes in, but there is no clear browser action for a user to mark an incident as resolved.

The app needs a manual resolution path for cases where the user has investigated the issue and wants to close it intentionally.

### Issue 11: Removing a broken monitor can leave its incident hanging

When a monitor that caused an incident is removed, the incident does not reset or resolve. It can remain open even though the monitor is no longer active.

This makes incidents feel stuck and creates confusion about whether the problem is still happening.

## Phase 6: Living Backlog For Next Release

As more issues come up while using the app over the next few days, add them under one of these buckets:

- Server status accuracy
- Monitor rollup behavior
- Missing or stale report behavior
- Report metadata and debuggability
- UI inspection and drawers
- Incident lifecycle and manual resolution
- Alerting or incidents affected by status
- Anything that feels confusing during real use

## Release Goal

The next release should make Orion's health model more trustworthy and explainable.

The main user-facing outcome: when a server says `up`, `degraded`, or `down`, the app should make it clear whether the problem is the server, the monitors, missing data, or the checks themselves.
