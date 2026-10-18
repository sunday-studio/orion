# Seed Demo Data

Use the Core seed script to generate dense local data for UI, API, and performance testing.

The script writes directly to the Core SQLite database, applies migrations first, and inserts namespaced `seed-*` rows.

Default run:

```sh
make seed-demo-data
```

Docker dev stack run:

```sh
make docker-dev-up      # starts Core + Worker, then runs the Console dev server (pnpm) in the foreground
make docker-dev-seed    # run in a second terminal
```

`make docker-dev-up` leaves the Console running at http://localhost:5173 (Core API on :8999). Use
`make console-dev` to run only the Console against an already-running Core.

The Docker command seeds the `orion-dev-data` Compose volume used by `docker-compose.dev.yml`.

Equivalent command:

```sh
cd apps/core
go run ./scripts/seed-demo-data
```

By default this writes to `apps/core/data/orion.db` and generates:

- 90 days of data;
- 10 seeded servers;
- all implemented monitor types per server;
- hourly server reports;
- hourly monitor reports;
- daily uptime rollups;
- open, acknowledged, and resolved incidents;
- pending, sent, failed, suppressed, and cooldown webhook alert deliveries;
- a published demo status page with sections, components, monitor mappings, public incidents,
  subscriber preferences, and delivery ledger rows.

## Custom Database

Seed a temporary database:

```sh
cd apps/core
go run ./scripts/seed-demo-data -db /tmp/orion-seed.db
```

Seed more volume:

```sh
cd apps/core
go run ./scripts/seed-demo-data -db /tmp/orion-heavy.db -days 90 -agents 25 -report-interval 30m
```

## Scenarios

The generated dataset covers these server states:

- healthy;
- degraded;
- down;
- maintenance;
- stale;
- unknown/no reports;
- flapping;
- TLS expiring;
- resource pressure;
- alert delivery edge cases.

The generated monitors cover:

- `http-healthcheck`;
- `website`;
- `tcp`;
- `resource-threshold`;
- `docker-container`;
- `systemd-service`;
- `pm2`;
- `command`;
- `internal-service`;
- disabled lifecycle rows;
- deleted lifecycle rows;
- active never-reported rows.

The generated status page data covers:

- `seed-orion-status`, a published public status page with SEO, metadata, theme, and custom-domain
  fields populated;
- public sections for customer-facing systems, infrastructure, and internal services;
- visible mapped components, manual components, and a hidden private component;
- monitor and server mappings so public component health and uptime can aggregate from seeded
  monitors and rollups;
- published active, resolved, and scheduled public incidents;
- private and draft incidents for Console editor and public-boundary checks;
- confirmed, scoped, pending, and unsubscribed public subscribers;
- sent and pending-sender-configuration subscriber delivery rows.

## Showcase: every state, by name

On top of the generated scenarios the seed adds a small, explicit set of named records (ids start with
`seed-state-`) so each screen state can be reviewed without creating data by hand. They live in
`apps/core/scripts/seed-demo-data/seed-states-*.go`, one file per feature, and `seed_test.go` fails when a
state goes missing.

- Servers: `web-prod-01` up, `api-prod-02` degraded and high CPU, `db-prod-01` down, `cache-stage-01` in
  maintenance, `batch-eu-03` stale, `edge-new-01` unknown (no reports), `files-prod-01` disk 94% and memory
  96%, `tools-dev-01` nearly idle, plus labels, notes, a token in its rotation grace window, a rotated token,
  a revoked token, a removed server with its history kept, and add-server sessions (waiting, claimed,
  expired, cancelled).
- Monitors: every Core kind (http, keyword, tcp, dns, tls, udp, ping, domain expiry, api request, smtp,
  synthetic, playwright, heartbeat ok, late and waiting) and agent kind, in up, degraded, down, stale,
  unknown, paused and maintenance (active and scheduled) states, with runbooks, spikes and timeouts, and
  agent monitor definitions that are reporting or still waiting.
- Incidents: open, acknowledged, covered by another incident, resolved (recovered, manual, monitor
  removed, decommissioned), reopened twice, declared by hand, every severity, notes, responders, and open
  and dismissed candidates.
- Alerts: healthy, failing, disabled, digest and untested channels with confirmed, pending and
  unsubscribed recipients; deliveries that are sent, retried then sent, failed, exhausted, pending,
  suppressed and in cooldown; rules with cooldown, suppression and disabled; label routes including a
  disabled one.
- Status pages: published with a pending domain, unlisted with a failed domain, verified, draft and empty
  pages; components in every state including hidden; public incidents in every status plus draft and
  private; maintenance scheduled, in progress and completed; subscribers confirmed, pending, expired,
  unsubscribed, disabled and bounced.

The showcase uses a one day reporting interval, because Core treats data as stale after five intervals; the
states therefore last for days instead of turning stale a few minutes after seeding. Nothing seeded is
picked up by the worker: Core monitors are scheduled ten years out, undelivered alerts are parked 36 hours
ahead, and silent monitors already have their stale incident. Set the same
`ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET` for the seed and for Core (the dev compose file does) and the
subscriber screens show full email addresses.

- Logs and settings: five apps (receiving, quiet, never connected; one near its size cap, one with custom
  redaction rules and a display profile, one that only sends plain text) with ingest keys that are in use,
  unused, revoked and mid-rotation; about 8,000 log lines over a week across debug to fatal with correlated
  requests, structured bodies, plain text lines whose severity is read from the text and a recently started
  failure for the Patterns tab; Core's own log lines for Internal logs; agent service log lines; saved views;
  one disabled log alert rule; an audit log with several people and API tokens; API tokens (read, write, unused,
  revoked) and instance settings. Log lines live in `logs.db` next to the main database, so `-db` or
  `-data-dir` decides where they go.

## Re-running

The script deletes previous `seed-*` rows before inserting new rows unless `-reset-seed=false` is passed.

```sh
cd apps/core
go run ./scripts/seed-demo-data -reset-seed=false
```

The script also upserts data lifecycle settings by default so rollup/archive screens have data to read. Disable that with:

```sh
cd apps/core
go run ./scripts/seed-demo-data -update-settings=false
```

## Flags

- `-db`: SQLite database path.
- `-data-dir`: Core data directory when `-db` is not set.
- `-days`: number of days to generate, default `90`.
- `-agents`: number of seeded servers, default `10`. Must be at least `10` to cover every scenario.
- `-report-interval`: generated report spacing, default `1h`.
- `-reset-seed`: remove previous `seed-*` rows before inserting, default `true`.
- `-update-settings`: upsert demo lifecycle settings, default `true`.

## Docker Dev Overrides

The Docker seed service uses the same defaults as the local script. Override them with environment
variables:

```sh
ORION_SEED_DAYS=14 ORION_SEED_AGENTS=10 make docker-dev-seed
```

Supported Docker seed variables:

- `ORION_SEED_DAYS`: maps to `-days`.
- `ORION_SEED_AGENTS`: maps to `-agents`.
- `ORION_SEED_REPORT_INTERVAL`: maps to `-report-interval`.
- `ORION_SEED_RESET`: maps to `-reset-seed`.
- `ORION_SEED_UPDATE_SETTINGS`: maps to `-update-settings`.
