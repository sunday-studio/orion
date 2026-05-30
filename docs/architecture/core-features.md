# Core Features

## API Surface

Core uses Gin and serves these route groups:

- Public:
  - `POST /v1/register`
  - `POST /v1/auth/login`
- Frontend-facing, JWT-protected only when frontend auth is configured:
  - `GET /v1/agents`
  - `GET /v1/agents/:id`
  - `GET /v1/agents/:id/health`
  - `GET /v1/agents/:id/reports`
  - `GET /v1/agents/:id/uptime`
  - `GET /v1/agents/:id/monitors`
  - `GET /v1/monitors/:id`
  - `GET /v1/monitors/:id/uptime`
  - `GET /v1/monitors/:id/history`
  - `GET /v1/health/summary`
  - `GET /v1/health/issues`
  - `GET /v1/incidents/candidates`
  - `GET /v1/settings/data-lifecycle`
  - `PUT /v1/settings/data-lifecycle`
  - `POST /v1/settings/data-lifecycle/actions/rollup`
  - `POST /v1/settings/data-lifecycle/actions/archive`
- Server-protected:
  - `POST /v1/agents/:agent_id/register-monitor`
  - `POST /v1/agents/:agent_id/unregister-monitor`
  - `POST /v1/agents/:agent_id/report`
  - `POST /v1/agents/:agent_id/:monitor_id/report`
  - `PUT /v1/agents/:agent_id/maintenance`
- Other:
  - `GET /health`
  - `GET /swagger/*any`

## Authentication

```mermaid
flowchart TD
  Request["HTTP request"] --> RouteType{"Route type"}
  RouteType -- "public" --> Handler["Handler"]
  RouteType -- "server protected" --> Bearer["Require Bearer token"]
  Bearer --> Validate["Validate token against agent_id"]
  Validate --> Handler
  RouteType -- "frontend" --> AuthOn{"Frontend auth enabled?"}
  AuthOn -- "no" --> Handler
  AuthOn -- "yes" --> JWT["Validate JWT"]
  JWT --> Handler
```

Server auth:

- Server receives a token at registration.
- Protected Server routes require `Authorization: Bearer <token>`.
- Core validates that token belongs to the route `agent_id`.

Frontend auth:

- Enabled only when `ORION_ADMIN_USERNAME` and `ORION_ADMIN_PASSWORD` are set.
- `ORION_JWT_SECRET` is required when frontend auth is enabled.
- Login uses constant-time username/password comparison.
- Failed login attempts are rate limited per client IP.
- Successful login returns a 24-hour JWT.

## Health Computation

Monitor health is derived from recent reports and cached on the monitor row.

Defaults:

- stale threshold: five missed reporting intervals, with a five-minute minimum window;
- flapping threshold: 3 transitions;
- degraded failure rate: 30%;
- last 20 reports are used for monitor health.

```mermaid
flowchart TD
  Compute["Compute monitor health"] --> Cache{"Cached value fresh?"}
  Cache -- "yes" --> ReturnCache["Return cached computed_health"]
  Cache -- "no" --> Reports["Load last 20 monitor reports"]
  Reports --> Empty{"No reports?"}
  Empty -- "yes" --> Unknown["unknown"]
  Empty -- "no" --> Stale{"Latest report stale?"}
  Stale -- "yes" --> Unknown
  Stale -- "no" --> Flap{"Flapping?"}
  Flap -- "yes" --> Degraded["degraded"]
  Flap -- "no" --> FailureRate{"Failure rate >= 30% and < 100%?"}
  FailureRate -- "yes" --> Degraded
  FailureRate -- "no" --> Current["Latest report health"]
  Unknown --> Persist["Update computed_health cache"]
  Degraded --> Persist
  Current --> Persist
```

Server health is derived from Server and monitor state:

- maintenance server returns `maintenance`;
- stale `last_seen` returns `stale` based on the Server reporting interval;
- server with no active monitors returns `up`;
- fresh server availability stays `up` while monitor failures are rolled up separately;
- mixed monitor failures return `overall_health = degraded`, so one failing monitor does not make a live Server look down;
- `overall_health = down` is reserved for a fresh server whose active monitors are all failing;
- stale monitor reports affect monitor rollup health and explanations, not Server availability.

## Incident Management

Core reconciles incidents after monitor reports and after system reports.

```mermaid
flowchart TD
  Report["Monitor report stored"] --> State["Derive incident_state"]
  State --> Skip{"Repeated healthy state with no active incident?"}
  Skip -- "yes" --> Done["No incident work"]
  Skip -- "no" --> TLS{"TLS expiring?"}
  TLS -- "yes" --> OpenDegraded["Open/update degraded incident"]
  TLS -- "no" --> Health{"Reported health"}
  Health -- "up" --> Resolve["Resolve active incident"]
  Health -- "down/degraded/stale" --> Maintenance{"Server in maintenance?"}
  Maintenance -- "yes" --> Suppress["Suppress incident open"]
  Maintenance -- "no" --> Open["Open or update incident"]
  Open --> Event["Create incident event"]
  Open --> Alert["Queue alert delivery"]
  Resolve --> ResolveEvent["Create resolved event"]
  Resolve --> Recovery{"Recovery notifications enabled?"}
  Recovery -- "yes" --> Alert
```

Incident rules:

- `down` and `stale` map to high severity.
- `degraded` maps to medium severity.
- other states map to low severity.
- Monitors cache `active_incident_id` and `incident_state` for the ingestion path.
- Active incidents are resolved or updated by cached id when possible.
- Fallback active incident lookup is matched by monitor id and status `open` or `acknowledged`.
- New failures update the current active incident instead of creating duplicates.
- Recovery resolves the active incident and records `resolved_at`.
- Stale monitor incidents are checked when a Server system report is received.

## Alerts

Alert deliveries are created for incident opened/resolved events and are governed
by editable alert rules.

```mermaid
flowchart TD
  Queue["Queue incident notification"] --> LoadRules["Load enabled alert rules"]
  LoadRules --> Rules{"Matching rules?"}
  Rules -- "no" --> Suppressed["Create suppressed delivery: no rule"]
  Rules -- "yes" --> Filters{"Trigger and filters match?"}
  Filters -- "no" --> Skip["Skip rule"]
  Filters -- "yes" --> Suppress{"Rule suppresses?"}
  Suppress -- "yes" --> SuppressedRule["Create suppressed delivery"]
  Suppress -- "no" --> Cooldown{"Cooldown active?"}
  Cooldown -- "yes" --> CooldownDelivery["Mark cooldown"]
  Cooldown -- "no" --> Webhook["POST generic webhook"]
  Webhook --> Sent{"2xx?"}
  Sent -- "yes" --> MarkSent["Mark sent"]
  Sent -- "no" --> MarkFailed["Mark failed"]
```

Supported internal alert destination:

- generic webhook only.

Configured behavior:

- rules own trigger, filter, suppression, grouping, cooldown, recovery, and destination selection behavior;
- rules can target one or more generic webhook destinations;
- disabled rules do not create delivery rows;
- suppressing rules create suppressed delivery rows;
- unmatched events do not create webhook delivery rows;
- Core maintenance mode stays outside rule matching because it suppresses incident candidates before alert delivery is queued;
- cooldown can prevent repeated sent alerts;
- recovery notifications can be disabled;
- TLS expiry threshold defaults to 14 days.

## Maintenance

There are two maintenance concepts:

- Server local state can pause report workers.
- Core `agents.maintenance_mode` suppresses incident opening and makes server health return `maintenance`.

The Server CLI can call Core to set maintenance mode. The Server also rereads local state before every report cycle so it can stop sending reports if local maintenance state changes.

## Settings

Core stores a singleton data lifecycle settings row:

- raw report hot days: default `90`;
- archive raw reports: default `true`;
- archive directory: default `<data_dir>/archive`;
- rollups enabled: default `true`;
- rollup retention days: optional;
- archive schedule: `daily` or `manual`;
- last rollup/archive run metadata.

Validation rules:

- `raw_report_hot_days` must be at least 1.
- archive directory is required when archiving is enabled.
- rollups must be enabled when archiving is enabled.
- rollup retention is either null or at least 1.
- archive schedule must be `daily` or `manual`.
