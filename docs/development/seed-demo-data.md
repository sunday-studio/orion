# Seed Demo Data

Use the Core seed script to generate dense local data for UI, API, and performance testing.

The script writes directly to the Core SQLite database, applies migrations first, and inserts namespaced `seed-*` rows.

Default run:

```sh
make seed-demo-data
```

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
- pending, sent, failed, suppressed, and cooldown alert deliveries;
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
