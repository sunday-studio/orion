# Public Uptime Formatting

Status: accepted

Date: 2026-05-27

Ticket: T-20260527-111953-42db

## Context

Published status pages need uptime figures that are easy for visitors to scan without implying more precision than Orion can support. Core already stores monitor uptime rollups, but public status pages are a product surface, not an operator debugging view.

## Decision

Show public component uptime as a percentage with two decimal places for normal ranges, for example `99.95%`.

Use these formatting rules:

- compute public uptime from published component health windows, not from raw internal report payloads;
- show `100%` without trailing decimals when there was no downtime in the selected window;
- show two decimal places for values from `99.00%` through `99.99%`;
- show one decimal place below `99.00%`, where the page should emphasize the incident history rather than fine-grained precision;
- show `No data` when the component has no trustworthy samples for the selected public window;
- never expose sample counts, monitor ids, agent ids, raw check timings, or internal error messages in the public uptime label.

The first release should default to `90 days` for public component uptime, with optional `24 hours`, `7 days`, and `30 days` windows once the history API can support them cheaply.

## Unknown State

Show `Unknown` as its own public component state by default. Do not collapse it into `degraded`, because that would imply known customer impact when Orion has only lost confidence in the signal.

Use `Unknown` when:

- a visible component has mapped resources but none have recent trustworthy samples;
- a mapped Agent or monitor has not reported inside the freshness window;
- Core cannot compute a reliable rollup without exposing internal details.

Public pages may place unknown components below active outages in sort order, but the label stays `Unknown`. If a page owner wants a friendlier public message, they should set a manual component status and public status reason.

## Timestamp Rounding

Round public status timestamps to the nearest minute. Do not show seconds or subsecond precision on public pages.

- Incident update timestamps render as absolute date plus minute in the viewer's locale when the client can localize them.
- Recent timestamps may also show relative copy such as `5 minutes ago`, but the API should continue returning ISO timestamps.
- Uptime window labels use whole-day windows such as `24h`, `7d`, `30d`, and `90d`.
- Do not expose raw check timestamps or individual rollup bucket boundaries in public responses.

## Display Rules

- Uptime labels belong beside public component names and in the status history view, not in incident update copy.
- Public pages should pair uptime with the current public component status so a historically healthy service can still show an active outage clearly.
- If a component is hidden, its uptime is also hidden.
- If a component is visible but has no mapped resource and no manual status history, show current manual status and `No data` for uptime.
- Maintenance windows should count as `maintenance`, not `downtime`, when the relevant monitor or component was explicitly in scheduled maintenance.

## API Implications

Public history DTOs should return both the numeric uptime ratio and the preformatted display string. The numeric value lets clients sort or draw charts; the display string keeps Core as the formatting authority for privacy and consistency.

Example:

```json
{
  "window": "90d",
  "uptime_ratio": 0.9995,
  "uptime_display": "99.95%"
}
```

Do not return raw rollup buckets from the public API unless a future chart endpoint sanitizes and aggregates them specifically for public display.

## Consequences

Two decimal places keep high-availability services readable while avoiding exaggerated precision for degraded services. The `No data` state also prevents Orion from pretending a new or unmapped component has historical reliability it has not measured.
